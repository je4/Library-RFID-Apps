package rfidclient

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/je4/wsbrokerclient/pkg/event"
)

func TestRFIDClient_WriteAFI(t *testing.T) {
	fake := newFakeBroker("controller-app")
	fake.requestFunc = func(ctx context.Context, evt *event.RawEvent) (*event.RawEvent, error) {
		respData := WriteAFIResponseData{
			Status:  "ok",
			UID:     "E004015099887766",
			AFI:     "C2",
			Message: "AFI written successfully",
		}
		raw, _ := json.Marshal(respData)
		return &event.RawEvent{
			BaseEvent: event.BaseEvent{
				Type:    EventWriteAFIResponse,
				Source:  evt.Target,
				Target:  evt.Source,
				Session: evt.Session,
			},
			Data: raw,
		}, nil
	}

	rfid, err := New(fake)
	if err != nil {
		t.Fatalf("failed to create RFIDClient: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	resp, err := rfid.WriteAFIValue(ctx, "rfid-reader-station-01", "E004015099887766", "C2")
	if err != nil {
		t.Fatalf("WriteAFIValue failed: %v", err)
	}

	if !resp.Data.IsSuccess() {
		t.Errorf("expected IsSuccess=true")
	}
	if resp.Data.AFI != "C2" {
		t.Errorf("AFI = %q, want 'C2'", resp.Data.AFI)
	}
}

func TestRFIDClient_AFIConvenienceMethods(t *testing.T) {
	fake := newFakeBroker("controller-app")
	var lastReq WriteAFIRequest
	var lastTarget string

	fake.requestFunc = func(ctx context.Context, evt *event.RawEvent) (*event.RawEvent, error) {
		lastTarget = evt.Target
		var req WriteAFIRequest
		if err := json.Unmarshal(evt.Data, &req); err != nil {
			return nil, err
		}
		lastReq = req

		respData := WriteAFIResponseData{
			Status: "ok",
			UID:    req.UID,
			AFI:    req.AFI,
		}
		raw, _ := json.Marshal(respData)
		return &event.RawEvent{
			BaseEvent: event.BaseEvent{
				Type:    EventWriteAFIResponse,
				Source:  evt.Target,
				Target:  evt.Source,
				Session: evt.Session,
			},
			Data: raw,
		}, nil
	}

	rfid, err := New(fake, WithDefaultTarget("default-station"))
	if err != nil {
		t.Fatalf("failed to create RFIDClient: %v", err)
	}

	ctx := context.Background()

	// 1. SecureTag
	resp, err := rfid.SecureTag(ctx, "station-01", "E00401")
	if err != nil {
		t.Fatalf("SecureTag failed: %v", err)
	}
	if lastTarget != "station-01" || lastReq.UID != "E00401" || lastReq.AFI != AFISecured {
		t.Errorf("SecureTag sent unexpected request: target=%s, req=%+v", lastTarget, lastReq)
	}
	if !resp.Data.IsSecured() || resp.Data.SecurityStatus() != "Gesichert" {
		t.Errorf("expected resp.Data.IsSecured()=true, got %+v", resp)
	}

	// 2. UnsecureTag
	resp, err = rfid.UnsecureTag(ctx, "station-02", "E00402")
	if err != nil {
		t.Fatalf("UnsecureTag failed: %v", err)
	}
	if lastTarget != "station-02" || lastReq.UID != "E00402" || lastReq.AFI != AFIUnsecured {
		t.Errorf("UnsecureTag sent unexpected request: target=%s, req=%+v", lastTarget, lastReq)
	}
	if !resp.Data.IsUnsecured() || resp.Data.SecurityStatus() != "Entsichert" {
		t.Errorf("expected resp.Data.IsUnsecured()=true, got %+v", resp)
	}

	// 3. SetAFISecured with default target
	resp, err = rfid.SetAFISecured(ctx, "", "E00403")
	if err != nil {
		t.Fatalf("SetAFISecured failed: %v", err)
	}
	if lastTarget != "default-station" || lastReq.UID != "E00403" || lastReq.AFI != AFISecured {
		t.Errorf("SetAFISecured sent unexpected request: target=%s, req=%+v", lastTarget, lastReq)
	}

	// 4. SetAFIUnsecured without UID
	resp, err = rfid.SetAFIUnsecured(ctx, "station-03")
	if err != nil {
		t.Fatalf("SetAFIUnsecured failed: %v", err)
	}
	if lastTarget != "station-03" || lastReq.UID != "" || lastReq.AFI != AFIUnsecured {
		t.Errorf("SetAFIUnsecured sent unexpected request: target=%s, req=%+v", lastTarget, lastReq)
	}
}

func TestRFIDClient_SendAFI(t *testing.T) {
	fake := newFakeBroker("controller-app")
	rfid, err := New(fake, WithDefaultTarget("default-reader"))
	if err != nil {
		t.Fatalf("failed to create RFIDClient: %v", err)
	}

	ctx := context.Background()

	// 1. Send with explicit target
	req := NewWriteAFIRequest(AFISecured, "E004015099887766")
	if err := rfid.SendAFI(ctx, "rfid:reader:01", req); err != nil {
		t.Fatalf("SendAFI failed: %v", err)
	}

	fake.mu.Lock()
	lastSent := fake.lastSent
	fake.mu.Unlock()

	if lastSent == nil {
		t.Fatal("expected message to be sent")
	}
	if lastSent.Target != "rfid:reader:01" {
		t.Errorf("expected target 'rfid:reader:01', got %s", lastSent.Target)
	}

	var parsedReq WriteAFIRequest
	if err := json.Unmarshal(lastSent.Data, &parsedReq); err != nil {
		t.Fatalf("failed to unmarshal sent data: %v", err)
	}
	if parsedReq.AFI != AFISecured || parsedReq.UID != "E004015099887766" {
		t.Errorf("unexpected payload: %+v", parsedReq)
	}

	// 2. Send with default target fallback
	req2 := NewWriteAFIRequest(AFIUnsecured)
	if err := rfid.SendAFI(ctx, "", req2); err != nil {
		t.Fatalf("SendAFI with default target failed: %v", err)
	}

	fake.mu.Lock()
	lastSent2 := fake.lastSent
	fake.mu.Unlock()

	if lastSent2.Target != "default-reader" {
		t.Errorf("expected target 'default-reader', got %s", lastSent2.Target)
	}

	// 3. Validation error on empty AFI
	invalidReq := WriteAFIRequest{AFI: ""}
	if err := rfid.SendAFI(ctx, "rfid:reader:01", invalidReq); err == nil {
		t.Error("expected error for empty AFI")
	}
}

func TestRFIDClient_BroadcastAFI(t *testing.T) {
	fake := newFakeBroker("controller-app")
	rfid, err := New(fake, WithChannels("channel1", "channel2"))
	if err != nil {
		t.Fatalf("failed to create RFIDClient: %v", err)
	}

	ctx := context.Background()

	// 1. Broadcast to specific channel
	req := NewWriteAFIRequest(AFISecured, "E004015099887766")
	if err := rfid.BroadcastAFI(ctx, "custom-channel", req); err != nil {
		t.Fatalf("BroadcastAFI failed: %v", err)
	}

	fake.mu.Lock()
	lastSent := fake.lastSent
	fake.mu.Unlock()

	if lastSent == nil {
		t.Fatal("expected message to be broadcast")
	}
	if lastSent.Target != "custom-channel" {
		t.Errorf("expected target 'custom-channel', got %s", lastSent.Target)
	}

	// 2. Broadcast without channel uses configured channels
	if err := rfid.BroadcastAFI(ctx, "", req); err != nil {
		t.Fatalf("BroadcastAFI to configured channels failed: %v", err)
	}

	// 3. Validation error on empty AFI
	invalidReq := WriteAFIRequest{AFI: ""}
	if err := rfid.BroadcastAFI(ctx, "custom-channel", invalidReq); err == nil {
		t.Error("expected error for empty AFI")
	}
}

func TestRFIDClient_WriteAFI_IDAndRefID_Correlation(t *testing.T) {
	fake := newFakeBroker("controller-app")
	rfid, err := New(fake, WithDefaultTarget("reader-01"))
	if err != nil {
		t.Fatalf("failed to create RFIDClient: %v", err)
	}

	fake.requestFunc = func(ctx context.Context, reqEvt *event.RawEvent) (*event.RawEvent, error) {
		if reqEvt.ID == "" {
			t.Errorf("expected reqEvt.ID to be non-empty")
		}
		respData := WriteAFIResponseData{
			Status: "ok",
			UID:    "E004010000001234",
			AFI:    "07",
		}
		dataBytes, _ := json.Marshal(respData)
		respEvt := &event.RawEvent{
			BaseEvent: event.BaseEvent{
				ID:      "resp-afi-id-888",
				RefID:   reqEvt.ID,
				Source:  "reader-01",
				Target:  "controller-app",
				Session: reqEvt.Session,
				Type:    EventWriteAFIResponse,
			},
			Data: dataBytes,
		}
		return respEvt, nil
	}

	req := NewWriteAFIRequest("07", "E004010000001234")
	resp, err := rfid.WriteAFI(context.Background(), "reader-01", req)
	if err != nil {
		t.Fatalf("WriteAFI failed: %v", err)
	}
	if resp.RefID == "" {
		t.Error("expected non-empty resp.RefID")
	}
	if resp.ID != "resp-afi-id-888" {
		t.Errorf("resp.ID = %q, want resp-afi-id-888", resp.ID)
	}
}

func TestRFIDClient_SendAFI_And_BroadcastAFI_IDPropagation(t *testing.T) {
	fake := newFakeBroker("controller-app")
	rfid, err := New(fake, WithDefaultTarget("reader-01"), WithChannels("chan1"))
	if err != nil {
		t.Fatalf("failed to create RFIDClient: %v", err)
	}

	// 1. SendAFI
	reqSend := NewWriteAFIRequest("07", "E00401")
	if err := rfid.SendAFI(context.Background(), "reader-01", reqSend); err != nil {
		t.Fatalf("SendAFI failed: %v", err)
	}
	fake.mu.Lock()
	sentEvt := fake.lastSent
	fake.mu.Unlock()
	if sentEvt.ID == "" {
		t.Errorf("expected sentEvt.ID to be non-empty")
	}

	// 2. BroadcastAFI
	reqBcast := NewWriteAFIRequest("C2", "E00402")
	if err := rfid.BroadcastAFI(context.Background(), "chan1", reqBcast); err != nil {
		t.Fatalf("BroadcastAFI failed: %v", err)
	}
	fake.mu.Lock()
	bcastEvt := fake.lastSent
	fake.mu.Unlock()
	if bcastEvt.ID == "" {
		t.Errorf("expected bcastEvt.ID to be non-empty")
	}
}

func TestRFIDClient_WriteAFIBatch(t *testing.T) {
	fake := newFakeBroker("controller-app")
	rfid, err := New(fake, WithDefaultTarget("reader-01"))
	if err != nil {
		t.Fatalf("failed to create RFIDClient: %v", err)
	}

	fake.requestFunc = func(ctx context.Context, reqEvt *event.RawEvent) (*event.RawEvent, error) {
		var req WriteAFIRequest
		if err := json.Unmarshal(reqEvt.Data, &req); err != nil {
			t.Fatalf("failed to unmarshal request: %v", err)
		}
		if len(req.UIDs) != 2 || req.UIDs[0] != "TAG-1" || req.UIDs[1] != "TAG-2" {
			t.Errorf("unexpected request UIDs: %v", req.UIDs)
		}

		respData := WriteAFIResponseData{
			Status:         "ok",
			UIDs:           req.UIDs,
			SuccessfulUIDs: req.UIDs,
			Results: []TagAFIResult{
				{UID: "TAG-1", Status: "ok"},
				{UID: "TAG-2", Status: "ok"},
			},
			AFI: req.AFI,
		}
		dataBytes, _ := json.Marshal(respData)
		respEvt := &event.RawEvent{
			BaseEvent: event.BaseEvent{
				ID:      "resp-batch-001",
				RefID:   reqEvt.ID,
				Source:  "reader-01",
				Target:  "controller-app",
				Session: reqEvt.Session,
				Type:    EventWriteAFIResponse,
			},
			Data: dataBytes,
		}
		return respEvt, nil
	}

	uids := []string{"TAG-1", "TAG-2"}
	resp, err := rfid.WriteAFIBatch(context.Background(), "reader-01", uids, "C2")
	if err != nil {
		t.Fatalf("WriteAFIBatch failed: %v", err)
	}
	if !resp.Data.IsSuccess() {
		t.Errorf("expected success response, got %s", resp.Data.Status)
	}
	if len(resp.Data.SuccessfulUIDs) != 2 {
		t.Errorf("expected 2 successful UIDs, got %d", len(resp.Data.SuccessfulUIDs))
	}
	if !resp.Data.IsTagSuccessful("TAG-1") || !resp.Data.IsTagSuccessful("TAG-2") {
		t.Errorf("expected TAG-1 and TAG-2 to be successful")
	}

	// Test SetAFISecured with multiple UIDs (batch)
	respSecured, err := rfid.SetAFISecured(context.Background(), "reader-01", "TAG-1", "TAG-2")
	if err != nil {
		t.Fatalf("SetAFISecured batch failed: %v", err)
	}
	if !respSecured.Data.IsSuccess() {
		t.Errorf("expected success response from SetAFISecured batch, got %s", respSecured.Data.Status)
	}
}

func TestRFIDClient_WriteAFI_TimeoutError(t *testing.T) {
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

	req := NewWriteAFIBatchRequest("07", []string{"UID-1", "UID-2"})
	req.ReaderID = "gate-01"
	_, err = rfid.WriteAFI(ctx, "daemon-01", req)
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

func TestRFIDClient_WriteAFIBatchStream(t *testing.T) {
	fake := newFakeBroker("controller-app")
	fake.requestNFunc = func(ctx context.Context, evt *event.RawEvent, numResults int, ch chan *event.RawEvent) error {
		resp1, _ := event.NewReplyEvent(evt, EventWriteAFIResponse, map[string]any{
			"status":  "ok",
			"uid":     "UID-1",
			"afi":     "07",
			"message": "secured 1",
		})
		resp2, _ := event.NewReplyEvent(evt, EventWriteAFIResponse, map[string]any{
			"status":  "ok",
			"uid":     "UID-2",
			"afi":     "07",
			"message": "secured 2",
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

	req := NewWriteAFIBatchRequest("07", []string{"UID-1", "UID-2"})
	ch, err := rfid.WriteAFIBatchStream(context.Background(), "daemon-01", req, 2)
	if err != nil {
		t.Fatalf("WriteAFIBatchStream failed: %v", err)
	}

	var results []*WriteAFIResponse
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
		resp, _ := event.NewReplyEvent(evt, EventWriteAFIResponse, map[string]any{
			"status":   "ok",
			"uid":      "UID-1",
			"readerId": "gate-01",
		})
		ch <- resp
		close(ch)
		return nil
	}
	scopedCh, err := scoped.WriteAFIBatchStream(context.Background(), "daemon-01", req, 1)
	if err != nil {
		t.Fatalf("scoped.WriteAFIBatchStream failed: %v", err)
	}
	var scopedResults []*WriteAFIResponse
	for r := range scopedCh {
		scopedResults = append(scopedResults, r)
	}
	if len(scopedResults) != 1 {
		t.Fatalf("expected 1 scoped result, got %d", len(scopedResults))
	}
}
