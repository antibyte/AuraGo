package memory

import (
	"io"
	"log/slog"
	"testing"
)

func TestBuildMemoryHealthReport(t *testing.T) {
	metas := []MemoryMeta{
		{
			DocID:                "doc-stale",
			AccessCount:          0,
			LastAccessed:         "2026-01-01 00:00:00",
			ExtractionConfidence: 0.40,
			VerificationStatus:   "unverified",
			SourceReliability:    0.40,
			UsefulCount:          0,
			UselessCount:         3,
		},
		{
			DocID:                "doc-good",
			AccessCount:          4,
			LastAccessed:         "2026-03-20 00:00:00",
			ExtractionConfidence: 0.91,
			VerificationStatus:   "confirmed",
			SourceReliability:    0.92,
			UsefulCount:          4,
			UselessCount:         1,
		},
		{
			DocID:                "doc-conflict",
			AccessCount:          1,
			LastAccessed:         "2026-02-01 00:00:00",
			ExtractionConfidence: 0.65,
			VerificationStatus:   "contradicted",
			SourceReliability:    0.80,
			UsefulCount:          0,
			UselessCount:         0,
		},
	}
	usage := MemoryUsageStats{
		TopReused: []MemoryUsageAggregate{
			{MemoryID: "doc-good", Count: 4},
		},
	}

	report := BuildMemoryHealthReport(metas, usage)

	if report.Confidence.Total != 3 {
		t.Fatalf("expected 3 memories in confidence stats, got %d", report.Confidence.Total)
	}
	if report.Confidence.Confirmed != 1 {
		t.Fatalf("expected 1 confirmed memory, got %d", report.Confidence.Confirmed)
	}
	if report.Curator.StaleCandidates == 0 {
		t.Fatal("expected stale candidates to be detected")
	}
	if report.Curator.Contradictions != 1 {
		t.Fatalf("expected 1 contradiction, got %d", report.Curator.Contradictions)
	}
	if report.Effectiveness.Tracked != 2 {
		t.Fatalf("expected 2 tracked memories, got %d", report.Effectiveness.Tracked)
	}
	if report.Effectiveness.Helpful != 1 {
		t.Fatalf("expected 1 helpful memory, got %d", report.Effectiveness.Helpful)
	}
	if report.Effectiveness.Underperforming != 1 {
		t.Fatalf("expected 1 underperforming memory, got %d", report.Effectiveness.Underperforming)
	}
	if report.Curator.LowEffectiveness != 1 {
		t.Fatalf("expected 1 low-effectiveness memory, got %d", report.Curator.LowEffectiveness)
	}
	if report.Curator.OverusedMemories != 1 {
		t.Fatalf("expected 1 overused memory, got %d", report.Curator.OverusedMemories)
	}
	if len(report.Curator.TopUnderperforming) != 1 || report.Curator.TopUnderperforming[0] != "doc-stale" {
		t.Fatalf("unexpected top underperforming memories: %v", report.Curator.TopUnderperforming)
	}
	if len(report.Curator.Suggestions) == 0 {
		t.Fatal("expected curator suggestions")
	}
}

func TestEnforceMemoryBudgetRespectsProtectedRows(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })

	for _, docID := range []string{"doc-a", "doc-b", "doc-c"} {
		if err := stm.UpsertMemoryMeta(docID); err != nil {
			t.Fatalf("UpsertMemoryMeta(%s): %v", docID, err)
		}
	}
	if err := stm.ApplyMemoryCurationAction(MemoryCurationAction{
		DocID:  "doc-c",
		Action: MemoryCurationActionProtect,
		Reason: "test protect",
	}, "test", false); err != nil {
		t.Fatalf("ApplyMemoryCurationAction protect: %v", err)
	}

	toEvict, err := stm.EnforceMemoryBudget(2)
	if err != nil {
		t.Fatalf("EnforceMemoryBudget: %v", err)
	}
	if len(toEvict) != 1 {
		t.Fatalf("len(toEvict) = %d, want 1", len(toEvict))
	}
	if toEvict[0] == "doc-c" {
		t.Fatalf("protected doc must not be evicted, got %q", toEvict[0])
	}
}

func TestApplyMemoryBudgetEnforcementDeletesFromLTM(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })

	ltm := &fakeRepairVectorDB{docs: map[string]string{
		"doc-a": "alpha",
		"doc-b": "beta",
		"doc-c": "gamma",
	}}
	for _, docID := range []string{"doc-a", "doc-b", "doc-c"} {
		if err := stm.UpsertMemoryMeta(docID); err != nil {
			t.Fatalf("UpsertMemoryMeta(%s): %v", docID, err)
		}
	}

	evicted, err := stm.ApplyMemoryBudgetEnforcement(2, ltm)
	if err != nil {
		t.Fatalf("ApplyMemoryBudgetEnforcement: %v", err)
	}
	if evicted != 1 {
		t.Fatalf("evicted = %d, want 1", evicted)
	}
	if len(ltm.deleted) != 1 {
		t.Fatalf("deleted docs = %v, want 1", ltm.deleted)
	}

	stats, err := stm.GetMemoryBudgetStats(2)
	if err != nil {
		t.Fatalf("GetMemoryBudgetStats: %v", err)
	}
	if stats.OverBudget {
		t.Fatalf("OverBudget = true after enforcement, stats = %+v", stats)
	}
	toEvict, err := stm.EnforceMemoryBudget(2)
	if err != nil {
		t.Fatalf("EnforceMemoryBudget after enforcement: %v", err)
	}
	if len(toEvict) != 0 {
		t.Fatalf("toEvict after enforcement = %v, want none", toEvict)
	}
}

func TestEnforceMemoryBudgetDoesNotEvictContradictedReviewRows(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })

	if err := stm.UpsertMemoryMetaWithDetails("doc-review", MemoryMetaUpdate{
		VerificationStatus: "contradicted",
	}); err != nil {
		t.Fatalf("UpsertMemoryMetaWithDetails review: %v", err)
	}
	if err := stm.UpsertMemoryMetaWithDetails("doc-low", MemoryMetaUpdate{
		VerificationStatus: "unverified",
	}); err != nil {
		t.Fatalf("UpsertMemoryMetaWithDetails low: %v", err)
	}

	toEvict, err := stm.EnforceMemoryBudget(1)
	if err != nil {
		t.Fatalf("EnforceMemoryBudget: %v", err)
	}
	if len(toEvict) != 1 || toEvict[0] != "doc-low" {
		t.Fatalf("toEvict = %v, want [doc-low]", toEvict)
	}
	stats, err := stm.GetMemoryBudgetStats(1)
	if err != nil {
		t.Fatalf("GetMemoryBudgetStats: %v", err)
	}
	if stats.Evictable != 1 {
		t.Fatalf("Evictable = %d, want 1", stats.Evictable)
	}
}
