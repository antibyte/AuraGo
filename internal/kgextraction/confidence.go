// Package kgextraction provides confidence scoring for KG extraction results.
//
// The confidence score is a heuristic value in [0.0, 1.0] that reflects how
// trustworthy an extraction result is likely to be. It is stored as a
// "confidence" property on nodes and edges so that downstream consumers can
// filter or prioritize accordingly.
//
// Heuristics (no ML, no LLM calls):
//   - Content length: longer input → more context → higher confidence
//   - File type: structured formats (Markdown, YAML, JSON) yield better results
//   - Truncation: content that was truncated loses confidence
//   - Entity density: too many entities from short content suggests hallucination
//   - Edge-to-node ratio: well-connected extractions are more reliable
package kgextraction

import (
	"fmt"
	"math"
	"path/filepath"
	"strconv"
	"strings"
)

// ConfidenceInput holds the metadata needed to compute an extraction confidence score.
type ConfidenceInput struct {
	// SourceType is the extraction source, e.g. "file_sync" or "auto_extraction".
	SourceType string

	// FilePath is the original file path (used for file extension heuristics).
	// Empty for conversation-based extraction.
	FilePath string

	// ContentLength is the character count of the input text sent to the LLM.
	ContentLength int

	// NodeCount is the number of entities extracted.
	NodeCount int

	// EdgeCount is the number of relationships extracted.
	EdgeCount int

	// WasTruncated indicates whether the input content was truncated before extraction.
	WasTruncated bool
}

// ComputeConfidence returns a heuristic confidence score in [0.0, 1.0].
//
// The scoring model uses simple additive adjustments from a neutral baseline:
//
//	Base: 0.50
//	+ Content length bonus/penalty     (±0.20)
//	+ File type bonus/penalty          (±0.15)
//	+ Truncation penalty               (-0.15)
//	+ Entity density penalty           (±0.15)
//	+ Edge-to-node ratio bonus         (+0.05)
//
// The result is clamped to [0.0, 1.0] and rounded to two decimal places.
func ComputeConfidence(input ConfidenceInput) float64 {
	confidence := 0.50

	// Factor 1: Content length — more context yields better extraction.
	switch {
	case input.ContentLength >= 2000:
		confidence += 0.20
	case input.ContentLength >= 500:
		confidence += 0.10
	case input.ContentLength < 100:
		confidence -= 0.20
	}

	// Factor 2: File type — structured formats yield cleaner extraction.
	if input.FilePath != "" {
		ext := strings.ToLower(filepath.Ext(input.FilePath))
		switch ext {
		case ".md":
			confidence += 0.10
		case ".json", ".yaml", ".yml", ".toml":
			confidence += 0.15
		case ".pdf", ".docx":
			confidence -= 0.10
		}
	}

	// Factor 3: Truncation penalty — partial content may miss important context.
	if input.WasTruncated {
		confidence -= 0.15
	}

	// Factor 4: Entity density — too many entities from short content is suspicious.
	if input.ContentLength > 0 && input.NodeCount > 0 {
		density := float64(input.NodeCount) / float64(input.ContentLength) * 1000
		switch {
		case density > 20:
			confidence -= 0.15
		case density > 10:
			confidence -= 0.05
		case density >= 1 && density <= 5:
			confidence += 0.05
		}
	}

	// Factor 5: Edge-to-node ratio — connected entities suggest coherent extraction.
	if input.NodeCount > 0 {
		ratio := float64(input.EdgeCount) / float64(input.NodeCount)
		if ratio >= 0.5 {
			confidence += 0.05
		}
	}

	return clampConfidence(confidence)
}

// FormatConfidence converts a float64 confidence score to a string suitable
// for storage as a node/edge property. Two decimal places are used.
func FormatConfidence(score float64) string {
	return fmt.Sprintf("%.2f", clampConfidence(score))
}

// ParseConfidence parses a confidence property string back to float64.
// Returns 0.0 on parse failure.
func ParseConfidence(s string) float64 {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0.0
	}
	return clampConfidence(f)
}

// clampConfidence ensures the score is within [0.0, 1.0] and rounds to 2 decimals.
func clampConfidence(v float64) float64 {
	if v < 0.0 {
		v = 0.0
	}
	if v > 1.0 {
		v = 1.0
	}
	return math.Round(v*100) / 100
}
