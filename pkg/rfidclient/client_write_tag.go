package rfidclient

import (
	"context"
	"strings"
	"time"

	"emperror.dev/errors"
	"github.com/je4/wsbrokerclient"
	"github.com/je4/wsbrokerclient/pkg/event"
)

// WriteTag sends a command to write ISO 28560 metadata and optional AFI to a transponder.
func (c *RFIDClient) WriteTag(ctx context.Context, target string, req WriteTagRequest) (*WriteTagResponse, error) {
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
				logEvt = logEvt.Str("uid", writeData.UID).
					Str("itemId", writeData.ItemID).
					Str("afi", writeData.AFI)
			}
			logEvt.Msg("RFID tag written successfully")
		} else if writeData.IsPartial() {
			c.logger.Warn().
				Str("id", writeResp.ID).
				Str("refId", writeResp.RefID).
				Str("target", targetAddr).
				Str("readerId", writeData.ReaderID).
				Strs("successfulUids", writeData.SuccessfulUIDs).
				Interface("failedUids", writeData.FailedUIDs).
				Str("error", writeData.GetError()).
				Msg("RFID tags partially written")
		} else {
			c.logger.Warn().
				Str("id", writeResp.ID).
				Str("refId", writeResp.RefID).
				Str("itemId", writeData.ItemID).
				Str("uid", writeData.UID).
				Str("target", targetAddr).
				Str("readerId", writeData.ReaderID).
				Str("error", writeData.GetError()).
				Msg("failed to write RFID tag")
		}
	}

	return writeResp, nil
}

// WriteTagForReader sends a write tag command directed to a specific logical reader ID.
func (c *RFIDClient) WriteTagForReader(ctx context.Context, target string, readerID string, req WriteTagRequest) (*WriteTagResponse, error) {
	req.ReaderID = readerID
	return c.WriteTag(ctx, target, req)
}

// WriteTagBatch sends a command to program a batch of RFID tags with individual metadata and optional AFI.
func (c *RFIDClient) WriteTagBatch(ctx context.Context, target string, tags []TagWriteItem) (*WriteTagResponse, error) {
	if len(tags) == 0 {
		return nil, errors.New("write tag batch: tags list must not be empty")
	}
	return c.WriteTag(ctx, target, NewWriteTagBatchRequest(tags))
}

// WriteTagBatchStream sends a batch write tag request and streams write tag responses via a channel using RequestN.
func (c *RFIDClient) WriteTagBatchStream(ctx context.Context, target string, tags []TagWriteItem, expectedResults int) (<-chan *WriteTagResponse, error) {
	return c.writeTagBatchStreamWithReader(ctx, target, tags, c.config.defaultReaderID, expectedResults)
}

