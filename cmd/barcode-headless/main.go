package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/je4/Library-RFID-Apps/pkg/barcodeclient"
	"github.com/je4/Library-RFID-Apps/pkg/barcodescanner"
	"github.com/je4/utils/v2/pkg/zLogger"
	"github.com/je4/wsbrokerclient"
	"github.com/rs/zerolog"
)

var configfile = flag.String("config", "", "location of toml configuration file")

func main() {
	flag.Parse()

	// 1. Load application configuration (from file or embedded default)
	conf, err := LoadConfig(*configfile)
	if err != nil {
		log.Fatalf("cannot load configuration: %v", err)
	}

	// 2. Initialize structured logging
	output := zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339}
	loggerZero := zerolog.New(output).With().Timestamp().Str("app", "barcode-headless").Logger()
	var logger zLogger.ZLogger = &loggerZero

	logger.Info().
		Str("clientName", conf.Name).
		Str("wsbroker", conf.WSBroker).
		Strs("channels", conf.Channels).
		Int("configuredReaders", len(conf.Readers)).
		Msg("starting barcode-headless daemon")

	// 3. Initialize Reader Registry and instances
	registry := barcodescanner.NewBarcodeReaderRegistry()
	for _, rCfg := range conf.Readers {
		if !rCfg.Enabled {
			logger.Info().Str("readerId", rCfg.ID).Str("port", rCfg.Port).Msg("reader disabled in configuration, skipping")
			continue
		}
		inst := barcodescanner.NewBarcodeReaderInstance(rCfg, logger)
		registry.RegisterReader(inst)
		logger.Info().
			Str("readerId", rCfg.ID).
			Str("device", rCfg.DeviceID).
			Str("port", rCfg.Port).
			Int("baudRate", rCfg.BaudRate).
			Strs("channels", rCfg.Channels).
			Msg("registered serial barcode reader")
	}

	// 4. Initialize WSBroker client with mTLS / Certloader
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

	// 5. Initialize WSBroker Dispatcher
	dispatcher, err := barcodescanner.NewBarcodeWsBrokerDispatcher(
		conf.Name,
		conf.Channels,
		registry,
		brokerClient,
		logger,
	)
	if err != nil {
		logger.Fatal().Err(err).Msg("cannot initialize barcode WSBroker dispatcher")
	}
	defer dispatcher.Close()

	// 6. Connect to WSBroker
	connectCtx, connectCancel := context.WithTimeout(context.Background(), 10*time.Second)
	_ = dispatcher.Connect(connectCtx)
	connectCancel()

	// 7. Start Barcode Scanners in Background
	scanCtx, scanCancel := context.WithCancel(context.Background())
	defer scanCancel()

	err = registry.StartAll(scanCtx, func(scan *barcodeclient.BarcodeReadData) {
		dispatcher.DispatchScan(context.Background(), scan)
	})
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to start serial barcode readers")
	}

	logger.Info().Msg("barcode-headless is operational and scanning for barcodes")

	// 8. Wait for OS termination signal (SIGINT / SIGTERM)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigChan
	logger.Info().Str("signal", sig.String()).Msg("termination signal received, shutting down gracefully...")

	// Graceful shutdown
	scanCancel()
	registry.StopAll()
	_ = dispatcher.Close()

	logger.Info().Msg("barcode-headless daemon stopped cleanly")
}
