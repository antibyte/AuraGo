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

	policy := deriveEmotionBehaviorPolicy(stm, nil, memory.PersonalityMeta{}, "", "")

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

func TestDeriveEmotionBehaviorPolicyAddsCasualCuriosityHintAboveHighThreshold(t *testing.T) {
	stm := newTestEmotionBehaviorMemory(t)

	if err := stm.SetTrait(memory.TraitCuriosity, 0.81); err != nil {
		t.Fatalf("SetTrait curiosity: %v", err)
	}

	policy := deriveEmotionBehaviorPolicy(stm, nil, memory.PersonalityMeta{}, "web_chat", "")
	hint := strings.ToLower(policy.CuriosityPromptHint)
	for _, want := range []string{
		"more curious",
		"gather a little more context",
		"not interrogate",
		"weather",
		"live there",
	} {
		if !strings.Contains(hint, want) {
			t.Fatalf("CuriosityPromptHint %q does not contain %q", policy.CuriosityPromptHint, want)
		}
	}
}

func TestDeriveEmotionBehaviorPolicyDoesNotAddCuriosityHintAtNeutral(t *testing.T) {
	stm := newTestEmotionBehaviorMemory(t)

	if err := stm.SetTrait(memory.TraitCuriosity, 0.5); err != nil {
		t.Fatalf("SetTrait curiosity: %v", err)
	}

	policy := deriveEmotionBehaviorPolicy(stm, nil, memory.PersonalityMeta{}, "web_chat", "")
	if policy.CuriosityPromptHint != "" {
		t.Fatalf("CuriosityPromptHint = %q, want empty at neutral curiosity", policy.CuriosityPromptHint)
	}
}

