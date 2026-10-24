package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/je4/Library-RFID-Apps/pkg/desk"
	"github.com/je4/Library-RFID-Apps/pkg/rfidclient"
)

func TestController_RoutesAndTemplates(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hub := desk.NewHub(50)
	hub.Start(ctx)
	defer hub.Close()

	ctrl, err := NewController(Config{
		Addr:          "localhost:8762",
		ExtAddr:       "http://localhost:8762",
		ClientName:    "test-rfid-desk",
		WSBroker:      "https://localhost:8765",
		Channels:      []string{"all", "intern", "rfid"},
		DefaultReader: "reader-01",
		Hub:           hub,
	})
	if err != nil {
		t.Fatalf("failed to create web controller: %v", err)
	}

	// 1. Test Static Asset Serving
	reqStatic := httptest.NewRequest(http.MethodGet, "/static/css/tailwind.min.css", nil)
	recStatic := httptest.NewRecorder()
	ctrl.router.ServeHTTP(recStatic, reqStatic)
	if recStatic.Code != http.StatusOK {
		t.Errorf("expected 200 for tailwind CSS, got %d", recStatic.Code)
	}

	reqHTMX := httptest.NewRequest(http.MethodGet, "/static/js/htmx-4.0.js", nil)
	recHTMX := httptest.NewRecorder()
	ctrl.router.ServeHTTP(recHTMX, reqHTMX)
	if recHTMX.Code != http.StatusOK {
		t.Errorf("expected 200 for htmx 4 JS, got %d", recHTMX.Code)
	}

	reqSSEJS := httptest.NewRequest(http.MethodGet, "/static/js/hx-sse.js", nil)
	recSSEJS := httptest.NewRecorder()
	ctrl.router.ServeHTTP(recSSEJS, reqSSEJS)
	if recSSEJS.Code != http.StatusOK {
		t.Errorf("expected 200 for hx-sse.js, got %d", recSSEJS.Code)
	}

	// 2. Test Dashboard UI Rendering
	reqUI := httptest.NewRequest(http.MethodGet, "/", nil)
	recUI := httptest.NewRecorder()
	ctrl.router.ServeHTTP(recUI, reqUI)
	if recUI.Code != http.StatusOK {
		t.Fatalf("expected 200 for dashboard UI, got %d", recUI.Code)
	}
	bodyUI := recUI.Body.String()
	if !strings.Contains(bodyUI, "RFID Desk") || !strings.Contains(bodyUI, "test-rfid-desk") {
		t.Errorf("expected dashboard UI to contain app name and client ID, got %s", bodyUI)
	}
	if !strings.Contains(bodyUI, "info-age GmbH, Basel") {
		t.Errorf("expected dashboard UI to contain copyright 'info-age GmbH, Basel', got %s", bodyUI)
	}
	if !strings.Contains(bodyUI, "Reader (Sender)") {
		t.Errorf("expected dashboard UI table header to contain 'Reader (Sender)', got %s", bodyUI)
	}

	// 3. Test API Status
	reqStatus := httptest.NewRequest(http.MethodGet, "/api/v1/status", nil)
	recStatus := httptest.NewRecorder()
	ctrl.router.ServeHTTP(recStatus, reqStatus)
	if recStatus.Code != http.StatusOK {
		t.Errorf("expected 200 for status API, got %d", recStatus.Code)
	}

	// 4. Test Tag Scan Broadcasting and API Tags
	tag := &rfidclient.TagReadEvent{
		UID:     "E004015099887766",
		ItemID:  "ITEM-TEST-123",
		ISIL:    "CH-000001-1",
		Country: "CH",
		CRCOk:   true,
		AFI:     "07",
		BaseRFIDPayload: rfidclient.BaseRFIDPayload{
			ClientID:  "reader-01",
			Timestamp: time.Now().UnixMilli(),
		},
	}
	ctrl.HandleTagRead(tag)

	reqTags := httptest.NewRequest(http.MethodGet, "/api/v1/tags", nil)
	recTags := httptest.NewRecorder()
	ctrl.router.ServeHTTP(recTags, reqTags)
	if recTags.Code != http.StatusOK {
		t.Errorf("expected 200 for tags API, got %d", recTags.Code)
	}
	if !strings.Contains(recTags.Body.String(), "ITEM-TEST-123") {
		t.Errorf("expected tags list to contain ITEM-TEST-123, got %s", recTags.Body.String())
	}

	// Test dashboard rendering with active tag
	recUIWithTag := httptest.NewRecorder()
	ctrl.router.ServeHTTP(recUIWithTag, reqUI)
	bodyWithTag := recUIWithTag.Body.String()
	if !strings.Contains(bodyWithTag, "reader-01") {
		t.Errorf("expected transponder row to contain reader 'reader-01', got %s", bodyWithTag)
	}
	if !strings.Contains(bodyWithTag, "Gesichert (07)") || !strings.Contains(bodyWithTag, "app-badge-afi-secured") {
		t.Errorf("expected transponder row to contain secured badge 'Gesichert (07)', got %s", bodyWithTag)
	}
	if !strings.Contains(bodyWithTag, "copy-itemid-btn") || !strings.Contains(bodyWithTag, `data-itemid="ITEM-TEST-123"`) {
		t.Errorf("expected transponder row to contain copy-itemid-btn, got %s", bodyWithTag)
	}

	// 5. Test Clear Tags (HTMX endpoint)
	reqClear := httptest.NewRequest(http.MethodPost, "/tags/clear", nil)
	recClear := httptest.NewRecorder()
	ctrl.router.ServeHTTP(recClear, reqClear)
	if recClear.Code != http.StatusOK {
		t.Errorf("expected 200 for clear tags, got %d", recClear.Code)
	}
	if !strings.Contains(recClear.Body.String(), "Noch keine RFID-Tags gescannt") {
		t.Errorf("expected empty table placeholder after clear, got %s", recClear.Body.String())
	}

	// 6. Test Index Top Button Bar (Loan button as first button)
	recIndex := httptest.NewRecorder()
	ctrl.router.ServeHTTP(recIndex, reqUI)
	bodyIndex := recIndex.Body.String()
	if !strings.Contains(bodyIndex, `id="nav-loan-btn"`) || !strings.Contains(bodyIndex, `href="/loan"`) {
		t.Errorf("expected index page to contain loan button in top button bar, got %s", bodyIndex)
	}

	// 7. Test Loan View (/loan)
	reqLoan := httptest.NewRequest(http.MethodGet, "/loan", nil)
	recLoan := httptest.NewRecorder()
	ctrl.router.ServeHTTP(recLoan, reqLoan)
	if recLoan.Code != http.StatusOK {
		t.Fatalf("expected 200 for /loan, got %d", recLoan.Code)
	}
	bodyLoan := recLoan.Body.String()
	if !strings.Contains(bodyLoan, "Ausleihe &amp; Rückgabe") && !strings.Contains(bodyLoan, "Ausleihe & Rückgabe") {
		t.Errorf("expected /loan to contain title 'Ausleihe & Rückgabe', got %s", bodyLoan)
	}
	if !strings.Contains(bodyLoan, "btn-copy-all-itemids") {
		t.Errorf("expected /loan to contain copy all item IDs button, got %s", bodyLoan)
	}
	if !strings.Contains(bodyLoan, "btn-unsecure") || !strings.Contains(bodyLoan, "btn-secure") {
		t.Errorf("expected /loan to contain Entsichern and Sichern buttons, got %s", bodyLoan)
	}
	if !strings.Contains(bodyLoan, `nav-desk-btn`) || !strings.Contains(bodyLoan, `href="/"`) {
		t.Errorf("expected /loan to contain link back to RFID Desk, got %s", bodyLoan)
	}

	// 8. Test Reader Qualification for Active Media on /loan
	// Tag 1: from configured reader "reader-01" -> should qualify
	tagQualified := &rfidclient.TagReadEvent{
		UID:    "E004015011112222",
		ItemID: "ITEM-QUALIFIED-1",
		ISIL:   "CH-000001-1",
		AFI:    "07",
		BaseRFIDPayload: rfidclient.BaseRFIDPayload{
			ClientID:  "reader-01",
			Timestamp: time.Now().UnixMilli(),
		},
	}
	// Tag 2: from another reader "other-reader-99" -> should NOT qualify for configured reader-01
	tagOther := &rfidclient.TagReadEvent{
		UID:    "E004015099998888",
		ItemID: "ITEM-OTHER-READER",
		ISIL:   "CH-000001-1",
		AFI:    "07",
		BaseRFIDPayload: rfidclient.BaseRFIDPayload{
			ClientID:  "other-reader-99",
			Timestamp: time.Now().UnixMilli(),
		},
	}
	ctrl.HandleTagRead(tagQualified)
	ctrl.HandleTagRead(tagOther)

	recLoanWithTags := httptest.NewRecorder()
	ctrl.router.ServeHTTP(recLoanWithTags, reqLoan)
	bodyLoanWithTags := recLoanWithTags.Body.String()
	if !strings.Contains(bodyLoanWithTags, "ITEM-QUALIFIED-1") {
		t.Errorf("expected qualified item ITEM-QUALIFIED-1 on loan page, got %s", bodyLoanWithTags)
	}
	if strings.Contains(bodyLoanWithTags, "ITEM-OTHER-READER") {
		t.Errorf("expected item from other-reader-99 to be excluded from configured reader-01 loan list, got %s", bodyLoanWithTags)
	}

	// 9. Test AFI Manipulation (Entsichern & Sichern)
	reqUnsecure := httptest.NewRequest(http.MethodPost, "/loan/afi/unsecure", nil)
	recUnsecure := httptest.NewRecorder()
	ctrl.router.ServeHTTP(recUnsecure, reqUnsecure)
	if recUnsecure.Code != http.StatusOK {
		t.Errorf("expected 200 for /loan/afi/unsecure, got %d", recUnsecure.Code)
	}
	if !strings.Contains(recUnsecure.Body.String(), "Entsichert (C2)") {
		t.Errorf("expected loan table to show 'Entsichert (C2)' after unsecure, got %s", recUnsecure.Body.String())
	}

	reqSecure := httptest.NewRequest(http.MethodPost, "/loan/afi/secure", nil)
	recSecure := httptest.NewRecorder()
	ctrl.router.ServeHTTP(recSecure, reqSecure)
	if recSecure.Code != http.StatusOK {
		t.Errorf("expected 200 for /loan/afi/secure, got %d", recSecure.Code)
	}
	if !strings.Contains(recSecure.Body.String(), "Gesichert (07)") {
		t.Errorf("expected loan table to show 'Gesichert (07)' after secure, got %s", recSecure.Body.String())
	}

	// 10. Test Antenna Field Cleared for Loan View
	ctrl.HandleTagsCleared()
	recLoanCleared := httptest.NewRecorder()
	ctrl.router.ServeHTTP(recLoanCleared, reqLoan)
	if !strings.Contains(recLoanCleared.Body.String(), "Keine aktiven Medien auf dem Lesefeld") {
		t.Errorf("expected placeholder 'Keine aktiven Medien auf dem Lesefeld' after antenna field cleared, got %s", recLoanCleared.Body.String())
	}
}

