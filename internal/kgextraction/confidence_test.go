package kgextraction

import (
	"testing"
)

func TestComputeConfidence_BaseCase(t *testing.T) {
	// Neutral input: moderate content, no file, reasonable extraction.
	score := ComputeConfidence(ConfidenceInput{
		SourceType:    "auto_extraction",
		ContentLength: 300,
		NodeCount:     3,
		EdgeCount:     2,
	})
	if score < 0.0 || score > 1.0 {
		t.Errorf("confidence %v out of [0,1] range", score)
	}
	// 300 chars → +0.0 (100-499 range), density=10.0/1000 → no adjustment (not >10, not 1-5),
	// ratio=2/3≈0.67 → +0.05
	// Expected: 0.50 + 0.00 + 0.00 + 0.05 = 0.55
	if score != 0.55 {
		t.Errorf("expected 0.55, got %.2f", score)
	}
}

func TestComputeConfidence_LongStructuredContent(t *testing.T) {
	// Long Markdown content with reasonable entity density.
	score := ComputeConfidence(ConfidenceInput{
		SourceType:    "file_sync",
		FilePath:      "/docs/architecture.md",
		ContentLength: 5000,
		NodeCount:     8,
		EdgeCount:     6,
	})
	// 5000 chars → +0.20, .md → +0.10, density=1.6/1000 → +0.05, ratio=6/8=0.75 → +0.05
	// Expected: 0.50 + 0.20 + 0.10 + 0.05 + 0.05 = 0.90
	if score != 0.90 {
		t.Errorf("expected 0.90, got %.2f", score)
	}
}

func TestComputeConfidence_ShortUnstructuredTruncated(t *testing.T) {
	// Short PDF content that was truncated, with many entities.
	score := ComputeConfidence(ConfidenceInput{
		SourceType:    "file_sync",
		FilePath:      "/docs/scan.pdf",
		ContentLength: 80,
		NodeCount:     5,
		EdgeCount:     1,
		WasTruncated:  true,
	})
	// 80 chars → -0.20, .pdf → -0.10, truncated → -0.15, density=62.5/1000 → -0.15
	// Expected: 0.50 - 0.20 - 0.10 - 0.15 - 0.15 = -0.10 → clamped to 0.00
	if score != 0.00 {
		t.Errorf("expected 0.00 (clamped), got %.2f", score)
	}
}

func TestComputeConfidence_JSONFileHighConfidence(t *testing.T) {
	// Structured JSON file with good content length.
	score := ComputeConfidence(ConfidenceInput{
		SourceType:    "file_sync",
		FilePath:      "/data/config.json",
		ContentLength: 2000,
		NodeCount:     4,
		EdgeCount:     3,
	})
	// 2000 chars → +0.20, .json → +0.15, density=2/1000 → +0.05, ratio=3/4=0.75 → +0.05
	// Expected: 0.50 + 0.20 + 0.15 + 0.05 + 0.05 = 0.95
	if score != 0.95 {
		t.Errorf("expected 0.95, got %.2f", score)
	}
}

func TestComputeConfidence_HighDensityPenalty(t *testing.T) {
	// Many entities from moderate content → hallucination signal.
	score := ComputeConfidence(ConfidenceInput{
		SourceType:    "file_sync",
		FilePath:      "/docs/notes.txt",
		ContentLength: 200,
		NodeCount:     15,
		EdgeCount:     2,
	})
	// 200 chars → +0.0, .txt → +0.0, density=75/1000 → -0.15, ratio=2/15≈0.13 → +0.0
	// Expected: 0.50 + 0.00 + 0.00 - 0.15 + 0.00 = 0.35
	if score != 0.35 {
		t.Errorf("expected 0.35, got %.2f", score)
	}
}

func TestComputeConfidence_TruncationPenalty(t *testing.T) {
	// Same as LongStructuredContent but truncated.
	score := ComputeConfidence(ConfidenceInput{
		SourceType:    "file_sync",
		FilePath:      "/docs/architecture.md",
		ContentLength: 5000,
		NodeCount:     8,
		EdgeCount:     6,
		WasTruncated:  true,
	})
	// 0.90 (from LongStructuredContent) - 0.15 = 0.75
	if score != 0.75 {
		t.Errorf("expected 0.75, got %.2f", score)
	}
}