func (c *RFIDClient) writeTagBatchStreamWithReader(ctx context.Context, target string, tags []TagWriteItem, readerID string, expectedResults int) (<-chan *WriteTagResponse, error) {
	if len(tags) == 0 {
		return nil, errors.New("write tag batch stream: tags list must not be empty")
	}
	if expectedResults <= 0 {
		expectedResults = len(tags)
	}
	req := NewWriteTagBatchRequest(tags)
	if readerID != "" {
		req.ReaderID = readerID
	} else if c.config.defaultReaderID != "" {
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

	rawCh := make(chan *event.RawEvent, expectedResults)
	outCh := make(chan *WriteTagResponse, expectedResults)

	go func() {
		defer close(outCh)
		for raw := range rawCh {
			if raw == nil || len(raw.Data) == 0 {
				continue
			}
			typed, err := wsbrokerclient.ToTyped[WriteTagResponseData](raw)
			if err != nil {
				if c.logger != nil {
					c.logger.Warn().Err(err).Msg("failed to decode streaming write tag response item")
				}
				continue
			}
			if typed.Data.ReaderID == "" && raw.Source != "" {
				typed.Data.ReaderID = raw.Source
			}
			if typed.RefID == "" {
				typed.RefID = reqID
			}
			outCh <- typed
		}
	}()

	if err := c.broker.RequestN(ctx, reqEvt, expectedResults, rawCh); err != nil {
		return nil, errors.Wrapf(err, "failed to start streaming batch write tag request to %s", targetAddr)
	}

	return outCh, nil
}

// WriteTags is a variadic convenience method for batch programming multiple RFID tags.
func (c *RFIDClient) WriteTags(ctx context.Context, target string, tags ...TagWriteItem) (*WriteTagResponse, error) {
	return c.WriteTagBatch(ctx, target, tags)
}

// SendWriteTagBatch sends an asynchronous batch write tag command to a specific reader target.
func (c *RFIDClient) SendWriteTagBatch(ctx context.Context, target string, tags []TagWriteItem) error {
	return c.sendWriteTagBatchWithReader(ctx, target, tags, "")
}

func (c *RFIDClient) sendWriteTagBatchWithReader(ctx context.Context, target string, tags []TagWriteItem, readerID string) error {
	if len(tags) == 0 {
		return errors.New("send write tag batch: tags list must not be empty")
	}
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
		return errors.Wrap(err, "failed to create batch write tag request event")
	}

	if c.logger != nil {
		c.logger.Info().
			Str("id", reqEvt.ID).
			Int("count", len(tags)).
			Str("target", targetAddr).
			Msg("sending batch RFID write tag command")
	}

	if err := c.broker.Request0(ctx, reqEvt); err != nil {
		if c.logger != nil {
			c.logger.Warn().
				Err(err).
				Str("id", reqEvt.ID).
				Str("target", targetAddr).
				Int("count", len(tags)).
				Msg("failed to send batch RFID write tag command")
		}
		return errors.Wrapf(err, "failed to send batch write tag command to target %s", targetAddr)
	}

	if c.logger != nil {
		c.logger.Info().
			Str("id", reqEvt.ID).
			Str("target", targetAddr).
			Int("count", len(tags)).
			Msg("batch RFID write tag command sent successfully")
	}

	return nil
}

// BroadcastWriteTagBatch broadcasts a batch write tag command to a specific broker channel or configured channels.
func (c *RFIDClient) BroadcastWriteTagBatch(ctx context.Context, channel string, tags []TagWriteItem) error {
	return c.broadcastWriteTagBatchWithReader(ctx, channel, tags, "")
}

func (c *RFIDClient) broadcastWriteTagBatchWithReader(ctx context.Context, channel string, tags []TagWriteItem, readerID string) error {
	if len(tags) == 0 {
		return errors.New("broadcast write tag batch: tags list must not be empty")
	}
	req := NewWriteTagBatchRequest(tags)
	if readerID != "" {
		req.ReaderID = readerID
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
		reqEvt, err := wsbrokerclient.NewRawEvent(EventWriteTag, c.broker.ClientName(), ch, req)
		if err != nil {
			if firstErr == nil {
				firstErr = errors.Wrap(err, "failed to create batch write tag broadcast event")
			}
			continue
		}
		reqEvt.Channel = ch

		if c.logger != nil {
			c.logger.Info().
				Str("id", reqEvt.ID).
				Int("count", len(tags)).
				Str("channel", ch).
				Msg("broadcasting batch RFID write tag command")
		}

		if err := c.broker.Request0(ctx, reqEvt); err != nil {
			if c.logger != nil {
				c.logger.Warn().
					Err(err).
					Str("id", reqEvt.ID).
					Str("channel", ch).
					Msg("failed to broadcast batch RFID write tag command")
			}
			if firstErr == nil {
				firstErr = errors.Wrapf(err, "failed to broadcast batch write tag command to channel %s", ch)
			}
		} else if c.logger != nil {
			c.logger.Info().
				Str("id", reqEvt.ID).
				Str("channel", ch).
				Int("count", len(tags)).
				Msg("batch RFID write tag command broadcast successfully")
		}
	}

	return firstErr
}
