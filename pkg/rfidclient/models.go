package rfidclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"emperror.dev/errors"
	"github.com/je4/wsbrokerclient"
)

// Standard RFID event identifiers exchanged with rfid-headless.
const (
	EventTagRead                = "rfid:tag_read"
	EventTagsCleared            = "rfid:tags_cleared"
	EventTagsRemoved            = "rfid:tags_removed"
	EventAntennaFieldCleared    = "rfid:antenna_field_cleared"
	EventGetDeviceMetadata      = "rfid:get_device_metadata"
	EventDeviceMetadata         = "rfid:device_metadata"
	EventDeviceMetadataResponse = "rfid:device_metadata_response"
	EventWriteTag               = "rfid:write_tag"
	EventWriteTagResponse       = "rfid:write_tag_response"
	EventWriteAFI               = "rfid:write_afi"
	EventWriteAFIResponse       = "rfid:write_afi_response"
)

// Standard ISO 28560 AFI (Application Family Identifier) security byte constants.
const (
	AFISecured   = "07" // Item in library / EAS active (gesichert)
	AFIUnsecured = "C2" // Item on loan / EAS deactivated (entsichert)
	AFIUnlocked  = AFIUnsecured
)

// BaseRFIDPayload represents common fields included in RFID event payloads.
type BaseRFIDPayload struct {
	Event     string `json:"event"`
	ID        string `json:"id,omitempty"`
	RefID     string `json:"ref_id,omitempty"`
	Session   string `json:"session,omitempty"`
	Timestamp int64  `json:"timestamp,omitempty"`
	ClientID  string `json:"clientId,omitempty"`
	Channel   string `json:"channel,omitempty"`
}

// Time converts the millisecond epoch timestamp to a standard time.Time.
func (p *BaseRFIDPayload) Time() time.Time {
	if p.Timestamp <= 0 {
		return time.Time{}
	}
	return time.UnixMilli(p.Timestamp)
}

// TagReadEvent represents an RFID tag scan event (ISO 15693 / ISO 28560-2).
type TagReadEvent struct {
	BaseRFIDPayload
	UID        string `json:"uid"`
	ItemID     string `json:"itemId"`
	ISIL       string `json:"isil,omitempty"`
	Country    string `json:"country,omitempty"`
	CRCOk      bool   `json:"crcOk"`
	IsCRCValid bool   `json:"isCrcValid"`
	CRC        string `json:"crc,omitempty"`
	TagName    string `json:"tagName,omitempty"`
	Raw        string `json:"raw,omitempty"`
	Parts      *int   `json:"parts,omitempty"`
	PartNo     *int   `json:"partNo,omitempty"`
	UsageType  *int   `json:"usageType,omitempty"`
	Version    *int   `json:"version,omitempty"`
	AFI        string `json:"afi,omitempty"`
}

// IsCRCValidStatus returns true if the CRC validation was successful.
func (e *TagReadEvent) IsCRCValidStatus() bool {
	if e.CRCOk || e.IsCRCValid {
		return true
	}
	return strings.EqualFold(e.CRC, "OK")
}

// GetParts returns the number of total parts, defaulting to 1 if unset.
func (e *TagReadEvent) GetParts() int {
	if e.Parts != nil {
		return *e.Parts
	}
	return 1
}

// GetPartNo returns the 1-based part index, defaulting to 1 if unset.
func (e *TagReadEvent) GetPartNo() int {
	if e.PartNo != nil {
		return *e.PartNo
	}
	return 1
}

// GetUsageType returns the ISO 28560 usage type, defaulting to 1 (circulating item) if unset.
func (e *TagReadEvent) GetUsageType() int {
	if e.UsageType != nil {
		return *e.UsageType
	}
	return 1
}

// IsMultiPart returns true if this item is part of a multi-part set (parts > 1).
func (e *TagReadEvent) IsMultiPart() bool {
	return e.GetParts() > 1
}

// NormalizeAFI normalizes an AFI hex string for comparison (trimmed and uppercase, with single-digit hex padded to 2 chars).
func NormalizeAFI(afi string) string {
	s := strings.ToUpper(strings.TrimSpace(afi))
	if len(s) == 1 {
		s = "0" + s
	}
	return s
}

