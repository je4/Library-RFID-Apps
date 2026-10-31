package rfidclient

import (
	"context"
	"strings"
	"time"

	"emperror.dev/errors"
	"github.com/je4/wsbrokerclient"
	"github.com/je4/wsbrokerclient/pkg/event"
)

// WriteAFI sends a command to set/toggle the AFI security byte on a transponder.
func (c *RFIDClient) WriteAFI(ctx context.Context, target string, req WriteAFIRequest) (*WriteAFIResponse, error) {
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
		// Dynamically scale timeout for batch operations if multiple UIDs are provided
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
				Str("afi", afiData.AFI).
				Str("status", AFISecurityStatus(afiData.AFI)).
				Str("target", targetAddr)
			if afiData.ReaderID != "" {
				logEvt = logEvt.Str("readerId", afiData.ReaderID)
			}
			if len(afiData.SuccessfulUIDs) > 0 {
				logEvt = logEvt.Strs("successfulUids", afiData.SuccessfulUIDs)
			} else if afiData.UID != "" {
				logEvt = logEvt.Str("uid", afiData.UID)
			}
			logEvt.Msg("RFID AFI security byte updated successfully")
		} else if afiData.IsPartial() {
			c.logger.Warn().
				Str("id", afiResp.ID).
				Str("refId", afiResp.RefID).
				Str("afi", afiData.AFI).
				Str("target", targetAddr).
				Str("readerId", afiData.ReaderID).
				Strs("successfulUids", afiData.SuccessfulUIDs).
				Interface("failedUids", afiData.FailedUIDs).
				Str("error", afiData.GetError()).
				Msg("RFID AFI security byte partially updated")
		} else {
			c.logger.Warn().
				Str("id", afiResp.ID).
				Str("refId", afiResp.RefID).
				Str("afi", afiData.AFI).
				Str("target", targetAddr).
				Str("readerId", afiData.ReaderID).
				Str("uid", afiData.UID).
				Str("error", afiData.GetError()).
				Msg("failed to update RFID AFI security byte")
		}
	}

	return afiResp, nil
}

// WriteAFIForReader sends an AFI write command directed to a specific logical reader ID.
func (c *RFIDClient) WriteAFIForReader(ctx context.Context, target string, readerID string, req WriteAFIRequest) (*WriteAFIResponse, error) {
	req.ReaderID = readerID
	return c.WriteAFI(ctx, target, req)
}

// WriteAFIValue is a convenience wrapper for writing an AFI value to a target reader/transponder.
func (c *RFIDClient) WriteAFIValue(ctx context.Context, target string, uid string, afi string) (*WriteAFIResponse, error) {
	return c.WriteAFI(ctx, target, NewWriteAFIRequest(afi, uid))
}

// WriteAFIBatch is a convenience wrapper for batch writing AFI to multiple transponders.
func (c *RFIDClient) WriteAFIBatch(ctx context.Context, target string, uids []string, afi string) (*WriteAFIResponse, error) {
	return c.WriteAFI(ctx, target, NewWriteAFIBatchRequest(afi, uids))
}

// WriteAFIBatchStream sends a batch AFI write request and streams responses via a channel using RequestN.
func (c *RFIDClient) WriteAFIBatchStream(ctx context.Context, target string, req WriteAFIRequest, expectedResults int) (<-chan *WriteAFIResponse, error) {
	if req.ReaderID == "" && c.config.defaultReaderID != "" {
		req.ReaderID = c.config.defaultReaderID
	}
	if err := req.Validate(); err != nil {
		return nil, err
	}
	if expectedResults <= 0 {
		if len(req.UIDs) > 0 {
			expectedResults = len(req.UIDs)
		} else {
			expectedResults = 1
		}
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
		return nil, errors.Wrap(err, "failed to create write AFI request event")
	}
	reqEvt.Session = session
	reqID := reqEvt.ID

	rawCh := make(chan *event.RawEvent, expectedResults)
	outCh := make(chan *WriteAFIResponse, expectedResults)

	go func() {
		defer close(outCh)
		for raw := range rawCh {
			if raw == nil || len(raw.Data) == 0 {
				continue
			}
			typed, err := wsbrokerclient.ToTyped[WriteAFIResponseData](raw)
			if err != nil {
				if c.logger != nil {
					c.logger.Warn().Err(err).Msg("failed to decode streaming write AFI response item")
				}
				continue
			}
			if typed.Data.ReaderID == "" && raw.Source != "" {
				typed.Data.ReaderID = raw.Source
			}
			if typed.Data.AFI == "" && req.AFI != "" {
				typed.Data.AFI = req.AFI
			}
			if typed.RefID == "" {
				typed.RefID = reqID
			}
			outCh <- typed
		}
	}()

	if err := c.broker.RequestN(ctx, reqEvt, expectedResults, rawCh); err != nil {
		return nil, errors.Wrapf(err, "failed to start streaming batch AFI write request to %s", targetAddr)
	}

	return outCh, nil
}

