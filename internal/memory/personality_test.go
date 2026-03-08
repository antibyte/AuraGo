package memory

import (
	"log/slog"
	"os"
	"testing"
)

func newTestPersonalityDB(t *testing.T) *SQLiteMemory {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	stm, err := NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	if err := stm.InitPersonalityTables(); err != nil {
		t.Fatalf("InitPersonalityTables: %v", err)
	}
	t.Cleanup(func() { stm.Close() })
	return stm
}

// ── Trait Tests ──────────────────────────────────────────────────────────────

func TestGetTraitsDefaults(t *testing.T) {
	stm := newTestPersonalityDB(t)
	traits, err := stm.GetTraits()
	if err != nil {
		t.Fatalf("GetTraits: %v", err)
	}
	if len(traits) != 7 {
		t.Fatalf("expected 7 traits, got %d", len(traits))
	}
	for _, trait := range []string{TraitCuriosity, TraitThoroughness, TraitCreativity, TraitEmpathy, TraitConfidence, TraitAffinity} {
		if v := traits[trait]; v != 0.5 {
			t.Errorf("trait %s: expected 0.5, got %.2f", trait, v)
		}
	}
	// Loneliness starts at 0.0
	if v := traits[TraitLoneliness]; v != 0.0 {
		t.Errorf("trait %s: expected 0.0, got %.2f", TraitLoneliness, v)
	}
}

func TestUpdateTraitClamp(t *testing.T) {
	stm := newTestPersonalityDB(t)
	// Push curiosity above 1.0
	_ = stm.UpdateTrait(TraitCuriosity, +0.8)
	traits, _ := stm.GetTraits()
	if v := traits[TraitCuriosity]; v > 1.0 {
		t.Errorf("curiosity should clamp at 1.0, got %.2f", v)
	}
	// Push confidence below 0.0
	_ = stm.UpdateTrait(TraitConfidence, -0.9)
	traits, _ = stm.GetTraits()
	if v := traits[TraitConfidence]; v < 0.0 {
		t.Errorf("confidence should clamp at 0.0, got %.2f", v)
	}
}

func TestDecayAllTraits(t *testing.T) {
	stm := newTestPersonalityDB(t)
	_ = stm.UpdateTrait(TraitCuriosity, +0.3)  // 0.8
	_ = stm.UpdateTrait(TraitConfidence, -0.3) // 0.2
	_ = stm.DecayAllTraits(0.1)
	traits, _ := stm.GetTraits()
	// Curiosity was 0.8, decay 0.1 → 0.7
	if v := traits[TraitCuriosity]; v < 0.69 || v > 0.71 {
		t.Errorf("curiosity after decay: expected ~0.7, got %.2f", v)
	}
	// Confidence was 0.2, decay 0.1 → 0.3
	if v := traits[TraitConfidence]; v < 0.29 || v > 0.31 {
		t.Errorf("confidence after decay: expected ~0.3, got %.2f", v)
	}
}

// ── Mood Tests ───────────────────────────────────────────────────────────────

func TestLogAndGetMood(t *testing.T) {
	stm := newTestPersonalityDB(t)
	// Default (no entries)
	if m := stm.GetCurrentMood(); m != MoodCurious {
		t.Errorf("default mood: expected curious, got %s", m)
	}
	_ = stm.LogMood(MoodPlayful, "haha")
	if m := stm.GetCurrentMood(); m != MoodPlayful {
		t.Errorf("after log: expected playful, got %s", m)
	}
}

// ── Milestone Tests ──────────────────────────────────────────────────────────

func TestAddAndGetMilestones(t *testing.T) {
	stm := newTestPersonalityDB(t)
	_ = stm.AddMilestone("Deep Explorer", "curiosity above 0.90")
	ms, err := stm.GetMilestones(5)
	if err != nil {
		t.Fatalf("GetMilestones: %v", err)
	}
	if len(ms) != 1 {
		t.Fatalf("expected 1 milestone, got %d", len(ms))
	}
	if !contains(ms[0], "Deep Explorer") {
		t.Errorf("milestone text should contain 'Deep Explorer': %s", ms[0])
	}
}

