package agent

import (
	"log/slog"
	"os"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/memory"
)

func newTestEmotionBehaviorMemory(t *testing.T) *memory.SQLiteMemory {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	if err := stm.InitPersonalityTables(); err != nil {
		t.Fatalf("InitPersonalityTables: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })
	return stm
}

func TestDeriveEmotionBehaviorPolicyUsesStructuredStateAndTraits(t *testing.T) {
	stm := newTestEmotionBehaviorMemory(t)

	if err := stm.SetTrait(memory.TraitConfidence, 0.22); err != nil {
		t.Fatalf("SetTrait confidence: %v", err)
	}
	if err := stm.SetTrait(memory.TraitThoroughness, 0.91); err != nil {
		t.Fatalf("SetTrait thoroughness: %v", err)
	}
	if err := stm.SetTrait(memory.TraitEmpathy, 0.86); err != nil {
		t.Fatalf("SetTrait empathy: %v", err)
	}
	if err := stm.InsertEmotionStateHistory(memory.EmotionState{
		Description:              "I feel tense but still trying to stay helpful.",
		PrimaryMood:              memory.MoodCautious,
		SecondaryMood:            "tense",
		Valence:                  -0.45,
		Arousal:                  0.82,
		Confidence:               0.81,
		Cause:                    "repeated tool failures",
		Source:                   "test",
		RecommendedResponseStyle: "warm_and_precise",
	}, "test"); err != nil {
		t.Fatalf("InsertEmotionStateHistory: %v", err)
	}

	policy := deriveEmotionBehaviorPolicy(stm, nil)

	if policy.MaxToolCallsDelta != -1 {
		t.Fatalf("MaxToolCallsDelta = %d, want -1", policy.MaxToolCallsDelta)
	}
	for _, want := range []string{
		"modify or delete data",
		"lightweight verification step",
		"exact last error",
		"warm and supportive",
	} {
		if !strings.Contains(strings.ToLower(policy.PromptHint), strings.ToLower(want)) {
			t.Fatalf("PromptHint %q does not contain %q", policy.PromptHint, want)
		}
	}
	if !strings.Contains(strings.ToLower(policy.RecoveryNudge), "avoid speculative retries") {
		t.Fatalf("RecoveryNudge = %q, want speculative-retry warning", policy.RecoveryNudge)
	}
}

func TestApplyEmotionRecoveryNudgeAppendsGuidance(t *testing.T) {
	got := applyEmotionRecoveryNudge("Base error.", emotionBehaviorPolicy{
		RecoveryNudge: "Inspect the exact last error.",
	})
	if !strings.Contains(got, "Base error.") || !strings.Contains(got, "Inspect the exact last error.") {
		t.Fatalf("applyEmotionRecoveryNudge() = %q", got)
	}
}

func TestCalculateEffectiveMaxCallsReducesForTenseRecoveryState(t *testing.T) {
	stm := newTestEmotionBehaviorMemory(t)
	if err := stm.InsertEmotionStateHistory(memory.EmotionState{
		Description: "I feel overloaded by repeated failures.",
		PrimaryMood: memory.MoodCautious,
		Valence:     -0.5,
		Arousal:     0.8,
		Confidence:  0.75,
		Source:      "test",
	}, "test"); err != nil {
		t.Fatalf("InsertEmotionStateHistory: %v", err)
	}

	cfg := &config.Config{}
	cfg.CircuitBreaker.MaxToolCalls = 10
	cfg.Personality.EngineV2 = true

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	got := calculateEffectiveMaxCalls(cfg, ToolCall{}, false, true, stm, logger)
	if got != 9 {
		t.Fatalf("calculateEffectiveMaxCalls() = %d, want 9", got)
	}
}
