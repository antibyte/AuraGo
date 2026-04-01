package agent

import (
	"testing"

	"aurago/internal/config"
)

func TestCollectNativePendingSummaryBatchCandidatesStopsAtFirstUnsupportedTool(t *testing.T) {
	cfg := &config.Config{}
	cfg.Tools.DDGSearch.SummaryMode = true
	cfg.Tools.Wikipedia.SummaryMode = true
	cfg.Tools.WebScraper.SummaryMode = true

	pending := []ToolCall{
		{Action: "ddg_search", NativeCallID: "call_1", Query: "backup issue"},
		{Action: "wikipedia_search", NativeCallID: "call_2", Query: "Network attached storage"},
		{Action: "manage_memory", NativeCallID: "call_3"},
		{Action: "web_scraper", NativeCallID: "call_4", URL: "https://example.com"},
	}

	got := collectNativePendingSummaryBatchCandidates(cfg, pending)
	if len(got) != 2 {
		t.Fatalf("len(candidates) = %d, want 2", len(got))
	}
	if got[0].EffectiveAction != "ddg_search" {
		t.Fatalf("candidate[0].EffectiveAction = %q", got[0].EffectiveAction)
	}
	if got[1].EffectiveAction != "wikipedia_search" {
		t.Fatalf("candidate[1].EffectiveAction = %q", got[1].EffectiveAction)
	}
}

func TestBuildPendingSummaryBatchCandidateUsesDefaultQueries(t *testing.T) {
	cfg := &config.Config{}
	cfg.Tools.PDFExtractor.SummaryMode = true

	got, ok := buildPendingSummaryBatchCandidate(cfg, ToolCall{
		Action:       "pdf_extractor",
		NativeCallID: "call_1",
		FilePath:     "docs/report.pdf",
	})
	if !ok {
		t.Fatal("expected pdf_extractor to be batch candidate")
	}
	if got.SearchQuery != "summarise the key content of this document" {
		t.Fatalf("default search query = %q", got.SearchQuery)
	}
	if got.SourceName != "PDF document" {
		t.Fatalf("source name = %q", got.SourceName)
	}
}
