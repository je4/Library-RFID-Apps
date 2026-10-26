package desk

import (
	"fmt"
	"strings"
	"time"

	"github.com/je4/Library-RFID-Apps/pkg/rfidclient"
)

// TagItem represents a view-friendly RFID tag record for dashboard rendering.
type TagItem struct {
	ID                string     `json:"id"`
	UID               string     `json:"uid"`
	ItemID            string     `json:"itemId"`
	ISIL              string     `json:"isil"`
	ISILName          string     `json:"isilName,omitempty"`
	ISILEntry         *ISILEntry `json:"isilEntry,omitempty"`
	Country           string     `json:"country"`
	CRCOk             bool       `json:"crcOk"`
	CRCStatus         string     `json:"crcStatus"`
	Parts             int        `json:"parts"`
	PartNo            int        `json:"partNo"`
	IsMultiPart       bool       `json:"isMultiPart"`
	AFI               string     `json:"afi"`
	AFISecured        bool       `json:"afiSecured"`
	AFIUnsecured      bool       `json:"afiUnsecured"`
	AFISecurityStatus string     `json:"afiSecurityStatus"`
	UsageType         int        `json:"usageType"`
	AFIWriteStatus    string     `json:"afiWriteStatus,omitempty"` // "success", "error", "pending"
	AFIWriteError     string     `json:"afiWriteError,omitempty"`  // specific error message
	AFIWriteTime      time.Time  `json:"afiWriteTime,omitempty"`   // timestamp of last write attempt
	AFIWritePending   bool       `json:"afiWritePending,omitempty"`
	Timestamp         int64      `json:"timestamp"`
	TimeFormatted     string     `json:"timeFormatted"`
	DateFormatted     string     `json:"dateFormatted"`
	Raw               string     `json:"raw,omitempty"`
	ClientID          string     `json:"clientId,omitempty"`
	ReaderID          string     `json:"readerId,omitempty"`
	Channel           string     `json:"channel,omitempty"`
	ReadTime          time.Time  `json:"readTime"`
	StaticPath        string     `json:"staticPath,omitempty"`
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
		ReaderID:          tag.GetReaderID(),
		Channel:           tag.Channel,
		ReadTime:          readTime,
		StaticPath:        "../static",
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

// EnrichISIL looks up and attaches the ISILEntry and library original name to the TagItem.
func (item *TagItem) EnrichISIL(isilMap map[string]*ISILEntry) {
	if item == nil || isilMap == nil {
		return
	}
	entry := FindISILEntry(isilMap, item.ISIL, item.Country)
	if entry != nil {
		item.ISILEntry = entry
		if entry.NameOriginal != "" {
			item.ISILName = entry.NameOriginal
		} else if entry.NameDE != "" {
			item.ISILName = entry.NameDE
		} else if entry.NameFR != "" {
			item.ISILName = entry.NameFR
		} else if entry.NameIT != "" {
			item.ISILName = entry.NameIT
		} else if entry.NameEN != "" {
			item.ISILName = entry.NameEN
		}
	}
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
	Skin          string
	ThemeMode     string
	StaticPath    string
}

// IsLoan returns true if this data model represents the loan & return view.
func (d DeskData) IsLoan() bool {
	return false
}

// IsEncode returns true if this data model represents the encode view.
func (d DeskData) IsEncode() bool {
	return false
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
	Skin              string
	ThemeMode         string
	StaticPath        string
}

// IsLoan returns true if this data model represents the loan & return view.
func (d LoanData) IsLoan() bool {
	return true
}

// IsEncode returns true if this data model represents the encode view.
func (d LoanData) IsEncode() bool {
	return false
}

// EncodeData encapsulates the view model passed to encode web templates.
type EncodeData struct {
	ClientName     string
	WSBroker       string
	Channels       []string
	Channel        string
	DefaultReader  string
	Addr           string
	ExtAddr        string
	Uptime         string
	Tags           []*TagItem
	ErrorMessage   string
	SuccessMessage string
	FailedItems    []string
	DefaultISIL    string
	DefaultAFI     string
	DefaultParts   int
	Skin           string
	ThemeMode      string
	StaticPath     string
}

// IsLoan returns true if this data model represents the loan & return view.
func (d EncodeData) IsLoan() bool {
	return false
}

// IsEncode returns true if this data model represents the encode view.
func (d EncodeData) IsEncode() bool {
	return true
}
