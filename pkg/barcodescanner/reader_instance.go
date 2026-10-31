package barcodescanner

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"emperror.dev/errors"
	"github.com/je4/Library-RFID-Apps/pkg/barcodeclient"
	"github.com/je4/utils/v2/pkg/zLogger"
	"github.com/rs/zerolog"
)

// ScanCallback is called whenever a new barcode is scanned and validated.
type ScanCallback func(data *barcodeclient.BarcodeReadData)

// BarcodeReaderInstance manages the serial communication and scan processing for a single barcode scanner.
type BarcodeReaderInstance struct {
	config     ReaderConfig
	logger     zLogger.ZLogger
	portOpener PortOpener

	mu           sync.RWMutex
	status       string
	lastError    string
	lastScanTime time.Time
	lastBarcode  string
	lastScanNano int64
	totalScans   int64

	cancelFunc context.CancelFunc
	wg         sync.WaitGroup
	running    atomic.Bool
}

// NewBarcodeReaderInstance creates a new barcode reader instance with configuration.
func NewBarcodeReaderInstance(cfg ReaderConfig, logger zLogger.ZLogger, opener ...PortOpener) *BarcodeReaderInstance {
	cfg.ApplyDefaults()

	if logger == nil {
		nop := zerolog.Nop()
		logger = &nop
	}

	subLogger := logger.With().
		Str("readerId", cfg.ID).
		Str("port", cfg.Port).
		Str("device", cfg.DeviceID).
		Logger()

	var portOpener PortOpener = DefaultPortOpener
	if len(opener) > 0 && opener[0] != nil {
		portOpener = opener[0]
	}

	return &BarcodeReaderInstance{
		config:     cfg,
		logger:     &subLogger,
		portOpener: portOpener,
		status:     "disconnected",
	}
}

// GetConfig returns the reader configuration.
func (r *BarcodeReaderInstance) GetConfig() ReaderConfig {
	return r.config
}

// GetReaderID returns the reader ID.
func (r *BarcodeReaderInstance) GetReaderID() string {
	return r.config.ID
}

// GetStatus returns the current connection status ("connected", "disconnected", "error").
func (r *BarcodeReaderInstance) GetStatus() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.status
}

// Start launches the background connection and scan processing loop.
func (r *BarcodeReaderInstance) Start(ctx context.Context, onScan ScanCallback) error {
	if r.running.Swap(true) {
		return nil // already running
	}

	ctx, cancel := context.WithCancel(ctx)
	r.cancelFunc = cancel

	r.wg.Add(1)
	go r.runLoop(ctx, onScan)

	return nil
}

// Stop gracefully terminates the reader loop.
func (r *BarcodeReaderInstance) Stop() {
	if !r.running.Swap(false) {
		return
	}
	if r.cancelFunc != nil {
		r.cancelFunc()
	}
	r.wg.Wait()

	r.mu.Lock()
	r.status = "stopped"
	r.mu.Unlock()
}

func (r *BarcodeReaderInstance) runLoop(ctx context.Context, onScan ScanCallback) {
	defer r.wg.Done()

	reconnectDelay := time.Duration(r.config.ReconnectSec) * time.Second
	if reconnectDelay <= 0 {
		reconnectDelay = 3 * time.Second
	}

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if err := r.connectAndRead(ctx, onScan); err != nil {
			if ctx.Err() != nil {
				return
			}
			r.logger.Warn().Err(err).Msgf("serial connection closed or failed on port %s, reconnecting in %v...", r.config.Port, reconnectDelay)
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(reconnectDelay):
		}
	}
}

func (r *BarcodeReaderInstance) connectAndRead(ctx context.Context, onScan ScanCallback) error {
	mode, err := r.config.ParseSerialMode()
	if err != nil {
		r.setErrStatus(err)
		return errors.Wrap(err, "invalid serial mode")
	}

	r.logger.Info().Msgf("opening serial port %s (baud: %d, dataBits: %d, parity: %s)...",
		r.config.Port, r.config.BaudRate, r.config.DataBits, r.config.Parity)

	port, err := r.portOpener(r.config.Port, mode)
	if err != nil {
		r.setErrStatus(err)
		return errors.Wrapf(err, "cannot open serial port '%s'", r.config.Port)
	}
	defer port.Close()

	_ = port.SetReadTimeout(500 * time.Millisecond)

	r.mu.Lock()
	r.status = "connected"
	r.lastError = ""
	r.mu.Unlock()

	r.logger.Info().Msgf("successfully connected to barcode reader on port %s", r.config.Port)

	// Close port immediately when context is canceled
	closeDone := make(chan struct{})
	defer close(closeDone)
	go func() {
		select {
		case <-ctx.Done():
			_ = port.Close()
		case <-closeDone:
		}
	}()

	var readBuf [512]byte
	var accBuf bytes.Buffer

	delimBytes := []byte(r.config.Delimiter)
	if len(delimBytes) == 0 {
		delimBytes = []byte("\r")
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		n, rErr := port.Read(readBuf[:])
		if n > 0 {
			accBuf.Write(readBuf[:n])
			r.processBuffer(&accBuf, delimBytes, onScan)
		}

		if rErr != nil {
			if ctx.Err() != nil {
				return nil
			}
			if rErr == io.EOF {
				r.setDisconnectedStatus("EOF from serial port")
				return errors.New("serial port EOF")
			}
			r.setErrStatus(rErr)
			return errors.Wrap(rErr, "serial port read error")
		}
	}
}

