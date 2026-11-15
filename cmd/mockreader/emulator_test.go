package main

import (
	"context"
	"encoding/json"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/beevik/ntp"
	"github.com/je4/Library-RFID-Apps/pkg/libraryclient"
	"github.com/je4/utils/v2/pkg/zLogger"
	"github.com/je4/wsbrokerclient/pkg/client"
	"github.com/je4/wsbrokerclient/pkg/event"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockBrokerClient struct {
	mu          sync.RWMutex
	clientName  string
	sentEvents  []*event.RawEvent
	msgHandlers []client.EventHandler
}

func newMockBroker(name string) *mockBrokerClient {
	return &mockBrokerClient{
		clientName: name,
	}
}

func (m *mockBrokerClient) ClientName() string                    { return m.clientName }
func (m *mockBrokerClient) Mode() client.ConnectionMode           { return client.ModeRW }
func (m *mockBrokerClient) IsConnected() bool                     { return true }
func (m *mockBrokerClient) GetState() client.ConnectionState      { return client.StateConnected }
func (m *mockBrokerClient) Channels() []string                    { return []string{"mediathek01", "gates"} }
func (m *mockBrokerClient) Connect(ctx context.Context) error     { return nil }
func (m *mockBrokerClient) ConnectSync(ctx context.Context) error { return nil }
func (m *mockBrokerClient) Close() error                          { return nil }
func (m *mockBrokerClient) Send(ctx context.Context, evt *event.RawEvent) error {
	m.mu.Lock()
	m.sentEvents = append(m.sentEvents, evt)
	m.mu.Unlock()
	return nil
}
func (m *mockBrokerClient) SendMessage(ctx context.Context, target string, data any) error {
	return nil
}
func (m *mockBrokerClient) Broadcast(ctx context.Context, channel string, data any) error { return nil }
func (m *mockBrokerClient) SendDirect(ctx context.Context, targetClient string, data any) error {
	return nil
}
func (m *mockBrokerClient) Reply(ctx context.Context, originalEvt *event.RawEvent, data any) error {
	return nil
}
func (m *mockBrokerClient) ReplyType(ctx context.Context, originalEvt *event.RawEvent, replyType event.EventType, data any) error {
	return nil
}
func (m *mockBrokerClient) CreateReply(original *event.RawEvent, replyType event.EventType, data any) (*event.RawEvent, error) {
	return nil, nil
}
func (m *mockBrokerClient) Request(ctx context.Context, evt *event.RawEvent) (*event.RawEvent, error) {
	return nil, nil
}
func (m *mockBrokerClient) Request0(ctx context.Context, evt *event.RawEvent) error {
	return nil
}
func (m *mockBrokerClient) Request1(ctx context.Context, evt *event.RawEvent) (*event.RawEvent, error) {
	return nil, nil
}
func (m *mockBrokerClient) RequestN(ctx context.Context, evt *event.RawEvent, numResults int, ch chan *event.RawEvent) error {
	return nil
}
func (m *mockBrokerClient) Attach(ctx context.Context, channels ...string) error { return nil }
func (m *mockBrokerClient) Detach(ctx context.Context, channels ...string) error { return nil }
func (m *mockBrokerClient) Subscribe(ctx context.Context, channel string) (client.Subscription, error) {
	return nil, nil
}
func (m *mockBrokerClient) Ping(ctx context.Context) error { return nil }
func (m *mockBrokerClient) QueryNTP(ctx context.Context, data []byte) ([]byte, error) {
	return nil, nil
}
func (m *mockBrokerClient) NTP(ctx context.Context, opts ...ntp.QueryOptions) (*ntp.Response, error) {
	return nil, nil
}
func (m *mockBrokerClient) ClockOffset() time.Duration { return 0 }
func (m *mockBrokerClient) Time() time.Time            { return time.Now() }
func (m *mockBrokerClient) SyncNTP(ctx context.Context, opts ...ntp.QueryOptions) (time.Duration, error) {
	return 0, nil
}
func (m *mockBrokerClient) NewNTPConn(ctx context.Context) net.Conn { return nil }
func (m *mockBrokerClient) QueryChannelInfo(ctx context.Context, channel string) (*event.ChannelInfo, error) {
	return nil, nil
}
func (m *mockBrokerClient) ChannelInfo(ctx context.Context, channel string) (*event.ChannelInfo, error) {
	return nil, nil
}
func (m *mockBrokerClient) OnEvent(handler client.EventHandler) client.UnsubscribeFunc {
	m.mu.Lock()
	m.msgHandlers = append(m.msgHandlers, handler)
	m.mu.Unlock()
	return func() {}
}
func (m *mockBrokerClient) OnMessage(handler client.EventHandler) client.UnsubscribeFunc {
	return func() {}
}
func (m *mockBrokerClient) OnChannel(channel string, handler client.EventHandler) client.UnsubscribeFunc {
	return func() {}
}
func (m *mockBrokerClient) OnDirect(handler client.EventHandler) client.UnsubscribeFunc {
	return func() {}
}
func (m *mockBrokerClient) OnType(t event.EventType, handler client.EventHandler) client.UnsubscribeFunc {
	return func() {}
}
func (m *mockBrokerClient) OnConnect(callback func()) client.UnsubscribeFunc {
	return func() {}
}
func (m *mockBrokerClient) OnDisconnect(callback func(error)) client.UnsubscribeFunc {
	return func() {}
}
func (m *mockBrokerClient) OnReconnect(callback func()) client.UnsubscribeFunc {
	return func() {}
}
func (m *mockBrokerClient) OnError(callback func(error)) client.UnsubscribeFunc {
	return func() {}
}

func (m *mockBrokerClient) triggerEvent(evt *event.RawEvent) {
	m.mu.RLock()
	handlers := append([]client.EventHandler(nil), m.msgHandlers...)
	m.mu.RUnlock()
	for _, h := range handlers {
		h(evt)
	}
}

func (m *mockBrokerClient) getSentEvents() []*event.RawEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]*event.RawEvent(nil), m.sentEvents...)
}

