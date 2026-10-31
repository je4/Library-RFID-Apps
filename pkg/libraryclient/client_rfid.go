package libraryclient

import (
	"context"
	"strings"
	"time"

	"emperror.dev/errors"
	"github.com/je4/wsbrokerclient"
	"github.com/je4/wsbrokerclient/pkg/event"
)

// GetDeviceMetadata queries reader hardware details and diagnostic startup report.
func (c *LibraryClient) GetDeviceMetadata(ctx context.Context, target ...string) (*DeviceMetadataResponse, error) {
	targetAddr, err := c.resolveTarget(target...)
	if err != nil {
		return nil, err
	}
	return c.GetDeviceMetadataForReader(ctx, targetAddr, c.config.defaultReaderID)
}

// GetDeviceMetadataForReader queries hardware metadata for a specific reader ID on the target daemon.
func (c *LibraryClient) GetDeviceMetadataForReader(ctx context.Context, target string, readerID string) (*DeviceMetadataResponse, error) {
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

	if c.logger != nil {
		c.logger.Debug().Msgf("sending device metadata request to %s (id: %s, reader: %s, session: %s)", targetAddr, reqID, readerID, session)
	}
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
func (c *LibraryClient) GetDeviceMetadataMulti(ctx context.Context, target string, expectedResponses int) ([]*DeviceMetadataResponse, error) {
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

// WriteTag sends a command to write ISO 28560 metadata and optional AFI to a transponder.
func (c *LibraryClient) WriteTag(ctx context.Context, target string, req WriteTagRequest) (*WriteTagResponse, error) {
	if req.ReaderID == "" && c.config.defaultReaderID != "" {
		req.ReaderID = c.config.defaultReaderID
	}
	if err := req.Validate(); err != nil {
		return nil, err
	}

	targetAddr, err := c.resolveTarget(target)
	if err != nil {
		return nil, err
	}

	session, err := wsbrokerclient.GenerateSessionID()
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate correlation session")
	}

	reqEvt, err := wsbrokerclient.NewRawEvent(EventWriteTag, c.broker.ClientName(), targetAddr, req)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create write tag request event")
	}
	reqEvt.Session = session
	reqID := reqEvt.ID

	reqCtx := ctx
	var cancel context.CancelFunc
	if _, hasDeadline := ctx.Deadline(); !hasDeadline && c.config.defaultTimeout > 0 {
		timeout := c.config.defaultTimeout
		count := len(req.Tags)
		if len(req.Items) > count {
			count = len(req.Items)
		}
		if len(req.UIDs) > count {
			count = len(req.UIDs)
		}
		if count > 1 {
			batchTimeout := time.Duration(count)*2*time.Second + 5*time.Second
			if batchTimeout > timeout {
				timeout = batchTimeout
			}
		}
		reqCtx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	if c.logger != nil {
		logEvt := c.logger.Info().
			Str("id", reqID).
			Str("target", targetAddr)
		if req.ReaderID != "" {
			logEvt = logEvt.Str("readerId", req.ReaderID)
		}
		if len(req.Tags) > 0 {
			logEvt = logEvt.Int("tagsCount", len(req.Tags))
		} else if len(req.Items) > 0 {
			logEvt = logEvt.Int("itemsCount", len(req.Items))
		} else {
			logEvt = logEvt.
				Str("itemId", req.ItemID).
				Str("uid", req.UID).
				Str("afi", req.AFI)
		}
		logEvt.Msg("writing RFID tag")
	}
	respEvt, err := c.broker.Request(reqCtx, reqEvt)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || strings.Contains(err.Error(), "context deadline") || strings.Contains(err.Error(), "deadline exceeded") {
			return nil, errors.Wrapf(err, "write tag request timed out waiting for response from reader hardware (target: %s, readerId: %s)", targetAddr, req.ReaderID)
		}
		return nil, errors.Wrapf(err, "write tag request failed for target %s", targetAddr)
	}

	if respEvt == nil || len(respEvt.Data) == 0 {
		return nil, errors.New("empty response received for write tag request")
	}

	writeResp, err := wsbrokerclient.ToTyped[WriteTagResponseData](respEvt)
	if err != nil {
		return nil, errors.Wrap(err, "failed to decode write tag response")
	}

	if writeResp.Data.ReaderID == "" && respEvt.Source != "" {
		writeResp.Data.ReaderID = respEvt.Source
	}

	if writeResp.RefID == "" {
		writeResp.RefID = reqID
	}

	writeData := writeResp.Data

	if c.logger != nil {
		if writeData.IsSuccess() {
			logEvt := c.logger.Info().
				Str("id", writeResp.ID).
				Str("refId", writeResp.RefID).
				Str("target", targetAddr)
			if writeData.ReaderID != "" {
				logEvt = logEvt.Str("readerId", writeData.ReaderID)
			}
			if len(writeData.SuccessfulUIDs) > 0 {
				logEvt = logEvt.Strs("successfulUids", writeData.SuccessfulUIDs)
			} else if writeData.UID != "" {
				logEvt = logEvt.Str("uid", writeData.UID).Str("itemId", writeData.ItemID)
			}
			if writeData.AFI != "" {
				logEvt = logEvt.Str("afi", writeData.AFI)
			}
			logEvt.Msg("RFID tag write succeeded")
		} else if writeData.IsPartial() {
			c.logger.Warn().
				Str("id", writeResp.ID).
				Str("refId", writeResp.RefID).
				Str("target", targetAddr).
				Strs("successfulUids", writeData.SuccessfulUIDs).
				Interface("failedUids", writeData.FailedUIDs).
				Str("error", writeData.GetError()).
				Msg("RFID batch tag write partially succeeded")
		} else {
			c.logger.Warn().
				Str("id", writeResp.ID).
				Str("refId", writeResp.RefID).
				Str("target", targetAddr).
				Str("error", writeData.GetError()).
				Msg("RFID tag write failed")
		}
	}

	return writeResp, nil
}

