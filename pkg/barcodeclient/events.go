package barcodeclient

import (
	"fmt"
	"strings"
	"time"

	"github.com/je4/wsbrokerclient"
)

// Standard Barcode event identifiers exchanged with barcode-headless.
const (
	EventBarcodeRead            = "barcode/read"
	EventGetDeviceMetadata      = "barcode/get_device_metadata"
	EventDeviceMetadataResponse = "barcode/device_metadata_response"
)

func init() {
	// Register Barcode event payloads with wsbrokerclient global registry
	wsbrokerclient.RegisterPayload[BarcodeReadData](EventBarcodeRead)
	wsbrokerclient.RegisterPayload[DeviceMetadataRequest](EventGetDeviceMetadata)
	wsbrokerclient.RegisterPayload[DeviceMetadataResponseData](EventDeviceMetadataResponse)
}

// BarcodeReadData represents a scanned barcode payload.
type BarcodeReadData struct {
	ReaderID   string `json:"readerId,omitempty"`
	Barcode    string `json:"barcode"`
	ItemID     string `json:"itemId,omitempty"`
	Symbology  string `json:"symbology,omitempty"`
	Raw        string `json:"raw,omitempty"`
	Timestamp  string `json:"timestamp,omitempty"`
	DeviceID   string `json:"deviceId,omitempty"`
	DeviceType string `json:"deviceType,omitempty"`
}

// GetReaderID returns the logical reader identifier.
func (e *BarcodeReadData) GetReaderID() string {
	return e.ReaderID
}

// WithReaderID sets the logical reader identifier on the payload.
func (e *BarcodeReadData) WithReaderID(readerID string) *BarcodeReadData {
	e.ReaderID = readerID
	return e
}

// GetBarcode returns the scanned barcode string.
func (e *BarcodeReadData) GetBarcode() string {
	return e.Barcode
}

// GetItemID returns the item ID or falls back to the barcode value.
func (e *BarcodeReadData) GetItemID() string {
	if e.ItemID != "" {
		return e.ItemID
	}
	return e.Barcode
}

// GetSymbology returns the barcode symbology (e.g. Code128, EAN13) or "Unknown".
func (e *BarcodeReadData) GetSymbology() string {
	if e.Symbology != "" {
		return e.Symbology
	}
	return "Unknown"
}

// Summary returns a concise human-readable description of the barcode read.
func (e *BarcodeReadData) Summary() string {
	readerStr := ""
	if rID := e.GetReaderID(); rID != "" {
		readerStr = fmt.Sprintf(" | Reader: %s", rID)
	}
	symbStr := ""
	if sym := e.Symbology; sym != "" {
		symbStr = fmt.Sprintf(" | Type: %s", sym)
	}
	return fmt.Sprintf("[BARCODE READ] Barcode: %s%s%s", e.Barcode, symbStr, readerStr)
}

// BarcodeReadEvent represents a typed wsbroker event containing BarcodeReadData.
type BarcodeReadEvent = wsbrokerclient.Event[BarcodeReadData]

// DeviceMetadataRequest represents a query for barcode reader hardware status and metadata.
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

// DeviceMetadataResponseData represents barcode reader hardware metadata and status.
type DeviceMetadataResponseData struct {
	ReaderID        string `json:"readerId,omitempty"`
	Status          string `json:"status,omitempty"`
	DeviceID        string `json:"deviceId,omitempty"`
	DeviceType      string `json:"deviceType,omitempty"`
	Port            string `json:"port,omitempty"`
	BaudRate        int    `json:"baudRate,omitempty"`
	Description     string `json:"description,omitempty"`
	TotalScans      int64  `json:"totalScans,omitempty"`
	LastScanTime    string `json:"lastScanTime,omitempty"`
	InitDescription string `json:"initDescription,omitempty"`
}

// GetReaderID returns the logical reader identifier.
func (r *DeviceMetadataResponseData) GetReaderID() string {
	return r.ReaderID
}

// DeviceMetadataResponse represents a typed wsbroker event containing DeviceMetadataResponseData.
type DeviceMetadataResponse = wsbrokerclient.Event[DeviceMetadataResponseData]

// IsSuccess returns true if the status equals "ok" or "connected".
func (r *DeviceMetadataResponseData) IsSuccess() bool {
	return strings.EqualFold(r.Status, "ok") || strings.EqualFold(r.Status, "connected")
}

// GetDescription returns the device description.
func (r *DeviceMetadataResponseData) GetDescription() string {
	if r.Description != "" {
		return r.Description
	}
	if r.InitDescription != "" {
		return r.InitDescription
	}
	return fmt.Sprintf("%s on %s", r.DeviceType, r.Port)
}

// Summary returns a concise human-readable description of the device metadata.
func (r *DeviceMetadataResponseData) Summary() string {
	readerStr := ""
	if rID := eReaderID(r.ReaderID); rID != "" {
		readerStr = fmt.Sprintf(" | Reader: %s", rID)
	}
	portStr := ""
	if r.Port != "" {
		portStr = fmt.Sprintf(" | Port: %s (%d baud)", r.Port, r.BaudRate)
	}
	return fmt.Sprintf("[BARCODE METADATA] Status: %s | ID: %s | Type: %s%s%s",
		r.Status, r.DeviceID, r.DeviceType, readerStr, portStr)
}

func eReaderID(id string) string {
	return strings.TrimSpace(id)
}

// FormatTime formats a time as RFC3339 for payload timestamp fields.
func FormatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}
