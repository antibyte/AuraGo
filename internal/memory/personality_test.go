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

// ── Weighted Decay Tests ─────────────────────────────────────────────────────

func TestDecayAllTraitsWeightedHighTraitsDecaySlower(t *testing.T) {
	stm := newTestPersonalityDB(t)
	// Set curiosity high (0.9) and creativity at center area (0.55)
	_ = stm.SetTrait(TraitCuriosity, 0.9)
	_ = stm.SetTrait(TraitCreativity, 0.55)

	meta := PersonalityMeta{Volatility: 1.0, TraitDecayRate: 1.0}
	_ = stm.DecayAllTraitsWeighted(0.1, meta)

	traits, _ := stm.GetTraits()
	// Curiosity (0.9, dist=0.4 >0.2) should decay by 0.1*0.5=0.05 → 0.85
	if v := traits[TraitCuriosity]; v < 0.84 || v > 0.86 {
		t.Errorf("curiosity: expected ~0.85, got %.4f", v)
	}
	// Creativity (0.55, dist=0.05 <0.1) should decay by 0.1*1.5=0.15 → 0.50
	if v := traits[TraitCreativity]; v < 0.49 || v > 0.51 {
		t.Errorf("creativity: expected ~0.50, got %.4f", v)
	}
}

func TestDecayAllTraitsWeightedRespectsAnchors(t *testing.T) {
	stm := newTestPersonalityDB(t)
	_ = stm.SetTrait(TraitEmpathy, 0.55)

	meta := PersonalityMeta{
		Volatility:     1.0,
		TraitDecayRate: 1.0,
		AnchorTraits:   map[string]float64{TraitEmpathy: 0.55},
	}
	// Large decay should not push empathy below the anchor floor
	_ = stm.DecayAllTraitsWeighted(1.0, meta)

	traits, _ := stm.GetTraits()
	if v := traits[TraitEmpathy]; v < 0.55 {
		t.Errorf("empathy should not decay below anchor 0.55, got %.4f", v)
	}
}

func TestDecayAllTraitsWeightedRespectsDecayResistance(t *testing.T) {
	stm := newTestPersonalityDB(t)
	_ = stm.SetTrait(TraitAffinity, 0.8)
	_ = stm.SetTrait(TraitConfidence, 0.8)

	meta := PersonalityMeta{
		Volatility:      1.0,
		TraitDecayRate:  1.0,
		DecayResistance: map[string]float64{TraitAffinity: 0.5}, // 50% resistance
	}
	_ = stm.DecayAllTraitsWeighted(0.1, meta)

	traits, _ := stm.GetTraits()
	// Affinity: base decay 0.05 (high trait factor) * 0.5 (resistance) = 0.025 → ~0.775
	// Confidence: base decay 0.05 (high trait factor) * 1.0 (no resistance) = 0.05 → ~0.75
	if traits[TraitAffinity] <= traits[TraitConfidence] {
		t.Errorf("affinity (with resistance) should decay less than confidence: A=%.4f, Co=%.4f",
			traits[TraitAffinity], traits[TraitConfidence])
	}
}

func TestDecayAllTraitsWeightedSkipsLoneliness(t *testing.T) {
	stm := newTestPersonalityDB(t)
	_ = stm.SetTrait(TraitLoneliness, 0.8)

	meta := PersonalityMeta{Volatility: 1.0, TraitDecayRate: 1.0}
	_ = stm.DecayAllTraitsWeighted(1.0, meta)

	traits, _ := stm.GetTraits()
	if v := traits[TraitLoneliness]; v != 0.8 {
		t.Errorf("loneliness should not be affected by decay, expected 0.8 got %.4f", v)
	}
}

// ── Trait Bounds Tests ───────────────────────────────────────────────────────

func TestTraitBoundsSetAndGet(t *testing.T) {
	stm := newTestPersonalityDB(t)
	err := stm.SetTraitBound(TraitCuriosity, 0.55, 1.0, 0.5)
	if err != nil {
		t.Fatalf("SetTraitBound: %v", err)
	}

	bounds := stm.GetAllTraitBounds()
	b, ok := bounds[TraitCuriosity]
	if !ok {
		t.Fatal("expected curiosity bounds")
	}
	if b.Floor != 0.55 || b.Ceiling != 1.0 || b.DecayResistance != 0.5 {
		t.Errorf("unexpected bounds: %+v", b)
	}
}

