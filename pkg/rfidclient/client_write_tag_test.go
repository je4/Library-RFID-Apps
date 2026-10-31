package rfidclient

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/je4/wsbrokerclient/pkg/event"
)

func TestRFIDClient_WriteTag(t *testing.T) {
	fake := newFakeBroker("controller-app")
	fake.requestFunc = func(ctx context.Context, evt *event.RawEvent) (*event.RawEvent, error) {
		respData := WriteTagResponseData{
			Status:    "ok",
			UID:       "E004015099887766",
			ItemID:    "3012345678",
			ISIL:      "CH-000001-1",
			Country:   "CH",
			Parts:     1,
			PartNo:    1,
			UsageType: 1,
			AFI:       "07",
			Message:   "Tag written successfully",
		}
		raw, _ := json.Marshal(respData)
		return &event.RawEvent{
			BaseEvent: event.BaseEvent{
				Type:    EventWriteTagResponse,
				Source:  evt.Target,
				Target:  evt.Source,
				Session: evt.Session,
			},
			Data: raw,
		}, nil
	}

	rfid, err := New(fake, WithDefaultTarget("rfid-reader-station-01"))
	if err != nil {
		t.Fatalf("failed to create RFIDClient: %v", err)
	}

	req := NewWriteTagRequest("3012345678", "E004015099887766")
	req.ISIL = "CH-000001-1"
	req.Country = "CH"
	req.AFI = "07"

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	resp, err := rfid.WriteTag(ctx, "", req)
	if err != nil {
		t.Fatalf("WriteTag failed: %v", err)
	}

	if !resp.Data.IsSuccess() {
		t.Errorf("expected IsSuccess=true")
	}
	if resp.Data.ItemID != "3012345678" {
		t.Errorf("ItemID = %q, want '3012345678'", resp.Data.ItemID)
	}
	if resp.Data.AFI != "07" {
		t.Errorf("AFI = %q, want '07'", resp.Data.AFI)
	}
}

func TestRFIDClient_WriteTag_IDAndRefID_Correlation(t *testing.T) {
	fake := newFakeBroker("controller-app")
	rfid, err := New(fake, WithDefaultTarget("reader-01"))
	if err != nil {
		t.Fatalf("failed to create RFIDClient: %v", err)
	}

	fake.requestFunc = func(ctx context.Context, reqEvt *event.RawEvent) (*event.RawEvent, error) {
		if reqEvt.ID == "" {
			t.Errorf("expected reqEvt.ID to be non-empty")
		}
		// Create correlated response with RefID = reqEvt.ID
		respData := WriteTagResponseData{
			Status: "ok",
			UID:    "E004010000001234",
			ItemID: "ITEM-WRITE-TEST",
		}
		dataBytes, _ := json.Marshal(respData)
		respEvt := &event.RawEvent{
			BaseEvent: event.BaseEvent{
				ID:      "resp-evt-id-999",
				RefID:   reqEvt.ID,
				Source:  "reader-01",
				Target:  "controller-app",
				Session: reqEvt.Session,
				Type:    EventWriteTagResponse,
			},
			Data: dataBytes,
		}
		return respEvt, nil
	}

	req := NewWriteTagRequest("ITEM-WRITE-TEST", "E004010000001234")
	resp, err := rfid.WriteTag(context.Background(), "reader-01", req)
	if err != nil {
		t.Fatalf("WriteTag failed: %v", err)
	}
	if resp.RefID == "" {
		t.Error("expected non-empty resp.RefID")
	}
	if resp.ID != "resp-evt-id-999" {
		t.Errorf("resp.ID = %q, want resp-evt-id-999", resp.ID)
	}
}

