package web

import (
	"context"
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