func TestTraitBoundsUpsertTakesHigherFloor(t *testing.T) {
	stm := newTestPersonalityDB(t)
	_ = stm.SetTraitBound(TraitCuriosity, 0.3, 1.0, 0.8)
	_ = stm.SetTraitBound(TraitCuriosity, 0.55, 1.0, 0.5)

	bounds := stm.GetAllTraitBounds()
	if b := bounds[TraitCuriosity]; b.Floor != 0.55 {
		t.Errorf("expected floor to be MAX(0.3, 0.55)=0.55, got %.2f", b.Floor)
	}
	if b := bounds[TraitCuriosity]; b.DecayResistance != 0.5 {
		t.Errorf("expected decay_resistance to be MIN(0.8, 0.5)=0.5, got %.2f", b.DecayResistance)
	}
}

func TestDecayRespectsTraitBoundsFromDB(t *testing.T) {
	stm := newTestPersonalityDB(t)
	_ = stm.SetTrait(TraitCuriosity, 0.6)
	_ = stm.SetTraitBound(TraitCuriosity, 0.6, 1.0, 1.0) // floor at 0.6

	meta := PersonalityMeta{Volatility: 1.0, TraitDecayRate: 1.0}
	_ = stm.DecayAllTraitsWeighted(1.0, meta)

	traits, _ := stm.GetTraits()
	if v := traits[TraitCuriosity]; v < 0.6 {
		t.Errorf("curiosity should not decay below DB floor 0.6, got %.4f", v)
	}
}

// ── Milestone Effect Tests ───────────────────────────────────────────────────

func TestApplyMilestoneEffectSetsTraitBounds(t *testing.T) {
	stm := newTestPersonalityDB(t)
	err := ApplyMilestoneEffect(stm, "Deep Explorer")
	if err != nil {
		t.Fatalf("ApplyMilestoneEffect: %v", err)
	}

	bounds := stm.GetAllTraitBounds()
	b, ok := bounds[TraitCuriosity]
	if !ok {
		t.Fatal("expected curiosity bounds after Deep Explorer milestone")
	}
	if b.Floor < 0.55 {
		t.Errorf("expected curiosity floor >= 0.55, got %.2f", b.Floor)
	}
	if b.DecayResistance > 0.5 {
		t.Errorf("expected curiosity decay resistance <= 0.5, got %.2f", b.DecayResistance)
	}
}

func TestApplyMilestoneEffectUnknownLabel(t *testing.T) {
	stm := newTestPersonalityDB(t)
	// Unknown milestone should not error
	err := ApplyMilestoneEffect(stm, "Nonexistent Milestone")
	if err != nil {
		t.Errorf("unexpected error for unknown milestone: %v", err)
	}
}

// TestDecayAllTraitsWeightedIsAtomic verifies that all trait updates from a single
// DecayAllTraitsWeighted call are applied atomically: either all traits are updated
// or none are (transaction safety). This test checks the observable outcome — that
// all traits shift consistently in the same call.
func TestDecayAllTraitsWeightedIsAtomic(t *testing.T) {
	stm := newTestPersonalityDB(t)

	// Set multiple traits above 0.5 so they all experience decay
	traitVals := map[string]float64{
		TraitCuriosity:    0.8,
		TraitThoroughness: 0.7,
		TraitCreativity:   0.75,
		TraitEmpathy:      0.65,
		TraitConfidence:   0.6,
		TraitAffinity:     0.7,
	}
	for trait, val := range traitVals {
		if err := stm.SetTrait(trait, val); err != nil {
			t.Fatalf("SetTrait(%s): %v", trait, err)
		}
	}

	meta := PersonalityMeta{Volatility: 1.0, TraitDecayRate: 1.0}
	if err := stm.DecayAllTraitsWeighted(0.05, meta); err != nil {
		t.Fatalf("DecayAllTraitsWeighted: %v", err)
	}

	traits, err := stm.GetTraits()
	if err != nil {
		t.Fatalf("GetTraits: %v", err)
	}
	// All non-loneliness traits that were above 0.5 should now be lower
	for trait, before := range traitVals {
		after := traits[trait]
		if after >= before {
			t.Errorf("trait %s was not decayed: before=%.4f after=%.4f", trait, before, after)
		}
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