func TestRFIDClient_WriteTagBatch_And_WriteTags(t *testing.T) {
	fake := newFakeBroker("controller-app")
	rfid, err := New(fake, WithDefaultTarget("reader-01"))
	if err != nil {
		t.Fatalf("failed to create RFIDClient: %v", err)
	}

	fake.requestFunc = func(ctx context.Context, reqEvt *event.RawEvent) (*event.RawEvent, error) {
		var req WriteTagRequest
		if err := json.Unmarshal(reqEvt.Data, &req); err != nil {
			t.Fatalf("failed to unmarshal write tag batch request: %v", err)
		}
		if len(req.Tags) != 2 {
			t.Errorf("expected 2 tags in request, got %d", len(req.Tags))
		}

		respData := WriteTagResponseData{
			Status:         "ok",
			SuccessfulUIDs: []string{"TAG-1", "TAG-2"},
			Results: []TagWriteResult{
				{UID: "TAG-1", ItemID: "ITEM-1", Status: "ok"},
				{UID: "TAG-2", ItemID: "ITEM-2", Status: "ok"},
			},
			Message: "2 tags written successfully",
		}
		dataBytes, _ := json.Marshal(respData)
		respEvt := &event.RawEvent{
			BaseEvent: event.BaseEvent{
				ID:      "resp-tag-batch-001",
				RefID:   reqEvt.ID,
				Source:  "reader-01",
				Target:  "controller-app",
				Session: reqEvt.Session,
				Type:    EventWriteTagResponse,
			},
			Data: dataBytes,
		}
		return respEvt, nil
	}

	items := []TagWriteItem{
		{UID: "TAG-1", ItemID: "ITEM-1", Parts: 2, PartNo: 1, AFI: "07"},
		{UID: "TAG-2", ItemID: "ITEM-2", Parts: 2, PartNo: 2, AFI: "07"},
	}

	// 1. WriteTagBatch
	resp, err := rfid.WriteTagBatch(context.Background(), "reader-01", items)
	if err != nil {
		t.Fatalf("WriteTagBatch failed: %v", err)
	}
	if !resp.Data.IsSuccess() {
		t.Errorf("expected WriteTagBatch success, got %s", resp.Data.Status)
	}
	if !resp.Data.IsTagSuccessful("TAG-1") || !resp.Data.IsTagSuccessful("TAG-2") {
		t.Errorf("expected TAG-1 and TAG-2 to be successful")
	}

	// 2. WriteTags (variadic)
	respVariadic, err := rfid.WriteTags(context.Background(), "reader-01", items[0], items[1])
	if err != nil {
		t.Fatalf("WriteTags failed: %v", err)
	}
	if !respVariadic.Data.IsSuccess() {
		t.Errorf("expected WriteTags success, got %s", respVariadic.Data.Status)
	}

	// 3. Empty batch error validation
	if _, err := rfid.WriteTagBatch(context.Background(), "reader-01", nil); err == nil {
		t.Error("expected error for empty tags list in WriteTagBatch")
	}
}

func TestRFIDClient_SendWriteTagBatch_And_BroadcastWriteTagBatch(t *testing.T) {
	fake := newFakeBroker("controller-app")
	rfid, err := New(fake, WithDefaultTarget("reader-01"), WithChannels("chan1"))
	if err != nil {
		t.Fatalf("failed to create RFIDClient: %v", err)
	}

	items := []TagWriteItem{
		{UID: "TAG-A", ItemID: "ITEM-A"},
		{UID: "TAG-B", ItemID: "ITEM-B"},
	}

	// 1. SendWriteTagBatch
	if err := rfid.SendWriteTagBatch(context.Background(), "reader-01", items); err != nil {
		t.Fatalf("SendWriteTagBatch failed: %v", err)
	}
	fake.mu.Lock()
	sentEvt := fake.lastSent
	fake.mu.Unlock()
	if sentEvt == nil || sentEvt.Target != "reader-01" {
		t.Errorf("expected sent event to target 'reader-01', got: %+v", sentEvt)
	}

	// 2. BroadcastWriteTagBatch
	if err := rfid.BroadcastWriteTagBatch(context.Background(), "chan1", items); err != nil {
		t.Fatalf("BroadcastWriteTagBatch failed: %v", err)
	}
	fake.mu.Lock()
	bcastEvt := fake.lastSent
	fake.mu.Unlock()
	if bcastEvt == nil || bcastEvt.Channel != "chan1" {
		t.Errorf("expected broadcast event to channel 'chan1', got: %+v", bcastEvt)
	}

	// 3. Empty batch error validation
	if err := rfid.SendWriteTagBatch(context.Background(), "reader-01", nil); err == nil {
		t.Error("expected error for empty SendWriteTagBatch")
	}
	if err := rfid.BroadcastWriteTagBatch(context.Background(), "chan1", nil); err == nil {
		t.Error("expected error for empty BroadcastWriteTagBatch")
	}
}

