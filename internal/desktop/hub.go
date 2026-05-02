package desktop

import (
	"fmt"
	"sync"
)

// Hub fans desktop events out to active browser clients.
type Hub struct {
	mu      sync.Mutex
	max     int
	clients map[chan Event]struct{}
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
	if len(h.clients) >= h.max {
		return nil, nil, fmt.Errorf("desktop websocket client limit reached")
	}
	ch := make(chan Event, 16)
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

// Broadcast sends an event to all clients without blocking callers.
func (h *Hub) Broadcast(event Event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.clients {
		select {
		case ch <- event:
		default:
		}
	}
}

// ClientCount returns the number of active clients.
func (h *Hub) ClientCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.clients)
}
