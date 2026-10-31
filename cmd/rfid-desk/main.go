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
	"github.com/je4/Library-RFID-Apps/pkg/libraryclient"
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

	// Initialize ISIL list
	isilList, err := desk.LoadISILList(logger)
	if err != nil {
		logger.Fatal().Err(err).Msg("cannot load ISIL list")
	}

	// 3. Initialize in-memory Event Hub
	maxTags := conf.MaxTags
	if maxTags <= 0 {
		maxTags = 100
	}
	hub := desk.NewHub(maxTags, isilList)
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
		DefaultISIL:   conf.DefaultISIL,
		DefaultAFI:    conf.DefaultAFI,
		Hub:           hub,
		ISILList:      isilList,
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

	rfidOpts := []libraryclient.Option{
		libraryclient.WithLogger(logger),
	}
	if conf.DefaultReader != "" {
		rfidOpts = append(rfidOpts, libraryclient.WithDefaultTarget(conf.DefaultReader))
	}

	rfidCli, err := libraryclient.NewFromConfig(brokerCfg, rfidOpts...)
	if err != nil {
		logger.Fatal().Err(err).Msg("cannot initialize rfid client")
	}
	webCtrl.SetRFIDClient(rfidCli)

	// 6. Register event listeners for tag scans and antenna field events
	rfidCli.OnTagRead(func(tag *libraryclient.TagReadEvent) {
		logger.Info().
			Str("id", tag.ID).
			Str("refId", tag.RefID).
			Str("uid", tag.Data.UID).
			Str("itemId", tag.Data.ItemID).
			Str("isil", tag.Data.ISIL).
			Str("reader", tag.Data.ReaderID).
			Str("source", tag.Source).
			Str("channel", tag.Channel).
			Str("afi", tag.Data.AFI).
			Bool("crcOk", tag.Data.IsCRCValidStatus()).
			Msg("NFC/RFID tag scanned")

		webCtrl.HandleTagRead(tag)
	})

	rfidCli.OnTagsCleared(func(cleared *libraryclient.TagsClearedEvent) {
		logger.Info().
			Str("id", cleared.ID).
			Str("refId", cleared.RefID).
			Msg("RFID antenna field cleared (tags removed)")
		webCtrl.HandleTagsCleared()
	})

	rfidCli.OnWriteTagResponse(func(resp *libraryclient.WriteTagResponse) {
		if resp.Data.IsSuccess() {
			logger.Info().
				Str("id", resp.ID).
				Str("refId", resp.RefID).
				Str("uid", resp.Data.UID).
				Str("itemId", resp.Data.ItemID).
				Str("isil", resp.Data.ISIL).
				Str("afi", resp.Data.AFI).
				Msg("RFID tag write response received: success")
		} else {
			logger.Warn().
				Str("id", resp.ID).
				Str("refId", resp.RefID).
				Str("uid", resp.Data.UID).
				Str("itemId", resp.Data.ItemID).
				Str("error", resp.Data.GetError()).
				Msg("RFID tag write response received: failed")
		}
	})

	rfidCli.OnWriteAFIResponse(func(resp *libraryclient.WriteAFIResponse) {
		secStatus := libraryclient.AFISecurityStatus(resp.Data.AFI)
		if secStatus == "" {
			secStatus = resp.Data.AFI
		}
		if resp.Data.IsSuccess() {
			logEvt := logger.Info().
				Str("id", resp.ID).
				Str("refId", resp.RefID).
				Str("afi", resp.Data.AFI).
				Str("securityStatus", secStatus)
			if len(resp.Data.SuccessfulUIDs) > 0 {
				logEvt = logEvt.Strs("successfulUids", resp.Data.SuccessfulUIDs)
			} else if resp.Data.UID != "" {
				logEvt = logEvt.Str("uid", resp.Data.UID)
			}
			logEvt.Msg("RFID AFI security update confirmed")
		} else if resp.Data.IsPartial() {
			logger.Warn().
				Str("id", resp.ID).
				Str("refId", resp.RefID).
				Str("afi", resp.Data.AFI).
				Strs("successfulUids", resp.Data.SuccessfulUIDs).
				Interface("failedUids", resp.Data.FailedUIDs).
				Str("error", resp.Data.GetError()).
				Msg("RFID AFI security update partially succeeded")
		} else {
			logger.Warn().
				Str("id", resp.ID).
				Str("refId", resp.RefID).
				Str("uid", resp.Data.UID).
				Str("afi", resp.Data.AFI).
				Str("error", resp.Data.GetError()).
				Msg("RFID AFI security update failed")
		}
		webCtrl.HandleWriteAFIResponse(resp)
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
	fmt.Printf(" Alma SOAP: %s/TagService\n", extAddr)
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
