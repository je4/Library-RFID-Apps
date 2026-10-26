package rfidclient

import (
	"bytes"
	"encoding/json"

	"emperror.dev/errors"
	"github.com/je4/wsbrokerclient"
)

// ParseRFIDPayload unmarshals raw JSON bytes into the concrete typed RFID payload struct.
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
		var tag TagReadEvent
		if err := json.Unmarshal(data, &tag); err != nil {
			return nil, errors.Wrap(err, "failed to unmarshal TagReadEvent")
		}
		return &tag, nil

	case EventTagsCleared, EventTagsRemoved, EventAntennaFieldCleared:
		var cleared TagsClearedEvent
		if err := json.Unmarshal(data, &cleared); err != nil {
			return nil, errors.Wrap(err, "failed to unmarshal TagsClearedEvent")
		}
		return &cleared, nil

	case EventDeviceMetadataResponse, EventDeviceMetadata:
		var meta DeviceMetadataResponse
		if err := json.Unmarshal(data, &meta); err != nil {
			return nil, errors.Wrap(err, "failed to unmarshal DeviceMetadataResponse")
		}
		return &meta, nil

	case EventGetDeviceMetadata:
		var req DeviceMetadataRequest
		if err := json.Unmarshal(data, &req); err != nil {
			return nil, errors.Wrap(err, "failed to unmarshal DeviceMetadataRequest")
		}
		return &req, nil

	case EventWriteTagResponse:
		var writeResp WriteTagResponse
		if err := json.Unmarshal(data, &writeResp); err != nil {
			return nil, errors.Wrap(err, "failed to unmarshal WriteTagResponse")
		}
		return &writeResp, nil

	case EventWriteTag:
		if header.Status != "" {
			var writeResp WriteTagResponse
			if err := json.Unmarshal(data, &writeResp); err != nil {
				return nil, errors.Wrap(err, "failed to unmarshal WriteTagResponse")
			}
			return &writeResp, nil
		}
		var writeReq WriteTagRequest
		if err := json.Unmarshal(data, &writeReq); err != nil {
			return nil, errors.Wrap(err, "failed to unmarshal WriteTagRequest")
		}
		return &writeReq, nil

	case EventWriteAFIResponse:
		var afiResp WriteAFIResponse
		if err := json.Unmarshal(data, &afiResp); err != nil {
			return nil, errors.Wrap(err, "failed to unmarshal WriteAFIResponse")
		}
		return &afiResp, nil

	case EventWriteAFI:
		if header.Status != "" {
			var afiResp WriteAFIResponse
			if err := json.Unmarshal(data, &afiResp); err != nil {
				return nil, errors.Wrap(err, "failed to unmarshal WriteAFIResponse")
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
func ParseEvent(evt *wsbrokerclient.Event) (any, error) {
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
	case *TagReadEvent:
		if v.ID == "" && evt.ID != "" {
			v.ID = evt.ID
		}
		if v.RefID == "" && evt.RefID != "" {
			v.RefID = evt.RefID
		}
		if v.ClientID == "" && evt.Source != "" {
			v.ClientID = evt.Source
		}
		if v.Channel == "" && evt.Channel != "" {
			v.Channel = evt.Channel
		}
	case *TagsClearedEvent:
		if v.ID == "" && evt.ID != "" {
			v.ID = evt.ID
		}
		if v.RefID == "" && evt.RefID != "" {
			v.RefID = evt.RefID
		}
		if v.ClientID == "" && evt.Source != "" {
			v.ClientID = evt.Source
		}
		if v.Channel == "" && evt.Channel != "" {
			v.Channel = evt.Channel
		}
	case *DeviceMetadataResponse:
		if v.ID == "" && evt.ID != "" {
			v.ID = evt.ID
		}
		if v.RefID == "" && evt.RefID != "" {
			v.RefID = evt.RefID
		}
		if v.ClientID == "" && evt.Source != "" {
			v.ClientID = evt.Source
		}
		if v.Channel == "" && evt.Channel != "" {
			v.Channel = evt.Channel
		}
	case *DeviceMetadataRequest:
		if v.ID == "" && evt.ID != "" {
			v.ID = evt.ID
		}
		if v.RefID == "" && evt.RefID != "" {
			v.RefID = evt.RefID
		}
	case *WriteTagResponse:
		if v.ID == "" && evt.ID != "" {
			v.ID = evt.ID
		}
		if v.RefID == "" && evt.RefID != "" {
			v.RefID = evt.RefID
		}
		if v.ClientID == "" && evt.Source != "" {
			v.ClientID = evt.Source
		}
		if v.Channel == "" && evt.Channel != "" {
			v.Channel = evt.Channel
		}
	case *WriteTagRequest:
		if v.ID == "" && evt.ID != "" {
			v.ID = evt.ID
		}
		if v.RefID == "" && evt.RefID != "" {
			v.RefID = evt.RefID
		}
	case *WriteAFIResponse:
		if v.ID == "" && evt.ID != "" {
			v.ID = evt.ID
		}
		if v.RefID == "" && evt.RefID != "" {
			v.RefID = evt.RefID
		}
		if v.ClientID == "" && evt.Source != "" {
			v.ClientID = evt.Source
		}
		if v.Channel == "" && evt.Channel != "" {
			v.Channel = evt.Channel
		}
	case *WriteAFIRequest:
		if v.ID == "" && evt.ID != "" {
			v.ID = evt.ID
		}
		if v.RefID == "" && evt.RefID != "" {
			v.RefID = evt.RefID
		}
	}
	return payload, nil
}
