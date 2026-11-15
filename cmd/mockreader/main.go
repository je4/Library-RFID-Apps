package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/je4/utils/v2/pkg/zLogger"
	"github.com/je4/wsbrokerclient"
	"github.com/rs/zerolog"
)

var (
	configFile = flag.String("config", "", "location of toml configuration file")
	wsBroker   = flag.String("wsbroker", "", "override WSBroker websocket endpoint URL")
	channels   = flag.String("channels", "", "comma-separated list of target WSBroker channels")
	interval   = flag.String("interval", "", "interval between tag emissions (e.g. 2s, 500ms)")
	afiMode    = flag.String("afi", "", "AFI security mode: random, open, closed")
	readerID   = flag.String("reader-id", "", "logical reader identifier")
)

func main() {
	flag.Parse()

	// 1. Load configuration from file or embedded default
	conf, err := LoadConfig(*configFile)
	if err != nil {
		log.Fatalf("cannot load configuration: %v", err)
	}

	// 2. Apply CLI flag overrides if provided
	if *wsBroker != "" {
		conf.WSBroker = *wsBroker
	}
	if *channels != "" {
		parts := strings.Split(*channels, ",")
		var cleaned []string
		for _, p := range parts {
			trimmed := strings.TrimSpace(p)
			if trimmed != "" {
				cleaned = append(cleaned, trimmed)
			}
		}
		if len(cleaned) > 0 {
			conf.Channels = cleaned
		}
	}
	if *interval != "" {
		conf.Interval = *interval
		if d, err := time.ParseDuration(*interval); err == nil {
			conf.IntervalDuration = d
		}
	}
	if *afiMode != "" {
		conf.AFIMode = *afiMode
	}
	if *readerID != "" {
		conf.ReaderID = *readerID
	}

	// 3. Initialize structured logging
	output := zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339}
	loggerZero := zerolog.New(output).With().Timestamp().Str("app", "mockreader").Logger()
	var logger zLogger.ZLogger = &loggerZero

	logger.Info().
		Str("clientName", conf.Name).
		Str("wsbroker", conf.WSBroker).
		Strs("channels", conf.Channels).
		Str("readerId", conf.ReaderID).
		Str("interval", conf.Interval).
		Str("afiMode", conf.AFIMode).
		Int("barcodesCount", len(conf.Barcodes)).
		Msg("starting mock tag reader emulator daemon")

	// 4. Initialize WSBroker client
	brokerCfg := wsbrokerclient.DefaultConfig()
	brokerCfg.Endpoint = conf.WSBroker
	brokerCfg.ClientName = conf.Name
	brokerCfg.AutoReconnect = true
	brokerCfg.Logger = logger

	if conf.ReadLimit > 0 {
		brokerCfg.ReadLimit = conf.ReadLimit
	}
	if len(conf.Channels) > 0 {
		brokerCfg.InitialChannels = conf.Channels
	}
	if conf.ClientTLS.Type != "" || conf.ClientTLS.File != nil || conf.ClientTLS.Vault != nil || conf.ClientTLS.Dev != nil || conf.ClientTLS.Env != nil || conf.ClientTLS.ACME != nil {
		brokerCfg.TLSCertLoader = &conf.ClientTLS
	}

	brokerClient, err := wsbrokerclient.New(brokerCfg)
	if err != nil {
		logger.Fatal().Err(err).Msg("cannot initialize WSBroker client")
	}

	// 5. Initialize MockReader Emulator Engine
	emulator, err := NewMockReader(conf, brokerClient, logger)
	if err != nil {
		logger.Fatal().Err(err).Msg("cannot initialize mock reader emulator")
	}
	defer emulator.Close()

	// 6. Connect to WSBroker
	connectCtx, connectCancel := context.WithTimeout(context.Background(), 10*time.Second)
	if err := brokerClient.Connect(connectCtx); err != nil {
		logger.Warn().Err(err).Msg("initial connection to WSBroker failed (auto-reconnect active)")
	}
	connectCancel()

	// Attach to initial channels
	if len(conf.Channels) > 0 {
		attachCtx, attachCancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := brokerClient.Attach(attachCtx, conf.Channels...); err != nil {
			logger.Warn().Err(err).Strs("channels", conf.Channels).Msg("failed to attach to initial channels")
		}
		attachCancel()
	}

	// 7. Start Tag Emulator Loop in background
	runCtx, runCancel := context.WithCancel(context.Background())
	defer runCancel()

	go func() {
		if err := emulator.Run(runCtx); err != nil && err != context.Canceled {
			logger.Error().Err(err).Msg("emulator generation loop encountered error")
		}
	}()

	logger.Info().Msg("mock reader emulator is operational and broadcasting tags")

	// 8. Graceful shutdown on OS signal (SIGINT, SIGTERM)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigChan
	logger.Info().Str("signal", sig.String()).Msg("termination signal received, shutting down gracefully...")

	runCancel()
	_ = emulator.Close()
	_ = brokerClient.Close()

	logger.Info().Msg("mockreader daemon stopped cleanly")
}
