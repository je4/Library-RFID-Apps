package desk

import (
	"fmt"
	"strings"
	"time"

	"github.com/je4/Library-RFID-Apps/pkg/rfidclient"
)

// TagItem represents a view-friendly RFID tag record for dashboard rendering.
type TagItem struct {
	ID                string    `json:"id"`
	UID               string    `json:"uid"`
	ItemID            string    `json:"itemId"`
	ISIL              string    `json:"isil"`
	Country           string    `json:"country"`
	CRCOk             bool      `json:"crcOk"`
	CRCStatus         string    `json:"crcStatus"`
	Parts             int       `json:"parts"`
	PartNo            int       `json:"partNo"`
	IsMultiPart       bool      `json:"isMultiPart"`
	AFI               string    `json:"afi"`
	AFISecured        bool      `json:"afiSecured"`
	AFIUnsecured      bool      `json:"afiUnsecured"`
	AFISecurityStatus string    `json:"afiSecurityStatus"`
	UsageType         int       `json:"usageType"`
	AFIWriteStatus    string    `json:"afiWriteStatus,omitempty"` // "success", "error", "pending"
	AFIWriteError     string    `json:"afiWriteError,omitempty"`  // specific error message
	AFIWriteTime      time.Time `json:"afiWriteTime,omitempty"`   // timestamp of last write attempt
	AFIWritePending   bool      `json:"afiWritePending,omitempty"`
	Timestamp         int64     `json:"timestamp"`
	TimeFormatted     string    `json:"timeFormatted"`
	DateFormatted     string    `json:"dateFormatted"`
	Raw               string    `json:"raw,omitempty"`
	ClientID          string    `json:"clientId,omitempty"`
	Channel           string    `json:"channel,omitempty"`
	ReadTime          time.Time `json:"readTime"`
}

// NewTagItem converts a low-level TagReadEvent into a display-oriented TagItem.
func NewTagItem(tag *rfidclient.TagReadEvent) *TagItem {
	if tag == nil {
		return &TagItem{}
	}

	readTime := tag.Time()
	if readTime.IsZero() {
		readTime = time.Now()
	}

	ts := tag.Timestamp
	if ts <= 0 {
		ts = readTime.UnixMilli()
	}

	crcOk := tag.IsCRCValidStatus()
	crcStatus := "Fehler"
	if crcOk {
		crcStatus = "OK (Gültig)"
	}

	parts := tag.GetParts()
	partNo := tag.GetPartNo()
	isMultiPart := tag.IsMultiPart()

	// Extract AFI if present in tag event or raw string
	afi := tag.AFI
	if afi == "" && strings.Contains(tag.Raw, "AFI:") {
		parts := strings.Split(tag.Raw, "AFI:")
		if len(parts) > 1 {
			afi = strings.TrimSpace(strings.Fields(parts[1])[0])
		}
	}

	afiSecured := rfidclient.IsAFISecured(afi)
	afiUnsecured := rfidclient.IsAFIUnsecured(afi)
	afiSecurityStatus := rfidclient.AFISecurityStatus(afi)

	// Create unique scan row ID
	rowID := fmt.Sprintf("%s-%d", strings.ReplaceAll(tag.UID, ":", ""), ts)

	return &TagItem{
		ID:                rowID,
		UID:               tag.UID,
		ItemID:            tag.ItemID,
		ISIL:              tag.ISIL,
		Country:           tag.Country,
		CRCOk:             crcOk,
		CRCStatus:         crcStatus,
		Parts:             parts,
		PartNo:            partNo,
		IsMultiPart:       isMultiPart,
		AFI:               afi,
		AFISecured:        afiSecured,
		AFIUnsecured:      afiUnsecured,
		AFISecurityStatus: afiSecurityStatus,
		UsageType:         tag.GetUsageType(),
		Timestamp:         ts,
		TimeFormatted:     readTime.Format("15:04:05"),
		DateFormatted:     readTime.Format("02.01.2006"),
		Raw:               tag.Raw,
		ClientID:          tag.ClientID,
		Channel:           tag.Channel,
		ReadTime:          readTime,
	}
}

// SetAFI updates the AFI security state and metadata on the TagItem.
func (item *TagItem) SetAFI(afi string) {
	item.AFI = afi
	item.AFISecured = rfidclient.IsAFISecured(afi)
	item.AFIUnsecured = rfidclient.IsAFIUnsecured(afi)
	item.AFISecurityStatus = rfidclient.AFISecurityStatus(afi)
}

// SetAFIWriteSuccess records a successful AFI write operation.
func (item *TagItem) SetAFIWriteSuccess(afi string) {
	item.SetAFI(afi)
	item.AFIWriteStatus = "success"
	item.AFIWriteError = ""
	item.AFIWritePending = false
	item.AFIWriteTime = time.Now()
}

// SetAFIWriteError records a failed AFI write operation.
func (item *TagItem) SetAFIWriteError(errStr string) {
	item.AFIWriteStatus = "error"
	item.AFIWriteError = strings.TrimSpace(errStr)
	item.AFIWritePending = false
	item.AFIWriteTime = time.Now()
}

// SetAFIWritePending marks an AFI write operation as in-progress.
func (item *TagItem) SetAFIWritePending() {
	item.AFIWriteStatus = "pending"
	item.AFIWritePending = true
	item.AFIWriteTime = time.Now()
}

// ClearAFIWriteStatus clears the write operation state and errors.
func (item *TagItem) ClearAFIWriteStatus() {
	item.AFIWriteStatus = ""
	item.AFIWriteError = ""
	item.AFIWritePending = false
	item.AFIWriteTime = time.Time{}
}

// DeskData encapsulates the view model passed to web templates.
type DeskData struct {
	ClientName    string
	WSBroker      string
	Channels      []string
	Channel       string
	DefaultReader string
	Addr          string
	ExtAddr       string
	Uptime        string
	Tags          []*TagItem
}

// LoanData encapsulates the view model passed to loan and return web templates.
type LoanData struct {
	ClientName        string
	WSBroker          string
	Channels          []string
	Channel           string
	DefaultReader     string
	Addr              string
	ExtAddr           string
	Uptime            string
	Tags              []*TagItem
	ErrorMessage      string
	SuccessMessage    string
	WriteSuccessCount int
	WriteErrorCount   int
	FailedItems       []string
}
