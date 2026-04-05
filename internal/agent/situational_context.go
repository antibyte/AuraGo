package agent

import (
	"sync"
	"sync/atomic"
	"time"

	"aurago/internal/config"
	"aurago/internal/memory"
)

// innerVoiceState tracks runtime state for the inner voice system within a session.
type innerVoiceState struct {
	mu              sync.Mutex
	lastGenerated   time.Time
	generationCount int
	turnsSinceLast  int    // turns elapsed since last inner voice was generated
	currentThought  string // the latest inner thought text
	nudgeCategory   string // category of the latest thought
}

// globalInnerVoiceState is the singleton state for the current session.
var globalInnerVoiceState = &innerVoiceState{}

// innerVoiceSessionCount tracks total inner voice generations for the current session.
var innerVoiceSessionCount atomic.Int32

// resetInnerVoiceState resets the inner voice state (e.g. on session reset).
func resetInnerVoiceState() {
	globalInnerVoiceState.mu.Lock()
	defer globalInnerVoiceState.mu.Unlock()
	globalInnerVoiceState.lastGenerated = time.Time{}
	globalInnerVoiceState.generationCount = 0
	globalInnerVoiceState.turnsSinceLast = 0
	globalInnerVoiceState.currentThought = ""
	globalInnerVoiceState.nudgeCategory = ""
	innerVoiceSessionCount.Store(0)
}

// tickInnerVoiceTurn increments the turn counter since last generation.
// Called once per agent loop iteration.
func tickInnerVoiceTurn() {
	globalInnerVoiceState.mu.Lock()
	defer globalInnerVoiceState.mu.Unlock()
	globalInnerVoiceState.turnsSinceLast++
}

// applyInnerVoiceResult stores a newly generated inner voice thought.
func applyInnerVoiceResult(thought, category string) {
	globalInnerVoiceState.mu.Lock()
	defer globalInnerVoiceState.mu.Unlock()
	globalInnerVoiceState.currentThought = thought
	globalInnerVoiceState.nudgeCategory = category
	globalInnerVoiceState.turnsSinceLast = 0
	globalInnerVoiceState.lastGenerated = time.Now()
	globalInnerVoiceState.generationCount++
	innerVoiceSessionCount.Add(1)
}

// getInnerVoiceForPrompt returns the current inner voice text and its nudge category
// if it hasn't decayed. Returns empty strings if decayed or not available.
func getInnerVoiceForPrompt(decayTurns int) (string, string) {
	globalInnerVoiceState.mu.Lock()
	defer globalInnerVoiceState.mu.Unlock()
	if globalInnerVoiceState.currentThought == "" {
		return "", ""
	}
	if decayTurns > 0 && globalInnerVoiceState.turnsSinceLast >= decayTurns {
		// Decayed — clear stale thought
		globalInnerVoiceState.currentThought = ""
		globalInnerVoiceState.nudgeCategory = ""
		return "", ""
	}
	return globalInnerVoiceState.currentThought, globalInnerVoiceState.nudgeCategory
}

// shouldGenerateInnerVoice determines whether the inner voice system should trigger.
// It evaluates rate limits, session caps, and situational triggers.
func shouldGenerateInnerVoice(
	cfg *config.Config,
	errorCount int,
	successCount int,
	taskCompleted bool,
	isMission bool,
	isCoAgent bool,
) bool {
	if !cfg.Personality.InnerVoice.Enabled {
		return false
	}
	// Skip for missions and co-agents — they are transactional
	if isMission || isCoAgent {
		return false
	}
	// Requires Emotion Synthesizer + V2
	if !cfg.Personality.EmotionSynthesizer.Enabled || !cfg.Personality.EngineV2 {
		return false
	}

	// Session cap
	if int(innerVoiceSessionCount.Load()) >= cfg.Personality.InnerVoice.MaxPerSession {
		return false
	}

	// Rate limiting
	globalInnerVoiceState.mu.Lock()
	elapsed := time.Since(globalInnerVoiceState.lastGenerated)
	globalInnerVoiceState.mu.Unlock()
	minInterval := time.Duration(cfg.Personality.InnerVoice.MinIntervalSecs) * time.Second
	if elapsed < minInterval {
		return false
	}

	// Trigger: error streak
	if errorCount >= cfg.Personality.InnerVoice.ErrorStreakMin {
		return true
	}

	// Trigger: task completed (success after errors — recovery)
	if taskCompleted && errorCount > 0 {
		return true
	}

	return false
}

// deriveTaskStatus computes a human-readable task status from error/success counts.
func deriveTaskStatus(consecutiveErrors, totalErrors, totalSuccesses int) string {
	if totalErrors == 0 && totalSuccesses == 0 {
		return "starting"
	}
	if consecutiveErrors >= 2 {
		return "struggling"
	}
	if totalErrors > 0 && consecutiveErrors == 0 && totalSuccesses > 0 {
		return "recovering"
	}
	if totalSuccesses > 0 && totalErrors == 0 {
		return "in_progress"
	}
	return "in_progress"
}

// enrichEmotionInputForInnerVoice adds inner voice context fields to an EmotionInput.
func enrichEmotionInputForInnerVoice(
	input *memory.EmotionInput,
	conversationTurns int,
	recoveryAttempts int,
	consecutiveErrors int,
	totalErrors int,
	totalSuccesses int,
	lessons []string,
) {
	input.InnerVoiceEnabled = true
	input.ConversationTurns = conversationTurns
	input.RecoveryAttempts = recoveryAttempts
	input.TaskStatus = deriveTaskStatus(consecutiveErrors, totalErrors, totalSuccesses)
	if len(lessons) > 3 {
		lessons = lessons[:3]
	}
	input.RelevantLessons = lessons
}
