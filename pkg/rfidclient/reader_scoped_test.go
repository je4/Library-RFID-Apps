package rfidclient

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/je4/wsbrokerclient/pkg/event"
)

func TestRFIDClient_MultiReader_ForReader(t *testing.T) {
	fake := newFakeBroker("controller-app")
	rfid, err := New(fake, WithDefaultTarget("daemon-01"), WithDefaultReaderID("default-reader"))
	if err != nil {
		t.Fatalf("failed to create RFIDClient: %v", err)
	}

	if rfid.DefaultReaderID() != "default-reader" {
		t.Errorf("DefaultReaderID() = %q, want 'default-reader'", rfid.DefaultReaderID())
	}

	gate1Client := rfid.ForReader("gate-01")
	if gate1Client.DefaultReaderID() != "gate-01" {
		t.Errorf("gate1Client.DefaultReaderID() = %q, want 'gate-01'", gate1Client.DefaultReaderID())
	}
	if gate1Client.BrokerClient() != fake {
		t.Error("expected BrokerClient() to match fake broker")
	}

	// 1. Test GetDeviceMetadata via ForReader
	fake.requestFunc = func(ctx context.Context, reqEvt *event.RawEvent) (*event.RawEvent, error) {
		var req DeviceMetadataRequest
		if err := json.Unmarshal(reqEvt.Data, &req); err != nil {
			t.Fatalf("failed to unmarshal request: %v", err)
		}
		if req.ReaderID != "gate-01" {
			t.Errorf("expected reader_id 'gate-01', got %q", req.ReaderID)
		}

		respData := DeviceMetadataResponseData{
			ReaderID:        req.ReaderID,
			Status:          "ok",
			DeviceID:        "FEIG-01",
			DeviceType:      "LRU1002",
			InitDescription: "Reader initialized for gate-01",
		}
		dataBytes, _ := json.Marshal(respData)
		return &event.RawEvent{
			BaseEvent: event.BaseEvent{
				ID:      "resp-meta-001",
				RefID:   reqEvt.ID,
				Source:  "daemon-01",
				Target:  "controller-app",
				Session: reqEvt.Session,
				Type:    EventDeviceMetadataResponse,
			},
			Data: dataBytes,
		}, nil
	}

	metaResp, err := gate1Client.GetDeviceMetadata(context.Background(), "daemon-01")
	if err != nil {
		t.Fatalf("GetDeviceMetadata failed: %v", err)
	}
	if metaResp.Data.ReaderID != "gate-01" {
		t.Errorf("metaResp.ReaderID = %q, want 'gate-01'", metaResp.Data.ReaderID)
	}

	// 2. Test WriteTag via ForReader
	fake.requestFunc = func(ctx context.Context, reqEvt *event.RawEvent) (*event.RawEvent, error) {
		var req WriteTagRequest
		if err := json.Unmarshal(reqEvt.Data, &req); err != nil {
			t.Fatalf("failed to unmarshal request: %v", err)
		}
		if req.ReaderID != "gate-01" {
			t.Errorf("expected reader_id 'gate-01', got %q", req.ReaderID)
		}
		if req.ItemID != "ITEM-456" {
			t.Errorf("expected itemId 'ITEM-456', got %q", req.ItemID)
		}

		respData := WriteTagResponseData{
			ReaderID: req.ReaderID,
			Status:   "ok",
			UID:      req.UID,
			ItemID:   req.ItemID,
		}
		dataBytes, _ := json.Marshal(respData)
		return &event.RawEvent{
			BaseEvent: event.BaseEvent{
				ID:      "resp-write-001",
				RefID:   reqEvt.ID,
				Source:  "daemon-01",
				Target:  "controller-app",
				Session: reqEvt.Session,
				Type:    EventWriteTagResponse,
			},
			Data: dataBytes,
		}, nil
	}

	writeReq := NewWriteTagRequest("ITEM-456", "UID-456")
	writeResp, err := gate1Client.WriteTag(context.Background(), "daemon-01", writeReq)
	if err != nil {
		t.Fatalf("WriteTag failed: %v", err)
	}
	if writeResp.Data.ReaderID != "gate-01" {
		t.Errorf("writeResp.ReaderID = %q, want 'gate-01'", writeResp.Data.ReaderID)
	}

	// 3. Test WriteAFI via ForReader
	fake.requestFunc = func(ctx context.Context, reqEvt *event.RawEvent) (*event.RawEvent, error) {
		var req WriteAFIRequest
		if err := json.Unmarshal(reqEvt.Data, &req); err != nil {
			t.Fatalf("failed to unmarshal request: %v", err)
		}
		if req.ReaderID != "gate-01" {
			t.Errorf("expected reader_id 'gate-01', got %q", req.ReaderID)
		}
		if req.AFI != "07" {
			t.Errorf("expected afi '07', got %q", req.AFI)
		}

		respData := WriteAFIResponseData{
			ReaderID: req.ReaderID,
			Status:   "ok",
			UID:      req.UID,
			AFI:      req.AFI,
		}
		dataBytes, _ := json.Marshal(respData)
		return &event.RawEvent{
			BaseEvent: event.BaseEvent{
				ID:      "resp-afi-001",
				RefID:   reqEvt.ID,
				Source:  "daemon-01",
				Target:  "controller-app",
				Session: reqEvt.Session,
				Type:    EventWriteAFIResponse,
			},
			Data: dataBytes,
		}, nil
	}

	afiResp, err := gate1Client.SecureTag(context.Background(), "daemon-01", "UID-456")
	if err != nil {
		t.Fatalf("SecureTag failed: %v", err)
	}
	if afiResp.Data.ReaderID != "gate-01" {
		t.Errorf("afiResp.ReaderID = %q, want 'gate-01'", afiResp.Data.ReaderID)
	}
}

