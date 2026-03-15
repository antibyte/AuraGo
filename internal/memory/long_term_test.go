package memory

import (
	"testing"
	"time"
)

func TestExtractSimilarityScore(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected float64
	}{
		{"standard format", "[Similarity: 0.95] some content here", 0.95},
		{"low similarity", "[Similarity: 0.32] other text", 0.32},
		{"perfect match", "[Similarity: 1.00] exact", 1.0},
		{"with domain tag", "[Similarity: 0.87] [tool_guides] docker help", 0.87},
		{"no similarity prefix", "just plain text", 0},
		{"malformed bracket", "[Similarity: bad] text", 0},
		{"empty string", "", 0},
		{"missing closing bracket", "[Similarity: 0.50 text", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractSimilarityScore(tt.input)
			if got != tt.expected {
				t.Errorf("extractSimilarityScore(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestCalculateBatchTimeout(t *testing.T) {
	tests := []struct {
		name     string
		docCount int
		minSecs  int
		maxSecs  int
	}{
		{"single doc", 1, 30, 35},
		{"10 docs", 10, 49, 51},
		{"50 docs", 50, 129, 131},
		{"100 docs", 100, 229, 231},
		{"1000 docs - capped", 1000, 299, 301},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculateBatchTimeout(tt.docCount)
			gotSecs := int(got.Seconds())
			if gotSecs < tt.minSecs || gotSecs > tt.maxSecs {
				t.Errorf("calculateBatchTimeout(%d) = %v (%ds), want between %ds and %ds",
					tt.docCount, got, gotSecs, tt.minSecs, tt.maxSecs)
			}
		})
	}

	// Cap at 5 minutes
	got := calculateBatchTimeout(10000)
	if got != 5*time.Minute {
		t.Errorf("calculateBatchTimeout(10000) = %v, want 5m0s (capped)", got)
	}
}

func TestQueryCacheEntry(t *testing.T) {
	entry := queryCacheEntry{
		embedding: []float32{0.1, 0.2, 0.3},
		timestamp: time.Now(),
	}

	if len(entry.embedding) != 3 {
		t.Errorf("expected 3 dimensions, got %d", len(entry.embedding))
	}

	if time.Since(entry.timestamp) > time.Second {
		t.Error("timestamp should be recent")
	}
}