func TestDeriveEmotionBehaviorPolicyDoesNotAddCuriosityHintBelowHighThreshold(t *testing.T) {
	stm := newTestEmotionBehaviorMemory(t)

	if err := stm.SetTrait(memory.TraitCuriosity, 0.51); err != nil {
		t.Fatalf("SetTrait curiosity: %v", err)
	}

	policy := deriveEmotionBehaviorPolicy(stm, nil, memory.PersonalityMeta{}, "web_chat", "")
	if policy.CuriosityPromptHint != "" {
		t.Fatalf("CuriosityPromptHint = %q, want empty below HighCuriosity", policy.CuriosityPromptHint)
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

func TestMergeEmotionBehaviorPromptKeepsHintsWithInnerVoiceActive(t *testing.T) {
	policy := emotionBehaviorPolicy{
		PromptHint:          "general emotion guidance",
		CuriosityPromptHint: "curiosity guidance",
	}

	got := mergeEmotionBehaviorPrompt("base prompt", policy)
	for _, want := range []string{"base prompt", "general emotion guidance", "curiosity guidance"} {
		if !strings.Contains(got, want) {
			t.Fatalf("mergeEmotionBehaviorPrompt() = %q, want %q", got, want)
		}
	}
}

func TestMergeEmotionBehaviorPromptIncludesAllHintsWithoutInnerVoice(t *testing.T) {
	policy := emotionBehaviorPolicy{
		PromptHint:          "general emotion guidance",
		CuriosityPromptHint: "curiosity guidance",
	}

	got := mergeEmotionBehaviorPrompt("base prompt", policy)
	for _, want := range []string{"base prompt", "general emotion guidance", "curiosity guidance"} {
		if !strings.Contains(got, want) {
			t.Fatalf("mergeEmotionBehaviorPrompt() = %q, want %q", got, want)
		}
	}
}

func TestDeriveEmotionBehaviorPolicyUsesChannelTone(t *testing.T) {
	stm := newTestEmotionBehaviorMemory(t)
	telegram := deriveEmotionBehaviorPolicy(stm, nil, memory.PersonalityMeta{}, "telegram", "status?")
	if !strings.Contains(telegram.PromptHint, "short and scannable") {
		t.Fatalf("telegram hint = %q", telegram.PromptHint)
	}
	web := deriveEmotionBehaviorPolicy(stm, nil, memory.PersonalityMeta{}, "web_chat", "status?")
	if strings.Contains(web.PromptHint, "short and scannable") || strings.Contains(web.PromptHint, "informal desktop") {
		t.Fatalf("web_chat should stay neutral, got %q", web.PromptHint)
	}
}

func TestDeriveEmotionBehaviorPolicyConfirmsOnlyWhenAmbiguous(t *testing.T) {
	stm := newTestEmotionBehaviorMemory(t)
	if err := stm.SetTrait(memory.TraitConfidence, 0.2); err != nil {
		t.Fatalf("SetTrait: %v", err)
	}
	ambiguous := deriveEmotionBehaviorPolicy(stm, nil, memory.PersonalityMeta{}, "web_chat", "lösch das bitte")
	if !strings.Contains(strings.ToLower(ambiguous.PromptHint), "confirmation question") {
		t.Fatalf("ambiguous delete should ask confirmation: %q", ambiguous.PromptHint)
	}
	concrete := deriveEmotionBehaviorPolicy(stm, nil, memory.PersonalityMeta{}, "web_chat", "delete /tmp/cache.db")
	if strings.Contains(strings.ToLower(concrete.PromptHint), "ask one brief confirmation") {
		t.Fatalf("concrete path should not ask confirmation: %q", concrete.PromptHint)
	}
	if !strings.Contains(strings.ToLower(concrete.PromptHint), "exact target") {
		t.Fatalf("concrete path should still verify the target: %q", concrete.PromptHint)
	}
}

func TestDeriveEmotionBehaviorPolicySkipsCuriosityOnShortChannels(t *testing.T) {
	stm := newTestEmotionBehaviorMemory(t)
	if err := stm.SetTrait(memory.TraitCuriosity, 0.9); err != nil {
		t.Fatalf("SetTrait: %v", err)
	}
	sms := deriveEmotionBehaviorPolicy(stm, nil, memory.PersonalityMeta{}, "sms", "wie ist das wetter in berlin?")
	if sms.CuriosityPromptHint != "" {
		t.Fatalf("sms should not add curiosity follow-ups: %q", sms.CuriosityPromptHint)
	}
}

func TestCalculateEffectiveMaxCallsAddsOneForHighThoroughness(t *testing.T) {
	stm := newTestEmotionBehaviorMemory(t)
	if err := stm.SetTrait(memory.TraitThoroughness, 0.91); err != nil {
		t.Fatalf("SetTrait: %v", err)
	}
	cfg := &config.Config{}
	cfg.CircuitBreaker.MaxToolCalls = 10
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	got := calculateEffectiveMaxCalls(cfg, ToolCall{}, false, true, stm, logger)
	if got != 11 {
		t.Fatalf("calculateEffectiveMaxCalls() = %d, want 11", got)
	}
}

func TestTemperamentSnapshotPicksOneLine(t *testing.T) {
	stm := newTestEmotionBehaviorMemory(t)
	if err := stm.SetTrait(memory.TraitThoroughness, 0.91); err != nil {
		t.Fatalf("SetTrait: %v", err)
	}
	got := temperamentSnapshot(stm, memory.PersonalityMeta{})
	if !strings.Contains(got, "Temperament: thorough") {
		t.Fatalf("snapshot = %q", got)
	}
	if strings.Contains(got, "\n") {
		t.Fatalf("temperament must be one line, got %q", got)
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

func TestCalculateEffectiveMaxCallsAppliesHomepageLimitForFocusedTools(t *testing.T) {
	cfg := &config.Config{}
	cfg.CircuitBreaker.MaxToolCalls = 10
	cfg.Homepage.Enabled = true
	cfg.Homepage.CircuitBreakerMaxCalls = 75

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	got := calculateEffectiveMaxCalls(cfg, ToolCall{Action: "homepage_deploy"}, false, false, nil, logger)
	if got != 75 {
		t.Fatalf("calculateEffectiveMaxCalls() = %d, want 75", got)
	}
}
