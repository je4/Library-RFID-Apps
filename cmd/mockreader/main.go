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
	configFile   = flag.String("config", "", "location of toml configuration file")
	wsBroker     = flag.String("wsbroker", "", "override WSBroker websocket endpoint URL")
	channels     = flag.String("channels", "", "comma-separated list of target WSBroker channels")
	interval     = flag.String("interval", "", "interval between tag emissions (e.g. 2s, 500ms)")
	afiMode      = flag.String("afi", "", "AFI security mode: random, open, closed")
	readerID     = flag.String("reader-id", "", "logical reader identifier")
	barcodes     = flag.String("barcodes", "", "comma-separated list of barcodes/item-IDs to send as mock UIDs")
	barcodeShort = flag.String("b", "", "comma-separated list of barcodes/item-IDs (shorthand)")
	empty        = flag.Bool("empty", false, "send empty reader (tags_cleared) event and exit")
	clear        = flag.Bool("clear", false, "alias for -empty: send empty reader (tags_cleared) event and exit")
	clearBefore  = flag.Bool("clear-before", true, "send empty reader (tags_cleared) event before sending barcode tags")
	once         = flag.Bool("once", false, "send specified barcodes once and exit (default when barcodes are specified)")
	loop         = flag.Bool("loop", false, "run continuously in loop even when barcodes are provided via CLI")
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

	// Collect CLI barcodes from flags (-barcodes, -b) and trailing positional arguments
	var cliBarcodes []string
	addBarcodes := func(s string) {
		for _, p := range strings.Split(s, ",") {
			trimmed := strings.TrimSpace(p)
			if trimmed != "" {
				cliBarcodes = append(cliBarcodes, trimmed)
			}
		}
	}
	if *barcodes != "" {
		addBarcodes(*barcodes)
	}
	if *barcodeShort != "" {
		addBarcodes(*barcodeShort)
	}
	for _, arg := range flag.Args() {
		addBarcodes(arg)
	}

	if len(cliBarcodes) > 0 {
		conf.Barcodes = cliBarcodes
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
		Bool("emptyFlag", *empty || *clear).
		Bool("loopFlag", *loop).
		Msg("starting mock tag reader emulator")

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

	// Mode A: Send empty reader event only and exit
	if (*empty || *clear) && len(cliBarcodes) == 0 {
		logger.Info().Msg("sending empty reader (tags_cleared) event")
		if err := emulator.EmitTagsCleared(context.Background()); err != nil {
			logger.Error().Err(err).Msg("failed to emit tags_cleared event")
		}
		time.Sleep(150 * time.Millisecond)
		_ = emulator.Close()
		_ = brokerClient.Close()
		logger.Info().Msg("empty reader event dispatched successfully, exiting")
		return
	}

	// Mode B: Send specific barcodes once (with empty before sending) and exit
	if len(cliBarcodes) > 0 && !*loop {
		shouldClear := *clearBefore || *empty || *clear
		logger.Info().
			Strs("barcodes", cliBarcodes).
			Bool("clearBefore", shouldClear).
			Msg("sending mock-UID tag read events for specified barcodes")

		if err := emulator.SendBarcodes(context.Background(), cliBarcodes, shouldClear); err != nil {
			logger.Error().Err(err).Msg("encountered error while sending barcode tags")
		}
		time.Sleep(200 * time.Millisecond)
		_ = emulator.Close()
		_ = brokerClient.Close()
		logger.Info().Int("count", len(cliBarcodes)).Msg("mock-UID tags dispatched successfully, exiting")
		return
	}

	// Mode C: Continuous daemon emulator loop
	if *clearBefore || *empty || *clear {
		_ = emulator.EmitTagsCleared(context.Background())
		time.Sleep(50 * time.Millisecond)
	}

	runCtx, runCancel := context.WithCancel(context.Background())
	defer runCancel()

	go func() {
		if err := emulator.Run(runCtx); err != nil && err != context.Canceled {
			logger.Error().Err(err).Msg("emulator generation loop encountered error")
		}
	}()

	logger.Info().Msg("mock reader emulator is operational and broadcasting tags")

	// Graceful shutdown on OS signal (SIGINT, SIGTERM)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigChan
	logger.Info().Str("signal", sig.String()).Msg("termination signal received, shutting down gracefully...")

	runCancel()
	_ = emulator.Close()
	_ = brokerClient.Close()

	logger.Info().Msg("mockreader daemon stopped cleanly")
}
