package desk

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/je4/Library-RFID-Apps/pkg/rfidclient"
)

// SSEMessage represents an individual event payload streamed to HTTP SSE clients.
type SSEMessage struct {
	Event string
	Data  string
	ID    string
}

// Hub coordinates active SSE client connections and maintains in-memory scan history
// as well as active media currently resting on the reader antenna pad.
type Hub struct {
	mu               sync.RWMutex
	tags             []*TagItem
	activeTags       map[string]*TagItem
	maxTags          int
	activeWindow     time.Duration
	configuredReader string
	isilList         map[string]*ISILEntry
	clients          map[chan *SSEMessage]string
	closed           bool
}

// NewHub creates and initializes a new thread-safe Event Hub.
func NewHub(maxTags int, isilList ...map[string]*ISILEntry) *Hub {
	if maxTags <= 0 {
		maxTags = 100
	}
	var isils map[string]*ISILEntry
	if len(isilList) > 0 {
		isils = isilList[0]
	}
	return &Hub{
		tags:         make([]*TagItem, 0, maxTags),
		activeTags:   make(map[string]*TagItem),
		maxTags:      maxTags,
		activeWindow: 0,
		isilList:     isils,
		clients:      make(map[chan *SSEMessage]string),
	}
}

// SetISILList updates the ISIL database map and enriches existing tag records.
func (h *Hub) SetISILList(isilList map[string]*ISILEntry) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.isilList = isilList
	for _, t := range h.tags {
		t.EnrichISIL(isilList)
	}
	for _, t := range h.activeTags {
		t.EnrichISIL(isilList)
	}
}

// GetISILList returns the current ISIL database map.
func (h *Hub) GetISILList() map[string]*ISILEntry {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.isilList
}

// SetConfiguredReader configures the target reader for active media filtering.
func (h *Hub) SetConfiguredReader(reader string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.configuredReader = reader
}

// GetConfiguredReader returns the target reader filter.
func (h *Hub) GetConfiguredReader() string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.configuredReader
}

// SetActiveWindow sets the maximum time duration for media to be considered active on the reader.
func (h *Hub) SetActiveWindow(d time.Duration) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if d > 0 {
		h.activeWindow = d
	}
}

// GetActiveWindow returns the active media time window duration.
func (h *Hub) GetActiveWindow() time.Duration {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.activeWindow
}

// Start registers context cancellation for graceful shutdown.
func (h *Hub) Start(ctx context.Context) {
	go func() {
		<-ctx.Done()
		h.Close()
	}()
}

// Close stops the hub and terminates all active subscriptions.
func (h *Hub) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return
	}
	h.closed = true
	for ch := range h.clients {
		close(ch)
		delete(h.clients, ch)
	}
}

// Subscribe registers a new SSE listener channel, optionally filtered by broker channel.
func (h *Hub) Subscribe(channel ...string) chan *SSEMessage {
	ch := make(chan *SSEMessage, 64)
	h.mu.Lock()
	defer h.mu.Unlock()
	chFilter := ""
	if len(channel) > 0 {
		chFilter = strings.TrimSpace(channel[0])
	}
	if !h.closed {
		h.clients[ch] = chFilter
	} else {
		close(ch)
	}
	return ch
}

// Unsubscribe removes an SSE listener channel.
func (h *Hub) Unsubscribe(ch chan *SSEMessage) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.clients[ch]; ok {
		delete(h.clients, ch)
		close(ch)
	}
}

// AddTag stores a new tag scan, prepends it to the in-memory history,
// and updates the active media table.
func (h *Hub) AddTag(tag *rfidclient.TagReadEvent) *TagItem {
	item := NewTagItem(tag)

	h.mu.Lock()
	defer h.mu.Unlock()

	if h.isilList != nil {
		item.EnrichISIL(h.isilList)
	}

	// If tag already exists in active tags and has write status, preserve status on re-scan
	if existing, ok := h.activeTags[item.UID]; ok {
		if item.AFIWriteStatus == "" && existing.AFIWriteStatus != "" {
			item.AFIWriteStatus = existing.AFIWriteStatus
			item.AFIWriteError = existing.AFIWriteError
			item.AFIWriteTime = existing.AFIWriteTime
			item.AFIWritePending = existing.AFIWritePending
		}
	}

	// Prepend tag to front (newest first)
	h.tags = append([]*TagItem{item}, h.tags...)
	if len(h.tags) > h.maxTags {
		h.tags = h.tags[:h.maxTags]
	}

	// Always update active tags table (accumulate media on reader pad)
	h.activeTags[item.UID] = item

	return item
}

// GetActiveTags returns all media currently active on the configured reader antenna field
// within the active time window. Outdated media are pruned.
func (h *Hub) GetActiveTags(readerFilter ...string) []*TagItem {
	return h.GetActiveTagsForChannel("", readerFilter...)
}

