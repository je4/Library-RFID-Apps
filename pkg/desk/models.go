package desk

import (
	"fmt"
	"strings"
	"time"

	"github.com/je4/Library-RFID-Apps/pkg/libraryclient"
	"github.com/je4/utils/v2/pkg/config"
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
	ReaderID          string     `json:"readerId,omitempty"`
	Source            string     `json:"source,omitempty"`
	Channel           string     `json:"channel,omitempty"`
	ReadTime          time.Time  `json:"readTime"`
	StaticPath        string     `json:"staticPath,omitempty"`
}

// NewTagItem converts a low-level TagReadEvent into a display-oriented TagItem.
func NewTagItem(tag *libraryclient.TagReadEvent) *TagItem {
	if tag == nil {
		return &TagItem{}
	}

	readTime := tag.Timestamp
	if readTime.IsZero() {
		readTime = time.Now()
	}

	ts := readTime.UnixMilli()

	crcOk := tag.Data.IsCRCValidStatus()
	crcStatus := "Fehler"
	if crcOk {
		crcStatus = "OK (Gültig)"
	}

	parts := tag.Data.GetParts()
	partNo := tag.Data.GetPartNo()
	isMultiPart := tag.Data.IsMultiPart()

	// Extract AFI if present in tag event or raw string
	afi := tag.Data.AFI
	if afi == "" && strings.Contains(tag.Data.Raw, "AFI:") {
		rawParts := strings.Split(tag.Data.Raw, "AFI:")
		if len(rawParts) > 1 {
			afi = strings.TrimSpace(strings.Fields(rawParts[1])[0])
		}
	}

	afiSecured := libraryclient.IsAFISecured(afi)
	afiUnsecured := libraryclient.IsAFIUnsecured(afi)
	afiSecurityStatus := libraryclient.AFISecurityStatus(afi)

	// Create unique scan row ID
	rowID := fmt.Sprintf("%s-%d", strings.ReplaceAll(tag.Data.UID, ":", ""), ts)

	readerID := tag.Data.GetReaderID()
	if readerID == "" {
		readerID = tag.Source
	}

	return &TagItem{
		ID:                rowID,
		UID:               tag.Data.UID,
		ItemID:            tag.Data.ItemID,
		ISIL:              tag.Data.ISIL,
		Country:           tag.Data.Country,
		CRCOk:             crcOk,
		CRCStatus:         crcStatus,
		Parts:             parts,
		PartNo:            partNo,
		IsMultiPart:       isMultiPart,
		AFI:               afi,
		AFISecured:        afiSecured,
		AFIUnsecured:      afiUnsecured,
		AFISecurityStatus: afiSecurityStatus,
		UsageType:         tag.Data.GetUsageType(),
		Timestamp:         ts,
		TimeFormatted:     readTime.Format("15:04:05"),
		DateFormatted:     readTime.Format("02.01.2006"),
		Raw:               tag.Data.Raw,
		ReaderID:          readerID,
		Source:            tag.Source,
		Channel:           tag.Channel,
		ReadTime:          readTime,
		StaticPath:        "../static",
	}
}