// WriteTagForReader sends a write tag command directed to a specific logical reader ID.
func (c *LibraryClient) WriteTagForReader(ctx context.Context, target string, readerID string, req WriteTagRequest) (*WriteTagResponse, error) {
	req.ReaderID = readerID
	return c.WriteTag(ctx, target, req)
}

// WriteTagBatch sends a command to program a batch of RFID tags.
func (c *LibraryClient) WriteTagBatch(ctx context.Context, target string, tags []TagWriteItem) (*WriteTagResponse, error) {
	req := NewWriteTagBatchRequest(tags)
	return c.WriteTag(ctx, target, req)
}

// WriteTagBatchStream sends a batch write tag request and streams write tag responses via a channel using RequestN.
func (c *LibraryClient) WriteTagBatchStream(ctx context.Context, target string, tags []TagWriteItem, expectedResults int) (<-chan *WriteTagResponse, error) {
	return c.writeTagBatchStreamWithReader(ctx, target, tags, c.config.defaultReaderID, expectedResults)
}

func (c *LibraryClient) writeTagBatchStreamWithReader(ctx context.Context, target string, tags []TagWriteItem, readerID string, expectedResults int) (<-chan *WriteTagResponse, error) {
	req := NewWriteTagBatchRequest(tags)
	if readerID != "" {
		req.ReaderID = readerID
	}
	if err := req.Validate(); err != nil {
		return nil, err
	}

	targetAddr, err := c.resolveTarget(target)
	if err != nil {
		return nil, err
	}

	if expectedResults <= 0 {
		expectedResults = len(tags)
		if expectedResults == 0 {
			expectedResults = 1
		}
	}

	session, err := wsbrokerclient.GenerateSessionID()
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate correlation session")
	}

	reqEvt, err := wsbrokerclient.NewRawEvent(EventWriteTag, c.broker.ClientName(), targetAddr, req)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create write tag batch request event")
	}
	reqEvt.Session = session
	reqID := reqEvt.ID

	reqCtx := ctx
	var cancel context.CancelFunc
	if _, hasDeadline := ctx.Deadline(); !hasDeadline && c.config.defaultTimeout > 0 {
		timeout := c.config.defaultTimeout
		count := len(tags)
		if count > 1 {
			batchTimeout := time.Duration(count)*2*time.Second + 5*time.Second
			if batchTimeout > timeout {
				timeout = batchTimeout
			}
		}
		reqCtx, cancel = context.WithTimeout(ctx, timeout)
	}

	rawCh := make(chan *event.RawEvent, expectedResults)
	if err := c.broker.RequestN(reqCtx, reqEvt, expectedResults, rawCh); err != nil {
		if cancel != nil {
			cancel()
		}
		return nil, errors.Wrapf(err, "batch write tag stream request failed for target %s", targetAddr)
	}

	respCh := make(chan *WriteTagResponse, expectedResults)

	go func() {
		defer func() {
			if cancel != nil {
				cancel()
			}
			close(respCh)
		}()

		for rawEvt := range rawCh {
			if rawEvt == nil || len(rawEvt.Data) == 0 {
				continue
			}
			typed, err := wsbrokerclient.ToTyped[WriteTagResponseData](rawEvt)
			if err != nil {
				if c.logger != nil {
					c.logger.Warn().Err(err).Msg("failed to decode streaming write tag response item")
				}
				continue
			}
			if typed.Data.ReaderID == "" && rawEvt.Source != "" {
				typed.Data.ReaderID = rawEvt.Source
			}
			if typed.RefID == "" {
				typed.RefID = reqID
			}

			select {
			case respCh <- typed:
			case <-ctx.Done():
				return
			}
		}
	}()

	return respCh, nil
}

