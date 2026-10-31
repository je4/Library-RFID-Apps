package rfidclient

import (
	"fmt"
	"strings"

	"github.com/je4/wsbrokerclient"
)

// Standard RFID event identifiers exchanged with rfid-headless.
const (
	EventTagRead             = "rfid/tag_read"
	EventTagsCleared         = "rfid/tags_cleared"
	EventTagsRemoved         = "rfid/tags_removed"
	EventAntennaFieldCleared = "rfid/antenna_field_cleared"
)

func init() {
	// Register RFID event payloads with wsbrokerclient global registry
	wsbrokerclient.RegisterPayload[TagReadData](EventTagRead)
	wsbrokerclient.RegisterPayload[TagsClearedData](EventTagsCleared)
	wsbrokerclient.RegisterPayload[DeviceMetadataResponseData](EventDeviceMetadataResponse)
	wsbrokerclient.RegisterPayload[DeviceMetadataResponseData](EventDeviceMetadata)
	wsbrokerclient.RegisterPayload[DeviceMetadataRequest](EventGetDeviceMetadata)
	wsbrokerclient.RegisterPayload[WriteTagResponseData](EventWriteTagResponse)
	wsbrokerclient.RegisterPayload[WriteTagRequest](EventWriteTag)
	wsbrokerclient.RegisterPayload[WriteAFIResponseData](EventWriteAFIResponse)
	wsbrokerclient.RegisterPayload[WriteAFIRequest](EventWriteAFI)
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
