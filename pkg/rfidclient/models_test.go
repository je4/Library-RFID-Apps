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
			name: "TagRead with aliases and multi-part item",
			jsonInput: `{
				"event": "rfid:tag_read",
				"uid": "E004015099887766",
				"imtemId": "BARCODE999",
				"country": "DE",
				"crcOk": false,
				"crc": "ERROR",
				"parts": 3,
				"partno": 2,
				"typeOfUsage": 2,
				"afiByte": "C2"
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
		{
			name: "TagRead with primaryItemId alias and string CRC OK",
			jsonInput: `{
				"event": "rfid:tag_read",
				"uid": "E004011122334455",
				"primaryItemId": "ITEM-ABC",
				"crc": "ok"
			}`,
			wantUID:      "E004011122334455",
			wantItemID:   "ITEM-ABC",
			wantParts:    1,
			wantPartNo:   1,
			wantUsage:    1,
			wantMulti:    false,
			wantCRCValid: true,
			wantAFI:      "",
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
	tests := []struct {
		name       string
		jsonInput  string
		wantID     string
		wantType   string
		wantDesc   string
		wantStatus string
		wantOK     bool
	}{
		{
			name: "Standard response format",
			jsonInput: `{
				"event": "rfid:device_metadata_response",
				"status": "ok",
				"deviceId": "0052E688",
				"deviceType": "ISC.MR102-USB",
				"initDescription": "FEIG Electronic ISC.MR102-USB (0052E688)\nReader initialized.",
				"timestamp": 1792756802045,
				"clientId": "rfid-reader-station-01"
			}`,
			wantID:     "0052E688",
			wantType:   "ISC.MR102-USB",
			wantDesc:   "FEIG Electronic ISC.MR102-USB (0052E688)\nReader initialized.",
			wantStatus: "ok",
			wantOK:     true,
		},
		{
			name: "Alias fields format",
			jsonInput: `{
				"event": "rfid:device_metadata",
				"status": "OK",
				"deviceID": "0099AABB",
				"Device Type": "ISC.PRH101",
				"Komplettbeschreibung aus dem init": "Complete init summary"
			}`,
			wantID:     "0099AABB",
			wantType:   "ISC.PRH101",
			wantDesc:   "Complete init summary",
			wantStatus: "OK",
			wantOK:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var resp DeviceMetadataResponse
			if err := json.Unmarshal([]byte(tt.jsonInput), &resp); err != nil {
				t.Fatalf("failed to unmarshal: %v", err)
			}

			if resp.DeviceID != tt.wantID {
				t.Errorf("DeviceID = %q, want %q", resp.DeviceID, tt.wantID)
			}
			if resp.DeviceType != tt.wantType {
				t.Errorf("DeviceType = %q, want %q", resp.DeviceType, tt.wantType)
			}
			if resp.GetDescription() != tt.wantDesc {
				t.Errorf("GetDescription() = %q, want %q", resp.GetDescription(), tt.wantDesc)
			}
			if resp.IsSuccess() != tt.wantOK {
				t.Errorf("IsSuccess() = %v, want %v", resp.IsSuccess(), tt.wantOK)
			}
			if resp.Summary() == "" {
				t.Error("Summary() should not be empty")
			}
		})
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
		"afi": "07",
		"message": "Tag written successfully"
	}`

	var writeResp WriteTagResponse
	if err := json.Unmarshal([]byte(respJSON), &writeResp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if !writeResp.IsSuccess() {
		t.Errorf("expected IsSuccess=true, got false")
	}
	if writeResp.UID != "E004015099887766" {
		t.Errorf("UID = %q, want E004015099887766", writeResp.UID)
	}
	if writeResp.AFI != "07" {
		t.Errorf("AFI = %q, want 07", writeResp.AFI)
	}

	errRespJSON := `{
		"event": "rfid:write_tag_response",
		"status": "error",
		"uid": "E004015099887766",
		"error": "Transponder timeout"
	}`
	var errResp WriteTagResponse
	if err := json.Unmarshal([]byte(errRespJSON), &errResp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if errResp.IsSuccess() {
		t.Errorf("expected IsSuccess=false, got true")
	}
	if errResp.GetError() != "Transponder timeout" {
		t.Errorf("GetError = %q, want 'Transponder timeout'", errResp.GetError())
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
		"afi": "07",
		"message": "AFI written successfully"
	}`
	var afiResp WriteAFIResponse
	if err := json.Unmarshal([]byte(respJSON), &afiResp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if !afiResp.IsSuccess() {
		t.Errorf("expected IsSuccess=true")
	}
	if afiResp.AFI != "07" {
		t.Errorf("AFI = %q, want 07", afiResp.AFI)
	}
}

func TestParseRFIDPayloadAndParseEvent(t *testing.T) {
	// 1. TagRead
	rawTag := []byte(`{"event":"rfid:tag_read","uid":"E00401","itemId":"12345"}`)
	p1, err := ParseRFIDPayload(rawTag)
	if err != nil {
		t.Fatalf("ParseRFIDPayload failed: %v", err)
	}
	if _, ok := p1.(*TagReadEvent); !ok {
		t.Fatalf("expected *TagReadEvent, got %T", p1)
	}

	// 2. TagsCleared and variations (tags_removed, antenna_field_cleared)
	clearedVariants := []string{
		`{"event":"rfid:tags_cleared","status":"empty"}`,
		`{"event":"rfid:tags_removed","status":"removed"}`,
		`{"event":"tags_removed"}`,
		`{"event":"antenna_field_cleared"}`,
		`{"status":"removed","totalTags":0}`,
	}
	for _, cv := range clearedVariants {
		pCleared, err := ParseRFIDPayload([]byte(cv))
		if err != nil {
			t.Fatalf("ParseRFIDPayload failed for %s: %v", cv, err)
		}
		if _, ok := pCleared.(*TagsClearedEvent); !ok {
			t.Fatalf("expected *TagsClearedEvent for %s, got %T", cv, pCleared)
		}
	}

	// 3. DeviceMetadataResponse
	rawMeta := []byte(`{"event":"rfid:device_metadata_response","deviceId":"123","deviceType":"MR102"}`)
	p3, err := ParseRFIDPayload(rawMeta)
	if err != nil {
		t.Fatalf("ParseRFIDPayload failed: %v", err)
	}
	if _, ok := p3.(*DeviceMetadataResponse); !ok {
		t.Fatalf("expected *DeviceMetadataResponse, got %T", p3)
	}

	// 4. WriteTagResponse
	rawWrite := []byte(`{"event":"rfid:write_tag_response","status":"ok","uid":"E004"}`)
	p4, err := ParseRFIDPayload(rawWrite)
	if err != nil {
		t.Fatalf("ParseRFIDPayload failed: %v", err)
	}
	if _, ok := p4.(*WriteTagResponse); !ok {
		t.Fatalf("expected *WriteTagResponse, got %T", p4)
	}

	// 5. WriteAFIResponse
	rawAFI := []byte(`{"event":"rfid:write_afi_response","status":"ok","afi":"07"}`)
	p5, err := ParseRFIDPayload(rawAFI)
	if err != nil {
		t.Fatalf("ParseRFIDPayload failed: %v", err)
	}
	if _, ok := p5.(*WriteAFIResponse); !ok {
		t.Fatalf("expected *WriteAFIResponse, got %T", p5)
	}

	// 6. Unknown event
	rawUnknown := []byte(`{"event":"rfid:unknown_action"}`)
	if _, err := ParseRFIDPayload(rawUnknown); err == nil {
		t.Error("expected error for unknown event")
	}

	// 7. ParseEvent via wsbroker Event
	wsEvt, err := wsbrokerclient.NewMessageEvent("rfid:reader:01", "rfid:desk:01", TagReadEvent{
		UID:    "E004019999",
		ItemID: "55555",
		AFI:    "07",
		BaseRFIDPayload: BaseRFIDPayload{
			Event: EventTagRead,
		},
	})
	if err != nil {
		t.Fatalf("NewMessageEvent failed: %v", err)
	}

	pEvt, err := ParseEvent(wsEvt)
	if err != nil {
		t.Fatalf("ParseEvent failed: %v", err)
	}
	tagEvt, ok := pEvt.(*TagReadEvent)
	if !ok || tagEvt.UID != "E004019999" {
		t.Errorf("ParseEvent returned unexpected object: %+v", pEvt)
	}
	if tagEvt.ClientID != "rfid:reader:01" {
		t.Errorf("expected ClientID to be populated from envelope source %q, got %q", "rfid:reader:01", tagEvt.ClientID)
	}
	if tagEvt.AFI != "07" {
		t.Errorf("expected AFI to be %q, got %q", "07", tagEvt.AFI)
	}

	// 8. ParseEvent with non-message type (e.g. pong) should return error
	pongEvt, err := wsbrokerclient.NewPongEvent("source", "target", "pong")
	if err != nil {
		t.Fatalf("NewPongEvent failed: %v", err)
	}
	if _, err := ParseEvent(pongEvt); err == nil {
		t.Error("expected error when ParseEvent is called with non-message event type")
	}
}

func TestAFISecurityConstantsAndHelpers(t *testing.T) {
	// 1. Verify constant values
	if AFISecured != "07" {
		t.Errorf("expected AFISecured = %q, got %q", "07", AFISecured)
	}
	if AFIUnsecured != "C2" {
		t.Errorf("expected AFIUnsecured = %q, got %q", "C2", AFIUnsecured)
	}
	if AFIUnlocked != AFIUnsecured {
		t.Errorf("expected AFIUnlocked = AFIUnsecured (%q), got %q", AFIUnsecured, AFIUnlocked)
	}

	// 2. Test NormalizeAFI
	normTests := []struct {
		input string
		want  string
	}{
		{"07", "07"},
		{"7", "07"},
		{" 07 ", "07"},
		{"c2", "C2"},
		{"C2", "C2"},
		{" C2\n", "C2"},
		{"", ""},
		{"01", "01"},
		{"ff", "FF"},
	}
	for _, tc := range normTests {
		got := NormalizeAFI(tc.input)
		if got != tc.want {
			t.Errorf("NormalizeAFI(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}

	// 3. Test IsAFISecured and IsAFIUnsecured
	securedCases := []struct {
		afi       string
		secured   bool
		unsecured bool
		status    string
	}{
		{"07", true, false, "Gesichert"},
		{"7", true, false, "Gesichert"},
		{" 07 ", true, false, "Gesichert"},
		{"C2", false, true, "Entsichert"},
		{"c2", false, true, "Entsichert"},
		{" C2\r\n", false, true, "Entsichert"},
		{"01", false, false, ""},
		{"FF", false, false, ""},
		{"", false, false, ""},
	}
	for _, tc := range securedCases {
		if got := IsAFISecured(tc.afi); got != tc.secured {
			t.Errorf("IsAFISecured(%q) = %v, want %v", tc.afi, got, tc.secured)
		}
		if got := IsAFIUnsecured(tc.afi); got != tc.unsecured {
			t.Errorf("IsAFIUnsecured(%q) = %v, want %v", tc.afi, got, tc.unsecured)
		}
		if got := AFISecurityStatus(tc.afi); got != tc.status {
			t.Errorf("AFISecurityStatus(%q) = %q, want %q", tc.afi, got, tc.status)
		}
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

	tagOther := &TagReadEvent{AFI: "01"}
	if tagOther.IsSecured() || tagOther.IsUnsecured() {
		t.Errorf("expected tagOther IsSecured and IsUnsecured = false")
	}
	if tagOther.SecurityStatus() != "" {
		t.Errorf("expected empty SecurityStatus(), got %q", tagOther.SecurityStatus())
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

func TestParseRFIDPayload_WriteAFIAndWriteTagVariants(t *testing.T) {
	// 1. WriteAFIRequest variations
	afiRequestCases := []string{
		`{"event":"rfid:write_afi","uid":"E004015099887766","afi":"07"}`,
		`{"event":"rfid:afi","uid":"E004015099887766","afi":"07"}`,
		`{"event":"write_afi","uid":"E004015099887766","afi":"C2"}`,
		`{"event":"writeafi","uid":"E004015099887766","afi":"07"}`,
		`{"event":"afi","uid":"E004015099887766","afi":"C2"}`,
		`{"event":"rfid:set_afi","uid":"E004015099887766","afi":"07"}`,
	}
	for _, raw := range afiRequestCases {
		p, err := ParseRFIDPayload([]byte(raw))
		if err != nil {
			t.Fatalf("ParseRFIDPayload failed for %s: %v", raw, err)
		}
		req, ok := p.(*WriteAFIRequest)
		if !ok {
			t.Fatalf("expected *WriteAFIRequest for %s, got %T", raw, p)
		}
		if req.UID != "E004015099887766" {
			t.Errorf("UID = %q, want E004015099887766", req.UID)
		}
		if req.AFI != "07" && req.AFI != "C2" {
			t.Errorf("AFI = %q, want 07 or C2", req.AFI)
		}
	}

	// 2. WriteAFIResponse variations
	afiResponseCases := []string{
		`{"event":"rfid:write_afi_response","status":"ok","uid":"E004015099887766","afi":"07"}`,
		`{"event":"write_afi_response","status":"ok","uid":"E004015099887766","afi":"07"}`,
		`{"event":"rfid:write_afi","status":"ok","uid":"E004015099887766","afi":"07"}`,
	}
	for _, raw := range afiResponseCases {
		p, err := ParseRFIDPayload([]byte(raw))
		if err != nil {
			t.Fatalf("ParseRFIDPayload failed for %s: %v", raw, err)
		}
		resp, ok := p.(*WriteAFIResponse)
		if !ok {
			t.Fatalf("expected *WriteAFIResponse for %s, got %T", raw, p)
		}
		if !resp.IsSuccess() {
			t.Errorf("expected IsSuccess() = true for %s", raw)
		}
	}

	// 3. WriteTagRequest variations
	tagRequestCases := []string{
		`{"event":"rfid:write_tag","uid":"E00401","itemId":"123456"}`,
		`{"event":"write_tag","uid":"E00401","itemId":"123456"}`,
		`{"event":"writetag","uid":"E00401","itemId":"123456"}`,
		`{"event":"tag_write","uid":"E00401","itemId":"123456"}`,
	}
	for _, raw := range tagRequestCases {
		p, err := ParseRFIDPayload([]byte(raw))
		if err != nil {
			t.Fatalf("ParseRFIDPayload failed for %s: %v", raw, err)
		}
		req, ok := p.(*WriteTagRequest)
		if !ok {
			t.Fatalf("expected *WriteTagRequest for %s, got %T", raw, p)
		}
		if req.ItemID != "123456" {
			t.Errorf("ItemID = %q, want 123456", req.ItemID)
		}
	}

	// 4. DeviceMetadataRequest variations
	metaRequestCases := []string{
		`{"event":"rfid:get_device_metadata"}`,
		`{"event":"get_device_metadata"}`,
	}
	for _, raw := range metaRequestCases {
		p, err := ParseRFIDPayload([]byte(raw))
		if err != nil {
			t.Fatalf("ParseRFIDPayload failed for %s: %v", raw, err)
		}
		if _, ok := p.(*DeviceMetadataRequest); !ok {
			t.Fatalf("expected *DeviceMetadataRequest for %s, got %T", raw, p)
		}
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

	// Ensure uniqueness
	if metaReq.ID == tagReq.ID || tagReq.ID == afiReq.ID {
		t.Errorf("expected distinct generated IDs across requests")
	}
}

func TestIDAndRefID_JSONSerialization(t *testing.T) {
	// 1. With ID and RefID populated
	payload := BaseRFIDPayload{
		Event: EventTagRead,
		ID:    "evt-12345",
		RefID: "req-67890",
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	jsonStr := string(data)
	if !strings.Contains(jsonStr, `"id":"evt-12345"`) {
		t.Errorf("expected serialized JSON to contain id field: %s", jsonStr)
	}
	if !strings.Contains(jsonStr, `"ref_id":"req-67890"`) {
		t.Errorf("expected serialized JSON to contain ref_id field: %s", jsonStr)
	}

	// 2. Empty ID and RefID should be omitted
	emptyPayload := BaseRFIDPayload{
		Event: EventTagRead,
	}
	dataEmpty, err := json.Marshal(emptyPayload)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	jsonEmptyStr := string(dataEmpty)
	if strings.Contains(jsonEmptyStr, `"id"`) || strings.Contains(jsonEmptyStr, `"ref_id"`) {
		t.Errorf("expected empty id/ref_id to be omitted: %s", jsonEmptyStr)
	}
}

func TestIDAndRefID_UnmarshalJSON_Aliases(t *testing.T) {
	// 1. TagReadEvent with refId alias
	tagJSON := `{"event":"rfid:tag_read","id":"t-1","refId":"r-1","uid":"E004","itemId":"ITEM-1"}`
	var tag TagReadEvent
	if err := json.Unmarshal([]byte(tagJSON), &tag); err != nil {
		t.Fatalf("failed to unmarshal TagReadEvent: %v", err)
	}
	if tag.ID != "t-1" || tag.RefID != "r-1" {
		t.Errorf("TagReadEvent ID/RefID = (%q, %q), want (t-1, r-1)", tag.ID, tag.RefID)
	}

	// 2. DeviceMetadataResponse with ref_id
	metaRespJSON := `{"event":"rfid:device_metadata_response","id":"m-1","ref_id":"r-meta","status":"ok","deviceId":"DEV-1"}`
	var metaResp DeviceMetadataResponse
	if err := json.Unmarshal([]byte(metaRespJSON), &metaResp); err != nil {
		t.Fatalf("failed to unmarshal DeviceMetadataResponse: %v", err)
	}
	if metaResp.ID != "m-1" || metaResp.RefID != "r-meta" {
		t.Errorf("DeviceMetadataResponse ID/RefID = (%q, %q), want (m-1, r-meta)", metaResp.ID, metaResp.RefID)
	}

	// 3. WriteTagRequest with ID and RefID
	writeTagReqJSON := `{"event":"rfid:write_tag","ID":"wt-req-1","RefID":"ref-call-1","itemId":"ITEM-X"}`
	var writeTagReq WriteTagRequest
	if err := json.Unmarshal([]byte(writeTagReqJSON), &writeTagReq); err != nil {
		t.Fatalf("failed to unmarshal WriteTagRequest: %v", err)
	}
	if writeTagReq.ID != "wt-req-1" || writeTagReq.RefID != "ref-call-1" {
		t.Errorf("WriteTagRequest ID/RefID = (%q, %q), want (wt-req-1, ref-call-1)", writeTagReq.ID, writeTagReq.RefID)
	}

	// 4. WriteTagResponse with refId
	writeTagRespJSON := `{"event":"rfid:write_tag_response","id":"wt-resp-1","refId":"wt-req-1","status":"ok","itemId":"ITEM-X"}`
	var writeTagResp WriteTagResponse
	if err := json.Unmarshal([]byte(writeTagRespJSON), &writeTagResp); err != nil {
		t.Fatalf("failed to unmarshal WriteTagResponse: %v", err)
	}
	if writeTagResp.ID != "wt-resp-1" || writeTagResp.RefID != "wt-req-1" {
		t.Errorf("WriteTagResponse ID/RefID = (%q, %q), want (wt-resp-1, wt-req-1)", writeTagResp.ID, writeTagResp.RefID)
	}

	// 5. WriteAFIRequest with ref_id
	writeAFIReqJSON := `{"event":"rfid:write_afi","id":"wa-req-1","ref_id":"wa-ref-0","uid":"E004","afi":"07"}`
	var writeAFIReq WriteAFIRequest
	if err := json.Unmarshal([]byte(writeAFIReqJSON), &writeAFIReq); err != nil {
		t.Fatalf("failed to unmarshal WriteAFIRequest: %v", err)
	}
	if writeAFIReq.ID != "wa-req-1" || writeAFIReq.RefID != "wa-ref-0" {
		t.Errorf("WriteAFIRequest ID/RefID = (%q, %q), want (wa-req-1, wa-ref-0)", writeAFIReq.ID, writeAFIReq.RefID)
	}

	// 6. WriteAFIResponse with refId
	writeAFIRespJSON := `{"event":"rfid:write_afi_response","id":"wa-resp-1","refId":"wa-req-1","status":"ok","afi":"07"}`
	var writeAFIResp WriteAFIResponse
	if err := json.Unmarshal([]byte(writeAFIRespJSON), &writeAFIResp); err != nil {
		t.Fatalf("failed to unmarshal WriteAFIResponse: %v", err)
	}
	if writeAFIResp.ID != "wa-resp-1" || writeAFIResp.RefID != "wa-req-1" {
		t.Errorf("WriteAFIResponse ID/RefID = (%q, %q), want (wa-resp-1, wa-req-1)", writeAFIResp.ID, writeAFIResp.RefID)
	}
}

func TestParseEvent_IDAndRefID_Propagation(t *testing.T) {
	// 1. Envelope has ID & RefID, payload has empty ID & RefID -> propagated
	evt := &wsbrokerclient.Event{
		ID:     "env-id-999",
		RefID:  "env-ref-888",
		Source: "rfid-reader-01",
		Target: "rfid-desk",
		Type:   wsbrokerclient.TypeMessage,
		Data:   []byte(`{"event":"rfid:tag_read","uid":"E0040100","itemId":"ITEM-99"}`),
	}
	parsed, err := ParseEvent(evt)
	if err != nil {
		t.Fatalf("ParseEvent failed: %v", err)
	}
	tag, ok := parsed.(*TagReadEvent)
	if !ok {
		t.Fatalf("expected *TagReadEvent, got %T", parsed)
	}
	if tag.ID != "env-id-999" {
		t.Errorf("tag.ID = %q, want env-id-999", tag.ID)
	}
	if tag.RefID != "env-ref-888" {
		t.Errorf("tag.RefID = %q, want env-ref-888", tag.RefID)
	}
	if tag.ClientID != "rfid-reader-01" {
		t.Errorf("tag.ClientID = %q, want rfid-reader-01", tag.ClientID)
	}

	// 2. Payload already has its own ID & RefID -> preserved
	evtPreserve := &wsbrokerclient.Event{
		ID:     "env-id-ignore",
		RefID:  "env-ref-ignore",
		Source: "rfid-reader-01",
		Type:   wsbrokerclient.TypeMessage,
		Data:   []byte(`{"event":"rfid:write_tag_response","id":"payload-id-123","ref_id":"payload-ref-456","status":"ok","itemId":"ITEM-99"}`),
	}
	parsedResp, err := ParseEvent(evtPreserve)
	if err != nil {
		t.Fatalf("ParseEvent failed: %v", err)
	}
	writeResp, ok := parsedResp.(*WriteTagResponse)
	if !ok {
		t.Fatalf("expected *WriteTagResponse, got %T", parsedResp)
	}
	if writeResp.ID != "payload-id-123" {
		t.Errorf("writeResp.ID = %q, want payload-id-123", writeResp.ID)
	}
	if writeResp.RefID != "payload-ref-456" {
		t.Errorf("writeResp.RefID = %q, want payload-ref-456", writeResp.RefID)
	}
}
