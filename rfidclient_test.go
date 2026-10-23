package rfidclient_test

import (
	"context"
	"testing"
	"time"

	rfidapps "github.com/je4/Library-RFID-Apps"
	"github.com/je4/wsbrokerclient"
)

func TestRootPackageExports(t *testing.T) {
	// Verify that root package exports match pkg/rfidclient
	req := rfidapps.NewWriteTagRequest("12345", "E00401")
	if req.Event != rfidapps.EventWriteTag {
		t.Errorf("expected EventWriteTag, got %s", req.Event)
	}

	afiReq := rfidapps.NewWriteAFIRequest("07", "E00401")
	if afiReq.Event != rfidapps.EventWriteAFI {
		t.Errorf("expected EventWriteAFI, got %s", afiReq.Event)
	}

	metaReq := rfidapps.NewDeviceMetadataRequest()
	if metaReq.Event != rfidapps.EventGetDeviceMetadata {
		t.Errorf("expected EventGetDeviceMetadata, got %s", metaReq.Event)
	}

	cfg := rfidapps.DefaultWSBrokerConfig()
	if cfg.ClientName != "" {
		t.Errorf("unexpected ClientName in default config: %s", cfg.ClientName)
	}

	payload, err := rfidapps.ParseRFIDPayload([]byte(`{"event":"rfid:tag_read","uid":"E004","itemId":"TEST-1"}`))
	if err != nil {
		t.Fatalf("ParseRFIDPayload failed: %v", err)
	}

	tag, ok := payload.(*rfidapps.TagReadEvent)
	if !ok || tag.ItemID != "TEST-1" {
		t.Errorf("unexpected parsed object: %+v", payload)
	}

	// Verify options exist
	_ = rfidapps.WithDefaultTarget("station-1")
	_ = rfidapps.WithDefaultTimeout(5 * time.Second)
	_ = rfidapps.WithChannels("all", "intern")
	_ = rfidapps.WithClientName("app-1")

	// Verify wsbrokerclient compatibility
	brokerCfg := wsbrokerclient.DefaultConfig()
	brokerCfg.ClientName = "test-app"
	client, err := wsbrokerclient.New(brokerCfg)
	if err != nil {
		t.Fatalf("failed to create broker client: %v", err)
	}
	defer client.Close()

	rfidClient, err := rfidapps.New(client, rfidapps.WithDefaultTarget("station-1"))
	if err != nil {
		t.Fatalf("failed to create rfid client: %v", err)
	}
	defer rfidClient.Close()

	if rfidClient.BrokerClient() == nil {
		t.Error("BrokerClient() should not be nil")
	}

	_ = rfidClient.OnTagRead(func(tag *rfidapps.TagReadEvent, rawEvt *wsbrokerclient.Event) {})
	_ = rfidClient.OnTagsCleared(func(cleared *rfidapps.TagsClearedEvent, rawEvt *wsbrokerclient.Event) {})
	_ = rfidClient.OnDeviceMetadata(func(meta *rfidapps.DeviceMetadataResponse, rawEvt *wsbrokerclient.Event) {})
	_ = rfidClient.OnWriteTagResponse(func(resp *rfidapps.WriteTagResponse, rawEvt *wsbrokerclient.Event) {})
	_ = rfidClient.OnWriteAFIResponse(func(resp *rfidapps.WriteAFIResponse, rawEvt *wsbrokerclient.Event) {})
	_ = rfidClient.OnAnyRFIDEvent(func(rfidEvt any, rawEvt *wsbrokerclient.Event) {})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_, unsub, err := rfidClient.SubscribeTags(ctx, "all")
	if err != nil {
		t.Fatalf("SubscribeTags failed: %v", err)
	}
	unsub()
}