func TestRFIDClient_MultiReader_EventFiltering(t *testing.T) {
	fake := newFakeBroker("controller-app")
	rfid, err := New(fake)
	if err != nil {
		t.Fatalf("failed to create RFIDClient: %v", err)
	}

	gate1Tags := make([]*TagReadEvent, 0)
	gate2Tags := make([]*TagReadEvent, 0)
	var mu sync.Mutex

	unsub1 := rfid.OnTagReadForReader("gate-01", func(tag *TagReadEvent) {
		mu.Lock()
		gate1Tags = append(gate1Tags, tag)
		mu.Unlock()
	})
	defer unsub1()

	unsub2 := rfid.OnTagReadForReader("gate-02", func(tag *TagReadEvent) {
		mu.Lock()
		gate2Tags = append(gate2Tags, tag)
		mu.Unlock()
	})
	defer unsub2()

	// Dispatch an event for gate-01
	evt1 := &event.RawEvent{
		BaseEvent: event.BaseEvent{
			ID:     "evt-1",
			Source: "daemon-01",
			Target: "all",
			Type:   EventTagRead,
		},
		Data: []byte(`{
			"readerId": "gate-01",
			"uid": "UID-G1",
			"itemId": "BOOK-G1"
		}`),
	}
	_, err = rfid.HandleEvent(evt1)
	if err != nil {
		t.Fatalf("HandleEvent evt1 failed: %v", err)
	}

	// Dispatch an event for gate-02
	evt2 := &event.RawEvent{
		BaseEvent: event.BaseEvent{
			ID:     "evt-2",
			Source: "daemon-01",
			Target: "all",
			Type:   EventTagRead,
		},
		Data: []byte(`{
			"readerId": "gate-02",
			"uid": "UID-G2",
			"itemId": "BOOK-G2"
		}`),
	}
	_, err = rfid.HandleEvent(evt2)
	if err != nil {
		t.Fatalf("HandleEvent evt2 failed: %v", err)
	}

	// Give goroutines time to run
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if len(gate1Tags) != 1 || gate1Tags[0].Data.UID != "UID-G1" {
		t.Errorf("expected gate1 to receive 1 tag UID-G1, got: %+v", gate1Tags)
	}
	if len(gate2Tags) != 1 || gate2Tags[0].Data.UID != "UID-G2" {
		t.Errorf("expected gate2 to receive 1 tag UID-G2, got: %+v", gate2Tags)
	}
}

func TestRFIDClient_MultiReader_SubscribeTagsForReader(t *testing.T) {
	fake := newFakeBroker("controller-app")
	rfid, err := New(fake)
	if err != nil {
		t.Fatalf("failed to create RFIDClient: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	tagCh, unsub, err := rfid.SubscribeTagsForReader(ctx, "all", "gate-01")
	if err != nil {
		t.Fatalf("SubscribeTagsForReader failed: %v", err)
	}
	defer unsub()

	// Dispatch event for gate-02 (should be filtered out)
	_, _ = rfid.HandleEvent(&event.RawEvent{
		BaseEvent: event.BaseEvent{
			ID:     "evt-g2",
			Source: "daemon-01",
			Target: "all",
			Type:   EventTagRead,
		},
		Data: []byte(`{
			"readerId": "gate-02",
			"uid": "UID-G2",
			"itemId": "BOOK-G2"
		}`),
	})

	// Dispatch event for gate-01 (should be received)
	_, _ = rfid.HandleEvent(&event.RawEvent{
		BaseEvent: event.BaseEvent{
			ID:     "evt-g1",
			Source: "daemon-01",
			Target: "all",
			Type:   EventTagRead,
		},
		Data: []byte(`{
			"readerId": "gate-01",
			"uid": "UID-G1",
			"itemId": "BOOK-G1"
		}`),
	})

	select {
	case tag := <-tagCh:
		if tag.Data.UID != "UID-G1" || tag.Data.ReaderID != "gate-01" {
			t.Errorf("unexpected tag received: %+v", tag)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for gate-01 tag")
	}
}
