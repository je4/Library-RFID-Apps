package rfidclient

import (
	"fmt"
	"strings"

	"emperror.dev/errors"
	"github.com/je4/wsbrokerclient"
)

// Write and command event identifiers.
const (
	EventWriteTag         = "rfid:write_tag"
	EventWriteTagResponse = "rfid:write_tag_response"
	EventWriteAFI         = "rfid:write_afi"
	EventWriteAFIResponse = "rfid:write_afi_response"
)

// Standard ISO 28560 AFI (Application Family Identifier) security byte constants.
const (
	AFISecured   = "07" // Item in library / EAS active (gesichert)
	AFIUnsecured = "C2" // Item on loan / EAS deactivated (entsichert)
	AFIUnlocked  = AFIUnsecured
)

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
