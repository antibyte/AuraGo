package memory

import (
	"fmt"
	"log/slog"
	"os"
	"testing"
)

func newTestProfileDB(t *testing.T) *SQLiteMemory {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	stm, err := NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { stm.Close() })
	return stm
}

func TestUpsertProfileEntryRejectsPII(t *testing.T) {
	stm := newTestProfileDB(t)
	if err := stm.UpsertProfileEntry("comm", "email", "user@example.com", "v2"); err == nil {
		t.Fatal("expected PII-containing value to be rejected")
	}
}

func TestUpsertProfileEntryRejectsInvalidCategory(t *testing.T) {
	stm := newTestProfileDB(t)
	if err := stm.UpsertProfileEntry("session", "language", "go", "v2"); err == nil {
		t.Fatal("expected invalid category to be rejected")
	}
}

func TestUpsertProfileEntryNormalizesValue(t *testing.T) {
	stm := newTestProfileDB(t)
	if err := stm.UpsertProfileEntry("tech", "language", " GoLang ", "v2"); err != nil {
		t.Fatalf("UpsertProfileEntry: %v", err)
	}
	entries, err := stm.GetProfileEntries("tech")
	if err != nil {
		t.Fatalf("GetProfileEntries: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Value != "go" {
		t.Fatalf("expected normalized value 'go', got %q", entries[0].Value)
	}
}

func TestUpsertProfileEntryEnforcesCategoryLimit(t *testing.T) {
	stm := newTestProfileDB(t)
	for i := 0; i < maxProfileEntriesPerCategory+5; i++ {
		if err := stm.UpsertProfileEntry("tech", fmt.Sprintf("tool_%02d", i), fmt.Sprintf("value_%02d", i), "v2"); err != nil {
			t.Fatalf("UpsertProfileEntry #%d: %v", i, err)
		}
	}
	entries, err := stm.GetProfileEntries("tech")
	if err != nil {
		t.Fatalf("GetProfileEntries: %v", err)
	}
	if len(entries) > maxProfileEntriesPerCategory {
		t.Fatalf("expected category limit %d, got %d", maxProfileEntriesPerCategory, len(entries))
	}
}
