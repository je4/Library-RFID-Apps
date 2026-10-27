package rfidclient

import (
	"context"
	"encoding/json"

	"emperror.dev/errors"
	"github.com/je4/wsbrokerclient"
	"github.com/je4/wsbrokerclient/pkg/event"
)

// GetDeviceMetadata queries reader hardware details and diagnostic startup report.
func (c *RFIDClient) GetDeviceMetadata(ctx context.Context, target ...string) (*DeviceMetadataResponse, error) {
	targetAddr, err := c.resolveTarget(target...)
	if err != nil {
		return nil, err
	}
	return c.GetDeviceMetadataForReader(ctx, targetAddr, c.config.defaultReaderID)
}

// GetDeviceMetadataForReader queries hardware metadata for a specific reader ID on the target daemon.
func (c *RFIDClient) GetDeviceMetadataForReader(ctx context.Context, target string, readerID string) (*DeviceMetadataResponse, error) {
	targetAddr, err := c.resolveTarget(target)
	if err != nil {
		return nil, err
	}

	reqPayload := NewDeviceMetadataRequest(readerID)
	token, err := wsbrokerclient.GenerateToken()
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate correlation token")
	}

	reqEvt, err := wsbrokerclient.NewMessageEvent(c.broker.ClientName(), targetAddr, reqPayload)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create device metadata request event")
	}
	reqEvt.Token = token
	reqEvt.ID = reqPayload.ID

	reqCtx := ctx
	var cancel context.CancelFunc
	if _, hasDeadline := ctx.Deadline(); !hasDeadline && c.config.defaultTimeout > 0 {
		reqCtx, cancel = context.WithTimeout(ctx, c.config.defaultTimeout)
		defer cancel()
	}

	c.logger.Debug().Msgf("sending device metadata request to %s (id: %s, reader: %s, token: %s)", targetAddr, reqPayload.ID, readerID, token)
	respEvt, err := c.broker.Request(reqCtx, reqEvt)
	if err != nil {
		return nil, errors.Wrapf(err, "device metadata request failed for target %s", targetAddr)
	}

	if respEvt == nil || len(respEvt.Data) == 0 {
		return nil, errors.New("empty response received for device metadata request")
	}

	var metaResp DeviceMetadataResponse
	if err := json.Unmarshal(respEvt.Data, &metaResp); err != nil {
		return nil, errors.Wrap(err, "failed to decode device metadata response")
	}

	if metaResp.ID == "" && respEvt.ID != "" {
		metaResp.ID = respEvt.ID
	}
	if metaResp.RefID == "" {
		if respEvt.RefID != "" {
			metaResp.RefID = respEvt.RefID
		} else {
			metaResp.RefID = reqPayload.ID
		}
	}
	if metaResp.Source == "" && respEvt.Source != "" {
		metaResp.Source = respEvt.Source
	}
	if metaResp.ReaderID == "" && respEvt.Source != "" {
		metaResp.ReaderID = respEvt.Source
	}

	return &metaResp, nil
}

// GetDeviceMetadataMulti queries hardware metadata from multiple readers or broadcast channels concurrently using RequestN.
func (c *RFIDClient) GetDeviceMetadataMulti(ctx context.Context, target string, expectedResponses int) ([]*DeviceMetadataResponse, error) {
	if expectedResponses <= 0 {
		expectedResponses = 1
	}
	targetAddr, err := c.resolveTarget(target)
	if err != nil {
		return nil, err
	}

	reqPayload := NewDeviceMetadataRequest(c.config.defaultReaderID)
	token, err := wsbrokerclient.GenerateToken()
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate correlation token")
	}

	reqEvt, err := wsbrokerclient.NewMessageEvent(c.broker.ClientName(), targetAddr, reqPayload)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create device metadata request event")
	}
	reqEvt.Token = token
	reqEvt.ID = reqPayload.ID

	reqCtx := ctx
	var cancel context.CancelFunc
	if _, hasDeadline := ctx.Deadline(); !hasDeadline && c.config.defaultTimeout > 0 {
		reqCtx, cancel = context.WithTimeout(ctx, c.config.defaultTimeout)
		defer cancel()
	}

	rawCh := make(chan *event.Event, expectedResponses)
	if err := c.broker.RequestN(reqCtx, reqEvt, expectedResponses, rawCh); err != nil {
		return nil, errors.Wrapf(err, "device metadata multi request failed for target %s", targetAddr)
	}

	var results []*DeviceMetadataResponse
	for respEvt := range rawCh {
		if respEvt == nil || len(respEvt.Data) == 0 {
			continue
		}
		var metaResp DeviceMetadataResponse
		if err := json.Unmarshal(respEvt.Data, &metaResp); err != nil {
			if c.logger != nil {
				c.logger.Warn().Err(err).Msg("failed to decode device metadata response item")
			}
			continue
		}
		if metaResp.ID == "" && respEvt.ID != "" {
			metaResp.ID = respEvt.ID
		}
		if metaResp.RefID == "" {
			if respEvt.RefID != "" {
				metaResp.RefID = respEvt.RefID
			} else {
				metaResp.RefID = reqPayload.ID
			}
		}
		if metaResp.Source == "" && respEvt.Source != "" {
			metaResp.Source = respEvt.Source
		}
		if metaResp.ReaderID == "" && respEvt.Source != "" {
			metaResp.ReaderID = respEvt.Source
		}
		results = append(results, &metaResp)
	}

	return results, nil
}