func (r *BarcodeReaderInstance) processBuffer(buf *bytes.Buffer, delim []byte, onScan ScanCallback) {
	for {
		data := buf.Bytes()
		idx := bytes.Index(data, delim)

		// Also check standard CR/LF as fallback delimiters if not found by configured delimiter
		if idx < 0 {
			idx = bytes.IndexAny(data, "\r\n")
		}

		if idx < 0 {
			// If buffer gets excessively large without delimiter, prevent memory leak
			if buf.Len() > 4096 {
				r.logger.Warn().Msg("buffer exceeded 4KB without delimiter, discarding stale data")
				buf.Reset()
			}
			break
		}

		rawChunk := string(data[:idx])
		// Advance buffer past delimiter
		delimLen := len(delim)
		if idx+delimLen <= len(data) && bytes.Equal(data[idx:idx+delimLen], delim) {
			buf.Next(idx + delimLen)
		} else {
			// Found fallback single char \r or \n
			buf.Next(idx + 1)
		}

		// Also consume any trailing \r or \n immediately after delimiter
		for buf.Len() > 0 {
			nextB := buf.Bytes()[0]
			if nextB == '\r' || nextB == '\n' {
				buf.Next(1)
			} else {
				break
			}
		}

		r.handleScannedRaw(rawChunk, onScan)
	}
}

func (r *BarcodeReaderInstance) handleScannedRaw(raw string, onScan ScanCallback) {
	cleanCode, symbology := ParseSymbology(raw, r.config.StripAIMID, r.config.TrimWhitespace)
	if cleanCode == "" {
		return
	}

	now := time.Now().UTC()
	nowNano := now.UnixNano()
	debounceDuration := time.Duration(r.config.DebounceMs) * time.Millisecond

	r.mu.Lock()
	if r.lastBarcode == cleanCode && debounceDuration > 0 {
		if time.Duration(nowNano-r.lastScanNano) < debounceDuration {
			r.mu.Unlock()
			r.logger.Debug().Str("barcode", cleanCode).Msg("ignoring duplicate scan within debounce window")
			return
		}
	}

	r.lastBarcode = cleanCode
	r.lastScanNano = nowNano
	r.lastScanTime = now
	r.totalScans++
	scansCount := r.totalScans
	r.mu.Unlock()

	r.logger.Info().
		Str("barcode", cleanCode).
		Str("symbology", symbology).
		Int64("totalScans", scansCount).
		Msg("scanned barcode")

	payload := &barcodeclient.BarcodeReadData{
		ReaderID:   r.config.ID,
		Barcode:    cleanCode,
		ItemID:     cleanCode,
		Symbology:  symbology,
		Raw:        raw,
		Timestamp:  barcodeclient.FormatTime(now),
		DeviceID:   r.config.DeviceID,
		DeviceType: r.config.DeviceType,
	}

	if onScan != nil {
		onScan(payload)
	}
}

func (r *BarcodeReaderInstance) setErrStatus(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.status = "error"
	if err != nil {
		r.lastError = err.Error()
	}
}

func (r *BarcodeReaderInstance) setDisconnectedStatus(reason string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.status = "disconnected"
	r.lastError = reason
}

// GetMetadata returns current device metadata and diagnostic status.
func (r *BarcodeReaderInstance) GetMetadata() *barcodeclient.DeviceMetadataResponseData {
	r.mu.RLock()
	defer r.mu.RUnlock()

	lastScanStr := ""
	if !r.lastScanTime.IsZero() {
		lastScanStr = barcodeclient.FormatTime(r.lastScanTime)
	}

	desc := fmt.Sprintf("%s on %s (%d baud, %d data bits, %s parity)",
		r.config.DeviceType, r.config.Port, r.config.BaudRate, r.config.DataBits, r.config.Parity)

	return &barcodeclient.DeviceMetadataResponseData{
		ReaderID:     r.config.ID,
		Status:       r.status,
		DeviceID:     r.config.DeviceID,
		DeviceType:   r.config.DeviceType,
		Port:         r.config.Port,
		BaudRate:     r.config.BaudRate,
		Description:  desc,
		TotalScans:   r.totalScans,
		LastScanTime: lastScanStr,
	}
}
