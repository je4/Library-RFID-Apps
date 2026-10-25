// Package rfidclient provides a high-level, production-ready Go client library
// for communicating with rfid-headless RFID reader/writer daemons via wsbrokerclient.
package rfidclient

import (
	"github.com/je4/Library-RFID-Apps/pkg/rfidclient"
	"github.com/je4/wsbrokerclient"
)

// Re-exported types from pkg/rfidclient for streamlined root import.
type (
	Client                 = rfidclient.Client
	RFIDClient             = rfidclient.RFIDClient
	Option                 = rfidclient.Option
	BaseRFIDPayload        = rfidclient.BaseRFIDPayload
	TagReadEvent           = rfidclient.TagReadEvent
	TagsClearedEvent       = rfidclient.TagsClearedEvent
	DeviceMetadataRequest  = rfidclient.DeviceMetadataRequest
	DeviceMetadataResponse = rfidclient.DeviceMetadataResponse
	WriteTagRequest        = rfidclient.WriteTagRequest
	WriteTagResponse       = rfidclient.WriteTagResponse
	TagWriteItem           = rfidclient.TagWriteItem
	TagWriteResult         = rfidclient.TagWriteResult
	WriteAFIRequest        = rfidclient.WriteAFIRequest
	WriteAFIResponse       = rfidclient.WriteAFIResponse
	TagAFIResult           = rfidclient.TagAFIResult

	TagReadHandler          = rfidclient.TagReadHandler
	TagsClearedHandler      = rfidclient.TagsClearedHandler
	DeviceMetadataHandler   = rfidclient.DeviceMetadataHandler
	WriteTagResponseHandler = rfidclient.WriteTagResponseHandler
	WriteAFIResponseHandler = rfidclient.WriteAFIResponseHandler
	AnyRFIDEventHandler     = rfidclient.AnyRFIDEventHandler
)

// Re-exported event constants and AFI security constants.
const (
	EventTagRead                = rfidclient.EventTagRead
	EventTagsCleared            = rfidclient.EventTagsCleared
	EventGetDeviceMetadata      = rfidclient.EventGetDeviceMetadata
	EventDeviceMetadata         = rfidclient.EventDeviceMetadata
	EventDeviceMetadataResponse = rfidclient.EventDeviceMetadataResponse
	EventWriteTag               = rfidclient.EventWriteTag
	EventWriteTagResponse       = rfidclient.EventWriteTagResponse
	EventWriteAFI               = rfidclient.EventWriteAFI
	EventWriteAFIResponse       = rfidclient.EventWriteAFIResponse

	AFISecured   = rfidclient.AFISecured
	AFIUnsecured = rfidclient.AFIUnsecured
	AFIUnlocked  = rfidclient.AFIUnlocked
)

// Re-exported constructors, helpers and options.
var (
	New                            = rfidclient.New
	NewFromConfig                  = rfidclient.NewFromConfig
	WithDefaultTarget              = rfidclient.WithDefaultTarget
	WithDefaultTimeout             = rfidclient.WithDefaultTimeout
	WithLogger                     = rfidclient.WithLogger
	WithChannels                   = rfidclient.WithChannels
	WithClientName                 = rfidclient.WithClientName
	NewDeviceMetadataRequest       = rfidclient.NewDeviceMetadataRequest
	NewWriteTagRequest             = rfidclient.NewWriteTagRequest
	NewWriteTagBatchRequest        = rfidclient.NewWriteTagBatchRequest
	NewWriteTagBatchRequestFromMap = rfidclient.NewWriteTagBatchRequestFromMap
	NewWriteAFIRequest             = rfidclient.NewWriteAFIRequest
	NewWriteAFIBatchRequest        = rfidclient.NewWriteAFIBatchRequest
	ParseRFIDPayload               = rfidclient.ParseRFIDPayload
	ParseEvent                     = rfidclient.ParseEvent
	NormalizeAFI                   = rfidclient.NormalizeAFI
	IsAFISecured                   = rfidclient.IsAFISecured
	IsAFIUnsecured                 = rfidclient.IsAFIUnsecured
	AFISecurityStatus              = rfidclient.AFISecurityStatus
)

// DefaultWSBrokerConfig returns standard WSBroker configuration with defaults.
func DefaultWSBrokerConfig() wsbrokerclient.Config {
	return wsbrokerclient.DefaultConfig()
}
