package agent

import (
	"log/slog"
	"os"
	"testing"

	"aurago/internal/config"
	"aurago/internal/memory"
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
