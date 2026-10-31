package rfidclient

import (
	"encoding/json"
	"strings"
	"testing"

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
				"readerId": "gate-south"
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
			var tag TagReadData
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
		"readerId": "reader-01"
	}`

	var cleared TagsClearedData
	if err := json.Unmarshal([]byte(raw), &cleared); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if cleared.Status != "empty" {
		t.Errorf("Status = %q, want 'empty'", cleared.Status)
	}
	if cleared.TotalTags != 0 {
		t.Errorf("TotalTags = %d, want 0", cleared.TotalTags)
	}
	if cleared.ReaderID != "reader-01" {
		t.Errorf("ReaderID = %q, want 'reader-01'", cleared.ReaderID)
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
		"readerId": "reader-main"
	}`

	var resp DeviceMetadataResponseData
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
	if resp.ReaderID != "reader-main" {
		t.Errorf("ReaderID = %q, want 'reader-main'", resp.ReaderID)
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

	var resp WriteTagResponseData
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

	errResp := WriteTagResponseData{
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
	if req.UID != "E004015099887766" || len(req.UIDs) != 1 {
		t.Errorf("unexpected req: %+v", req)
	}

	multiReq := NewWriteAFIRequest("07", "TAG-1", "TAG-2")
	if len(multiReq.UIDs) != 2 || multiReq.UID != "TAG-1" {
		t.Errorf("unexpected multiReq: %+v", multiReq)
	}

	batchReq := NewWriteAFIBatchRequest("07", []string{"TAG-A", "TAG-B"})
	if len(batchReq.UIDs) != 2 || batchReq.UID != "TAG-A" {
		t.Errorf("unexpected batchReq: %+v", batchReq)
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

	var resp WriteAFIResponseData
	if err := json.Unmarshal([]byte(respJSON), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if !resp.IsSuccess() || resp.IsPartial() || resp.IsError() {
		t.Error("expected IsSuccess() = true, IsPartial() = false, IsError() = false")
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

	// Test Batch Multi-Tag Success Response
	batchSuccessJSON := `{
		"event": "rfid:write_afi_response",
		"status": "ok",
		"uids": ["TAG-1", "TAG-2"],
		"successfulUids": ["TAG-1", "TAG-2"],
		"results": [
			{"uid": "TAG-1", "status": "ok"},
			{"uid": "TAG-2", "status": "ok"}
		],
		"afi": "C2"
	}`
	var batchSuccessResp WriteAFIResponseData
	if err := json.Unmarshal([]byte(batchSuccessJSON), &batchSuccessResp); err != nil {
		t.Fatalf("failed to unmarshal batch success: %v", err)
	}
	if !batchSuccessResp.IsSuccess() || batchSuccessResp.IsPartial() {
		t.Error("batchSuccessResp should be success and not partial")
	}
	if !batchSuccessResp.IsTagSuccessful("TAG-1") || !batchSuccessResp.IsTagSuccessful("TAG-2") {
		t.Error("TAG-1 and TAG-2 should be successful")
	}
	if batchSuccessResp.IsTagSuccessful("TAG-3") {
		t.Error("TAG-3 should not be successful")
	}

	// Test Batch Partial Failure Response
	batchPartialJSON := `{
		"event": "rfid:write_afi_response",
		"status": "partial",
		"uids": ["TAG-1", "TAG-2"],
		"successfulUids": ["TAG-1"],
		"failedUids": {
			"TAG-2": "AFI locked"
		},
		"results": [
			{"uid": "TAG-1", "status": "ok"},
			{"uid": "TAG-2", "status": "error", "error": "AFI locked"}
		],
		"afi": "07",
		"error": "1 of 2 tags failed"
	}`
	var batchPartialResp WriteAFIResponseData
	if err := json.Unmarshal([]byte(batchPartialJSON), &batchPartialResp); err != nil {
		t.Fatalf("failed to unmarshal batch partial: %v", err)
	}
	if batchPartialResp.IsSuccess() || !batchPartialResp.IsPartial() || batchPartialResp.IsError() {
		t.Error("batchPartialResp should be partial (IsSuccess=false, IsPartial=true, IsError=false)")
	}
	if !batchPartialResp.IsTagSuccessful("TAG-1") {
		t.Error("TAG-1 should be successful in partial response")
	}
	if batchPartialResp.IsTagSuccessful("TAG-2") {
		t.Error("TAG-2 should not be successful in partial response")
	}
	if batchPartialResp.GetTagError("TAG-2") != "AFI locked" {
		t.Errorf("GetTagError(TAG-2) = %q, want 'AFI locked'", batchPartialResp.GetTagError("TAG-2"))
	}
	if batchPartialResp.GetTagError("TAG-1") != "" {
		t.Errorf("GetTagError(TAG-1) = %q, want empty string for successful tag", batchPartialResp.GetTagError("TAG-1"))
	}
	if batchPartialResp.Summary() == "" {
		t.Error("Summary() on partial response should not be empty")
	}

	// Test Single Tag Error does not bleed onto other UIDs
	singleTagErrResp := WriteAFIResponseData{
		Status: "error",
		UID:    "TAG-FAIL",
		Error:  "Transponder write error",
		AFI:    "07",
	}
	if singleTagErrResp.IsTagSuccessful("TAG-FAIL") {
		t.Error("TAG-FAIL should not be successful")
	}
	if singleTagErrResp.GetTagError("TAG-FAIL") != "Transponder write error" {
		t.Errorf("GetTagError(TAG-FAIL) = %q, want 'Transponder write error'", singleTagErrResp.GetTagError("TAG-FAIL"))
	}
	if singleTagErrResp.IsTagSuccessful("TAG-OTHER") {
		t.Error("TAG-OTHER should not be marked successful from unrelated tag's response")
	}
	if singleTagErrResp.GetTagError("TAG-OTHER") != "" {
		t.Errorf("GetTagError(TAG-OTHER) = %q, want empty string for unrelated tag", singleTagErrResp.GetTagError("TAG-OTHER"))
	}
}

func TestParseRFIDPayloadAndParseEvent(t *testing.T) {
	// 1. TagRead
	rawTag := []byte(`{"event":"rfid/tag_read","uid":"E00401","itemId":"12345"}`)
	payload, err := ParseRFIDPayload(rawTag)
	if err != nil {
		t.Fatalf("ParseRFIDPayload failed: %v", err)
	}
	tag, ok := payload.(TagReadData)
	if !ok || tag.ItemID != "12345" {
		t.Fatalf("expected TagReadData, got %T", payload)
	}

	// 2. TagsCleared
	rawCleared := []byte(`{"event":"rfid/tags_cleared","status":"cleared","totalTags":0}`)
	payload, err = ParseRFIDPayload(rawCleared)
	if err != nil {
		t.Fatalf("ParseRFIDPayload failed: %v", err)
	}
	cleared, ok := payload.(TagsClearedData)
	if !ok || cleared.Status != "cleared" {
		t.Fatalf("expected TagsClearedData, got %T", payload)
	}

	// 3. DeviceMetadataResponse
	rawMeta := []byte(`{"event":"rfid/device_metadata_response","status":"ok","deviceId":"DEV-1"}`)
	payload, err = ParseRFIDPayload(rawMeta)
	if err != nil {
		t.Fatalf("ParseRFIDPayload failed: %v", err)
	}
	meta, ok := payload.(DeviceMetadataResponseData)
	if !ok || meta.DeviceID != "DEV-1" {
		t.Fatalf("expected DeviceMetadataResponseData, got %T", payload)
	}

	// 4. DeviceMetadataRequest
	rawMetaReq := []byte(`{"event":"rfid/get_device_metadata"}`)
	payload, err = ParseRFIDPayload(rawMetaReq)
	if err != nil {
		t.Fatalf("ParseRFIDPayload failed: %v", err)
	}
	if _, ok := payload.(DeviceMetadataRequest); !ok {
		t.Fatalf("expected DeviceMetadataRequest, got %T", payload)
	}

	// 5. WriteTagResponse
	rawWriteResp := []byte(`{"event":"rfid/write_tag_response","status":"ok","uid":"E004"}`)
	payload, err = ParseRFIDPayload(rawWriteResp)
	if err != nil {
		t.Fatalf("ParseRFIDPayload failed: %v", err)
	}
	if _, ok := payload.(WriteTagResponseData); !ok {
		t.Fatalf("expected WriteTagResponseData, got %T", payload)
	}

	// 6. WriteTagRequest
	rawWriteReq := []byte(`{"event":"rfid/write_tag","itemId":"ITEM-1","uid":"E004"}`)
	payload, err = ParseRFIDPayload(rawWriteReq)
	if err != nil {
		t.Fatalf("ParseRFIDPayload failed: %v", err)
	}
	if _, ok := payload.(WriteTagRequest); !ok {
		t.Fatalf("expected WriteTagRequest, got %T", payload)
	}

	// 7. WriteAFIResponse
	rawAFIResp := []byte(`{"event":"rfid/write_afi_response","status":"ok","afi":"07"}`)
	payload, err = ParseRFIDPayload(rawAFIResp)
	if err != nil {
		t.Fatalf("ParseRFIDPayload failed: %v", err)
	}
	if _, ok := payload.(WriteAFIResponseData); !ok {
		t.Fatalf("expected WriteAFIResponseData, got %T", payload)
	}

	// 8. WriteAFIRequest
	rawAFIReq := []byte(`{"event":"rfid/write_afi","uid":"E004","afi":"07"}`)
	payload, err = ParseRFIDPayload(rawAFIReq)
	if err != nil {
		t.Fatalf("ParseRFIDPayload failed: %v", err)
	}
	if _, ok := payload.(WriteAFIRequest); !ok {
		t.Fatalf("expected WriteAFIRequest, got %T", payload)
	}

	// 9. Quoted JSON string payload
	quotedTag := []byte(`"{\"event\":\"rfid/tag_read\",\"uid\":\"E00402\",\"itemId\":\"67890\"}"`)
	payload, err = ParseRFIDPayload(quotedTag)
	if err != nil {
		t.Fatalf("ParseRFIDPayload failed for quoted JSON: %v", err)
	}
	tagQuoted, ok := payload.(TagReadData)
	if !ok || tagQuoted.ItemID != "67890" {
		t.Fatalf("expected TagReadData from quoted JSON, got %T", payload)
	}

	// 10. ParseEvent with wsbroker Event envelope and source as reader name
	evt := &wsbrokerclient.RawEvent{
		BaseEvent: wsbrokerclient.BaseEvent{
			Source:  "rfid:reader:desk01",
			Channel: "rfid",
			Type:    EventTagRead,
		},
		Data: rawTag,
	}
	parsedEvt, err := ParseEvent(evt)
	if err != nil {
		t.Fatalf("ParseEvent failed: %v", err)
	}
	tagFromEvt, ok := parsedEvt.(*TagReadEvent)
	if !ok {
		t.Fatalf("expected *TagReadEvent, got %T", parsedEvt)
	}
	if tagFromEvt.Data.ReaderID != "rfid:reader:desk01" && tagFromEvt.Source != "rfid:reader:desk01" {
		t.Errorf("ReaderID = %q, want 'rfid:reader:desk01'", tagFromEvt.Data.ReaderID)
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
	tagSecured := &TagReadData{AFI: "07"}
	if !tagSecured.IsSecured() {
		t.Errorf("expected tagSecured.IsSecured() = true")
	}
	if tagSecured.IsUnsecured() {
		t.Errorf("expected tagSecured.IsUnsecured() = false")
	}
	if tagSecured.SecurityStatus() != "Gesichert" {
		t.Errorf("expected SecurityStatus() = %q, got %q", "Gesichert", tagSecured.SecurityStatus())
	}

	tagUnsecured := &TagReadData{AFI: "C2"}
	if tagUnsecured.IsSecured() {
		t.Errorf("expected tagUnsecured.IsSecured() = false")
	}
	if !tagUnsecured.IsUnsecured() {
		t.Errorf("expected tagUnsecured.IsUnsecured() = true")
	}
	if tagUnsecured.SecurityStatus() != "Entsichert" {
		t.Errorf("expected SecurityStatus() = %q, got %q", "Entsichert", tagUnsecured.SecurityStatus())
	}

	tagNone := &TagReadData{}
	if tagNone.IsSecured() || tagNone.IsUnsecured() {
		t.Errorf("expected unset AFI to be neither secured nor unsecured")
	}
	if tagNone.SecurityStatus() != "" {
		t.Errorf("expected SecurityStatus() = '', got %q", tagNone.SecurityStatus())
	}
}

func TestWriteAFIResponse_SecurityMethods(t *testing.T) {
	respSecured := &WriteAFIResponseData{AFI: "07", Status: "ok"}
	if !respSecured.IsSecured() {
		t.Errorf("expected respSecured.IsSecured() = true")
	}
	if respSecured.IsUnsecured() {
		t.Errorf("expected respSecured.IsUnsecured() = false")
	}
	if respSecured.SecurityStatus() != "Gesichert" {
		t.Errorf("expected SecurityStatus() = %q, got %q", "Gesichert", respSecured.SecurityStatus())
	}

	respUnsecured := &WriteAFIResponseData{AFI: "c2", Status: "ok"}
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
	evt1, err := wsbrokerclient.NewRawEvent(EventGetDeviceMetadata, "src", "tgt", NewDeviceMetadataRequest())
	if err != nil || evt1.ID == "" {
		t.Errorf("NewRawEvent(EventGetDeviceMetadata) ID = %q, err = %v", evt1.ID, err)
	}

	evt2, err := wsbrokerclient.NewRawEvent(EventWriteTag, "src", "tgt", NewWriteTagRequest("ITEM-001", "E004010000001234"))
	if err != nil || evt2.ID == "" {
		t.Errorf("NewRawEvent(EventWriteTag) ID = %q, err = %v", evt2.ID, err)
	}

	evt3, err := wsbrokerclient.NewRawEvent(EventWriteAFI, "src", "tgt", NewWriteAFIRequest("07", "E004010000001234"))
	if err != nil || evt3.ID == "" {
		t.Errorf("NewRawEvent(EventWriteAFI) ID = %q, err = %v", evt3.ID, err)
	}

	if evt1.ID == evt2.ID || evt2.ID == evt3.ID {
		t.Errorf("expected distinct generated IDs across request events")
	}
}

func TestIDAndRefID_JSONSerialization(t *testing.T) {
	payload := wsbrokerclient.BaseEvent{
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
	if !strings.Contains(str, `"refId":"req-67890"`) {
		t.Errorf("expected json to contain \"refId\":\"req-67890\", got: %s", str)
	}
}

func TestParseEvent_IDAndRefID_Propagation(t *testing.T) {
	evt := &wsbrokerclient.RawEvent{
		BaseEvent: wsbrokerclient.BaseEvent{
			ID:      "env-evt-001",
			RefID:   "env-req-002",
			Source:  "reader-desk-1",
			Channel: "rfid-all",
			Type:    EventTagRead,
		},
		Data: []byte(`{"uid":"E0040100","itemId":"ITEM-99"}`),
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
	if tag.Data.ReaderID != "reader-desk-1" {
		t.Errorf("tag.Data.ReaderID = %q, want 'reader-desk-1'", tag.Data.ReaderID)
	}
	if tag.Channel != "rfid-all" {
		t.Errorf("tag.Channel = %q, want 'rfid-all'", tag.Channel)
	}
	if tag.Data.UID != "E0040100" {
		t.Errorf("tag.Data.UID = %q, want 'E0040100'", tag.Data.UID)
	}
	if tag.Data.ItemID != "ITEM-99" {
		t.Errorf("tag.Data.ItemID = %q, want 'ITEM-99'", tag.Data.ItemID)
	}
}

func TestWriteTagBatch_ModelsAndConstructors(t *testing.T) {
	// 1. NewWriteTagBatchRequest
	items := []TagWriteItem{
		{UID: "TAG-001", ItemID: "BOOK-1", Parts: 2, PartNo: 1, AFI: "07"},
		{UID: "TAG-002", ItemID: "BOOK-2", AFI: "C2"},
	}
	req := NewWriteTagBatchRequest(items)
	if req.UID != "TAG-001" {
		t.Errorf("expected primary UID = 'TAG-001', got %q", req.UID)
	}
	if len(req.Tags) != 2 {
		t.Fatalf("expected 2 tags in request, got %d", len(req.Tags))
	}
	if req.Tags[1].Parts != 1 || req.Tags[1].PartNo != 1 || req.Tags[1].UsageType != 1 {
		t.Errorf("expected default values populated for item 1, got %+v", req.Tags[1])
	}
	if err := req.Validate(); err != nil {
		t.Errorf("expected batch request to be valid, got: %v", err)
	}

	// 2. NewWriteTagBatchRequestFromMap
	itemMap := map[string]string{
		"TAG-10": "ITEM-10",
		"TAG-20": "ITEM-20",
	}
	reqFromMap := NewWriteTagBatchRequestFromMap(itemMap)
	if len(reqFromMap.Tags) != 2 {
		t.Fatalf("expected 2 tags from map, got %d", len(reqFromMap.Tags))
	}
	if err := reqFromMap.Validate(); err != nil {
		t.Errorf("expected map request to be valid, got: %v", err)
	}

	// 3. Validation failure cases
	invalidReq := WriteTagRequest{
		Tags: []TagWriteItem{
			{UID: "", ItemID: ""},
		},
	}
	if err := invalidReq.Validate(); err == nil {
		t.Error("expected validation error for empty TagWriteItem")
	}

	singleInvalid := WriteTagRequest{}
	if err := singleInvalid.Validate(); err == nil {
		t.Error("expected validation error for single request with empty itemId")
	}
}

func TestWriteTagBatch_JSONSerialization(t *testing.T) {
	req := NewWriteTagBatchRequest([]TagWriteItem{
		{UID: "E004015099887766", ItemID: "3012345678", ISIL: "000001-1", Country: "CH", Parts: 2, PartNo: 1, AFI: "07"},
		{UID: "E004015099887777", ItemID: "3012345679", ISIL: "000001-1", Country: "CH", Parts: 2, PartNo: 2, AFI: "C2"},
	})
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var parsed WriteTagRequest
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}
	if len(parsed.Tags) != 2 {
		t.Fatalf("expected 2 tags, got %d", len(parsed.Tags))
	}
	if parsed.Tags[0].UID != "E004015099887766" || parsed.Tags[0].ItemID != "3012345678" {
		t.Errorf("unexpected parsed tag 0: %+v", parsed.Tags[0])
	}
	if parsed.Tags[1].UID != "E004015099887777" || parsed.Tags[1].AFI != "C2" {
		t.Errorf("unexpected parsed tag 1: %+v", parsed.Tags[1])
	}
}

func TestWriteTagResponse_BatchSuccess(t *testing.T) {
	jsonResp := `{
		"event": "rfid:write_tag_response",
		"status": "ok",
		"successfulUids": ["TAG1", "TAG2"],
		"results": [
			{"uid": "TAG1", "itemId": "3012345678", "status": "ok"},
			{"uid": "TAG2", "itemId": "3012345679", "status": "ok"}
		],
		"message": "2 tags written successfully"
	}`

	var resp WriteTagResponseData
	if err := json.Unmarshal([]byte(jsonResp), &resp); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if !resp.IsSuccess() {
		t.Error("expected IsSuccess() = true")
	}
	if resp.IsPartial() {
		t.Error("expected IsPartial() = false")
	}
	if resp.IsError() {
		t.Error("expected IsError() = false")
	}
	if !resp.IsTagSuccessful("TAG1") || !resp.IsTagSuccessful("TAG2") {
		t.Error("expected both TAG1 and TAG2 to be successful")
	}
	if resp.IsTagSuccessful("TAG3") {
		t.Error("expected TAG3 to be false")
	}
	if !strings.Contains(resp.Summary(), "2 tags written successfully") {
		t.Errorf("unexpected summary: %s", resp.Summary())
	}
}

func TestWriteTagResponse_BatchPartialFailure(t *testing.T) {
	jsonResp := `{
		"event": "rfid:write_tag_response",
		"status": "partial",
		"successfulUids": ["TAG1"],
		"failedUids": {
			"TAG2": "lock bit set"
		},
		"results": [
			{"uid": "TAG1", "status": "ok"},
			{"uid": "TAG2", "status": "error", "error": "lock bit set"}
		],
		"message": "1/2 tags written successfully",
		"error": "1 tag failed"
	}`

	var resp WriteTagResponseData
	if err := json.Unmarshal([]byte(jsonResp), &resp); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if resp.IsSuccess() {
		t.Error("expected IsSuccess() = false")
	}
	if !resp.IsPartial() {
		t.Error("expected IsPartial() = true")
	}
	if resp.IsError() {
		t.Error("expected IsError() = false")
	}
	if !resp.IsTagSuccessful("TAG1") {
		t.Error("expected TAG1 to be successful")
	}
	if resp.IsTagSuccessful("TAG2") {
		t.Error("expected TAG2 to be unsuccessful")
	}
	if resp.GetTagError("TAG2") != "lock bit set" {
		t.Errorf("expected tag error 'lock bit set', got %q", resp.GetTagError("TAG2"))
	}
	if !strings.Contains(resp.Summary(), "PARTIAL") {
		t.Errorf("unexpected summary: %s", resp.Summary())
	}
}

func TestParseRFIDPayload_WriteTagBatch(t *testing.T) {
	payloadBytes := []byte(`{
		"event": "rfid/write_tag",
		"tags": [
			{"uid": "TAG-1", "itemId": "BOOK-1"}
		]
	}`)
	payload, err := ParseRFIDPayload(payloadBytes)
	if err != nil {
		t.Fatalf("ParseRFIDPayload failed: %v", err)
	}
	req, ok := payload.(WriteTagRequest)
	if !ok {
		t.Fatalf("expected WriteTagRequest, got %T", payload)
	}
	if len(req.Tags) != 1 || req.Tags[0].ItemID != "BOOK-1" {
		t.Errorf("unexpected parsed request: %+v", req)
	}

	respBytes := []byte(`{
		"event": "rfid/write_tag_response",
		"status": "ok",
		"successfulUids": ["TAG-1"]
	}`)
	respPayload, err := ParseRFIDPayload(respBytes)
	if err != nil {
		t.Fatalf("ParseRFIDPayload failed: %v", err)
	}
	resp, ok := respPayload.(WriteTagResponseData)
	if !ok {
		t.Fatalf("expected WriteTagResponseData, got %T", respPayload)
	}
	if !resp.IsTagSuccessful("TAG-1") {
		t.Error("expected TAG-1 to be successful")
	}
}

func TestMultiReaderPayloadModels(t *testing.T) {
	// 1. TagReadEvent with readerId
	tagJSON := `{
		"event": "rfid/tag_read",
		"readerId": "gate-01",
		"uid": "E00401509200B02A",
		"itemId": "12345678",
		"country": "CH",
		"isil": "ISIL-001",
		"parts": 1,
		"partNo": 1,
		"crcOk": true,
		"afi": "07"
	}`

	parsedTag, err := ParseRFIDPayload([]byte(tagJSON))
	if err != nil {
		t.Fatalf("ParseRFIDPayload failed for tag: %v", err)
	}
	tagEvent, ok := parsedTag.(TagReadData)
	if !ok {
		t.Fatalf("expected TagReadData, got %T", parsedTag)
	}
	if tagEvent.GetReaderID() != "gate-01" {
		t.Errorf("GetReaderID() = %q, want 'gate-01'", tagEvent.GetReaderID())
	}
	if !strings.Contains(tagEvent.Summary(), "Reader: gate-01") {
		t.Errorf("Summary() should include reader: %s", tagEvent.Summary())
	}

	// 2. TagsClearedEvent with readerId
	clearedJSON := `{
		"event": "rfid/tags_cleared",
		"readerId": "gate-02",
		"status": "cleared"
	}`
	parsedCleared, err := ParseRFIDPayload([]byte(clearedJSON))
	if err != nil {
		t.Fatalf("ParseRFIDPayload failed for cleared: %v", err)
	}
	clearedEvent, ok := parsedCleared.(TagsClearedData)
	if !ok {
		t.Fatalf("expected TagsClearedData, got %T", parsedCleared)
	}
	if clearedEvent.GetReaderID() != "gate-02" {
		t.Errorf("GetReaderID() = %q, want 'gate-02'", clearedEvent.GetReaderID())
	}
	if !strings.Contains(clearedEvent.Summary(), "Reader: gate-02") {
		t.Errorf("Summary() should include reader: %s", clearedEvent.Summary())
	}

	// 3. DeviceMetadataRequest constructors and methods
	metaReq := NewDeviceMetadataRequest("gate-03")
	if metaReq.GetReaderID() != "gate-03" {
		t.Errorf("GetReaderID() = %q, want 'gate-03'", metaReq.GetReaderID())
	}
	metaReq2 := NewDeviceMetadataRequestForReader("gate-04")
	if metaReq2.GetReaderID() != "gate-04" {
		t.Errorf("GetReaderID() = %q, want 'gate-04'", metaReq2.GetReaderID())
	}
	metaReq.WithReaderID("gate-05")
	if metaReq.GetReaderID() != "gate-05" {
		t.Errorf("WithReaderID failed: %s", metaReq.GetReaderID())
	}

	// 4. DeviceMetadataResponse with readerId
	metaJSON := `{
		"event": "rfid/device_metadata",
		"readerId": "gate-01",
		"status": "ok",
		"deviceId": "OBID-LRU1002-01",
		"deviceType": "FEIG LRU1002",
		"initDescription": "Firmware v04.01"
	}`
	parsedMeta, err := ParseRFIDPayload([]byte(metaJSON))
	if err != nil {
		t.Fatalf("ParseRFIDPayload failed for metadata: %v", err)
	}
	metaResp, ok := parsedMeta.(DeviceMetadataResponseData)
	if !ok {
		t.Fatalf("expected DeviceMetadataResponseData, got %T", parsedMeta)
	}
	if metaResp.GetReaderID() != "gate-01" {
		t.Errorf("GetReaderID() = %q, want 'gate-01'", metaResp.GetReaderID())
	}
	if !strings.Contains(metaResp.Summary(), "Reader: gate-01") {
		t.Errorf("Summary() should include reader: %s", metaResp.Summary())
	}

	// 5. WriteTagRequest with readerId
	writeTagReq := NewWriteTagRequest("BOOK-123", "UID-123")
	writeTagReq.WithReaderID("gate-01")
	if writeTagReq.GetReaderID() != "gate-01" {
		t.Errorf("GetReaderID() = %q, want 'gate-01'", writeTagReq.GetReaderID())
	}
	reqData, err := json.Marshal(writeTagReq)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	if !strings.Contains(string(reqData), `"readerId":"gate-01"`) {
		t.Errorf("marshaled json missing readerId: %s", string(reqData))
	}

	// 6. WriteAFIRequest with readerId
	writeAFIReq := NewWriteAFIRequest(AFISecured, "UID-123")
	writeAFIReq.WithReaderID("gate-01")
	if writeAFIReq.GetReaderID() != "gate-01" {
		t.Errorf("GetReaderID() = %q, want 'gate-01'", writeAFIReq.GetReaderID())
	}
	afiData, err := json.Marshal(writeAFIReq)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	if !strings.Contains(string(afiData), `"readerId":"gate-01"`) {
		t.Errorf("marshaled json missing readerId: %s", string(afiData))
	}

	// 7. WriteTagResponse & WriteAFIResponse summaries with reader_id
	writeTagResp := WriteTagResponseData{
		ReaderID: "gate-01",
		Status:   "ok",
		UID:      "UID-123",
		ItemID:   "BOOK-123",
	}
	if !strings.Contains(writeTagResp.Summary(), "Reader: gate-01") {
		t.Errorf("WriteTagResponse summary should include reader: %s", writeTagResp.Summary())
	}

	writeAFIResp := WriteAFIResponseData{
		ReaderID: "gate-01",
		Status:   "ok",
		UID:      "UID-123",
		AFI:      "07",
	}
	if !strings.Contains(writeAFIResp.Summary(), "Reader: gate-01") {
		t.Errorf("WriteAFIResponse summary should include reader: %s", writeAFIResp.Summary())
	}
}