type mockRFIDClient struct {
	rfidclient.Client
	broadcastAFIErr  error
	writeAFIErr      error
	writeAFIResp     *rfidclient.WriteAFIResponse
	writeAFIHandler  func(ctx context.Context, target string, req rfidclient.WriteAFIRequest) (*rfidclient.WriteAFIResponse, error)
	lastBroadcastCh  string
	lastBroadcastReq rfidclient.WriteAFIRequest
	lastWriteTarget  string
	lastWriteReq     rfidclient.WriteAFIRequest
	broadcastCount   int
	writeCount       int
}

func (m *mockRFIDClient) BroadcastAFI(ctx context.Context, channel string, req rfidclient.WriteAFIRequest) error {
	m.broadcastCount++
	m.lastBroadcastCh = channel
	m.lastBroadcastReq = req
	return m.broadcastAFIErr
}

func (m *mockRFIDClient) WriteAFI(ctx context.Context, target string, req rfidclient.WriteAFIRequest) (*rfidclient.WriteAFIResponse, error) {
	m.writeCount++
	m.lastWriteTarget = target
	m.lastWriteReq = req
	if m.writeAFIHandler != nil {
		return m.writeAFIHandler(ctx, target, req)
	}
	if m.writeAFIErr != nil {
		return nil, m.writeAFIErr
	}
	if m.writeAFIResp != nil {
		return m.writeAFIResp, nil
	}
	return &rfidclient.WriteAFIResponse{
		BaseRFIDPayload: rfidclient.BaseRFIDPayload{
			Event: rfidclient.EventWriteAFIResponse,
		},
		Status: "ok",
		UID:    req.UID,
		AFI:    req.AFI,
	}, nil
}

