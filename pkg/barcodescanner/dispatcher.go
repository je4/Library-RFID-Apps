package barcodescanner

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"emperror.dev/errors"
	"github.com/je4/Library-RFID-Apps/pkg/barcodeclient"
	"github.com/je4/utils/v2/pkg/zLogger"
	"github.com/je4/wsbrokerclient"
	"github.com/je4/wsbrokerclient/pkg/event"
	"github.com/rs/zerolog"
)

// BarcodeWsBrokerDispatcher coordinates WSBroker connectivity, event broadcasting, and command routing.
type BarcodeWsBrokerDispatcher struct {
	clientName     string
	globalChannels []string
	registry       *BarcodeReaderRegistry
	broker         wsbrokerclient.ClientInterface
	logger         zLogger.ZLogger

	unsubBroker wsbrokerclient.UnsubscribeFunc
	closeOnce   sync.Once
}

// NewBarcodeWsBrokerDispatcher creates a new dispatcher instance.
func NewBarcodeWsBrokerDispatcher(
	clientName string,
	globalChannels []string,
	registry *BarcodeReaderRegistry,
	broker wsbrokerclient.ClientInterface,
	logger zLogger.ZLogger,
) (*BarcodeWsBrokerDispatcher, error) {
	if registry == nil {
		return nil, errors.New("registry cannot be nil")
	}
	if broker == nil {
		return nil, errors.New("broker cannot be nil")
	}
	if clientName == "" {
		clientName = "barcode-headless"
	}

	if logger == nil {
		nop := zerolog.Nop()
		logger = &nop
	}

	subLogger := logger.With().Str("component", "barcode-dispatcher").Str("client", clientName).Logger()

	d := &BarcodeWsBrokerDispatcher{
		clientName:     clientName,
		globalChannels: globalChannels,
		registry:       registry,
		broker:         broker,
		logger:         &subLogger,
	}

	// Register event router with wsbrokerclient
	d.unsubBroker = d.broker.OnEvent(func(evt *event.RawEvent) {
		d.handleIncomingEvent(evt)
	})

	return d, nil
}

// ClientName returns the configured dispatcher client identifier.
func (d *BarcodeWsBrokerDispatcher) ClientName() string {
	return d.clientName
}

// Connect connects the underlying WSBroker client and subscribes to all configured channels.
func (d *BarcodeWsBrokerDispatcher) Connect(ctx context.Context) error {
	d.logger.Info().Msgf("connecting to WSBroker as '%s'...", d.clientName)
	if err := d.broker.Connect(ctx); err != nil {
		d.logger.Warn().Err(err).Msg("initial WSBroker connection attempt failed (auto-reconnect active)")
		return err
	}

	allChannels := d.CollectAllChannels()
	if len(allChannels) > 0 {
		d.logger.Info().Strs("channels", allChannels).Msg("attaching to configured channels...")
		if err := d.broker.Attach(ctx, allChannels...); err != nil {
			d.logger.Warn().Err(err).Msg("failed to attach to some initial channels")
		}
	}

	return nil
}

// CollectAllChannels returns the deduplicated union of global channels and all per-reader channels.
func (d *BarcodeWsBrokerDispatcher) CollectAllChannels() []string {
	seen := make(map[string]struct{})
	var res []string

	for _, ch := range d.globalChannels {
		trimmed := strings.TrimSpace(ch)
		if trimmed != "" {
			if _, ok := seen[trimmed]; !ok {
				seen[trimmed] = struct{}{}
				res = append(res, trimmed)
			}
		}
	}

	for _, inst := range d.registry.GetAllReaders() {
		for _, ch := range inst.GetConfig().Channels {
			trimmed := strings.TrimSpace(ch)
			if trimmed != "" {
				if _, ok := seen[trimmed]; !ok {
					seen[trimmed] = struct{}{}
					res = append(res, trimmed)
				}
			}
		}
	}

	return res
}

// ChannelsForReader returns the channels that should receive scans from a given reader.
func (d *BarcodeWsBrokerDispatcher) ChannelsForReader(readerID string) []string {
	if inst, ok := d.registry.GetReader(readerID); ok {
		readerChans := inst.GetConfig().Channels
		if len(readerChans) > 0 {
			return readerChans
		}
	}
	return d.globalChannels
}

