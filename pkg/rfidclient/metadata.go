package rfidclient

import (
	"fmt"
	"strings"

	"github.com/je4/wsbrokerclient"
)

// Hardware metadata event identifiers.
const (
	EventGetDeviceMetadata      = "rfid:get_device_metadata"
	EventDeviceMetadata         = "rfid:device_metadata"
	EventDeviceMetadataResponse = "rfid:device_metadata_response"
)

// DeviceMetadataRequest represents a query for reader hardware status and metadata.
type DeviceMetadataRequest struct {
	Event    string `json:"event"`
	ID       string `json:"id,omitempty"`
	RefID    string `json:"refId,omitempty"`
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
// An optional readerID can be specified to query a specific reader on a multi-reader daemon.
func NewDeviceMetadataRequest(readerID ...string) DeviceMetadataRequest {
	var rID string
	if len(readerID) > 0 {
		rID = readerID[0]
	}
	return DeviceMetadataRequest{
		Event:    EventGetDeviceMetadata,
		ID:       wsbrokerclient.GenerateEventID(),
		ReaderID: rID,
	}
}

// NewDeviceMetadataRequestForReader creates a new query request for a specific reader hardware metadata.
func NewDeviceMetadataRequestForReader(readerID string) DeviceMetadataRequest {
	return NewDeviceMetadataRequest(readerID)
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
	readerStr := ""
	if rID := r.GetReaderID(); rID != "" {
		readerStr = fmt.Sprintf(" | Reader: %s", rID)
	}
	return fmt.Sprintf("[DEVICE METADATA] Status: %s | ID: %s | Type: %s%s | Description: %s",
		r.Status, r.DeviceID, r.DeviceType, readerStr, desc)
}