func TestGenerateNextTag_CyclingAndStructure(t *testing.T) {
	conf := &MockReaderConfig{
		Name:                  "test:mockreader",
		ReaderID:              "reader-42",
		Channels:              []string{"ch1"},
		AFIMode:               "random",
		AFISecuredProbability: 0.5,
		ISIL:                  "CH-001596-0",
		Country:               "CH",
		CRCOk:                 true,
		Barcodes:              []string{"BC-001", "BC-002", "BC-003"},
	}
	require.NoError(t, conf.applyDefaults())

	broker := newMockBroker("test:mockreader")
	nop := zerolog.Nop()
	var logger zLogger.ZLogger = &nop

	emulator, err := NewMockReader(conf, broker, logger)
	require.NoError(t, err)
	defer emulator.Close()

	// Generate 3 tags, verify they cycle through the barcodes
	tag1 := emulator.GenerateNextTag()
	tag2 := emulator.GenerateNextTag()
	tag3 := emulator.GenerateNextTag()
	tag4 := emulator.GenerateNextTag() // should wrap around to BC-001

	assert.Equal(t, "BC-001", tag1.ItemID)
	assert.Equal(t, "BC-002", tag2.ItemID)
	assert.Equal(t, "BC-003", tag3.ItemID)
	assert.Equal(t, "BC-001", tag4.ItemID)

	assert.Equal(t, "reader-42", tag1.ReaderID)
	assert.True(t, tag1.CRCOk)
	assert.True(t, tag1.IsCRCValid)
	assert.Equal(t, "OK", tag1.CRC)
	assert.Equal(t, "CH-001596-0", tag1.ISIL)
	assert.Equal(t, "CH", tag1.Country)
	assert.NotNil(t, tag1.Time)
	assert.NotEmpty(t, tag1.UID)
	assert.True(t, len(tag1.UID) >= 16)
}

