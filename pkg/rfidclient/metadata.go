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
	Event string `json:"event"`
	ID    string `json:"id,omitempty"`
	RefID string `json:"ref_id,omitempty"`
}

// NewDeviceMetadataRequest creates a new query request for reader hardware metadata.
func NewDeviceMetadataRequest() DeviceMetadataRequest {
	return DeviceMetadataRequest{
		Event: EventGetDeviceMetadata,
		ID:    wsbrokerclient.GenerateEventID(),
	}
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
	return fmt.Sprintf("[DEVICE METADATA] Status: %s | ID: %s | Type: %s | Description: %s",
		r.Status, r.DeviceID, r.DeviceType, desc)
}
