package agent

import "testing"

func TestMergeHelperLLMRuntimeStatsAccumulatesCounts(t *testing.T) {
	ResetHelperLLMRuntimeStats()
	t.Cleanup(ResetHelperLLMRuntimeStats)

	MergeHelperLLMRuntimeStats("content_summaries", HelperLLMOperationStats{
		Requests:     1,
		LLMCalls:     1,
		BatchedItems: 2,
		SavedCalls:   1,
		LastDetail:   "llm_call",
	})
	MergeHelperLLMRuntimeStats("content_summaries", HelperLLMOperationStats{
		Requests:     1,
		CacheHits:    1,
		BatchedItems: 2,
		SavedCalls:   1,
		LastDetail:   "cache_hit",
	})

	snapshot := SnapshotHelperLLMRuntimeStats()
	stats, ok := snapshot.Operations["content_summaries"]
	if !ok {
		t.Fatal("missing content_summaries stats")
	}
	if stats.Requests != 2 {
		t.Fatalf("requests = %d, want 2", stats.Requests)
	}
	if stats.LLMCalls != 1 {
		t.Fatalf("llm_calls = %d, want 1", stats.LLMCalls)
	}
	if stats.CacheHits != 1 {
		t.Fatalf("cache_hits = %d, want 1", stats.CacheHits)
	}
	if stats.BatchedItems != 4 {
		t.Fatalf("batched_items = %d, want 4", stats.BatchedItems)
	}
	if stats.SavedCalls != 2 {
		t.Fatalf("saved_calls = %d, want 2", stats.SavedCalls)
	}
	if stats.LastDetail != "cache_hit" {
		t.Fatalf("last_detail = %q, want cache_hit", stats.LastDetail)
	}
	if snapshot.UpdatedAt.IsZero() {
		t.Fatal("expected updated_at to be set")
	}
}
