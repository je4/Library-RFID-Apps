package libraryclient

import (
	"context"
	"net"
	"strings"
	"sync"
	"time"

	"emperror.dev/errors"
	"github.com/beevik/ntp"
	"github.com/je4/wsbrokerclient/pkg/client"
	"github.com/je4/wsbrokerclient/pkg/event"
)

// fakeBrokerClient implements wsbrokerclient.ClientInterface for comprehensive testing.
type fakeBrokerClient struct {
	mu           sync.RWMutex
	clientName   string
	mode         client.ConnectionMode
	channels     map[string]struct{}
	state        client.ConnectionState
	connected    bool
	msgHandlers  []client.EventHandler
	lastSent     *event.RawEvent
	requestFunc  func(ctx context.Context, evt *event.RawEvent) (*event.RawEvent, error)
	requestNFunc func(ctx context.Context, evt *event.RawEvent, numResults int, ch chan *event.RawEvent) error
	attachedChan []string
}

func newFakeBroker(clientName string) *fakeBrokerClient {
	return &fakeBrokerClient{
		clientName: clientName,
		mode:       client.ModeRW,
		channels:   make(map[string]struct{}),
		state:      client.StateConnected,
		connected:  true,
	}
}

func (f *fakeBrokerClient) ClientName() string               { return f.clientName }
func (f *fakeBrokerClient) Mode() client.ConnectionMode      { return f.mode }
func (f *fakeBrokerClient) IsConnected() bool                { return f.connected }
func (f *fakeBrokerClient) GetState() client.ConnectionState { return f.state }
func (f *fakeBrokerClient) Channels() []string {
	f.mu.RLock()
	defer f.mu.RUnlock()
	res := make([]string, 0, len(f.channels))
	for ch := range f.channels {
		res = append(res, ch)
	}
	return res
}
func (f *fakeBrokerClient) Connect(ctx context.Context) error     { return nil }
func (f *fakeBrokerClient) ConnectSync(ctx context.Context) error { return nil }
func (f *fakeBrokerClient) Close() error                          { return nil }

func (f *fakeBrokerClient) Send(ctx context.Context, evt *event.RawEvent) error {
	f.mu.Lock()
	f.lastSent = evt
	f.mu.Unlock()
	return nil
}
func (f *fakeBrokerClient) SendMessage(ctx context.Context, target string, data any) error {
	evt, err := event.NewMessageEvent(f.clientName, target, data)
	if err != nil {
		return err
	}
	return f.Send(ctx, evt)
}
func (f *fakeBrokerClient) Broadcast(ctx context.Context, channel string, data any) error {
	return f.SendMessage(ctx, channel, data)
}
func (f *fakeBrokerClient) SendDirect(ctx context.Context, targetClient string, data any) error {
	return f.SendMessage(ctx, targetClient, data)
}
func (f *fakeBrokerClient) Reply(ctx context.Context, originalEvt *event.RawEvent, data any) error {
	replyEvt, err := event.NewReplyEvent(originalEvt, event.TypeMessage, data)
	if err != nil {
		return err
	}
	replyEvt.Source = f.clientName
	return f.Send(ctx, replyEvt)
}
func (f *fakeBrokerClient) ReplyType(ctx context.Context, originalEvt *event.RawEvent, replyType event.EventType, data any) error {
	replyEvt, err := event.NewReplyEvent(originalEvt, replyType, data)
	if err != nil {
		return err
	}
	replyEvt.Source = f.clientName
	return f.Send(ctx, replyEvt)
}
func (f *fakeBrokerClient) CreateReply(original *event.RawEvent, replyType event.EventType, data any) (*event.RawEvent, error) {
	replyEvt, err := event.NewReplyEvent(original, replyType, data)
	if err != nil {
		return nil, err
	}
	replyEvt.Source = f.clientName
	return replyEvt, nil
}
func (f *fakeBrokerClient) Request(ctx context.Context, evt *event.RawEvent) (*event.RawEvent, error) {
	f.mu.Lock()
	f.lastSent = evt
	reqFunc := f.requestFunc
	f.mu.Unlock()

	if reqFunc != nil {
		return reqFunc(ctx, evt)
	}
	return nil, errors.New("no requestFunc configured on fake broker")
}
func (f *fakeBrokerClient) Request0(ctx context.Context, evt *event.RawEvent) error {
	f.mu.Lock()
	f.lastSent = evt
	f.mu.Unlock()
	return f.Send(ctx, evt)
}
func (f *fakeBrokerClient) Request1(ctx context.Context, evt *event.RawEvent) (*event.RawEvent, error) {
	return f.Request(ctx, evt)
}
func (f *fakeBrokerClient) RequestN(ctx context.Context, evt *event.RawEvent, numResults int, ch chan *event.RawEvent) error {
	f.mu.Lock()
	f.lastSent = evt
	reqNFunc := f.requestNFunc
	reqFunc := f.requestFunc
	f.mu.Unlock()

	if reqNFunc != nil {
		return reqNFunc(ctx, evt, numResults, ch)
	}

	if numResults <= 0 {
		if ch != nil {
			close(ch)
		}
		return f.Send(ctx, evt)
	}
	if reqFunc != nil {
		resp, err := reqFunc(ctx, evt)
		if err != nil {
			if ch != nil {
				close(ch)
			}
			return err
		}
		if ch != nil {
			if resp != nil {
				ch <- resp
			}
			close(ch)
		}
		return nil
	}
	if ch != nil {
		close(ch)
	}
	return f.Send(ctx, evt)
}
func (f *fakeBrokerClient) Attach(ctx context.Context, channels ...string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, ch := range channels {
		f.channels[ch] = struct{}{}
		f.attachedChan = append(f.attachedChan, ch)
	}
	return nil
}
func (f *fakeBrokerClient) Detach(ctx context.Context, channels ...string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, ch := range channels {
		delete(f.channels, ch)
	}
	return nil
}

