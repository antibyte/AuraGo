package memory

import (
	"testing"
	"time"
)

func TestMemoryExpiryRoundTripAndClear(t *testing.T) {
	s := newInMemorySQLiteMemory(t)
	ctx := t.Context()
	if _, ok, err := s.MemoryExpiry(ctx, "doc"); err != nil || ok {
		t.Fatalf("fresh expiry ok=%v err=%v", ok, err)
	}
	at := time.Date(2026, 11, 2, 23, 59, 59, 0, time.FixedZone("CET", 3600))
	if err := s.SetMemoryExpiry(ctx, []string{"doc", " ", "other"}, at); err != nil {
		t.Fatal(err)
	}
	got, ok, err := s.MemoryExpiry(ctx, "doc")
	if err != nil || !ok || !got.Equal(at) {
		t.Fatalf("expiry=%v ok=%v err=%v, want %v", got, ok, err, at)
	}
	if err := s.SetMemoryExpiry(ctx, []string{"doc"}, time.Time{}); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := s.MemoryExpiry(ctx, "doc"); err != nil || ok {
		t.Fatalf("cleared expiry ok=%v err=%v", ok, err)
	}
	if _, ok, _ := s.MemoryExpiry(ctx, "other"); !ok {
		t.Fatal("clearing one document removed another")
	}
}
