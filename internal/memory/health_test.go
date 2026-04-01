package memory

import "testing"

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
