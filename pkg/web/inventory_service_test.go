package web

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/je4/Library-RFID-Apps/pkg/desk"
	"github.com/je4/Library-RFID-Apps/pkg/inventory"
	"github.com/je4/Library-RFID-Apps/pkg/libraryclient"
	"github.com/je4/utils/v2/pkg/config"
)

type mockDispatcher struct {
	mu         sync.Mutex
	dispatched []inventory.ScanPayload
	response   *inventory.DispatchResponse
	err        error
}

func (m *mockDispatcher) Dispatch(ctx context.Context, payload inventory.ScanPayload) (*inventory.DispatchResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.dispatched = append(m.dispatched, payload)
	if m.response != nil {
		return m.response, m.err
	}
	return &inventory.DispatchResponse{StatusCode: 200, Success: true}, m.err
}

func (m *mockDispatcher) InterpolateURL(targetTemplate string, payload inventory.ScanPayload) string {
	return targetTemplate
}

func (m *mockDispatcher) GenerateJWT(payload inventory.ScanPayload) (string, error) {
	return "test.jwt.token", nil
}

func (m *mockDispatcher) getDispatchedCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.dispatched)
}

func TestInventoryService_SessionLifecycle(t *testing.T) {
	cfg := desk.InventoryConfig{
		Enabled: true,
		Webservice: desk.WebserviceConfig{
			TargetURL: "https://example.com/inv",
		},
	}
	disp := &mockDispatcher{}
	svc := NewInventoryService(cfg, "desk-01", nil, disp, nil)

	// 1. Initial state
	sess := svc.GetSession()
	if sess.Active {
		t.Error("expected inactive session initially")
	}

	// Starting with empty marker should not activate session
	sessEmpty := svc.StartSession("")
	if sessEmpty.Active {
		t.Error("expected session to remain inactive when marker is empty")
	}
	sessWhitespace := svc.StartSession("   ")
	if sessWhitespace.Active {
		t.Error("expected session to remain inactive when marker is whitespace")
	}

	// 2. Start session
	sess = svc.StartSession("SHELF-A1")
	if !sess.Active || sess.Paused {
		t.Errorf("expected active, unpaused session: %+v", sess)
	}
	if sess.Marker != "SHELF-A1" {
		t.Errorf("expected marker SHELF-A1, got %s", sess.Marker)
	}

	// 3. Pause session
	sess = svc.PauseSession()
	if !sess.Paused {
		t.Error("expected session to be paused")
	}

	// 4. Resume session
	sess = svc.ResumeSession()
	if sess.Paused {
		t.Error("expected session to be resumed (unpaused)")
	}

	// 5. Update Marker
	svc.SetMarker("SHELF-B2")
	sess = svc.GetSession()
	if sess.Marker != "SHELF-B2" {
		t.Errorf("expected marker SHELF-B2, got %s", sess.Marker)
	}

	// 6. Reset session
	sess = svc.ResetSession()
	if sess.Active {
		t.Error("expected inactive session after reset")
	}
}

func TestInventoryService_ProcessTagRead_DeduplicationAndStats(t *testing.T) {
	cfg := desk.InventoryConfig{
		Enabled: true,
		Webservice: desk.WebserviceConfig{
			TargetURL: "https://example.com/inv",
		},
	}
	disp := &mockDispatcher{}
	svc := NewInventoryService(cfg, "desk-01", nil, disp, nil)

	// Ignore scans when session is not started
	tag1 := &libraryclient.TagReadEvent{
		Data: libraryclient.TagReadData{
			UID:    "E004015011111111",
			ItemID: "BARCODE-1",
			CRCOk:  true,
		},
	}
	item, cue, ok := svc.ProcessTagRead(tag1)
	if ok || item != nil {
		t.Error("expected scan to be ignored when session is inactive")
	}

	// Start session
	svc.StartSession("SHELF-01")

	// First scan (valid unique item)
	item, cue, ok = svc.ProcessTagRead(tag1)
	if !ok || item == nil {
		t.Fatalf("expected successful tag processing, got ok=%v", ok)
	}
	if cue != AudioCueSuccess {
		t.Errorf("expected AudioCueSuccess, got %s", cue)
	}
	if item.IsDuplicate {
		t.Error("expected item not to be duplicate")
	}

	// Second scan of same UID (duplicate)
	item2, cue2, ok2 := svc.ProcessTagRead(tag1)
	if !ok2 || item2 == nil {
		t.Fatalf("expected duplicate tag processing, got ok=%v", ok2)
	}
	if cue2 != AudioCueDuplicate {
		t.Errorf("expected AudioCueDuplicate, got %s", cue2)
	}
	if !item2.IsDuplicate {
		t.Error("expected item to be marked as duplicate")
	}

	// Check stats
	sess := svc.GetSession()
	if sess.Stats.TotalScans != 2 {
		t.Errorf("expected TotalScans 2, got %d", sess.Stats.TotalScans)
	}
	if sess.Stats.UniqueItems != 1 {
		t.Errorf("expected UniqueItems 1, got %d", sess.Stats.UniqueItems)
	}
	if sess.Stats.DuplicateTags != 1 {
		t.Errorf("expected DuplicateTags 1, got %d", sess.Stats.DuplicateTags)
	}

	// Give async dispatcher time to run
	time.Sleep(50 * time.Millisecond)
	if disp.getDispatchedCount() != 1 {
		t.Errorf("expected exactly 1 dispatch call (duplicates skipped), got %d", disp.getDispatchedCount())
	}
}