// GetActiveTagsForChannel returns active media for a specific channel and optional reader filter.
func (h *Hub) GetActiveTagsForChannel(channel string, readerFilter ...string) []*TagItem {
	h.mu.Lock()
	defer h.mu.Unlock()

	now := time.Now()
	targetReader := h.configuredReader
	if len(readerFilter) > 0 && strings.TrimSpace(readerFilter[0]) != "" {
		targetReader = strings.TrimSpace(readerFilter[0])
	}
	targetChannel := strings.TrimSpace(channel)

	var active []*TagItem
	for uid, item := range h.activeTags {
		if h.activeWindow > 0 && now.Sub(item.ReadTime) > h.activeWindow {
			delete(h.activeTags, uid)
			continue
		}
		if targetChannel != "" && !MatchChannel(item.Channel, targetChannel) {
			continue
		}
		if targetReader != "" && !MatchReader(item.ClientID, targetReader) {
			continue
		}
		active = append(active, item)
	}

	// Fallback: If a specific reader filter returned 0 matches but active tags exist,
	// return active tags from current session matching the channel to prevent empty list on reader name mismatch.
	if len(active) == 0 && len(h.activeTags) > 0 && targetReader != "" {
		for _, item := range h.activeTags {
			if targetChannel != "" && !MatchChannel(item.Channel, targetChannel) {
				continue
			}
			if h.activeWindow == 0 || now.Sub(item.ReadTime) <= h.activeWindow {
				active = append(active, item)
			}
		}
	}

	// Sort active media deterministically: ItemID first, then UID
	sort.Slice(active, func(i, j int) bool {
		if active[i].ItemID != active[j].ItemID {
			return active[i].ItemID < active[j].ItemID
		}
		return active[i].UID < active[j].UID
	})

	return active
}

// ClearActive empties the active media currently lying on the reader.
func (h *Hub) ClearActive(reader ...string) {
	h.ClearActiveForChannel("", reader...)
}

// ClearActiveForChannel empties active media for a specific channel.
func (h *Hub) ClearActiveForChannel(channel string, reader ...string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	targetReader := h.configuredReader
	if len(reader) > 0 && strings.TrimSpace(reader[0]) != "" {
		targetReader = strings.TrimSpace(reader[0])
	}
	targetChannel := strings.TrimSpace(channel)

	if targetReader == "" || len(reader) == 0 || (len(reader) > 0 && MatchReader(reader[0], targetReader)) {
		if targetChannel == "" {
			h.activeTags = make(map[string]*TagItem)
		} else {
			for uid, item := range h.activeTags {
				if MatchChannel(item.Channel, targetChannel) {
					delete(h.activeTags, uid)
				}
			}
		}
	} else {
		for uid, item := range h.activeTags {
			if targetChannel != "" && !MatchChannel(item.Channel, targetChannel) {
				continue
			}
			if MatchReader(item.ClientID, targetReader) {
				delete(h.activeTags, uid)
			}
		}
	}
}

// MatchReader compares a tag's reader client ID against a target reader filter.
func MatchReader(tagReader, targetReader string) bool {
	tagReader = strings.TrimSpace(tagReader)
	targetReader = strings.TrimSpace(targetReader)

	if targetReader == "" || targetReader == "*" || strings.EqualFold(targetReader, "all") || strings.EqualFold(targetReader, "alle") {
		return true
	}
	if tagReader == "" {
		return true
	}
	if strings.EqualFold(tagReader, targetReader) {
		return true
	}

	normTag := normalizeReader(tagReader)
	normTarget := normalizeReader(targetReader)
	if normTag != "" && normTarget != "" && strings.EqualFold(normTag, normTarget) {
		return true
	}

	if strings.HasSuffix(strings.ToLower(tagReader), strings.ToLower(targetReader)) ||
		strings.HasSuffix(strings.ToLower(targetReader), strings.ToLower(tagReader)) {
		return true
	}

	return false
}

func normalizeReader(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	s = strings.TrimPrefix(s, "wsbroker:client:")
	s = strings.TrimPrefix(s, "wsbroker:")
	s = strings.TrimPrefix(s, "client:")
	return s
}

// UpdateActiveAFI updates the AFI security status for active tags (and recent scan entries).
func (h *Hub) UpdateActiveAFI(afi string, uids ...string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	uidMap := make(map[string]bool)
	for _, u := range uids {
		if strings.TrimSpace(u) != "" {
			uidMap[strings.TrimSpace(u)] = true
		}
	}

	// Update active tags
	for uid, item := range h.activeTags {
		if len(uidMap) == 0 || uidMap[uid] {
			item.SetAFIWriteSuccess(afi)
		}
	}

	// Also update recent history items for consistency
	for _, item := range h.tags {
		if len(uidMap) == 0 || uidMap[item.UID] {
			item.SetAFIWriteSuccess(afi)
		}
	}
}

