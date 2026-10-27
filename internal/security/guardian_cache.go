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
	entry, exists := c.entries[key]
	c.mu.RUnlock()
	if !exists {
		return GuardianResult{}, false
	}
	if time.Since(entry.timestamp) > c.ttl {
		c.mu.Lock()
		if current, ok := c.entries[key]; ok && time.Since(current.timestamp) > c.ttl {
			delete(c.entries, key)
		}
		c.mu.Unlock()
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

	c.cleanupExpiredLocked(time.Now())
	if len(c.entries) >= c.maxSize {
		c.evictOldest()
	}
	c.entries[key] = guardianCacheEntry{
		result:    result,
		timestamp: time.Now(),
	}
}

// cleanupExpiredLocked opportunistically removes expired entries before eviction pressure builds up.
// Caller must hold the write lock.
func (c *GuardianCache) cleanupExpiredLocked(now time.Time) {
	if len(c.entries) == 0 {
		return
	}
	for key, entry := range c.entries {
		if now.Sub(entry.timestamp) > c.ttl {
			delete(c.entries, key)
		}
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
// It is injective over both: the operation is length-prefixed like every
// parameter name and value, so operation text can never pass for parameters.
func GenerateCacheKey(operation string, params map[string]string) string {
	return generateCacheKey([]string{operation}, params)
}

// generateToolCacheKey keys one tool-call verdict. The operation and the
// context snippet (raw, attacker-influenced user text) are separate fields, so
// no context can spell a parameter list or another operation.
func generateToolCacheKey(operation, contextSnippet string, params map[string]string) string {
	return generateCacheKey([]string{operation, contextSnippet}, params)
}

// generateCacheKey hashes a field count, the fixed fields and the sorted
// parameters. Every field is "<length>:<bytes>", so the encoding parses back
// to exactly one (fields, params) pair; the count keeps keys of different
// shapes apart. e.g. {a="b:c", d=""} and {a="b", c="d"} must hash differently.
func generateCacheKey(fields []string, params map[string]string) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var sb strings.Builder
	fmt.Fprintf(&sb, "%d;", len(fields))
	for _, field := range fields {
		writeCacheKeyField(&sb, field)
	}
	for _, k := range keys {
		writeCacheKeyField(&sb, k)
		writeCacheKeyField(&sb, params[k])
	}
	hash := sha256.Sum256([]byte(sb.String()))
	return fmt.Sprintf("%x", hash[:16]) // 128-bit key is sufficient
}

func writeCacheKeyField(sb *strings.Builder, field string) {
	fmt.Fprintf(sb, "%d:%s", len(field), field)
}
