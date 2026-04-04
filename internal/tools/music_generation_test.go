package tools

import (
	"testing"
	"time"
)

func TestMusicCounterIncrement(t *testing.T) {
	// Reset counter state
	musicCounter.mu.Lock()
	musicCounter.date = ""
	musicCounter.count = 0
	musicCounter.mu.Unlock()

	// Unlimited (maxDaily=0) should always allow
	count, allowed := musicCounterIncrement(0)
	if !allowed {
		t.Fatal("expected unlimited to allow, got denied")
	}
	if count != 1 {
		t.Fatalf("expected count=1, got %d", count)
	}

	// Reset for limit test
	musicCounter.mu.Lock()
	musicCounter.count = 0
	musicCounter.mu.Unlock()

	// Limited to 2: first two should pass, third should fail
	_, ok1 := musicCounterIncrement(2)
	_, ok2 := musicCounterIncrement(2)
	_, ok3 := musicCounterIncrement(2)
	if !ok1 || !ok2 {
		t.Fatal("expected first two increments to be allowed")
	}
	if ok3 {
		t.Fatal("expected third increment to be denied with maxDaily=2")
	}
}

func TestMusicCounterDateReset(t *testing.T) {
	// Simulate yesterday's counter
	musicCounter.mu.Lock()
	musicCounter.date = "2020-01-01"
	musicCounter.count = 999
	musicCounter.mu.Unlock()

	// Should reset and allow
	count, allowed := musicCounterIncrement(5)
	if !allowed {
		t.Fatal("expected date reset to allow new increment")
	}
	if count != 1 {
		t.Fatalf("expected count=1 after date reset, got %d", count)
	}
}

func TestMusicCounterGet(t *testing.T) {
	// Set counter to today's date so MusicCounterGet returns the count
	today := time.Now().Format("2006-01-02")
	musicCounter.mu.Lock()
	musicCounter.date = today
	musicCounter.count = 42
	musicCounter.mu.Unlock()

	got := MusicCounterGet()
	if got != 42 {
		t.Fatalf("expected MusicCounterGet()=42, got %d", got)
	}

	// Different date should return 0
	musicCounter.mu.Lock()
	musicCounter.date = "2020-01-01"
	musicCounter.count = 99
	musicCounter.mu.Unlock()

	got = MusicCounterGet()
	if got != 0 {
		t.Fatalf("expected MusicCounterGet()=0 for old date, got %d", got)
	}
}

func TestMusicGenParamsValidation(t *testing.T) {
	// Empty prompt should be caught by the caller, but struct should work
	p := MusicGenParams{
		Prompt:       "test prompt",
		Lyrics:       "[Verse]\nHello world\n",
		Instrumental: false,
		Title:        "Test Song",
	}
	if p.Prompt != "test prompt" {
		t.Fatal("unexpected prompt value")
	}
	if p.Instrumental {
		t.Fatal("expected instrumental=false")
	}
}