// SetTagAFIResult records individual success or failure for a given tag UID.
func (h *Hub) SetTagAFIResult(uid string, afi string, success bool, errStr string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	trimmedUID := strings.TrimSpace(uid)
	if trimmedUID == "" {
		return
	}

	if item, ok := h.activeTags[trimmedUID]; ok {
		if success {
			item.SetAFIWriteSuccess(afi)
		} else {
			item.SetAFIWriteError(errStr)
		}
	}

	for _, item := range h.tags {
		if item.UID == trimmedUID {
			if success {
				item.SetAFIWriteSuccess(afi)
			} else {
				item.SetAFIWriteError(errStr)
			}
		}
	}
}

// SetTagAFIPending marks specified UIDs (or all active tags) as pending write operation.
func (h *Hub) SetTagAFIPending(uids ...string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	uidMap := make(map[string]bool)
	for _, u := range uids {
		if strings.TrimSpace(u) != "" {
			uidMap[strings.TrimSpace(u)] = true
		}
	}

	for uid, item := range h.activeTags {
		if len(uidMap) == 0 || uidMap[uid] {
			item.SetAFIWritePending()
		}
	}

	for _, item := range h.tags {
		if len(uidMap) == 0 || uidMap[item.UID] {
			item.SetAFIWritePending()
		}
	}
}

// ClearActiveAFIWriteStatus clears the write status indicators on active tags.
func (h *Hub) ClearActiveAFIWriteStatus(uids ...string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	uidMap := make(map[string]bool)
	for _, u := range uids {
		if strings.TrimSpace(u) != "" {
			uidMap[strings.TrimSpace(u)] = true
		}
	}

	for uid, item := range h.activeTags {
		if len(uidMap) == 0 || uidMap[uid] {
			item.ClearAFIWriteStatus()
		}
	}

	for _, item := range h.tags {
		if len(uidMap) == 0 || uidMap[item.UID] {
			item.ClearAFIWriteStatus()
		}
	}
}

// MatchChannel compares a tag's channel against a target channel filter.
func MatchChannel(tagChannel, targetChannel string) bool {
	tagChannel = strings.TrimSpace(tagChannel)
	targetChannel = strings.TrimSpace(targetChannel)

	if targetChannel == "" || targetChannel == "*" || strings.EqualFold(targetChannel, "all") || strings.EqualFold(targetChannel, "alle") {
		return true
	}
	if tagChannel == "" || tagChannel == "*" || strings.EqualFold(tagChannel, "all") || strings.EqualFold(tagChannel, "alle") {
		return true
	}
	return strings.EqualFold(tagChannel, targetChannel)
}

// Broadcast sends an SSE message to connected clients.
// If channel is specified, only clients subscribed to that channel (or to all channels "") receive it.
func (h *Hub) Broadcast(msg *SSEMessage, channel ...string) {
	if msg == nil {
		return
	}
	targetChannel := ""
	if len(channel) > 0 {
		targetChannel = strings.TrimSpace(channel[0])
	}

	h.mu.RLock()
	defer h.mu.RUnlock()
	for clientChan, clientFilter := range h.clients {
		if targetChannel != "" && clientFilter != "" && !MatchChannel(targetChannel, clientFilter) {
			continue
		}
		select {
		case clientChan <- msg:
		default:
			// Non-blocking drop if channel buffer full
		}
	}
}

// Clear empties the in-memory scan history and notifies clients.
func (h *Hub) Clear() {
	h.ClearForChannel("")
}

// ClearForChannel empties scan history for a specific channel and notifies clients.
func (h *Hub) ClearForChannel(channel string) {
	h.mu.Lock()
	targetChannel := strings.TrimSpace(channel)
	if targetChannel == "" {
		h.tags = make([]*TagItem, 0, h.maxTags)
		h.activeTags = make(map[string]*TagItem)
	} else {
		newTags := make([]*TagItem, 0, cap(h.tags))
		for _, item := range h.tags {
			if !MatchChannel(item.Channel, targetChannel) {
				newTags = append(newTags, item)
			}
		}
		h.tags = newTags
		for uid, item := range h.activeTags {
			if MatchChannel(item.Channel, targetChannel) {
				delete(h.activeTags, uid)
			}
		}
	}
	h.mu.Unlock()

	h.Broadcast(&SSEMessage{
		Event: "tags_cleared",
		Data:  "cleared",
	}, targetChannel)
}

// GetTags returns a copy of all retained tag scans, optionally filtered by channel.
func (h *Hub) GetTags(channel ...string) []*TagItem {
	h.mu.RLock()
	defer h.mu.RUnlock()

	var ch string
	if len(channel) > 0 {
		ch = strings.TrimSpace(channel[0])
	}

	res := make([]*TagItem, 0, len(h.tags))
	for _, item := range h.tags {
		if ch == "" || MatchChannel(item.Channel, ch) {
			res = append(res, item)
		}
	}
	return res
}

// Count returns the number of retained tags, optionally filtered by channel.
func (h *Hub) Count(channel ...string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	var ch string
	if len(channel) > 0 {
		ch = strings.TrimSpace(channel[0])
	}
	if ch == "" {
		return len(h.tags)
	}

	cnt := 0
	for _, item := range h.tags {
		if MatchChannel(item.Channel, ch) {
			cnt++
		}
	}
	return cnt
}
