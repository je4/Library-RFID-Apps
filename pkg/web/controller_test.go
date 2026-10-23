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

	// 3. Test API Status
	reqStatus := httptest.NewRequest(http.MethodGet, "/api/v1/status", nil)
	recStatus := httptest.NewRecorder()
	ctrl.router.ServeHTTP(recStatus, reqStatus)
	if recStatus.Code != http.StatusOK {
		t.Errorf("expected 200 for status API, got %d", recStatus.Code)
	}

	// 4. Test Tag Scan Broadcasting and API Tags
	tag := &rfidclient.TagReadEvent{
		UID:       "E004015099887766",
		ItemID:    "ITEM-TEST-123",
		ISIL:      "CH-000001-1",
		Country:   "CH",
		CRCOk:     true,
		Timestamp: time.Now().UnixMilli(),
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
}
