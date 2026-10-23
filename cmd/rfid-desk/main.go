package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/je4/Library-RFID-Apps/pkg/desk"
	"github.com/je4/Library-RFID-Apps/pkg/rfidclient"
	"github.com/je4/Library-RFID-Apps/pkg/web"
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
	loggerZero := zerolog.New(output).With().Timestamp().Str("app", "rfid-desk").Logger()
	var logger zLogger.ZLogger = &loggerZero

	addr := conf.Addr
	if addr == "" {
		addr = "localhost:8762"
	}
	extAddr := conf.ExtAddr
	if extAddr == "" {
		extAddr = fmt.Sprintf("http://%s", addr)
	}

	appStartTime := time.Now().UTC()

	// 3. Initialize in-memory Event Hub
	maxTags := conf.MaxTags
	if maxTags <= 0 {
		maxTags = 100
	}
	hub := desk.NewHub(maxTags)
	hubCtx, hubCancel := context.WithCancel(context.Background())
	defer hubCancel()
	hub.Start(hubCtx)

	// 4. Initialize Web Controller with embedded templates and assets
	webCtrl, err := web.NewController(web.Config{
		Addr:          addr,
		ExtAddr:       extAddr,
		ClientName:    conf.Name,
		WSBroker:      conf.WSBroker,
		Channels:      conf.Channels,
		DefaultReader: conf.DefaultReader,
		Hub:           hub,
		Logger:        logger,
		StartTime:     appStartTime,
	})
	if err != nil {
		logger.Fatal().Err(err).Msg("cannot initialize web controller")
	}

	// 5. Initialize WSBroker and RFID Client
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

	rfidOpts := []rfidclient.Option{
		rfidclient.WithLogger(logger),
	}
	if conf.DefaultReader != "" {
		rfidOpts = append(rfidOpts, rfidclient.WithDefaultTarget(conf.DefaultReader))
	}

	rfidCli, err := rfidclient.NewFromConfig(brokerCfg, rfidOpts...)
	if err != nil {
		logger.Fatal().Err(err).Msg("cannot initialize rfid client")
	}

	// 6. Register event listeners for tag scans and antenna field events
	rfidCli.OnTagRead(func(tag *rfidclient.TagReadEvent, rawEvt *wsbrokerclient.Event) {
		if tag.ClientID == "" && rawEvt != nil && rawEvt.Source != "" {
			tag.ClientID = rawEvt.Source
		}

		logger.Info().
			Str("uid", tag.UID).
			Str("itemId", tag.ItemID).
			Str("isil", tag.ISIL).
			Str("reader", tag.ClientID).
			Str("afi", tag.AFI).
			Bool("crcOk", tag.IsCRCValidStatus()).
			Msg("NFC/RFID tag scanned")

		webCtrl.HandleTagRead(tag)
	})

	rfidCli.OnTagsCleared(func(cleared *rfidclient.TagsClearedEvent, rawEvt *wsbrokerclient.Event) {
		logger.Info().Msg("RFID antenna field cleared (tags removed)")
		webCtrl.HandleTagsCleared()
	})

	// 7. Start Web Server in background
	var wg sync.WaitGroup
	webCtrl.Start(&wg)

	// 8. Connect to WSBroker
	if err := rfidCli.Connect(context.Background()); err != nil {
		logger.Warn().Err(err).Msg("failed to initiate initial connection to wsbroker")
	}

	fmt.Println("==================================================")
	fmt.Printf(" RFID Desk Web Application Started\n")
	fmt.Printf(" Client:    %s\n", conf.Name)
	fmt.Printf(" Broker:    %s\n", conf.WSBroker)
	fmt.Printf(" Channels:  %v\n", conf.Channels)
	fmt.Printf(" Web UI:    %s (%s/desk)\n", extAddr, extAddr)
	fmt.Printf(" SSE Stream:%s/events\n", extAddr)
	fmt.Println("==================================================")
	fmt.Println("Press Ctrl+C to terminate...")

	// 9. Await termination signals for graceful shutdown
	done := make(chan os.Signal, 1)
	signal.Notify(done, syscall.SIGINT, syscall.SIGTERM)
	<-done

	fmt.Println("\nShutting down gracefully...")
	hubCancel()
	hub.Close()

	if err := rfidCli.Close(); err != nil {
		logger.Warn().Err(err).Msg("error closing rfid client")
	}

	webCtrl.GracefulStop()
	wg.Wait()

	fmt.Println("RFID Desk stopped.")
}
