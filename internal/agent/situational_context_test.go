package agent

import (
	"testing"

	"aurago/internal/config"
	"aurago/internal/memory"
)

func TestDeriveTaskStatus(t *testing.T) {
	tests := []struct {
		name            string
		consecutiveErrs int
		totalErrs       int
		totalSuccesses  int
		expected        string
	}{
		{"starting", 0, 0, 0, "starting"},
		{"struggling", 3, 3, 0, "struggling"},
		{"recovering", 0, 2, 1, "recovering"},
		{"in_progress_clean", 0, 0, 5, "in_progress"},
		{"in_progress_mixed", 1, 1, 2, "in_progress"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := deriveTaskStatus(tt.consecutiveErrs, tt.totalErrs, tt.totalSuccesses)
			if result != tt.expected {
				t.Fatalf("deriveTaskStatus(%d, %d, %d) = %q, want %q",
					tt.consecutiveErrs, tt.totalErrs, tt.totalSuccesses, result, tt.expected)
			}
		})
	}
}

func TestGetInnerVoiceForPrompt_Decay(t *testing.T) {
	ResetInnerVoiceState()

	// No thought yet — should be empty
	if got, _ := getInnerVoiceForPrompt(3); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}

	// Apply a thought
	applyInnerVoiceResult("Be patient here", "patience")
	if got, cat := getInnerVoiceForPrompt(3); got != "Be patient here" {
		t.Fatalf("expected thought, got %q (category=%q)", got, cat)
	}

	// Tick 2 turns — still within decay window
	tickInnerVoiceTurn()
	tickInnerVoiceTurn()
	if got, _ := getInnerVoiceForPrompt(3); got != "Be patient here" {
		t.Fatalf("expected thought after 2 ticks, got %q", got)
	}

	// Tick 1 more — now at decay threshold (3 turns since)
	tickInnerVoiceTurn()
	if got, _ := getInnerVoiceForPrompt(3); got != "" {
		t.Fatalf("expected decayed/empty after 3 ticks, got %q", got)
	}
}

func TestGetInnerVoiceForPrompt_NoDecay(t *testing.T) {
	ResetInnerVoiceState()
	applyInnerVoiceResult("persistent thought", "reflection")

	// With decayTurns=0, thought should never decay
	for i := 0; i < 10; i++ {
		tickInnerVoiceTurn()
	}
	if got, _ := getInnerVoiceForPrompt(0); got != "persistent thought" {
		t.Fatalf("expected persistent thought with decay=0, got %q", got)
	}
}

func TestResetInnerVoiceState(t *testing.T) {
	applyInnerVoiceResult("some thought", "cat")
	tickInnerVoiceTurn()

	ResetInnerVoiceState()

	if got, _ := getInnerVoiceForPrompt(3); got != "" {
		t.Fatalf("expected empty after reset, got %q", got)
	}
	if count := innerVoiceSessionCount.Load(); count != 0 {
		t.Fatalf("expected session count 0 after reset, got %d", count)
	}
}

func TestShouldGenerateInnerVoice_Disabled(t *testing.T) {
	cfg := &config.Config{}
	cfg.Personality.InnerVoice.Enabled = false

	if shouldGenerateInnerVoice(cfg, 5, 0, false, false, false) {
		t.Fatal("should not generate when disabled")
	}
}

func TestShouldGenerateInnerVoice_SkipMissionAndCoAgent(t *testing.T) {
	cfg := &config.Config{}
	cfg.Personality.InnerVoice.Enabled = true
	cfg.Personality.EmotionSynthesizer.Enabled = true
	cfg.Personality.EngineV2 = true
	cfg.Personality.InnerVoice.MaxPerSession = 20
	cfg.Personality.InnerVoice.MinIntervalSecs = 0
	cfg.Personality.InnerVoice.ErrorStreakMin = 2

	ResetInnerVoiceState()

	if shouldGenerateInnerVoice(cfg, 5, 0, false, true, false) {
		t.Fatal("should not generate for missions")
	}
	if shouldGenerateInnerVoice(cfg, 5, 0, false, false, true) {
		t.Fatal("should not generate for co-agents")
	}
}

