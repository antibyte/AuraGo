package agent

import (
	"io"
	"log/slog"
	"testing"
	"time"

	"aurago/internal/memory"
)

func TestApplySessionMemoryReusePenaltyPrefersNovelMemories(t *testing.T) {
	candidates := []rankedMemory{
		{text: "repeat", docID: "doc-repeat", score: 0.92},
		{text: "fresh", docID: "doc-fresh", score: 0.88},
	}
	used := map[string]int{"doc-repeat": 2}

	ranked := applySessionMemoryReusePenalty(candidates, used)

	if len(ranked) != 2 {
		t.Fatalf("len(ranked) = %d, want 2", len(ranked))
	}
	if ranked[0].docID != "doc-fresh" {
		t.Fatalf("top docID = %q, want %q after reuse penalty", ranked[0].docID, "doc-fresh")
	}
	if ranked[1].docID != "doc-repeat" {
		t.Fatalf("second docID = %q, want %q", ranked[1].docID, "doc-repeat")
	}
}

func TestApplySessionMemoryReusePenaltyLeavesFreshCandidatesUntouched(t *testing.T) {
	candidates := []rankedMemory{
		{text: "a", docID: "doc-a", score: 0.77},
		{text: "b", docID: "doc-b", score: 0.66},
	}

	ranked := applySessionMemoryReusePenalty(candidates, map[string]int{})

	if ranked[0].docID != "doc-a" || ranked[1].docID != "doc-b" {
		t.Fatalf("unexpected order after empty reuse map: %+v", ranked)
	}
	if ranked[0].score != candidates[0].score || ranked[1].score != candidates[1].score {
		t.Fatalf("scores changed unexpectedly: %+v", ranked)
	}
}

func TestMarkMemoryDocIDsUsedCountsSelectedMemories(t *testing.T) {
	used := map[string]int{}
	selected := []rankedMemory{
		{docID: "doc-a"},
		{docID: "doc-b"},
		{docID: "doc-a"},
		{docID: ""},
	}

	markMemoryDocIDsUsed(used, selected)

	if used["doc-a"] != 2 {
		t.Fatalf("used[doc-a] = %d, want 2", used["doc-a"])
	}
	if used["doc-b"] != 1 {
		t.Fatalf("used[doc-b] = %d, want 1", used["doc-b"])
	}
}

func TestRankMemoryCandidatesIntegratesReusePenalty(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })

	if err := stm.UpsertMemoryMetaWithDetails("doc-repeat", memory.MemoryMetaUpdate{
		ExtractionConfidence: 0.90,
		VerificationStatus:   "confirmed",
		SourceType:           "memory_analysis",
		SourceReliability:    0.90,
	}); err != nil {
		t.Fatalf("UpsertMemoryMetaWithDetails doc-repeat: %v", err)
	}
	if err := stm.UpsertMemoryMetaWithDetails("doc-fresh", memory.MemoryMetaUpdate{
		ExtractionConfidence: 0.90,
		VerificationStatus:   "confirmed",
		SourceType:           "memory_analysis",
		SourceReliability:    0.90,
	}); err != nil {
		t.Fatalf("UpsertMemoryMetaWithDetails doc-fresh: %v", err)
	}

	memories := []string{
		"[Similarity: 0.92] repeated memory",
		"[Similarity: 0.88] fresh memory",
	}
	docIDs := []string{"doc-repeat", "doc-fresh"}

	ranked := rankMemoryCandidates(memories, docIDs, stm, map[string]int{"doc-repeat": 2}, time.Now())
	if len(ranked) != 2 {
		t.Fatalf("len(ranked) = %d, want 2", len(ranked))
	}
	if ranked[0].docID != "doc-fresh" {
		t.Fatalf("top docID = %q, want doc-fresh after integrated reuse penalty", ranked[0].docID)
	}
}
