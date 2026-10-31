package libraryclient

import (
	"fmt"
	"slices"
	"strings"

	"emperror.dev/errors"
	"github.com/je4/wsbrokerclient"
)

// Standard RFID event identifiers exchanged with rfid-headless.
const (
	EventTagRead                = "rfid/tag_read"
	EventTagsCleared            = "rfid/tags_cleared"
	EventGetDeviceMetadata      = "rfid/get_device_metadata"
	EventDeviceMetadataResponse = "rfid/device_metadata_response"
	EventWriteTag               = "rfid/write_tag"
	EventWriteTagResponse       = "rfid/write_tag_response"
	EventWriteAFI               = "rfid/write_afi"
	EventWriteAFIResponse       = "rfid/write_afi_response"
)

// Standard ISO 28560 AFI (Application Family Identifier) security byte constants.
const (
	AFISecured   = "07" // Item in library / EAS active (gesichert)
	AFIUnsecured = "C2" // Item on loan / EAS deactivated (entsichert)
	AFIUnlocked  = AFIUnsecured
)

func init() {
	// Register RFID event payloads with wsbrokerclient global registry
	wsbrokerclient.RegisterPayload[TagReadData](EventTagRead)
	wsbrokerclient.RegisterPayload[TagsClearedData](EventTagsCleared)
	wsbrokerclient.RegisterPayload[DeviceMetadataResponseData](EventDeviceMetadataResponse)
	wsbrokerclient.RegisterPayload[DeviceMetadataRequest](EventGetDeviceMetadata)
	wsbrokerclient.RegisterPayload[WriteTagResponseData](EventWriteTagResponse)
	wsbrokerclient.RegisterPayload[WriteTagRequest](EventWriteTag)
	wsbrokerclient.RegisterPayload[WriteAFIResponseData](EventWriteAFIResponse)
	wsbrokerclient.RegisterPayload[WriteAFIRequest](EventWriteAFI)
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

// TagReadData represents the RFID transponder payload (ISO 15693 / ISO 28560-2).
type TagReadData struct {
	ReaderID   string `json:"readerId,omitempty"`
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

// GetReaderID returns the logical reader identifier.
func (e *TagReadData) GetReaderID() string {
	return e.ReaderID
}

// WithReaderID sets the logical reader identifier on the payload.
func (e *TagReadData) WithReaderID(readerID string) *TagReadData {
	e.ReaderID = readerID
	return e
}

// TagReadEvent represents a typed wsbroker event containing TagReadData.
type TagReadEvent = wsbrokerclient.Event[TagReadData]

// IsCRCValidStatus returns true if the CRC validation was successful.
func (e *TagReadData) IsCRCValidStatus() bool {
	if e.CRCOk || e.IsCRCValid {
		return true
	}
	return strings.EqualFold(e.CRC, "OK")
}

// GetParts returns the number of total parts, defaulting to 1 if unset.
func (e *TagReadData) GetParts() int {
	if e.Parts != nil {
		return *e.Parts
	}
	return 1
}

// GetPartNo returns the 1-based part index, defaulting to 1 if unset.
func (e *TagReadData) GetPartNo() int {
	if e.PartNo != nil {
		return *e.PartNo
	}
	return 1
}

// GetUsageType returns the ISO 28560 usage type, defaulting to 1 (circulating item) if unset.
func (e *TagReadData) GetUsageType() int {
	if e.UsageType != nil {
		return *e.UsageType
	}
	return 1
}

// IsMultiPart returns true if this item is part of a multi-part set (parts > 1).
func (e *TagReadData) IsMultiPart() bool {
	return e.GetParts() > 1
}

// IsSecured returns true if the tag AFI indicates secured status (07).
func (e *TagReadData) IsSecured() bool {
	return IsAFISecured(e.AFI)
}

// IsUnsecured returns true if the tag AFI indicates unsecured status (C2).
func (e *TagReadData) IsUnsecured() bool {
	return IsAFIUnsecured(e.AFI)
}

// SecurityStatus returns "Gesichert" for 07, "Entsichert" for C2, or empty string if unset/other.
func (e *TagReadData) SecurityStatus() string {
	return AFISecurityStatus(e.AFI)
}

// Summary returns a concise human-readable description of the tag read.
func (e *TagReadData) Summary() string {
	crcStatus := "ERROR"
	if e.IsCRCValidStatus() {
		crcStatus = "OK"
	}
	afiStr := ""
	if e.AFI != "" {
		afiStr = fmt.Sprintf(" | AFI: %s", e.AFI)
	}
	readerStr := ""
	if rID := e.GetReaderID(); rID != "" {
		readerStr = fmt.Sprintf(" | Reader: %s", rID)
	}
	return fmt.Sprintf("[TAG READ] UID: %s | ItemID: %s | ISIL: %s | Country: %s%s%s | CRC: %s",
		e.UID, e.ItemID, e.ISIL, e.Country, afiStr, readerStr, crcStatus)
}

// TagsClearedData represents the payload of a reader cleared event (all RFID tags removed from reader field).
type TagsClearedData struct {
	ReaderID  string `json:"readerId,omitempty"`
	Status    string `json:"status,omitempty"`
	TotalTags int    `json:"totalTags,omitempty"`
}

// GetReaderID returns the logical reader identifier.
func (e *TagsClearedData) GetReaderID() string {
	return e.ReaderID
}

// WithReaderID sets the logical reader identifier on the payload.
func (e *TagsClearedData) WithReaderID(readerID string) *TagsClearedData {
	e.ReaderID = readerID
	return e
}

// TagsClearedEvent represents a typed wsbroker event containing TagsClearedData.
type TagsClearedEvent = wsbrokerclient.Event[TagsClearedData]

// Summary returns a concise human-readable description of the tags cleared event.
func (e *TagsClearedData) Summary() string {
	if rID := e.GetReaderID(); rID != "" {
		return fmt.Sprintf("[READER CLEARED] Reader: %s | All RFID tags removed from reader field", rID)
	}
	return "[READER CLEARED] All RFID tags removed from reader field"
}

// DeviceMetadataRequest represents a query for RFID reader hardware status and metadata.
type DeviceMetadataRequest struct {
	ReaderID string `json:"readerId,omitempty"`
}

// GetReaderID returns the target reader identifier.
func (r *DeviceMetadataRequest) GetReaderID() string {
	return r.ReaderID
}

// WithReaderID sets the target reader identifier on the request.
func (r *DeviceMetadataRequest) WithReaderID(readerID string) *DeviceMetadataRequest {
	r.ReaderID = readerID
	return r
}

// NewDeviceMetadataRequest creates a new query request for reader hardware metadata.
func NewDeviceMetadataRequest(readerID ...string) DeviceMetadataRequest {
	var rID string
	if len(readerID) > 0 {
		rID = readerID[0]
	}
	req := DeviceMetadataRequest{}
	req.ReaderID = rID
	return req
}

// NewDeviceMetadataRequestForReader creates a new query request for a specific reader hardware metadata.
func NewDeviceMetadataRequestForReader(readerID string) DeviceMetadataRequest {
	return NewDeviceMetadataRequest(readerID)
}

// DeviceMetadataResponseData represents RFID reader hardware metadata and initialization diagnostics data.
type DeviceMetadataResponseData struct {
	ReaderID        string `json:"readerId,omitempty"`
	Status          string `json:"status,omitempty"`
	DeviceID        string `json:"deviceId,omitempty"`
	DeviceType      string `json:"deviceType,omitempty"`
	InitDescription string `json:"initDescription,omitempty"`
	Description     string `json:"description,omitempty"`
}

// GetReaderID returns the logical reader identifier.
func (r *DeviceMetadataResponseData) GetReaderID() string {
	return r.ReaderID
}

// DeviceMetadataResponse represents a typed wsbroker event containing DeviceMetadataResponseData.
type DeviceMetadataResponse = wsbrokerclient.Event[DeviceMetadataResponseData]

// IsSuccess returns true if the status equals "ok".
func (r *DeviceMetadataResponseData) IsSuccess() bool {
	return strings.EqualFold(r.Status, "ok")
}

// GetDescription returns the full init diagnostic report / device description.
func (r *DeviceMetadataResponseData) GetDescription() string {
	if r.InitDescription != "" {
		return r.InitDescription
	}
	return r.Description
}

// Summary returns a concise human-readable description of the device metadata.
func (r *DeviceMetadataResponseData) Summary() string {
	desc := r.GetDescription()
	if len(desc) > 60 {
		desc = desc[:57] + "..."
	}
	readerStr := ""
	if rID := r.GetReaderID(); rID != "" {
		readerStr = fmt.Sprintf(" | Reader: %s", rID)
	}
	return fmt.Sprintf("[DEVICE METADATA] Status: %s | ID: %s | Type: %s%s | Description: %s",
		r.Status, r.DeviceID, r.DeviceType, readerStr, desc)
}

// TagWriteItem represents an individual tag's write parameters in a batch write request.
type TagWriteItem struct {
	UID       string `json:"uid"`
	ItemID    string `json:"itemId"`
	ISIL      string `json:"isil,omitempty"`
	Country   string `json:"country,omitempty"`
	Parts     int    `json:"parts,omitempty"`
	PartNo    int    `json:"partNo,omitempty"`
	UsageType int    `json:"usageType,omitempty"`
	AFI       string `json:"afi,omitempty"`
}

// TagWriteResult represents the outcome for an individual transponder in a batch tag write operation.
type TagWriteResult struct {
	UID     string `json:"uid"`
	ItemID  string `json:"itemId,omitempty"`
	PartNo  int    `json:"partNo,omitempty"`
	AFI     string `json:"afi,omitempty"`
	Status  string `json:"status"` // "ok" | "error"
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

// IsSuccess returns true if this specific tag write succeeded.
func (r *TagWriteResult) IsSuccess() bool {
	return strings.EqualFold(r.Status, "ok")
}

// WriteTagRequest represents a command to write ISO 28560-2 library metadata and optional AFI to an RFID tag (single or batch).
type WriteTagRequest struct {
	ReaderID  string         `json:"readerId,omitempty"`
	UID       string         `json:"uid,omitempty"`
	ItemID    string         `json:"itemId,omitempty"`
	ISIL      string         `json:"isil,omitempty"`
	Country   string         `json:"country,omitempty"`
	Parts     int            `json:"parts,omitempty"`
	PartNo    int            `json:"partNo,omitempty"`
	UsageType int            `json:"usageType,omitempty"`
	AFI       string         `json:"afi,omitempty"`
	Tags      []TagWriteItem `json:"tags,omitempty"`
	Items     []TagWriteItem `json:"items,omitempty"`
	UIDs      []string       `json:"uids,omitempty"`
	ItemIDs   []string       `json:"itemIds,omitempty"`
	AFIs      []string       `json:"afis,omitempty"`
}

// GetReaderID returns the target reader identifier.
func (r *WriteTagRequest) GetReaderID() string {
	return r.ReaderID
}

// WithReaderID sets the target reader identifier on the write request.
func (r *WriteTagRequest) WithReaderID(readerID string) *WriteTagRequest {
	r.ReaderID = readerID
	return r
}

// NewWriteTagRequest creates a single-tag write request with default parts=1, partNo=1, usageType=1.
func NewWriteTagRequest(itemID string, uid ...string) WriteTagRequest {
	var targetUID string
	if len(uid) > 0 {
		targetUID = strings.TrimSpace(uid[0])
	}
	return WriteTagRequest{
		ItemID:    itemID,
		UID:       targetUID,
		Parts:     1,
		PartNo:    1,
		UsageType: 1,
	}
}

// NewWriteTagBatchRequest creates a batch tag write request from a list of TagWriteItem structs.
func NewWriteTagBatchRequest(tags []TagWriteItem) WriteTagRequest {
	return WriteTagRequest{
		Tags: tags,
	}
}

// NewWriteTagBatchRequestFromMap creates a batch tag write request mapping UID -> ItemID.
func NewWriteTagBatchRequestFromMap(tagMap map[string]string, isil string, afi ...string) WriteTagRequest {
	var tags []TagWriteItem
	var defaultAFI string
	if len(afi) > 0 {
		defaultAFI = afi[0]
	}
	for uid, itemID := range tagMap {
		tags = append(tags, TagWriteItem{
			UID:       uid,
			ItemID:    itemID,
			ISIL:      isil,
			Parts:     1,
			PartNo:    1,
			UsageType: 1,
			AFI:       defaultAFI,
		})
	}
	return WriteTagRequest{
		Tags: tags,
	}
}

// Validate checks if the write tag request contains valid parameters.
func (r *WriteTagRequest) Validate() error {
	allTags := r.GetAllTags()
	if len(allTags) > 0 {
		for i, tag := range allTags {
			if strings.TrimSpace(tag.ItemID) == "" {
				return errors.Errorf("invalid batch item at index %d: itemId is required", i)
			}
		}
		return nil
	}

	if strings.TrimSpace(r.ItemID) == "" {
		return errors.New("write tag request requires a non-empty itemId or at least one tag in batch")
	}
	return nil
}

// GetAllTags returns all configured tag items from single fields or batch arrays.
func (r *WriteTagRequest) GetAllTags() []TagWriteItem {
	if len(r.Tags) > 0 {
		return r.Tags
	}
	if len(r.Items) > 0 {
		return r.Items
	}
	if len(r.UIDs) > 0 {
		var list []TagWriteItem
		for i, uid := range r.UIDs {
			item := TagWriteItem{
				UID:       uid,
				ISIL:      r.ISIL,
				Country:   r.Country,
				Parts:     r.Parts,
				PartNo:    r.PartNo,
				UsageType: r.UsageType,
				AFI:       r.AFI,
			}
			if i < len(r.ItemIDs) {
				item.ItemID = r.ItemIDs[i]
			} else {
				item.ItemID = r.ItemID
			}
			if i < len(r.AFIs) {
				item.AFI = r.AFIs[i]
			}
			list = append(list, item)
		}
		return list
	}
	return nil
}

// WriteTagResponseData represents the response payload of a tag write operation.
type WriteTagResponseData struct {
	ReaderID       string            `json:"readerId,omitempty"`
	Status         string            `json:"status"`
	UID            string            `json:"uid,omitempty"`
	ItemID         string            `json:"itemId,omitempty"`
	ISIL           string            `json:"isil,omitempty"`
	AFI            string            `json:"afi,omitempty"`
	TagStatus      string            `json:"tagStatus,omitempty"`
	AFIStatus      string            `json:"afiStatus,omitempty"`
	Results        []TagWriteResult  `json:"results,omitempty"`
	SuccessfulUIDs []string          `json:"successfulUids,omitempty"`
	FailedUIDs     map[string]string `json:"failedUids,omitempty"`
	Error          string            `json:"error,omitempty"`
	Message        string            `json:"message,omitempty"`
}

// GetReaderID returns the logical reader identifier.
func (r *WriteTagResponseData) GetReaderID() string {
	return r.ReaderID
}

// WriteTagResponse represents a typed wsbroker event containing WriteTagResponseData.
type WriteTagResponse = wsbrokerclient.Event[WriteTagResponseData]

// IsSuccess returns true if the write tag operation was entirely successful.
func (r *WriteTagResponseData) IsSuccess() bool {
	if strings.EqualFold(r.Status, "ok") {
		return true
	}
	return strings.EqualFold(r.TagStatus, "ok") && (r.AFIStatus == "" || strings.EqualFold(r.AFIStatus, "ok"))
}

// IsPartial returns true if some tags succeeded while others failed.
func (r *WriteTagResponseData) IsPartial() bool {
	return strings.EqualFold(r.Status, "partial") || (len(r.SuccessfulUIDs) > 0 && len(r.FailedUIDs) > 0)
}

// IsError returns true if the tag write completely failed.
func (r *WriteTagResponseData) IsError() bool {
	return strings.EqualFold(r.Status, "error") || (!r.IsSuccess() && !r.IsPartial())
}

// IsTagSuccessful checks whether a specific tag UID was written successfully.
func (r *WriteTagResponseData) IsTagSuccessful(uid string) bool {
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
		if r.IsSuccess() || r.IsPartial() {
			return true
		}
	}
	if r.UID != "" {
		if r.UID == uid {
			return r.IsSuccess()
		}
		return false
	}
	if len(r.SuccessfulUIDs) == 0 && len(r.Results) == 0 && (r.FailedUIDs == nil || len(r.FailedUIDs) == 0) {
		return r.IsSuccess()
	}
	return false
}

// GetTagError returns the specific error message for a tag UID, or the general error.
func (r *WriteTagResponseData) GetTagError(uid string) string {
	if r.FailedUIDs != nil {
		if errStr, failed := r.FailedUIDs[uid]; failed {
			if errStr != "" {
				return errStr
			}
			return r.GetError()
		}
		if r.IsTagSuccessful(uid) {
			return ""
		}
	}
	for _, res := range r.Results {
		if res.UID == uid {
			if res.IsSuccess() {
				return ""
			}
			if res.Error != "" {
				return res.Error
			}
			return r.GetError()
		}
	}
	if r.UID != "" && r.UID != uid {
		return ""
	}
	if r.IsTagSuccessful(uid) {
		return ""
	}
	return r.GetError()
}

// GetError returns error or failure message if present.
func (r *WriteTagResponseData) GetError() string {
	if r.Error != "" {
		return r.Error
	}
	return r.Message
}

// Summary returns a concise human-readable description of the write tag response.
func (r *WriteTagResponseData) Summary() string {
	readerStr := ""
	if rID := r.GetReaderID(); rID != "" {
		readerStr = fmt.Sprintf(" | Reader: %s", rID)
	}
	if r.IsSuccess() {
		return fmt.Sprintf("[TAG WRITE OK] UID: %s | ItemID: %s%s", r.UID, r.ItemID, readerStr)
	}
	if r.IsPartial() {
		return fmt.Sprintf("[TAG WRITE PARTIAL] Success: %d | Failed: %d%s", len(r.SuccessfulUIDs), len(r.FailedUIDs), readerStr)
	}
	return fmt.Sprintf("[TAG WRITE FAILED] Error: %s%s", r.GetError(), readerStr)
}

// TagAFIResult represents the outcome for an individual transponder in a batch AFI write operation.
type TagAFIResult struct {
	UID     string `json:"uid"`
	AFI     string `json:"afi"`
	Status  string `json:"status"` // "ok" | "error"
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

// IsSuccess returns true if this specific tag AFI write succeeded.
func (r *TagAFIResult) IsSuccess() bool {
	return strings.EqualFold(r.Status, "ok")
}

// WriteAFIRequest represents a command to set/toggle the AFI byte on a transponder or batch of transponders.
type WriteAFIRequest struct {
	ReaderID string   `json:"readerId,omitempty"`
	AFI      string   `json:"afi"`
	UID      string   `json:"uid,omitempty"`
	UIDs     []string `json:"uids,omitempty"`
}

// GetReaderID returns the target reader identifier.
func (r *WriteAFIRequest) GetReaderID() string {
	return r.ReaderID
}

// WithReaderID sets the target reader identifier on the write request.
func (r *WriteAFIRequest) WithReaderID(readerID string) *WriteAFIRequest {
	r.ReaderID = readerID
	return r
}

// NewWriteAFIRequest creates a new single AFI write request.
func NewWriteAFIRequest(afi string, uid ...string) WriteAFIRequest {
	req := WriteAFIRequest{
		AFI: NormalizeAFI(afi),
	}
	if len(uid) > 0 && strings.TrimSpace(uid[0]) != "" {
		req.UID = strings.TrimSpace(uid[0])
	}
	return req
}

// NewWriteAFIBatchRequest creates a new batch AFI write request for multiple UIDs.
func NewWriteAFIBatchRequest(afi string, uids []string) WriteAFIRequest {
	return WriteAFIRequest{
		AFI:  NormalizeAFI(afi),
		UIDs: uids,
	}
}

// Validate checks if the write AFI request contains valid parameters.
func (r *WriteAFIRequest) Validate() error {
	norm := NormalizeAFI(r.AFI)
	if norm == "" {
		return errors.New("write AFI request requires a valid non-empty AFI hex value")
	}
	return nil
}

// WriteAFIResponseData represents the response payload of an AFI write operation.
type WriteAFIResponseData struct {
	ReaderID       string            `json:"readerId,omitempty"`
	Status         string            `json:"status"`
	AFI            string            `json:"afi,omitempty"`
	UID            string            `json:"uid,omitempty"`
	UIDs           []string          `json:"uids,omitempty"`
	Results        []TagAFIResult    `json:"results,omitempty"`
	SuccessfulUIDs []string          `json:"successfulUids,omitempty"`
	FailedUIDs     map[string]string `json:"failedUids,omitempty"`
	Error          string            `json:"error,omitempty"`
	Message        string            `json:"message,omitempty"`
}

// GetReaderID returns the logical reader identifier.
func (r *WriteAFIResponseData) GetReaderID() string {
	return r.ReaderID
}

// WriteAFIResponse represents a typed wsbroker event containing WriteAFIResponseData.
type WriteAFIResponse = wsbrokerclient.Event[WriteAFIResponseData]

// IsSuccess returns true if the AFI write operation was successful.
func (r *WriteAFIResponseData) IsSuccess() bool {
	return strings.EqualFold(r.Status, "ok")
}

// IsPartial returns true if some tags succeeded while others failed.
func (r *WriteAFIResponseData) IsPartial() bool {
	return strings.EqualFold(r.Status, "partial") || (len(r.SuccessfulUIDs) > 0 && len(r.FailedUIDs) > 0)
}

// IsError returns true if the AFI write completely failed.
func (r *WriteAFIResponseData) IsError() bool {
	return strings.EqualFold(r.Status, "error") || (!r.IsSuccess() && !r.IsPartial())
}

// IsTagSuccessful checks whether a specific tag UID was written successfully.
func (r *WriteAFIResponseData) IsTagSuccessful(uid string) bool {
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
		if r.IsSuccess() || r.IsPartial() {
			return true
		}
	}
	if r.UID != "" {
		if r.UID == uid {
			return r.IsSuccess()
		}
		return false
	}
	if len(r.SuccessfulUIDs) == 0 && len(r.Results) == 0 && (r.FailedUIDs == nil || len(r.FailedUIDs) == 0) {
		return r.IsSuccess()
	}
	return false
}

// GetTagError returns the specific error message for a tag UID, or the general error.
func (r *WriteAFIResponseData) GetTagError(uid string) string {
	if r.FailedUIDs != nil {
		if errStr, failed := r.FailedUIDs[uid]; failed {
			if errStr != "" {
				return errStr
			}
			return r.GetError()
		}
		if r.IsTagSuccessful(uid) {
			return ""
		}
	}
	for _, res := range r.Results {
		if res.UID == uid {
			if res.IsSuccess() {
				return ""
			}
			if res.Error != "" {
				return res.Error
			}
			return r.GetError()
		}
	}
	if r.UID != "" && r.UID != uid {
		return ""
	}
	if r.IsTagSuccessful(uid) {
		return ""
	}
	return r.GetError()
}

// GetError returns error or failure message if present.
func (r *WriteAFIResponseData) GetError() string {
	if r.Error != "" {
		return r.Error
	}
	return r.Message
}

// IsSecured returns true if the resulting AFI value represents the secured status (07).
func (r *WriteAFIResponseData) IsSecured() bool {
	return IsAFISecured(r.AFI)
}

// IsUnsecured returns true if the resulting AFI value represents the unsecured status (C2).
func (r *WriteAFIResponseData) IsUnsecured() bool {
	return IsAFIUnsecured(r.AFI)
}

// SecurityStatus returns "Gesichert", "Entsichert", or empty string.
func (r *WriteAFIResponseData) SecurityStatus() string {
	return AFISecurityStatus(r.AFI)
}

// Summary returns a concise human-readable description of the AFI write response.
func (r *WriteAFIResponseData) Summary() string {
	readerStr := ""
	if rID := r.GetReaderID(); rID != "" {
		readerStr = fmt.Sprintf(" | Reader: %s", rID)
	}
	secStr := ""
	if status := r.SecurityStatus(); status != "" {
		secStr = fmt.Sprintf(" (%s)", status)
	}
	if r.IsSuccess() {
		if len(r.SuccessfulUIDs) > 1 {
			return fmt.Sprintf("[AFI WRITE OK] AFI: %s%s | Tags: %d%s", r.AFI, secStr, len(r.SuccessfulUIDs), readerStr)
		}
		uid := r.UID
		if uid == "" && len(r.SuccessfulUIDs) == 1 {
			uid = r.SuccessfulUIDs[0]
		}
		if uid != "" {
			return fmt.Sprintf("[AFI WRITE OK] UID: %s | AFI: %s%s%s", uid, r.AFI, secStr, readerStr)
		}
		return fmt.Sprintf("[AFI WRITE OK] AFI: %s%s%s", r.AFI, secStr, readerStr)
	}
	if r.IsPartial() {
		return fmt.Sprintf("[AFI WRITE PARTIAL] AFI: %s%s | Success: %d | Failed: %d%s",
			r.AFI, secStr, len(r.SuccessfulUIDs), len(r.FailedUIDs), readerStr)
	}
	return fmt.Sprintf("[AFI WRITE FAILED] Error: %s%s", r.GetError(), readerStr)
}

// Handler function signatures for RFID events.
type (
	// TagReadHandler receives tag read scan events.
	TagReadHandler func(evt *TagReadEvent)

	// TagsClearedHandler receives reader field cleared events.
	TagsClearedHandler func(evt *TagsClearedEvent)

	// DeviceMetadataHandler receives reader hardware metadata responses.
	DeviceMetadataHandler func(evt *DeviceMetadataResponse)

	// WriteTagResponseHandler receives tag write responses.
	WriteTagResponseHandler func(evt *WriteTagResponse)

	// WriteAFIResponseHandler receives AFI byte write responses.
	WriteAFIResponseHandler func(evt *WriteAFIResponse)

	// AnyRFIDEventHandler receives any parsed RFID event payload.
	AnyRFIDEventHandler func(rfidEvt any)
)

var _ = slices.Contains[[]string, string]
