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
		},
		{
			DocID:                "doc-good",
			AccessCount:          4,
			LastAccessed:         "2026-03-20 00:00:00",
			ExtractionConfidence: 0.91,
			VerificationStatus:   "confirmed",
			SourceReliability:    0.92,
		},
		{
			DocID:                "doc-conflict",
			AccessCount:          1,
			LastAccessed:         "2026-02-01 00:00:00",
			ExtractionConfidence: 0.65,
			VerificationStatus:   "contradicted",
			SourceReliability:    0.80,
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
	if report.Curator.OverusedMemories != 1 {
		t.Fatalf("expected 1 overused memory, got %d", report.Curator.OverusedMemories)
	}
	if len(report.Curator.Suggestions) == 0 {
		t.Fatal("expected curator suggestions")
	}
}
