package rfidclient

import (
	"fmt"
	"strings"
	"time"
)

// Standard RFID event identifiers exchanged with rfid-headless.
const (
	EventTagRead             = "rfid:tag_read"
	EventTagsCleared         = "rfid:tags_cleared"
	EventTagsRemoved         = "rfid:tags_removed"
	EventAntennaFieldCleared = "rfid:antenna_field_cleared"
)

// BaseRFIDPayload represents common fields included in RFID event payloads.
type BaseRFIDPayload struct {
	Event     string `json:"event"`
	ID        string `json:"id,omitempty"`
	RefID     string `json:"refId,omitempty"`
	Session   string `json:"session,omitempty"`
	Timestamp int64  `json:"timestamp,omitempty"`
	ReaderID  string `json:"readerId,omitempty"`
	Channel   string `json:"channel,omitempty"`
}

// GetReaderID returns the logical reader identifier.
func (p *BaseRFIDPayload) GetReaderID() string {
	return p.ReaderID
}

// WithReaderID sets the logical reader identifier on the payload.
func (p *BaseRFIDPayload) WithReaderID(readerID string) *BaseRFIDPayload {
	p.ReaderID = readerID
	return p
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
	readerStr := ""
	if rID := e.GetReaderID(); rID != "" {
		readerStr = fmt.Sprintf(" | Reader: %s", rID)
	}
	return fmt.Sprintf("[TAG READ] UID: %s | ItemID: %s | ISIL: %s | Country: %s%s%s | CRC: %s",
		e.UID, e.ItemID, e.ISIL, e.Country, afiStr, readerStr, crcStatus)
}

// TagsClearedEvent represents a reader cleared event (all RFID tags removed from reader field).
type TagsClearedEvent struct {
	BaseRFIDPayload
	Status    string `json:"status"`
	TotalTags int    `json:"totalTags"`
}

// Summary returns a concise human-readable description of the tags cleared event.
func (e *TagsClearedEvent) Summary() string {
	if rID := e.GetReaderID(); rID != "" {
		return fmt.Sprintf("[READER CLEARED] Reader: %s | All RFID tags removed from reader field", rID)
	}
	return "[READER CLEARED] All RFID tags removed from reader field"
}