func TestRFIDClient_WriteTag_TimeoutError(t *testing.T) {
	fake := newFakeBroker("controller-app")
	fake.requestFunc = func(ctx context.Context, evt *event.RawEvent) (*event.RawEvent, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}

	rfid, err := New(fake, WithDefaultTimeout(50*time.Millisecond))
	if err != nil {
		t.Fatalf("failed to create RFIDClient: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	req := NewWriteTagRequest("3012345678", "E00401509200B02A")
	req.ReaderID = "gate-01"
	_, err = rfid.WriteTag(ctx, "daemon-01", req)
	if err == nil {
		t.Fatal("expected error on timeout, got nil")
	}
	if !strings.Contains(err.Error(), "timed out waiting for response from reader hardware") {
		t.Errorf("expected timeout message in error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "daemon-01") || !strings.Contains(err.Error(), "gate-01") {
		t.Errorf("expected target and reader in error message, got: %v", err)
	}
}

func TestRFIDClient_WriteTagBatchStream(t *testing.T) {
	fake := newFakeBroker("controller-app")
	fake.requestNFunc = func(ctx context.Context, evt *event.RawEvent, numResults int, ch chan *event.RawEvent) error {
		resp1, _ := event.NewReplyEvent(evt, EventWriteTagResponse, map[string]any{
			"status":  "ok",
			"uid":     "UID-1",
			"itemId":  "ITEM-1",
			"message": "written 1",
		})
		resp2, _ := event.NewReplyEvent(evt, EventWriteTagResponse, map[string]any{
			"status":  "ok",
			"uid":     "UID-2",
			"itemId":  "ITEM-2",
			"message": "written 2",
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

	tags := []TagWriteItem{
		{UID: "UID-1", ItemID: "ITEM-1"},
		{UID: "UID-2", ItemID: "ITEM-2"},
	}
	ch, err := rfid.WriteTagBatchStream(context.Background(), "daemon-01", tags, 2)
	if err != nil {
		t.Fatalf("WriteTagBatchStream failed: %v", err)
	}

	var results []*WriteTagResponse
	for r := range ch {
		results = append(results, r)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 stream results, got %d", len(results))
	}
	if results[0].Data.UID != "UID-1" || results[1].Data.UID != "UID-2" {
		t.Errorf("unexpected stream items: %+v, %+v", results[0], results[1])
	}

	// Test scoped view
	scoped := rfid.ForReader("gate-01")
	fake.requestNFunc = func(ctx context.Context, evt *event.RawEvent, numResults int, ch chan *event.RawEvent) error {
		resp, _ := event.NewReplyEvent(evt, EventWriteTagResponse, map[string]any{
			"status":   "ok",
			"uid":      "UID-1",
			"readerId": "gate-01",
		})
		ch <- resp
		close(ch)
		return nil
	}
	scopedCh, err := scoped.WriteTagBatchStream(context.Background(), "daemon-01", tags, 1)
	if err != nil {
		t.Fatalf("scoped.WriteTagBatchStream failed: %v", err)
	}
	var scopedResults []*WriteTagResponse
	for r := range scopedCh {
		scopedResults = append(scopedResults, r)
	}
	if len(scopedResults) != 1 {
		t.Fatalf("expected 1 scoped result, got %d", len(scopedResults))
	}
}