// IsAFISecured returns true if the AFI byte corresponds to secured state ("07" or "7").
func IsAFISecured(afi string) bool {
	return NormalizeAFI(afi) == AFISecured
}

// IsAFIUnsecured returns true if the AFI byte corresponds to unsecured state ("C2").
func IsAFIUnsecured(afi string) bool {
	return NormalizeAFI(afi) == AFIUnsecured
}

// AFISecurityStatus returns "Gesichert" for 07, "Entsichert" for C2, or empty string if unset/other.
func AFISecurityStatus(afi string) string {
	switch NormalizeAFI(afi) {
	case AFISecured:
		return "Gesichert"
	case AFIUnsecured:
		return "Entsichert"
	default:
		return ""
	}
}

// IsSecured returns true if the tag AFI indicates secured status (07).
func (e *TagReadEvent) IsSecured() bool {
	return IsAFISecured(e.AFI)
}

// IsUnsecured returns true if the tag AFI indicates unsecured status (C2).
func (e *TagReadEvent) IsUnsecured() bool {
	return IsAFIUnsecured(e.AFI)
}

// SecurityStatus returns "Gesichert" for 07, "Entsichert" for C2, or empty string if unset/other.
func (e *TagReadEvent) SecurityStatus() string {
	return AFISecurityStatus(e.AFI)
}

// Summary returns a concise human-readable description of the tag read.
func (e *TagReadEvent) Summary() string {
	crcStatus := "ERROR"
	if e.IsCRCValidStatus() {
		crcStatus = "OK"
	}
	afiStr := ""
	if e.AFI != "" {
		afiStr = fmt.Sprintf(" | AFI: %s", e.AFI)
	}
	return fmt.Sprintf("[TAG READ] UID: %s | ItemID: %s | ISIL: %s | Country: %s%s | CRC: %s",
		e.UID, e.ItemID, e.ISIL, e.Country, afiStr, crcStatus)
}

// TagsClearedEvent represents a reader cleared event (all RFID tags removed from reader field).
type TagsClearedEvent struct {
	BaseRFIDPayload
	Status    string `json:"status"`
	TotalTags int    `json:"totalTags"`
}

// Summary returns a concise human-readable description of the tags cleared event.
func (e *TagsClearedEvent) Summary() string {
	return "[READER CLEARED] All RFID tags removed from reader field"
}

// DeviceMetadataRequest represents a query for reader hardware status and metadata.
type DeviceMetadataRequest struct {
	Event string `json:"event"`
	ID    string `json:"id,omitempty"`
	RefID string `json:"ref_id,omitempty"`
}

// NewDeviceMetadataRequest creates a new query request for reader hardware metadata.
func NewDeviceMetadataRequest() DeviceMetadataRequest {
	return DeviceMetadataRequest{
		Event: EventGetDeviceMetadata,
		ID:    wsbrokerclient.GenerateEventID(),
	}
}

// DeviceMetadataResponse represents reader hardware metadata and initialization diagnostics.
type DeviceMetadataResponse struct {
	BaseRFIDPayload
	Status          string `json:"status"`
	DeviceID        string `json:"deviceId"`
	DeviceType      string `json:"deviceType"`
	InitDescription string `json:"initDescription"`
	Description     string `json:"description,omitempty"`
}

// IsSuccess returns true if the status equals "ok".
func (r *DeviceMetadataResponse) IsSuccess() bool {
	return strings.EqualFold(r.Status, "ok")
}

// GetDescription returns the full init diagnostic report / device description.
func (r *DeviceMetadataResponse) GetDescription() string {
	if r.InitDescription != "" {
		return r.InitDescription
	}
	return r.Description
}

// Summary returns a concise human-readable description of the device metadata.
func (r *DeviceMetadataResponse) Summary() string {
	desc := r.GetDescription()
	if len(desc) > 60 {
		desc = desc[:57] + "..."
	}
	return fmt.Sprintf("[DEVICE METADATA] Status: %s | ID: %s | Type: %s | Description: %s",
		r.Status, r.DeviceID, r.DeviceType, desc)
}

