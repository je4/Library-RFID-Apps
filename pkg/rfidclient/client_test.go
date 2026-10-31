package rfidclient

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/je4/wsbrokerclient"
	"github.com/rs/zerolog"
)

func TestNewFromConfig_InheritsBrokerLogger(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf)

	cfg := wsbrokerclient.DefaultConfig()
	cfg.ClientName = "test-client"
	cfg.Endpoint = "wss://localhost:8080/ws"
	cfg.Logger = &logger

	cli, err := NewFromConfig(cfg)
	if err != nil {
		t.Fatalf("NewFromConfig failed: %v", err)
	}
	if cli == nil {
		t.Fatal("expected non-nil client")
	}
	if cli.logger == nil {
		t.Fatal("expected non-nil logger")
	}
	cli.logger.Info().Msg("inherited test message")
	if !strings.Contains(buf.String(), "inherited test message") {
		t.Errorf("expected log buffer to contain message, got: %s", buf.String())
	}
}

func TestNewFromConfig_ExplicitWithLoggerOverrides(t *testing.T) {
	var buf1, buf2 bytes.Buffer
	brokerLogger := zerolog.New(&buf1)
	overrideLogger := zerolog.New(&buf2)

	cfg := wsbrokerclient.DefaultConfig()
	cfg.ClientName = "test-client"
	cfg.Endpoint = "wss://localhost:8080/ws"
	cfg.Logger = &brokerLogger

	cli, err := NewFromConfig(cfg, WithLogger(&overrideLogger))
	if err != nil {
		t.Fatalf("NewFromConfig failed: %v", err)
	}
	cli.logger.Info().Msg("override message")
	if strings.Contains(buf1.String(), "override message") {
		t.Errorf("broker logger should NOT have received message")
	}
	if !strings.Contains(buf2.String(), "override message") {
		t.Errorf("expected override logger to contain message, got: %s", buf2.String())
	}
}

func TestRFIDClient_NilSafety(t *testing.T) {
	fake := newFakeBroker("controller-app")
	cli, err := New(fake, WithLogger(nil))
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	if cli.logger == nil {
		t.Fatal("expected logger to fallback to non-nil")
	}
	// Calling methods shouldn't panic
	_ = cli.BroadcastAFI(context.Background(), "all", NewWriteAFIRequest("07", "E00401"))
	_ = cli.SendAFI(context.Background(), "reader-01", NewWriteAFIRequest("07", "E00401"))
}

func TestRFIDClient_Request0_Commands(t *testing.T) {
	fake := newFakeBroker("controller-app")
	rfid, err := New(fake)
	if err != nil {
		t.Fatalf("failed to create RFIDClient: %v", err)
	}

	ctx := context.Background()
	tags := []TagWriteItem{
		{UID: "UID-1", ItemID: "ITEM-1"},
	}
	if err := rfid.SendWriteTagBatch(ctx, "daemon-01", tags); err != nil {
		t.Errorf("SendWriteTagBatch failed: %v", err)
	}
	if err := rfid.BroadcastWriteTagBatch(ctx, "all", tags); err != nil {
		t.Errorf("BroadcastWriteTagBatch failed: %v", err)
	}

	afiReq := NewWriteAFIRequest("07", "UID-1")
	if err := rfid.SendAFI(ctx, "daemon-01", afiReq); err != nil {
		t.Errorf("SendAFI failed: %v", err)
	}
	if err := rfid.BroadcastAFI(ctx, "all", afiReq); err != nil {
		t.Errorf("BroadcastAFI failed: %v", err)
	}

	// Scoped commands
	scoped := rfid.ForReader("gate-01")
	if err := scoped.SendWriteTagBatch(ctx, "daemon-01", tags); err != nil {
		t.Errorf("scoped.SendWriteTagBatch failed: %v", err)
	}
	if err := scoped.BroadcastWriteTagBatch(ctx, "all", tags); err != nil {
		t.Errorf("scoped.BroadcastWriteTagBatch failed: %v", err)
	}
	if err := scoped.SendAFI(ctx, "daemon-01", afiReq); err != nil {
		t.Errorf("scoped.SendAFI failed: %v", err)
	}
	if err := scoped.BroadcastAFI(ctx, "all", afiReq); err != nil {
		t.Errorf("scoped.BroadcastAFI failed: %v", err)
	}
}
