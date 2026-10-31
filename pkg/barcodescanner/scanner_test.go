package barcodescanner_test

import (
	"bytes"
	"context"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/je4/Library-RFID-Apps/pkg/barcodescanner"
	"github.com/je4/Library-RFID-Apps/pkg/libraryclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.bug.st/serial"
)

// mockSerialPort implements barcodescanner.SerialPort for testing.
type mockSerialPort struct {
	mu     sync.Mutex
	buf    *bytes.Buffer
	closed bool
	notify chan struct{}
}

func newMockSerialPort(data []byte) *mockSerialPort {
	return &mockSerialPort{
		buf:    bytes.NewBuffer(data),
		notify: make(chan struct{}, 1),
	}
}

func (m *mockSerialPort) Read(p []byte) (n int, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return 0, io.EOF
	}

	if m.buf.Len() == 0 {
		return 0, io.EOF
	}

	return m.buf.Read(p)
}

func (m *mockSerialPort) Write(p []byte) (n int, err error) {
	return len(p), nil
}

func (m *mockSerialPort) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}

func (m *mockSerialPort) SetReadTimeout(t time.Duration) error {
	return nil
}

func (m *mockSerialPort) AppendData(data []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.buf.Write(data)
}

func TestBarcodeReaderInstance_ScanProcessing(t *testing.T) {
	testData := []byte("]C13012345678\r\n]C13012345679\r\n")
	mockPort := newMockSerialPort(testData)

	opener := func(portName string, mode *serial.Mode) (barcodescanner.SerialPort, error) {
		return mockPort, nil
	}

	cfg := barcodescanner.ReaderConfig{
		ID:             "scanner-01",
		DeviceID:       "honeywell-1300g-01",
		DeviceType:     "honeywell-1300g",
		Port:           "COM3",
		BaudRate:       9600,
		Delimiter:      "\r\n",
		TrimWhitespace: true,
		StripAIMID:     true,
		DebounceMs:     100,
		ReconnectSec:   1,
	}

	inst := barcodescanner.NewBarcodeReaderInstance(cfg, nil, opener)
	assert.Equal(t, "scanner-01", inst.GetReaderID())

	var scanned []*libraryclient.BarcodeReadData
	var mu sync.Mutex

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := inst.Start(ctx, func(data *libraryclient.BarcodeReadData) {
		mu.Lock()
		scanned = append(scanned, data)
		mu.Unlock()
	})
	require.NoError(t, err)

	// Wait briefly for read loop to process input
	time.Sleep(150 * time.Millisecond)
	inst.Stop()

	mu.Lock()
	defer mu.Unlock()

	require.Len(t, scanned, 2)
	assert.Equal(t, "3012345678", scanned[0].Barcode)
	assert.Equal(t, "scanner-01", scanned[0].ReaderID)
	assert.Equal(t, "Code 128 / GS1-128", scanned[0].Symbology)

	assert.Equal(t, "3012345679", scanned[1].Barcode)
	assert.Equal(t, "scanner-01", scanned[1].ReaderID)

	meta := inst.GetMetadata()
	assert.Equal(t, "scanner-01", meta.ReaderID)
	assert.Equal(t, int64(2), meta.TotalScans)
}

func TestBarcodeReaderRegistry(t *testing.T) {
	reg := barcodescanner.NewBarcodeReaderRegistry()

	cfg1 := barcodescanner.ReaderConfig{ID: "reader-01", Port: "COM1"}
	cfg2 := barcodescanner.ReaderConfig{ID: "reader-02", Port: "COM2"}

	inst1 := barcodescanner.NewBarcodeReaderInstance(cfg1, nil)
	inst2 := barcodescanner.NewBarcodeReaderInstance(cfg2, nil)

	reg.RegisterReader(inst1)
	reg.RegisterReader(inst2)

	assert.Len(t, reg.GetAllReaders(), 2)

	found1, ok := reg.GetReader("READER-01")
	assert.True(t, ok)
	assert.Equal(t, "reader-01", found1.GetReaderID())

	def := reg.GetDefaultReader()
	require.NotNil(t, def)
	assert.Equal(t, "reader-01", def.GetReaderID())
}