// WriteTagRequest represents a command to write ISO 28560-2 library metadata and optional AFI to an RFID tag.
type WriteTagRequest struct {
	Event     string `json:"event"`
	ID        string `json:"id,omitempty"`
	RefID     string `json:"ref_id,omitempty"`
	UID       string `json:"uid,omitempty"`
	ItemID    string `json:"itemId"`
	ISIL      string `json:"isil,omitempty"`
	Country   string `json:"country,omitempty"`
	Parts     int    `json:"parts,omitempty"`
	PartNo    int    `json:"partNo,omitempty"`
	UsageType int    `json:"usageType,omitempty"`
	AFI       string `json:"afi,omitempty"`
}

// NewWriteTagRequest creates a new WriteTagRequest with sensible defaults.
func NewWriteTagRequest(itemID string, uid ...string) WriteTagRequest {
	var targetUID string
	if len(uid) > 0 {
		targetUID = uid[0]
	}
	return WriteTagRequest{
		Event:     EventWriteTag,
		ID:        wsbrokerclient.GenerateEventID(),
		UID:       targetUID,
		ItemID:    itemID,
		Parts:     1,
		PartNo:    1,
		UsageType: 1,
	}
}

// Validate checks whether the write tag request contains mandatory fields.
func (r *WriteTagRequest) Validate() error {
	if strings.TrimSpace(r.ItemID) == "" {
		return errors.New("write tag request: itemId must not be empty")
	}
	if r.Parts <= 0 {
		r.Parts = 1
	}
	if r.PartNo <= 0 {
		r.PartNo = 1
	}
	if r.UsageType <= 0 {
		r.UsageType = 1
	}
	return nil
}

// WriteTagResponse represents the result of an RFID tag write operation.
type WriteTagResponse struct {
	BaseRFIDPayload
	Status    string `json:"status"`
	UID       string `json:"uid,omitempty"`
	ItemID    string `json:"itemId,omitempty"`
	ISIL      string `json:"isil,omitempty"`
	Country   string `json:"country,omitempty"`
	Parts     int    `json:"parts,omitempty"`
	PartNo    int    `json:"partNo,omitempty"`
	UsageType int    `json:"usageType,omitempty"`
	AFI       string `json:"afi,omitempty"`
	Message   string `json:"message,omitempty"`
	Error     string `json:"error,omitempty"`
}

// IsSuccess returns true if the write was successful (status == "ok").
func (r *WriteTagResponse) IsSuccess() bool {
	return strings.EqualFold(r.Status, "ok")
}

// GetError returns the error description if write failed.
func (r *WriteTagResponse) GetError() string {
	if r.Error != "" {
		return r.Error
	}
	if !r.IsSuccess() {
		return r.Message
	}
	return ""
}

// Summary returns a concise human-readable description of the write tag result.
func (r *WriteTagResponse) Summary() string {
	if r.IsSuccess() {
		afiStr := ""
		if r.AFI != "" {
			afiStr = " | AFI: " + r.AFI
		}
		return fmt.Sprintf("[WRITE TAG] Status: OK | UID: %s | ItemID: %s | ISIL: %s | Country: %s%s",
			r.UID, r.ItemID, r.ISIL, r.Country, afiStr)
	}
	errText := r.GetError()
	return fmt.Sprintf("[WRITE TAG ERROR] UID: %s | Error: %s", r.UID, errText)
}

// WriteAFIRequest represents a command to write or toggle the AFI byte (e.g. EAS security state).
type WriteAFIRequest struct {
	Event string `json:"event"`
	ID    string `json:"id,omitempty"`
	RefID string `json:"ref_id,omitempty"`
	UID   string `json:"uid,omitempty"`
	AFI   string `json:"afi"`
}

// NewWriteAFIRequest creates a new WriteAFIRequest for security state manipulation.
func NewWriteAFIRequest(afi string, uid ...string) WriteAFIRequest {
	var targetUID string
	if len(uid) > 0 {
		targetUID = uid[0]
	}
	return WriteAFIRequest{
		Event: EventWriteAFI,
		ID:    wsbrokerclient.GenerateEventID(),
		UID:   targetUID,
		AFI:   afi,
	}
}

// Validate checks whether AFI value is specified.
func (r *WriteAFIRequest) Validate() error {
	if strings.TrimSpace(r.AFI) == "" {
		return errors.New("write afi request: afi byte must not be empty")
	}
	return nil
}

