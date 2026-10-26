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

	batchAfiReq := rfidapps.NewWriteAFIBatchRequest("C2", []string{"E00401", "E00402"})
	if batchAfiReq.Event != rfidapps.EventWriteAFI || len(batchAfiReq.UIDs) != 2 || batchAfiReq.UID != "E00401" {
		t.Errorf("unexpected batch AFI request: %+v", batchAfiReq)
	}

	// Verify batch tag write constructors and types
	tagItems := []rfidapps.TagWriteItem{
		{UID: "TAG-1", ItemID: "ITEM-1", Parts: 2, PartNo: 1, AFI: "07"},
		{UID: "TAG-2", ItemID: "ITEM-2", Parts: 2, PartNo: 2, AFI: "C2"},
	}
	batchTagReq := rfidapps.NewWriteTagBatchRequest(tagItems)
	if batchTagReq.Event != rfidapps.EventWriteTag || len(batchTagReq.Tags) != 2 || batchTagReq.UID != "TAG-1" {
		t.Errorf("unexpected batch tag write request: %+v", batchTagReq)
	}

	mapTagReq := rfidapps.NewWriteTagBatchRequestFromMap(map[string]string{
		"TAG-1": "ITEM-1",
		"TAG-2": "ITEM-2",
	})
	if mapTagReq.Event != rfidapps.EventWriteTag || len(mapTagReq.Tags) != 2 {
		t.Errorf("unexpected map tag write request: %+v", mapTagReq)
	}

	var _ rfidapps.TagWriteResult
	var _ rfidapps.TagWriteItem

	// Verify AFI security constants and helper functions
	if rfidapps.AFISecured != "07" {
		t.Errorf("expected AFISecured = '07', got %s", rfidapps.AFISecured)
	}
	if rfidapps.AFIUnsecured != "C2" {
		t.Errorf("expected AFIUnsecured = 'C2', got %s", rfidapps.AFIUnsecured)
	}
	if rfidapps.AFIUnlocked != "C2" {
		t.Errorf("expected AFIUnlocked = 'C2', got %s", rfidapps.AFIUnlocked)
	}
	if !rfidapps.IsAFISecured("07") || !rfidapps.IsAFISecured("7") {
		t.Errorf("IsAFISecured failed for '07'/'7'")
	}
	if !rfidapps.IsAFIUnsecured("C2") || !rfidapps.IsAFIUnsecured("c2") {
		t.Errorf("IsAFIUnsecured failed for 'C2'/'c2'")
	}
	if rfidapps.AFISecurityStatus("07") != "Gesichert" || rfidapps.AFISecurityStatus("c2") != "Entsichert" {
		t.Errorf("AFISecurityStatus returned unexpected string")
	}

	metaReq := rfidapps.NewDeviceMetadataRequest()
	if metaReq.Event != rfidapps.EventGetDeviceMetadata {
		t.Errorf("expected EventGetDeviceMetadata, got %s", metaReq.Event)
	}

	metaReqReader := rfidapps.NewDeviceMetadataRequestForReader("gate-01")
	if metaReqReader.GetReaderID() != "gate-01" {
		t.Errorf("expected GetReaderID() = 'gate-01', got %s", metaReqReader.GetReaderID())
	}

	cfg := rfidapps.DefaultWSBrokerConfig()
	if cfg.ClientName != "" {
		t.Errorf("unexpected ClientName in default config: %s", cfg.ClientName)
	}

	payload, err := rfidapps.ParseRFIDPayload([]byte(`{"event":"rfid:tag_read","uid":"E004","itemId":"TEST-1","readerId":"gate-01"}`))
	if err != nil {
		t.Fatalf("ParseRFIDPayload failed: %v", err)
	}

	tag, ok := payload.(*rfidapps.TagReadEvent)
	if !ok || tag.ItemID != "TEST-1" || tag.GetReaderID() != "gate-01" {
		t.Errorf("unexpected parsed object: %+v", payload)
	}

	// Verify options exist
	_ = rfidapps.WithDefaultTarget("station-1")
	_ = rfidapps.WithDefaultReaderID("gate-01")
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

	rfidClient, err := rfidapps.New(client, rfidapps.WithDefaultTarget("station-1"), rfidapps.WithDefaultReaderID("gate-01"))
	if err != nil {
		t.Fatalf("failed to create rfid client: %v", err)
	}
	defer rfidClient.Close()

	if rfidClient.BrokerClient() == nil {
		t.Error("BrokerClient() should not be nil")
	}

	scopedClient := rfidClient.ForReader("gate-02")
	if scopedClient.DefaultReaderID() != "gate-02" {
		t.Errorf("scopedClient.DefaultReaderID() = %q, want 'gate-02'", scopedClient.DefaultReaderID())
	}

	_ = rfidClient.OnTagRead(func(tag *rfidapps.TagReadEvent, rawEvt *wsbrokerclient.Event) {})
	_ = rfidClient.OnTagReadForReader("gate-01", func(tag *rfidapps.TagReadEvent, rawEvt *wsbrokerclient.Event) {})
	_ = rfidClient.OnTagsCleared(func(cleared *rfidapps.TagsClearedEvent, rawEvt *wsbrokerclient.Event) {})
	_ = rfidClient.OnTagsClearedForReader("gate-01", func(cleared *rfidapps.TagsClearedEvent, rawEvt *wsbrokerclient.Event) {})
	_ = rfidClient.OnDeviceMetadata(func(meta *rfidapps.DeviceMetadataResponse, rawEvt *wsbrokerclient.Event) {})
	_ = rfidClient.OnDeviceMetadataForReader("gate-01", func(meta *rfidapps.DeviceMetadataResponse, rawEvt *wsbrokerclient.Event) {})
	_ = rfidClient.OnWriteTagResponse(func(resp *rfidapps.WriteTagResponse, rawEvt *wsbrokerclient.Event) {})
	_ = rfidClient.OnWriteTagResponseForReader("gate-01", func(resp *rfidapps.WriteTagResponse, rawEvt *wsbrokerclient.Event) {})
	_ = rfidClient.OnWriteAFIResponse(func(resp *rfidapps.WriteAFIResponse, rawEvt *wsbrokerclient.Event) {})
	_ = rfidClient.OnWriteAFIResponseForReader("gate-01", func(resp *rfidapps.WriteAFIResponse, rawEvt *wsbrokerclient.Event) {})
	_ = rfidClient.OnAnyRFIDEvent(func(rfidEvt any, rawEvt *wsbrokerclient.Event) {})
	_ = rfidClient.OnAnyRFIDEventForReader("gate-01", func(rfidEvt any, rawEvt *wsbrokerclient.Event) {})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_, unsub, err := rfidClient.SubscribeTags(ctx, "all")
	if err != nil {
		t.Fatalf("SubscribeTags failed: %v", err)
	}
	unsub()

	_, unsubReader, err := rfidClient.SubscribeTagsForReader(ctx, "all", "gate-01")
	if err != nil {
		t.Fatalf("SubscribeTagsForReader failed: %v", err)
	}
	unsubReader()
}
