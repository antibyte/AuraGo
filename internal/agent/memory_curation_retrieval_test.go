package agent

import (
	"io"
	"log/slog"
	"testing"
	"time"

	"aurago/internal/memory"
)

func TestRankMemoryCandidatesFiltersArchivedMemories(t *testing.T) {
	resetMemoryMetaCacheForTests()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })

	if err := stm.UpsertMemoryMetaWithDetails("doc-active", memory.MemoryMetaUpdate{VerificationStatus: "confirmed", ExtractionConfidence: 0.95, SourceReliability: 0.90}); err != nil {
		t.Fatalf("active meta: %v", err)
	}
	if err := stm.UpsertMemoryMetaWithDetails("doc-archived", memory.MemoryMetaUpdate{VerificationStatus: "unverified", ExtractionConfidence: 0.95, SourceReliability: 0.90}); err != nil {
		t.Fatalf("archived meta: %v", err)
	}
	if err := stm.ApplyMemoryCurationAction(memory.MemoryCurationAction{DocID: "doc-archived", Action: memory.MemoryCurationActionArchive, Reason: "test"}, "system", false); err != nil {
		t.Fatalf("archive meta: %v", err)
	}

	ranked := rankMemoryCandidates(
		[]string{"active memory", "archived memory"},
		[]string{"doc-active", "doc-archived"},
		stm,
		nil,
		time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC),
	)
	if len(ranked) != 1 {
		t.Fatalf("ranked len = %d, want 1; ranked=%+v", len(ranked), ranked)
	}
	if ranked[0].docID != "doc-active" {
		t.Fatalf("ranked doc = %q, want doc-active", ranked[0].docID)
	}
}
