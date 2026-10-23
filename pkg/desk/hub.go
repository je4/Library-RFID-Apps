package desk

import (
	"context"
	"sync"

	"github.com/je4/Library-RFID-Apps/pkg/rfidclient"
)

// SSEMessage represents an individual event payload streamed to HTTP SSE clients.
type SSEMessage struct {
	Event string
	Data  string
	ID    string
}

// Hub coordinates active SSE client connections and maintains in-memory scan history.
type Hub struct {
	mu      sync.RWMutex
	tags    []*TagItem
	maxTags int
	clients map[chan *SSEMessage]struct{}
	closed  bool
}

// NewHub creates and initializes a new thread-safe Event Hub.
func NewHub(maxTags int) *Hub {
	if maxTags <= 0 {
		maxTags = 100
	}
	return &Hub{
		tags:    make([]*TagItem, 0, maxTags),
		maxTags: maxTags,
		clients: make(map[chan *SSEMessage]struct{}),
	}
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

// AddTag stores a new tag scan and prepends it to the in-memory history.
func (h *Hub) AddTag(tag *rfidclient.TagReadEvent) *TagItem {
	item := NewTagItem(tag)

	h.mu.Lock()
	defer h.mu.Unlock()

	// Prepend tag to front (newest first)
	h.tags = append([]*TagItem{item}, h.tags...)
	if len(h.tags) > h.maxTags {
		h.tags = h.tags[:h.maxTags]
	}

	return item
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
