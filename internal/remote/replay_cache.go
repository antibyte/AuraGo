package remote

import (
	"fmt"
	"sync"
	"time"
)

// NonceReplayTTL is how long a frame nonce stays cached on both ends.
// ValidateTimestamp accepts ±MaxTimestampDrift, so a nonce must stay cached for
// the full window. The extra second covers the boundary instant, which
// ValidateTimestamp still accepts, and RFC3339's whole-second timestamps.
const NonceReplayTTL = 2*MaxTimestampDrift + time.Second

type nonceReplayCache struct {
	mu         sync.Mutex
	entries    map[string]time.Time
	ttl        time.Duration
	maxEntries int
	// failClosed reports a new nonce as seen instead of evicting a live entry
	// when the cache is full, so flooding it cannot reopen a replay window.
	failClosed bool
}

func newNonceReplayCache(ttl time.Duration, maxEntries int) *nonceReplayCache {
	if ttl <= 0 {
		ttl = NonceReplayTTL
	}
	if maxEntries <= 0 {
		maxEntries = 10000
	}
	return &nonceReplayCache{
		entries:    make(map[string]time.Time),
		ttl:        ttl,
		maxEntries: maxEntries,
	}
}

func (c *nonceReplayCache) Seen(deviceID, nonce string, now time.Time) bool {
	if deviceID == "" || nonce == "" {
		return true
	}

	key := replayCacheKey(deviceID, nonce)
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cleanupExpiredLocked(now)
	if expiresAt, exists := c.entries[key]; exists && now.Before(expiresAt) {
		return true
	}
	if c.failClosed && len(c.entries) >= c.maxEntries {
		// Every entry is live (expired ones were just removed). Evicting one
		// would make its nonce replayable, so drop the new frame instead.
		return true
	}
	c.entries[key] = now.Add(c.ttl)
	if len(c.entries) > c.maxEntries {
		c.cleanupExpiredLocked(now)
		if len(c.entries) > c.maxEntries {
			c.evictOldestLocked()
		}
	}
	return false
}

func (c *nonceReplayCache) cleanupExpiredLocked(now time.Time) {
	for key, expiresAt := range c.entries {
		if !now.Before(expiresAt) {
			delete(c.entries, key)
		}
	}
}

func (c *nonceReplayCache) evictOldestLocked() {
	var oldestKey string
	var oldestExpiry time.Time
	first := true
	for key, expiresAt := range c.entries {
		if first || expiresAt.Before(oldestExpiry) {
			oldestKey = key
			oldestExpiry = expiresAt
			first = false
		}
	}
	if oldestKey != "" {
		delete(c.entries, oldestKey)
	}
}

func replayCacheKey(deviceID, nonce string) string {
	return fmt.Sprintf("%s:%s", deviceID, nonce)
}

// NonceReplayCache is the exported form for the remote agent, which must apply
// the same replay window as the supervisor: per-hub on the supervisor, across
// reconnects on the agent.
type NonceReplayCache struct{ inner *nonceReplayCache }

// NewNonceReplayCache returns an evicting cache like the supervisor's: when
// full of live entries it drops the oldest one to admit a new nonce.
func NewNonceReplayCache(ttl time.Duration, maxEntries int) *NonceReplayCache {
	return &NonceReplayCache{inner: newNonceReplayCache(ttl, maxEntries)}
}

// NewFailClosedNonceReplayCache returns a cache that never evicts a live
// entry: when full, every new nonce is reported as seen until entries expire.
// The agent uses it because anyone who can get enough fresh signed frames
// delivered could otherwise flush a captured frame's nonce and replay it. The
// supervisor keeps the evicting form so one device cannot lock out others.
func NewFailClosedNonceReplayCache(ttl time.Duration, maxEntries int) *NonceReplayCache {
	inner := newNonceReplayCache(ttl, maxEntries)
	inner.failClosed = true
	return &NonceReplayCache{inner: inner}
}

// Seen records the nonce and reports whether it was already seen within the TTL.
// Empty device IDs or nonces are always reported as seen.
func (c *NonceReplayCache) Seen(deviceID, nonce string, now time.Time) bool {
	return c.inner.Seen(deviceID, nonce, now)
}
