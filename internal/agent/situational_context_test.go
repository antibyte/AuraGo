package agent

import (
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/memory"

	"github.com/sashabaranov/go-openai"
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

func TestResetInnerVoiceSessionIsScoped(t *testing.T) {
	ResetInnerVoiceState()
	applyInnerVoiceResult("sess-a", "clear this thought", "focus", 0.8)
	applyInnerVoiceResult("sess-b", "keep this thought", "focus", 0.8)

	ResetInnerVoiceSession("sess-a")

	if got, _ := getInnerVoiceForPrompt("sess-a", 3, 300); got != "" {
		t.Fatalf("expected sess-a thought to be cleared, got %q", got)
	}
	if got, _ := getInnerVoiceForPrompt("sess-b", 3, 300); got != "keep this thought" {
		t.Fatalf("expected sess-b thought to be preserved, got %q", got)
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

func TestShouldGenerateInnerVoice_RecoveryTriggerRequiresToolSuccess(t *testing.T) {
	cfg := &config.Config{}
	cfg.Personality.InnerVoice.Enabled = true
	cfg.Personality.EmotionSynthesizer.Enabled = true
	cfg.Personality.EngineV2 = true
	cfg.Personality.InnerVoice.MaxPerSession = 20
	cfg.Personality.InnerVoice.MinIntervalSecs = 0

	ResetInnerVoiceState()
	applyInnerVoiceResult("sess-a", "seed", "seed", 0.8)

	if shouldGenerateInnerVoice("sess-a", cfg, 0, 5, 0, true, false, false) {
		t.Fatal("should not generate a recovery/completion inner voice when no tool succeeded after errors")
	}
}

func TestShouldGenerateInnerVoice_DesktopSuppressesPeriodicTrigger(t *testing.T) {
	cfg := &config.Config{}
	cfg.Personality.InnerVoice.Enabled = true
	cfg.Personality.EmotionSynthesizer.Enabled = true
	cfg.Personality.EngineV2 = true
	cfg.Personality.InnerVoice.MaxPerSession = 20
	cfg.Personality.InnerVoice.MinIntervalSecs = 0
	cfg.Personality.InnerVoice.DecayTurns = 1

	ResetInnerVoiceState()

	if shouldGenerateInnerVoice("virtual-desktop", cfg, 0, 0, 0, false, false, false) {
		t.Fatal("desktop chat should not generate periodic inner voice thoughts")
	}
}

func TestShouldGenerateInnerVoice_DesktopRecoveryIsStronglyRateLimited(t *testing.T) {
	cfg := &config.Config{}
	cfg.Personality.InnerVoice.Enabled = true
	cfg.Personality.EmotionSynthesizer.Enabled = true
	cfg.Personality.EngineV2 = true
	cfg.Personality.InnerVoice.MaxPerSession = 20
	cfg.Personality.InnerVoice.MinIntervalSecs = 0
	cfg.Personality.InnerVoice.DecayTurns = 1

	ResetInnerVoiceState()
	NoteInnerVoiceUserTurn("virtual-desktop")

	if !shouldGenerateInnerVoice("virtual-desktop", cfg, 0, 3, 1, true, false, false) {
		t.Fatal("desktop chat may generate one inner voice after a real recovery")
	}
	if shouldGenerateInnerVoice("virtual-desktop", cfg, 0, 3, 1, true, false, false) {
		t.Fatal("desktop chat recovery inner voice should be rate-limited after first request")
	}
}

func TestShouldGenerateInnerVoice_DesktopSessionCapIsLow(t *testing.T) {
	cfg := &config.Config{}
	cfg.Personality.InnerVoice.Enabled = true
	cfg.Personality.EmotionSynthesizer.Enabled = true
	cfg.Personality.EngineV2 = true
	cfg.Personality.InnerVoice.MaxPerSession = 20
	cfg.Personality.InnerVoice.MinIntervalSecs = 0
	cfg.Personality.InnerVoice.DecayTurns = 1

	ResetInnerVoiceState()
	NoteInnerVoiceUserTurn("virtual-desktop")
	applyInnerVoiceResult("virtual-desktop", "first", "focus", 0.8)
	applyInnerVoiceResult("virtual-desktop", "second", "focus", 0.8)

	if shouldGenerateInnerVoice("virtual-desktop", cfg, 0, 3, 1, true, false, false) {
		t.Fatal("desktop chat should cap inner voice to a very small number per session")
	}
}

func TestShouldGenerateInnerVoice_SessionCap(t *testing.T) {
	cfg := &config.Config{}
	cfg.Personality.InnerVoice.Enabled = true
	cfg.Personality.EmotionSynthesizer.Enabled = true
	cfg.Personality.EngineV2 = true
	cfg.Personality.InnerVoice.MaxPerSession = 2
	cfg.Personality.InnerVoice.MinIntervalSecs = 0

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

func TestNormalizeInnerVoiceCommandFilter(t *testing.T) {
	tests := []struct {
		thought  string
		accepted bool
	}{
		{"I should double-check this.", false},
		{"Let's verify it.", false},
		{"Ich sollte geduldig bleiben.", false},
		{"You must hurry.", false},
		{"Whenever this works, I feel relieved.", true},
		{"I never expected the config to be this tidy.", true},
		{"I'm curious whether the backup holds.", true},
	}
	for _, tt := range tests {
		t.Run(tt.thought, func(t *testing.T) {
			ResetInnerVoiceState()
			_, _, _, accepted, reason := normalizeInnerVoice("sess-a", tt.thought, "focus", 0.8)
			if accepted != tt.accepted {
				t.Fatalf("normalizeInnerVoice(%q) accepted=%v (reason %q), want %v", tt.thought, accepted, reason, tt.accepted)
			}
			if !accepted && reason != "command_tone" {
				t.Fatalf("expected command_tone rejection, got %q", reason)
			}
		})
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

func TestNormalizeInnerVoiceRejectsChecklists(t *testing.T) {
	for _, thought := range []string{"I feel fine.\n- check logs", "I feel fine.\n1. restart"} {
		ResetInnerVoiceState()
		if _, _, _, accepted, reason := normalizeInnerVoice("sess-a", thought, "focus", 0.8); accepted || reason != "checklist_like" {
			t.Fatalf("normalizeInnerVoice(%q) accepted=%v reason=%q, want checklist_like rejection", thought, accepted, reason)
		}
	}
}

func TestReleaseInnerVoiceReservationAllowsFallbackRequest(t *testing.T) {
	cfg := &config.Config{}
	cfg.Personality.InnerVoice.Enabled = true
	cfg.Personality.EmotionSynthesizer.Enabled = true
	cfg.Personality.EngineV2 = true
	cfg.Personality.InnerVoice.MaxPerSession = 20
	cfg.Personality.InnerVoice.MinIntervalSecs = 3600
	ResetInnerVoiceState()
	t.Cleanup(ResetInnerVoiceState)

	if !shouldGenerateInnerVoice("sess-a", cfg, 0, 0, 0, false, false, false) {
		t.Fatal("expected the first periodic request to reserve a slot")
	}
	if shouldGenerateInnerVoice("sess-a", cfg, 0, 0, 0, false, false, false) {
		t.Fatal("expected a second request within min_interval to be refused")
	}
	releaseInnerVoiceReservation("sess-a")
	if !shouldGenerateInnerVoice("sess-a", cfg, 0, 0, 0, false, false, false) {
		t.Fatal("expected the released slot to be reservable again within min_interval")
	}

	applyInnerVoiceResult("sess-b", "I feel calm.", "focus", 0.8)
	releaseInnerVoiceReservation("sess-b")
	globalInnerVoiceStore.mu.Lock()
	state := *getInnerVoiceStateLocked("sess-b")
	globalInnerVoiceStore.mu.Unlock()
	if state.generationCount != 1 || state.lastGenerated.IsZero() {
		t.Fatalf("release must keep generation bookkeeping, got count=%d lastGenerated=%v", state.generationCount, state.lastGenerated)
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
	if input.ConversationPhase != "execution" {
		t.Fatalf("expected ConversationPhase='execution', got %q", input.ConversationPhase)
	}
}

func TestDeriveConversationPhase(t *testing.T) {
	for _, tt := range []struct {
		turns, successes int
		want             string
	}{
		{0, 0, "opening"}, {1, 3, "opening"}, {4, 0, "exploration"}, {4, 2, "execution"}, {12, 2, "execution"},
	} {
		if got := deriveConversationPhase(tt.turns, tt.successes); got != tt.want {
			t.Fatalf("deriveConversationPhase(%d, %d) = %q, want %q", tt.turns, tt.successes, got, tt.want)
		}
	}
}

func TestInnerVoiceTurnFacts(t *testing.T) {
	call := func(names ...string) openai.ChatCompletionMessage {
		msg := openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant}
		for _, name := range names {
			msg.ToolCalls = append(msg.ToolCalls, openai.ToolCall{Function: openai.FunctionCall{Name: name}})
		}
		return msg
	}
	msgs := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: "system"},
		{Role: openai.ChatMessageRoleUser, Content: "first"},
		call("old_tool"),
		{Role: openai.ChatMessageRoleTool, Content: "ok"},
		{Role: openai.ChatMessageRoleAssistant, Content: "done"},
		{Role: openai.ChatMessageRoleUser, Content: "second"},
		call("shell", "docker"),
		{Role: openai.ChatMessageRoleTool, Content: "ok"},
		{Role: openai.ChatMessageRoleUser, Content: "Tool Output: text mode result"},
		call("shell"),
	}
	if got := innerVoiceUserTurns(msgs); got != 2 {
		t.Fatalf("innerVoiceUserTurns = %d, want 2 (system, tool, assistant and text-mode tool results are no turns)", got)
	}
	if got := strings.Join(innerVoiceTurnToolNames(msgs), ","); got != "shell,docker,shell" {
		t.Fatalf("innerVoiceTurnToolNames = %q, want only this turn's tools", got)
	}
	if innerVoiceUserTurns(nil) != 0 || len(innerVoiceTurnToolNames(nil)) != 0 {
		t.Fatal("empty history must yield no turns and no tools")
	}
}