func TestCheckMilestonesTriggered(t *testing.T) {
	traits := PersonalityTraits{
		TraitCuriosity:    0.95,
		TraitThoroughness: 0.5,
		TraitCreativity:   0.5,
		TraitEmpathy:      0.5,
		TraitConfidence:   0.10,
	}
	triggered := CheckMilestones(traits)
	labels := make(map[string]bool)
	for _, m := range triggered {
		labels[m.Label] = true
	}
	if !labels["Deep Explorer"] {
		t.Error("expected 'Deep Explorer' milestone")
	}
	if !labels["Crisis of Confidence"] {
		t.Error("expected 'Crisis of Confidence' milestone")
	}
}

// ── DetectMood Tests ─────────────────────────────────────────────────────────

func detectMoodDefault(msg, result string) (Mood, map[string]float64) {
	return DetectMood(msg, result, PersonalityMeta{Volatility: 1.0, EmpathyBias: 1.0, ConflictResponse: "neutral"})
}

func TestDetectMoodPlayful(t *testing.T) {
	tests := []string{"haha das ist lustig", "lol nice one", "mdr c'est marrant", "jaja buenísimo", "kkk engraçado", "grappig!"}
	for _, msg := range tests {
		mood, _ := detectMoodDefault(msg, "")
		if mood != MoodPlayful {
			t.Errorf("detectMoodDefault(%q) = %s, want playful", msg, mood)
		}
	}
}

func TestDetectMoodCautious(t *testing.T) {
	tests := []string{"das ist falsch!", "that's wrong", "c'est faux", "esto está mal", "sbagliato!", "dat is fout", "det er forkert"}
	for _, msg := range tests {
		mood, _ := detectMoodDefault(msg, "")
		if mood != MoodCautious {
			t.Errorf("detectMoodDefault(%q) = %s, want cautious", msg, mood)
		}
	}
}

func TestDetectMoodCautiousFromToolError(t *testing.T) {
	mood, deltas := detectMoodDefault("run my script", "[EXECUTION ERROR] something broke")
	if mood != MoodCautious {
		t.Errorf("expected cautious on tool error, got %s", mood)
	}
	if deltas[TraitConfidence] >= 0 {
		t.Error("confidence should decrease on error")
	}
}

func TestDetectMoodCreative(t *testing.T) {
	tests := []string{"ich hab eine idee", "let's brainstorm", "j'ai une idée créatif", "vamos a diseñar", "laten we ontwerpen"}
	for _, msg := range tests {
		mood, _ := detectMoodDefault(msg, "")
		if mood != MoodCreative {
			t.Errorf("detectMoodDefault(%q) = %s, want creative", msg, mood)
		}
	}
}

func TestDetectMoodAnalytical(t *testing.T) {
	tests := []string{"warum funktioniert das?", "why does this work?", "pourquoi ça marche?", "por qué funciona?", "waarom werkt dit?"}
	for _, msg := range tests {
		mood, _ := detectMoodDefault(msg, "")
		if mood != MoodAnalytical {
			t.Errorf("detectMoodDefault(%q) = %s, want analytical", msg, mood)
		}
	}
}

func TestDetectMoodCurious(t *testing.T) {
	tests := []string{"was ist kubernetes?", "how does docker work?", "qu'est-ce que python?", "hvad er rust?", "vad är golang?"}
	for _, msg := range tests {
		mood, _ := detectMoodDefault(msg, "")
		if mood != MoodCurious {
			t.Errorf("detectMoodDefault(%q) = %s, want curious", msg, mood)
		}
	}
}

func TestDetectMoodPositiveEmoji(t *testing.T) {
	mood, _ := detectMoodDefault("👍", "")
	if mood != MoodFocused {
		t.Errorf("expected focused for positive emoji feedback, got %s", mood)
	}
}

func TestDetectMoodNegativeEmoji(t *testing.T) {
	mood, _ := detectMoodDefault("👎", "")
	if mood != MoodCautious {
		t.Errorf("expected cautious for negative emoji, got %s", mood)
	}
}

func TestDetectMoodShortFeedback(t *testing.T) {
	// Short positive-ish messages without '?' = focused
	mood, _ := detectMoodDefault("ok", "")
	if mood != MoodFocused {
		t.Errorf("expected focused for short feedback 'ok', got %s", mood)
	}
}

func TestGetPersonalityLine(t *testing.T) {
	stm := newTestPersonalityDB(t)
	line := stm.GetPersonalityLine(false)
	if !contains(line, "[Self: mood=") {
		t.Errorf("unexpected personality line: %s", line)
	}
	if !contains(line, "C:0.50") {
		t.Errorf("expected default trait value in line: %s", line)
	}
}

// helper
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
