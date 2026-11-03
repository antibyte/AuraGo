package whatsapp

import (
	"container/list"
	"sync"
	"time"
)

const (
	// sentIDCacheSize and sentIDCacheTTL bound the IDs of our own outgoing
	// messages. The inbound filter uses them as the echo guard.
	sentIDCacheSize = 5000
	sentIDCacheTTL  = 20 * time.Minute
	// dedupeCacheSize bounds the IDs of accepted inbound messages. It has no
	// TTL: a bounded FIFO.
	dedupeCacheSize = 2048
)

// idCache remembers message IDs, at most max of them, each for ttl (0 = no expiry).
// The oldest entry is evicted first. It is safe for concurrent use.
type idCache struct {
	mu    sync.Mutex
	max   int
	ttl   time.Duration
	now   func() time.Time
	order *list.List // front = newest; values are idEntry
	items map[string]*list.Element
}

type idEntry struct {
	id string
	at time.Time
}

// newIDCache returns an empty cache. A max below 1 is raised to 1 so the cache
// stays bounded, and a nil now means time.Now.
func newIDCache(max int, ttl time.Duration, now func() time.Time) *idCache {
	if max < 1 {
		max = 1
	}
	if now == nil {
		now = time.Now
	}
	return &idCache{max: max, ttl: ttl, now: now, order: list.New(), items: make(map[string]*list.Element)}
}

// expired reports whether e has outlived the TTL at now. The caller holds the lock.
func (c *idCache) expired(e idEntry, now time.Time) bool {
	return c.ttl > 0 && !now.Before(e.at.Add(c.ttl))
}

// remove drops el from the cache. The caller holds the lock.
func (c *idCache) remove(el *list.Element) {
	delete(c.items, el.Value.(idEntry).id)
	c.order.Remove(el)
}

// Add remembers id, or refreshes it when it is already present. It then evicts
// expired entries from the back, and the oldest entries beyond max. An empty id
// is ignored.
func (c *idCache) Add(id string) {
	if id == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.now()
	if el, ok := c.items[id]; ok {
		el.Value = idEntry{id: id, at: now}
		c.order.MoveToFront(el)
	} else {
		c.items[id] = c.order.PushFront(idEntry{id: id, at: now})
	}
	for el := c.order.Back(); el != nil && c.expired(el.Value.(idEntry), now); el = c.order.Back() {
		c.remove(el)
	}
	for c.order.Len() > c.max {
		c.remove(c.order.Back())
	}
}

// Contains reports whether id is present. An expired entry counts as absent and
// is removed. It never refreshes an entry.
func (c *idCache) Contains(id string) bool {
	if id == "" {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.items[id]
	if !ok {
		return false
	}
	if c.expired(el.Value.(idEntry), c.now()) {
		c.remove(el)
		return false
	}
	return true
}

// Len is the number of stored entries, expired ones included until the next
// Add or Contains removes them.
func (c *idCache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.order.Len()
}
