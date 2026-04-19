package agent

import (
	"strings"
	"testing"
)

// TestCacheHashKeyProducesCompactKeys verifies HIGH-3 fix:
// Hash-based keys should be significantly shorter than full-text keys.
func TestCacheHashKeyProducesCompactKeys(t *testing.T) {
	longText := strings.Repeat("This is a long text that would bloat memory as a map key. ", 1000)
	key := cacheHashKey(longText, "test-model")

	// Hash key should be very short (base36 of uint64, max ~13 chars)
	if len(key) > 20 {
		t.Errorf("cacheHashKey produced unexpectedly long key: %d chars", len(key))
	}
	if key == "" {
		t.Error("cacheHashKey should not return empty string")
	}
}

func TestCacheHashKeyDifferentInputs(t *testing.T) {
	key1 := cacheHashKey("hello", "model-a")
	key2 := cacheHashKey("hello", "model-b")
	key3 := cacheHashKey("world", "model-a")

	if key1 == key2 {
		t.Error("different models should produce different keys for same text")
	}
	if key1 == key3 {
		t.Error("different texts should produce different keys for same model")
	}
}

func TestTokenCountCacheBasicOperations(t *testing.T) {
	cache := newTokenCountCache(100)

	// Empty text returns 0
	if v := cache.Count("", "model"); v != 0 {
		t.Errorf("empty text should return 0, got %d", v)
	}

	// Non-empty text returns a count
	v1 := cache.Count("hello world", "test-model")
	if v1 <= 0 {
		t.Error("expected positive token count for non-empty text")
	}

	// Same input returns cached value
	v2 := cache.Count("hello world", "test-model")
	if v1 != v2 {
		t.Errorf("cached value should match: first=%d, second=%d", v1, v2)
	}
}

func TestTokenCountCacheEviction(t *testing.T) {
	cache := newTokenCountCache(10)

	// Fill beyond capacity
	for i := 0; i < 20; i++ {
		cache.Count(strings.Repeat("x", i+1), "model")
	}

	// Cache should have evicted some entries but still work
	if len(cache.counts) > 15 { // allow some slack
		t.Errorf("cache should have evicted entries, got %d", len(cache.counts))
	}
}

func TestTokenCountCacheMemoryEfficiency(t *testing.T) {
	cache := newTokenCountCache(100)

	// Simulate large tool outputs (the original bug scenario)
	largeText := strings.Repeat("Tool output line with data. ", 4000) // ~100KB
	v1 := cache.Count(largeText, "model")

	// The key stored should be tiny, not the full text
	for key := range cache.counts {
		if len(key) > 100 {
			t.Errorf("cache key is too long (%d chars), hash-based keys should be compact", len(key))
		}
	}

	// Same large text should hit cache
	v2 := cache.Count(largeText, "model")
	if v1 != v2 {
		t.Errorf("cache miss for same input: %d vs %d", v1, v2)
	}
}