func TestInventoryService_ProcessTagRead_ErrorTag(t *testing.T) {
	cfg := desk.InventoryConfig{
		Enabled: true,
		Webservice: desk.WebserviceConfig{
			TargetURL: "https://example.com/inv",
		},
	}
	disp := &mockDispatcher{}
	svc := NewInventoryService(cfg, "", nil, disp, nil)
	svc.StartSession("SHELF-ERR")

	// Tag with CRC Error
	tagErr := &libraryclient.TagReadEvent{
		Data: libraryclient.TagReadData{
			UID:    "E004015099999999",
			ItemID: "BARCODE-ERR",
			CRCOk:  false,
		},
	}

	item, cue, ok := svc.ProcessTagRead(tagErr)
	if !ok || item == nil {
		t.Fatalf("expected tag to be processed, got ok=%v", ok)
	}
	if cue != AudioCueError {
		t.Errorf("expected AudioCueError for CRC error tag, got %s", cue)
	}

	sess := svc.GetSession()
	if sess.Stats.ErrorTags != 1 {
		t.Errorf("expected ErrorTags 1, got %d", sess.Stats.ErrorTags)
	}
}

func TestInventoryService_MarkerValidation(t *testing.T) {
	cfg := desk.InventoryConfig{
		Enabled: true,
		Webservice: desk.WebserviceConfig{
			TargetURL: "https://example.com/inv",
		},
	}
	disp := &mockDispatcher{}
	svc := NewInventoryService(cfg, "", nil, disp, nil)

	// Inactive by default
	if svc.GetSession().Active {
		t.Error("expected inactive initially")
	}

	// Empty marker must not start session
	sess := svc.StartSession("")
	if sess.Active {
		t.Error("session should not start with empty marker")
	}

	// Whitespace marker must not start session
	sess = svc.StartSession("   ")
	if sess.Active {
		t.Error("session should not start with whitespace marker")
	}

	// Continuous marker updates during session
	sess = svc.StartSession("MARKER-1")
	if !sess.Active || sess.Marker != "MARKER-1" {
		t.Fatalf("expected active session with MARKER-1, got %+v", sess)
	}

	svc.SetMarker("MARKER-2")
	if svc.GetSession().Marker != "MARKER-2" {
		t.Errorf("expected marker to update to MARKER-2, got %s", svc.GetSession().Marker)
	}
}

