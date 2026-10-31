// Package rfidclient provides a high-level, production-ready Go client library
// for communicating with rfid-headless RFID reader/writer daemons via wsbrokerclient.
//
// # Generic Event Architecture
//
// The client implements a clean generic event architecture built on wsbrokerclient.Event[T]:
//
//  1. Transport Metadata (wsbrokerclient.BaseEvent): Embedded in each event, carrying routing metadata
//     (Source, Target, Channel, Token, Type, Timestamp, ID, RefID).
//  2. Domain Data (TagReadData, TagsClearedData, etc.): Strongly typed RFID data structures
//     with ISO 28560-2 library metadata, AFI security flags, and domain helper methods.
//
// # Single-Envelope Handler Signatures
//
// All event listener callbacks (OnTagRead, OnTagsCleared, OnWriteTagResponse, OnWriteAFIResponse)
// receive strongly typed *wsbrokerclient.Event[T] envelopes:
//
//	client.OnTagRead(func(evt *rfidclient.TagReadEvent) {
//	    // Access typed domain fields:
//	    fmt.Printf("Scanned Item ID: %s, UID: %s\n", evt.Data.ItemID, evt.Data.UID)
//	    // Access envelope routing metadata directly:
//	    fmt.Printf("Received from source: %s on channel: %s\n", evt.Source, evt.Channel)
//	})
package rfidclient

import (
	"github.com/je4/Library-RFID-Apps/pkg/rfidclient"
	"github.com/je4/wsbrokerclient"
)

// Re-exported types from pkg/rfidclient for streamlined root import.
type (
	Client                     = rfidclient.Client
	RFIDClient                 = rfidclient.RFIDClient
	Option                     = rfidclient.Option
	TagReadData                = rfidclient.TagReadData
	TagReadEvent               = rfidclient.TagReadEvent
	TagsClearedData            = rfidclient.TagsClearedData
	TagsClearedEvent           = rfidclient.TagsClearedEvent
	DeviceMetadataRequest      = rfidclient.DeviceMetadataRequest
	DeviceMetadataResponseData = rfidclient.DeviceMetadataResponseData
	DeviceMetadataResponse     = rfidclient.DeviceMetadataResponse
	WriteTagRequest            = rfidclient.WriteTagRequest
	WriteTagResponseData       = rfidclient.WriteTagResponseData
	WriteTagResponse           = rfidclient.WriteTagResponse
	TagWriteItem               = rfidclient.TagWriteItem
	TagWriteResult             = rfidclient.TagWriteResult
	WriteAFIRequest            = rfidclient.WriteAFIRequest
	WriteAFIResponseData       = rfidclient.WriteAFIResponseData
	WriteAFIResponse           = rfidclient.WriteAFIResponse
	TagAFIResult               = rfidclient.TagAFIResult

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
	New                               = rfidclient.New
	NewFromConfig                     = rfidclient.NewFromConfig
	WithDefaultTarget                 = rfidclient.WithDefaultTarget
	WithDefaultReaderID               = rfidclient.WithDefaultReaderID
	WithDefaultTimeout                = rfidclient.WithDefaultTimeout
	WithLogger                        = rfidclient.WithLogger
	WithChannels                      = rfidclient.WithChannels
	WithClientName                    = rfidclient.WithClientName
	NewDeviceMetadataRequest          = rfidclient.NewDeviceMetadataRequest
	NewDeviceMetadataRequestForReader = rfidclient.NewDeviceMetadataRequestForReader
	NewWriteTagRequest                = rfidclient.NewWriteTagRequest
	NewWriteTagBatchRequest           = rfidclient.NewWriteTagBatchRequest
	NewWriteTagBatchRequestFromMap    = rfidclient.NewWriteTagBatchRequestFromMap
	NewWriteAFIRequest                = rfidclient.NewWriteAFIRequest
	NewWriteAFIBatchRequest           = rfidclient.NewWriteAFIBatchRequest
	ParseRFIDPayload                  = rfidclient.ParseRFIDPayload
	ParseEvent                        = rfidclient.ParseEvent
	NormalizeAFI                      = rfidclient.NormalizeAFI
	IsAFISecured                      = rfidclient.IsAFISecured
	IsAFIUnsecured                    = rfidclient.IsAFIUnsecured
	AFISecurityStatus                 = rfidclient.AFISecurityStatus
)

// DefaultWSBrokerConfig returns standard WSBroker configuration with defaults.
func DefaultWSBrokerConfig() wsbrokerclient.Config {
	return wsbrokerclient.DefaultConfig()
}
