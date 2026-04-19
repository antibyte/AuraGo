package tools

import (
	"testing"
	"time"
)

func TestMusicCounterReserve(t *testing.T) {
	// Reset counter state
	musicCounter.mu.Lock()
	musicCounter.date = ""
	musicCounter.count = 0
	musicCounter.mu.Unlock()

	// Unlimited (maxDaily=0) should always allow
	count, allowed := musicCounterReserve(0)
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
	_, ok1 := musicCounterReserve(2)
	_, ok2 := musicCounterReserve(2)
	_, ok3 := musicCounterReserve(2)
	if !ok1 || !ok2 {
		t.Fatal("expected first two reserves to be allowed")
	}
	if ok3 {
		t.Fatal("expected third reserve to be denied with maxDaily=2")
	}
}

func TestMusicCounterRelease(t *testing.T) {
	// Reset counter state
	musicCounter.mu.Lock()
	musicCounter.date = ""
	musicCounter.count = 0
	musicCounter.mu.Unlock()

	// Reserve 2 slots
	musicCounterReserve(10)
	musicCounterReserve(10)

	// Verify count is 2
	if got := MusicCounterGet(); got != 2 {
		t.Fatalf("expected count=2, got %d", got)
	}

	// Release one slot
	musicCounterRelease()
	if got := MusicCounterGet(); got != 1 {
		t.Fatalf("expected count=1 after release, got %d", got)
	}

	// Release another
	musicCounterRelease()
	if got := MusicCounterGet(); got != 0 {
		t.Fatalf("expected count=0 after release, got %d", got)
	}

	// Release below zero should be safe (clamped at 0)
	musicCounterRelease()
	if got := MusicCounterGet(); got != 0 {
		t.Fatalf("expected count=0 after over-release, got %d", got)
	}
}

func TestMusicCounterReserveAndRelease(t *testing.T) {
	// Simulate a failed generation: reserve, then release
	musicCounter.mu.Lock()
	musicCounter.date = ""
	musicCounter.count = 0
	musicCounter.mu.Unlock()

	// With maxDaily=1, reserve should succeed once
	count, ok := musicCounterReserve(1)
	if !ok || count != 1 {
		t.Fatalf("expected first reserve to succeed with count=1, got ok=%v count=%d", ok, count)
	}

	// Second reserve should fail
	_, ok2 := musicCounterReserve(1)
	if ok2 {
		t.Fatal("expected second reserve to be denied")
	}

	// Release the slot (simulating API failure)
	musicCounterRelease()

	// Now reserve should succeed again
	count, ok3 := musicCounterReserve(1)
	if !ok3 || count != 1 {
		t.Fatalf("expected reserve after release to succeed, got ok=%v count=%d", ok3, count)
	}
}

func TestMusicCounterDateReset(t *testing.T) {
	// Simulate yesterday's counter
	musicCounter.mu.Lock()
	musicCounter.date = "2020-01-01"
	musicCounter.count = 999
	musicCounter.mu.Unlock()

	// Should reset and allow
	count, allowed := musicCounterReserve(5)
	if !allowed {
		t.Fatal("expected date reset to allow new reserve")
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
