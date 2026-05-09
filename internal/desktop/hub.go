package desktop

import (
	"fmt"
	"log/slog"
	"sync"
)

// Hub fans desktop events out to active browser clients.
type Hub struct {
	mu      sync.Mutex
	max     int
	clients map[chan Event]struct{}
	closed  bool
}

// NewHub creates an event hub with a client cap.
func NewHub(maxClients int) *Hub {
	if maxClients <= 0 {
		maxClients = 8
	}
	return &Hub{
		max:     maxClients,
		clients: make(map[chan Event]struct{}),
	}
}

// Subscribe registers one client and returns its event channel plus a cleanup function.
func (h *Hub) Subscribe() (<-chan Event, func(), error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return nil, nil, fmt.Errorf("desktop hub is closed")
	}
	if len(h.clients) >= h.max {
		return nil, nil, fmt.Errorf("desktop websocket client limit reached")
	}
	ch := make(chan Event, 64)
	h.clients[ch] = struct{}{}
	cancel := func() {
		h.mu.Lock()
		if _, ok := h.clients[ch]; ok {
			delete(h.clients, ch)
			close(ch)
		}
		h.mu.Unlock()
	}
	return ch, cancel, nil
}

// Close shuts down the hub and closes all client channels so subscribers
// unblock immediately instead of leaking goroutines.
func (h *Hub) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.closed = true
	for ch := range h.clients {
		close(ch)
		delete(h.clients, ch)
	}
}

// Broadcast sends an event to all clients without blocking callers.
// Returns the number of clients that missed the event due to a full channel.
func (h *Hub) Broadcast(event Event) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	var dropped int
	for ch := range h.clients {
		select {
		case ch <- event:
		default:
			dropped++
			slog.Warn("desktop event dropped, client channel full", "event_type", event.Type)
		}
	}
	return dropped
}

// ClientCount returns the number of active clients.
func (h *Hub) ClientCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.clients)
}
