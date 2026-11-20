package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math/rand"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"emperror.dev/errors"
	"github.com/je4/Library-RFID-Apps/pkg/libraryclient"
	"github.com/je4/utils/v2/pkg/zLogger"
	"github.com/je4/wsbrokerclient"
	"github.com/je4/wsbrokerclient/pkg/event"
	"github.com/rs/zerolog"
)

// MockReader emulates a physical RFID tag reader by periodically generating and publishing
// RFID tag read and clear events to configured WSBroker channels, and responding to device queries.
type MockReader struct {
	config *MockReaderConfig
	broker wsbrokerclient.ClientInterface
	logger zLogger.ZLogger

	tagIndex    uint64
	tagStoreMu  sync.RWMutex
	inMemoryMap map[string]*libraryclient.TagReadData // UID -> TagReadData for simulated write persistence

	unsubBroker wsbrokerclient.UnsubscribeFunc
	closeOnce   sync.Once
	rnd         *rand.Rand
	rndMu       sync.Mutex
}

// NewMockReader creates a new MockReader instance.
func NewMockReader(
	config *MockReaderConfig,
	broker wsbrokerclient.ClientInterface,
	logger zLogger.ZLogger,
) (*MockReader, error) {
	if config == nil {
		return nil, errors.New("config cannot be nil")
	}
	if broker == nil {
		return nil, errors.New("broker cannot be nil")
	}
	if logger == nil {
		nop := zerolog.Nop()
		logger = &nop
	}

	subLogger := logger.With().
		Str("component", "mockreader").
		Str("readerId", config.ReaderID).
		Logger()

	src := rand.NewSource(time.Now().UnixNano())
	r := &MockReader{
		config:      config,
		broker:      broker,
		logger:      &subLogger,
		inMemoryMap: make(map[string]*libraryclient.TagReadData),
		rnd:         rand.New(src),
	}

	// Register event listener with wsbrokerclient for device metadata and write requests
	r.unsubBroker = r.broker.OnEvent(func(evt *event.RawEvent) {
		r.handleIncomingEvent(evt)
	})

	return r, nil
}

// NextRandomFloat returns a thread-safe random float64 in [0.0, 1.0).
func (r *MockReader) NextRandomFloat() float64 {
	r.rndMu.Lock()
	defer r.rndMu.Unlock()
	return r.rnd.Float64()
}

// GenerateNextTag constructs the next simulated TagReadData payload based on config and in-memory tags.
func (r *MockReader) GenerateNextTag() *libraryclient.TagReadData {
	now := time.Now().UTC()
	parts := 1
	partNo := 1
	usageType := 1

	// If predefined tags are configured, cycle through them
	if len(r.config.Tags) > 0 {
		idx := atomic.AddUint64(&r.tagIndex, 1) - 1
		def := r.config.Tags[idx%uint64(len(r.config.Tags))]

		uid := def.UID
		if uid == "" {
			uid = GenerateUIDFromBarcode(def.ItemID)
		}

		afi := def.AFI
		if afi == "" {
			afi = r.config.DetermineAFI(r.NextRandomFloat())
		}

		r.tagStoreMu.RLock()
		if stored, ok := r.inMemoryMap[uid]; ok {
			// Use updated values if tag was rewritten in memory
			afi = stored.AFI
		}
		r.tagStoreMu.RUnlock()

		p := def.Parts
		if p <= 0 {
			p = parts
		}
		pn := def.PartNo
		if pn <= 0 {
			pn = partNo
		}
		ut := def.UsageType
		if ut <= 0 {
			ut = usageType
		}

		return &libraryclient.TagReadData{
			ReaderID:   r.config.ReaderID,
			UID:        uid,
			ItemID:     def.ItemID,
			ISIL:       def.ISIL,
			Country:    def.Country,
			CRCOk:      r.config.CRCOk,
			IsCRCValid: r.config.CRCOk,
			CRC:        "OK",
			Parts:      &p,
			PartNo:     &pn,
			UsageType:  &ut,
			AFI:        libraryclient.NormalizeAFI(afi),
			Time:       &now,
		}
	}

	// Otherwise, generate from configured Barcodes
	barcodes := r.config.Barcodes
	if len(barcodes) == 0 {
		barcodes = []string{"31145000012345"}
	}

	idx := atomic.AddUint64(&r.tagIndex, 1) - 1
	barcode := barcodes[idx%uint64(len(barcodes))]
	return r.GenerateTagForBarcode(barcode)
}