func TestController_AFIViaRFIDClient_Success(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hub := desk.NewHub(50)
	hub.Start(ctx)
	defer hub.Close()

	mockCli := &mockRFIDClient{}

	ctrl, err := NewController(Config{
		Addr:          "localhost:8762",
		ClientName:    "test-rfid-desk",
		Channels:      []string{"rfid-ch"},
		DefaultReader: "reader-01",
		Hub:           hub,
		RFIDClient:    mockCli,
	})
	if err != nil {
		t.Fatalf("failed to create web controller: %v", err)
	}

	// Add an active tag
	ctrl.HandleTagRead(&rfidclient.TagReadEvent{
		UID:    "E004015011112222",
		ItemID: "ITEM-SUCCESS-1",
		AFI:    "07",
		BaseRFIDPayload: rfidclient.BaseRFIDPayload{
			ClientID:  "reader-01",
			Timestamp: time.Now().UnixMilli(),
		},
	})

	// 1. Post unsecure
	reqUnsecure := httptest.NewRequest(http.MethodPost, "/loan/afi/unsecure", nil)
	recUnsecure := httptest.NewRecorder()
	ctrl.router.ServeHTTP(recUnsecure, reqUnsecure)
	if recUnsecure.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", recUnsecure.Code)
	}
	bodyUnsecure := recUnsecure.Body.String()
	if !strings.Contains(bodyUnsecure, "Entsichert (C2)") {
		t.Errorf("expected loan table to show 'Entsichert (C2)', got: %s", bodyUnsecure)
	}
	if !strings.Contains(bodyUnsecure, "loan-action-success-row") {
		t.Errorf("expected success notification banner in table response, got: %s", bodyUnsecure)
	}
	if mockCli.broadcastCount == 0 || mockCli.writeCount == 0 {
		t.Errorf("expected broadcast and write calls on mock client, got broadcast=%d, write=%d", mockCli.broadcastCount, mockCli.writeCount)
	}

	// 2. Post secure
	reqSecure := httptest.NewRequest(http.MethodPost, "/loan/afi/secure", nil)
	recSecure := httptest.NewRecorder()
	ctrl.router.ServeHTTP(recSecure, reqSecure)
	if recSecure.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", recSecure.Code)
	}
	bodySecure := recSecure.Body.String()
	if !strings.Contains(bodySecure, "Gesichert (07)") {
		t.Errorf("expected loan table to show 'Gesichert (07)', got: %s", bodySecure)
	}
	if !strings.Contains(bodySecure, "loan-action-success-row") {
		t.Errorf("expected success notification banner in table response, got: %s", bodySecure)
	}

	// 3. API loan AFI
	reqAPI := httptest.NewRequest(http.MethodPost, "/api/v1/loan/afi", strings.NewReader(`{"action":"unsecure"}`))
	reqAPI.Header.Set("Content-Type", "application/json")
	recAPI := httptest.NewRecorder()
	ctrl.router.ServeHTTP(recAPI, reqAPI)
	if recAPI.Code != http.StatusOK {
		t.Errorf("expected 200 for API loan AFI, got %d", recAPI.Code)
	}
}

