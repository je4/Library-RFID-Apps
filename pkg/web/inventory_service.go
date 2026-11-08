package web

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"emperror.dev/errors"
	"github.com/je4/Library-RFID-Apps/pkg/desk"
	"github.com/je4/Library-RFID-Apps/pkg/inventory"
	"github.com/je4/Library-RFID-Apps/pkg/libraryclient"
	"github.com/je4/utils/v2/pkg/zLogger"
)

// InventoryAudioCue indicates the acoustic feedback tone to play in the web client.
type InventoryAudioCue string

const (
	AudioCueNone      InventoryAudioCue = "none"
	AudioCueSuccess   InventoryAudioCue = "success"
	AudioCueDuplicate InventoryAudioCue = "duplicate"
	AudioCueError     InventoryAudioCue = "error"
)

// InventoryTagEventPayload defines the SSE payload when a tag is processed in inventory mode.
type InventoryTagEventPayload struct {
	Item    *desk.InventoryItem `json:"item"`
	Stats   desk.InventoryStats `json:"stats"`
	Audio   InventoryAudioCue   `json:"audio"`
	HTMLRow string              `json:"htmlRow,omitempty"`
}

// InventoryService orchestrates inventory scanning sessions, de-duplication, KPI statistics,
// and asynchronous webservice dispatching.
type InventoryService struct {
	mu         sync.RWMutex
	cfg        desk.InventoryConfig
	hub        *desk.Hub
	dispatcher inventory.Dispatcher
	logger     zLogger.ZLogger
	sessionID  string
	marker     string
	active     bool
	paused     bool
	startTime  time.Time
	items      []*desk.InventoryItem
	uidSet     map[string]bool
	stats      desk.InventoryStats
	onTagEvent func(payload InventoryTagEventPayload, channel string)
}

// NewInventoryService creates a new InventoryService instance.
func NewInventoryService(
	cfg desk.InventoryConfig,
	hub *desk.Hub,
	dispatcher inventory.Dispatcher,
	logger zLogger.ZLogger,
) *InventoryService {
	if dispatcher == nil && cfg.Webservice.TargetURL != "" {
		dispatcher = inventory.NewWebserviceDispatcher(cfg.Webservice, logger)
	}

	svc := &InventoryService{
		cfg:        cfg,
		hub:        hub,
		dispatcher: dispatcher,
		logger:     logger,
		active:     false,
		paused:     false,
		uidSet:     make(map[string]bool),
		items:      make([]*desk.InventoryItem, 0),
	}
	return svc
}

// SetOnTagEvent registers an optional callback for SSE broadcasting.
func (s *InventoryService) SetOnTagEvent(cb func(payload InventoryTagEventPayload, channel string)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onTagEvent = cb
}

// StartSession initiates a new inventory session with the specified location/shelf marker.
func (s *InventoryService) StartSession(marker string) desk.InventorySession {
	s.mu.Lock()
	defer s.mu.Unlock()

	marker = strings.TrimSpace(marker)
	if marker == "" {
		return s.getSessionLocked()
	}

	s.sessionID = fmt.Sprintf("inv-%s", time.Now().UTC().Format("20060102-150405"))
	s.marker = marker
	s.active = true
	s.paused = false
	s.startTime = time.Now().UTC()
	s.items = make([]*desk.InventoryItem, 0)
	s.uidSet = make(map[string]bool)
	s.stats = desk.InventoryStats{}

	return s.getSessionLocked()
}

// SetMarker updates the current shelf/location marker during an active session.
func (s *InventoryService) SetMarker(marker string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.marker = strings.TrimSpace(marker)
}

// PauseSession temporarily ignores incoming RFID tag scans.
func (s *InventoryService) PauseSession() desk.InventorySession {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active {
		s.paused = true
	}
	return s.getSessionLocked()
}

// ResumeSession re-activates tag scan processing.
func (s *InventoryService) ResumeSession() desk.InventorySession {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active {
		s.paused = false
	}
	return s.getSessionLocked()
}

// ResetSession clears all session counters and recorded items, stopping active processing.
func (s *InventoryService) ResetSession() desk.InventorySession {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.sessionID = ""
	s.active = false
	s.paused = false
	s.items = make([]*desk.InventoryItem, 0)
	s.uidSet = make(map[string]bool)
	s.stats = desk.InventoryStats{}

	return s.getSessionLocked()
}

// GetSession returns a snapshot of the current session state.
func (s *InventoryService) GetSession() desk.InventorySession {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.getSessionLocked()
}

func (s *InventoryService) getSessionLocked() desk.InventorySession {
	itemsCopy := make([]*desk.InventoryItem, len(s.items))
	copy(itemsCopy, s.items)

	return desk.InventorySession{
		SessionID: s.sessionID,
		Marker:    s.marker,
		Active:    s.active,
		Paused:    s.paused,
		StartTime: s.startTime,
		Stats:     s.stats,
		Items:     itemsCopy,
	}
}