// WriteTags is a variadic convenience method for batch programming multiple RFID tags.
func (c *LibraryClient) WriteTags(ctx context.Context, target string, tags ...TagWriteItem) (*WriteTagResponse, error) {
	return c.WriteTagBatch(ctx, target, tags)
}

// SendWriteTagBatch sends an asynchronous batch write tag command to a specific reader target.
func (c *LibraryClient) SendWriteTagBatch(ctx context.Context, target string, tags []TagWriteItem) error {
	return c.sendWriteTagBatchWithReader(ctx, target, tags, c.config.defaultReaderID)
}

func (c *LibraryClient) sendWriteTagBatchWithReader(ctx context.Context, target string, tags []TagWriteItem, readerID string) error {
	req := NewWriteTagBatchRequest(tags)
	if readerID != "" {
		req.ReaderID = readerID
	}
	if err := req.Validate(); err != nil {
		return err
	}

	targetAddr, err := c.resolveTarget(target)
	if err != nil {
		return err
	}

	reqEvt, err := wsbrokerclient.NewRawEvent(EventWriteTag, c.broker.ClientName(), targetAddr, req)
	if err != nil {
		return errors.Wrap(err, "failed to create write tag batch event")
	}

	return c.broker.Send(ctx, reqEvt)
}

// BroadcastWriteTagBatch broadcasts a batch write tag command to a specific broker channel.
func (c *LibraryClient) BroadcastWriteTagBatch(ctx context.Context, channel string, tags []TagWriteItem) error {
	return c.broadcastWriteTagBatchWithReader(ctx, channel, tags, c.config.defaultReaderID)
}

func (c *LibraryClient) broadcastWriteTagBatchWithReader(ctx context.Context, channel string, tags []TagWriteItem, readerID string) error {
	req := NewWriteTagBatchRequest(tags)
	if readerID != "" {
		req.ReaderID = readerID
	}
	if err := req.Validate(); err != nil {
		return err
	}

	targetChan := channel
	if targetChan == "" {
		if len(c.config.channels) > 0 {
			targetChan = c.config.channels[0]
		} else {
			targetChan = "all"
		}
	}

	reqEvt, err := wsbrokerclient.NewRawEvent(EventWriteTag, c.broker.ClientName(), "", req)
	if err != nil {
		return errors.Wrap(err, "failed to create broadcast write tag batch event")
	}
	reqEvt.Channel = targetChan

	return c.broker.Send(ctx, reqEvt)
}

