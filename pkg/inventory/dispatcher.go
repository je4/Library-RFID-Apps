package inventory

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"emperror.dev/errors"
	"github.com/je4/Library-RFID-Apps/pkg/desk"
	"github.com/je4/utils/v2/pkg/zLogger"
)

// ScanPayload contains all fields relevant for dispatching a tag scan to an external webservice.
type ScanPayload struct {
	Marker     string `json:"marker"`
	SessionID  string `json:"session"`
	Raw        string `json:"raw,omitempty"`
	UID        string `json:"uid"`
	Timestamp  int64  `json:"timestamp"`
	JWT        string `json:"jwt,omitempty"`
	ClientID   string `json:"clientId,omitempty"`
	ItemID     string `json:"itemId,omitempty"`
	Country    string `json:"country,omitempty"`
	ISIL       string `json:"isil,omitempty"`
	Parts      int    `json:"parts,omitempty"`
	PartNumber int    `json:"partNo,omitempty"`
	UsageType  int    `json:"usageType,omitempty"`
	Version    int    `json:"version,omitempty"`
	IsCRCValid bool   `json:"isCrcValid,omitempty"`
	AFI        string `json:"afi,omitempty"`
}

// DispatchResponse contains the HTTP result from a webservice dispatch call.
type DispatchResponse struct {
	StatusCode int           `json:"statusCode"`
	StatusText string        `json:"statusText"`
	Body       string        `json:"body,omitempty"`
	Duration   time.Duration `json:"duration"`
	Error      string        `json:"error,omitempty"`
	Success    bool          `json:"success"`
}

// Dispatcher defines the interface for dispatching inventory scans to external services.
type Dispatcher interface {
	Dispatch(ctx context.Context, payload ScanPayload) (*DispatchResponse, error)
	InterpolateURL(targetTemplate string, payload ScanPayload) string
	GenerateJWT(payload ScanPayload) (string, error)
}

// WebserviceDispatcher implements the Dispatcher interface over HTTP GET/POST with optional HMAC-SHA256 JWT auth.
type WebserviceDispatcher struct {
	cfg        desk.WebserviceConfig
	httpClient *http.Client
	logger     zLogger.ZLogger
}

// NewWebserviceDispatcher creates a new WebserviceDispatcher from the provided configuration.
func NewWebserviceDispatcher(cfg desk.WebserviceConfig, logger zLogger.ZLogger) *WebserviceDispatcher {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	return &WebserviceDispatcher{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		logger: logger,
	}
}

// InterpolateURL replaces placeholders in the target URL template with values from ScanPayload.
func (d *WebserviceDispatcher) InterpolateURL(targetTemplate string, payload ScanPayload) string {
	res := targetTemplate
	res = strings.ReplaceAll(res, "{marker}", url.QueryEscape(payload.Marker))
	res = strings.ReplaceAll(res, "{location}", url.QueryEscape(payload.Marker))
	res = strings.ReplaceAll(res, "{session}", url.QueryEscape(payload.SessionID))
	res = strings.ReplaceAll(res, "{uid}", url.QueryEscape(payload.UID))
	res = strings.ReplaceAll(res, "{raw}", url.QueryEscape(payload.Raw))
	res = strings.ReplaceAll(res, "{itemid}", url.QueryEscape(payload.ItemID))
	res = strings.ReplaceAll(res, "{clientid}", url.QueryEscape(payload.ClientID))
	res = strings.ReplaceAll(res, "{client_id}", url.QueryEscape(payload.ClientID))
	res = strings.ReplaceAll(res, "{clientId}", url.QueryEscape(payload.ClientID))
	res = strings.ReplaceAll(res, "{country}", url.QueryEscape(payload.Country))
	res = strings.ReplaceAll(res, "{isil}", url.QueryEscape(payload.ISIL))
	res = strings.ReplaceAll(res, "{parts}", strconv.Itoa(payload.Parts))
	res = strings.ReplaceAll(res, "{partno}", strconv.Itoa(payload.PartNumber))
	res = strings.ReplaceAll(res, "{usagetype}", strconv.Itoa(payload.UsageType))
	res = strings.ReplaceAll(res, "{version}", strconv.Itoa(payload.Version))
	res = strings.ReplaceAll(res, "{jwt}", url.QueryEscape(payload.JWT))

	tsMillis := payload.Timestamp
	if tsMillis <= 0 {
		tsMillis = time.Now().UnixMilli()
	}
	res = strings.ReplaceAll(res, "{timestamp}", strconv.FormatInt(tsMillis, 10))
	res = strings.ReplaceAll(res, "{ts}", strconv.FormatInt(tsMillis, 10))

	dateIso := time.UnixMilli(tsMillis).UTC().Format(time.RFC3339)
	res = strings.ReplaceAll(res, "{date}", url.QueryEscape(dateIso))

	return res
}

// GenerateJWT creates a signed HMAC-SHA256 JWT token with the scan payload claims.
func (d *WebserviceDispatcher) GenerateJWT(payload ScanPayload) (string, error) {
	if d.cfg.JWTKey == "" {
		return "", nil
	}

	header := map[string]string{
		"alg": "HS256",
		"typ": "JWT",
	}
	headerBytes, err := json.Marshal(header)
	if err != nil {
		return "", errors.Wrap(err, "cannot marshal JWT header")
	}

	now := time.Now().UTC()
	iatSeconds := now.Unix()
	expSeconds := iatSeconds + 60

	claims := map[string]interface{}{
		"iat": iatSeconds,
		"exp": expSeconds,
	}
	if payload.UID != "" {
		claims["sub"] = payload.UID
	}
	if payload.ClientID != "" {
		claims["client_id"] = payload.ClientID
	}
	claimsBytes, err := json.Marshal(claims)
	if err != nil {
		return "", errors.Wrap(err, "cannot marshal JWT claims")
	}

	encHeader := base64.RawURLEncoding.EncodeToString(headerBytes)
	encClaims := base64.RawURLEncoding.EncodeToString(claimsBytes)
	signingInput := encHeader + "." + encClaims

	h := hmac.New(sha256.New, []byte(d.cfg.JWTKey))
	h.Write([]byte(signingInput))
	sig := h.Sum(nil)
	encSig := base64.RawURLEncoding.EncodeToString(sig)

	return signingInput + "." + encSig, nil
}

