package remote

import (
	"fmt"
	"sync"
	"time"
)

type nonceReplayCache struct {
	mu         sync.Mutex
	entries    map[string]time.Time
	ttl        time.Duration
	maxEntries int
}

func newNonceReplayCache(ttl time.Duration, maxEntries int) *nonceReplayCache {
	if ttl <= 0 {
		ttl = MaxTimestampDrift
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
