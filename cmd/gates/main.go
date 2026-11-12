package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/je4/Library-RFID-Apps/pkg/libraryclient"
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
	loggerZero := zerolog.New(output).With().Timestamp().Str("app", "gates").Logger()
	var logger zLogger.ZLogger = &loggerZero

	logger.Info().
		Str("clientName", conf.Name).
		Str("wsbroker", conf.WSBroker).
		Strs("channels", conf.Channels).
		Int("configuredGates", len(conf.Gates)).
		Msg("starting gates daemon")

	// 3. Initialize configured gate instances
	for _, gCfg := range conf.Gates {
		if !gCfg.Enabled {
			logger.Info().Str("gateId", gCfg.ID).Str("port", gCfg.Port).Msg("gate disabled in configuration, skipping")
			continue
		}
		logger.Info().
			Str("gateId", gCfg.ID).
			Str("device", gCfg.DeviceID).
			Str("deviceType", gCfg.DeviceType).
			Str("port", gCfg.Port).
			Int("baudRate", gCfg.BaudRate).
			Strs("channels", gCfg.Channels).
			Msg("configured gate reader")
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

	// 5. Initialize unified library client attached to WSBroker channels
	client, err := libraryclient.NewFromConfig(brokerCfg, libraryclient.WithLogger(logger))
	if err != nil {
		logger.Fatal().Err(err).Msg("cannot initialize library client for gates")
	}

	// 6. Register event listeners for tag passages, security alarms, and metadata
	client.OnTagRead(func(tag *libraryclient.TagReadEvent) {
		isSecured := libraryclient.IsAFISecured(tag.Data.AFI)
		secStatus := libraryclient.AFISecurityStatus(tag.Data.AFI)

		logEvt := logger.Info()
		if isSecured {
			logEvt = logger.Warn().Str("alarm", "AFI_SECURED_TRIGGERED")
		}

		logEvt.
			Str("id", tag.ID).
			Str("refId", tag.RefID).
			Str("uid", tag.Data.UID).
			Str("itemId", tag.Data.ItemID).
			Str("isil", tag.Data.ISIL).
			Str("reader", tag.Data.ReaderID).
			Str("source", tag.Source).
			Str("channel", tag.Channel).
			Str("afi", tag.Data.AFI).
			Str("securityStatus", secStatus).
			Bool("isSecured", isSecured).
			Bool("crcOk", tag.Data.IsCRCValidStatus()).
			Msg("Gate passage: RFID tag detected")
	})

	client.OnTagsCleared(func(cleared *libraryclient.TagsClearedEvent) {
		logger.Debug().
			Str("id", cleared.ID).
			Str("refId", cleared.RefID).
			Str("reader", cleared.Data.ReaderID).
			Msg("Gate passage field cleared")
	})

	client.OnBarcodeRead(func(barcodeEvt *libraryclient.BarcodeReadEvent) {
		logger.Info().
			Str("id", barcodeEvt.ID).
			Str("barcode", barcodeEvt.Data.Barcode).
			Str("itemId", barcodeEvt.Data.ItemID).
			Str("reader", barcodeEvt.Data.ReaderID).
			Str("symbology", barcodeEvt.Data.Symbology).
			Str("channel", barcodeEvt.Channel).
			Msg("Gate barcode read received")
	})

	// 7. Connect to WSBroker
	connectCtx, connectCancel := context.WithTimeout(context.Background(), 10*time.Second)
	if err := client.ConnectSync(connectCtx); err != nil {
		logger.Warn().Err(err).Msg("failed initial sync connection to wsbroker, will retry asynchronously in background")
		_ = client.Connect(context.Background())
	}
	connectCancel()

	logger.Info().
		Str("client", conf.Name).
		Str("wsbroker", conf.WSBroker).
		Strs("channels", conf.Channels).
		Msg("gates daemon is operational and connected to wsbroker channels")

	// 8. Wait for OS termination signal (SIGINT / SIGTERM)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigChan
	logger.Info().Str("signal", sig.String()).Msg("termination signal received, shutting down gracefully...")

	// Graceful shutdown
	signal.Stop(sigChan)
	if err := client.Close(); err != nil {
		logger.Warn().Err(err).Msg("error closing library client")
	}

	logger.Info().Msg("gates daemon stopped cleanly")
}
