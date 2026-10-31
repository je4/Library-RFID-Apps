package libraryclient

import (
	"context"

	"emperror.dev/errors"
	"github.com/je4/wsbrokerclient"
)

// GetBarcodeDeviceMetadata queries barcode reader hardware details and status report.
func (c *LibraryClient) GetBarcodeDeviceMetadata(ctx context.Context, target ...string) (*BarcodeDeviceMetadataResponse, error) {
	targetAddr, err := c.resolveTarget(target...)
	if err != nil {
		return nil, err
	}
	return c.GetBarcodeDeviceMetadataForReader(ctx, targetAddr, c.config.defaultReaderID)
}

// GetBarcodeDeviceMetadataForReader queries hardware metadata for a specific reader ID on the target barcode daemon.
func (c *LibraryClient) GetBarcodeDeviceMetadataForReader(ctx context.Context, target string, readerID string) (*BarcodeDeviceMetadataResponse, error) {
	targetAddr, err := c.resolveTarget(target)
	if err != nil {
		return nil, err
	}

	reqPayload := NewBarcodeDeviceMetadataRequest(readerID)
	session, err := wsbrokerclient.GenerateSessionID()
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate correlation session")
	}

	reqEvt, err := wsbrokerclient.NewRawEvent(EventBarcodeGetDeviceMetadata, c.broker.ClientName(), targetAddr, reqPayload)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create barcode device metadata request event")
	}
	reqEvt.Session = session
	reqID := reqEvt.ID

	reqCtx := ctx
	var cancel context.CancelFunc
	if _, hasDeadline := ctx.Deadline(); !hasDeadline && c.config.defaultTimeout > 0 {
		reqCtx, cancel = context.WithTimeout(ctx, c.config.defaultTimeout)
		defer cancel()
	}

	if c.logger != nil {
		c.logger.Debug().Msgf("sending barcode device metadata request to %s (id: %s, reader: %s, session: %s)", targetAddr, reqID, readerID, session)
	}
	respEvt, err := c.broker.Request(reqCtx, reqEvt)
	if err != nil {
		return nil, errors.Wrapf(err, "barcode device metadata request failed for target %s", targetAddr)
	}

	if respEvt == nil || len(respEvt.Data) == 0 {
		return nil, errors.New("empty response received for barcode device metadata request")
	}

	metaResp, err := wsbrokerclient.ToTyped[BarcodeDeviceMetadataResponseData](respEvt)
	if err != nil {
		return nil, errors.Wrap(err, "failed to decode barcode device metadata response")
	}

	if metaResp.Data.ReaderID == "" && respEvt.Source != "" {
		metaResp.Data.ReaderID = respEvt.Source
	}

	if metaResp.RefID == "" {
		metaResp.RefID = reqID
	}

	return metaResp, nil
}
