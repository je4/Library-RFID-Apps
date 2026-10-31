package rfidclient

import (
	"context"

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
	session, err := wsbrokerclient.GenerateSessionID()
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate correlation session")
	}

	reqEvt, err := wsbrokerclient.NewRawEvent(EventGetDeviceMetadata, c.broker.ClientName(), targetAddr, reqPayload)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create device metadata request event")
	}
	reqEvt.Session = session
	reqID := reqEvt.ID

	reqCtx := ctx
	var cancel context.CancelFunc
	if _, hasDeadline := ctx.Deadline(); !hasDeadline && c.config.defaultTimeout > 0 {
		reqCtx, cancel = context.WithTimeout(ctx, c.config.defaultTimeout)
		defer cancel()
	}

	c.logger.Debug().Msgf("sending device metadata request to %s (id: %s, reader: %s, session: %s)", targetAddr, reqID, readerID, session)
	respEvt, err := c.broker.Request(reqCtx, reqEvt)
	if err != nil {
		return nil, errors.Wrapf(err, "device metadata request failed for target %s", targetAddr)
	}

	if respEvt == nil || len(respEvt.Data) == 0 {
		return nil, errors.New("empty response received for device metadata request")
	}

	metaResp, err := wsbrokerclient.ToTyped[DeviceMetadataResponseData](respEvt)
	if err != nil {
		return nil, errors.Wrap(err, "failed to decode device metadata response")
	}

	if metaResp.Data.ReaderID == "" && respEvt.Source != "" {
		metaResp.Data.ReaderID = respEvt.Source
	}

	if metaResp.RefID == "" {
		metaResp.RefID = reqID
	}

	return metaResp, nil
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
	session, err := wsbrokerclient.GenerateSessionID()
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate correlation session")
	}

	reqEvt, err := wsbrokerclient.NewRawEvent(EventGetDeviceMetadata, c.broker.ClientName(), targetAddr, reqPayload)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create device metadata request event")
	}
	reqEvt.Session = session
	reqID := reqEvt.ID

	reqCtx := ctx
	var cancel context.CancelFunc
	if _, hasDeadline := ctx.Deadline(); !hasDeadline && c.config.defaultTimeout > 0 {
		reqCtx, cancel = context.WithTimeout(ctx, c.config.defaultTimeout)
		defer cancel()
	}

	rawCh := make(chan *event.RawEvent, expectedResponses)
	if err := c.broker.RequestN(reqCtx, reqEvt, expectedResponses, rawCh); err != nil {
		return nil, errors.Wrapf(err, "device metadata multi request failed for target %s", targetAddr)
	}

	var results []*DeviceMetadataResponse
	for respEvt := range rawCh {
		if respEvt == nil || len(respEvt.Data) == 0 {
			continue
		}
		typed, err := wsbrokerclient.ToTyped[DeviceMetadataResponseData](respEvt)
		if err != nil {
			if c.logger != nil {
				c.logger.Warn().Err(err).Msg("failed to decode device metadata response item")
			}
			continue
		}
		if typed.Data.ReaderID == "" && respEvt.Source != "" {
			typed.Data.ReaderID = respEvt.Source
		}
		if typed.RefID == "" {
			typed.RefID = reqID
		}
		results = append(results, typed)
	}

	return results, nil
}
