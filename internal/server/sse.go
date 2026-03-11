package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"aurago/internal/security"
)

// SSEBroadcaster manages Server-Sent Events connections and broadcasts messages.
type SSEBroadcaster struct {
	mu      sync.RWMutex
	clients map[chan string]struct{}
}

// NewSSEBroadcaster creates a new broadcaster instance.
func NewSSEBroadcaster() *SSEBroadcaster {
	return &SSEBroadcaster{
		clients: make(map[chan string]struct{}),
	}
}

// Send broadcasts a JSON event string to all connected SSE clients (non-blocking).
func (b *SSEBroadcaster) Send(event, detail string) {
	detail = security.Scrub(detail)
	payload, _ := json.Marshal(struct {
		Event  string `json:"event"`
		Detail string `json:"detail"`
	}{event, detail})
	msg := string(payload)
	b.mu.RLock()
	defer b.mu.RUnlock()
	for ch := range b.clients {
		select {
		case ch <- msg:
		default:
			// Client buffer full, skip to avoid blocking the supervisor loop
		}
	}
}

// SendJSON broadcasts a raw JSON string to all connected SSE clients (non-blocking).
func (b *SSEBroadcaster) SendJSON(jsonMsg string) {
	jsonMsg = security.Scrub(jsonMsg)
	b.mu.RLock()
	defer b.mu.RUnlock()
	for ch := range b.clients {
		select {
		case ch <- jsonMsg:
		default:
		}
	}
}

// ClientCount returns the number of currently connected SSE clients.
func (b *SSEBroadcaster) ClientCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.clients)
}

// subscribe registers a new client channel.
func (b *SSEBroadcaster) subscribe() chan string {
	ch := make(chan string, 16)
	b.mu.Lock()
	b.clients[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

// unsubscribe removes a client channel and closes it.
func (b *SSEBroadcaster) unsubscribe(ch chan string) {
	b.mu.Lock()
	delete(b.clients, ch)
	b.mu.Unlock()
	close(ch)
}

// ServeHTTP implements the /events SSE endpoint handler.
func (b *SSEBroadcaster) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	ch := b.subscribe()
	defer b.unsubscribe(ch)

	// Flush headers immediately so the browser's EventSource fires onopen
	// without waiting for the first real message.
	fmt.Fprintf(w, ": ping\n\n")
	flusher.Flush()

	ctx := r.Context()
	for {
		select {
		case msg := <-ch:
			fmt.Fprintf(w, "data: %s\n\n", msg)
			flusher.Flush()
		case <-ctx.Done():
			return
		}
	}
}