// ProcessTagRead processes an incoming RFID tag read event during an active inventory session.
func (s *InventoryService) ProcessTagRead(tag *libraryclient.TagReadEvent) (*desk.InventoryItem, InventoryAudioCue, bool) {
	if tag == nil {
		return nil, AudioCueError, false
	}

	s.mu.Lock()
	if !s.active || s.paused {
		s.mu.Unlock()
		return nil, AudioCueError, false
	}

	tagItem := desk.NewTagItem(tag)
	if s.hub != nil && s.hub.GetISILList() != nil {
		tagItem.EnrichISIL(s.hub.GetISILList())
	}

	s.stats.TotalScans++
	uid := tagItem.UID

	var audioCue InventoryAudioCue
	var invItem *desk.InventoryItem

	if s.uidSet[uid] {
		s.stats.DuplicateTags++
		audioCue = AudioCueDuplicate

		invItem = &desk.InventoryItem{
			TagItem:       tagItem,
			Marker:        s.marker,
			SessionID:     s.sessionID,
			IsDuplicate:   true,
			DispatchState: "duplicate",
			DispatchError: "Bereits in dieser Sitzung erfasst (Dublette)",
			DispatchedAt:  time.Now().UTC(),
		}
	} else {
		s.uidSet[uid] = true

		isError := tagItem.CRCError() || tagItem.IsEmpty()
		if isError {
			s.stats.ErrorTags++
			audioCue = AudioCueError
		} else {
			s.stats.UniqueItems++
			audioCue = AudioCueSuccess
		}

		initialDispatchState := "pending"
		if s.dispatcher == nil {
			initialDispatchState = "local"
		}

		invItem = &desk.InventoryItem{
			TagItem:       tagItem,
			Marker:        s.marker,
			SessionID:     s.sessionID,
			IsDuplicate:   false,
			DispatchState: initialDispatchState,
		}

		// Insert at the beginning of items list (most recent first)
		s.items = append([]*desk.InventoryItem{invItem}, s.items...)

		if s.dispatcher != nil {
			// Trigger asynchronous webservice dispatch for new valid/empty items
			go s.dispatchItem(invItem)
		}
	}

	statsCopy := s.stats
	cb := s.onTagEvent
	s.mu.Unlock()

	if cb != nil {
		cb(InventoryTagEventPayload{
			Item:  invItem,
			Stats: statsCopy,
			Audio: audioCue,
		}, tag.Channel)
	}

	return invItem, audioCue, true
}

// dispatchItem sends the inventory item to the external webservice.
func (s *InventoryService) dispatchItem(item *desk.InventoryItem) {
	if item == nil {
		return
	}
	if s.dispatcher == nil {
		s.mu.Lock()
		item.DispatchState = "local"
		s.mu.Unlock()
		return
	}

	payload := inventory.ScanPayload{
		ClientID:   s.cfg.Webservice.ClientID,
		SessionID:  item.SessionID,
		Marker:     item.Marker,
		UID:        item.UID,
		ItemID:     item.ItemID,
		Raw:        item.Raw,
		Country:    item.Country,
		ISIL:       item.ISIL,
		Parts:      item.Parts,
		PartNumber: item.PartNo,
		UsageType:  item.UsageType,
		IsCRCValid: item.CRCOk,
		AFI:        item.AFI,
		Timestamp:  time.Now().UnixMilli(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := s.dispatcher.Dispatch(ctx, payload)

	s.mu.Lock()
	item.DispatchedAt = time.Now().UTC()
	if err != nil || (resp != nil && !resp.Success) {
		item.DispatchState = "error"
		if resp != nil && resp.Error != "" {
			item.DispatchError = resp.Error
			item.HTTPStatus = resp.StatusCode
		} else if err != nil {
			item.DispatchError = err.Error()
		}

		if s.logger != nil {
			s.logger.Error().
				Str("uid", item.UID).
				Str("itemId", item.ItemID).
				Str("marker", item.Marker).
				Int("httpStatus", item.HTTPStatus).
				Str("error", item.DispatchError).
				Msg("inventory scan webservice dispatch failed")
		}
	} else if resp != nil && resp.Success {
		item.DispatchState = "success"
		item.HTTPStatus = resp.StatusCode
		item.DispatchError = ""

		if s.logger != nil {
			s.logger.Info().
				Str("uid", item.UID).
				Str("itemId", item.ItemID).
				Str("marker", item.Marker).
				Int("httpStatus", resp.StatusCode).
				Msg("inventory scan webservice dispatch succeeded")
		}
	}

	statsCopy := s.stats
	cb := s.onTagEvent
	s.mu.Unlock()

	if cb != nil {
		channel := ""
		if item.TagItem != nil {
			channel = item.TagItem.Channel
		}
		cb(InventoryTagEventPayload{
			Item:  item,
			Stats: statsCopy,
			Audio: AudioCueNone,
		}, channel)
	}
}

// RetryDispatch re-sends a failed scan item to the external webservice.
func (s *InventoryService) RetryDispatch(uidOrItemID string) (*desk.InventoryItem, error) {
	s.mu.Lock()
	var targetItem *desk.InventoryItem
	for _, it := range s.items {
		if it.UID == uidOrItemID || (it.ItemID != "" && it.ItemID == uidOrItemID) {
			targetItem = it
			break
		}
	}

	if targetItem == nil {
		s.mu.Unlock()
		return nil, errors.Errorf("item with UID or ItemID '%s' not found in current session", uidOrItemID)
	}

	targetItem.DispatchState = "pending"
	targetItem.DispatchError = ""

	statsCopy := s.stats
	cb := s.onTagEvent
	s.mu.Unlock()

	if cb != nil {
		channel := ""
		if targetItem.TagItem != nil {
			channel = targetItem.TagItem.Channel
		}
		cb(InventoryTagEventPayload{
			Item:  targetItem,
			Stats: statsCopy,
			Audio: AudioCueNone,
		}, channel)
	}

	go s.dispatchItem(targetItem)
	return targetItem, nil
}