func TestInventoryService_DispatchStateTransitionsAndRetry(t *testing.T) {
	cfg := desk.InventoryConfig{
		Enabled: true,
		Webservice: desk.WebserviceConfig{
			TargetURL: "https://example.com/inv",
		},
	}
	disp := &mockDispatcher{}
	svc := NewInventoryService(cfg, "desk-01", nil, disp, nil)

	var callbackEvents []InventoryTagEventPayload
	var cbMu sync.Mutex
	svc.SetOnTagEvent(func(payload InventoryTagEventPayload, channel string) {
		cbMu.Lock()
		defer cbMu.Unlock()
		callbackEvents = append(callbackEvents, payload)
	})

	svc.StartSession("SHELF-DISPATCH")

	tag := &libraryclient.TagReadEvent{
		Channel: "ch1",
		Data: libraryclient.TagReadData{
			UID:    "E004015011112222",
			ItemID: "BARCODE-DISP-01",
			CRCOk:  true,
		},
	}

	item, _, ok := svc.ProcessTagRead(tag)
	if !ok || item == nil {
		t.Fatalf("expected successful tag processing")
	}

	// Initially pending
	if item.DispatchState != "pending" && item.DispatchState != "success" {
		t.Errorf("expected pending or success, got %s", item.DispatchState)
	}

	// Wait for async dispatch to complete
	time.Sleep(50 * time.Millisecond)

	if item.DispatchState != "success" {
		t.Errorf("expected dispatchState success after completion, got %s", item.DispatchState)
	}

	cbMu.Lock()
	numEvents := len(callbackEvents)
	cbMu.Unlock()
	if numEvents < 2 {
		t.Errorf("expected at least 2 callback events (initial pending + completion), got %d", numEvents)
	}

	// Test RetryDispatch
	disp.err = nil
	disp.response = &inventory.DispatchResponse{StatusCode: 200, Success: true}
	retriedItem, err := svc.RetryDispatch("E004015011112222")
	if err != nil {
		t.Fatalf("expected RetryDispatch to succeed, got error: %v", err)
	}
	if retriedItem.DispatchState != "pending" && retriedItem.DispatchState != "success" {
		t.Errorf("expected pending or success on retry, got %s", retriedItem.DispatchState)
	}

	time.Sleep(50 * time.Millisecond)
	if retriedItem.DispatchState != "success" {
		t.Errorf("expected success after retry dispatch, got %s", retriedItem.DispatchState)
	}
}

func TestInventoryService_DispatchUsesApplicationName(t *testing.T) {
	type dispatchRequest struct {
		payload   inventory.ScanPayload
		urlName   string
		authority string
	}
	requests := make(chan dispatchRequest, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("failed to read dispatch body: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		var payload inventory.ScanPayload
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Errorf("failed to decode dispatch payload: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		requests <- dispatchRequest{
			payload:   payload,
			urlName:   r.URL.Query().Get("clientid"),
			authority: r.Header.Get("Authorization"),
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	const appName = "mediathek:desk:01"
	cfg := desk.InventoryConfig{
		Enabled: true,
		Webservice: desk.WebserviceConfig{
			TargetURL:  config.EnvString(server.URL + "/inventory?clientid={clientid}"),
			HTTPMethod: http.MethodPost,
			JWTKey:     config.EnvString("inventory-test-secret"),
		},
	}
	dispatcher := inventory.NewWebserviceDispatcher(cfg.Webservice, nil)
	svc := NewInventoryService(cfg, appName, nil, dispatcher, nil)
	svc.StartSession("SHELF-01")

	tag := &libraryclient.TagReadEvent{
		Data: libraryclient.TagReadData{
			UID:    "E004015099887766",
			ItemID: "ITEM-01",
			CRCOk:  true,
		},
	}
	if item, _, ok := svc.ProcessTagRead(tag); !ok || item == nil {
		t.Fatalf("expected scan to be processed, got item=%v ok=%v", item, ok)
	}

	var request dispatchRequest
	select {
	case request = <-requests:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for webservice dispatch")
	}

	if request.payload.ClientID != appName {
		t.Errorf("expected payload clientId %q, got %q", appName, request.payload.ClientID)
	}
	if request.urlName != appName {
		t.Errorf("expected URL clientid %q, got %q", appName, request.urlName)
	}
	if request.payload.JWT == "" {
		t.Fatal("expected JWT in dispatch payload")
	}
	if request.authority != "Bearer "+request.payload.JWT {
		t.Errorf("expected Bearer header to match payload JWT, got %q", request.authority)
	}

	jwtParts := strings.Split(request.payload.JWT, ".")
	if len(jwtParts) != 3 {
		t.Fatalf("expected JWT to contain 3 parts, got %d", len(jwtParts))
	}
	claimsBytes, err := base64.RawURLEncoding.DecodeString(jwtParts[1])
	if err != nil {
		t.Fatalf("failed to decode JWT claims: %v", err)
	}
	var claims map[string]interface{}
	if err := json.Unmarshal(claimsBytes, &claims); err != nil {
		t.Fatalf("failed to unmarshal JWT claims: %v", err)
	}
	if claims["client_id"] != appName {
		t.Errorf("expected JWT client_id %q, got %v", appName, claims["client_id"])
	}
}
