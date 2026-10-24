package rfidclient

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/je4/wsbrokerclient"
)

func TestTagReadEvent_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name         string
		jsonInput    string
		wantUID      string
		wantItemID   string
		wantISIL     string
		wantCountry  string
		wantCRCOk    bool
		wantParts    int
		wantPartNo   int
		wantUsage    int
		wantMulti    bool
		wantCRCValid bool
		wantAFI      string
	}{
		{
			name: "Standard rfid-headless TagRead event with AFI",
			jsonInput: `{
				"event": "rfid:tag_read",
				"uid": "E004010000001234",
				"itemId": "3012345678",
				"isil": "000001-1",
				"country": "CH",
				"crcOk": true,
				"isCrcValid": true,
				"crc": "OK",
				"tagName": "ISO 15693 : NXP I-Code SLIX",
				"raw": "01020304",
				"parts": 1,
				"partNo": 1,
				"usageType": 1,
				"version": 1,
				"afi": "07",
				"session": "main-scan",
				"timestamp": 1792756801123,
				"clientId": "rfid-reader-station-01"
			}`,
			wantUID:      "E004010000001234",
			wantItemID:   "3012345678",
			wantISIL:     "000001-1",
			wantCountry:  "CH",
			wantCRCOk:    true,
			wantParts:    1,
			wantPartNo:   1,
			wantUsage:    1,
			wantMulti:    false,
			wantCRCValid: true,
			wantAFI:      "07",
		},
		{
			name: "TagRead multi-part item",
			jsonInput: `{
				"event": "rfid:tag_read",
				"uid": "E004015099887766",
				"itemId": "BARCODE999",
				"country": "DE",
				"crcOk": false,
				"crc": "ERROR",
				"parts": 3,
				"partNo": 2,
				"usageType": 2,
				"afi": "C2"
			}`,
			wantUID:      "E004015099887766",
			wantItemID:   "BARCODE999",
			wantISIL:     "",
			wantCountry:  "DE",
			wantCRCOk:    false,
			wantParts:    3,
			wantPartNo:   2,
			wantUsage:    2,
			wantMulti:    true,
			wantCRCValid: false,
			wantAFI:      "C2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var tag TagReadEvent
			if err := json.Unmarshal([]byte(tt.jsonInput), &tag); err != nil {
				t.Fatalf("failed to unmarshal: %v", err)
			}

			if tag.UID != tt.wantUID {
				t.Errorf("UID = %q, want %q", tag.UID, tt.wantUID)
			}
			if tag.ItemID != tt.wantItemID {
				t.Errorf("ItemID = %q, want %q", tag.ItemID, tt.wantItemID)
			}
			if tag.ISIL != tt.wantISIL {
				t.Errorf("ISIL = %q, want %q", tag.ISIL, tt.wantISIL)
			}
			if tag.Country != tt.wantCountry {
				t.Errorf("Country = %q, want %q", tag.Country, tt.wantCountry)
			}
			if tag.GetParts() != tt.wantParts {
				t.Errorf("GetParts() = %d, want %d", tag.GetParts(), tt.wantParts)
			}
			if tag.GetPartNo() != tt.wantPartNo {
				t.Errorf("GetPartNo() = %d, want %d", tag.GetPartNo(), tt.wantPartNo)
			}
			if tag.GetUsageType() != tt.wantUsage {
				t.Errorf("GetUsageType() = %d, want %d", tag.GetUsageType(), tt.wantUsage)
			}
			if tag.IsMultiPart() != tt.wantMulti {
				t.Errorf("IsMultiPart() = %v, want %v", tag.IsMultiPart(), tt.wantMulti)
			}
			if tag.IsCRCValidStatus() != tt.wantCRCValid {
				t.Errorf("IsCRCValidStatus() = %v, want %v", tag.IsCRCValidStatus(), tt.wantCRCValid)
			}
			if tag.AFI != tt.wantAFI {
				t.Errorf("AFI = %q, want %q", tag.AFI, tt.wantAFI)
			}

			summary := tag.Summary()
			if summary == "" {
				t.Error("Summary() should not be empty")
			}
		})
	}
}

