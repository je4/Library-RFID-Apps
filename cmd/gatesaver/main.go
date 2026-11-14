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
	loggerZero := zerolog.New(output).With().Timestamp().Str("app", "gatesaver").Logger()
	var logger zLogger.ZLogger = &loggerZero

	logger.Info().
		Str("name", conf.Name).
		Str("broker", conf.WSBroker).
		Strs("channels", conf.Channels).
		Msg("starting gatesaver")

	// 3. Initialize WSBroker and RFID Client
	brokerCfg := wsbrokerclient.DefaultConfig()
	brokerCfg.Endpoint = conf.WSBroker
	brokerCfg.ClientName = conf.Name
	brokerCfg.AutoReconnect = true
	brokerCfg.Logger = logger
	brokerCfg.InitialChannels = conf.Channels

	if conf.ReadLimit > 0 {
		brokerCfg.ReadLimit = conf.ReadLimit
	}
	if conf.ClientTLS.Type != "" || conf.ClientTLS.File != nil || conf.ClientTLS.Vault != nil || conf.ClientTLS.Dev != nil || conf.ClientTLS.Env != nil || conf.ClientTLS.ACME != nil {
		brokerCfg.TLSCertLoader = &conf.ClientTLS
	}

	client, err := libraryclient.NewFromConfig(brokerCfg, libraryclient.WithLogger(logger))
	if err != nil {
		logger.Fatal().Err(err).Msg("cannot initialize rfid client")
	}

	// 4. Register event listener for gate tagreader events
	client.OnTagRead(func(evt *libraryclient.TagReadEvent) {
		handleTagRead(logger, evt)
	})

	// 5. Connect to WSBroker
	if err := client.Connect(context.Background()); err != nil {
		logger.Warn().Err(err).Msg("failed to initiate initial connection to wsbroker")
	}

	logger.Info().Msg("gatesaver running, waiting for tag events...")

	// 6. Graceful shutdown
	done := make(chan os.Signal, 1)
	signal.Notify(done, syscall.SIGINT, syscall.SIGTERM)
	<-done
	signal.Stop(done)

	logger.Info().Msg("shutting down gatesaver...")
	if err := client.Close(); err != nil {
		logger.Warn().Err(err).Msg("error closing rfid client")
	}
	logger.Info().Msg("gatesaver stopped")
}

// handleTagRead processes incoming TagReadEvent and will persist tag data to the database.
func handleTagRead(logger zLogger.ZLogger, evt *libraryclient.TagReadEvent) {
	tag := evt.Data
	timeStr := ""
	if tag.Time != nil {
		timeStr = tag.Time.Format(time.RFC3339)
	}

	logger.Info().
		Str("channel", evt.Channel).
		Str("source", evt.Source).
		Str("reader_id", tag.ReaderID).
		Str("uid", tag.UID).
		Str("item_id", tag.ItemID).
		Str("isil", tag.ISIL).
		Str("afi", tag.AFI).
		Bool("crc_ok", tag.CRCOk).
		Str("tag_time", timeStr).
		Msg("tag read event received")

	// TODO: Persist tag read event to database (database integration to be implemented)
}