// SetAFI updates the AFI security state and metadata on the TagItem.
func (item *TagItem) SetAFI(afi string) {
	item.AFI = afi
	item.AFISecured = libraryclient.IsAFISecured(afi)
	item.AFIUnsecured = libraryclient.IsAFIUnsecured(afi)
	item.AFISecurityStatus = libraryclient.AFISecurityStatus(afi)
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

// IsEmpty returns true if the tag contains no item identifier or is unreadable.
func (item *TagItem) IsEmpty() bool {
	return item == nil || item.ItemID == ""
}

// CRCError returns true if CRC validation failed.
func (item *TagItem) CRCError() bool {
	return item != nil && !item.CRCOk
}

// DeskData encapsulates the view model passed to web templates.
type DeskData struct {
	ClientName       string
	WSBroker         string
	Channels         []string
	Channel          string
	DefaultReader    string
	Addr             string
	ExtAddr          string
	Uptime           string
	Tags             []*TagItem
	Skin             string
	Theme            string
	ThemeMode        string
	ThemeLight       string
	ThemeDark        string
	StaticPath       string
	InventoryEnabled bool
}

// IsLoan returns true if this data model represents the loan & return view.
func (d DeskData) IsLoan() bool {
	return false
}

// IsEncode returns true if this data model represents the encode view.
func (d DeskData) IsEncode() bool {
	return false
}

// IsInventory returns true if this data model represents the inventory view.
func (d DeskData) IsInventory() bool {
	return false
}

// HasExplicitThemes returns true if both light and dark themes are explicitly configured.
func (d DeskData) HasExplicitThemes() bool {
	return d.ThemeLight != "" && d.ThemeDark != ""
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
	Theme             string
	ThemeMode         string
	ThemeLight        string
	ThemeDark         string
	StaticPath        string
	InventoryEnabled  bool
}

// IsLoan returns true if this data model represents the loan & return view.
func (d LoanData) IsLoan() bool {
	return true
}

// IsEncode returns true if this data model represents the encode view.
func (d LoanData) IsEncode() bool {
	return false
}

// IsInventory returns true if this data model represents the inventory view.
func (d LoanData) IsInventory() bool {
	return false
}

// HasExplicitThemes returns true if both light and dark themes are explicitly configured.
func (d LoanData) HasExplicitThemes() bool {
	return d.ThemeLight != "" && d.ThemeDark != ""
}

// EncodeData encapsulates the view model passed to encode web templates.
type EncodeData struct {
	ClientName       string
	WSBroker         string
	Channels         []string
	Channel          string
	DefaultReader    string
	Addr             string
	ExtAddr          string
	Uptime           string
	Tags             []*TagItem
	ErrorMessage     string
	SuccessMessage   string
	FailedItems      []string
	DefaultISIL      string
	DefaultAFI       string
	DefaultParts     int
	Skin             string
	Theme            string
	ThemeMode        string
	ThemeLight       string
	ThemeDark        string
	StaticPath       string
	InventoryEnabled bool
}

// IsLoan returns true if this data model represents the loan & return view.
func (d EncodeData) IsLoan() bool {
	return false
}

// IsEncode returns true if this data model represents the encode view.
func (d EncodeData) IsEncode() bool {
	return true
}

// IsInventory returns true if this data model represents the inventory view.
func (d EncodeData) IsInventory() bool {
	return false
}

// HasExplicitThemes returns true if both light and dark themes are explicitly configured.
func (d EncodeData) HasExplicitThemes() bool {
	return d.ThemeLight != "" && d.ThemeDark != ""
}

// WebserviceConfig defines parameters for dispatching RFID scan events to an external webservice.
type WebserviceConfig struct {
	TargetURL  config.EnvString `toml:"target_url" json:"targetUrl"`
	ClientID   string           `toml:"client_id" json:"clientId"`
	HTTPMethod string           `toml:"http_method" json:"httpMethod"` // POST (default) or GET
	JWTKey     config.EnvString `toml:"jwt_key" json:"jwtKey"`
	DebugMode  bool             `toml:"debug_mode" json:"debugMode"`
	Timeout    time.Duration    `toml:"timeout" json:"timeout"`
}

// InventoryConfig defines settings for the inventory scanning and webservice dispatching subsystem.
type InventoryConfig struct {
	Enabled    bool             `toml:"enabled" json:"enabled"`
	Webservice WebserviceConfig `toml:"webservice" json:"webservice"`
}

// InventoryItem represents a scanned RFID tag within an inventory session including its dispatch state.
type InventoryItem struct {
	*TagItem
	Marker        string    `json:"marker"`
	SessionID     string    `json:"sessionId"`
	IsDuplicate   bool      `json:"isDuplicate"`
	DispatchState string    `json:"dispatchState"` // "success", "error", "pending"
	DispatchError string    `json:"dispatchError,omitempty"`
	DispatchedAt  time.Time `json:"dispatchedAt,omitempty"`
	HTTPStatus    int       `json:"httpStatus,omitempty"`
	Signature     string    `json:"signature,omitempty"`
}

// InventoryStats contains aggregated counters for the current inventory session.
type InventoryStats struct {
	TotalScans    int `json:"totalScans"`
	UniqueItems   int `json:"uniqueItems"`
	ErrorTags     int `json:"errorTags"`
	DuplicateTags int `json:"duplicateTags"`
}

// InventorySession holds the live in-memory state of an active or paused inventory session.
type InventorySession struct {
	SessionID string           `json:"sessionId"`
	Marker    string           `json:"marker"`
	Active    bool             `json:"active"`
	Paused    bool             `json:"paused"`
	StartTime time.Time        `json:"startTime"`
	Stats     InventoryStats   `json:"stats"`
	Items     []*InventoryItem `json:"items"`
}

// InventoryData encapsulates the view model passed to the inventory web template.
type InventoryData struct {
	ClientName       string
	WSBroker         string
	Channels         []string
	Channel          string
	DefaultReader    string
	Addr             string
	ExtAddr          string
	Uptime           string
	Marker           string
	SessionActive    bool
	IsPaused         bool
	TotalScans       int
	UniqueItems      int
	ErrorTags        int
	DuplicateTags    int
	Items            []*InventoryItem
	LastItem         *InventoryItem
	InventoryConf    *InventoryConfig
	Skin             string
	Theme            string
	ThemeMode        string
	ThemeLight       string
	ThemeDark        string
	StaticPath       string
	InventoryEnabled bool
}

// IsLoan returns false for the inventory view.
func (d InventoryData) IsLoan() bool {
	return false
}

// IsEncode returns false for the inventory view.
func (d InventoryData) IsEncode() bool {
	return false
}

// IsInventory returns true for the inventory view.
func (d InventoryData) IsInventory() bool {
	return true
}

// HasExplicitThemes returns true if both light and dark themes are explicitly configured.
func (d InventoryData) HasExplicitThemes() bool {
	return d.ThemeLight != "" && d.ThemeDark != ""
}