func TestTagsClearedEvent(t *testing.T) {
	raw := `{
		"event": "rfid:tags_cleared",
		"status": "empty",
		"totalTags": 0,
		"session": "main-scan",
		"timestamp": 1792756801500,
		"clientId": "rfid-station-01"
	}`

	var cleared TagsClearedEvent
	if err := json.Unmarshal([]byte(raw), &cleared); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if cleared.Status != "empty" {
		t.Errorf("Status = %q, want 'empty'", cleared.Status)
	}
	if cleared.TotalTags != 0 {
		t.Errorf("TotalTags = %d, want 0", cleared.TotalTags)
	}
	if cleared.ClientID != "rfid-station-01" {
		t.Errorf("ClientID = %q, want 'rfid-station-01'", cleared.ClientID)
	}

	expectedTime := time.UnixMilli(1792756801500)
	if !cleared.Time().Equal(expectedTime) {
		t.Errorf("Time() = %v, want %v", cleared.Time(), expectedTime)
	}

	if cleared.Summary() == "" {
		t.Error("Summary() should not be empty")
	}
}

func TestDeviceMetadataResponse_UnmarshalJSON(t *testing.T) {
	jsonInput := `{
		"event": "rfid:device_metadata_response",
		"status": "ok",
		"deviceId": "0052E688",
		"deviceType": "ISC.MR102-USB",
		"initDescription": "FEIG Electronic ISC.MR102-USB (0052E688)\nReader initialized.",
		"timestamp": 1792756802045,
		"clientId": "rfid-reader-station-01"
	}`

	var resp DeviceMetadataResponse
	if err := json.Unmarshal([]byte(jsonInput), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if resp.DeviceID != "0052E688" {
		t.Errorf("DeviceID = %q, want '0052E688'", resp.DeviceID)
	}
	if resp.DeviceType != "ISC.MR102-USB" {
		t.Errorf("DeviceType = %q, want 'ISC.MR102-USB'", resp.DeviceType)
	}
	if resp.GetDescription() != "FEIG Electronic ISC.MR102-USB (0052E688)\nReader initialized." {
		t.Errorf("GetDescription() = %q", resp.GetDescription())
	}
	if !resp.IsSuccess() {
		t.Errorf("IsSuccess() = false, want true")
	}
	if resp.Summary() == "" {
		t.Error("Summary() should not be empty")
	}
}

func TestWriteTagRequestAndResponse(t *testing.T) {
	req := NewWriteTagRequest("3012345678", "E004015099887766")
	req.ISIL = "CH-000001-1"
	req.Country = "CH"
	req.AFI = "07"

	if err := req.Validate(); err != nil {
		t.Fatalf("validation failed: %v", err)
	}

	invalidReq := WriteTagRequest{}
	if err := invalidReq.Validate(); err == nil {
		t.Error("expected error validating empty WriteTagRequest")
	}

	respJSON := `{
		"event": "rfid:write_tag_response",
		"status": "ok",
		"uid": "E004015099887766",
		"itemId": "3012345678",
		"isil": "CH-000001-1",
		"country": "CH",
		"parts": 1,
		"partNo": 1,
		"usageType": 1,
		"afi": "07"
	}`

	var resp WriteTagResponse
	if err := json.Unmarshal([]byte(respJSON), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if !resp.IsSuccess() {
		t.Error("expected IsSuccess() = true")
	}
	if resp.UID != "E004015099887766" {
		t.Errorf("UID = %q, want E004015099887766", resp.UID)
	}
	if resp.ItemID != "3012345678" {
		t.Errorf("ItemID = %q, want 3012345678", resp.ItemID)
	}
	if resp.AFI != "07" {
		t.Errorf("AFI = %q, want 07", resp.AFI)
	}
	if resp.Summary() == "" {
		t.Error("Summary() should not be empty")
	}

	errResp := WriteTagResponse{
		Status:  "error",
		UID:     "E004015099887766",
		Message: "Tag not in field",
	}
	if errResp.IsSuccess() {
		t.Error("expected IsSuccess() = false for error response")
	}
	if errResp.GetError() != "Tag not in field" {
		t.Errorf("GetError() = %q, want 'Tag not in field'", errResp.GetError())
	}
}

func TestWriteAFIRequestAndResponse(t *testing.T) {
	req := NewWriteAFIRequest("C2", "E004015099887766")
	if err := req.Validate(); err != nil {
		t.Fatalf("validation failed: %v", err)
	}

	invalidReq := WriteAFIRequest{}
	if err := invalidReq.Validate(); err == nil {
		t.Error("expected error validating empty WriteAFIRequest")
	}

	respJSON := `{
		"event": "rfid:write_afi_response",
		"status": "ok",
		"uid": "E004015099887766",
		"afi": "C2"
	}`

	var resp WriteAFIResponse
	if err := json.Unmarshal([]byte(respJSON), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if !resp.IsSuccess() {
		t.Error("expected IsSuccess() = true")
	}
	if resp.UID != "E004015099887766" {
		t.Errorf("UID = %q, want E004015099887766", resp.UID)
	}
	if resp.AFI != "C2" {
		t.Errorf("AFI = %q, want C2", resp.AFI)
	}
	if resp.Summary() == "" {
		t.Error("Summary() should not be empty")
	}
}

func TestParseRFIDPayloadAndParseEvent(t *testing.T) {
	// 1. TagRead
	rawTag := []byte(`{"event":"rfid:tag_read","uid":"E00401","itemId":"12345"}`)
	payload, err := ParseRFIDPayload(rawTag)
	if err != nil {
		t.Fatalf("ParseRFIDPayload failed: %v", err)
	}
	tag, ok := payload.(*TagReadEvent)
	if !ok || tag.ItemID != "12345" {
		t.Fatalf("expected *TagReadEvent, got %T", payload)
	}

	// 2. TagsCleared
	rawCleared := []byte(`{"event":"rfid:tags_cleared","status":"cleared","totalTags":0}`)
	payload, err = ParseRFIDPayload(rawCleared)
	if err != nil {
		t.Fatalf("ParseRFIDPayload failed: %v", err)
	}
	cleared, ok := payload.(*TagsClearedEvent)
	if !ok || cleared.Status != "cleared" {
		t.Fatalf("expected *TagsClearedEvent, got %T", payload)
	}

	// 3. DeviceMetadataResponse
	rawMeta := []byte(`{"event":"rfid:device_metadata_response","status":"ok","deviceId":"DEV-1"}`)
	payload, err = ParseRFIDPayload(rawMeta)
	if err != nil {
		t.Fatalf("ParseRFIDPayload failed: %v", err)
	}
	meta, ok := payload.(*DeviceMetadataResponse)
	if !ok || meta.DeviceID != "DEV-1" {
		t.Fatalf("expected *DeviceMetadataResponse, got %T", payload)
	}

	// 4. DeviceMetadataRequest
	rawMetaReq := []byte(`{"event":"rfid:get_device_metadata"}`)
	payload, err = ParseRFIDPayload(rawMetaReq)
	if err != nil {
		t.Fatalf("ParseRFIDPayload failed: %v", err)
	}
	if _, ok := payload.(*DeviceMetadataRequest); !ok {
		t.Fatalf("expected *DeviceMetadataRequest, got %T", payload)
	}

	// 5. WriteTagResponse
	rawWriteResp := []byte(`{"event":"rfid:write_tag_response","status":"ok","uid":"E004"}`)
	payload, err = ParseRFIDPayload(rawWriteResp)
	if err != nil {
		t.Fatalf("ParseRFIDPayload failed: %v", err)
	}
	if _, ok := payload.(*WriteTagResponse); !ok {
		t.Fatalf("expected *WriteTagResponse, got %T", payload)
	}

	// 6. WriteTagRequest
	rawWriteReq := []byte(`{"event":"rfid:write_tag","itemId":"ITEM-1","uid":"E004"}`)
	payload, err = ParseRFIDPayload(rawWriteReq)
	if err != nil {
		t.Fatalf("ParseRFIDPayload failed: %v", err)
	}
	if _, ok := payload.(*WriteTagRequest); !ok {
		t.Fatalf("expected *WriteTagRequest, got %T", payload)
	}

	// 7. WriteAFIResponse
	rawAFIResp := []byte(`{"event":"rfid:write_afi_response","status":"ok","afi":"07"}`)
	payload, err = ParseRFIDPayload(rawAFIResp)
	if err != nil {
		t.Fatalf("ParseRFIDPayload failed: %v", err)
	}
	if _, ok := payload.(*WriteAFIResponse); !ok {
		t.Fatalf("expected *WriteAFIResponse, got %T", payload)
	}

	// 8. WriteAFIRequest
	rawAFIReq := []byte(`{"event":"rfid:write_afi","uid":"E004","afi":"07"}`)
	payload, err = ParseRFIDPayload(rawAFIReq)
	if err != nil {
		t.Fatalf("ParseRFIDPayload failed: %v", err)
	}
	if _, ok := payload.(*WriteAFIRequest); !ok {
		t.Fatalf("expected *WriteAFIRequest, got %T", payload)
	}

	// 9. Quoted JSON string payload
	quotedTag := []byte(`"{\"event\":\"rfid:tag_read\",\"uid\":\"E00402\",\"itemId\":\"67890\"}"`)
	payload, err = ParseRFIDPayload(quotedTag)
	if err != nil {
		t.Fatalf("ParseRFIDPayload failed for quoted JSON: %v", err)
	}
	tagQuoted, ok := payload.(*TagReadEvent)
	if !ok || tagQuoted.ItemID != "67890" {
		t.Fatalf("expected *TagReadEvent from quoted JSON, got %T", payload)
	}

	// 10. ParseEvent with wsbroker Event envelope and source as reader name
	evt := &wsbrokerclient.Event{
		Source:  "rfid:reader:desk01",
		Channel: "rfid",
		Type:    wsbrokerclient.TypeMessage,
		Data:    rawTag,
	}
	parsedEvt, err := ParseEvent(evt)
	if err != nil {
		t.Fatalf("ParseEvent failed: %v", err)
	}
	tagFromEvt, ok := parsedEvt.(*TagReadEvent)
	if !ok {
		t.Fatalf("expected *TagReadEvent, got %T", parsedEvt)
	}
	if tagFromEvt.ClientID != "rfid:reader:desk01" {
		t.Errorf("ClientID = %q, want 'rfid:reader:desk01'", tagFromEvt.ClientID)
	}
	if tagFromEvt.Channel != "rfid" {
		t.Errorf("Channel = %q, want 'rfid'", tagFromEvt.Channel)
	}
}

func TestAFISecurityConstantsAndHelpers(t *testing.T) {
	if AFISecured != "07" {
		t.Errorf("expected AFISecured = '07', got %s", AFISecured)
	}
	if AFIUnsecured != "C2" {
		t.Errorf("expected AFIUnsecured = 'C2', got %s", AFIUnsecured)
	}
	if AFIUnlocked != "C2" {
		t.Errorf("expected AFIUnlocked = 'C2', got %s", AFIUnlocked)
	}

	if !IsAFISecured("07") || !IsAFISecured("7") || !IsAFISecured(" 07 ") {
		t.Errorf("IsAFISecured failed for '07' / '7' / ' 07 '")
	}
	if IsAFISecured("C2") || IsAFISecured("") {
		t.Errorf("IsAFISecured unexpectedly returned true for non-secured AFI")
	}

	if !IsAFIUnsecured("C2") || !IsAFIUnsecured("c2") || !IsAFIUnsecured(" C2 ") {
		t.Errorf("IsAFIUnsecured failed for 'C2' / 'c2' / ' C2 '")
	}
	if IsAFIUnsecured("07") || IsAFIUnsecured("") {
		t.Errorf("IsAFIUnsecured unexpectedly returned true for non-unsecured AFI")
	}

	if AFISecurityStatus("07") != "Gesichert" {
		t.Errorf("expected 'Gesichert', got %q", AFISecurityStatus("07"))
	}
	if AFISecurityStatus("C2") != "Entsichert" {
		t.Errorf("expected 'Entsichert', got %q", AFISecurityStatus("C2"))
	}
	if AFISecurityStatus("") != "" {
		t.Errorf("expected '', got %q", AFISecurityStatus(""))
	}

	if NormalizeAFI("7") != "07" || NormalizeAFI(" 07 ") != "07" || NormalizeAFI("c2") != "C2" {
		t.Errorf("NormalizeAFI returned unexpected result")
	}
}

func TestTagReadEvent_SecurityMethods(t *testing.T) {
	tagSecured := &TagReadEvent{AFI: "07"}
	if !tagSecured.IsSecured() {
		t.Errorf("expected tagSecured.IsSecured() = true")
	}
	if tagSecured.IsUnsecured() {
		t.Errorf("expected tagSecured.IsUnsecured() = false")
	}
	if tagSecured.SecurityStatus() != "Gesichert" {
		t.Errorf("expected SecurityStatus() = %q, got %q", "Gesichert", tagSecured.SecurityStatus())
	}

	tagUnsecured := &TagReadEvent{AFI: "C2"}
	if tagUnsecured.IsSecured() {
		t.Errorf("expected tagUnsecured.IsSecured() = false")
	}
	if !tagUnsecured.IsUnsecured() {
		t.Errorf("expected tagUnsecured.IsUnsecured() = true")
	}
	if tagUnsecured.SecurityStatus() != "Entsichert" {
		t.Errorf("expected SecurityStatus() = %q, got %q", "Entsichert", tagUnsecured.SecurityStatus())
	}

	tagNone := &TagReadEvent{}
	if tagNone.IsSecured() || tagNone.IsUnsecured() {
		t.Errorf("expected unset AFI to be neither secured nor unsecured")
	}
	if tagNone.SecurityStatus() != "" {
		t.Errorf("expected SecurityStatus() = '', got %q", tagNone.SecurityStatus())
	}
}

func TestWriteAFIResponse_SecurityMethods(t *testing.T) {
	respSecured := &WriteAFIResponse{AFI: "07", Status: "ok"}
	if !respSecured.IsSecured() {
		t.Errorf("expected respSecured.IsSecured() = true")
	}
	if respSecured.IsUnsecured() {
		t.Errorf("expected respSecured.IsUnsecured() = false")
	}
	if respSecured.SecurityStatus() != "Gesichert" {
		t.Errorf("expected SecurityStatus() = %q, got %q", "Gesichert", respSecured.SecurityStatus())
	}

	respUnsecured := &WriteAFIResponse{AFI: "c2", Status: "ok"}
	if respUnsecured.IsSecured() {
		t.Errorf("expected respUnsecured.IsSecured() = false")
	}
	if !respUnsecured.IsUnsecured() {
		t.Errorf("expected respUnsecured.IsUnsecured() = true")
	}
	if respUnsecured.SecurityStatus() != "Entsichert" {
		t.Errorf("expected SecurityStatus() = %q, got %q", "Entsichert", respUnsecured.SecurityStatus())
	}
}

func TestIDAndRefID_Constructors(t *testing.T) {
	metaReq := NewDeviceMetadataRequest()
	if metaReq.ID == "" || len(metaReq.ID) != 32 {
		t.Errorf("NewDeviceMetadataRequest() ID = %q, expected 32-char hex string", metaReq.ID)
	}

	tagReq := NewWriteTagRequest("ITEM-001", "E004010000001234")
	if tagReq.ID == "" || len(tagReq.ID) != 32 {
		t.Errorf("NewWriteTagRequest() ID = %q, expected 32-char hex string", tagReq.ID)
	}

	afiReq := NewWriteAFIRequest("07", "E004010000001234")
	if afiReq.ID == "" || len(afiReq.ID) != 32 {
		t.Errorf("NewWriteAFIRequest() ID = %q, expected 32-char hex string", afiReq.ID)
	}

	if metaReq.ID == tagReq.ID || tagReq.ID == afiReq.ID {
		t.Errorf("expected distinct generated IDs across requests")
	}
}

func TestIDAndRefID_JSONSerialization(t *testing.T) {
	payload := BaseRFIDPayload{
		Event: EventTagRead,
		ID:    "evt-12345",
		RefID: "req-67890",
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	str := string(data)
	if !strings.Contains(str, `"id":"evt-12345"`) {
		t.Errorf("expected json to contain \"id\":\"evt-12345\", got: %s", str)
	}
	if !strings.Contains(str, `"ref_id":"req-67890"`) {
		t.Errorf("expected json to contain \"ref_id\":\"req-67890\", got: %s", str)
	}
}

func TestParseEvent_IDAndRefID_Propagation(t *testing.T) {
	evt := &wsbrokerclient.Event{
		ID:      "env-evt-001",
		RefID:   "env-req-002",
		Source:  "reader-desk-1",
		Channel: "rfid-all",
		Type:    wsbrokerclient.TypeMessage,
		Data:    []byte(`{"event":"rfid:tag_read","uid":"E0040100","itemId":"ITEM-99"}`),
	}
	parsed, err := ParseEvent(evt)
	if err != nil {
		t.Fatalf("ParseEvent failed: %v", err)
	}
	tag, ok := parsed.(*TagReadEvent)
	if !ok {
		t.Fatalf("expected *TagReadEvent, got %T", parsed)
	}
	if tag.ID != "env-evt-001" {
		t.Errorf("tag.ID = %q, want 'env-evt-001'", tag.ID)
	}
	if tag.RefID != "env-req-002" {
		t.Errorf("tag.RefID = %q, want 'env-req-002'", tag.RefID)
	}
	if tag.ClientID != "reader-desk-1" {
		t.Errorf("tag.ClientID = %q, want 'reader-desk-1'", tag.ClientID)
	}
	if tag.Channel != "rfid-all" {
		t.Errorf("tag.Channel = %q, want 'rfid-all'", tag.Channel)
	}
}
