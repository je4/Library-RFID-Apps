// Package rfidclient provides backward-compatible re-exports and aliases
// forwarding to the unified pkg/libraryclient package.
//
// Deprecated: Direct use of rfidclient is deprecated. Use github.com/je4/Library-RFID-Apps/pkg/libraryclient instead.
package rfidclient

import (
	"context"

	"github.com/je4/Library-RFID-Apps/pkg/libraryclient"
	"github.com/je4/utils/v2/pkg/zLogger"
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
	GateAddress                = libraryclient.GateAddress
	GateCounter                = libraryclient.GateCounter
	GateDiagnostic             = libraryclient.GateDiagnostic
	GateStatusData             = libraryclient.GateStatusData
	GateStatusEvent            = libraryclient.GateStatusEvent
	PeopleCounterData          = libraryclient.PeopleCounterData
	PeopleCounterEvent         = libraryclient.PeopleCounterEvent

	TagReadHandler          = libraryclient.TagReadHandler
	TagsClearedHandler      = libraryclient.TagsClearedHandler
	DeviceMetadataHandler   = libraryclient.DeviceMetadataHandler
	WriteTagResponseHandler = libraryclient.WriteTagResponseHandler
	WriteAFIResponseHandler = libraryclient.WriteAFIResponseHandler
	GateStatusHandler       = libraryclient.GateStatusHandler
	PeopleCounterHandler    = libraryclient.PeopleCounterHandler
	AnyRFIDEventHandler     = libraryclient.AnyRFIDEventHandler
	ReaderIdentifiable      = libraryclient.ReaderIdentifiable
	HandlerRegistry[H any]  = libraryclient.HandlerRegistry[H]
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
	EventGateStatus             = libraryclient.EventGateStatus

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

// Subscribe is a generic helper forwarding to libraryclient.Subscribe.
func Subscribe[T any, H ~func(*wsbrokerclient.Event[T])](
	ctx context.Context,
	channel string,
	register func(handler H) wsbrokerclient.UnsubscribeFunc,
	logger ...zLogger.ZLogger,
) (<-chan *wsbrokerclient.Event[T], wsbrokerclient.UnsubscribeFunc, error) {
	return libraryclient.Subscribe(ctx, channel, register, logger...)
}

// DefaultWSBrokerConfig returns standard WSBroker configuration with defaults.
func DefaultWSBrokerConfig() wsbrokerclient.Config {
	return wsbrokerclient.DefaultConfig()
}