// Dispatch sends the scan payload to the configured external webservice.
func (d *WebserviceDispatcher) Dispatch(ctx context.Context, payload ScanPayload) (*DispatchResponse, error) {
	if payload.ClientID == "" && d.cfg.ClientID != "" {
		payload.ClientID = d.cfg.ClientID
	}
	if payload.Timestamp <= 0 {
		payload.Timestamp = time.Now().UnixMilli()
	}

	var jwtToken string
	if d.cfg.JWTKey != "" {
		token, err := d.GenerateJWT(payload)
		if err != nil {
			if d.logger != nil {
				d.logger.Warn().Err(err).Msg("failed to generate JWT token for inventory dispatch")
			}
		} else if token != "" {
			jwtToken = token
			payload.JWT = token
		}
	}

	targetURL := d.InterpolateURL(d.cfg.TargetURL.String(), payload)
	if targetURL == "" {
		if d.logger != nil {
			d.logger.Error().Msg("webservice dispatch aborted: target URL is empty")
		}
		return &DispatchResponse{
			Success: false,
			Error:   "target webservice URL is empty",
		}, errors.New("target webservice URL is empty")
	}

	method := strings.ToUpper(strings.TrimSpace(d.cfg.HTTPMethod))
	if method == "" {
		method = http.MethodPost
	}

	var reqBody io.Reader
	var requestBodyStr string
	if method == http.MethodPost || method == http.MethodPut {
		bodyBytes, err := json.Marshal(payload)
		if err != nil {
			if d.logger != nil {
				d.logger.Error().Err(err).Msg("cannot marshal scan payload to JSON for webservice dispatch")
			}
			return nil, errors.Wrap(err, "cannot marshal scan payload to JSON")
		}
		requestBodyStr = string(bodyBytes)
		reqBody = bytes.NewReader(bodyBytes)
	}

	req, err := http.NewRequestWithContext(ctx, method, targetURL, reqBody)
	if err != nil {
		if d.logger != nil {
			d.logger.Error().Err(err).Str("url", targetURL).Msg("cannot create HTTP request for webservice dispatch")
		}
		return nil, errors.Wrapf(err, "cannot create HTTP request for %s", targetURL)
	}

	req.Header.Set("User-Agent", "Iso15693NfcReader/1.0 (Desktop)")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	if method == http.MethodPost || method == http.MethodPut {
		req.Header.Set("Content-Type", "application/json; charset=utf-8")
	}

	if jwtToken != "" {
		req.Header.Set("Authorization", "Bearer "+jwtToken)
	}

	if d.logger != nil {
		event := d.logger.Info()
		if !d.cfg.DebugMode {
			event = d.logger.Debug()
		}
		event.
			Str("method", method).
			Str("url", targetURL).
			Str("uid", payload.UID).
			Str("itemId", payload.ItemID).
			Str("marker", payload.Marker).
			Msg("dispatching inventory scan to webservice")
	}

	start := time.Now()
	resp, err := d.httpClient.Do(req)
	duration := time.Since(start)

	if err != nil {
		if d.logger != nil {
			d.logger.Error().
				Err(err).
				Str("method", method).
				Str("url", targetURL).
				Str("uid", payload.UID).
				Str("itemId", payload.ItemID).
				Str("marker", payload.Marker).
				Str("body", requestBodyStr).
				Msg("webservice dispatch failed with network/HTTP error")
		}
		return &DispatchResponse{
			StatusCode: 0,
			StatusText: "Network Error",
			Duration:   duration,
			Error:      err.Error(),
			Success:    false,
		}, err
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
	bodyStr := string(bodyBytes)

	isSuccess := resp.StatusCode >= 200 && resp.StatusCode < 300
	var dispatchErr string
	if !isSuccess {
		dispatchErr = fmt.Sprintf("HTTP %d: %s", resp.StatusCode, resp.Status)
		if bodyStr != "" {
			dispatchErr += " - " + bodyStr
		}

		if d.logger != nil {
			d.logger.Error().
				Int("status", resp.StatusCode).
				Str("statusText", resp.Status).
				Str("method", method).
				Str("url", targetURL).
				Str("uid", payload.UID).
				Str("itemId", payload.ItemID).
				Str("marker", payload.Marker).
				Str("requestBody", requestBodyStr).
				Str("responseBody", bodyStr).
				Str("duration", duration.String()).
				Msg("webservice dispatch failed with non-2xx HTTP status")
		}
	} else {
		if d.logger != nil {
			d.logger.Info().
				Int("status", resp.StatusCode).
				Str("method", method).
				Str("url", targetURL).
				Str("uid", payload.UID).
				Str("itemId", payload.ItemID).
				Str("marker", payload.Marker).
				Str("duration", duration.String()).
				Msg("webservice dispatch succeeded")
		}
	}

	return &DispatchResponse{
		StatusCode: resp.StatusCode,
		StatusText: resp.Status,
		Body:       bodyStr,
		Duration:   duration,
		Error:      dispatchErr,
		Success:    isSuccess,
	}, nil
}
