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