func TestGenerateNextTag_PredefinedTags(t *testing.T) {
	p := 2
	pn := 1
	ut := 3
	conf := &MockReaderConfig{
		Name:     "test:mockreader",
		ReaderID: "reader-01",
		Channels: []string{"ch1"},
		Tags: []TagDefinition{
			{
				UID:       "E004010099999991",
				ItemID:    "CUSTOM-01",
				AFI:       "07",
				ISIL:      "CH-999999-0",
				Country:   "CH",
				Parts:     p,
				PartNo:    pn,
				UsageType: ut,
			},
		},
	}
	require.NoError(t, conf.applyDefaults())

	broker := newMockBroker("test:mockreader")
	nop := zerolog.Nop()
	var logger zLogger.ZLogger = &nop

	emulator, err := NewMockReader(conf, broker, logger)
	require.NoError(t, err)
	defer emulator.Close()

	tag := emulator.GenerateNextTag()
	assert.Equal(t, "E004010099999991", tag.UID)
	assert.Equal(t, "CUSTOM-01", tag.ItemID)
	assert.Equal(t, "07", tag.AFI)
	assert.Equal(t, "CH-999999-0", tag.ISIL)
	assert.Equal(t, 2, tag.GetParts())
	assert.Equal(t, 1, tag.GetPartNo())
	assert.Equal(t, 3, tag.GetUsageType())
}

func TestDetermineAFI_RandomDistribution(t *testing.T) {
	conf := &MockReaderConfig{
		Name:                  "test:mockreader",
		ReaderID:              "reader-01",
		Channels:              []string{"ch1"},
		AFIMode:               "random",
		AFISecuredProbability: 0.5,
		Barcodes:              []string{"BC-A", "BC-B"},
	}
	require.NoError(t, conf.applyDefaults())

	broker := newMockBroker("test:mockreader")
	nop := zerolog.Nop()
	var logger zLogger.ZLogger = &nop

	emulator, err := NewMockReader(conf, broker, logger)
	require.NoError(t, err)
	defer emulator.Close()

	securedCount := 0
	unsecuredCount := 0
	const samples = 200

	for i := 0; i < samples; i++ {
		tag := emulator.GenerateNextTag()
		if tag.AFI == libraryclient.AFISecured {
			securedCount++
		} else if tag.AFI == libraryclient.AFIUnsecured {
			unsecuredCount++
		}
	}

	assert.True(t, securedCount > 30, "expected some secured tags (got %d)", securedCount)
	assert.True(t, unsecuredCount > 30, "expected some unsecured tags (got %d)", unsecuredCount)
	assert.Equal(t, samples, securedCount+unsecuredCount)
}

func TestEmitTagReadAndClear(t *testing.T) {
	conf := &MockReaderConfig{
		Name:     "test:mockreader",
		ReaderID: "reader-01",
		Channels: []string{"mediathek01", "gates"},
		Barcodes: []string{"BC-001"},
	}
	require.NoError(t, conf.applyDefaults())

	broker := newMockBroker("test:mockreader")
	nop := zerolog.Nop()
	var logger zLogger.ZLogger = &nop

	emulator, err := NewMockReader(conf, broker, logger)
	require.NoError(t, err)
	defer emulator.Close()

	tag := emulator.GenerateNextTag()
	err = emulator.EmitTagRead(context.Background(), tag)
	require.NoError(t, err)

	events := broker.getSentEvents()
	require.Len(t, events, 2)
	assert.Equal(t, "mediathek01", events[0].Channel)
	assert.Equal(t, event.EventType(libraryclient.EventTagRead), events[0].Type)
	assert.Equal(t, "gates", events[1].Channel)
	assert.Equal(t, event.EventType(libraryclient.EventTagRead), events[1].Type)

	// Emit tags cleared
	err = emulator.EmitTagsCleared(context.Background())
	require.NoError(t, err)

	events = broker.getSentEvents()
	require.Len(t, events, 4)
	assert.Equal(t, event.EventType(libraryclient.EventTagsCleared), events[2].Type)
	assert.Equal(t, event.EventType(libraryclient.EventTagsCleared), events[3].Type)
}