// WriteAFI sends a command to set/toggle the AFI security byte on a transponder.
func (c *LibraryClient) WriteAFI(ctx context.Context, target string, req WriteAFIRequest) (*WriteAFIResponse, error) {
	if req.ReaderID == "" && c.config.defaultReaderID != "" {
		req.ReaderID = c.config.defaultReaderID
	}
	if err := req.Validate(); err != nil {
		return nil, err
	}

	targetAddr, err := c.resolveTarget(target)
	if err != nil {
		return nil, err
	}

	session, err := wsbrokerclient.GenerateSessionID()
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate correlation session")
	}

	reqEvt, err := wsbrokerclient.NewRawEvent(EventWriteAFI, c.broker.ClientName(), targetAddr, req)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create write afi request event")
	}
	reqEvt.Session = session
	reqID := reqEvt.ID

	reqCtx := ctx
	var cancel context.CancelFunc
	if _, hasDeadline := ctx.Deadline(); !hasDeadline && c.config.defaultTimeout > 0 {
		timeout := c.config.defaultTimeout
		count := len(req.UIDs)
		if count > 1 {
			batchTimeout := time.Duration(count)*1500*time.Millisecond + 5*time.Second
			if batchTimeout > timeout {
				timeout = batchTimeout
			}
		}
		reqCtx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	if c.logger != nil {
		logEvt := c.logger.Info().
			Str("id", reqID).
			Str("afi", req.AFI).
			Str("status", AFISecurityStatus(req.AFI)).
			Str("target", targetAddr)
		if req.ReaderID != "" {
			logEvt = logEvt.Str("readerId", req.ReaderID)
		}
		if len(req.UIDs) > 0 {
			logEvt = logEvt.Strs("uids", req.UIDs)
		} else if req.UID != "" {
			logEvt = logEvt.Str("uid", req.UID)
		}
		logEvt.Msg("writing RFID AFI security byte")
	}
	respEvt, err := c.broker.Request(reqCtx, reqEvt)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || strings.Contains(err.Error(), "context deadline") || strings.Contains(err.Error(), "deadline exceeded") {
			return nil, errors.Wrapf(err, "write afi request timed out waiting for response from reader hardware (target: %s, readerId: %s)", targetAddr, req.ReaderID)
		}
		return nil, errors.Wrapf(err, "write afi request failed for target %s", targetAddr)
	}

	if respEvt == nil || len(respEvt.Data) == 0 {
		return nil, errors.New("empty response received for write afi request")
	}

	afiResp, err := wsbrokerclient.ToTyped[WriteAFIResponseData](respEvt)
	if err != nil {
		return nil, errors.Wrap(err, "failed to decode write afi response")
	}

	if afiResp.Data.ReaderID == "" && respEvt.Source != "" {
		afiResp.Data.ReaderID = respEvt.Source
	}
	if afiResp.Data.AFI == "" && req.AFI != "" {
		afiResp.Data.AFI = req.AFI
	}
	if len(afiResp.Data.UIDs) == 0 && len(req.UIDs) > 0 {
		afiResp.Data.UIDs = req.UIDs
	}
	if afiResp.Data.UID == "" && req.UID != "" {
		afiResp.Data.UID = req.UID
	}

	if afiResp.RefID == "" {
		afiResp.RefID = reqID
	}

	afiData := afiResp.Data

	if c.logger != nil {
		if afiData.IsSuccess() {
			logEvt := c.logger.Info().
				Str("id", afiResp.ID).
				Str("refId", afiResp.RefID).
				Str("target", targetAddr).
				Str("afi", afiData.AFI).
				Str("securityStatus", afiData.SecurityStatus())
			if afiData.ReaderID != "" {
				logEvt = logEvt.Str("readerId", afiData.ReaderID)
			}
			if len(afiData.SuccessfulUIDs) > 0 {
				logEvt = logEvt.Strs("successfulUids", afiData.SuccessfulUIDs)
			} else if afiData.UID != "" {
				logEvt = logEvt.Str("uid", afiData.UID)
			}
			logEvt.Msg("RFID AFI security update confirmed")
		} else if afiData.IsPartial() {
			c.logger.Warn().
				Str("id", afiResp.ID).
				Str("refId", afiResp.RefID).
				Str("target", targetAddr).
				Str("afi", afiData.AFI).
				Strs("successfulUids", afiData.SuccessfulUIDs).
				Interface("failedUids", afiData.FailedUIDs).
				Str("error", afiData.GetError()).
				Msg("RFID AFI security update partially succeeded")
		} else {
			c.logger.Warn().
				Str("id", afiResp.ID).
				Str("refId", afiResp.RefID).
				Str("target", targetAddr).
				Str("afi", afiData.AFI).
				Str("error", afiData.GetError()).
				Msg("RFID AFI security update failed")
		}
	}

	return afiResp, nil
}