// GenerateTagForBarcode constructs a simulated TagReadData payload for a specific barcode/itemID.
func (r *MockReader) GenerateTagForBarcode(barcode string) *libraryclient.TagReadData {
	now := time.Now().UTC()
	parts := 1
	partNo := 1
	usageType := 1

	uid := GenerateUIDFromBarcode(barcode)
	afi := r.config.DetermineAFI(r.NextRandomFloat())

	r.tagStoreMu.RLock()
	if stored, ok := r.inMemoryMap[uid]; ok {
		// Use updated values if tag was rewritten in memory
		afi = stored.AFI
		if stored.ItemID != "" {
			barcode = stored.ItemID
		}
	}
	r.tagStoreMu.RUnlock()

	return &libraryclient.TagReadData{
		ReaderID:   r.config.ReaderID,
		UID:        uid,
		ItemID:     barcode,
		ISIL:       r.config.ISIL,
		Country:    r.config.Country,
		CRCOk:      r.config.CRCOk,
		IsCRCValid: r.config.CRCOk,
		CRC:        "OK",
		Parts:      &parts,
		PartNo:     &partNo,
		UsageType:  &usageType,
		AFI:        libraryclient.NormalizeAFI(afi),
		Time:       &now,
	}
}

// SendBarcodes sends mock-UID TagRead events for a list of barcodes.
// If clearBefore is true, an empty reader (tags_cleared) event is emitted first.
func (r *MockReader) SendBarcodes(ctx context.Context, barcodes []string, clearBefore bool) error {
	if clearBefore {
		if err := r.EmitTagsCleared(ctx); err != nil {
			r.logger.Warn().Err(err).Msg("failed to emit initial tags_cleared event")
		}
		time.Sleep(50 * time.Millisecond)
	}

	var errs []error
	for _, bc := range barcodes {
		trimmed := strings.TrimSpace(bc)
		if trimmed == "" {
			continue
		}
		tag := r.GenerateTagForBarcode(trimmed)
		if err := r.EmitTagRead(ctx, tag); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return errors.Combine(errs...)
	}
	return nil
}

// SendEmpty sends an empty reader (tags_cleared) event to all configured channels.
func (r *MockReader) SendEmpty(ctx context.Context) error {
	return r.EmitTagsCleared(ctx)
}

// GenerateUIDFromBarcode creates a deterministic, valid-looking 16-character hex ISO 15693 UID.
func GenerateUIDFromBarcode(barcode string) string {
	if strings.HasPrefix(strings.ToUpper(barcode), "E004") && len(barcode) == 16 {
		return strings.ToUpper(barcode)
	}
	hash := sha256.Sum256([]byte(barcode))
	hexStr := hex.EncodeToString(hash[:6]) // 12 hex chars
	return "E004" + strings.ToUpper(hexStr)
}

// EmitTagRead publishes a TagReadEvent for the given tag to all configured channels.
func (r *MockReader) EmitTagRead(ctx context.Context, tag *libraryclient.TagReadData) error {
	if tag == nil {
		return errors.New("tag cannot be nil")
	}

	r.logger.Info().
		Str("readerId", tag.ReaderID).
		Str("uid", tag.UID).
		Str("itemId", tag.ItemID).
		Str("afi", tag.AFI).
		Str("status", tag.SecurityStatus()).
		Strs("channels", r.config.Channels).
		Msg("emitted simulated tag_read event")

	var errs []error
	for _, ch := range r.config.Channels {
		rawEvt, err := wsbrokerclient.NewRawEvent(
			libraryclient.EventTagRead,
			r.config.Name,
			ch,
			tag,
		)
		if err != nil {
			r.logger.Error().Err(err).Str("channel", ch).Msg("failed to create tag_read raw event")
			errs = append(errs, err)
			continue
		}
		rawEvt.Channel = ch

		if err := r.broker.Send(ctx, rawEvt); err != nil {
			r.logger.Warn().Err(err).Str("channel", ch).Msg("failed to send tag_read event to broker channel")
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return errors.Combine(errs...)
	}
	return nil
}

// EmitTagsCleared publishes a TagsClearedEvent to all configured channels.
func (r *MockReader) EmitTagsCleared(ctx context.Context) error {
	clearedData := libraryclient.TagsClearedData{
		ReaderID: r.config.ReaderID,
		Status:   "cleared",
	}

	r.logger.Debug().
		Str("readerId", clearedData.ReaderID).
		Strs("channels", r.config.Channels).
		Msg("emitted simulated tags_cleared event")

	var errs []error
	for _, ch := range r.config.Channels {
		rawEvt, err := wsbrokerclient.NewRawEvent(
			libraryclient.EventTagsCleared,
			r.config.Name,
			ch,
			clearedData,
		)
		if err != nil {
			r.logger.Error().Err(err).Str("channel", ch).Msg("failed to create tags_cleared raw event")
			errs = append(errs, err)
			continue
		}
		rawEvt.Channel = ch

		if err := r.broker.Send(ctx, rawEvt); err != nil {
			r.logger.Warn().Err(err).Str("channel", ch).Msg("failed to send tags_cleared event to broker channel")
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return errors.Combine(errs...)
	}
	return nil
}

// NextInterval computes the delay before the next tag emission cycle.
func (r *MockReader) NextInterval() time.Duration {
	if !r.config.RandomInterval {
		return r.config.IntervalDuration
	}

	minDur := r.config.MinIntervalDuration
	maxDur := r.config.MaxIntervalDuration
	if maxDur <= minDur {
		return minDur
	}

	diff := maxDur - minDur
	r.rndMu.Lock()
	randAdd := time.Duration(r.rnd.Int63n(int64(diff)))
	r.rndMu.Unlock()

	return minDur + randAdd
}

// Run executes the continuous tag generation and dispatching loop until context is canceled.
func (r *MockReader) Run(ctx context.Context) error {
	r.logger.Info().
		Str("interval", r.config.Interval).
		Bool("randomInterval", r.config.RandomInterval).
		Str("afiMode", r.config.AFIMode).
		Float64("afiSecuredProb", r.config.AFISecuredProbability).
		Bool("sendClearEvent", r.config.SendClearEvent).
		Str("tagDuration", r.config.TagDuration).
		Strs("channels", r.config.Channels).
		Int("barcodesCount", len(r.config.Barcodes)).
		Msg("starting mockreader tag generation loop")

	for {
		interval := r.NextInterval()
		select {
		case <-ctx.Done():
			r.logger.Info().Msg("mockreader generation loop stopped")
			return ctx.Err()
		case <-time.After(interval):
		}

		tag := r.GenerateNextTag()
		_ = r.EmitTagRead(ctx, tag)

		if r.config.SendClearEvent && r.config.TagDurationValue > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(r.config.TagDurationValue):
			}
			_ = r.EmitTagsCleared(ctx)
		}
	}
}

