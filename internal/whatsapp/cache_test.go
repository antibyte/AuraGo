package whatsapp

import (
	"testing"
	"time"
)

func TestIDCacheExpiresAfterTTL(t *testing.T) {
	now := fixedNow()
	c := newIDCache(10, 20*time.Minute, func() time.Time { return now })
	c.Add("a")
	c.Add("b")

	now = now.Add(19*time.Minute + 59*time.Second)
	if !c.Contains("a") {
		t.Fatal("an ID must still be present at 19m59s")
	}

	now = now.Add(2 * time.Second) // 20m01s
	if got := c.Len(); got != 2 {
		t.Fatalf("Len before the next Add = %d, want 2 (expired entries are removed lazily)", got)
	}
	c.Add("c")
	if got := c.Len(); got != 1 {
		t.Fatalf("Len after the next Add = %d, want 1 (a and b expired)", got)
	}
	if c.Contains("a") || c.Contains("b") {
		t.Fatal("an ID must be absent at 20m01s")
	}
	if !c.Contains("c") {
		t.Fatal("the fresh ID must be present")
	}

	// Contains alone also drops an expired entry.
	now = now.Add(21 * time.Minute)
	if c.Contains("c") || c.Len() != 0 {
		t.Fatalf("Contains kept an expired entry, Len = %d", c.Len())
	}
}

func TestIDCacheEvictsOldestBeyondCapacity(t *testing.T) {
	now := fixedNow()
	c := newIDCache(3, 0, func() time.Time { return now }) // ttl 0: no expiry
	for _, id := range []string{"a", "b", "c", "d"} {
		c.Add(id)
	}
	if c.Contains("a") || !c.Contains("b") || !c.Contains("c") || !c.Contains("d") || c.Len() != 3 {
		t.Fatalf("adding a-d to a cache of 3 must evict only a, Len = %d", c.Len())
	}

	c.Add("b") // refresh: b is now the newest, c the oldest
	c.Add("e")
	if !c.Contains("b") || c.Contains("c") || !c.Contains("d") || !c.Contains("e") || c.Len() != 3 {
		t.Fatalf("after refreshing b, the next Add must evict c, Len = %d", c.Len())
	}

	now = now.Add(1000 * time.Hour)
	if !c.Contains("b") {
		t.Fatal("a cache without TTL must not expire entries")
	}
	c.Add("")
	if c.Contains("") || c.Len() != 3 {
		t.Fatal("an empty ID must be ignored")
	}
}
