package agent

import "testing"

func TestDeriveConflictSignalsDetectsDifferentLanguageClaims(t *testing.T) {
	signals := deriveConflictSignals("User prefers German")
	if len(signals) != 1 {
		t.Fatalf("len(signals) = %d, want 1", len(signals))
	}
	if signals[0].Key != "user|preference" || signals[0].Value != "german" {
		t.Fatalf("unexpected signal: %+v", signals[0])
	}
}

func TestNormalizeConflictTextKeepsLegitimateBracketContent(t *testing.T) {
	input := `Alice prefers JSON ["home","lab"] backups`

	got := normalizeConflictText(input)

	if got != `Alice prefers JSON ["home","lab"] backups` {
		t.Fatalf("normalizeConflictText() = %q, want original content preserved", got)
	}
}

func TestNormalizeConflictTextStripsKnownSimilarityPrefix(t *testing.T) {
	input := `[Similarity: 0.87] Alice prefers rsync backups`

	got := normalizeConflictText(input)

	if got != "Alice prefers rsync backups" {
		t.Fatalf("normalizeConflictText() = %q, want prefix stripped", got)
	}
}
