// Package rfidclient provides backward-compatible re-exports and aliases
// forwarding to the unified pkg/libraryclient package.
//
// Deprecated: Direct use of rfidclient is deprecated. Use github.com/je4/Library-RFID-Apps/pkg/libraryclient instead.
package rfidclient

import (
	"github.com/je4/Library-RFID-Apps/pkg/libraryclient"
	"github.com/je4/wsbrokerclient"
)

// Re-exported types from pkg/libraryclient for backward compatibility.
type (
	Client                     = libraryclient.Client
	LibraryClient              = libraryclient.LibraryClient
	RFIDClient                 = libraryclient.LibraryClient
	Option                     = libraryclient.Option
	TagReadData                = libraryclient.TagReadData
	TagReadEvent               = libraryclient.TagReadEvent
	TagsClearedData            = libraryclient.TagsClearedData
	TagsClearedEvent           = libraryclient.TagsClearedEvent
	DeviceMetadataRequest      = libraryclient.DeviceMetadataRequest
	DeviceMetadataResponseData = libraryclient.DeviceMetadataResponseData
	DeviceMetadataResponse     = libraryclient.DeviceMetadataResponse
	WriteTagRequest            = libraryclient.WriteTagRequest
	WriteTagResponseData       = libraryclient.WriteTagResponseData
	WriteTagResponse           = libraryclient.WriteTagResponse
	TagWriteItem               = libraryclient.TagWriteItem
	TagWriteResult             = libraryclient.TagWriteResult
	WriteAFIRequest            = libraryclient.WriteAFIRequest
	WriteAFIResponseData       = libraryclient.WriteAFIResponseData
	WriteAFIResponse           = libraryclient.WriteAFIResponse
	TagAFIResult               = libraryclient.TagAFIResult

	TagReadHandler          = libraryclient.TagReadHandler
	TagsClearedHandler      = libraryclient.TagsClearedHandler
	DeviceMetadataHandler   = libraryclient.DeviceMetadataHandler
	WriteTagResponseHandler = libraryclient.WriteTagResponseHandler
	WriteAFIResponseHandler = libraryclient.WriteAFIResponseHandler
	AnyRFIDEventHandler     = libraryclient.AnyRFIDEventHandler
)

// Re-exported event constants and AFI security constants.
const (
	EventTagRead                = libraryclient.EventTagRead
	EventTagsCleared            = libraryclient.EventTagsCleared
	EventGetDeviceMetadata      = libraryclient.EventGetDeviceMetadata
	EventDeviceMetadataResponse = libraryclient.EventDeviceMetadataResponse
	EventWriteTag               = libraryclient.EventWriteTag
	EventWriteTagResponse       = libraryclient.EventWriteTagResponse
	EventWriteAFI               = libraryclient.EventWriteAFI
	EventWriteAFIResponse       = libraryclient.EventWriteAFIResponse

	AFISecured   = libraryclient.AFISecured
	AFIUnsecured = libraryclient.AFIUnsecured
	AFIUnlocked  = libraryclient.AFIUnlocked
)

// Re-exported constructors, helpers and options.
var (
	New                               = libraryclient.New
	NewFromConfig                     = libraryclient.NewFromConfig
	WithDefaultTarget                 = libraryclient.WithDefaultTarget
	WithDefaultReaderID               = libraryclient.WithDefaultReaderID
	WithDefaultTimeout                = libraryclient.WithDefaultTimeout
	WithLogger                        = libraryclient.WithLogger
	WithChannels                      = libraryclient.WithChannels
	WithClientName                    = libraryclient.WithClientName
	NewDeviceMetadataRequest          = libraryclient.NewDeviceMetadataRequest
	NewDeviceMetadataRequestForReader = libraryclient.NewDeviceMetadataRequestForReader
	NewWriteTagRequest                = libraryclient.NewWriteTagRequest
	NewWriteTagBatchRequest           = libraryclient.NewWriteTagBatchRequest
	NewWriteTagBatchRequestFromMap    = libraryclient.NewWriteTagBatchRequestFromMap
	NewWriteAFIRequest                = libraryclient.NewWriteAFIRequest
	NewWriteAFIBatchRequest           = libraryclient.NewWriteAFIBatchRequest
	NormalizeAFI                      = libraryclient.NormalizeAFI
	IsAFISecured                      = libraryclient.IsAFISecured
	IsAFIUnsecured                    = libraryclient.IsAFIUnsecured
	AFISecurityStatus                 = libraryclient.AFISecurityStatus
)

// DefaultWSBrokerConfig returns standard WSBroker configuration with defaults.
func DefaultWSBrokerConfig() wsbrokerclient.Config {
	return wsbrokerclient.DefaultConfig()
}