type fakeSubscription struct {
	channel string
	events  chan *event.RawEvent
	unsub   func()
	once    sync.Once
}

func (s *fakeSubscription) Channel() string                { return s.channel }
func (s *fakeSubscription) Events() <-chan *event.RawEvent { return s.events }
func (s *fakeSubscription) Unsubscribe() error {
	s.once.Do(func() {
		if s.unsub != nil {
			s.unsub()
		}
	})
	return nil
}

func (f *fakeBrokerClient) Subscribe(ctx context.Context, channel string) (client.Subscription, error) {
	eventsCh := make(chan *event.RawEvent, 64)
	unsub := f.OnEvent(func(evt *event.RawEvent) {
		if channel != "" && evt.Target != "" && !strings.EqualFold(evt.Target, channel) && !strings.EqualFold(evt.Target, "all") && !strings.EqualFold(evt.Channel, channel) && !strings.EqualFold(evt.Channel, "all") {
			return
		}
		select {
		case eventsCh <- evt:
		default:
		}
	})

	sub := &fakeSubscription{
		channel: channel,
		events:  eventsCh,
		unsub: func() {
			unsub()
			close(eventsCh)
		},
	}
	return sub, nil
}
func (f *fakeBrokerClient) Ping(ctx context.Context) error { return nil }
func (f *fakeBrokerClient) QueryNTP(ctx context.Context, data []byte) ([]byte, error) {
	return nil, nil
}
func (f *fakeBrokerClient) NTP(ctx context.Context, opts ...ntp.QueryOptions) (*ntp.Response, error) {
	return nil, nil
}
func (f *fakeBrokerClient) ClockOffset() time.Duration { return 0 }
func (f *fakeBrokerClient) Time() time.Time            { return time.Now() }
func (f *fakeBrokerClient) SyncNTP(ctx context.Context, opts ...ntp.QueryOptions) (time.Duration, error) {
	return 0, nil
}
func (f *fakeBrokerClient) NewNTPConn(ctx context.Context) net.Conn { return nil }
func (f *fakeBrokerClient) QueryChannelInfo(ctx context.Context, channel string) (*event.ChannelInfo, error) {
	return nil, nil
}
func (f *fakeBrokerClient) ChannelInfo(ctx context.Context, channel string) (*event.ChannelInfo, error) {
	return nil, nil
}
func (f *fakeBrokerClient) OnEvent(handler client.EventHandler) client.UnsubscribeFunc {
	f.mu.Lock()
	f.msgHandlers = append(f.msgHandlers, handler)
	f.mu.Unlock()
	return func() {
		f.mu.Lock()
		defer f.mu.Unlock()
		for i, h := range f.msgHandlers {
			if strings.EqualFold(f.clientName, f.clientName) && i < len(f.msgHandlers) {
				_ = h
				f.msgHandlers = append(f.msgHandlers[:i], f.msgHandlers[i+1:]...)
				break
			}
		}
	}
}
func (f *fakeBrokerClient) OnMessage(handler client.EventHandler) client.UnsubscribeFunc {
	f.mu.Lock()
	f.msgHandlers = append(f.msgHandlers, handler)
	f.mu.Unlock()
	return func() {}
}
func (f *fakeBrokerClient) OnChannel(channel string, handler client.EventHandler) client.UnsubscribeFunc {
	return func() {}
}
func (f *fakeBrokerClient) OnDirect(handler client.EventHandler) client.UnsubscribeFunc {
	return func() {}
}
func (f *fakeBrokerClient) OnType(t event.EventType, handler client.EventHandler) client.UnsubscribeFunc {
	return func() {}
}
func (f *fakeBrokerClient) OnConnect(callback func()) client.UnsubscribeFunc {
	return func() {}
}
func (f *fakeBrokerClient) OnDisconnect(callback func(error)) client.UnsubscribeFunc {
	return func() {}
}
func (f *fakeBrokerClient) OnReconnect(callback func()) client.UnsubscribeFunc {
	return func() {}
}
func (f *fakeBrokerClient) OnError(callback func(error)) client.UnsubscribeFunc {
	return func() {}
}

func (f *fakeBrokerClient) emitIncomingEvent(evt *event.RawEvent) {
	f.mu.RLock()
	handlers := append([]client.EventHandler(nil), f.msgHandlers...)
	f.mu.RUnlock()

	for _, h := range handlers {
		h(evt)
	}
}