func TestController_AFIViaRFIDClient_Error(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hub := desk.NewHub(50)
	hub.Start(ctx)
	defer hub.Close()

	mockCli := &mockRFIDClient{
		writeAFIErr: http.ErrHandlerTimeout,
	}

	ctrl, err := NewController(Config{
		Addr:          "localhost:8762",
		ClientName:    "test-rfid-desk",
		Channels:      []string{"rfid-ch"},
		DefaultReader: "reader-01",
		Hub:           hub,
		RFIDClient:    mockCli,
	})
	if err != nil {
		t.Fatalf("failed to create web controller: %v", err)
	}

	// Add an active tag with AFI 07 (Gesichert)
	ctrl.HandleTagRead(&rfidclient.TagReadEvent{
		UID:    "E004015099998888",
		ItemID: "ITEM-ERR-1",
		AFI:    "07",
		BaseRFIDPayload: rfidclient.BaseRFIDPayload{
			ClientID:  "reader-01",
			Timestamp: time.Now().UnixMilli(),
		},
	})

	// Post unsecure when write fails
	reqUnsecure := httptest.NewRequest(http.MethodPost, "/loan/afi/unsecure", nil)
	recUnsecure := httptest.NewRecorder()
	ctrl.router.ServeHTTP(recUnsecure, reqUnsecure)
	if recUnsecure.Code != http.StatusOK {
		t.Errorf("expected 200 with error fragment, got %d", recUnsecure.Code)
	}
	body := recUnsecure.Body.String()
	if !strings.Contains(body, "loan-action-error-row") {
		t.Errorf("expected error row in loan table, got: %s", body)
	}
	if !strings.Contains(body, "Fehler beim Setzen der AFI") {
		t.Errorf("expected error message in loan table, got: %s", body)
	}
	// Tag should NOT have been updated to C2 in hub
	active := hub.GetActiveTags("reader-01")
	if len(active) > 0 && active[0].AFI != "07" {
		t.Errorf("expected tag AFI to remain '07' after failure, got %s", active[0].AFI)
	}

	// Test API returns 500 when write fails
	reqAPI := httptest.NewRequest(http.MethodPost, "/api/v1/loan/afi", strings.NewReader(`{"action":"secure"}`))
	reqAPI.Header.Set("Content-Type", "application/json")
	recAPI := httptest.NewRecorder()
	ctrl.router.ServeHTTP(recAPI, reqAPI)
	if recAPI.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 for failing API loan AFI, got %d", recAPI.Code)
	}
}

