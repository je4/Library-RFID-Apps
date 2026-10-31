package rfidclient

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/je4/wsbrokerclient/pkg/event"
)

func TestRFIDClient_GetDeviceMetadata(t *testing.T) {
	fake := newFakeBroker("controller-app")
	fake.requestFunc = func(ctx context.Context, evt *event.RawEvent) (*event.RawEvent, error) {
		if evt.Target != "rfid-reader-station-01" {
			t.Errorf("expected target 'rfid-reader-station-01', got %q", evt.Target)
		}
		respData := DeviceMetadataResponseData{
			ReaderID:        "rfid-reader-station-01",
			Status:          "ok",
			DeviceID:        "0052E688",
			DeviceType:      "ISC.MR102-USB",
			InitDescription: "Reader operational. RF Field ON.",
		}
		raw, _ := json.Marshal(respData)
		return &event.RawEvent{
			BaseEvent: event.BaseEvent{
				Type:    EventDeviceMetadataResponse,
				Source:  "rfid-reader-station-01",
				Target:  "controller-app",
				Session: evt.Session,
			},
			Data: raw,
		}, nil
	}

	rfid, err := New(fake, WithDefaultTarget("rfid-reader-station-01"))
	if err != nil {
		t.Fatalf("failed to create RFIDClient: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	meta, err := rfid.GetDeviceMetadata(ctx)
	if err != nil {
		t.Fatalf("GetDeviceMetadata failed: %v", err)
	}

	if !meta.Data.IsSuccess() {
		t.Errorf("expected IsSuccess=true, got false")
	}
	if meta.Data.DeviceID != "0052E688" {
		t.Errorf("DeviceID = %q, want '0052E688'", meta.Data.DeviceID)
	}
	if meta.Data.DeviceType != "ISC.MR102-USB" {
		t.Errorf("DeviceType = %q, want 'ISC.MR102-USB'", meta.Data.DeviceType)
	}
	if meta.Data.GetDescription() != "Reader operational. RF Field ON." {
		t.Errorf("GetDescription = %q", meta.Data.GetDescription())
	}
}

func TestRFIDClient_GetDeviceMetadata_IDAndRefID_Correlation(t *testing.T) {
	fake := newFakeBroker("controller-app")
	rfid, err := New(fake, WithDefaultTarget("reader-01"))
	if err != nil {
		t.Fatalf("failed to create RFIDClient: %v", err)
	}

	fake.requestFunc = func(ctx context.Context, reqEvt *event.RawEvent) (*event.RawEvent, error) {
		if reqEvt.ID == "" {
			t.Errorf("expected reqEvt.ID to be non-empty")
		}
		respData := DeviceMetadataResponseData{
			Status:   "ok",
			DeviceID: "DEV-FEIG-01",
		}
		dataBytes, _ := json.Marshal(respData)
		respEvt := &event.RawEvent{
			BaseEvent: event.BaseEvent{
				ID:      "meta-resp-id-777",
				RefID:   reqEvt.ID,
				Source:  "reader-01",
				Target:  "controller-app",
				Session: reqEvt.Session,
				Type:    EventDeviceMetadataResponse,
			},
			Data: dataBytes,
		}
		return respEvt, nil
	}

	resp, err := rfid.GetDeviceMetadata(context.Background(), "reader-01")
	if err != nil {
		t.Fatalf("GetDeviceMetadata failed: %v", err)
	}
	if resp.RefID == "" {
		t.Error("expected non-empty resp.RefID")
	}
	if resp.ID != "meta-resp-id-777" {
		t.Errorf("resp.ID = %q, want meta-resp-id-777", resp.ID)
	}
}

func TestRFIDClient_GetDeviceMetadataMulti(t *testing.T) {
	fake := newFakeBroker("controller-app")
	fake.requestNFunc = func(ctx context.Context, evt *event.RawEvent, numResults int, ch chan *event.RawEvent) error {
		resp1, _ := event.NewReplyEvent(evt, EventDeviceMetadataResponse, map[string]any{
			"readerId": "reader-1",
			"device":   "Simulated Reader 1",
		})
		resp2, _ := event.NewReplyEvent(evt, EventDeviceMetadataResponse, map[string]any{
			"readerId": "reader-2",
			"device":   "Simulated Reader 2",
		})
		ch <- resp1
		ch <- resp2
		close(ch)
		return nil
	}

	rfid, err := New(fake)
	if err != nil {
		t.Fatalf("failed to create RFIDClient: %v", err)
	}

	res, err := rfid.GetDeviceMetadataMulti(context.Background(), "all", 2)
	if err != nil {
		t.Fatalf("GetDeviceMetadataMulti failed: %v", err)
	}
	if len(res) != 2 {
		t.Fatalf("expected 2 responses, got %d", len(res))
	}
	if res[0].Data.ReaderID != "reader-1" || res[1].Data.ReaderID != "reader-2" {
		t.Errorf("unexpected reader IDs: %s, %s", res[0].Data.ReaderID, res[1].Data.ReaderID)
	}

	// Test scoped view
	scoped := rfid.ForReader("reader-1")
	scopedRes, err := scoped.GetDeviceMetadataMulti(context.Background(), "all", 2)
	if err != nil {
		t.Fatalf("scoped.GetDeviceMetadataMulti failed: %v", err)
	}
	if len(scopedRes) != 2 {
		t.Fatalf("expected 2 responses, got %d", len(scopedRes))
	}
}
