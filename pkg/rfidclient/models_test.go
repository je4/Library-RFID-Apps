package rfidclient

import (
	"encoding/json"
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
	}{
		{
			name: "Standard rfid-headless TagRead event",
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
				"typeOfUsage": 2
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

	// 2. TagsCleared
	rawCleared := []byte(`{"event":"rfid:tags_cleared","status":"empty"}`)
	p2, err := ParseRFIDPayload(rawCleared)
	if err != nil {
		t.Fatalf("ParseRFIDPayload failed: %v", err)
	}
	if _, ok := p2.(*TagsClearedEvent); !ok {
		t.Fatalf("expected *TagsClearedEvent, got %T", p2)
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
	wsEvt, err := wsbrokerclient.NewMessageEvent("source", "target", TagReadEvent{
		UID:    "E004019999",
		ItemID: "55555",
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
}