// DispatchScan broadcasts a scanned barcode event to all target channels.
func (d *BarcodeWsBrokerDispatcher) DispatchScan(ctx context.Context, scan *barcodeclient.BarcodeReadData) {
	if scan == nil || !d.broker.IsConnected() {
		return
	}

	targetChannels := d.ChannelsForReader(scan.ReaderID)
	if len(targetChannels) == 0 {
		d.logger.Warn().Str("readerId", scan.ReaderID).Msg("no target channels configured for reader, skipping broadcast")
		return
	}

	d.logger.Debug().
		Str("barcode", scan.Barcode).
		Str("readerId", scan.ReaderID).
		Strs("channels", targetChannels).
		Msg("broadcasting barcode read event")

	for _, ch := range targetChannels {
		rawEvt, err := wsbrokerclient.NewRawEvent(
			barcodeclient.EventBarcodeRead,
			d.clientName,
			ch,
			scan,
		)
		if err != nil {
			d.logger.Error().Err(err).Msg("failed to create BarcodeRead event")
			continue
		}
		rawEvt.Channel = ch

		if err := d.broker.Send(ctx, rawEvt); err != nil {
			d.logger.Warn().Err(err).Str("channel", ch).Msg("failed to send barcode read event to channel")
		}
	}
}

// Close gracefully disconnects and releases listeners.
func (d *BarcodeWsBrokerDispatcher) Close() error {
	var err error
	d.closeOnce.Do(func() {
		if d.unsubBroker != nil {
			d.unsubBroker()
		}
		err = d.broker.Close()
	})
	return err
}

func (d *BarcodeWsBrokerDispatcher) handleIncomingEvent(raw *event.RawEvent) {
	if raw == nil || len(raw.Data) == 0 {
		return
	}

	// Ignore events generated by ourselves
	if raw.Source != "" && strings.EqualFold(raw.Source, d.clientName) {
		return
	}

	// Ignore standard ping / pong / attach / detach
	switch raw.Type {
	case event.TypePing, event.TypePong, event.TypeAttach, event.TypeDetach:
		return
	}

	eventName := extractEventNameFromRaw(raw)
	normalized := strings.TrimSpace(strings.ToLower(eventName))

	switch normalized {
	case barcodeclient.EventGetDeviceMetadata,
		"barcode:get_device_metadata",
		"rfid/get_device_metadata", "rfid:get_device_metadata":
		d.handleMetadataQuery(raw)
	}
}

func (d *BarcodeWsBrokerDispatcher) handleMetadataQuery(raw *event.RawEvent) {
	var req barcodeclient.DeviceMetadataRequest
	_ = json.Unmarshal(raw.Data, &req)

	targetReaderID := strings.TrimSpace(req.ReaderID)

	var matchedReaders []*BarcodeReaderInstance
	if targetReaderID != "" && !strings.EqualFold(targetReaderID, "all") && !strings.EqualFold(targetReaderID, "*") {
		if inst, ok := d.registry.GetReader(targetReaderID); ok {
			matchedReaders = append(matchedReaders, inst)
		} else {
			d.logger.Warn().Str("readerId", targetReaderID).Msg("metadata requested for unknown reader")
		}
	} else {
		// All readers or default reader
		all := d.registry.GetAllReaders()
		if len(all) > 0 {
			matchedReaders = all
		}
	}

	replyTarget := raw.Source
	if replyTarget == "" {
		replyTarget = raw.Channel
	}

	if len(matchedReaders) == 0 {
		respData := &barcodeclient.DeviceMetadataResponseData{
			ReaderID:   targetReaderID,
			Status:     "disconnected",
			DeviceID:   targetReaderID,
			DeviceType: "serial-barcode-scanner",
		}
		d.sendMetadataResponse(raw, replyTarget, respData)
		return
	}

	for _, inst := range matchedReaders {
		respData := inst.GetMetadata()
		d.sendMetadataResponse(raw, replyTarget, respData)
	}
}

func (d *BarcodeWsBrokerDispatcher) sendMetadataResponse(orig *event.RawEvent, target string, data *barcodeclient.DeviceMetadataResponseData) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	respEvt, err := wsbrokerclient.NewRawEvent(
		barcodeclient.EventDeviceMetadataResponse,
		d.clientName,
		target,
		data,
	)
	if err != nil {
		d.logger.Error().Err(err).Msg("failed to construct metadata response event")
		return
	}

	respEvt.RefID = orig.ID
	respEvt.Session = orig.Session

	if err := d.broker.Send(ctx, respEvt); err != nil {
		d.logger.Warn().Err(err).Str("target", target).Msg("failed to send device metadata response")
	} else {
		d.logger.Debug().Str("target", target).Str("readerId", data.ReaderID).Msg("sent device metadata response")
	}
}

func extractEventNameFromRaw(raw *event.RawEvent) string {
	rawTypeStr := string(raw.Type)
	if rawTypeStr != "" && rawTypeStr != "message" && rawTypeStr != "action" {
		return rawTypeStr
	}

	var envelope struct {
		Event string `json:"event"`
	}
	if len(raw.Data) > 0 {
		_ = json.Unmarshal(raw.Data, &envelope)
		if envelope.Event != "" {
			return envelope.Event
		}
	}

	return rawTypeStr
}