func TestHandleDeviceMetadataRequest(t *testing.T) {
	conf := &MockReaderConfig{
		Name:       "test:mockreader",
		ReaderID:   "reader-01",
		DeviceID:   "mock-feig-01",
		DeviceType: "mock-rfid-reader",
		Channels:   []string{"mediathek01"},
	}
	require.NoError(t, conf.applyDefaults())

	broker := newMockBroker("test:mockreader")
	nop := zerolog.Nop()
	var logger zLogger.ZLogger = &nop

	emulator, err := NewMockReader(conf, broker, logger)
	require.NoError(t, err)
	defer emulator.Close()

	reqEvt, err := event.NewMessageEvent("client-requester", "mediathek01", libraryclient.DeviceMetadataRequest{
		ReaderID: "reader-01",
	})
	require.NoError(t, err)
	reqEvt.Type = event.EventType(libraryclient.EventGetDeviceMetadata)

	broker.triggerEvent(reqEvt)

	events := broker.getSentEvents()
	require.Len(t, events, 1)
	assert.Equal(t, event.EventType(libraryclient.EventDeviceMetadataResponse), events[0].Type)

	var respData libraryclient.DeviceMetadataResponseData
	err = json.Unmarshal(events[0].Data, &respData)
	require.NoError(t, err)
	assert.Equal(t, "reader-01", respData.ReaderID)
	assert.Equal(t, "online", respData.Status)
	assert.Equal(t, "mock-feig-01", respData.DeviceID)
}

func TestHandleWriteTagAndAFIRequest(t *testing.T) {
	conf := &MockReaderConfig{
		Name:     "test:mockreader",
		ReaderID: "reader-01",
		Channels: []string{"mediathek01"},
		Barcodes: []string{"BC-001"},
	}
	require.NoError(t, conf.applyDefaults())

	broker := newMockBroker("test:mockreader")
	nop := zerolog.Nop()
	var logger zLogger.ZLogger = &nop

	emulator, err := NewMockReader(conf, broker, logger)
	require.NoError(t, err)
	defer emulator.Close()

	// Write tag
	writeTagEvt, err := event.NewMessageEvent("client-requester", "mediathek01", libraryclient.WriteTagRequest{
		ReaderID: "reader-01",
		Tags: []libraryclient.TagWriteItem{
			{UID: "E004010011223344", ItemID: "NEW-BARCODE-999"},
		},
	})
	require.NoError(t, err)
	writeTagEvt.Type = event.EventType(libraryclient.EventWriteTag)

	broker.triggerEvent(writeTagEvt)

	events := broker.getSentEvents()
	require.Len(t, events, 1)
	assert.Equal(t, event.EventType(libraryclient.EventWriteTagResponse), events[0].Type)

	// Write AFI
	writeAFIEvt, err := event.NewMessageEvent("client-requester", "mediathek01", libraryclient.WriteAFIRequest{
		ReaderID: "reader-01",
		AFI:      "C2",
		UIDs:     []string{"E004010011223344"},
	})
	require.NoError(t, err)
	writeAFIEvt.Type = event.EventType(libraryclient.EventWriteAFI)

	broker.triggerEvent(writeAFIEvt)

	events = broker.getSentEvents()
	require.Len(t, events, 2)
	assert.Equal(t, event.EventType(libraryclient.EventWriteAFIResponse), events[1].Type)
}

func TestLifecycle_RunAndStop(t *testing.T) {
	conf := &MockReaderConfig{
		Name:           "test:mockreader",
		ReaderID:       "reader-01",
		Channels:       []string{"mediathek01"},
		Interval:       "50ms",
		SendClearEvent: true,
		TagDuration:    "20ms",
		Barcodes:       []string{"BC-001", "BC-002"},
	}
	require.NoError(t, conf.applyDefaults())

	broker := newMockBroker("test:mockreader")
	nop := zerolog.Nop()
	var logger zLogger.ZLogger = &nop

	emulator, err := NewMockReader(conf, broker, logger)
	require.NoError(t, err)
	defer emulator.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	_ = emulator.Run(ctx)

	events := broker.getSentEvents()
	assert.NotEmpty(t, events)
}
