package web

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/je4/Library-RFID-Apps/pkg/desk"
	"github.com/je4/Library-RFID-Apps/pkg/rfidclient"
	"github.com/je4/wsbrokerclient"
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

	reqDaisy := httptest.NewRequest(http.MethodGet, "/static/css/daisyui.min.css", nil)
	recDaisy := httptest.NewRecorder()
	ctrl.router.ServeHTTP(recDaisy, reqDaisy)
	if recDaisy.Code != http.StatusOK {
		t.Errorf("expected 200 for daisyui CSS, got %d", recDaisy.Code)
	}
	if !strings.Contains(recDaisy.Body.String(), ".btn") || !strings.Contains(recDaisy.Body.String(), "data-theme") {
		t.Errorf("expected daisyui.min.css to contain component rules and data-theme definitions, got %s", recDaisy.Body.String())
	}

	reqDeskCSS := httptest.NewRequest(http.MethodGet, "/static/scss/desk.css", nil)
	recDeskCSS := httptest.NewRecorder()
	ctrl.router.ServeHTTP(recDeskCSS, reqDeskCSS)
	if recDeskCSS.Code != http.StatusOK {
		t.Errorf("expected 200 for desk.css, got %d", recDeskCSS.Code)
	}
	if !strings.Contains(recDeskCSS.Body.String(), "--color-bg-image") || !strings.Contains(recDeskCSS.Body.String(), "background.jpg") {
		t.Errorf("expected desk.css to contain background image variable, got %s", recDeskCSS.Body.String())
	}

	reqDeskMap := httptest.NewRequest(http.MethodGet, "/static/scss/desk.css.map", nil)
	recDeskMap := httptest.NewRecorder()
	ctrl.router.ServeHTTP(recDeskMap, reqDeskMap)
	if recDeskMap.Code != http.StatusOK {
		t.Errorf("expected 200 for desk.css.map, got %d", recDeskMap.Code)
	}

	reqBgImg := httptest.NewRequest(http.MethodGet, "/static/img/background.jpg", nil)
	recBgImg := httptest.NewRecorder()
	ctrl.router.ServeHTTP(recBgImg, reqBgImg)
	if recBgImg.Code != http.StatusOK {
		t.Errorf("expected 200 for background.jpg, got %d", recBgImg.Code)
	}

	reqBubblegumBgImg := httptest.NewRequest(http.MethodGet, "/static/img/bubblegum_background.jpg", nil)
	recBubblegumBgImg := httptest.NewRecorder()
	ctrl.router.ServeHTTP(recBubblegumBgImg, reqBubblegumBgImg)
	if recBubblegumBgImg.Code != http.StatusOK {
		t.Errorf("expected 200 for bubblegum_background.jpg, got %d", recBubblegumBgImg.Code)
	}

	reqIcon := httptest.NewRequest(http.MethodGet, "/static/img/icons/bolt.svg", nil)
	recIcon := httptest.NewRecorder()
	ctrl.router.ServeHTTP(recIcon, reqIcon)
	if recIcon.Code != http.StatusOK {
		t.Errorf("expected 200 for /static/img/icons/bolt.svg, got %d", recIcon.Code)
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

	for _, modulePath := range []string{
		"/static/js/theme-init.js",
		"/static/js/desk.js",
		"/static/js/modules/audio.js",
		"/static/js/modules/table-actions.js",
		"/static/js/modules/isil-flyout.js",
		"/static/js/modules/afi-modal.js",
		"/static/js/modules/sse.js",
		"/static/js/modules/skin-selector.js",
		"/static/js/modules/theme-selector.js",
		"/static/js/modules/encode-controller.js",
	} {
		reqMod := httptest.NewRequest(http.MethodGet, modulePath, nil)
		recMod := httptest.NewRecorder()
		ctrl.router.ServeHTTP(recMod, reqMod)
		if recMod.Code != http.StatusOK {
			t.Errorf("expected 200 for %s, got %d", modulePath, recMod.Code)
		}
	}

	// 2. Test Channel Selection Index Page
	reqIndex := httptest.NewRequest(http.MethodGet, "/", nil)
	recIndex := httptest.NewRecorder()
	ctrl.router.ServeHTTP(recIndex, reqIndex)
	if recIndex.Code != http.StatusOK {
		t.Fatalf("expected 200 for index page, got %d", recIndex.Code)
	}
	bodyIndex := recIndex.Body.String()
	if !strings.Contains(bodyIndex, "RFID Desk") || !strings.Contains(bodyIndex, "test-rfid-desk") {
		t.Errorf("expected index page to contain app name and client ID, got %s", bodyIndex)
	}
	if !strings.Contains(bodyIndex, "Channel-Auswahl") {
		t.Errorf("expected index page to contain 'Channel-Auswahl', got %s", bodyIndex)
	}
	if !strings.Contains(bodyIndex, "all") || !strings.Contains(bodyIndex, "intern") || !strings.Contains(bodyIndex, "rfid") {
		t.Errorf("expected index page to list channels, got %s", bodyIndex)
	}
	if !strings.Contains(bodyIndex, "info-age GmbH, Basel") {
		t.Errorf("expected index page to contain copyright 'info-age GmbH, Basel', got %s", bodyIndex)
	}

	// 2b. Test Channel Dashboard UI Rendering (/rfid/)
	reqUI := httptest.NewRequest(http.MethodGet, "/rfid/", nil)
	recUI := httptest.NewRecorder()
	ctrl.router.ServeHTTP(recUI, reqUI)
	if recUI.Code != http.StatusOK {
		t.Fatalf("expected 200 for channel dashboard UI, got %d", recUI.Code)
	}
	bodyUI := recUI.Body.String()
	if !strings.Contains(bodyUI, "RFID Desk") || !strings.Contains(bodyUI, "test-rfid-desk") {
		t.Errorf("expected dashboard UI to contain app name and client ID, got %s", bodyUI)
	}
	if !strings.Contains(bodyUI, "Reader (Sender)") {
		t.Errorf("expected dashboard UI table header to contain 'Reader (Sender)', got %s", bodyUI)
	}
	if !strings.Contains(bodyUI, "Channel: <strong>rfid</strong>") && !strings.Contains(bodyUI, "rfid") {
		t.Errorf("expected dashboard UI to contain current channel 'rfid', got %s", bodyUI)
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
		BaseEvent: wsbrokerclient.BaseEvent{
			Channel: "rfid",
		},
		Data: rfidclient.TagReadData{
			UID:     "E004015099887766",
			ItemID:  "ITEM-TEST-123",
			ISIL:    "CH-000001-1",
			Country: "CH",
			CRCOk:   true,
			AFI:     "07",
			BaseRFIDPayload: rfidclient.BaseRFIDPayload{
				ReaderID: "rfid:reader:01",
			},
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
	if !strings.Contains(bodyWithTag, "rfid:reader:01") {
		t.Errorf("expected transponder row to contain reader 'rfid:reader:01', got %s", bodyWithTag)
	}
	if !strings.Contains(bodyWithTag, "#rfid") || !strings.Contains(bodyWithTag, `title="Channel: rfid"`) {
		t.Errorf("expected transponder row to contain channel '#rfid', got %s", bodyWithTag)
	}
	if !strings.Contains(bodyWithTag, "Gesichert (07)") {
		t.Errorf("expected transponder row to contain AFI status 'Gesichert (07)', got %s", bodyWithTag)
	}
	if !strings.Contains(bodyWithTag, "copy-itemid-btn") || !strings.Contains(bodyWithTag, `data-itemid="ITEM-TEST-123"`) {
		t.Errorf("expected transponder row to contain copy-itemid-btn, got %s", bodyWithTag)
	}

	// 5. Test Clear Tags (HTMX endpoint)
	reqClear := httptest.NewRequest(http.MethodPost, "/rfid/tags/clear", nil)
	recClear := httptest.NewRecorder()
	ctrl.router.ServeHTTP(recClear, reqClear)
	if recClear.Code != http.StatusOK {
		t.Errorf("expected 200 for clear tags, got %d", recClear.Code)
	}
	if !strings.Contains(recClear.Body.String(), "Noch keine RFID-Tags gescannt") {
		t.Errorf("expected empty table placeholder after clear, got %s", recClear.Body.String())
	}

	// 6. Test Dashboard Top Button Bar (Loan button as first button)
	recDashboard := httptest.NewRecorder()
	ctrl.router.ServeHTTP(recDashboard, reqUI)
	bodyDashboard := recDashboard.Body.String()
	if !strings.Contains(bodyDashboard, `id="nav-loan-btn"`) || !strings.Contains(bodyDashboard, `href="loan"`) {
		t.Errorf("expected channel dashboard page to contain loan button with relative link, got %s", bodyDashboard)
	}

	// 7. Test Loan View (/rfid/loan)
	reqLoan := httptest.NewRequest(http.MethodGet, "/rfid/loan", nil)
	recLoan := httptest.NewRecorder()
	ctrl.router.ServeHTTP(recLoan, reqLoan)
	if recLoan.Code != http.StatusOK {
		t.Fatalf("expected 200 for /rfid/loan, got %d", recLoan.Code)
	}
	bodyLoan := recLoan.Body.String()
	if !strings.Contains(bodyLoan, "Ausleihe &amp; Rückgabe") && !strings.Contains(bodyLoan, "Ausleihe & Rückgabe") {
		t.Errorf("expected /rfid/loan to contain title 'Ausleihe & Rückgabe', got %s", bodyLoan)
	}
	if !strings.Contains(bodyLoan, "btn-copy-all-itemids") {
		t.Errorf("expected /rfid/loan to contain copy all item IDs button, got %s", bodyLoan)
	}
	if !strings.Contains(bodyLoan, "btn-unsecure") || !strings.Contains(bodyLoan, "btn-secure") {
		t.Errorf("expected /rfid/loan to contain Entsichern and Sichern buttons, got %s", bodyLoan)
	}
	if !strings.Contains(bodyLoan, `nav-desk-btn`) || !strings.Contains(bodyLoan, `href="."`) {
		t.Errorf("expected /rfid/loan to contain relative link back to RFID Desk, got %s", bodyLoan)
	}
	if !strings.Contains(bodyLoan, "afi-modal-overlay") || !strings.Contains(bodyLoan, "AFI wird geschrieben") {
		t.Errorf("expected /rfid/loan to contain afi-modal-overlay, got %s", bodyLoan)
	}

	// 8. Test Reader Qualification for Active Media on /rfid/loan
	// Tag 1: from configured reader "reader-01" -> should qualify
	tagQualified := &rfidclient.TagReadEvent{
		Data: rfidclient.TagReadData{
			UID:    "E004015011112222",
			ItemID: "ITEM-QUALIFIED-1",
			ISIL:   "CH-000001-1",
			AFI:    "07",
			BaseRFIDPayload: rfidclient.BaseRFIDPayload{
				ReaderID: "reader-01",
			},
		},
	}
	// Tag 2: from another reader "other-reader-99" -> should NOT qualify for configured reader-01
	tagOther := &rfidclient.TagReadEvent{
		Data: rfidclient.TagReadData{
			UID:    "E004015099998888",
			ItemID: "ITEM-OTHER-READER",
			ISIL:   "CH-000001-1",
			AFI:    "07",
			BaseRFIDPayload: rfidclient.BaseRFIDPayload{
				ReaderID: "other-reader-99",
			},
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
	reqUnsecure := httptest.NewRequest(http.MethodPost, "/rfid/loan/afi/unsecure", nil)
	recUnsecure := httptest.NewRecorder()
	ctrl.router.ServeHTTP(recUnsecure, reqUnsecure)
	if recUnsecure.Code != http.StatusOK {
		t.Errorf("expected 200 for /rfid/loan/afi/unsecure, got %d", recUnsecure.Code)
	}
	if !strings.Contains(recUnsecure.Body.String(), "Entsichert (C2)") {
		t.Errorf("expected loan table to show 'Entsichert (C2)' after unsecure, got %s", recUnsecure.Body.String())
	}

	reqSecure := httptest.NewRequest(http.MethodPost, "/rfid/loan/afi/secure", nil)
	recSecure := httptest.NewRecorder()
	ctrl.router.ServeHTTP(recSecure, reqSecure)
	if recSecure.Code != http.StatusOK {
		t.Errorf("expected 200 for /rfid/loan/afi/secure, got %d", recSecure.Code)
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

	// 11. Test Invalid Channel returns 404
	reqInvalid := httptest.NewRequest(http.MethodGet, "/invalid-channel/", nil)
	recInvalid := httptest.NewRecorder()
	ctrl.router.ServeHTTP(recInvalid, reqInvalid)
	if recInvalid.Code != http.StatusNotFound {
		t.Errorf("expected 404 for invalid channel, got %d", recInvalid.Code)
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
	var succ []string
	if len(req.UIDs) > 0 {
		succ = req.UIDs
	} else if req.UID != "" {
		succ = []string{req.UID}
	}
	return &rfidclient.WriteAFIResponse{
		Data: rfidclient.WriteAFIResponseData{
			BaseRFIDPayload: rfidclient.BaseRFIDPayload{
				Event: rfidclient.EventWriteAFIResponse,
			},
			Status:         "ok",
			UID:            req.UID,
			UIDs:           req.UIDs,
			SuccessfulUIDs: succ,
			AFI:            req.AFI,
		},
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
		Data: rfidclient.TagReadData{
			UID:    "E004015011112222",
			ItemID: "ITEM-SUCCESS-1",
			AFI:    "07",
			BaseRFIDPayload: rfidclient.BaseRFIDPayload{
				ReaderID: "reader-01",
			},
		},
	})

	// 1. Post unsecure
	reqUnsecure := httptest.NewRequest(http.MethodPost, "/rfid-ch/loan/afi/unsecure", nil)
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
	if mockCli.writeCount == 0 {
		t.Errorf("expected write calls on mock client, got write=%d", mockCli.writeCount)
	}

	// 2. Post secure
	reqSecure := httptest.NewRequest(http.MethodPost, "/rfid-ch/loan/afi/secure", nil)
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
		Data: rfidclient.TagReadData{
			UID:    "E004015099998888",
			ItemID: "ITEM-ERR-1",
			AFI:    "07",
			BaseRFIDPayload: rfidclient.BaseRFIDPayload{
				ReaderID: "reader-01",
			},
		},
	})

	// Post unsecure when write fails
	reqUnsecure := httptest.NewRequest(http.MethodPost, "/rfid-ch/loan/afi/unsecure", nil)
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
		Data: rfidclient.TagReadData{
			UID:    "E004015011223344",
			ItemID: "ITEM-ASYNC-1",
			AFI:    "07",
			BaseRFIDPayload: rfidclient.BaseRFIDPayload{
				ReaderID: "reader-01",
			},
		},
	})

	sseChan := hub.Subscribe()

	// Simulate successful asynchronous WriteAFIResponse
	resp := &rfidclient.WriteAFIResponse{
		Data: rfidclient.WriteAFIResponseData{
			BaseRFIDPayload: rfidclient.BaseRFIDPayload{
				Event: rfidclient.EventWriteAFIResponse,
			},
			Status: "ok",
			UID:    "E004015011223344",
			AFI:    "C2",
		},
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
		Data: rfidclient.TagReadData{
			UID:    "E00401080AFD252F",
			ItemID: "ITEM-ERR-ASYNC",
			AFI:    "07",
			BaseRFIDPayload: rfidclient.BaseRFIDPayload{
				ReaderID: "reader-01",
			},
		},
	})

	sseChan := hub.Subscribe()

	// Simulate failed asynchronous WriteAFIResponse (e.g. ISO 149 / 0x0F error)
	errText := "Failed to write AFI byte 0xC2 (status: 149 (Iso error (149)), ISO Error: 0x0F (Unknown error))"
	resp := &rfidclient.WriteAFIResponse{
		Data: rfidclient.WriteAFIResponseData{
			BaseRFIDPayload: rfidclient.BaseRFIDPayload{
				Event: rfidclient.EventWriteAFIResponse,
			},
			Status: "error",
			Error:  errText,
			UID:    "E00401080AFD252F",
			AFI:    "C2",
		},
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
			Data: rfidclient.WriteAFIResponseData{
				Status: "error",
				Error:  "ISO error: 149 / 0x0F write protection failed",
			},
		},
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

	ctrl.HandleTagRead(&rfidclient.TagReadEvent{
		Data: rfidclient.TagReadData{
			UID:    "E00401080AFD252F",
			ItemID: "ITEM-SYNC-ERR",
			AFI:    "07",
			BaseRFIDPayload: rfidclient.BaseRFIDPayload{
				ReaderID: "reader-01",
			},
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/rfid-ch/loan/afi/unsecure", nil)
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
		Channels:      []string{"rfid-ch"},
		DefaultReader: "",
		Hub:           hub,
		RFIDClient:    mockCli,
	})
	if err != nil {
		t.Fatalf("failed to create web controller: %v", err)
	}

	// Tag scanned from reader 'rfid:reader:desk-antenna-01'
	ctrl.HandleTagRead(&rfidclient.TagReadEvent{
		Data: rfidclient.TagReadData{
			UID:    "E004015099887766",
			ItemID: "ITEM-TARGETED-1",
			AFI:    "07",
			BaseRFIDPayload: rfidclient.BaseRFIDPayload{
				ReaderID: "rfid:reader:desk-antenna-01",
			},
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/rfid-ch/loan/afi/unsecure", nil)
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
		Channels:      []string{"rfid-ch"},
		DefaultReader: "reader-01",
		Hub:           hub,
		RFIDClient:    mockCli,
	})
	if err != nil {
		t.Fatalf("failed to create web controller: %v", err)
	}

	// Add two tags with AFI 07 (Gesichert)
	ctrl.HandleTagRead(&rfidclient.TagReadEvent{
		Data: rfidclient.TagReadData{
			UID:    "TAG-01",
			ItemID: "BOOK-01",
			AFI:    "07",
			BaseRFIDPayload: rfidclient.BaseRFIDPayload{
				ReaderID: "reader-01",
			},
		},
	})
	ctrl.HandleTagRead(&rfidclient.TagReadEvent{
		Data: rfidclient.TagReadData{
			UID:    "TAG-02",
			ItemID: "BOOK-02",
			AFI:    "07",
			BaseRFIDPayload: rfidclient.BaseRFIDPayload{
				ReaderID: "reader-01",
			},
		},
	})

	// 1. Target single tag TAG-01 with unsecure
	req := httptest.NewRequest(http.MethodPost, "/rfid-ch/loan/afi/unsecure?uid=TAG-01", nil)
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
	reqSingle := httptest.NewRequest(http.MethodPost, "/rfid-ch/loan/afi/single?uid=TAG-01&action=secure", nil)
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
			if slices.Contains(req.UIDs, "TAG-02") || req.UID == "TAG-02" {
				if len(req.UIDs) > 1 {
					return &rfidclient.WriteAFIResponse{
						Data: rfidclient.WriteAFIResponseData{
							BaseRFIDPayload: rfidclient.BaseRFIDPayload{Event: rfidclient.EventWriteAFIResponse},
							Status:          "partial",
							UIDs:            req.UIDs,
							SuccessfulUIDs:  []string{"TAG-01"},
							FailedUIDs:      map[string]string{"TAG-02": "Antenna communication timeout"},
							Results: []rfidclient.TagAFIResult{
								{UID: "TAG-01", Status: "ok"},
								{UID: "TAG-02", Status: "error", Error: "Antenna communication timeout"},
							},
							AFI: req.AFI,
						},
					}, nil
				}
				return &rfidclient.WriteAFIResponse{
					Data: rfidclient.WriteAFIResponseData{
						BaseRFIDPayload: rfidclient.BaseRFIDPayload{Event: rfidclient.EventWriteAFIResponse},
						Status:          "error",
						Error:           "Antenna communication timeout",
						UID:             req.UID,
						AFI:             req.AFI,
					},
				}, nil
			}
			return &rfidclient.WriteAFIResponse{
				Data: rfidclient.WriteAFIResponseData{
					BaseRFIDPayload: rfidclient.BaseRFIDPayload{Event: rfidclient.EventWriteAFIResponse},
					Status:          "ok",
					UID:             req.UID,
					UIDs:            req.UIDs,
					SuccessfulUIDs:  req.UIDs,
					AFI:             req.AFI,
				},
			}, nil
		},
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

	ctrl.HandleTagRead(&rfidclient.TagReadEvent{
		Data: rfidclient.TagReadData{
			UID:             "TAG-01",
			ItemID:          "BOOK-SUCCESS",
			AFI:             "07",
			BaseRFIDPayload: rfidclient.BaseRFIDPayload{ReaderID: "reader-01"},
		},
	})
	ctrl.HandleTagRead(&rfidclient.TagReadEvent{
		Data: rfidclient.TagReadData{
			UID:             "TAG-02",
			ItemID:          "BOOK-FAIL",
			AFI:             "07",
			BaseRFIDPayload: rfidclient.BaseRFIDPayload{ReaderID: "reader-01"},
		},
	})

	// Execute batch unsecure
	req := httptest.NewRequest(http.MethodPost, "/rfid-ch/loan/afi/unsecure", nil)
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
	if !strings.Contains(body, "1 von 2 Medien fehlgeschlagen") {
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
	reqRetry := httptest.NewRequest(http.MethodPost, "/rfid-ch/loan/afi/unsecure?uid=TAG-02", nil)
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

func TestController_LoanAFIFailurePreservedOnContinuousRescans(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hub := desk.NewHub(50)
	hub.Start(ctx)
	defer hub.Close()

	failUID := "E00401080AFD252F"
	failErrMsg := "Failed to write AFI byte 0xC2 (status: 149 (Iso error (149)), ISO Error: 0x0F (Unknown error))"

	mockCli := &mockRFIDClient{
		writeAFIHandler: func(ctx context.Context, target string, req rfidclient.WriteAFIRequest) (*rfidclient.WriteAFIResponse, error) {
			return &rfidclient.WriteAFIResponse{
				Data: rfidclient.WriteAFIResponseData{
					BaseRFIDPayload: rfidclient.BaseRFIDPayload{Event: rfidclient.EventWriteAFIResponse},
					Status:          "partial",
					UIDs:            req.UIDs,
					SuccessfulUIDs:  []string{"E00401080AD4B4E7", "E004010843BC4D8B", "E004010843BC4D8C"},
					FailedUIDs:      map[string]string{failUID: failErrMsg},
					Results: []rfidclient.TagAFIResult{
						{UID: "E00401080AD4B4E7", Status: "ok"},
						{UID: "E004010843BC4D8B", Status: "ok"},
						{UID: "E004010843BC4D8C", Status: "ok"},
						{UID: failUID, Status: "error", Error: failErrMsg},
					},
					AFI: req.AFI,
				},
			}, nil
		},
	}

	ctrl, err := NewController(Config{
		Addr:          "localhost:8762",
		ClientName:    "test-rfid-desk",
		Channels:      []string{"rfid"},
		DefaultReader: "reader-01",
		Hub:           hub,
		RFIDClient:    mockCli,
	})
	if err != nil {
		t.Fatalf("failed to create web controller: %v", err)
	}

	// 1. Initial tag scans
	tags := []struct {
		uid    string
		itemID string
		afi    string
	}{
		{"E00401080AD4B4E7", "FHNW000018121", "07"},
		{"E004010843BC4D8B", "", "07"},
		{"E004010843BC4D8C", "", "07"},
		{failUID, "A1001141486", "07"},
	}
	for _, tg := range tags {
		ctrl.HandleTagRead(&rfidclient.TagReadEvent{
			BaseEvent: wsbrokerclient.BaseEvent{Channel: "rfid"},
			Data: rfidclient.TagReadData{
				UID:             tg.uid,
				ItemID:          tg.itemID,
				AFI:             tg.afi,
				BaseRFIDPayload: rfidclient.BaseRFIDPayload{ReaderID: "reader-01"},
			},
		})
	}

	// 2. Perform batch unsecure to C2
	req := httptest.NewRequest(http.MethodPost, "/rfid/loan/afi/unsecure", nil)
	rec := httptest.NewRecorder()
	ctrl.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 from loan/afi/unsecure, got %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "1 von 4 Medien fehlgeschlagen") {
		t.Errorf("expected error summary in response, got: %s", body)
	}
	if !strings.Contains(body, "A1001141486") || !strings.Contains(body, "Iso error (149)") {
		t.Errorf("expected failed item details in response, got: %s", body)
	}

	// 3. Simulate 5 continuous scan cycles of the tags on reader
	for cycle := 1; cycle <= 5; cycle++ {
		// Re-scan tags in varying order
		for i := len(tags) - 1; i >= 0; i-- {
			tg := tags[i]
			currentAFI := "C2"
			if tg.uid == failUID {
				currentAFI = "07"
			}
			ctrl.HandleTagRead(&rfidclient.TagReadEvent{
				BaseEvent: wsbrokerclient.BaseEvent{Channel: "rfid"},
				Data: rfidclient.TagReadData{
					UID:             tg.uid,
					ItemID:          tg.itemID,
					AFI:             currentAFI,
					BaseRFIDPayload: rfidclient.BaseRFIDPayload{ReaderID: "reader-01"},
				},
			})
		}

		// Verify loan table fragment still contains error row and failed status
		reqFrag := httptest.NewRequest(http.MethodGet, "/rfid/loan/tags", nil)
		recFrag := httptest.NewRecorder()
		ctrl.router.ServeHTTP(recFrag, reqFrag)
		fragBody := recFrag.Body.String()

		if !strings.Contains(fragBody, "1 von 4 Medien fehlgeschlagen") {
			t.Errorf("cycle %d: expected error summary in loan tags fragment, got: %s", cycle, fragBody)
		}
		if !strings.Contains(fragBody, "A1001141486") || !strings.Contains(fragBody, "Iso error (149)") {
			t.Errorf("cycle %d: expected failed item details in loan tags fragment, got: %s", cycle, fragBody)
		}
		if !strings.Contains(fragBody, `badge-error`) || !strings.Contains(fragBody, `Fehler`) {
			t.Errorf("cycle %d: expected error badge on failed row, got: %s", cycle, fragBody)
		}

		// Also verify desk fragment renders the error badge and reason
		reqDesk := httptest.NewRequest(http.MethodGet, "/rfid/", nil)
		recDesk := httptest.NewRecorder()
		ctrl.router.ServeHTTP(recDesk, reqDesk)
		deskBody := recDesk.Body.String()
		if !strings.Contains(deskBody, "Fehler") || !strings.Contains(deskBody, "Iso error (149)") {
			t.Errorf("cycle %d: expected desk view to display error badge & reason, got: %s", cycle, deskBody)
		}
	}
}

func TestBatchAFIWrite_OnlyDifferingTagsTargeted(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hub := desk.NewHub(50)
	hub.Start(ctx)
	defer hub.Close()

	var targetedUIDs []string
	mockCli := &mockRFIDClient{
		writeAFIHandler: func(ctx context.Context, target string, req rfidclient.WriteAFIRequest) (*rfidclient.WriteAFIResponse, error) {
			if len(req.UIDs) > 0 {
				targetedUIDs = append(targetedUIDs, req.UIDs...)
			} else if req.UID != "" {
				targetedUIDs = append(targetedUIDs, req.UID)
			}
			return nil, errors.New("write afi request timed out waiting for response from reader hardware")
		},
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

	// 4 tags on reader: Top one is 07 (Gesichert), bottom 3 are already C2 (Entsichert)
	ctrl.HandleTagRead(&rfidclient.TagReadEvent{
		Data: rfidclient.TagReadData{
			UID:             "TAG-TOP",
			ItemID:          "BOOK-TOP",
			AFI:             "07",
			BaseRFIDPayload: rfidclient.BaseRFIDPayload{ReaderID: "reader-01"},
		},
	})
	ctrl.HandleTagRead(&rfidclient.TagReadEvent{
		Data: rfidclient.TagReadData{
			UID:             "TAG-MID-1",
			ItemID:          "BOOK-MID-1",
			AFI:             "C2",
			BaseRFIDPayload: rfidclient.BaseRFIDPayload{ReaderID: "reader-01"},
		},
	})
	ctrl.HandleTagRead(&rfidclient.TagReadEvent{
		Data: rfidclient.TagReadData{
			UID:             "TAG-MID-2",
			ItemID:          "",
			AFI:             "C2",
			BaseRFIDPayload: rfidclient.BaseRFIDPayload{ReaderID: "reader-01"},
		},
	})
	ctrl.HandleTagRead(&rfidclient.TagReadEvent{
		Data: rfidclient.TagReadData{
			UID:             "TAG-BOT",
			ItemID:          "",
			AFI:             "C2",
			BaseRFIDPayload: rfidclient.BaseRFIDPayload{ReaderID: "reader-01"},
		},
	})

	// Execute batch unsecure (to C2)
	req := httptest.NewRequest(http.MethodPost, "/rfid-ch/loan/afi/unsecure", nil)
	rec := httptest.NewRecorder()
	ctrl.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	// Verify only TAG-TOP was targeted because other 3 were already C2
	if len(targetedUIDs) != 1 || targetedUIDs[0] != "TAG-TOP" {
		t.Errorf("expected only TAG-TOP to be targeted, got %v", targetedUIDs)
	}

	// Verify only TAG-TOP has error status, others remain clean C2
	active := hub.GetActiveTags("reader-01")
	for _, item := range active {
		if item.UID == "TAG-TOP" {
			if item.AFIWriteStatus != "error" || item.AFIWriteError == "" {
				t.Errorf("expected TAG-TOP to have error, got status=%s, error=%s", item.AFIWriteStatus, item.AFIWriteError)
			}
		} else {
			if item.AFIWriteStatus == "error" || item.AFIWriteError != "" {
				t.Errorf("expected %s to have no error, got status=%s, error=%s", item.UID, item.AFIWriteStatus, item.AFIWriteError)
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
			if slices.Contains(req.UIDs, "TAG-ERR") || req.UID == "TAG-ERR" {
				if len(req.UIDs) > 1 {
					return &rfidclient.WriteAFIResponse{
						Data: rfidclient.WriteAFIResponseData{
							Status:         "partial",
							UIDs:           req.UIDs,
							SuccessfulUIDs: []string{"TAG-OK"},
							FailedUIDs:     map[string]string{"TAG-ERR": "Write lock active"},
							Results: []rfidclient.TagAFIResult{
								{UID: "TAG-OK", Status: "ok"},
								{UID: "TAG-ERR", Status: "error", Error: "Write lock active"},
							},
							AFI: req.AFI,
						},
					}, nil
				}
				return &rfidclient.WriteAFIResponse{
					Data: rfidclient.WriteAFIResponseData{
						Status: "error",
						Error:  "Write lock active",
						UID:    req.UID,
						AFI:    req.AFI,
					},
				}, nil
			}
			return &rfidclient.WriteAFIResponse{
				Data: rfidclient.WriteAFIResponseData{
					Status:         "ok",
					UID:            req.UID,
					UIDs:           req.UIDs,
					SuccessfulUIDs: req.UIDs,
					AFI:            req.AFI,
				},
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
		Data: rfidclient.TagReadData{
			UID:             "TAG-OK",
			ItemID:          "BOOK-OK",
			AFI:             "07",
			BaseRFIDPayload: rfidclient.BaseRFIDPayload{ReaderID: "reader-01"},
		},
	})
	ctrl.HandleTagRead(&rfidclient.TagReadEvent{
		Data: rfidclient.TagReadData{
			UID:             "TAG-ERR",
			ItemID:          "BOOK-ERR",
			AFI:             "07",
			BaseRFIDPayload: rfidclient.BaseRFIDPayload{ReaderID: "reader-01"},
		},
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

func TestController_ChannelIsolationAndCardDisplay(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hub := desk.NewHub(50)
	hub.Start(ctx)
	defer hub.Close()

	ctrl, err := NewController(Config{
		Addr:          "localhost:8762",
		ClientName:    "test-rfid-desk",
		Channels:      []string{"desk-a", "desk-b"},
		DefaultReader: "reader-01",
		Hub:           hub,
	})
	if err != nil {
		t.Fatalf("failed to create web controller: %v", err)
	}

	tagA := &rfidclient.TagReadEvent{
		BaseEvent: wsbrokerclient.BaseEvent{
			Channel: "desk-a",
		},
		Data: rfidclient.TagReadData{
			UID:    "UID-AAA-111",
			ItemID: "BOOK-AAA",
			AFI:    "07",
			BaseRFIDPayload: rfidclient.BaseRFIDPayload{
				ReaderID: "reader-01",
			},
		},
	}
	tagB := &rfidclient.TagReadEvent{
		BaseEvent: wsbrokerclient.BaseEvent{
			Channel: "desk-b",
		},
		Data: rfidclient.TagReadData{
			UID:    "UID-BBB-222",
			ItemID: "BOOK-BBB",
			AFI:    "07",
			BaseRFIDPayload: rfidclient.BaseRFIDPayload{
				ReaderID: "reader-01",
			},
		},
	}

	ctrl.HandleTagRead(tagA)
	ctrl.HandleTagRead(tagB)

	// 1. Verify /desk-a/ displays only tagA and shows "desk-a" as Channel card
	recA := httptest.NewRecorder()
	reqA := httptest.NewRequest(http.MethodGet, "/desk-a/", nil)
	ctrl.router.ServeHTTP(recA, reqA)
	if recA.Code != http.StatusOK {
		t.Fatalf("expected 200 for /desk-a/, got %d", recA.Code)
	}
	bodyA := recA.Body.String()
	if !strings.Contains(bodyA, "BOOK-AAA") || !strings.Contains(bodyA, "UID-AAA-111") {
		t.Errorf("expected /desk-a/ to contain BOOK-AAA / UID-AAA-111, got: %s", bodyA)
	}
	if strings.Contains(bodyA, "BOOK-BBB") || strings.Contains(bodyA, "UID-BBB-222") {
		t.Errorf("/desk-a/ should NOT contain tags from desk-b (BOOK-BBB), got: %s", bodyA)
	}
	if strings.Contains(bodyA, "Abonnierte Channels") {
		t.Errorf("/desk-a/ should not contain 'Abonnierte Channels'")
	}
	if !strings.Contains(bodyA, "desk-a") {
		t.Errorf("/desk-a/ should contain 'desk-a' in Channel card")
	}

	// 2. Verify /desk-b/ displays only tagB and shows "desk-b" as Channel card
	recB := httptest.NewRecorder()
	reqB := httptest.NewRequest(http.MethodGet, "/desk-b/", nil)
	ctrl.router.ServeHTTP(recB, reqB)
	if recB.Code != http.StatusOK {
		t.Fatalf("expected 200 for /desk-b/, got %d", recB.Code)
	}
	bodyB := recB.Body.String()
	if !strings.Contains(bodyB, "BOOK-BBB") || !strings.Contains(bodyB, "UID-BBB-222") {
		t.Errorf("expected /desk-b/ to contain BOOK-BBB / UID-BBB-222, got: %s", bodyB)
	}
	if strings.Contains(bodyB, "BOOK-AAA") || strings.Contains(bodyB, "UID-AAA-111") {
		t.Errorf("/desk-b/ should NOT contain tags from desk-a (BOOK-AAA), got: %s", bodyB)
	}

	// 3. Verify /desk-a/loan displays only tagA
	recLoanA := httptest.NewRecorder()
	reqLoanA := httptest.NewRequest(http.MethodGet, "/desk-a/loan", nil)
	ctrl.router.ServeHTTP(recLoanA, reqLoanA)
	if recLoanA.Code != http.StatusOK {
		t.Fatalf("expected 200 for /desk-a/loan, got %d", recLoanA.Code)
	}
	bodyLoanA := recLoanA.Body.String()
	if !strings.Contains(bodyLoanA, "BOOK-AAA") {
		t.Errorf("expected /desk-a/loan to contain BOOK-AAA, got: %s", bodyLoanA)
	}
	if strings.Contains(bodyLoanA, "BOOK-BBB") {
		t.Errorf("/desk-a/loan should NOT contain BOOK-BBB, got: %s", bodyLoanA)
	}

	// 4. Verify /desk-b/loan displays only tagB
	recLoanB := httptest.NewRecorder()
	reqLoanB := httptest.NewRequest(http.MethodGet, "/desk-b/loan", nil)
	ctrl.router.ServeHTTP(recLoanB, reqLoanB)
	if recLoanB.Code != http.StatusOK {
		t.Fatalf("expected 200 for /desk-b/loan, got %d", recLoanB.Code)
	}
	bodyLoanB := recLoanB.Body.String()
	if !strings.Contains(bodyLoanB, "BOOK-BBB") {
		t.Errorf("expected /desk-b/loan to contain BOOK-BBB, got: %s", bodyLoanB)
	}
	if strings.Contains(bodyLoanB, "BOOK-AAA") {
		t.Errorf("/desk-b/loan should NOT contain BOOK-AAA, got: %s", bodyLoanB)
	}

	// 5. Clear tags on /desk-a/tags/clear
	recClearA := httptest.NewRecorder()
	reqClearA := httptest.NewRequest(http.MethodPost, "/desk-a/tags/clear", nil)
	ctrl.router.ServeHTTP(recClearA, reqClearA)
	if recClearA.Code != http.StatusOK {
		t.Fatalf("expected 200 for /desk-a/tags/clear, got %d", recClearA.Code)
	}

	// Verify desk-a is empty, desk-b still has tagB
	recAAfter := httptest.NewRecorder()
	ctrl.router.ServeHTTP(recAAfter, reqA)
	if strings.Contains(recAAfter.Body.String(), "BOOK-AAA") {
		t.Errorf("expected BOOK-AAA to be cleared from /desk-a/, got: %s", recAAfter.Body.String())
	}

	recBAfter := httptest.NewRecorder()
	ctrl.router.ServeHTTP(recBAfter, reqB)
	if !strings.Contains(recBAfter.Body.String(), "BOOK-BBB") {
		t.Errorf("expected BOOK-BBB to remain on /desk-b/ after clearing desk-a, got: %s", recBAfter.Body.String())
	}
}

func TestController_LoanAFI_ClusteredByReader(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hub := desk.NewHub(50)
	hub.Start(ctx)
	defer hub.Close()

	var writeCallsMu sync.Mutex
	type writeCallRecord struct {
		Target string
		UIDs   []string
		AFI    string
	}
	var recordedCalls []writeCallRecord

	mockCli := &mockRFIDClient{
		writeAFIHandler: func(ctx context.Context, target string, req rfidclient.WriteAFIRequest) (*rfidclient.WriteAFIResponse, error) {
			writeCallsMu.Lock()
			recordedCalls = append(recordedCalls, writeCallRecord{
				Target: target,
				UIDs:   req.UIDs,
				AFI:    req.AFI,
			})
			writeCallsMu.Unlock()

			return &rfidclient.WriteAFIResponse{
				Data: rfidclient.WriteAFIResponseData{
					BaseRFIDPayload: rfidclient.BaseRFIDPayload{Event: rfidclient.EventWriteAFIResponse},
					Status:          "ok",
					UIDs:            req.UIDs,
					SuccessfulUIDs:  req.UIDs,
					AFI:             req.AFI,
				},
			}, nil
		},
	}

	ctrl, err := NewController(Config{
		Addr:          "localhost:8762",
		ClientName:    "test-rfid-desk",
		Channels:      []string{"rfid-ch"},
		DefaultReader: "reader-default",
		Hub:           hub,
		RFIDClient:    mockCli,
	})
	if err != nil {
		t.Fatalf("failed to create web controller: %v", err)
	}

	// Scan 2 tags on reader-01 and 2 tags on reader-02
	ctrl.HandleTagRead(&rfidclient.TagReadEvent{
		BaseEvent: wsbrokerclient.BaseEvent{Channel: "rfid-ch"},
		Data: rfidclient.TagReadData{
			UID:             "TAG-R1-A",
			ItemID:          "BOOK-R1-A",
			AFI:             "07",
			BaseRFIDPayload: rfidclient.BaseRFIDPayload{ReaderID: "reader-01"},
		},
	})
	ctrl.HandleTagRead(&rfidclient.TagReadEvent{
		BaseEvent: wsbrokerclient.BaseEvent{Channel: "rfid-ch"},
		Data: rfidclient.TagReadData{
			UID:             "TAG-R1-B",
			ItemID:          "BOOK-R1-B",
			AFI:             "07",
			BaseRFIDPayload: rfidclient.BaseRFIDPayload{ReaderID: "reader-01"},
		},
	})
	ctrl.HandleTagRead(&rfidclient.TagReadEvent{
		BaseEvent: wsbrokerclient.BaseEvent{Channel: "rfid-ch"},
		Data: rfidclient.TagReadData{
			UID:             "TAG-R2-A",
			ItemID:          "BOOK-R2-A",
			AFI:             "07",
			BaseRFIDPayload: rfidclient.BaseRFIDPayload{ReaderID: "reader-02"},
		},
	})
	ctrl.HandleTagRead(&rfidclient.TagReadEvent{
		BaseEvent: wsbrokerclient.BaseEvent{Channel: "rfid-ch"},
		Data: rfidclient.TagReadData{
			UID:             "TAG-R2-B",
			ItemID:          "BOOK-R2-B",
			AFI:             "07",
			BaseRFIDPayload: rfidclient.BaseRFIDPayload{ReaderID: "reader-02"},
		},
	})

	// 1. Web desk unsecure operation
	reqUI := httptest.NewRequest(http.MethodPost, "/rfid-ch/loan/afi/unsecure", nil)
	recUI := httptest.NewRecorder()
	ctrl.router.ServeHTTP(recUI, reqUI)

	if recUI.Code != http.StatusOK {
		t.Fatalf("expected 200 from /loan/afi/unsecure, got %d", recUI.Code)
	}

	writeCallsMu.Lock()
	if len(recordedCalls) != 2 {
		t.Fatalf("expected 2 clustered WriteAFI calls (one per reader), got %d", len(recordedCalls))
	}
	for _, call := range recordedCalls {
		if call.Target == "reader-01" {
			if len(call.UIDs) != 2 || !slices.Contains(call.UIDs, "TAG-R1-A") || !slices.Contains(call.UIDs, "TAG-R1-B") {
				t.Errorf("reader-01 received unexpected UIDs: %v", call.UIDs)
			}
			if slices.Contains(call.UIDs, "TAG-R2-A") || slices.Contains(call.UIDs, "TAG-R2-B") {
				t.Errorf("reader-01 received tags from reader-02: %v", call.UIDs)
			}
		} else if call.Target == "reader-02" {
			if len(call.UIDs) != 2 || !slices.Contains(call.UIDs, "TAG-R2-A") || !slices.Contains(call.UIDs, "TAG-R2-B") {
				t.Errorf("reader-02 received unexpected UIDs: %v", call.UIDs)
			}
			if slices.Contains(call.UIDs, "TAG-R1-A") || slices.Contains(call.UIDs, "TAG-R1-B") {
				t.Errorf("reader-02 received tags from reader-01: %v", call.UIDs)
			}
		} else {
			t.Errorf("unexpected target reader in write call: %s", call.Target)
		}
	}
	writeCallsMu.Unlock()

	// 2. REST API secure operation with specified UIDs across multiple readers
	writeCallsMu.Lock()
	recordedCalls = nil
	writeCallsMu.Unlock()

	reqAPI := httptest.NewRequest(http.MethodPost, "/api/v1/loan/afi", strings.NewReader(`{"action":"secure","uids":["TAG-R1-A","TAG-R2-B"]}`))
	reqAPI.Header.Set("Content-Type", "application/json")
	recAPI := httptest.NewRecorder()
	ctrl.router.ServeHTTP(recAPI, reqAPI)

	if recAPI.Code != http.StatusOK {
		t.Fatalf("expected 200 from /api/v1/loan/afi, got %d", recAPI.Code)
	}

	writeCallsMu.Lock()
	if len(recordedCalls) != 2 {
		t.Fatalf("expected 2 clustered API WriteAFI calls, got %d", len(recordedCalls))
	}
	for _, call := range recordedCalls {
		if call.Target == "reader-01" {
			if len(call.UIDs) != 1 || call.UIDs[0] != "TAG-R1-A" {
				t.Errorf("reader-01 API call received unexpected UIDs: %v", call.UIDs)
			}
		} else if call.Target == "reader-02" {
			if len(call.UIDs) != 1 || call.UIDs[0] != "TAG-R2-B" {
				t.Errorf("reader-02 API call received unexpected UIDs: %v", call.UIDs)
			}
		} else {
			t.Errorf("unexpected target reader in API write call: %s", call.Target)
		}
	}
	writeCallsMu.Unlock()
}

func TestController_LoanAFI_ClusteredBySourceAndReaderID(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hub := desk.NewHub(50)
	hub.Start(ctx)
	defer hub.Close()

	var writeCallsMu sync.Mutex
	type writeCallRecord struct {
		Target   string
		ReaderID string
		UIDs     []string
		AFI      string
	}
	var recordedCalls []writeCallRecord

	mockCli := &mockRFIDClient{
		writeAFIHandler: func(ctx context.Context, target string, req rfidclient.WriteAFIRequest) (*rfidclient.WriteAFIResponse, error) {
			writeCallsMu.Lock()
			recordedCalls = append(recordedCalls, writeCallRecord{
				Target:   target,
				ReaderID: req.ReaderID,
				UIDs:     req.UIDs,
				AFI:      req.AFI,
			})
			writeCallsMu.Unlock()

			return &rfidclient.WriteAFIResponse{
				Data: rfidclient.WriteAFIResponseData{
					BaseRFIDPayload: rfidclient.BaseRFIDPayload{Event: rfidclient.EventWriteAFIResponse},
					Status:          "ok",
					UIDs:            req.UIDs,
					SuccessfulUIDs:  req.UIDs,
					AFI:             req.AFI,
				},
			}, nil
		},
	}

	ctrl, err := NewController(Config{
		Addr:          "localhost:8762",
		ClientName:    "test-rfid-desk",
		Channels:      []string{"rfid-ch"},
		DefaultReader: "rfid:reader:01",
		Hub:           hub,
		RFIDClient:    mockCli,
	})
	if err != nil {
		t.Fatalf("failed to create web controller: %v", err)
	}

	// 2 tags on daemon-A (gate-01) and 2 tags on daemon-B (gate-02)
	ctrl.HandleTagRead(&rfidclient.TagReadEvent{
		BaseEvent: wsbrokerclient.BaseEvent{Source: "rfid:daemon:01", Channel: "rfid-ch"},
		Data: rfidclient.TagReadData{
			UID:             "TAG-DA-1",
			ItemID:          "BOOK-DA-1",
			AFI:             "07",
			BaseRFIDPayload: rfidclient.BaseRFIDPayload{ReaderID: "gate-01"},
		},
	})
	ctrl.HandleTagRead(&rfidclient.TagReadEvent{
		BaseEvent: wsbrokerclient.BaseEvent{Source: "rfid:daemon:01", Channel: "rfid-ch"},
		Data: rfidclient.TagReadData{
			UID:             "TAG-DA-2",
			ItemID:          "BOOK-DA-2",
			AFI:             "07",
			BaseRFIDPayload: rfidclient.BaseRFIDPayload{ReaderID: "gate-01"},
		},
	})
	ctrl.HandleTagRead(&rfidclient.TagReadEvent{
		BaseEvent: wsbrokerclient.BaseEvent{Source: "rfid:daemon:02", Channel: "rfid-ch"},
		Data: rfidclient.TagReadData{
			UID:             "TAG-DB-1",
			ItemID:          "BOOK-DB-1",
			AFI:             "07",
			BaseRFIDPayload: rfidclient.BaseRFIDPayload{ReaderID: "gate-02"},
		},
	})

	reqUI := httptest.NewRequest(http.MethodPost, "/rfid-ch/loan/afi/unsecure", nil)
	recUI := httptest.NewRecorder()
	ctrl.router.ServeHTTP(recUI, reqUI)

	if recUI.Code != http.StatusOK {
		t.Fatalf("expected 200 from /loan/afi/unsecure, got %d", recUI.Code)
	}

	writeCallsMu.Lock()
	if len(recordedCalls) != 2 {
		t.Fatalf("expected 2 clustered WriteAFI calls, got %d", len(recordedCalls))
	}
	for _, call := range recordedCalls {
		if call.Target == "rfid:daemon:01" {
			if call.ReaderID != "gate-01" {
				t.Errorf("expected readerId gate-01 on target rfid:daemon:01, got: %s", call.ReaderID)
			}
			if len(call.UIDs) != 2 || !slices.Contains(call.UIDs, "TAG-DA-1") || !slices.Contains(call.UIDs, "TAG-DA-2") {
				t.Errorf("rfid:daemon:01 received unexpected UIDs: %v", call.UIDs)
			}
		} else if call.Target == "rfid:daemon:02" {
			if call.ReaderID != "gate-02" {
				t.Errorf("expected readerId gate-02 on target rfid:daemon:02, got: %s", call.ReaderID)
			}
			if len(call.UIDs) != 1 || call.UIDs[0] != "TAG-DB-1" {
				t.Errorf("rfid:daemon:02 received unexpected UIDs: %v", call.UIDs)
			}
		} else {
			t.Errorf("unexpected target in write call: %s", call.Target)
		}
	}
	writeCallsMu.Unlock()
}

func TestController_HeaderFooterTemplates(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hub := desk.NewHub(10)
	hub.Start(ctx)
	defer hub.Close()

	ctrl, err := NewController(Config{
		Addr:          "localhost:8762",
		ClientName:    "bib-client-01",
		WSBroker:      "wss://broker.example.com",
		Channels:      []string{"channel-1"},
		DefaultReader: "desk-reader",
		Hub:           hub,
	})
	if err != nil {
		t.Fatalf("failed to create web controller: %v", err)
	}

	// 1. Index Page Header & Footer verification
	reqIndex := httptest.NewRequest(http.MethodGet, "/", nil)
	recIndex := httptest.NewRecorder()
	ctrl.router.ServeHTTP(recIndex, reqIndex)
	if recIndex.Code != http.StatusOK {
		t.Fatalf("expected 200 for index, got %d", recIndex.Code)
	}
	bodyIndex := recIndex.Body.String()
	if !strings.Contains(bodyIndex, "Channel-Auswahl") {
		t.Errorf("expected index header to have 'Channel-Auswahl', got: %s", bodyIndex)
	}
	if !strings.Contains(bodyIndex, "RFID Desk &copy; 2026 info-age GmbH, Basel") {
		t.Errorf("expected index footer copyright, got: %s", bodyIndex)
	}
	if !strings.Contains(bodyIndex, "Webserver: <span class=\"server-addr\">localhost:8762</span>") {
		t.Errorf("expected index footer server addr, got: %s", bodyIndex)
	}

	// 2. Desk Dashboard Page Header & Footer verification
	reqDesk := httptest.NewRequest(http.MethodGet, "/channel-1/", nil)
	recDesk := httptest.NewRecorder()
	ctrl.router.ServeHTTP(recDesk, reqDesk)
	if recDesk.Code != http.StatusOK {
		t.Fatalf("expected 200 for desk, got %d", recDesk.Code)
	}
	bodyDesk := recDesk.Body.String()
	if !strings.Contains(bodyDesk, "RFID Desk") || !strings.Contains(bodyDesk, "Channel: <strong>channel-1</strong>") {
		t.Errorf("expected desk header channel, got: %s", bodyDesk)
	}
	if !strings.Contains(bodyDesk, "RFID Desk &copy; 2026 info-age GmbH, Basel") {
		t.Errorf("expected desk footer copyright, got: %s", bodyDesk)
	}
	if !strings.Contains(bodyDesk, "nav-channels-btn") || !strings.Contains(bodyDesk, "sound-toggle") {
		t.Errorf("expected desk header buttons, got: %s", bodyDesk)
	}

	// 3. Loan Page Header & Footer verification
	reqLoan := httptest.NewRequest(http.MethodGet, "/channel-1/loan", nil)
	recLoan := httptest.NewRecorder()
	ctrl.router.ServeHTTP(recLoan, reqLoan)
	if recLoan.Code != http.StatusOK {
		t.Fatalf("expected 200 for loan, got %d", recLoan.Code)
	}
	bodyLoan := recLoan.Body.String()
	if !strings.Contains(bodyLoan, "Ausleihe &amp; Rückgabe") && !strings.Contains(bodyLoan, "Ausleihe & Rückgabe") {
		t.Errorf("expected loan header title, got: %s", bodyLoan)
	}
	if !strings.Contains(bodyLoan, "nav-desk-btn") || !strings.Contains(bodyLoan, "nav-channels-btn") {
		t.Errorf("expected loan header desk and channels buttons, got: %s", bodyLoan)
	}
	if !strings.Contains(bodyLoan, "RFID Desk &copy; 2026 info-age GmbH, Basel") {
		t.Errorf("expected loan footer copyright, got: %s", bodyLoan)
	}
}

func TestController_ISILLibraryNameAndFlyoutRendering(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	isilMap := map[string]*desk.ISILEntry{
		"CH-000050-X": {
			ISIL:                  "CH-000050-X",
			Status:                "aktiv",
			NameOriginal:          "Aargauer Kantonsbibliothek",
			Strasse:               "Aargauerplatz",
			Postleitzahl:          "5001",
			Ortschaft:             "Aarau",
			Kanton:                "Aargau",
			Telefon:               "+41 62 835 22 22",
			EMail:                 "kantonsbibliothek@ag.ch",
			Webseite:              "https://www.ag.ch/kantonsbibliothek",
			Institutionstyp:       "Kantonsbibliothek",
			Institutionskategorie: "Wissenschaftliche Bibliothek",
		},
	}

	hub := desk.NewHub(10, isilMap)
	hub.Start(ctx)
	defer hub.Close()

	ctrl, err := NewController(Config{
		Addr:          "localhost:8762",
		ClientName:    "bib-client-01",
		WSBroker:      "wss://broker.example.com",
		Channels:      []string{"all"},
		DefaultReader: "desk-reader",
		Hub:           hub,
		ISILList:      isilMap,
	})
	if err != nil {
		t.Fatalf("failed to create web controller: %v", err)
	}

	// Add tag with ISIL
	tag := &rfidclient.TagReadEvent{
		BaseEvent: wsbrokerclient.BaseEvent{
			Channel: "all",
		},
		Data: rfidclient.TagReadData{
			UID:     "E004015011223344",
			ItemID:  "BOOK-999",
			ISIL:    "CH-000050-X",
			Country: "CH",
			CRCOk:   true,
			BaseRFIDPayload: rfidclient.BaseRFIDPayload{
				ReaderID: "desk-reader",
			},
		},
	}
	hub.AddTag(tag)

	// Test desk view rendering
	reqDesk := httptest.NewRequest(http.MethodGet, "/all/", nil)
	recDesk := httptest.NewRecorder()
	ctrl.router.ServeHTTP(recDesk, reqDesk)
	if recDesk.Code != http.StatusOK {
		t.Fatalf("expected 200 for desk, got %d", recDesk.Code)
	}
	bodyDesk := recDesk.Body.String()
	if !strings.Contains(bodyDesk, "Aargauer Kantonsbibliothek") {
		t.Errorf("expected desk page to contain library name 'Aargauer Kantonsbibliothek', got: %s", bodyDesk)
	}
	if !strings.Contains(bodyDesk, "app-isil-flyout") {
		t.Errorf("expected desk page to contain 'app-isil-flyout', got: %s", bodyDesk)
	}
	if !strings.Contains(bodyDesk, "kantonsbibliothek@ag.ch") || !strings.Contains(bodyDesk, "Aargauerplatz") {
		t.Errorf("expected flyout details (email, address) in desk page, got: %s", bodyDesk)
	}

	// Test loan view rendering
	reqLoan := httptest.NewRequest(http.MethodGet, "/all/loan", nil)
	recLoan := httptest.NewRecorder()
	ctrl.router.ServeHTTP(recLoan, reqLoan)
	if recLoan.Code != http.StatusOK {
		t.Fatalf("expected 200 for loan, got %d", recLoan.Code)
	}
	bodyLoan := recLoan.Body.String()
	if !strings.Contains(bodyLoan, "Aargauer Kantonsbibliothek") {
		t.Errorf("expected loan page to contain library name 'Aargauer Kantonsbibliothek', got: %s", bodyLoan)
	}
	if !strings.Contains(bodyLoan, "app-isil-flyout") {
		t.Errorf("expected loan page to contain 'app-isil-flyout', got: %s", bodyLoan)
	}
}

func TestController_StaticPathRendering(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hub := desk.NewHub(10, nil)
	hub.Start(ctx)
	defer hub.Close()

	ctrl, err := NewController(Config{
		Addr:          "localhost:8762",
		ClientName:    "bib-client-01",
		WSBroker:      "wss://broker.example.com",
		Channels:      []string{"channel-1"},
		DefaultReader: "desk-reader",
		Hub:           hub,
	})
	if err != nil {
		t.Fatalf("failed to create web controller: %v", err)
	}

	// 1. Root Index should render StaticPath as "static"
	reqIndex := httptest.NewRequest(http.MethodGet, "/", nil)
	recIndex := httptest.NewRecorder()
	ctrl.router.ServeHTTP(recIndex, reqIndex)
	if recIndex.Code != http.StatusOK {
		t.Fatalf("expected 200 for index, got %d", recIndex.Code)
	}
	bodyIndex := recIndex.Body.String()
	if !strings.Contains(bodyIndex, `href="static/css/tailwind.min.css"`) {
		t.Errorf("expected index to use 'static/css/tailwind.min.css', got: %s", bodyIndex)
	}
	if !strings.Contains(bodyIndex, `src="static/img/icons/bolt.svg"`) {
		t.Errorf("expected index to use 'static/img/icons/bolt.svg', got: %s", bodyIndex)
	}
	if !strings.Contains(bodyIndex, `href="static/scss/desk.css"`) {
		t.Errorf("expected index to use 'static/scss/desk.css', got: %s", bodyIndex)
	}

	// 2. Channel Desk should render StaticPath as "../static"
	reqDesk := httptest.NewRequest(http.MethodGet, "/channel-1/", nil)
	recDesk := httptest.NewRecorder()
	ctrl.router.ServeHTTP(recDesk, reqDesk)
	if recDesk.Code != http.StatusOK {
		t.Fatalf("expected 200 for desk, got %d", recDesk.Code)
	}
	bodyDesk := recDesk.Body.String()
	if !strings.Contains(bodyDesk, `href="../static/css/tailwind.min.css"`) {
		t.Errorf("expected desk to use '../static/css/tailwind.min.css', got: %s", bodyDesk)
	}
	if !strings.Contains(bodyDesk, `src="../static/img/icons/bolt.svg"`) {
		t.Errorf("expected desk to use '../static/img/icons/bolt.svg', got: %s", bodyDesk)
	}
	if !strings.Contains(bodyDesk, `href="../static/scss/desk.css"`) {
		t.Errorf("expected desk to use '../static/scss/desk.css', got: %s", bodyDesk)
	}

	// 3. Channel Loan should render StaticPath as "../static"
	reqLoan := httptest.NewRequest(http.MethodGet, "/channel-1/loan", nil)
	recLoan := httptest.NewRecorder()
	ctrl.router.ServeHTTP(recLoan, reqLoan)
	if recLoan.Code != http.StatusOK {
		t.Fatalf("expected 200 for loan, got %d", recLoan.Code)
	}
	bodyLoan := recLoan.Body.String()
	if !strings.Contains(bodyLoan, `href="../static/css/tailwind.min.css"`) {
		t.Errorf("expected loan to use '../static/css/tailwind.min.css', got: %s", bodyLoan)
	}
	if !strings.Contains(bodyLoan, `src="../static/img/icons/book-open.svg"`) {
		t.Errorf("expected loan to use '../static/img/icons/book-open.svg', got: %s", bodyLoan)
	}
	if !strings.Contains(bodyLoan, `href="../static/scss/desk.css"`) {
		t.Errorf("expected loan to use '../static/scss/desk.css', got: %s", bodyLoan)
	}

	// 4. Tag Row fragment rendering
	tag := &rfidclient.TagReadEvent{
		BaseEvent: wsbrokerclient.BaseEvent{
			Channel: "channel-1",
		},
		Data: rfidclient.TagReadData{
			UID:    "E004015011223344",
			ItemID: "BOOK-123",
			BaseRFIDPayload: rfidclient.BaseRFIDPayload{
				ReaderID: "desk-reader",
			},
		},
	}
	item := hub.AddTag(tag)
	if item.StaticPath != "../static" {
		t.Errorf("expected item.StaticPath to be '../static', got '%s'", item.StaticPath)
	}
}

func TestController_SkinCookieAndHeaderSelector(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hub := desk.NewHub(10, nil)
	hub.Start(ctx)
	defer hub.Close()

	ctrl, err := NewController(Config{
		Addr:          "localhost:8762",
		ClientName:    "bib-client-01",
		WSBroker:      "wss://broker.example.com",
		Channels:      []string{"channel-1"},
		DefaultReader: "desk-reader",
		Hub:           hub,
	})
	if err != nil {
		t.Fatalf("failed to create web controller: %v", err)
	}

	// 1. Requesting index, desk, and loan -> contains #theme-dialog-btn and #theme-modal-dialog
	for _, path := range []string{"/", "/channel-1/", "/channel-1/loan"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		ctrl.router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 for %s, got %d", path, rec.Code)
		}
		body := rec.Body.String()
		if !strings.Contains(body, `id="theme-dialog-btn"`) {
			t.Errorf("expected %s to contain #theme-dialog-btn trigger button", path)
		}
		if !strings.Contains(body, `id="theme-modal-dialog"`) {
			t.Errorf("expected %s to contain #theme-modal-dialog", path)
		}
		if !strings.Contains(body, `id="theme-live-preview-box"`) {
			t.Errorf("expected %s to contain #theme-live-preview-box", path)
		}
		if strings.Contains(body, `id="skin-select"`) {
			t.Errorf("expected %s not to contain obsolete #skin-select dropdown", path)
		}
	}

	// 2. Requesting with rfid_theme=emerald cookie -> renders data-theme="emerald"
	reqEmerald := httptest.NewRequest(http.MethodGet, "/channel-1/", nil)
	reqEmerald.AddCookie(&http.Cookie{Name: "rfid_theme", Value: "emerald"})
	recEmerald := httptest.NewRecorder()
	ctrl.router.ServeHTTP(recEmerald, reqEmerald)
	if recEmerald.Code != http.StatusOK {
		t.Fatalf("expected 200 for desk with emerald cookie, got %d", recEmerald.Code)
	}
	bodyEmerald := recEmerald.Body.String()
	if !strings.Contains(bodyEmerald, `data-theme="emerald"`) {
		t.Errorf("expected desk with emerald cookie to have data-theme=\"emerald\", got: %s", bodyEmerald)
	}

	// 3. Requesting loan with rfid_theme=nord cookie -> renders data-theme="nord"
	reqNord := httptest.NewRequest(http.MethodGet, "/channel-1/loan", nil)
	reqNord.AddCookie(&http.Cookie{Name: "rfid_theme", Value: "nord"})
	recNord := httptest.NewRecorder()
	ctrl.router.ServeHTTP(recNord, reqNord)
	if recNord.Code != http.StatusOK {
		t.Fatalf("expected 200 for loan with nord cookie, got %d", recNord.Code)
	}
	bodyNord := recNord.Body.String()
	if !strings.Contains(bodyNord, `data-theme="nord"`) {
		t.Errorf("expected loan with nord cookie to have data-theme=\"nord\", got: %s", bodyNord)
	}

	// 4. Requesting desk with rfid_theme=synthwave cookie -> renders data-theme="synthwave"
	reqSynth := httptest.NewRequest(http.MethodGet, "/channel-1/", nil)
	reqSynth.AddCookie(&http.Cookie{Name: "rfid_theme", Value: "synthwave"})
	recSynth := httptest.NewRecorder()
	ctrl.router.ServeHTTP(recSynth, reqSynth)
	if recSynth.Code != http.StatusOK {
		t.Fatalf("expected 200 for desk with synthwave cookie, got %d", recSynth.Code)
	}
	bodySynth := recSynth.Body.String()
	if !strings.Contains(bodySynth, `data-theme="synthwave"`) {
		t.Errorf("expected desk with synthwave cookie to have data-theme=\"synthwave\", got: %s", bodySynth)
	}
}

func TestController_EncodeEndpoints(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hub := desk.NewHub(10, nil)
	hub.Start(ctx)
	defer hub.Close()

	ctrl, err := NewController(Config{
		Addr:          "localhost:8762",
		ClientName:    "bib-client-01",
		WSBroker:      "wss://broker.example.com",
		Channels:      []string{"channel-1"},
		DefaultReader: "desk-reader",
		DefaultISIL:   "CH-000001",
		DefaultAFI:    "07",
		Hub:           hub,
	})
	if err != nil {
		t.Fatalf("failed to create web controller: %v", err)
	}

	// Add an active tag
	hub.AddTag(&rfidclient.TagReadEvent{
		BaseEvent: wsbrokerclient.BaseEvent{
			Channel: "channel-1",
		},
		Data: rfidclient.TagReadData{
			UID:    "E004015099887766",
			ItemID: "OLD-ITEM",
			ISIL:   "CH-000001",
			BaseRFIDPayload: rfidclient.BaseRFIDPayload{
				ReaderID: "desk-reader",
			},
		},
	})

	// 0. Test /:channel/encode page rendering
	reqEncode := httptest.NewRequest(http.MethodGet, "/channel-1/encode", nil)
	recEncode := httptest.NewRecorder()
	ctrl.router.ServeHTTP(recEncode, reqEncode)
	if recEncode.Code != http.StatusOK {
		t.Errorf("expected 200 for /channel-1/encode, got %d", recEncode.Code)
	}
	if !strings.Contains(recEncode.Body.String(), "Medium kodieren") {
		t.Errorf("expected encode page to contain 'Medium kodieren', got: %s", recEncode.Body.String())
	}
	if !strings.Contains(recEncode.Body.String(), "CH-000001") {
		t.Errorf("expected encode page to contain default ISIL CH-000001, got: %s", recEncode.Body.String())
	}

	// 1. Test /:channel/encode/tags fragment
	reqTags := httptest.NewRequest(http.MethodGet, "/channel-1/encode/tags", nil)
	recTags := httptest.NewRecorder()
	ctrl.router.ServeHTTP(recTags, reqTags)
	if recTags.Code != http.StatusOK {
		t.Errorf("expected 200 for /channel-1/encode/tags, got %d", recTags.Code)
	}
	if !strings.Contains(recTags.Body.String(), "E004015099887766") {
		t.Errorf("expected encode tags fragment to contain tag UID, got: %s", recTags.Body.String())
	}

	// 2. Test /:channel/encode/write with valid payload
	writeJSON := `{
		"uid": "E004015099887766",
		"itemId": "NEW-ITEM-123",
		"isil": "CH-000001",
		"parts": 3,
		"partNo": 1,
		"afi": "07"
	}`
	reqWrite := httptest.NewRequest(http.MethodPost, "/channel-1/encode/write", strings.NewReader(writeJSON))
	reqWrite.Header.Set("Content-Type", "application/json")
	recWrite := httptest.NewRecorder()
	ctrl.router.ServeHTTP(recWrite, reqWrite)
	if recWrite.Code != http.StatusOK {
		t.Errorf("expected 200 for /channel-1/encode/write, got %d (body: %s)", recWrite.Code, recWrite.Body.String())
	}

	var writeResp map[string]interface{}
	if err := json.Unmarshal(recWrite.Body.Bytes(), &writeResp); err != nil {
		t.Fatalf("failed to parse write response JSON: %v", err)
	}
	if writeResp["status"] != "success" {
		t.Errorf("expected status 'success', got %v", writeResp["status"])
	}
	if writeResp["itemId"] != "NEW-ITEM-123" {
		t.Errorf("expected itemId 'NEW-ITEM-123', got %v", writeResp["itemId"])
	}

	// Verify hub updated
	active := hub.GetActiveTags()
	if len(active) == 0 || active[0].ItemID != "NEW-ITEM-123" {
		t.Errorf("expected hub active tag to have updated ItemID, got %+v", active)
	}
	if active[0].Parts != 3 || active[0].PartNo != 1 {
		t.Errorf("expected parts=3, partNo=1, got parts=%d, partNo=%d", active[0].Parts, active[0].PartNo)
	}

	// 3. Test /:channel/encode/write with empty ItemID -> 400
	emptyItemJSON := `{"uid": "E004015099887766", "itemId": ""}`
	reqEmpty := httptest.NewRequest(http.MethodPost, "/channel-1/encode/write", strings.NewReader(emptyItemJSON))
	reqEmpty.Header.Set("Content-Type", "application/json")
	recEmpty := httptest.NewRecorder()
	ctrl.router.ServeHTTP(recEmpty, reqEmpty)
	if recEmpty.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for empty itemId, got %d", recEmpty.Code)
	}

	// 4. Test /api/v1/encode/write API endpoint
	apiJSON := `{
		"uid": "E004015099887766",
		"itemId": "API-ITEM-456",
		"isil": "CH-000001",
		"parts": 1,
		"partNo": 1,
		"afi": "C2"
	}`
	reqAPI := httptest.NewRequest(http.MethodPost, "/api/v1/encode/write?channel=channel-1", strings.NewReader(apiJSON))
	reqAPI.Header.Set("Content-Type", "application/json")
	recAPI := httptest.NewRecorder()
	ctrl.router.ServeHTTP(recAPI, reqAPI)
	if recAPI.Code != http.StatusOK {
		t.Errorf("expected 200 for /api/v1/encode/write, got %d (body: %s)", recAPI.Code, recAPI.Body.String())
	}
}

func TestThemeModeAndColumnBadges(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hub := desk.NewHub(10, nil)
	hub.Start(ctx)
	defer hub.Close()

	ctrl, err := NewController(Config{
		Addr:          "localhost:8763",
		ClientName:    "bib-client-mode-test",
		WSBroker:      "wss://broker.example.com",
		Channels:      []string{"channel-1"},
		DefaultReader: "reader-01",
		Hub:           hub,
	})
	if err != nil {
		t.Fatalf("failed to create web controller: %v", err)
	}

	// 1. Test Theme Dialog & Mode Controls
	testCases := []struct {
		modeCookie string
	}{
		{""},
		{"auto"},
		{"lightmode"},
		{"darkmode"},
	}

	for _, tc := range testCases {
		req := httptest.NewRequest(http.MethodGet, "/channel-1/loan", nil)
		if tc.modeCookie != "" {
			req.AddCookie(&http.Cookie{Name: "rfid_theme_mode", Value: tc.modeCookie})
		}
		rec := httptest.NewRecorder()
		ctrl.router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 for /channel-1/loan, got %d", rec.Code)
		}
		body := rec.Body.String()
		if !strings.Contains(body, `id="theme-dialog-btn"`) {
			t.Errorf("expected theme-dialog-btn element in header, got: %s", body)
		}
		if !strings.Contains(body, `id="theme-mode-radios"`) {
			t.Errorf("expected theme-mode-radios element in theme modal, got: %s", body)
		}
	}

	// 2. Add Tag with valid CRC and Secured AFI
	hub.AddTag(&rfidclient.TagReadEvent{
		BaseEvent: wsbrokerclient.BaseEvent{
			Channel: "channel-1",
		},
		Data: rfidclient.TagReadData{
			UID:    "E004015011112222",
			ItemID: "ITEM-AFI-07",
			ISIL:   "CH-000001-1",
			AFI:    "07",
			CRCOk:  true,
			BaseRFIDPayload: rfidclient.BaseRFIDPayload{
				ReaderID: "reader-01",
			},
		},
	})

	// Render desk page and loan table fragment
	reqDesk := httptest.NewRequest(http.MethodGet, "/channel-1/", nil)
	recDesk := httptest.NewRecorder()
	ctrl.router.ServeHTTP(recDesk, reqDesk)
	deskBody := recDesk.Body.String()

	reqLoan := httptest.NewRequest(http.MethodGet, "/channel-1/loan/tags", nil)
	recLoan := httptest.NewRecorder()
	ctrl.router.ServeHTTP(recLoan, reqLoan)
	loanBody := recLoan.Body.String()

	// Verify text and icon presence
	for name, body := range map[string]string{"desk": deskBody, "loan": loanBody} {
		if !strings.Contains(body, "OK (Gültig)") {
			t.Errorf("[%s] expected 'OK (Gültig)', got: %s", name, body)
		}
		if !strings.Contains(body, "Gesichert (07)") {
			t.Errorf("[%s] expected 'Gesichert (07)', got: %s", name, body)
		}
		if !strings.Contains(body, "icon-check") {
			t.Errorf("[%s] expected 'icon-check', got: %s", name, body)
		}
		if !strings.Contains(body, "icon-lock-closed") {
			t.Errorf("[%s] expected 'icon-lock-closed', got: %s", name, body)
		}

		// Verify NO badge classes in the CRC and AFI display
		if strings.Contains(body, `badge badge-sm badge-success gap-1 text-[11px] font-mono`) {
			t.Errorf("[%s] found obsolete badge class on CRC/AFI element in body: %s", name, body)
		}
		if strings.Contains(body, `badge badge-sm badge-info gap-1 text-[11px] font-mono`) {
			t.Errorf("[%s] found obsolete badge-info class on AFI element in body: %s", name, body)
		}
	}

	// Verify theme-init.js inclusion in all full page views
	for _, pagePath := range []string{"/", "/channel-1/", "/channel-1/loan", "/channel-1/encode"} {
		reqPage := httptest.NewRequest(http.MethodGet, pagePath, nil)
		recPage := httptest.NewRecorder()
		ctrl.router.ServeHTTP(recPage, reqPage)
		if recPage.Code != http.StatusOK {
			t.Errorf("expected 200 for %s, got %d", pagePath, recPage.Code)
		}
		pBody := recPage.Body.String()
		if !strings.Contains(pBody, `/js/theme-init.js`) {
			t.Errorf("expected %s to contain /js/theme-init.js script reference, got: %s", pagePath, pBody)
		}
		if strings.Contains(pBody, "var themeMap =") {
			t.Errorf("expected %s not to contain redundant inline themeMap script", pagePath)
		}
	}
}

func TestReaderIDRenderingInDeskTable(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hub := desk.NewHub(10, nil)
	hub.Start(ctx)
	defer hub.Close()

	ctrl, err := NewController(Config{
		Addr:          "localhost:8764",
		ClientName:    "reader-id-test-desk",
		WSBroker:      "wss://broker.example.com",
		Channels:      []string{"rfid"},
		DefaultReader: "reader-01",
		Hub:           hub,
	})
	if err != nil {
		t.Fatalf("failed to create web controller: %v", err)
	}

	// 1. Tag with ReaderID
	tagWithReader := &rfidclient.TagReadEvent{
		BaseEvent: wsbrokerclient.BaseEvent{
			Channel: "rfid",
		},
		Data: rfidclient.TagReadData{
			UID:    "E004015011110001",
			ItemID: "ITEM-READER-01",
			BaseRFIDPayload: rfidclient.BaseRFIDPayload{
				ReaderID: "gate-reader-01",
			},
		},
	}
	hub.AddTag(tagWithReader)

	req := httptest.NewRequest(http.MethodGet, "/rfid/", nil)
	rec := httptest.NewRecorder()
	ctrl.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()

	if !strings.Contains(body, "gate-reader-01") {
		t.Errorf("expected body to contain ReaderID 'gate-reader-01', got: %s", body)
	}
	if !strings.Contains(body, `title="Reader: gate-reader-01"`) {
		t.Errorf("expected body to contain title attribute for ReaderID, got: %s", body)
	}

	// 2. Tag with second ReaderID
	tagOnlyReaderID := &rfidclient.TagReadEvent{
		BaseEvent: wsbrokerclient.BaseEvent{
			Channel: "rfid",
		},
		Data: rfidclient.TagReadData{
			UID:    "E004015011110002",
			ItemID: "ITEM-READER-ONLY",
			BaseRFIDPayload: rfidclient.BaseRFIDPayload{
				ReaderID: "desk-pad-02",
			},
		},
	}
	hub.AddTag(tagOnlyReaderID)

	rec2 := httptest.NewRecorder()
	ctrl.router.ServeHTTP(rec2, req)
	body2 := rec2.Body.String()
	if !strings.Contains(body2, "desk-pad-02") {
		t.Errorf("expected body to contain ReaderID 'desk-pad-02', got: %s", body2)
	}
	if !strings.Contains(body2, `title="Reader: desk-pad-02"`) {
		t.Errorf("expected body to contain title attribute for ReaderID, got: %s", body2)
	}
}
