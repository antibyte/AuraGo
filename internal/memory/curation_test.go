package memory

import (
	"testing"
	"time"
)

func TestBuildMemoryCurationPlanAutoConfirmsSafeUnverifiedMemory(t *testing.T) {
	now := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)
	usage := MemoryUsageStats{
		TopReused: []MemoryUsageAggregate{{
			MemoryID:         "doc-safe",
			Count:            2,
			WasCitedRecently: true,
		}},
	}
	plan := BuildMemoryCurationPlan([]MemoryMeta{{
		DocID:                "doc-safe",
		AccessCount:          2,
		LastAccessed:         now.Add(-2 * 24 * time.Hour).Format("2006-01-02 15:04:05"),
		ExtractionConfidence: 0.95,
		VerificationStatus:   "unverified",
		SourceReliability:    0.91,
	}}, usage, MemoryCurationOptions{
		ConfirmThreshold: 0.92,
		MaxActions:       100,
		Now:              now,
	})

	if len(plan.AutoConfirm) != 1 {
		t.Fatalf("AutoConfirm len = %d, want 1; plan=%+v", len(plan.AutoConfirm), plan)
	}
	action := plan.AutoConfirm[0]
	if action.DocID != "doc-safe" || action.Action != MemoryCurationActionConfirm {
		t.Fatalf("unexpected action: %+v", action)
	}
	if len(plan.AutoArchive) != 0 || len(plan.ReviewRequired) != 0 {
		t.Fatalf("unexpected non-confirm actions: %+v", plan)
	}
}

func TestBuildMemoryCurationPlanArchivesWeakStaleMemoryButSkipsProtectedAndConfirmed(t *testing.T) {
	now := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)
	old := now.Add(-60 * 24 * time.Hour).Format("2006-01-02 15:04:05")
	plan := BuildMemoryCurationPlan([]MemoryMeta{
		{DocID: "doc-weak", LastAccessed: old, ExtractionConfidence: 0.50, VerificationStatus: "unverified", SourceReliability: 0.50},
		{DocID: "doc-protected", LastAccessed: old, ExtractionConfidence: 0.20, VerificationStatus: "unverified", Protected: true},
		{DocID: "doc-confirmed", LastAccessed: old, ExtractionConfidence: 0.20, VerificationStatus: "confirmed"},
	}, MemoryUsageStats{}, MemoryCurationOptions{ConfirmThreshold: 0.92, MaxActions: 100, Now: now})

	if len(plan.AutoArchive) != 1 {
		t.Fatalf("AutoArchive len = %d, want 1; plan=%+v", len(plan.AutoArchive), plan)
	}
	if plan.AutoArchive[0].DocID != "doc-weak" {
		t.Fatalf("archived doc = %q, want doc-weak", plan.AutoArchive[0].DocID)
	}
}

func TestBuildMemoryCurationPlanKeepsContradictionsForReview(t *testing.T) {
	now := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)
	plan := BuildMemoryCurationPlan([]MemoryMeta{{
		DocID:                "doc-conflict",
		LastAccessed:         now.Add(-90 * 24 * time.Hour).Format("2006-01-02 15:04:05"),
		ExtractionConfidence: 0.20,
		VerificationStatus:   "contradicted",
	}}, MemoryUsageStats{}, MemoryCurationOptions{ConfirmThreshold: 0.92, MaxActions: 100, Now: now})

	if len(plan.ReviewRequired) != 1 {
		t.Fatalf("ReviewRequired len = %d, want 1; plan=%+v", len(plan.ReviewRequired), plan)
	}
	if len(plan.AutoArchive) != 0 {
		t.Fatalf("contradicted memory should not be auto-archived: %+v", plan.AutoArchive)
	}
}

func TestSQLiteMemoryApplyCurationActionArchivesWithEventLog(t *testing.T) {
	stm, cleanup := newTestSQLiteMemory(t)
	defer cleanup()
	if err := stm.UpsertMemoryMetaWithDetails("doc-archive", MemoryMetaUpdate{
		ExtractionConfidence: 0.40,
		VerificationStatus:   "unverified",
		SourceReliability:    0.40,
	}); err != nil {
		t.Fatalf("UpsertMemoryMetaWithDetails: %v", err)
	}

	if err := stm.ApplyMemoryCurationAction(MemoryCurationAction{
		DocID:  "doc-archive",
		Action: MemoryCurationActionArchive,
		Reason: "stale weak memory",
	}, "system", false); err != nil {
		t.Fatalf("ApplyMemoryCurationAction: %v", err)
	}

	metas, err := stm.GetAllMemoryMeta(10, 0)
	if err != nil {
		t.Fatalf("GetAllMemoryMeta: %v", err)
	}
	if metas[0].VerificationStatus != "archived" {
		t.Fatalf("VerificationStatus = %q, want archived", metas[0].VerificationStatus)
	}
	if metas[0].ArchivedAt == "" || metas[0].ArchivedReason != "stale weak memory" {
		t.Fatalf("archive metadata not persisted: %+v", metas[0])
	}

	events, err := stm.ListMemoryCurationEvents(10)
	if err != nil {
		t.Fatalf("ListMemoryCurationEvents: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("events len = %d, want 1", len(events))
	}
	if events[0].DocID != "doc-archive" || events[0].PreviousStatus != "unverified" || events[0].NewStatus != "archived" {
		t.Fatalf("unexpected event: %+v", events[0])
	}
}
