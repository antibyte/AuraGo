package server

import (
	"sync"

	"aurago/internal/flows"
)

// The scrubbed run events kept for reuse: at most flowSSECacheEntries entries and
// flowSSECacheBytes bytes; an event over flowSSECacheMaxEvent is scrubbed per stream.
const (
	flowSSECacheEntries  = 1024
	flowSSECacheBytes    = 16 << 20
	flowSSECacheMaxEvent = 1 << 20
)

type flowSSEKey struct {
	run string
	seq int
}

// flowSSECache keeps the scrubbed JSON of run events by (run, seq). Events are immutable and
// shared by every subscriber (flows.EventBus), so N open views of a run scrub each event once.
// Entries leave in insertion order. Limit: a secret registered after an event was cached stays in
// that cached copy; the stored run is scrubbed at write time and on every read.
type flowSSECache struct {
	mu      sync.Mutex
	entries map[flowSSEKey][]byte
	order   []flowSSEKey
	bytes   int
}

// scrubbedEvent returns ev's scrubbed JSON (flowScrubbedJSON), cached. The bytes are shared:
// never modify them.
func (c *flowSSECache) scrubbedEvent(ev flows.RunEvent) ([]byte, error) {
	key := flowSSEKey{ev.RunID, ev.Seq}
	c.mu.Lock()
	data, ok := c.entries[key]
	c.mu.Unlock()
	if ok {
		return data, nil
	}
	data, err := flowScrubbedJSON(ev)
	if err != nil || ev.RunID == "" || ev.Seq <= 0 || len(data) > flowSSECacheMaxEvent {
		return data, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if cached, ok := c.entries[key]; ok {
		return cached, nil
	}
	if c.entries == nil {
		c.entries = map[flowSSEKey][]byte{}
	}
	c.entries[key] = data
	c.order = append(c.order, key)
	c.bytes += len(data)
	for len(c.order) > flowSSECacheEntries || c.bytes > flowSSECacheBytes {
		old := c.order[0]
		c.order = c.order[1:]
		c.bytes -= len(c.entries[old])
		delete(c.entries, old)
	}
	return data, nil
}