// WriteAFIResponse represents the result of writing the AFI byte.
type WriteAFIResponse struct {
	BaseRFIDPayload
	Status  string `json:"status"`
	UID     string `json:"uid,omitempty"`
	AFI     string `json:"afi,omitempty"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

// IsSuccess returns true if the AFI write was successful.
func (r *WriteAFIResponse) IsSuccess() bool {
	return strings.EqualFold(r.Status, "ok")
}

// IsSecured returns true if the response AFI indicates secured status (07).
func (r *WriteAFIResponse) IsSecured() bool {
	return IsAFISecured(r.AFI)
}

// IsUnsecured returns true if the response AFI indicates unsecured status (C2).
func (r *WriteAFIResponse) IsUnsecured() bool {
	return IsAFIUnsecured(r.AFI)
}

// SecurityStatus returns "Gesichert" for 07, "Entsichert" for C2, or empty string if unset/other.
func (r *WriteAFIResponse) SecurityStatus() string {
	return AFISecurityStatus(r.AFI)
}

// GetError returns the error description if AFI write failed.
func (r *WriteAFIResponse) GetError() string {
	if r.Error != "" {
		return r.Error
	}
	if !r.IsSuccess() {
		return r.Message
	}
	return ""
}

// Summary returns a concise human-readable description of the AFI write result.
func (r *WriteAFIResponse) Summary() string {
	if r.IsSuccess() {
		return fmt.Sprintf("[WRITE AFI] Status: OK | UID: %s | AFI: %s", r.UID, r.AFI)
	}
	errText := r.GetError()
	return fmt.Sprintf("[WRITE AFI ERROR] UID: %s | Error: %s", r.UID, errText)
}

// ParseRFIDPayload unmarshals raw JSON bytes into the concrete typed RFID payload struct.
func ParseRFIDPayload(data []byte) (any, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return nil, errors.New("empty payload data")
	}

	// If data is a quoted JSON string, unquote it
	if len(trimmed) > 0 && trimmed[0] == '"' {
		var unquoted string
		if err := json.Unmarshal(trimmed, &unquoted); err == nil && len(unquoted) > 0 {
			data = []byte(unquoted)
		}
	}

	var header struct {
		Event  string `json:"event"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(data, &header); err != nil {
		return nil, errors.Wrap(err, "failed to decode json header")
	}

	switch header.Event {
	case EventTagRead:
		var tag TagReadEvent
		if err := json.Unmarshal(data, &tag); err != nil {
			return nil, errors.Wrap(err, "failed to unmarshal TagReadEvent")
		}
		return &tag, nil

	case EventTagsCleared, EventTagsRemoved, EventAntennaFieldCleared:
		var cleared TagsClearedEvent
		if err := json.Unmarshal(data, &cleared); err != nil {
			return nil, errors.Wrap(err, "failed to unmarshal TagsClearedEvent")
		}
		return &cleared, nil

	case EventDeviceMetadataResponse, EventDeviceMetadata:
		var meta DeviceMetadataResponse
		if err := json.Unmarshal(data, &meta); err != nil {
			return nil, errors.Wrap(err, "failed to unmarshal DeviceMetadataResponse")
		}
		return &meta, nil

	case EventGetDeviceMetadata:
		var req DeviceMetadataRequest
		if err := json.Unmarshal(data, &req); err != nil {
			return nil, errors.Wrap(err, "failed to unmarshal DeviceMetadataRequest")
		}
		return &req, nil

	case EventWriteTagResponse:
		var writeResp WriteTagResponse
		if err := json.Unmarshal(data, &writeResp); err != nil {
			return nil, errors.Wrap(err, "failed to unmarshal WriteTagResponse")
		}
		return &writeResp, nil

	case EventWriteTag:
		if header.Status != "" {
			var writeResp WriteTagResponse
			if err := json.Unmarshal(data, &writeResp); err != nil {
				return nil, errors.Wrap(err, "failed to unmarshal WriteTagResponse")
			}
			return &writeResp, nil
		}
		var writeReq WriteTagRequest
		if err := json.Unmarshal(data, &writeReq); err != nil {
			return nil, errors.Wrap(err, "failed to unmarshal WriteTagRequest")
		}
		return &writeReq, nil

	case EventWriteAFIResponse:
		var afiResp WriteAFIResponse
		if err := json.Unmarshal(data, &afiResp); err != nil {
			return nil, errors.Wrap(err, "failed to unmarshal WriteAFIResponse")
		}
		return &afiResp, nil

	case EventWriteAFI:
		if header.Status != "" {
			var afiResp WriteAFIResponse
			if err := json.Unmarshal(data, &afiResp); err != nil {
				return nil, errors.Wrap(err, "failed to unmarshal WriteAFIResponse")
			}
			return &afiResp, nil
		}
		var afiReq WriteAFIRequest
		if err := json.Unmarshal(data, &afiReq); err != nil {
			return nil, errors.Wrap(err, "failed to unmarshal WriteAFIRequest")
		}
		return &afiReq, nil

	default:
		return nil, errors.Errorf("unrecognized rfid event type: %q", header.Event)
	}
}

