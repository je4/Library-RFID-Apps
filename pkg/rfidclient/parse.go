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
		Event  string `json:"event"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(data, &header); err != nil {
		return nil, errors.Wrap(err, "failed to decode json header")
	}

	switch header.Event {
	case EventTagRead:
		var tag TagReadData
		if err := json.Unmarshal(data, &tag); err != nil {
			return nil, errors.Wrap(err, "failed to unmarshal TagReadData")
		}
		return &tag, nil

	case EventTagsCleared, EventTagsRemoved, EventAntennaFieldCleared:
		var cleared TagsClearedData
		if err := json.Unmarshal(data, &cleared); err != nil {
			return nil, errors.Wrap(err, "failed to unmarshal TagsClearedData")
		}
		return &cleared, nil

	case EventDeviceMetadataResponse, EventDeviceMetadata:
		var meta DeviceMetadataResponseData
		if err := json.Unmarshal(data, &meta); err != nil {
			return nil, errors.Wrap(err, "failed to unmarshal DeviceMetadataResponseData")
		}
		return &meta, nil

	case EventGetDeviceMetadata:
		var req DeviceMetadataRequest
		if err := json.Unmarshal(data, &req); err != nil {
			return nil, errors.Wrap(err, "failed to unmarshal DeviceMetadataRequest")
		}
		return &req, nil

	case EventWriteTagResponse:
		var writeResp WriteTagResponseData
		if err := json.Unmarshal(data, &writeResp); err != nil {
			return nil, errors.Wrap(err, "failed to unmarshal WriteTagResponseData")
		}
		return &writeResp, nil

	case EventWriteTag:
		if header.Status != "" {
			var writeResp WriteTagResponseData
			if err := json.Unmarshal(data, &writeResp); err != nil {
				return nil, errors.Wrap(err, "failed to unmarshal WriteTagResponseData")
			}
			return &writeResp, nil
		}
		var writeReq WriteTagRequest
		if err := json.Unmarshal(data, &writeReq); err != nil {
			return nil, errors.Wrap(err, "failed to unmarshal WriteTagRequest")
		}
		return &writeReq, nil

	case EventWriteAFIResponse:
		var afiResp WriteAFIResponseData
		if err := json.Unmarshal(data, &afiResp); err != nil {
			return nil, errors.Wrap(err, "failed to unmarshal WriteAFIResponseData")
		}
		return &afiResp, nil

	case EventWriteAFI:
		if header.Status != "" {
			var afiResp WriteAFIResponseData
			if err := json.Unmarshal(data, &afiResp); err != nil {
				return nil, errors.Wrap(err, "failed to unmarshal WriteAFIResponseData")
			}
			return &afiResp, nil
		}
		var afiReq WriteAFIRequest
		if err := json.Unmarshal(data, &afiReq); err != nil {
			return nil, errors.Wrap(err, "failed to unmarshal WriteAFIRequest")
		}
		return &afiReq, nil

	default:
		return nil, errors.Errorf("unrecognized rfid event type: %q", header.Event)
	}
}

// ParseEvent extracts and parses the RFID payload from a wsbroker event envelope.
func ParseEvent(evt *wsbrokerclient.RawEvent) (any, error) {
	if evt == nil {
		return nil, errors.New("event envelope is nil")
	}
	if evt.Type != "" && evt.Type != wsbrokerclient.TypeMessage {
		return nil, errors.Errorf("event type %q is not %q", evt.Type, wsbrokerclient.TypeMessage)
	}
	if len(evt.Data) == 0 {
		return nil, errors.New("event data is empty")
	}
	payload, err := ParseRFIDPayload(evt.Data)
	if err != nil {
		return nil, err
	}
	switch v := payload.(type) {
	case *TagReadData:
		if v.ReaderID == "" && evt.Source != "" {
			v.ReaderID = evt.Source
		}
		return &TagReadEvent{
			BaseEvent: evt.BaseEvent,
			Data:      *v,
		}, nil
	case *TagsClearedData:
		if v.ReaderID == "" && evt.Source != "" {
			v.ReaderID = evt.Source
		}
		return &TagsClearedEvent{
			BaseEvent: evt.BaseEvent,
			Data:      *v,
		}, nil
	case *DeviceMetadataResponseData:
		if v.ReaderID == "" && evt.Source != "" {
			v.ReaderID = evt.Source
		}
		return &DeviceMetadataResponse{
			BaseEvent: evt.BaseEvent,
			Data:      *v,
		}, nil
	case *DeviceMetadataRequest:
		if v.ID == "" && evt.ID != "" {
			v.ID = evt.ID
		}
		if v.RefID == "" && evt.RefID != "" {
			v.RefID = evt.RefID
		}
		return v, nil
	case *WriteTagResponseData:
		if v.ReaderID == "" && evt.Source != "" {
			v.ReaderID = evt.Source
		}
		return &WriteTagResponse{
			BaseEvent: evt.BaseEvent,
			Data:      *v,
		}, nil
	case *WriteTagRequest:
		if v.ID == "" && evt.ID != "" {
			v.ID = evt.ID
		}
		if v.RefID == "" && evt.RefID != "" {
			v.RefID = evt.RefID
		}
		return v, nil
	case *WriteAFIResponseData:
		if v.ReaderID == "" && evt.Source != "" {
			v.ReaderID = evt.Source
		}
		return &WriteAFIResponse{
			BaseEvent: evt.BaseEvent,
			Data:      *v,
		}, nil
	case *WriteAFIRequest:
		if v.ID == "" && evt.ID != "" {
			v.ID = evt.ID
		}
		if v.RefID == "" && evt.RefID != "" {
			v.RefID = evt.RefID
		}
		return v, nil
	default:
		return payload, nil
	}
}