// WriteAFIForReader sends an AFI write command directed to a specific logical reader ID.
func (c *LibraryClient) WriteAFIForReader(ctx context.Context, target string, readerID string, req WriteAFIRequest) (*WriteAFIResponse, error) {
	req.ReaderID = readerID
	return c.WriteAFI(ctx, target, req)
}

// WriteAFIValue is a convenience wrapper for writing an AFI value to a target transponder.
func (c *LibraryClient) WriteAFIValue(ctx context.Context, target string, uid string, afi string) (*WriteAFIResponse, error) {
	req := NewWriteAFIRequest(afi, uid)
	return c.WriteAFI(ctx, target, req)
}

// WriteAFIBatch is a convenience wrapper for batch writing AFI to multiple transponders.
func (c *LibraryClient) WriteAFIBatch(ctx context.Context, target string, uids []string, afi string) (*WriteAFIResponse, error) {
	req := NewWriteAFIBatchRequest(afi, uids)
	return c.WriteAFI(ctx, target, req)
}

// WriteAFIBatchStream sends a batch AFI write request and streams responses via a channel using RequestN.
func (c *LibraryClient) WriteAFIBatchStream(ctx context.Context, target string, req WriteAFIRequest, expectedResults int) (<-chan *WriteAFIResponse, error) {
	return c.writeAFIBatchStreamWithReader(ctx, target, req, c.config.defaultReaderID, expectedResults)
}

func (c *LibraryClient) writeAFIBatchStreamWithReader(ctx context.Context, target string, req WriteAFIRequest, readerID string, expectedResults int) (<-chan *WriteAFIResponse, error) {
	if readerID != "" {
		req.ReaderID = readerID
	}
	if err := req.Validate(); err != nil {
		return nil, err
	}

	targetAddr, err := c.resolveTarget(target)
	if err != nil {
		return nil, err
	}

	if expectedResults <= 0 {
		expectedResults = len(req.UIDs)
		if expectedResults == 0 {
			expectedResults = 1
		}
	}

	session, err := wsbrokerclient.GenerateSessionID()
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate correlation session")
	}

	reqEvt, err := wsbrokerclient.NewRawEvent(EventWriteAFI, c.broker.ClientName(), targetAddr, req)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create write afi batch request event")
	}
	reqEvt.Session = session
	reqID := reqEvt.ID

	reqCtx := ctx
	var cancel context.CancelFunc
	if _, hasDeadline := ctx.Deadline(); !hasDeadline && c.config.defaultTimeout > 0 {
		timeout := c.config.defaultTimeout
		count := len(req.UIDs)
		if count > 1 {
			batchTimeout := time.Duration(count)*1500*time.Millisecond + 5*time.Second
			if batchTimeout > timeout {
				timeout = batchTimeout
			}
		}
		reqCtx, cancel = context.WithTimeout(ctx, timeout)
	}

	rawCh := make(chan *event.RawEvent, expectedResults)
	if err := c.broker.RequestN(reqCtx, reqEvt, expectedResults, rawCh); err != nil {
		if cancel != nil {
			cancel()
		}
		return nil, errors.Wrapf(err, "batch write afi stream request failed for target %s", targetAddr)
	}

	respCh := make(chan *WriteAFIResponse, expectedResults)

	go func() {
		defer func() {
			if cancel != nil {
				cancel()
			}
			close(respCh)
		}()

		for rawEvt := range rawCh {
			if rawEvt == nil || len(rawEvt.Data) == 0 {
				continue
			}
			typed, err := wsbrokerclient.ToTyped[WriteAFIResponseData](rawEvt)
			if err != nil {
				if c.logger != nil {
					c.logger.Warn().Err(err).Msg("failed to decode streaming write afi response item")
				}
				continue
			}
			if typed.Data.ReaderID == "" && rawEvt.Source != "" {
				typed.Data.ReaderID = rawEvt.Source
			}
			if typed.RefID == "" {
				typed.RefID = reqID
			}

			select {
			case respCh <- typed:
			case <-ctx.Done():
				return
			}
		}
	}()

	return respCh, nil
}

