package rfidclient

import (
	"bytes"
	"encoding/json"

	"emperror.dev/errors"
	"github.com/je4/wsbrokerclient"
)

// ParseRFIDPayload unmarshals raw JSON bytes into the concrete typed RFID payload data struct.
func ParseRFIDPayload(data []byte) (any, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return nil, errors.New("empty payload data")
	}

	// If data is a quoted JSON string, unquote it
	if len(trimmed) > 0 && trimmed[0] == '"' {
		var unquoted string
		if err := json.Unmarshal(trimmed, &unquoted); err == nil && len(unquoted) > 0 {
			data = []byte(unquoted)
		}
	}

	var header struct {
		Event string `json:"event"`
	}
	if err := json.Unmarshal(data, &header); err == nil && header.Event != "" {
		raw := &wsbrokerclient.RawEvent{
			BaseEvent: wsbrokerclient.BaseEvent{Type: wsbrokerclient.EventType(header.Event)},
			Data:      data,
		}
		return raw.GetData()
	}

	return nil, errors.New("failed to identify payload event type")
}

// ParseEvent extracts and parses the RFID payload from a wsbroker event envelope.
func ParseEvent(evt *wsbrokerclient.RawEvent) (any, error) {
	if evt == nil {
		return nil, errors.New("event envelope is nil")
	}
	if len(evt.Data) == 0 {
		return nil, errors.New("event data is empty")
	}

	eventType := string(evt.Type)
	switch eventType {
	case EventTagRead:
		typed, err := wsbrokerclient.ToTyped[TagReadData](evt)
		if err != nil {
			return nil, err
		}
		if typed.Data.ReaderID == "" && evt.Source != "" {
			typed.Data.ReaderID = evt.Source
		}
		return typed, nil

	case EventTagsCleared, EventTagsRemoved, EventAntennaFieldCleared:
		typed, err := wsbrokerclient.ToTyped[TagsClearedData](evt)
		if err != nil {
			return nil, err
		}
		if typed.Data.ReaderID == "" && evt.Source != "" {
			typed.Data.ReaderID = evt.Source
		}
		return typed, nil

	case EventDeviceMetadataResponse, EventDeviceMetadata:
		typed, err := wsbrokerclient.ToTyped[DeviceMetadataResponseData](evt)
		if err != nil {
			return nil, err
		}
		if typed.Data.ReaderID == "" && evt.Source != "" {
			typed.Data.ReaderID = evt.Source
		}
		return typed, nil

	case EventGetDeviceMetadata:
		typed, err := wsbrokerclient.ToTyped[DeviceMetadataRequest](evt)
		if err != nil {
			return nil, err
		}
		return &typed.Data, nil

	case EventWriteTagResponse:
		typed, err := wsbrokerclient.ToTyped[WriteTagResponseData](evt)
		if err != nil {
			return nil, err
		}
		if typed.Data.ReaderID == "" && evt.Source != "" {
			typed.Data.ReaderID = evt.Source
		}
		return typed, nil

	case EventWriteTag:
		var statusCheck struct {
			Status string `json:"status"`
		}
		_ = json.Unmarshal(evt.Data, &statusCheck)
		if statusCheck.Status != "" {
			typed, err := wsbrokerclient.ToTyped[WriteTagResponseData](evt)
			if err != nil {
				return nil, err
			}
			if typed.Data.ReaderID == "" && evt.Source != "" {
				typed.Data.ReaderID = evt.Source
			}
			return typed, nil
		}
		typed, err := wsbrokerclient.ToTyped[WriteTagRequest](evt)
		if err != nil {
			return nil, err
		}
		return &typed.Data, nil

	case EventWriteAFIResponse:
		typed, err := wsbrokerclient.ToTyped[WriteAFIResponseData](evt)
		if err != nil {
			return nil, err
		}
		if typed.Data.ReaderID == "" && evt.Source != "" {
			typed.Data.ReaderID = evt.Source
		}
		return typed, nil

	case EventWriteAFI:
		var statusCheck struct {
			Status string `json:"status"`
		}
		_ = json.Unmarshal(evt.Data, &statusCheck)
		if statusCheck.Status != "" {
			typed, err := wsbrokerclient.ToTyped[WriteAFIResponseData](evt)
			if err != nil {
				return nil, err
			}
			if typed.Data.ReaderID == "" && evt.Source != "" {
				typed.Data.ReaderID = evt.Source
			}
			return typed, nil
		}
		typed, err := wsbrokerclient.ToTyped[WriteAFIRequest](evt)
		if err != nil {
			return nil, err
		}
		return &typed.Data, nil

	default:
		return nil, errors.Errorf("unrecognized rfid event type: %q", eventType)
	}
}