// SetAFISecured sets the AFI byte of a tag (or batch of tags / all tags) to AFISecured ("07").
func (c *RFIDClient) SetAFISecured(ctx context.Context, target string, uid ...string) (*WriteAFIResponse, error) {
	return c.WriteAFI(ctx, target, NewWriteAFIRequest(AFISecured, uid...))
}

// SetAFIUnsecured sets the AFI byte of a tag (or batch of tags / all tags) to AFIUnsecured ("C2").
func (c *RFIDClient) SetAFIUnsecured(ctx context.Context, target string, uid ...string) (*WriteAFIResponse, error) {
	return c.WriteAFI(ctx, target, NewWriteAFIRequest(AFIUnsecured, uid...))
}

// SecureTag writes AFISecured ("07") to the specified transponder UID.
func (c *RFIDClient) SecureTag(ctx context.Context, target string, uid string) (*WriteAFIResponse, error) {
	return c.WriteAFIValue(ctx, target, uid, AFISecured)
}

// UnsecureTag writes AFIUnsecured ("C2") to the specified transponder UID.
func (c *RFIDClient) UnsecureTag(ctx context.Context, target string, uid string) (*WriteAFIResponse, error) {
	return c.WriteAFIValue(ctx, target, uid, AFIUnsecured)
}

// SendAFI sends an asynchronous AFI write command to a specific reader target or default reader.
func (c *RFIDClient) SendAFI(ctx context.Context, target string, req WriteAFIRequest) error {
	if req.ReaderID == "" && c.config.defaultReaderID != "" {
		req.ReaderID = c.config.defaultReaderID
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
		return errors.Wrap(err, "failed to create AFI write request event")
	}

	if c.logger != nil {
		logEvt := c.logger.Info().
			Str("id", reqEvt.ID).
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
		logEvt.Msg("sending RFID AFI write command")
	}

	if err := c.broker.Request0(ctx, reqEvt); err != nil {
		c.logger.Warn().
			Err(err).
			Str("id", reqEvt.ID).
			Str("afi", req.AFI).
			Str("target", targetAddr).
			Str("uid", req.UID).
			Msg("failed to send RFID AFI write command")
		return errors.Wrapf(err, "failed to send AFI write command to target %s", targetAddr)
	}

	c.logger.Info().
		Str("id", reqEvt.ID).
		Str("afi", req.AFI).
		Str("status", AFISecurityStatus(req.AFI)).
		Str("target", targetAddr).
		Str("uid", req.UID).
		Msg("RFID AFI write command sent successfully")

	return nil
}

// BroadcastAFI broadcasts an AFI write command to a specific broker channel or configured channels.
func (c *RFIDClient) BroadcastAFI(ctx context.Context, channel string, req WriteAFIRequest) error {
	if req.ReaderID == "" && c.config.defaultReaderID != "" {
		req.ReaderID = c.config.defaultReaderID
	}
	if err := req.Validate(); err != nil {
		return err
	}

	channels := make([]string, 0)
	if strings.TrimSpace(channel) != "" {
		channels = append(channels, strings.TrimSpace(channel))
	} else if len(c.config.channels) > 0 {
		for _, ch := range c.config.channels {
			if strings.TrimSpace(ch) != "" {
				channels = append(channels, strings.TrimSpace(ch))
			}
		}
	} else {
		channels = append(channels, "all")
	}

	var firstErr error
	for _, ch := range channels {
		reqEvt, err := wsbrokerclient.NewRawEvent(EventWriteAFI, c.broker.ClientName(), ch, req)
		if err != nil {
			if firstErr == nil {
				firstErr = errors.Wrap(err, "failed to create AFI broadcast event")
			}
			continue
		}
		reqEvt.Channel = ch

		c.logger.Info().
			Str("id", reqEvt.ID).
			Str("afi", req.AFI).
			Str("status", AFISecurityStatus(req.AFI)).
			Str("channel", ch).
			Str("uid", req.UID).
			Msg("broadcasting RFID AFI write command")

		if err := c.broker.Request0(ctx, reqEvt); err != nil {
			c.logger.Warn().
				Err(err).
				Str("id", reqEvt.ID).
				Str("afi", req.AFI).
				Str("channel", ch).
				Str("uid", req.UID).
				Msg("failed to broadcast RFID AFI write command")
			if firstErr == nil {
				firstErr = errors.Wrapf(err, "failed to broadcast AFI write command to channel %s", ch)
			}
		} else {
			c.logger.Info().
				Str("id", reqEvt.ID).
				Str("afi", req.AFI).
				Str("status", AFISecurityStatus(req.AFI)).
				Str("channel", ch).
				Str("uid", req.UID).
				Msg("RFID AFI write command broadcast successfully")
		}
	}

	return firstErr
}