// Close gracefully closes the mock reader and unregisters broker handlers.
func (r *MockReader) Close() error {
	var err error
	r.closeOnce.Do(func() {
		if r.unsubBroker != nil {
			r.unsubBroker()
		}
	})
	return err
}

func (r *MockReader) handleIncomingEvent(raw *event.RawEvent) {
	if raw == nil || len(raw.Data) == 0 {
		return
	}

	// Ignore events generated by ourselves
	if raw.Source != "" && strings.EqualFold(raw.Source, r.config.Name) {
		return
	}

	switch raw.Type {
	case event.TypePing, event.TypePong, event.TypeAttach, event.TypeDetach:
		return
	}

	eventName := string(raw.Type)
	normalized := strings.TrimSpace(strings.ToLower(eventName))

	switch normalized {
	case libraryclient.EventGetDeviceMetadata, "rfid:get_device_metadata":
		r.handleDeviceMetadataRequest(raw)
	case libraryclient.EventWriteTag, "rfid:write_tag":
		r.handleWriteTagRequest(raw)
	case libraryclient.EventWriteAFI, "rfid:write_afi":
		r.handleWriteAFIRequest(raw)
	}
}

func (r *MockReader) handleDeviceMetadataRequest(raw *event.RawEvent) {
	var req libraryclient.DeviceMetadataRequest
	_ = json.Unmarshal(raw.Data, &req)

	if req.ReaderID != "" && !strings.EqualFold(req.ReaderID, "all") && !strings.EqualFold(req.ReaderID, "*") && !strings.EqualFold(req.ReaderID, r.config.ReaderID) {
		r.logger.Debug().Str("requestedReaderId", req.ReaderID).Msg("metadata requested for different reader ID, ignoring")
		return
	}

	respData := libraryclient.DeviceMetadataResponseData{
		ReaderID:        r.config.ReaderID,
		Status:          "online",
		DeviceID:        r.config.DeviceID,
		DeviceType:      r.config.DeviceType,
		InitDescription: "Feig Mock ISO15693 Reader (v2.5.0-mock)",
		Description:     "Feig Mock ISO15693 Reader",
	}

	replyTarget := raw.Source
	if replyTarget == "" {
		replyTarget = raw.Channel
	}

	respEvt, err := wsbrokerclient.NewRawEvent(
		libraryclient.EventDeviceMetadataResponse,
		r.config.Name,
		replyTarget,
		respData,
	)
	if err != nil {
		r.logger.Error().Err(err).Msg("failed to construct device metadata response event")
		return
	}
	respEvt.RefID = raw.ID
	respEvt.Session = raw.Session

	if err := r.broker.Send(context.Background(), respEvt); err != nil {
		r.logger.Warn().Err(err).Msg("failed to send device metadata response")
	} else {
		r.logger.Info().Str("target", replyTarget).Msg("sent mock device metadata response")
	}
}

