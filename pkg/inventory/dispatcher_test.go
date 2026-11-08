package inventory

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/je4/Library-RFID-Apps/pkg/desk"
	"github.com/je4/utils/v2/pkg/config"
)

func TestWebserviceDispatcher_InterpolateURL(t *testing.T) {
	d := NewWebserviceDispatcher(desk.WebserviceConfig{}, nil)

	ts := time.Date(2026, 11, 8, 18, 30, 0, 0, time.UTC).UnixMilli()
	payload := ScanPayload{
		ClientID:   "station-01",
		SessionID:  "sess-99",
		Marker:     "EG-R04-F02",
		UID:        "E004015099887766",
		ItemID:     "301012345",
		Raw:        "01020304",
		Country:    "CH",
		ISIL:       "001596-0",
		Parts:      2,
		PartNumber: 1,
		Timestamp:  ts,
	}

	tmpl := "https://example.com/api?marker={marker}&session={session}&uid={uid}&itemid={itemid}&clientid={clientid}&country={country}&isil={isil}&parts={parts}&partno={partno}&raw={raw}"
	res := d.InterpolateURL(tmpl, payload)

	if !strings.Contains(res, "marker=EG-R04-F02") {
		t.Errorf("expected marker in URL, got %s", res)
	}
	if !strings.Contains(res, "session=sess-99") {
		t.Errorf("expected session in URL, got %s", res)
	}
	if !strings.Contains(res, "uid=E004015099887766") {
		t.Errorf("expected uid in URL, got %s", res)
	}
	if !strings.Contains(res, "itemid=301012345") {
		t.Errorf("expected itemid in URL, got %s", res)
	}
	if !strings.Contains(res, "clientid=station-01") {
		t.Errorf("expected clientid in URL, got %s", res)
	}
	if !strings.Contains(res, "parts=2") {
		t.Errorf("expected parts in URL, got %s", res)
	}
	if !strings.Contains(res, "partno=1") {
		t.Errorf("expected partno in URL, got %s", res)
	}
}

func TestWebserviceDispatcher_GenerateJWT(t *testing.T) {
	secret := "test-secret-key-123"
	d := NewWebserviceDispatcher(desk.WebserviceConfig{
		JWTKey: config.EnvString(secret),
	}, nil)

	payload := ScanPayload{
		ClientID:  "stat-1",
		SessionID: "s-1",
		Marker:    "R01",
		UID:       "E004",
		ItemID:    "B001",
	}

	token, err := d.GenerateJWT(payload)
	if err != nil {
		t.Fatalf("failed to generate JWT: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty JWT token")
	}

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("expected 3 parts in JWT token, got %d", len(parts))
	}

	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		t.Fatalf("failed to decode header: %v", err)
	}
	var header map[string]string
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		t.Fatalf("failed to unmarshal header: %v", err)
	}
	if header["alg"] != "HS256" {
		t.Errorf("expected alg HS256, got %s", header["alg"])
	}

	claimsBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatalf("failed to decode claims: %v", err)
	}
	var claims map[string]interface{}
	if err := json.Unmarshal(claimsBytes, &claims); err != nil {
		t.Fatalf("failed to unmarshal claims: %v", err)
	}
	if claims["sub"] != "E004" || claims["client_id"] != "stat-1" {
		t.Errorf("unexpected claims: %v", claims)
	}

	signingInput := parts[0] + "." + parts[1]
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(signingInput))
	expectedSig := base64.RawURLEncoding.EncodeToString(h.Sum(nil))

	if parts[2] != expectedSig {
		t.Errorf("signature mismatch: expected %s, got %s", expectedSig, parts[2])
	}
}

func TestWebserviceDispatcher_Dispatch_PostSuccess(t *testing.T) {
	var receivedAuth string
	var receivedBody ScanPayload

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		bodyBytes, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(bodyBytes, &receivedBody)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok","acknowledged":true}`))
	}))
	defer srv.Close()

	d := NewWebserviceDispatcher(desk.WebserviceConfig{
		TargetURL:  config.EnvString(srv.URL + "/inventory"),
		ClientID:   "client-alpha",
		HTTPMethod: "POST",
		JWTKey:     "jwt-secret-xyz",
		Timeout:    2 * time.Second,
	}, nil)

	payload := ScanPayload{
		SessionID: "sess-abc",
		Marker:    "SHELF-1",
		UID:       "E00411223344",
		ItemID:    "ITEM-999",
	}

	resp, err := d.Dispatch(context.Background(), payload)
	if err != nil {
		t.Fatalf("dispatch returned error: %v", err)
	}
	if !resp.Success {
		t.Errorf("expected success true, got false with error: %s", resp.Error)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
	if !strings.HasPrefix(receivedAuth, "Bearer ") {
		t.Errorf("expected Bearer Authorization header, got: %s", receivedAuth)
	}
	if receivedBody.UID != "E00411223344" || receivedBody.ClientID != "client-alpha" {
		t.Errorf("unexpected body received: %+v", receivedBody)
	}
}

func TestWebserviceDispatcher_Dispatch_HttpError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid barcode format"}`))
	}))
	defer srv.Close()

	d := NewWebserviceDispatcher(desk.WebserviceConfig{
		TargetURL:  config.EnvString(srv.URL + "/inventory"),
		HTTPMethod: "POST",
		Timeout:    1 * time.Second,
	}, nil)

	payload := ScanPayload{
		UID:    "E00411223344",
		ItemID: "INVALID",
	}

	resp, err := d.Dispatch(context.Background(), payload)
	if err != nil {
		t.Fatalf("unexpected dispatch error: %v", err)
	}
	if resp.Success {
		t.Error("expected success false on HTTP 400")
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", resp.StatusCode)
	}
	if !strings.Contains(resp.Error, "invalid barcode format") {
		t.Errorf("expected error message in response error, got %s", resp.Error)
	}
}