func TestComputeConfidence_NoEntities(t *testing.T) {
	// No entities extracted — confidence should be base.
	score := ComputeConfidence(ConfidenceInput{
		SourceType:    "file_sync",
		FilePath:      "/docs/readme.txt",
		ContentLength: 1000,
		NodeCount:     0,
		EdgeCount:     0,
	})
	// 1000 chars → +0.10, .txt → +0.0, no density check (NodeCount=0), no ratio check
	// Expected: 0.50 + 0.10 = 0.60
	if score != 0.60 {
		t.Errorf("expected 0.60, got %.2f", score)
	}
}

func TestComputeConfidence_ConversationExtraction(t *testing.T) {
	// Conversation-based extraction has no FilePath.
	score := ComputeConfidence(ConfidenceInput{
		SourceType:    "auto_extraction",
		ContentLength: 1500,
		NodeCount:     5,
		EdgeCount:     4,
	})
	// 1500 chars → +0.10, no file → +0.0, density=3.33/1000 → +0.05, ratio=4/5=0.8 → +0.05
	// Expected: 0.50 + 0.10 + 0.05 + 0.05 = 0.70
	if score != 0.70 {
		t.Errorf("expected 0.70, got %.2f", score)
	}
}

func TestComputeConfidence_ClampedUpperBound(t *testing.T) {
	// Extreme case: everything maxed out.
	score := ComputeConfidence(ConfidenceInput{
		SourceType:    "file_sync",
		FilePath:      "/data/config.yaml",
		ContentLength: 10000,
		NodeCount:     3,
		EdgeCount:     3,
	})
	// 10000 chars → +0.20, .yaml → +0.15, density=0.3/1000 → +0.0 (below 1), ratio=1.0 → +0.05
	// Expected: 0.50 + 0.20 + 0.15 + 0.05 = 0.90
	if score != 0.90 {
		t.Errorf("expected 0.90, got %.2f", score)
	}
}

func TestComputeConfidence_ClampedLowerBound(t *testing.T) {
	// Worst case: very short, PDF, truncated, high density.
	score := ComputeConfidence(ConfidenceInput{
		SourceType:    "file_sync",
		FilePath:      "/docs/bad.pdf",
		ContentLength: 50,
		NodeCount:     10,
		EdgeCount:     0,
		WasTruncated:  true,
	})
	if score != 0.00 {
		t.Errorf("expected 0.00 (clamped lower), got %.2f", score)
	}
}

func TestFormatConfidence(t *testing.T) {
	tests := []struct {
		input    float64
		expected string
	}{
		{0.50, "0.50"},
		{0.123, "0.12"},
		{0.999, "1.00"},
		{0.0, "0.00"},
		{-0.5, "0.00"},
		{1.5, "1.00"},
	}
	for _, tc := range tests {
		result := FormatConfidence(tc.input)
		if result != tc.expected {
			t.Errorf("FormatConfidence(%v) = %q, want %q", tc.input, result, tc.expected)
		}
	}
}

func TestParseConfidence(t *testing.T) {
	tests := []struct {
		input    string
		expected float64
	}{
		{"0.50", 0.50},
		{"0.75", 0.75},
		{"1.00", 1.00},
		{"0.00", 0.00},
		{"invalid", 0.0},
		{"", 0.0},
		{"-0.5", 0.0}, // clamped
		{"1.5", 1.0},  // clamped
	}
	for _, tc := range tests {
		result := ParseConfidence(tc.input)
		if result != tc.expected {
			t.Errorf("ParseConfidence(%q) = %v, want %v", tc.input, result, tc.expected)
		}
	}
}

func TestFormatParseRoundTrip(t *testing.T) {
	values := []float64{0.0, 0.15, 0.33, 0.50, 0.75, 0.95, 1.0}
	for _, v := range values {
		formatted := FormatConfidence(v)
		parsed := ParseConfidence(formatted)
		if parsed != v {
			t.Errorf("round-trip failed: %v → %q → %v", v, formatted, parsed)
		}
	}
}
