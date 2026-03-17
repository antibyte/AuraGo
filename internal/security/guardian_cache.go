package security

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// GuardianCache provides an in-memory cache for LLM Guardian decisions.
// Identical tool calls with the same parameters reuse previous results
// within the configured TTL window.
type GuardianCache struct {
	mu      sync.RWMutex
	entries map[string]guardianCacheEntry
	ttl     time.Duration
	maxSize int
}

type guardianCacheEntry struct {
	result    GuardianResult
	timestamp time.Time
}

// NewGuardianCache creates a cache with the given TTL and max entries.
func NewGuardianCache(ttlSeconds, maxSize int) *GuardianCache {
	if maxSize <= 0 {
		maxSize = 1000
	}
	return &GuardianCache{
		entries: make(map[string]guardianCacheEntry),
		ttl:     time.Duration(ttlSeconds) * time.Second,
		maxSize: maxSize,
	}
}

// Get returns a cached result if available and not expired.
func (c *GuardianCache) Get(key string) (GuardianResult, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, exists := c.entries[key]
	if !exists {
		return GuardianResult{}, false
	}
	if time.Since(entry.timestamp) > c.ttl {
		return GuardianResult{}, false
	}
	result := entry.result
	result.Cached = true
	return result, true
}

// Set stores a result in the cache, evicting the oldest entry if full.
func (c *GuardianCache) Set(key string, result GuardianResult) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.entries) >= c.maxSize {
		c.evictOldest()
	}
	c.entries[key] = guardianCacheEntry{
		result:    result,
		timestamp: time.Now(),
	}
}

// evictOldest removes the oldest cache entry. Caller must hold the write lock.
func (c *GuardianCache) evictOldest() {
	var oldestKey string
	var oldestTime time.Time
	first := true
	for k, v := range c.entries {
		if first || v.timestamp.Before(oldestTime) {
			oldestKey = k
			oldestTime = v.timestamp
			first = false
		}
	}
	if oldestKey != "" {
		delete(c.entries, oldestKey)
	}
}

// Size returns the number of cached entries.
func (c *GuardianCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

// GenerateCacheKey creates a deterministic hash from operation + parameters.
func GenerateCacheKey(operation string, params map[string]string) string {
	// Sort keys for deterministic ordering
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Use length-prefixed encoding to prevent key/value collisions.
	// e.g. {a="b:c", d=""} and {a="b", c="d"} must hash differently.
	var sb strings.Builder
	sb.WriteString(operation)
	for _, k := range keys {
		fmt.Fprintf(&sb, ":%d:%s=%d:%s", len(k), k, len(params[k]), params[k])
	}
	hash := sha256.Sum256([]byte(sb.String()))
	return fmt.Sprintf("%x", hash[:16]) // 128-bit key is sufficient
}
