package memory

import (
	"strings"
	"testing"
)

func TestPersonalityIntegrityZeroModifiersStayZero(t *testing.T) {
	meta := PersonalityMeta{Volatility: 0, EmpathyBias: 0, LonelinessSusceptibility: 0, TraitDecayRate: 1}.Normalized()
	if meta.Volatility != 0 || meta.EmpathyBias != 0 || meta.LonelinessSusceptibility != 0 {
		t.Fatalf("zero modifiers changed: %+v", meta)
	}
	if again := meta.Normalized(); again.Volatility != 0 || again.LonelinessSusceptibility != 0 {
		t.Fatal("normalization not idempotent")
	}
}

func TestPersonalityIntegrityRuntimeDoesNotReplaceVoice(t *testing.T) {
	db := newTestPersonalityDB(t)
	for _, trait := range []string{TraitLoneliness, TraitAffinity, TraitEmpathy, TraitCuriosity} {
		if err := db.SetTrait(trait, .95); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.LogMood(MoodFrustrated, "test"); err != nil {
		t.Fatal(err)
	}
	for _, affinity := range []float64{.1, .95} {
		if err := db.SetTrait(TraitAffinity, affinity); err != nil {
			t.Fatal(err)
		}
		line := db.GetPersonalityLineWithMeta(true, PersonalityMeta{Volatility: .1, LonelinessSusceptibility: 0})
		for _, forbidden := range []string{"extremely informal", "highly professional, formal", "ask the user for clarification before retrying", "missed them", "without the user", "explore tangents", "welcome back", "welcoming greeting"} {
			if strings.Contains(line, forbidden) {
				t.Errorf("state overrides persona or task: %q", forbidden)
			}
		}
	}
	if line := db.GetPersonalityLineWithMeta(true, DefaultPersonalityMeta()); !strings.Contains(line, "welcome back") {
		t.Fatal("enabled loneliness no longer permits a welcome")
	}
}
