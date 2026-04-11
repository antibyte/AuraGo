package agent

import (
	"io"
	"log/slog"
	"testing"
	"time"

	"aurago/internal/memory"
)

func TestLoadMemoryMetaMapUsesTTLCache(t *testing.T) {
	resetMemoryMetaCacheForTests()
	t.Cleanup(resetMemoryMetaCacheForTests)

	oldNow := memoryMetaCacheNow
	oldTTL := memoryMetaCacheTTL
	defer func() {
		memoryMetaCacheNow = oldNow
		memoryMetaCacheTTL = oldTTL
	}()

	baseTime := time.Date(2026, time.April, 1, 10, 0, 0, 0, time.UTC)
	memoryMetaCacheNow = func() time.Time { return baseTime }
	memoryMetaCacheTTL = 5 * time.Minute

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })

	if err := stm.UpsertMemoryMeta("doc-1"); err != nil {
		t.Fatalf("UpsertMemoryMeta doc-1: %v", err)
	}
	first := loadMemoryMetaMap(stm)
	if _, ok := first["doc-1"]; !ok {
		t.Fatal("expected doc-1 in initial cache load")
	}

	if err := stm.UpsertMemoryMeta("doc-2"); err != nil {
		t.Fatalf("UpsertMemoryMeta doc-2: %v", err)
	}
	second := loadMemoryMetaMap(stm)
	if _, ok := second["doc-2"]; ok {
		t.Fatal("did not expect doc-2 before cache expiry")
	}

	memoryMetaCacheNow = func() time.Time { return baseTime.Add(6 * time.Minute) }
	third := loadMemoryMetaMap(stm)
	if _, ok := third["doc-2"]; !ok {
		t.Fatal("expected doc-2 after cache expiry reload")
	}
}

func TestLoadMemoryMetaMapReloadsForDifferentStores(t *testing.T) {
	resetMemoryMetaCacheForTests()
	t.Cleanup(resetMemoryMetaCacheForTests)

	oldNow := memoryMetaCacheNow
	oldTTL := memoryMetaCacheTTL
	defer func() {
		memoryMetaCacheNow = oldNow
		memoryMetaCacheTTL = oldTTL
	}()

	memoryMetaCacheNow = func() time.Time { return time.Date(2026, time.April, 1, 11, 0, 0, 0, time.UTC) }
	memoryMetaCacheTTL = time.Hour

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stmA, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory A: %v", err)
	}
	t.Cleanup(func() { _ = stmA.Close() })
	stmB, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory B: %v", err)
	}
	t.Cleanup(func() { _ = stmB.Close() })

	if err := stmA.UpsertMemoryMeta("doc-a"); err != nil {
		t.Fatalf("UpsertMemoryMeta doc-a: %v", err)
	}
	if err := stmB.UpsertMemoryMeta("doc-b"); err != nil {
		t.Fatalf("UpsertMemoryMeta doc-b: %v", err)
	}

	metaA := loadMemoryMetaMap(stmA)
	metaB := loadMemoryMetaMap(stmB)

	if _, ok := metaA["doc-a"]; !ok {
		t.Fatal("expected doc-a in store A metadata")
	}
	if _, ok := metaB["doc-b"]; !ok {
		t.Fatal("expected doc-b in store B metadata")
	}
	if _, ok := metaB["doc-a"]; ok {
		t.Fatal("did not expect cached metadata from store A to leak into store B")
	}
}