// SetAFISecured sets the AFI byte of a tag (or batch of tags / all tags) to AFISecured ("07").
func (c *LibraryClient) SetAFISecured(ctx context.Context, target string, uid ...string) (*WriteAFIResponse, error) {
	req := WriteAFIRequest{
		AFI: AFISecured,
	}
	if len(uid) == 1 {
		req.UID = strings.TrimSpace(uid[0])
	} else if len(uid) > 1 {
		req.UIDs = uid
	}
	return c.WriteAFI(ctx, target, req)
}

// SetAFIUnsecured sets the AFI byte of a tag (or batch of tags / all tags) to AFIUnsecured ("C2").
func (c *LibraryClient) SetAFIUnsecured(ctx context.Context, target string, uid ...string) (*WriteAFIResponse, error) {
	req := WriteAFIRequest{
		AFI: AFIUnsecured,
	}
	if len(uid) == 1 {
		req.UID = strings.TrimSpace(uid[0])
	} else if len(uid) > 1 {
		req.UIDs = uid
	}
	return c.WriteAFI(ctx, target, req)
}

// SecureTag is a convenience method that writes AFISecured ("07") to a specific tag UID.
func (c *LibraryClient) SecureTag(ctx context.Context, target string, uid string) (*WriteAFIResponse, error) {
	return c.SetAFISecured(ctx, target, uid)
}

// UnsecureTag is a convenience method that writes AFIUnsecured ("C2") to a specific tag UID.
func (c *LibraryClient) UnsecureTag(ctx context.Context, target string, uid string) (*WriteAFIResponse, error) {
	return c.SetAFIUnsecured(ctx, target, uid)
}

// SendAFI sends an asynchronous AFI write command to a specific reader target.
func (c *LibraryClient) SendAFI(ctx context.Context, target string, req WriteAFIRequest) error {
	return c.sendAFIWithReader(ctx, target, req, c.config.defaultReaderID)
}

func (c *LibraryClient) sendAFIWithReader(ctx context.Context, target string, req WriteAFIRequest, readerID string) error {
	if readerID != "" {
		req.ReaderID = readerID
	}
	if err := req.Validate(); err != nil {
		return err
	}

	targetAddr, err := c.resolveTarget(target)
	if err != nil {
		return err
	}

	reqEvt, err := wsbrokerclient.NewRawEvent(EventWriteAFI, c.broker.ClientName(), targetAddr, req)
	if err != nil {
		return errors.Wrap(err, "failed to create write afi event")
	}

	return c.broker.Send(ctx, reqEvt)
}

// BroadcastAFI broadcasts an AFI write command to a specific broker channel.
func (c *LibraryClient) BroadcastAFI(ctx context.Context, channel string, req WriteAFIRequest) error {
	return c.broadcastAFIWithReader(ctx, channel, req, c.config.defaultReaderID)
}

func (c *LibraryClient) broadcastAFIWithReader(ctx context.Context, channel string, req WriteAFIRequest, readerID string) error {
	if readerID != "" {
		req.ReaderID = readerID
	}
	if err := req.Validate(); err != nil {
		return err
	}

	targetChan := channel
	if targetChan == "" {
		if len(c.config.channels) > 0 {
			targetChan = c.config.channels[0]
		} else {
			targetChan = "all"
		}
	}

	reqEvt, err := wsbrokerclient.NewRawEvent(EventWriteAFI, c.broker.ClientName(), "", req)
	if err != nil {
		return errors.Wrap(err, "failed to create broadcast write afi event")
	}
	reqEvt.Channel = targetChan

	return c.broker.Send(ctx, reqEvt)
}
