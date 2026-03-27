package memory

import (
	"io"
	"log/slog"
	"testing"
)

func TestRecordMemoryUsageAndGetRecentMemoryUsage(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })

	if err := stm.RecordMemoryUsage("doc-1", "ltm_retrieved", "session-a", 0.91, false); err != nil {
		t.Fatalf("RecordMemoryUsage doc-1: %v", err)
	}
	if err := stm.RecordMemoryUsage("doc-2", "ltm_predicted", "session-a", 0, false); err != nil {
		t.Fatalf("RecordMemoryUsage doc-2: %v", err)
	}
	if err := stm.RecordMemoryUsage("doc-3", "ltm_retrieved", "session-b", 0.55, true); err != nil {
		t.Fatalf("RecordMemoryUsage doc-3: %v", err)
	}

	entries, err := stm.GetRecentMemoryUsage("session-a", 10)
	if err != nil {
		t.Fatalf("GetRecentMemoryUsage session-a: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("len(entries) = %d, want 2", len(entries))
	}
	if entries[0].SessionID != "session-a" || entries[1].SessionID != "session-a" {
		t.Fatalf("unexpected session IDs: %+v", entries)
	}

	seen := map[string]MemoryUsageEntry{}
	for _, entry := range entries {
		seen[entry.MemoryID] = entry
	}
	if seen["doc-1"].MemoryType != "ltm_retrieved" {
		t.Fatalf("doc-1 memory_type = %q, want ltm_retrieved", seen["doc-1"].MemoryType)
	}
	if seen["doc-1"].ContextRelevance != 0.91 {
		t.Fatalf("doc-1 context_relevance = %v, want 0.91", seen["doc-1"].ContextRelevance)
	}
	if seen["doc-2"].MemoryType != "ltm_predicted" {
		t.Fatalf("doc-2 memory_type = %q, want ltm_predicted", seen["doc-2"].MemoryType)
	}
}

func TestRecordMemoryUsageClampsAndDefaults(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })

	if err := stm.RecordMemoryUsage("doc-4", "", "", 9.5, false); err != nil {
		t.Fatalf("RecordMemoryUsage: %v", err)
	}

	entries, err := stm.GetRecentMemoryUsage("", 5)
	if err != nil {
		t.Fatalf("GetRecentMemoryUsage: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("len(entries) = %d, want 1", len(entries))
	}
	if entries[0].MemoryType != "ltm" {
		t.Fatalf("memory_type = %q, want ltm", entries[0].MemoryType)
	}
	if entries[0].SessionID != "default" {
		t.Fatalf("session_id = %q, want default", entries[0].SessionID)
	}
	if entries[0].ContextRelevance != 1 {
		t.Fatalf("context_relevance = %v, want 1", entries[0].ContextRelevance)
	}
}
