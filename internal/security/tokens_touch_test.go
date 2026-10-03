package security

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestTouchLastUsedThrottlesPersistence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tokens.bin")
	vault, err := NewVault(strings.Repeat("c", 64), filepath.Join(dir, "vault.bin"))
	if err != nil {
		t.Fatalf("NewVault: %v", err)
	}
	tm, err := NewTokenManager(vault, path)
	if err != nil {
		t.Fatalf("NewTokenManager: %v", err)
	}
	_, meta, err := tm.Create("display", []string{"cyd"}, nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	now := time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)
	tm.clock = func() time.Time { return now }

	tm.TouchLastUsed(meta.ID) // first touch persists
	first, _ := os.ReadFile(path)

	now = now.Add(10 * time.Second)
	tm.TouchLastUsed(meta.ID)
	second, _ := os.ReadFile(path)
	if !bytes.Equal(first, second) {
		t.Fatal("touch inside the persistence interval rewrote the token file")
	}
	got, err := tm.Get(meta.ID)
	if err != nil || got.LastUsedAt == nil || !got.LastUsedAt.Equal(now) {
		t.Fatalf("in-memory last use = %v (%v), want %v", got.LastUsedAt, err, now)
	}

	now = now.Add(tokenLastUsedPersistInterval)
	tm.TouchLastUsed(meta.ID)
	third, _ := os.ReadFile(path)
	if bytes.Equal(second, third) {
		t.Fatal("touch after the persistence interval was not written")
	}
	reopened, err := NewTokenManager(vault, path)
	if err != nil {
		t.Fatalf("NewTokenManager(reopen): %v", err)
	}
	persisted, err := reopened.Get(meta.ID)
	if err != nil || persisted.LastUsedAt == nil || !persisted.LastUsedAt.Equal(now) {
		t.Fatalf("persisted last use = %v (%v), want %v", persisted.LastUsedAt, err, now)
	}
}
