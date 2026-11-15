package web

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/je4/Library-RFID-Apps/pkg/desk"
	"github.com/je4/Library-RFID-Apps/pkg/libraryclient"
	"github.com/je4/utils/v2/pkg/zLogger"
)

// GateAlarmItem represents an individual item that triggered a gate alarm due to secured AFI.
type GateAlarmItem struct {
	ID         string    `json:"id"`
	Barcode    string    `json:"barcode"`
	UID        string    `json:"uid"`
	Gate       string    `json:"gate"`
	Channel    string    `json:"channel"`
	ReaderID   string    `json:"readerId"`
	AFI        string    `json:"afi"`
	ISIL       string    `json:"isil"`
	Time       time.Time `json:"time"`
	TimeString string    `json:"timeString"`
}

// GateAlarmEventPayload represents the SSE payload broadcast to web clients when a gate alarm occurs.
type GateAlarmEventPayload struct {
	IncidentID string          `json:"incidentId"`
	Gate       string          `json:"gate"`
	Channel    string          `json:"channel"`
	ReaderID   string          `json:"readerId"`
	Time       time.Time       `json:"time"`
	TimeString string          `json:"timeString"`
	Item       GateAlarmItem   `json:"item"`
	Items      []GateAlarmItem `json:"items"`
	TotalCount int             `json:"totalCount"`
}

// GateAlarmService manages gate monitoring, AFI alarm detection, and in-memory alarm history.
type GateAlarmService struct {
	mu           sync.RWMutex
	gateChannels []string
	alarms       []GateAlarmItem
	maxAlarms    int
	logger       zLogger.ZLogger
}

// NewGateAlarmService creates a new GateAlarmService instance.
func NewGateAlarmService(gateChannels []string, maxAlarms int, logger zLogger.ZLogger) *GateAlarmService {
	if maxAlarms <= 0 {
		maxAlarms = 100
	}
	// Clean and normalize channel names
	var cleanedChannels []string
	for _, ch := range gateChannels {
		trimmed := strings.TrimSpace(ch)
		if trimmed != "" {
			cleanedChannels = append(cleanedChannels, trimmed)
		}
	}
	return &GateAlarmService{
		gateChannels: cleanedChannels,
		alarms:       make([]GateAlarmItem, 0, maxAlarms),
		maxAlarms:    maxAlarms,
		logger:       logger,
	}
}

// IsGateChannel returns true if the specified channel is configured as a monitored gate channel.
func (s *GateAlarmService) IsGateChannel(channel string) bool {
	if s == nil {
		return false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	target := strings.TrimSpace(channel)
	if target == "" {
		return false
	}
	for _, ch := range s.gateChannels {
		if desk.MatchChannel(target, ch) {
			return true
		}
	}
	return false
}

// GetGateChannels returns the configured list of gate channels.
func (s *GateAlarmService) GetGateChannels() []string {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	res := make([]string, len(s.gateChannels))
	copy(res, s.gateChannels)
	return res
}

// ProcessTagRead checks whether an incoming TagReadEvent represents a gate alarm.
// An alarm is triggered if:
// 1. The event arrived on a configured gate channel.
// 2. The tag's AFI byte is secured (AFI "07" / closed).
// Returns the created alarm item and event payload if an alarm was triggered.
func (s *GateAlarmService) ProcessTagRead(tag *libraryclient.TagReadEvent) (*GateAlarmItem, *GateAlarmEventPayload, bool) {
	if s == nil || tag == nil {
		return nil, nil, false
	}

	if !s.IsGateChannel(tag.Channel) {
		return nil, nil, false
	}

	// Gate alarm triggers when AFI is secured ("07" / closed)
	if !libraryclient.IsAFISecured(tag.Data.AFI) {
		return nil, nil, false
	}

	scanTime := time.Now()
	if tag.Data.Time != nil && !tag.Data.Time.IsZero() {
		scanTime = *tag.Data.Time
	}

	barcode := strings.TrimSpace(tag.Data.ItemID)
	if barcode == "" {
		barcode = strings.TrimSpace(tag.Data.UID)
	}

	gateName := strings.TrimSpace(tag.Data.ReaderID)
	if gateName == "" {
		gateName = strings.TrimSpace(tag.Channel)
	}

	alarmID := fmt.Sprintf("alarm-%s-%d", uuid.New().String()[:8], scanTime.UnixMilli())

	item := GateAlarmItem{
		ID:         alarmID,
		Barcode:    barcode,
		UID:        strings.TrimSpace(tag.Data.UID),
		Gate:       gateName,
		Channel:    strings.TrimSpace(tag.Channel),
		ReaderID:   strings.TrimSpace(tag.Data.ReaderID),
		AFI:        libraryclient.NormalizeAFI(tag.Data.AFI),
		ISIL:       strings.TrimSpace(tag.Data.ISIL),
		Time:       scanTime,
		TimeString: scanTime.Format("15:04:05"),
	}

	s.mu.Lock()
	// Prepend newest alarm first
	s.alarms = append([]GateAlarmItem{item}, s.alarms...)
	if len(s.alarms) > s.maxAlarms {
		s.alarms = s.alarms[:s.maxAlarms]
	}
	allAlarms := make([]GateAlarmItem, len(s.alarms))
	copy(allAlarms, s.alarms)
	totalCount := len(s.alarms)
	s.mu.Unlock()

	if s.logger != nil {
		s.logger.Warn().
			Str("alarmId", item.ID).
			Str("barcode", item.Barcode).
			Str("uid", item.UID).
			Str("gate", item.Gate).
			Str("channel", item.Channel).
			Str("afi", item.AFI).
			Str("time", item.TimeString).
			Msg("GATE ALARM TRIGGERED: Secured AFI transponder detected at gate")
	}

	payload := &GateAlarmEventPayload{
		IncidentID: alarmID,
		Gate:       gateName,
		Channel:    item.Channel,
		ReaderID:   item.ReaderID,
		Time:       scanTime,
		TimeString: scanTime.Format("15:04:05"),
		Item:       item,
		Items:      allAlarms,
		TotalCount: totalCount,
	}

	return &item, payload, true
}

// GetAlarms returns a slice of all stored gate alarms (newest first).
func (s *GateAlarmService) GetAlarms() []GateAlarmItem {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	res := make([]GateAlarmItem, len(s.alarms))
	copy(res, s.alarms)
	return res
}

// ClearAlarms clears all stored gate alarms.
func (s *GateAlarmService) ClearAlarms() {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.alarms = s.alarms[:0]
}

// AcknowledgeAlarm removes a specific alarm item by ID.
func (s *GateAlarmService) AcknowledgeAlarm(id string) bool {
	if s == nil || id == "" {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, alarm := range s.alarms {
		if alarm.ID == id {
			s.alarms = append(s.alarms[:i], s.alarms[i+1:]...)
			return true
		}
	}
	return false
}
