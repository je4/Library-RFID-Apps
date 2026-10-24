package rfidclient

import (
	"fmt"
	"slices"
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

// TagAFIResult represents the AFI write status of an individual RFID tag in a batch operation.
type TagAFIResult struct {
	UID    string `json:"uid"`
	Status string `json:"status"` // "ok" | "error"
	Error  string `json:"error,omitempty"`
}

// IsSuccess returns true if this specific tag was written successfully.
func (r *TagAFIResult) IsSuccess() bool {
	return strings.EqualFold(r.Status, "ok")
}

// WriteAFIRequest represents a command to write or toggle the AFI byte (single tag or batch).
type WriteAFIRequest struct {
	Event string   `json:"event"`
	ID    string   `json:"id,omitempty"`
	RefID string   `json:"ref_id,omitempty"`
	UID   string   `json:"uid,omitempty"`
	UIDs  []string `json:"uids,omitempty"`
	AFI   string   `json:"afi"`
}

// NewWriteAFIRequest creates a new WriteAFIRequest for security state manipulation.
// If multiple UIDs are provided, it populates both UIDs and the primary UID.
func NewWriteAFIRequest(afi string, uid ...string) WriteAFIRequest {
	var targetUID string
	var targetUIDs []string
	if len(uid) == 1 {
		targetUID = uid[0]
		targetUIDs = []string{uid[0]}
	} else if len(uid) > 1 {
		targetUID = uid[0]
		targetUIDs = uid
	}
	return WriteAFIRequest{
		Event: EventWriteAFI,
		ID:    wsbrokerclient.GenerateEventID(),
		UID:   targetUID,
		UIDs:  targetUIDs,
		AFI:   afi,
	}
}

// NewWriteAFIBatchRequest creates a new batch WriteAFIRequest for multiple tag UIDs.
func NewWriteAFIBatchRequest(afi string, uids []string) WriteAFIRequest {
	var targetUID string
	if len(uids) > 0 {
		targetUID = uids[0]
	}
	return WriteAFIRequest{
		Event: EventWriteAFI,
		ID:    wsbrokerclient.GenerateEventID(),
		UID:   targetUID,
		UIDs:  uids,
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

// WriteAFIResponse represents the result of writing the AFI byte (single tag or batch).
type WriteAFIResponse struct {
	BaseRFIDPayload
	Status         string            `json:"status"` // "ok" | "partial" | "error"
	UID            string            `json:"uid,omitempty"`
	UIDs           []string          `json:"uids,omitempty"`
	SuccessfulUIDs []string          `json:"successfulUids,omitempty"`
	FailedUIDs     map[string]string `json:"failedUids,omitempty"`
	Results        []TagAFIResult    `json:"results,omitempty"`
	AFI            string            `json:"afi,omitempty"`
	Message        string            `json:"message,omitempty"`
	Error          string            `json:"error,omitempty"`
}

// IsSuccess returns true if the AFI write was completely successful.
func (r *WriteAFIResponse) IsSuccess() bool {
	return strings.EqualFold(r.Status, "ok")
}

// IsPartial returns true if some tags succeeded and some failed ("partial").
func (r *WriteAFIResponse) IsPartial() bool {
	return strings.EqualFold(r.Status, "partial")
}

// IsError returns true if the AFI write completely failed.
func (r *WriteAFIResponse) IsError() bool {
	return strings.EqualFold(r.Status, "error") || (!r.IsSuccess() && !r.IsPartial())
}

// IsTagSuccessful checks whether a specific tag UID was written successfully.
func (r *WriteAFIResponse) IsTagSuccessful(uid string) bool {
	if uid == "" {
		return r.IsSuccess()
	}
	if slices.Contains(r.SuccessfulUIDs, uid) {
		return true
	}
	for _, res := range r.Results {
		if res.UID == uid {
			return res.IsSuccess()
		}
	}
	if r.FailedUIDs != nil {
		if _, failed := r.FailedUIDs[uid]; failed {
			return false
		}
	}
	// Fallback for single-tag responses where SuccessfulUIDs is not populated
	if r.UID == uid || (len(r.UIDs) == 0 && len(r.SuccessfulUIDs) == 0 && len(r.Results) == 0) {
		return r.IsSuccess()
	}
	return false
}

// GetTagError returns the specific error message for a tag UID, or the general error.
func (r *WriteAFIResponse) GetTagError(uid string) string {
	if r.FailedUIDs != nil {
		if errStr, failed := r.FailedUIDs[uid]; failed && errStr != "" {
			return errStr
		}
	}
	for _, res := range r.Results {
		if res.UID == uid && res.Error != "" {
			return res.Error
		}
	}
	return r.GetError()
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
		if len(r.UIDs) > 1 || len(r.SuccessfulUIDs) > 1 {
			count := len(r.SuccessfulUIDs)
			if count == 0 {
				count = len(r.UIDs)
			}
			return fmt.Sprintf("[WRITE AFI] Status: OK | AFI: %s | %d tags written successfully", r.AFI, count)
		}
		return fmt.Sprintf("[WRITE AFI] Status: OK | UID: %s | AFI: %s", r.UID, r.AFI)
	}
	if r.IsPartial() {
		return fmt.Sprintf("[WRITE AFI PARTIAL] Status: PARTIAL | AFI: %s | Successful: %d | Failed: %d | Error: %s",
			r.AFI, len(r.SuccessfulUIDs), len(r.FailedUIDs), r.GetError())
	}
	errText := r.GetError()
	if len(r.UIDs) > 1 {
		return fmt.Sprintf("[WRITE AFI ERROR] AFI: %s | Failed: %d tags | Error: %s", r.AFI, len(r.UIDs), errText)
	}
	return fmt.Sprintf("[WRITE AFI ERROR] UID: %s | Error: %s", r.UID, errText)
}
