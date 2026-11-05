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
	conf, err := LoadConfig(*configfile)
	if err != nil {
		log.Fatalf("cannot load configuration: %v", err)
	}

	output := zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339}
	loggerZero := zerolog.New(output).With().Timestamp().Str("app", "selfcheckout").Logger()
	var logger zLogger.ZLogger = &loggerZero

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
		logger.Fatal().Err(err).Msg("cannot initialize library client")
	}
	if err := client.Connect(context.Background()); err != nil {
		logger.Warn().Err(err).Msg("failed to initiate initial connection to wsbroker")
	}

	logger.Info().Str("client", conf.Name).Str("broker", conf.WSBroker).Strs("channels", conf.Channels).Msg("selfcheckout started")
	done := make(chan os.Signal, 1)
	signal.Notify(done, syscall.SIGINT, syscall.SIGTERM)
	<-done
	signal.Stop(done)
	if err := client.Close(); err != nil {
		logger.Warn().Err(err).Msg("error closing library client")
	}
}
