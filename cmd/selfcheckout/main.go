package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/je4/Library-RFID-Apps/pkg/libraryclient"
	"github.com/je4/Library-RFID-Apps/pkg/sip2"
	"github.com/je4/utils/v2/pkg/zLogger"
	"github.com/je4/wsbrokerclient"
	"github.com/rs/zerolog"
)

var configfile = flag.String("config", "", "location of toml configuration file")

func connectSIP2(ctx context.Context, addr string, conf SIP2Config) (*sip2.Client, error) {
	client, err := sip2.Dial(ctx, addr)
	if err != nil {
		return nil, err
	}
	ok, err := client.Login(ctx, conf.Username, conf.Password, conf.Location)
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("SIP2 login failed: %w", err)
	}
	if !ok {
		client.Close()
		return nil, errors.New("SIP2 login rejected")
	}
	status, err := client.SCStatus(ctx, sip2.SCStatusRequest{StatusCode: '0', MaxPrintWidth: "080", ProtocolVersion: "2.00"})
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("SIP2 status exchange failed: %w", err)
	}
	log.Printf("SIP2 status: online=%t, protocol=%s", status.Online, status.ProtocolVersion)
	return client, nil
}

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
	tunnel, err := sip2.OpenTunnel(conf.Tunnel)
	if err != nil {
		logger.Fatal().Err(err).Msg("cannot open SIP2 tunnel")
	}
	logger.Info().Str("addr", tunnel.Addr().String()).Msg("SIP2 tunnel listening")
	loginCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	sip2Client, err := connectSIP2(loginCtx, tunnel.Addr().String(), conf.SIP2)
	cancel()
	if err != nil {
		_ = tunnel.Close()
		logger.Fatal().Err(err).Msg("cannot connect and log in to SIP2")
	}
	logger.Info().Msg("SIP2 login successful")
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
	if err := sip2Client.Close(); err != nil {
		logger.Warn().Err(err).Msg("error closing SIP2 client")
	}
	if err := tunnel.Close(); err != nil {
		logger.Warn().Err(err).Msg("error closing SIP2 tunnel")
	}
}