// ParseEvent extracts and parses the RFID payload from a wsbroker event envelope.
func ParseEvent(evt *wsbrokerclient.Event) (any, error) {
	if evt == nil {
		return nil, errors.New("event envelope is nil")
	}
	if evt.Type != "" && evt.Type != wsbrokerclient.TypeMessage {
		return nil, errors.Errorf("event type %q is not %q", evt.Type, wsbrokerclient.TypeMessage)
	}
	if len(evt.Data) == 0 {
		return nil, errors.New("event data is empty")
	}
	payload, err := ParseRFIDPayload(evt.Data)
	if err != nil {
		return nil, err
	}
	switch v := payload.(type) {
	case *TagReadEvent:
		if v.ID == "" && evt.ID != "" {
			v.ID = evt.ID
		}
		if v.RefID == "" && evt.RefID != "" {
			v.RefID = evt.RefID
		}
		if evt.Source != "" {
			v.ClientID = evt.Source
		}
		if v.Channel == "" && evt.Channel != "" {
			v.Channel = evt.Channel
		}
	case *TagsClearedEvent:
		if v.ID == "" && evt.ID != "" {
			v.ID = evt.ID
		}
		if v.RefID == "" && evt.RefID != "" {
			v.RefID = evt.RefID
		}
		if evt.Source != "" {
			v.ClientID = evt.Source
		}
		if v.Channel == "" && evt.Channel != "" {
			v.Channel = evt.Channel
		}
	case *DeviceMetadataResponse:
		if v.ID == "" && evt.ID != "" {
			v.ID = evt.ID
		}
		if v.RefID == "" && evt.RefID != "" {
			v.RefID = evt.RefID
		}
		if evt.Source != "" {
			v.ClientID = evt.Source
		}
		if v.Channel == "" && evt.Channel != "" {
			v.Channel = evt.Channel
		}
	case *DeviceMetadataRequest:
		if v.ID == "" && evt.ID != "" {
			v.ID = evt.ID
		}
		if v.RefID == "" && evt.RefID != "" {
			v.RefID = evt.RefID
		}
	case *WriteTagResponse:
		if v.ID == "" && evt.ID != "" {
			v.ID = evt.ID
		}
		if v.RefID == "" && evt.RefID != "" {
			v.RefID = evt.RefID
		}
		if evt.Source != "" {
			v.ClientID = evt.Source
		}
		if v.Channel == "" && evt.Channel != "" {
			v.Channel = evt.Channel
		}
	case *WriteTagRequest:
		if v.ID == "" && evt.ID != "" {
			v.ID = evt.ID
		}
		if v.RefID == "" && evt.RefID != "" {
			v.RefID = evt.RefID
		}
	case *WriteAFIResponse:
		if v.ID == "" && evt.ID != "" {
			v.ID = evt.ID
		}
		if v.RefID == "" && evt.RefID != "" {
			v.RefID = evt.RefID
		}
		if evt.Source != "" {
			v.ClientID = evt.Source
		}
		if v.Channel == "" && evt.Channel != "" {
			v.Channel = evt.Channel
		}
	case *WriteAFIRequest:
		if v.ID == "" && evt.ID != "" {
			v.ID = evt.ID
		}
		if v.RefID == "" && evt.RefID != "" {
			v.RefID = evt.RefID
		}
	}
	return payload, nil
}
