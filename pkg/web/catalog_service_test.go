package web

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/je4/Library-RFID-Apps/pkg/catalog"
	"github.com/je4/Library-RFID-Apps/pkg/desk"
)

func TestCatalogEndpoint(t *testing.T) {
	calls := map[string]int{}
	ctrl, err := NewController(Config{Hub: desk.NewHub(10), Channels: []string{"desk"}, SourceRecords: []SourceRecordMapping{{Network: "network", BaseURL: "https://example.test/source", Institutions: map[string]string{"ETH": "41SLSP_ETH:ETH"}}}, CatalogSearch: func(_ context.Context, barcode string) ([]catalog.Book, error) {
		calls[barcode]++
		switch barcode {
		case "failure":
			return nil, errors.New("unavailable")
		case "empty":
			return nil, nil
		default:
			return []catalog.Book{{ID: barcode, Titles: []catalog.Title{{Title: "Example"}}}}, nil
		}
	}})
	if err != nil {
		t.Fatal(err)
	}
	request := func(path string, status int, body string) {
		t.Helper()
		rec := httptest.NewRecorder()
		ctrl.router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != status || !strings.Contains(rec.Body.String(), body) {
			t.Fatalf("%s: status %d, body %s", path, rec.Code, rec.Body.String())
		}
	}
	request("/desk/catalog?barcode=123", http.StatusOK, `"title":"Example"`)
	request("/desk/catalog?barcode=123", http.StatusOK, `"sourceRecords":[{"network":"network","baseUrl":"https://example.test/source","institutions":{"ETH":"41SLSP_ETH:ETH"}}]`)
	request("/desk/catalog?barcode=123", http.StatusOK, `"id":"123"`)
	request("/desk/catalog?barcode=empty", http.StatusOK, `"books":[]`)
	request("/desk/catalog?barcode=empty", http.StatusOK, `"books":[]`)
	request("/desk/catalog?barcode=failure", http.StatusBadGateway, "")
	request("/desk/catalog?barcode=failure", http.StatusBadGateway, "")
	request("/desk/catalog", http.StatusBadRequest, "")
	request("/invalid/catalog?barcode=123", http.StatusNotFound, "")
	if calls["123"] != 1 || calls["empty"] != 1 || calls["failure"] != 2 {
		t.Fatalf("unexpected search calls: %v", calls)
	}
}

func TestCatalogDisabled(t *testing.T) {
	ctrl, err := NewController(Config{Hub: desk.NewHub(10)})
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	ctrl.router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/desk/catalog?barcode=123", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("got status %d", rec.Code)
	}
}