func (r *MockReader) handleWriteTagRequest(raw *event.RawEvent) {
	var req libraryclient.WriteTagRequest
	if err := json.Unmarshal(raw.Data, &req); err != nil {
		r.logger.Warn().Err(err).Msg("failed to unmarshal WriteTagRequest")
		return
	}

	if req.ReaderID != "" && !strings.EqualFold(req.ReaderID, r.config.ReaderID) {
		return
	}

	results := make([]libraryclient.TagWriteResult, 0, len(req.Tags))
	r.tagStoreMu.Lock()
	for _, item := range req.Tags {
		uid := item.UID
		if uid == "" {
			uid = GenerateUIDFromBarcode(item.ItemID)
		}
		if existing, ok := r.inMemoryMap[uid]; ok {
			existing.ItemID = item.ItemID
		} else {
			r.inMemoryMap[uid] = &libraryclient.TagReadData{
				ReaderID: r.config.ReaderID,
				UID:      uid,
				ItemID:   item.ItemID,
				AFI:      libraryclient.AFISecured,
			}
		}
		results = append(results, libraryclient.TagWriteResult{
			UID:    uid,
			ItemID: item.ItemID,
			Status: "ok",
		})
	}
	r.tagStoreMu.Unlock()

	respData := libraryclient.WriteTagResponseData{
		Status:   "ok",
		ReaderID: r.config.ReaderID,
		UID:      req.UID,
		ItemID:   req.ItemID,
		Results:  results,
	}

	replyTarget := raw.Source
	if replyTarget == "" {
		replyTarget = raw.Channel
	}

	respEvt, err := wsbrokerclient.NewRawEvent(
		libraryclient.EventWriteTagResponse,
		r.config.Name,
		replyTarget,
		respData,
	)
	if err != nil {
		r.logger.Error().Err(err).Msg("failed to construct WriteTagResponse event")
		return
	}
	respEvt.RefID = raw.ID
	respEvt.Session = raw.Session

	if err := r.broker.Send(context.Background(), respEvt); err != nil {
		r.logger.Warn().Err(err).Msg("failed to send WriteTagResponse")
	} else {
		r.logger.Info().Str("target", replyTarget).Msg("sent simulated write_tag response")
	}
}

func (r *MockReader) handleWriteAFIRequest(raw *event.RawEvent) {
	var req libraryclient.WriteAFIRequest
	if err := json.Unmarshal(raw.Data, &req); err != nil {
		r.logger.Warn().Err(err).Msg("failed to unmarshal WriteAFIRequest")
		return
	}

	if req.ReaderID != "" && !strings.EqualFold(req.ReaderID, r.config.ReaderID) {
		return
	}

	normalizedAFI := libraryclient.NormalizeAFI(req.AFI)
	var results []libraryclient.TagAFIResult

	r.tagStoreMu.Lock()
	if len(req.UIDs) > 0 {
		for _, uid := range req.UIDs {
			if existing, ok := r.inMemoryMap[uid]; ok {
				existing.AFI = normalizedAFI
			} else {
				r.inMemoryMap[uid] = &libraryclient.TagReadData{
					ReaderID: r.config.ReaderID,
					UID:      uid,
					AFI:      normalizedAFI,
				}
			}
			results = append(results, libraryclient.TagAFIResult{
				UID:    uid,
				AFI:    normalizedAFI,
				Status: "ok",
			})
		}
	} else if req.UID != "" {
		if existing, ok := r.inMemoryMap[req.UID]; ok {
			existing.AFI = normalizedAFI
		} else {
			r.inMemoryMap[req.UID] = &libraryclient.TagReadData{
				ReaderID: r.config.ReaderID,
				UID:      req.UID,
				AFI:      normalizedAFI,
			}
		}
		results = append(results, libraryclient.TagAFIResult{
			UID:    req.UID,
			AFI:    normalizedAFI,
			Status: "ok",
		})
	}
	r.tagStoreMu.Unlock()

	respData := libraryclient.WriteAFIResponseData{
		Status:   "ok",
		ReaderID: r.config.ReaderID,
		UID:      req.UID,
		AFI:      normalizedAFI,
		Results:  results,
	}

	replyTarget := raw.Source
	if replyTarget == "" {
		replyTarget = raw.Channel
	}

	respEvt, err := wsbrokerclient.NewRawEvent(
		libraryclient.EventWriteAFIResponse,
		r.config.Name,
		replyTarget,
		respData,
	)
	if err != nil {
		r.logger.Error().Err(err).Msg("failed to construct WriteAFIResponse event")
		return
	}
	respEvt.RefID = raw.ID
	respEvt.Session = raw.Session

	if err := r.broker.Send(context.Background(), respEvt); err != nil {
		r.logger.Warn().Err(err).Msg("failed to send WriteAFIResponse")
	} else {
		r.logger.Info().
			Str("target", replyTarget).
			Str("afi", normalizedAFI).
			Msg("sent simulated write_afi response")
	}
}