func TestController_HandleWriteAFIResponse_Success(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hub := desk.NewHub(50)
	hub.Start(ctx)
	defer hub.Close()

	ctrl, err := NewController(Config{
		Addr:          "localhost:8762",
		ClientName:    "test-rfid-desk",
		DefaultReader: "reader-01",
		Hub:           hub,
	})
	if err != nil {
		t.Fatalf("failed to create web controller: %v", err)
	}

	// Add an active tag
	ctrl.HandleTagRead(&rfidclient.TagReadEvent{
		UID:    "E004015011223344",
		ItemID: "ITEM-ASYNC-1",
		AFI:    "07",
		BaseRFIDPayload: rfidclient.BaseRFIDPayload{
			ClientID:  "reader-01",
			Timestamp: time.Now().UnixMilli(),
		},
	})

	sseChan := hub.Subscribe()

	// Simulate successful asynchronous WriteAFIResponse
	resp := &rfidclient.WriteAFIResponse{
		BaseRFIDPayload: rfidclient.BaseRFIDPayload{
			Event: rfidclient.EventWriteAFIResponse,
		},
		Status: "ok",
		UID:    "E004015011223344",
		AFI:    "C2",
	}

	ctrl.HandleWriteAFIResponse(resp)

	// Verify tag in Hub updated to C2
	active := hub.GetActiveTags("reader-01")
	if len(active) == 0 || active[0].AFI != "C2" {
		t.Fatalf("expected tag AFI in Hub to be 'C2', got %+v", active)
	}

	// Verify SSE broadcast
	select {
	case msg := <-sseChan:
		if msg.Event != "loan_tags" {
			t.Errorf("expected SSE event 'loan_tags', got '%s'", msg.Event)
		}
		if !strings.Contains(msg.Data, "loan-action-success-row") {
			t.Errorf("expected success row in SSE broadcast, got: %s", msg.Data)
		}
		if !strings.Contains(msg.Data, "Entsichert (C2)") {
			t.Errorf("expected updated AFI status 'Entsichert (C2)' in SSE broadcast, got: %s", msg.Data)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for SSE loan_tags message")
	}
}

func TestController_HandleWriteAFIResponse_Error(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hub := desk.NewHub(50)
	hub.Start(ctx)
	defer hub.Close()

	ctrl, err := NewController(Config{
		Addr:          "localhost:8762",
		ClientName:    "test-rfid-desk",
		DefaultReader: "reader-01",
		Hub:           hub,
	})
	if err != nil {
		t.Fatalf("failed to create web controller: %v", err)
	}

	// Add an active tag with AFI 07 (Gesichert)
	ctrl.HandleTagRead(&rfidclient.TagReadEvent{
		UID:    "E00401080AFD252F",
		ItemID: "ITEM-ERR-ASYNC",
		AFI:    "07",
		BaseRFIDPayload: rfidclient.BaseRFIDPayload{
			ClientID:  "reader-01",
			Timestamp: time.Now().UnixMilli(),
		},
	})

	sseChan := hub.Subscribe()

	// Simulate failed asynchronous WriteAFIResponse (e.g. ISO 149 / 0x0F error)
	errText := "Failed to write AFI byte 0xC2 (status: 149 (Iso error (149)), ISO Error: 0x0F (Unknown error))"
	resp := &rfidclient.WriteAFIResponse{
		BaseRFIDPayload: rfidclient.BaseRFIDPayload{
			Event: rfidclient.EventWriteAFIResponse,
		},
		Status: "error",
		Error:  errText,
		UID:    "E00401080AFD252F",
		AFI:    "C2",
	}

	ctrl.HandleWriteAFIResponse(resp)

	// Verify tag in Hub was NOT modified and remains "07"
	active := hub.GetActiveTags("reader-01")
	if len(active) == 0 || active[0].AFI != "07" {
		t.Fatalf("expected tag AFI in Hub to remain '07', got %+v", active)
	}

	// Verify SSE broadcast contains error notification
	select {
	case msg := <-sseChan:
		if msg.Event != "loan_tags" {
			t.Errorf("expected SSE event 'loan_tags', got '%s'", msg.Event)
		}
		if !strings.Contains(msg.Data, "loan-action-error-row") {
			t.Errorf("expected error row in SSE broadcast, got: %s", msg.Data)
		}
		if !strings.Contains(msg.Data, "Iso error (149)") {
			t.Errorf("expected error details in SSE broadcast, got: %s", msg.Data)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for SSE loan_tags message")
	}
}

func TestController_SetAFI_SynchronousFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hub := desk.NewHub(50)
	hub.Start(ctx)
	defer hub.Close()

	mockCli := &mockRFIDClient{
		writeAFIResp: &rfidclient.WriteAFIResponse{
			Status: "error",
			Error:  "ISO error: 149 / 0x0F write protection failed",
		},
	}

	ctrl, err := NewController(Config{
		Addr:          "localhost:8762",
		ClientName:    "test-rfid-desk",
		DefaultReader: "reader-01",
		Hub:           hub,
		RFIDClient:    mockCli,
	})
	if err != nil {
		t.Fatalf("failed to create web controller: %v", err)
	}

	ctrl.HandleTagRead(&rfidclient.TagReadEvent{
		UID:    "E00401080AFD252F",
		ItemID: "ITEM-SYNC-ERR",
		AFI:    "07",
		BaseRFIDPayload: rfidclient.BaseRFIDPayload{
			ClientID:  "reader-01",
			Timestamp: time.Now().UnixMilli(),
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/loan/afi/unsecure", nil)
	rec := httptest.NewRecorder()
	ctrl.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "loan-action-error-row") {
		t.Errorf("expected loan-action-error-row in response body, got: %s", body)
	}
	if !strings.Contains(body, "149 / 0x0F write protection failed") {
		t.Errorf("expected error text in response body, got: %s", body)
	}

	// Hub AFI should remain "07"
	active := hub.GetActiveTags("reader-01")
	if len(active) == 0 || active[0].AFI != "07" {
		t.Fatalf("expected tag AFI to remain 07, got %+v", active)
	}
}

func TestController_SetAFI_TargetedReader(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hub := desk.NewHub(50)
	hub.Start(ctx)
	defer hub.Close()

	mockCli := &mockRFIDClient{}

	// No DefaultReader configured
	ctrl, err := NewController(Config{
		Addr:          "localhost:8762",
		ClientName:    "test-rfid-desk",
		DefaultReader: "",
		Hub:           hub,
		RFIDClient:    mockCli,
	})
	if err != nil {
		t.Fatalf("failed to create web controller: %v", err)
	}

	// Tag scanned from reader 'rfid:reader:desk-antenna-01'
	ctrl.HandleTagRead(&rfidclient.TagReadEvent{
		UID:    "E004015099887766",
		ItemID: "ITEM-TARGETED-1",
		AFI:    "07",
		BaseRFIDPayload: rfidclient.BaseRFIDPayload{
			ClientID:  "rfid:reader:desk-antenna-01",
			Timestamp: time.Now().UnixMilli(),
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/loan/afi/unsecure", nil)
	rec := httptest.NewRecorder()
	ctrl.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	if mockCli.lastWriteTarget != "rfid:reader:desk-antenna-01" {
		t.Errorf("expected write target 'rfid:reader:desk-antenna-01', got '%s'", mockCli.lastWriteTarget)
	}
}

func TestController_LoanAFISingleTag(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hub := desk.NewHub(50)
	hub.Start(ctx)
	defer hub.Close()

	mockCli := &mockRFIDClient{}

	ctrl, err := NewController(Config{
		Addr:          "localhost:8762",
		ClientName:    "test-rfid-desk",
		DefaultReader: "reader-01",
		Hub:           hub,
		RFIDClient:    mockCli,
	})
	if err != nil {
		t.Fatalf("failed to create web controller: %v", err)
	}

	// Add two tags with AFI 07 (Gesichert)
	ctrl.HandleTagRead(&rfidclient.TagReadEvent{
		UID:    "TAG-01",
		ItemID: "BOOK-01",
		AFI:    "07",
		BaseRFIDPayload: rfidclient.BaseRFIDPayload{
			ClientID:  "reader-01",
			Timestamp: time.Now().UnixMilli(),
		},
	})
	ctrl.HandleTagRead(&rfidclient.TagReadEvent{
		UID:    "TAG-02",
		ItemID: "BOOK-02",
		AFI:    "07",
		BaseRFIDPayload: rfidclient.BaseRFIDPayload{
			ClientID:  "reader-01",
			Timestamp: time.Now().UnixMilli(),
		},
	})

	// 1. Target single tag TAG-01 with unsecure
	req := httptest.NewRequest(http.MethodPost, "/loan/afi/unsecure?uid=TAG-01", nil)
	rec := httptest.NewRecorder()
	ctrl.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	active := hub.GetActiveTags("reader-01")
	var tag1, tag2 *desk.TagItem
	for _, item := range active {
		if item.UID == "TAG-01" {
			tag1 = item
		}
		if item.UID == "TAG-02" {
			tag2 = item
		}
	}

	if tag1 == nil || tag2 == nil {
		t.Fatalf("expected both tags to be active")
	}

	if tag1.AFI != "C2" || tag1.AFIWriteStatus != "success" {
		t.Errorf("expected TAG-01 to be C2 success, got AFI=%s, status=%s", tag1.AFI, tag1.AFIWriteStatus)
	}
	if tag2.AFI != "07" {
		t.Errorf("expected TAG-02 to remain 07, got AFI=%s", tag2.AFI)
	}

	// 2. Target TAG-01 with single endpoint to secure it back
	reqSingle := httptest.NewRequest(http.MethodPost, "/loan/afi/single?uid=TAG-01&action=secure", nil)
	recSingle := httptest.NewRecorder()
	ctrl.router.ServeHTTP(recSingle, reqSingle)

	if recSingle.Code != http.StatusOK {
		t.Errorf("expected 200 for single endpoint, got %d", recSingle.Code)
	}

	active = hub.GetActiveTags("reader-01")
	for _, item := range active {
		if item.UID == "TAG-01" && item.AFI != "07" {
			t.Errorf("expected TAG-01 to be secured back to 07, got %s", item.AFI)
		}
	}
}

func TestController_LoanAFIMixedBatch(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hub := desk.NewHub(50)
	hub.Start(ctx)
	defer hub.Close()

	// Mock client where TAG-01 succeeds and TAG-02 fails
	mockCli := &mockRFIDClient{
		writeAFIHandler: func(ctx context.Context, target string, req rfidclient.WriteAFIRequest) (*rfidclient.WriteAFIResponse, error) {
			if req.UID == "TAG-02" {
				return &rfidclient.WriteAFIResponse{
					BaseRFIDPayload: rfidclient.BaseRFIDPayload{Event: rfidclient.EventWriteAFIResponse},
					Status:          "error",
					Error:           "Antenna communication timeout",
					UID:             req.UID,
					AFI:             req.AFI,
				}, nil
			}
			return &rfidclient.WriteAFIResponse{
				BaseRFIDPayload: rfidclient.BaseRFIDPayload{Event: rfidclient.EventWriteAFIResponse},
				Status:          "ok",
				UID:             req.UID,
				AFI:             req.AFI,
			}, nil
		},
	}

	ctrl, err := NewController(Config{
		Addr:          "localhost:8762",
		ClientName:    "test-rfid-desk",
		DefaultReader: "reader-01",
		Hub:           hub,
		RFIDClient:    mockCli,
	})
	if err != nil {
		t.Fatalf("failed to create web controller: %v", err)
	}

	ctrl.HandleTagRead(&rfidclient.TagReadEvent{
		UID:             "TAG-01",
		ItemID:          "BOOK-SUCCESS",
		AFI:             "07",
		BaseRFIDPayload: rfidclient.BaseRFIDPayload{ClientID: "reader-01", Timestamp: time.Now().UnixMilli()},
	})
	ctrl.HandleTagRead(&rfidclient.TagReadEvent{
		UID:             "TAG-02",
		ItemID:          "BOOK-FAIL",
		AFI:             "07",
		BaseRFIDPayload: rfidclient.BaseRFIDPayload{ClientID: "reader-01", Timestamp: time.Now().UnixMilli()},
	})

	// Execute batch unsecure
	req := httptest.NewRequest(http.MethodPost, "/loan/afi/unsecure", nil)
	rec := httptest.NewRecorder()
	ctrl.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	body := rec.Body.String()

	// Verify differentiated messages in HTML fragment
	if !strings.Contains(body, "1 von 2 Medien erfolgreich aktualisiert") {
		t.Errorf("expected partial success message in body, got: %s", body)
	}
	if !strings.Contains(body, "1 fehlgeschlagen") {
		t.Errorf("expected failure summary in body, got: %s", body)
	}
	if !strings.Contains(body, "Antenna communication timeout") {
		t.Errorf("expected specific error reason in body, got: %s", body)
	}

	// Verify row statuses
	active := hub.GetActiveTags("reader-01")
	for _, item := range active {
		if item.UID == "TAG-01" {
			if item.AFI != "C2" || item.AFIWriteStatus != "success" {
				t.Errorf("expected TAG-01 success C2, got AFI=%s, status=%s", item.AFI, item.AFIWriteStatus)
			}
		}
		if item.UID == "TAG-02" {
			if item.AFI != "07" || item.AFIWriteStatus != "error" || item.AFIWriteError != "Antenna communication timeout" {
				t.Errorf("expected TAG-02 error, got AFI=%s, status=%s, error=%s", item.AFI, item.AFIWriteStatus, item.AFIWriteError)
			}
		}
	}

	// Retry TAG-02 individually with success
	mockCli.writeAFIHandler = nil // will return default success
	reqRetry := httptest.NewRequest(http.MethodPost, "/loan/afi/unsecure?uid=TAG-02", nil)
	recRetry := httptest.NewRecorder()
	ctrl.router.ServeHTTP(recRetry, reqRetry)

	if recRetry.Code != http.StatusOK {
		t.Errorf("expected 200 for retry, got %d", recRetry.Code)
	}

	active = hub.GetActiveTags("reader-01")
	for _, item := range active {
		if item.UID == "TAG-02" {
			if item.AFI != "C2" || item.AFIWriteStatus != "success" {
				t.Errorf("expected TAG-02 to succeed after retry, got AFI=%s, status=%s", item.AFI, item.AFIWriteStatus)
			}
		}
	}
}

func TestController_APILoanAFI_MixedBatch(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hub := desk.NewHub(50)
	hub.Start(ctx)
	defer hub.Close()

	mockCli := &mockRFIDClient{
		writeAFIHandler: func(ctx context.Context, target string, req rfidclient.WriteAFIRequest) (*rfidclient.WriteAFIResponse, error) {
			if req.UID == "TAG-ERR" {
				return &rfidclient.WriteAFIResponse{
					Status: "error",
					Error:  "Write lock active",
					UID:    req.UID,
					AFI:    req.AFI,
				}, nil
			}
			return &rfidclient.WriteAFIResponse{
				Status: "ok",
				UID:    req.UID,
				AFI:    req.AFI,
			}, nil
		},
	}

	ctrl, err := NewController(Config{
		Addr:          "localhost:8762",
		ClientName:    "test-rfid-desk",
		DefaultReader: "reader-01",
		Hub:           hub,
		RFIDClient:    mockCli,
	})
	if err != nil {
		t.Fatalf("failed to create web controller: %v", err)
	}

	ctrl.HandleTagRead(&rfidclient.TagReadEvent{
		UID:             "TAG-OK",
		ItemID:          "BOOK-OK",
		AFI:             "07",
		BaseRFIDPayload: rfidclient.BaseRFIDPayload{ClientID: "reader-01", Timestamp: time.Now().UnixMilli()},
	})
	ctrl.HandleTagRead(&rfidclient.TagReadEvent{
		UID:             "TAG-ERR",
		ItemID:          "BOOK-ERR",
		AFI:             "07",
		BaseRFIDPayload: rfidclient.BaseRFIDPayload{ClientID: "reader-01", Timestamp: time.Now().UnixMilli()},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/loan/afi", strings.NewReader(`{"action":"unsecure"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	ctrl.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for partial batch, got %d", rec.Code)
	}

	var resp struct {
		Status       string `json:"status"`
		SuccessCount int    `json:"successCount"`
		ErrorCount   int    `json:"errorCount"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response JSON: %v", err)
	}

	if resp.Status != "partial" || resp.SuccessCount != 1 || resp.ErrorCount != 1 {
		t.Errorf("unexpected API response: %+v", resp)
	}
}