func TestShouldGenerateInnerVoice_RequiresEmotionSynthesizer(t *testing.T) {
	cfg := &config.Config{}
	cfg.Personality.InnerVoice.Enabled = true
	cfg.Personality.EmotionSynthesizer.Enabled = false
	cfg.Personality.EngineV2 = true

	ResetInnerVoiceState()

	if shouldGenerateInnerVoice(cfg, 5, 0, false, false, false) {
		t.Fatal("should not generate without emotion synthesizer")
	}
}

func TestShouldGenerateInnerVoice_ErrorStreak(t *testing.T) {
	cfg := &config.Config{}
	cfg.Personality.InnerVoice.Enabled = true
	cfg.Personality.EmotionSynthesizer.Enabled = true
	cfg.Personality.EngineV2 = true
	cfg.Personality.InnerVoice.MaxPerSession = 20
	cfg.Personality.InnerVoice.MinIntervalSecs = 0
	cfg.Personality.InnerVoice.ErrorStreakMin = 2

	ResetInnerVoiceState()

	// Below threshold — no trigger
	if shouldGenerateInnerVoice(cfg, 1, 0, false, false, false) {
		t.Fatal("should not trigger with only 1 error")
	}

	// At threshold — trigger
	if !shouldGenerateInnerVoice(cfg, 2, 0, false, false, false) {
		t.Fatal("should trigger with error streak >= 2")
	}
}

func TestShouldGenerateInnerVoice_RecoveryTrigger(t *testing.T) {
	cfg := &config.Config{}
	cfg.Personality.InnerVoice.Enabled = true
	cfg.Personality.EmotionSynthesizer.Enabled = true
	cfg.Personality.EngineV2 = true
	cfg.Personality.InnerVoice.MaxPerSession = 20
	cfg.Personality.InnerVoice.MinIntervalSecs = 0
	cfg.Personality.InnerVoice.ErrorStreakMin = 5

	ResetInnerVoiceState()

	// Task completed after errors — recovery trigger
	if !shouldGenerateInnerVoice(cfg, 1, 3, true, false, false) {
		t.Fatal("should trigger on task completion after errors")
	}

	// Task completed without prior errors — no trigger
	if shouldGenerateInnerVoice(cfg, 0, 3, true, false, false) {
		t.Fatal("should not trigger on clean task completion")
	}
}

func TestShouldGenerateInnerVoice_SessionCap(t *testing.T) {
	cfg := &config.Config{}
	cfg.Personality.InnerVoice.Enabled = true
	cfg.Personality.EmotionSynthesizer.Enabled = true
	cfg.Personality.EngineV2 = true
	cfg.Personality.InnerVoice.MaxPerSession = 2
	cfg.Personality.InnerVoice.MinIntervalSecs = 0
	cfg.Personality.InnerVoice.ErrorStreakMin = 1

	ResetInnerVoiceState()

	// Generate up to cap
	applyInnerVoiceResult("first", "a")
	applyInnerVoiceResult("second", "b")

	if shouldGenerateInnerVoice(cfg, 5, 0, false, false, false) {
		t.Fatal("should not generate past session cap")
	}
}

func TestEnrichEmotionInputForInnerVoice(t *testing.T) {
	input := &memory.EmotionInput{}
	lessons := []string{"lesson1", "lesson2", "lesson3", "lesson4", "lesson5"}

	enrichEmotionInputForInnerVoice(input, 10, 2, 3, 5, 7, lessons)

	if !input.InnerVoiceEnabled {
		t.Fatal("expected InnerVoiceEnabled = true")
	}
	if input.ConversationTurns != 10 {
		t.Fatalf("expected ConversationTurns=10, got %d", input.ConversationTurns)
	}
	if input.RecoveryAttempts != 2 {
		t.Fatalf("expected RecoveryAttempts=2, got %d", input.RecoveryAttempts)
	}
	if input.TaskStatus != "struggling" {
		t.Fatalf("expected TaskStatus=struggling, got %q", input.TaskStatus)
	}
	// Lessons should be truncated to 3
	if len(input.RelevantLessons) != 3 {
		t.Fatalf("expected 3 lessons, got %d", len(input.RelevantLessons))
	}
}
