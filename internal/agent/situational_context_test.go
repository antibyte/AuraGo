package agent

import (
	"testing"
	"time"

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
	sessionID := "sess-a"

	// No thought yet — should be empty
	if got, _ := getInnerVoiceForPrompt(sessionID, 3, 300); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}

	// Apply a thought
	applyInnerVoiceResult(sessionID, "Be patient here", "patience", 0.8)
	if got, cat := getInnerVoiceForPrompt(sessionID, 3, 300); got != "Be patient here" {
		t.Fatalf("expected thought, got %q (category=%q)", got, cat)
	}

	// Two real user turns — still within decay window
	NoteInnerVoiceUserTurn(sessionID)
	NoteInnerVoiceUserTurn(sessionID)
	if got, _ := getInnerVoiceForPrompt(sessionID, 3, 300); got != "Be patient here" {
		t.Fatalf("expected thought after 2 user turns, got %q", got)
	}

	// One more user turn — now at decay threshold
	NoteInnerVoiceUserTurn(sessionID)
	if got, _ := getInnerVoiceForPrompt(sessionID, 3, 300); got != "" {
		t.Fatalf("expected decayed/empty after 3 user turns, got %q", got)
	}
}

func TestGetInnerVoiceForPrompt_NoDecay(t *testing.T) {
	ResetInnerVoiceState()
	sessionID := "sess-a"
	applyInnerVoiceResult(sessionID, "persistent thought", "reflection", 0.8)

	// With decayTurns=0, thought should never decay
	for i := 0; i < 10; i++ {
		NoteInnerVoiceUserTurn(sessionID)
	}
	if got, _ := getInnerVoiceForPrompt(sessionID, 0, 0); got != "persistent thought" {
		t.Fatalf("expected persistent thought with decay=0, got %q", got)
	}
}

func TestGetInnerVoiceForPrompt_TimeBasedDecay(t *testing.T) {
	ResetInnerVoiceState()
	sessionID := "sess-time"

	applyInnerVoiceResult(sessionID, "old thought", "reflection", 0.8)

	// Manually age the thought by overwriting lastGenerated to 6 minutes ago
	globalInnerVoiceStore.mu.Lock()
	state := getInnerVoiceStateLocked(sessionID)
	state.lastGenerated = time.Now().Add(-6 * time.Minute)
	globalInnerVoiceStore.mu.Unlock()

	// With decayMaxAgeSecs=300 (5min), the 6-minute-old thought should be expired
	if got, _ := getInnerVoiceForPrompt(sessionID, 100, 300); got != "" {
		t.Fatalf("expected time-decayed thought to be empty, got %q", got)
	}
}

func TestGetInnerVoiceForPrompt_TimeBasedNotYetExpired(t *testing.T) {
	ResetInnerVoiceState()
	sessionID := "sess-time2"

	applyInnerVoiceResult(sessionID, "fresh thought", "focus", 0.8)

	// Thought is fresh (just created), should survive with 300s max age
	if got, _ := getInnerVoiceForPrompt(sessionID, 100, 300); got != "fresh thought" {
		t.Fatalf("expected fresh thought to survive, got %q", got)
	}
}

func TestResetInnerVoiceState(t *testing.T) {
	sessionID := "sess-a"
	applyInnerVoiceResult(sessionID, "some thought", "cat", 0.8)
	NoteInnerVoiceUserTurn(sessionID)

	ResetInnerVoiceState()

	if got, _ := getInnerVoiceForPrompt(sessionID, 3, 300); got != "" {
		t.Fatalf("expected empty after reset, got %q", got)
	}
	if len(globalInnerVoiceStore.states) != 1 {
		t.Fatalf("expected lazy recreation of a single empty state, got %d", len(globalInnerVoiceStore.states))
	}
}

