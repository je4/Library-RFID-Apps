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
	clients          map[chan *SSEMessage]struct{}
	closed           bool
}

// NewHub creates and initializes a new thread-safe Event Hub.
func NewHub(maxTags int) *Hub {
	if maxTags <= 0 {
		maxTags = 100
	}
	return &Hub{
		tags:         make([]*TagItem, 0, maxTags),
		activeTags:   make(map[string]*TagItem),
		maxTags:      maxTags,
		activeWindow: 0,
		clients:      make(map[chan *SSEMessage]struct{}),
	}
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

// Subscribe registers a new SSE listener channel.
func (h *Hub) Subscribe() chan *SSEMessage {
	ch := make(chan *SSEMessage, 64)
	h.mu.Lock()
	defer h.mu.Unlock()
	if !h.closed {
		h.clients[ch] = struct{}{}
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
	h.mu.Lock()
	defer h.mu.Unlock()

	now := time.Now()
	targetReader := h.configuredReader
	if len(readerFilter) > 0 && strings.TrimSpace(readerFilter[0]) != "" {
		targetReader = strings.TrimSpace(readerFilter[0])
	}

	var active []*TagItem
	for uid, item := range h.activeTags {
		if h.activeWindow > 0 && now.Sub(item.ReadTime) > h.activeWindow {
			delete(h.activeTags, uid)
			continue
		}
		if targetReader != "" && !MatchReader(item.ClientID, targetReader) {
			continue
		}
		active = append(active, item)
	}

	// Fallback: If a specific reader filter returned 0 matches but active tags exist,
	// return active tags from current session to prevent empty list on reader name mismatch.
	if len(active) == 0 && len(h.activeTags) > 0 && targetReader != "" {
		for _, item := range h.activeTags {
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
	h.mu.Lock()
	defer h.mu.Unlock()

	targetReader := h.configuredReader
	if len(reader) > 0 && strings.TrimSpace(reader[0]) != "" {
		targetReader = strings.TrimSpace(reader[0])
	}

	if targetReader == "" || len(reader) == 0 || MatchReader(reader[0], targetReader) {
		h.activeTags = make(map[string]*TagItem)
	} else {
		for uid, item := range h.activeTags {
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
			item.SetAFI(afi)
		}
	}

	// Also update recent history items for consistency
	for _, item := range h.tags {
		if len(uidMap) == 0 || uidMap[item.UID] {
			item.SetAFI(afi)
		}
	}
}

// Broadcast sends an SSE message to all connected clients.
func (h *Hub) Broadcast(msg *SSEMessage) {
	if msg == nil {
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for ch := range h.clients {
		select {
		case ch <- msg:
		default:
			// Non-blocking drop if channel buffer full
		}
	}
}

// Clear empties the in-memory scan history and notifies clients.
func (h *Hub) Clear() {
	h.mu.Lock()
	h.tags = make([]*TagItem, 0, h.maxTags)
	h.activeTags = make(map[string]*TagItem)
	h.mu.Unlock()

	h.Broadcast(&SSEMessage{
		Event: "tags_cleared",
		Data:  "cleared",
	})
}

// GetTags returns a copy of all retained tag scans.
func (h *Hub) GetTags() []*TagItem {
	h.mu.RLock()
	defer h.mu.RUnlock()

	res := make([]*TagItem, len(h.tags))
	copy(res, h.tags)
	return res
}

// Count returns the number of retained tags.
func (h *Hub) Count() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.tags)
}
