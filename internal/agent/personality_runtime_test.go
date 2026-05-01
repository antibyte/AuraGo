package agent

import (
	"log/slog"
	"os"
	"testing"

	"aurago/internal/config"
	"aurago/internal/memory"

	"github.com/sashabaranov/go-openai"
)

func newTestPersonalityRuntimeMemory(t *testing.T) *memory.SQLiteMemory {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	if err := stm.InitEmotionTables(); err != nil {
		t.Fatalf("InitEmotionTables: %v", err)
	}
	if err := stm.InitPersonalityTables(); err != nil {
		t.Fatalf("InitPersonalityTables: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })
	return stm
}

func TestLatestEmotionDescriptionFallsBackToPersistedHistory(t *testing.T) {
	stm := newTestPersonalityRuntimeMemory(t)
	if err := stm.InsertEmotionHistory("I feel calm and ready to help.", "focused", "user asked a clear question"); err != nil {
		t.Fatalf("InsertEmotionHistory: %v", err)
	}

	got := latestEmotionDescription(stm, nil)
	if got != "I feel calm and ready to help." {
		t.Fatalf("latestEmotionDescription() = %q, want persisted emotion", got)
	}
}

func TestResolvePersonalityModelPrefersHelperLLM(t *testing.T) {
	cfg := &config.Config{}
	cfg.LLM.Model = "main-model"
	cfg.Personality.V2Model = "v2-model"
	cfg.LLM.HelperEnabled = true
	cfg.LLM.HelperResolvedModel = "helper-model"

	if got := resolvePersonalityModel(cfg); got != "helper-model" {
		t.Fatalf("resolvePersonalityModel() = %q, want %q", got, "helper-model")
	}
}

func TestResolvePersonalityAnalyzerClientPrefersHelperLLM(t *testing.T) {
	cfg := &config.Config{}
	cfg.LLM.HelperEnabled = true
	cfg.LLM.HelperProviderType = "custom"
	cfg.LLM.HelperBaseURL = "https://example.com/v1"
	cfg.LLM.HelperAPIKey = "helper-key"
	cfg.LLM.HelperResolvedModel = "helper-model"

	fallback := &fakeActivityDigestClient{}
	got := resolvePersonalityAnalyzerClient(cfg, fallback)
	if got == nil {
		t.Fatal("resolvePersonalityAnalyzerClient() = nil, want helper client")
	}
	if got == fallback {
		t.Fatal("resolvePersonalityAnalyzerClient() returned fallback client, want helper client")
	}
}

func TestNormalizeHelperTurnPersonalityResultDefaultsMissingOptionalMoodFields(t *testing.T) {
	got, ok := normalizeHelperTurnPersonalityResult(helperTurnPersonalityBlock{}, memory.PersonalityMeta{})
	if !ok {
		t.Fatal("normalizeHelperTurnPersonalityResult() rejected missing helper mood fields, want neutral defaults")
	}
	if got.Mood != memory.MoodFocused {
		t.Fatalf("Mood = %q, want %q", got.Mood, memory.MoodFocused)
	}
	if got.AffinityDelta != 0 {
		t.Fatalf("AffinityDelta = %v, want 0", got.AffinityDelta)
	}
}

func TestDampenTraitDeltaReducesNearExtremes(t *testing.T) {
	if got := dampenTraitDelta(0.5, 0.1); got != 0.1 {
		t.Errorf("dampenTraitDelta(0.5, 0.1) = %.4f, want 0.1", got)
	}

	got := dampenTraitDelta(0.9, 0.1)
	if got < 0.019 || got > 0.021 {
		t.Errorf("dampenTraitDelta(0.9, 0.1) = %.4f, want ~0.020", got)
	}

	got = dampenTraitDelta(0.1, -0.1)
	if got < -0.021 || got > -0.019 {
		t.Errorf("dampenTraitDelta(0.1, -0.1) = %.4f, want ~-0.020", got)
	}

	got = dampenTraitDelta(0.99, 0.1)
	if got < 0.0019 || got > 0.0021 {
		t.Errorf("dampenTraitDelta(0.99, 0.1) = %.4f, want ~0.002", got)
	}
	if 0.99+got >= 1.0 {
		t.Errorf("dampenTraitDelta(0.99, 0.1) would saturate trait to %.4f", 0.99+got)
	}

	got = dampenTraitDelta(1.0, 0.1)
	if got != 0 {
		t.Errorf("dampenTraitDelta(1.0, 0.1) = %.4f, want 0", got)
	}

	if got := dampenTraitDelta(0.9, -0.1); got != -0.1 {
		t.Errorf("dampenTraitDelta(0.9, -0.1) = %.4f, want -0.1 for center-bound movement", got)
	}
	if got := dampenTraitDelta(0.1, 0.1); got != 0.1 {
		t.Errorf("dampenTraitDelta(0.1, 0.1) = %.4f, want 0.1 for center-bound movement", got)
	}
}

func TestApplyPersonalityV2AnalysisResultPersistsPrecomputedBatch(t *testing.T) {
	stm := newTestPersonalityRuntimeMemory(t)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	cfg := &config.Config{}
	cfg.Personality.UserProfiling = true
	cfg.Personality.EmotionSynthesizer.TriggerAlways = true

	es := memory.NewEmotionSynthesizer(nil, "", 60, 100, "English", logger)
	result := personalityV2AnalysisResult{
		Mood:          memory.MoodFocused,
		AffinityDelta: 0.04,
		TraitDeltas: map[string]float64{
			memory.TraitEmpathy: 0.03,
		},
		ProfileUpdates: []memory.ProfileUpdate{
			{Category: "tech", Key: "language", Value: "go"},
		},
		SynthesizedEmotion: &memory.EmotionState{
			Description:              "I feel calm and ready to help.",
			PrimaryMood:              memory.MoodFocused,
			SecondaryMood:            "steady",
			Valence:                  0.1,
			Arousal:                  0.3,
			Confidence:               0.8,
			Cause:                    "clear technical request",
			RecommendedResponseStyle: "calm_and_clear",
		},
	}

	applyPersonalityV2AnalysisResult(
		"default",
		cfg,
		logger,
		stm,
		es,
		nil,
		"user asked for a focused fix",
		memory.EmotionTriggerConversation,
		"",
		0,
		true,
		0,
		1,
		result,
	)

	if got := stm.GetCurrentMood(); got != memory.MoodFocused {
		t.Fatalf("GetCurrentMood() = %q, want %q", got, memory.MoodFocused)
	}
	if last := es.GetLastEmotion(); last == nil || last.Description != "I feel calm and ready to help." {
		t.Fatalf("GetLastEmotion() = %#v, want persisted precomputed emotion", last)
	}
	entries, err := stm.GetProfileEntries("tech")
	if err != nil {
		t.Fatalf("GetProfileEntries: %v", err)
	}
	if len(entries) == 0 || entries[0].Key != "language" || entries[0].Value != "go" {
		t.Fatalf("profile entries = %#v", entries)
	}
}

func TestApplyPersonalityV2AnalysisResultDampensAffinityNearCeiling(t *testing.T) {
	stm := newTestPersonalityRuntimeMemory(t)
	if err := stm.SetTrait(memory.TraitAffinity, 0.99); err != nil {
		t.Fatalf("SetTrait affinity: %v", err)
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	applyPersonalityV2AnalysisResult(
		"default",
		&config.Config{},
		logger,
		stm,
		nil,
		nil,
		"positive feedback",
		memory.EmotionTriggerConversation,
		"",
		0,
		false,
		0,
		1,
		personalityV2AnalysisResult{
			Mood:          memory.MoodFocused,
			AffinityDelta: 0.1,
		},
	)

	traits, err := stm.GetTraits()
	if err != nil {
		t.Fatalf("GetTraits: %v", err)
	}
	got := traits[memory.TraitAffinity]
	if got >= 1.0 {
		t.Fatalf("affinity after damped update = %.4f, want below saturation", got)
	}
	if got <= 0.99 {
		t.Fatalf("affinity after damped update = %.4f, want a small positive movement", got)
	}
}

func TestProcessBehavioralEventsLonelinessConvergesFasterWhenActive(t *testing.T) {
	stm := newTestPersonalityRuntimeMemory(t)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	// Seed loneliness to a high value so we can observe the drop.
	if err := stm.SetTrait(memory.TraitLoneliness, 0.8); err != nil {
		t.Fatalf("SetTrait loneliness: %v", err)
	}

	// Simulate an active session with a very recent user message (< 6 hours).
	_, err := stm.InsertMessage("test-session", "user", "hello", false, false)
	if err != nil {
		t.Fatalf("InsertMessage: %v", err)
	}

	// Verify the message is readable and hours are near zero.
	hours, err := stm.GetHoursSinceLastUserMessage("test-session")
	if err != nil {
		t.Fatalf("GetHoursSinceLastUserMessage: %v", err)
	}
	if hours > 1 {
		t.Fatalf("expected recent message (<1h), got %.2f hours", hours)
	}

	msgs := []openai.ChatCompletionMessage{}
	meta := memory.PersonalityMeta{Volatility: 1.0, TraitDecayRate: 1.0, LonelinessSusceptibility: 1.0}
	processBehavioralEvents(stm, &msgs, "test-session", meta, logger)

	traits, err := stm.GetTraits()
	if err != nil {
		t.Fatalf("GetTraits: %v", err)
	}
	loneliness := traits[memory.TraitLoneliness]

	// With the faster convergence rate (0.6 instead of 0.2) for recent activity,
	// loneliness should drop noticeably from 0.8 toward the target (~0.0 for recent activity).
	// target = min(1.0, (0/72)*1.0) = 0.0
	// delta = (0 - 0.8) * 0.6 = -0.48
	// new value = 0.8 - 0.48 = 0.32
	if loneliness > 0.5 {
		t.Errorf("loneliness after recent activity = %.4f, expected significant drop below 0.5 due to faster convergence (target ~0, convergence 0.6)", loneliness)
	}
}