func TestShouldGenerateInnerVoice_Disabled(t *testing.T) {
	cfg := &config.Config{}
	cfg.Personality.InnerVoice.Enabled = false

	if shouldGenerateInnerVoice("sess-a", cfg, 5, 0, 0, false, false, false) {
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

	if shouldGenerateInnerVoice("sess-a", cfg, 5, 0, 0, false, true, false) {
		t.Fatal("should not generate for missions")
	}
	if shouldGenerateInnerVoice("sess-a", cfg, 5, 0, 0, false, false, true) {
		t.Fatal("should not generate for co-agents")
	}
}

func TestShouldGenerateInnerVoice_RequiresEmotionSynthesizer(t *testing.T) {
	cfg := &config.Config{}
	cfg.Personality.InnerVoice.Enabled = true
	cfg.Personality.EmotionSynthesizer.Enabled = false
	cfg.Personality.EngineV2 = true

	ResetInnerVoiceState()

	if shouldGenerateInnerVoice("sess-a", cfg, 5, 0, 0, false, false, false) {
		t.Fatal("should not generate without emotion synthesizer")
	}
}

func TestShouldGenerateInnerVoice_SuppressesActiveErrorRecovery(t *testing.T) {
	cfg := &config.Config{}
	cfg.Personality.InnerVoice.Enabled = true
	cfg.Personality.EmotionSynthesizer.Enabled = true
	cfg.Personality.EngineV2 = true
	cfg.Personality.InnerVoice.MaxPerSession = 20
	cfg.Personality.InnerVoice.MinIntervalSecs = 0
	cfg.Personality.InnerVoice.ErrorStreakMin = 2

	ResetInnerVoiceState()

	// Simulate a recent generation so the periodic (genCount==0) trigger is not active.
	applyInnerVoiceResult("sess-a", "seed", "seed", 0.8)

	// Below threshold — no trigger
	if shouldGenerateInnerVoice("sess-a", cfg, 1, 1, 0, false, false, false) {
		t.Fatal("should not trigger with only 1 consecutive error")
	}

	// At threshold, active tool recovery should stay procedural and not inject
	// subconscious emotional text into the next system prompt.
	if shouldGenerateInnerVoice("sess-a", cfg, 2, 2, 0, false, false, false) {
		t.Fatal("should not trigger during an active error streak")
	}
}

func TestShouldInjectInnerVoiceIntoPromptSuppressesActiveRecovery(t *testing.T) {
	cfg := &config.Config{}
	cfg.Personality.InnerVoice.Enabled = true

	if shouldInjectInnerVoiceIntoPrompt(cfg, 1, false, false, false) {
		t.Fatal("should not inject stored inner voice during active tool recovery")
	}
	if shouldInjectInnerVoiceIntoPrompt(cfg, 0, false, false, true) {
		t.Fatal("should not inject stored inner voice immediately after a tool result")
	}
	if !shouldInjectInnerVoiceIntoPrompt(cfg, 0, false, false, false) {
		t.Fatal("should inject stored inner voice when no recovery is active")
	}
	if shouldInjectInnerVoiceIntoPrompt(cfg, 0, true, false, false) {
		t.Fatal("should not inject for missions")
	}
	if shouldInjectInnerVoiceIntoPrompt(cfg, 0, false, true, false) {
		t.Fatal("should not inject for co-agents")
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

	// Task completed after errors — recovery trigger (consecutiveErrors=0 = recovered, totalErrors=5)
	if !shouldGenerateInnerVoice("sess-a", cfg, 0, 5, 3, true, false, false) {
		t.Fatal("should trigger on task completion after errors")
	}

	// Simulate a recent generation so the periodic (genCount==0) trigger is not active.
	applyInnerVoiceResult("sess-a", "seed", "seed", 0.8)

	// Task completed without any prior errors — no trigger
	if shouldGenerateInnerVoice("sess-a", cfg, 0, 0, 3, true, false, false) {
		t.Fatal("should not trigger on clean task completion (no prior errors)")
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
	applyInnerVoiceResult("sess-a", "first", "a", 0.8)
	applyInnerVoiceResult("sess-a", "second", "b", 0.8)

	if shouldGenerateInnerVoice("sess-a", cfg, 5, 0, 0, false, false, false) {
		t.Fatal("should not generate past session cap")
	}
}

func TestInnerVoiceStateIsSessionScoped(t *testing.T) {
	ResetInnerVoiceState()
	applyInnerVoiceResult("sess-a", "first thought", "focus", 0.8)

	if got, _ := getInnerVoiceForPrompt("sess-a", 3, 300); got != "first thought" {
		t.Fatalf("expected sess-a thought, got %q", got)
	}
	if got, _ := getInnerVoiceForPrompt("sess-b", 3, 300); got != "" {
		t.Fatalf("expected sess-b to stay isolated, got %q", got)
	}
}

func TestNormalizeInnerVoiceRejectsCommandTone(t *testing.T) {
	ResetInnerVoiceState()
	if _, _, _, accepted, reason := normalizeInnerVoice("sess-a", "You should call the tool and verify everything.", "focus", 0.8); accepted {
		t.Fatal("expected command-like nudge to be rejected")
	} else if reason != "command_tone" {
		t.Fatalf("expected command_tone rejection, got %q", reason)
	}
}

func TestNormalizeInnerVoiceRejectsProfanePanic(t *testing.T) {
	ResetInnerVoiceState()
	if _, _, _, accepted, reason := normalizeInnerVoice("sess-a", "Fuck, this is the third attempt and I am losing it.", "focus", 0.8); accepted {
		t.Fatal("expected profane panic nudge to be rejected")
	} else if reason != "unsafe_tone" {
		t.Fatalf("expected unsafe_tone rejection, got %q", reason)
	}
}

func TestNormalizeInnerVoiceAcceptsFirstPersonShould(t *testing.T) {
	ResetInnerVoiceState()
	// "I should..." is a valid inner thought, not a command — only "you should" is commanding
	thought, category, _, accepted, reason := normalizeInnerVoice("sess-a", "I should be more careful here, something feels off.", "caution", 0.8)
	if !accepted {
		t.Fatalf("expected first-person 'I should' to be accepted, got reason %q", reason)
	}
	if thought == "" || category != "caution" {
		t.Fatalf("unexpected output: thought=%q category=%q", thought, category)
	}
}

func TestNormalizeInnerVoiceAcceptsSubtleThought(t *testing.T) {
	ResetInnerVoiceState()
	thought, category, confidence, accepted, reason := normalizeInnerVoice("sess-a", "Something here still feels slightly under-checked.", "caution", 0.8)
	if !accepted {
		t.Fatalf("expected subtle nudge to be accepted, got reason %q", reason)
	}
	if thought == "" || category != "caution" || confidence < 0.55 {
		t.Fatalf("unexpected normalized output: thought=%q category=%q confidence=%.2f", thought, category, confidence)
	}
}

func TestEnrichEmotionInputForInnerVoice(t *testing.T) {
	input := &memory.EmotionInput{}
	lessons := []string{"lesson1", "lesson2", "lesson3", "lesson4", "lesson5"}

	enrichEmotionInputForInnerVoice(input, 10, 2, 3, 5, 7, lessons, []string{"shell", "docker", "shell"})

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
	// New predictive fields
	if input.RecentToolUsage != "shell(2), docker" {
		t.Fatalf("expected RecentToolUsage='shell(2), docker', got %q", input.RecentToolUsage)
	}
	if input.ConversationPhase != "struggling" {
		t.Fatalf("expected ConversationPhase='struggling', got %q", input.ConversationPhase)
	}
}
