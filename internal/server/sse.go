package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"aurago/internal/security"
)

// SSEEventType is a strongly-typed SSE event identifier for typed broadcast messages.
// Typed events use the format {"type":"<event>","payload":{...}} and are distinct
// from the legacy {"event":"...","detail":"..."} format used by Send/SendJSON.
type SSEEventType string

const (
	EventSystemMetrics     SSEEventType = "system_metrics"
	EventMemoryUpdate      SSEEventType = "memory_update"
	EventBudgetUpdate      SSEEventType = "budget_update"
	EventAgentStatus       SSEEventType = "agent_status"
	EventMissionUpdate     SSEEventType = "mission_update"
	EventContainerUpdate   SSEEventType = "container_update"
	EventPersonalityUpdate SSEEventType = "personality_update"
	EventTsnetStatus       SSEEventType = "tsnet_status"
	EventLogLine           SSEEventType = "log_line"
	EventToast             SSEEventType = "toast"
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
	b.broadcast(string(payload))
}

// SendJSON broadcasts a raw JSON string to all connected SSE clients (non-blocking).
func (b *SSEBroadcaster) SendJSON(jsonMsg string) {
	jsonMsg = security.Scrub(jsonMsg)
	b.broadcast(jsonMsg)
}

func (b *SSEBroadcaster) broadcast(msg string) {
	b.mu.RLock()
	staleClients := make([]chan string, 0)
	for ch := range b.clients {
		select {
		case ch <- msg:
		default:
			staleClients = append(staleClients, ch)
		}
	}
	b.mu.RUnlock()

	for _, ch := range staleClients {
		b.unsubscribe(ch)
	}
}

// BroadcastType sends a typed SSE event with a structured payload to all clients.
// Messages are formatted as {"type":"<eventType>","payload":<payload>}.
func (b *SSEBroadcaster) BroadcastType(eventType SSEEventType, payload any) {
	msg, err := json.Marshal(struct {
		Type    SSEEventType `json:"type"`
		Payload any          `json:"payload"`
	}{eventType, payload})
	if err != nil {
		return
	}
	b.SendJSON(string(msg))
}

// ClientCount returns the number of currently connected SSE clients.
func (b *SSEBroadcaster) ClientCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.clients)
}

// subscribe registers a new client channel.
func (b *SSEBroadcaster) subscribe() chan string {
	ch := make(chan string, 64)
	b.mu.Lock()
	b.clients[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

// unsubscribe removes a client channel and closes it.
func (b *SSEBroadcaster) unsubscribe(ch chan string) {
	b.mu.Lock()
	if _, ok := b.clients[ch]; !ok {
		b.mu.Unlock()
		return
	}
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
		jsonError(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	ch := b.subscribe()
	defer b.unsubscribe(ch)

	// Flush headers immediately so the browser's EventSource fires onopen
	// without waiting for the first real message.
	fmt.Fprintf(w, ": ping\n\n")
	flusher.Flush()

	ctx := r.Context()
	heartbeat := time.NewTicker(20 * time.Second)
	defer heartbeat.Stop()
	for {
		select {
		case msg := <-ch:
			fmt.Fprintf(w, "data: %s\n\n", msg)
			flusher.Flush()
		case <-heartbeat.C:
			fmt.Fprintf(w, ": ping\n\n")
			flusher.Flush()
		case <-ctx.Done():
			return
		}
	}
}
