package chibisensei

import "sync"

// Event is one server-sent event of a project: Name is job, rows or checks.
type Event struct {
	Name string
	Data any
}

// Hub fans project events out to SSE subscribers.
type Hub struct {
	mu   sync.Mutex
	subs map[string]map[chan Event]struct{}
}

// NewHub returns an empty hub.
func NewHub() *Hub { return &Hub{subs: map[string]map[chan Event]struct{}{}} }

// Subscribe returns a channel buffered 32 and an idempotent cancel func that also closes the channel.
func (h *Hub) Subscribe(projectID string) (<-chan Event, func()) {
	ch := make(chan Event, 32)
	h.mu.Lock()
	if h.subs[projectID] == nil {
		h.subs[projectID] = map[chan Event]struct{}{}
	}
	h.subs[projectID][ch] = struct{}{}
	h.mu.Unlock()
	var once sync.Once
	cancel := func() {
		once.Do(func() {
			h.mu.Lock()
			defer h.mu.Unlock()
			if set := h.subs[projectID]; set != nil {
				delete(set, ch)
				if len(set) == 0 {
					delete(h.subs, projectID)
				}
			}
			close(ch)
		})
	}
	return ch, cancel
}

// Publish never blocks: a full subscriber drops the event (the UI also polls).
func (h *Hub) Publish(projectID string, e Event) {
	if h == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.subs[projectID] {
		select {
		case ch <- e:
		default:
		}
	}
}

// DataChanged publishes rows {"data_rev":n} and checks {"stale":true}; it is the Store notifier.
func (h *Hub) DataChanged(projectID string, dataRev int64) {
	h.Publish(projectID, Event{Name: "rows", Data: map[string]any{"data_rev": dataRev}})
	h.Publish(projectID, Event{Name: "checks", Data: map[string]any{"stale": true}})
}
