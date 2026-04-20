package agent

import (
	"fmt"
	"math"
	"regexp"
	"strings"
	"sync"
	"time"

	"aurago/internal/config"
	"aurago/internal/memory"
)

// innerVoiceState tracks runtime state for the inner voice system within a session.
type innerVoiceState struct {
	lastGenerated      time.Time
	generationCount    int
	userTurnsSinceLast int
	currentThought     string
	nudgeCategory      string
	lastConfidence     float64
}

type innerVoiceStore struct {
	mu     sync.Mutex
	states map[string]*innerVoiceState
}

var globalInnerVoiceStore = &innerVoiceStore{
	states: make(map[string]*innerVoiceState),
}

var innerVoiceWhitespaceRx = regexp.MustCompile(`\s+`)

func normalizeInnerVoiceSessionID(sessionID string) string {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return "default"
	}
	return sessionID
}

func getInnerVoiceStateLocked(sessionID string) *innerVoiceState {
	sessionID = normalizeInnerVoiceSessionID(sessionID)
	state, ok := globalInnerVoiceStore.states[sessionID]
	if !ok {
		state = &innerVoiceState{}
		globalInnerVoiceStore.states[sessionID] = state
	}
	return state
}

// ResetInnerVoiceState resets the inner voice state (e.g. on session reset).
// Exported so that the commands and server packages can call it on /reset.
func ResetInnerVoiceState() {
	globalInnerVoiceStore.mu.Lock()
	defer globalInnerVoiceStore.mu.Unlock()
	globalInnerVoiceStore.states = make(map[string]*innerVoiceState)
}

// NoteInnerVoiceUserTurn increments the user-turn counter for a session.
// This tracks real user-visible turns instead of internal loop iterations.
func NoteInnerVoiceUserTurn(sessionID string) {
	globalInnerVoiceStore.mu.Lock()
	defer globalInnerVoiceStore.mu.Unlock()
	state := getInnerVoiceStateLocked(sessionID)
	state.userTurnsSinceLast++
}

// applyInnerVoiceResult stores a newly generated inner voice thought.
func applyInnerVoiceResult(sessionID, thought, category string, confidence float64) {
	globalInnerVoiceStore.mu.Lock()
	defer globalInnerVoiceStore.mu.Unlock()
	state := getInnerVoiceStateLocked(sessionID)
	state.currentThought = thought
	state.nudgeCategory = category
	state.lastConfidence = confidence
	state.userTurnsSinceLast = 0
	state.lastGenerated = time.Now()
	state.generationCount++
}

// getInnerVoiceForPrompt returns the current inner voice text and its nudge category
// if it hasn't decayed. Returns empty strings if decayed or not available.
// Decay happens when either the turn count exceeds decayTurns OR the thought age
// exceeds decayMaxAgeSecs (time-based expiry prevents stale thoughts from lingering).
func getInnerVoiceForPrompt(sessionID string, decayTurns, decayMaxAgeSecs int) (string, string) {
	globalInnerVoiceStore.mu.Lock()
	defer globalInnerVoiceStore.mu.Unlock()
	state := getInnerVoiceStateLocked(sessionID)
	if state.currentThought == "" {
		return "", ""
	}
	// Time-based decay: expire thoughts older than decayMaxAgeSecs
	if decayMaxAgeSecs > 0 && !state.lastGenerated.IsZero() {
		age := time.Since(state.lastGenerated)
		if age > time.Duration(decayMaxAgeSecs)*time.Second {
			state.currentThought = ""
			state.nudgeCategory = ""
			state.lastConfidence = 0
			return "", ""
		}
	}
	// Turn-based decay: expire after N user turns
	if decayTurns > 0 && state.userTurnsSinceLast >= decayTurns {
		// Decayed — clear stale thought
		state.currentThought = ""
		state.nudgeCategory = ""
		state.lastConfidence = 0
		return "", ""
	}
	return state.currentThought, state.nudgeCategory
}

// shouldGenerateInnerVoice determines whether the inner voice system should trigger.
// It evaluates rate limits, session caps, and situational triggers.
func shouldGenerateInnerVoice(
	sessionID string,
	cfg *config.Config,
	consecutiveErrors int,
	totalErrors int,
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

	globalInnerVoiceStore.mu.Lock()
	state := getInnerVoiceStateLocked(sessionID)
	elapsed := time.Since(state.lastGenerated)
	genCount := state.generationCount
	userTurnsSinceLast := state.userTurnsSinceLast
	globalInnerVoiceStore.mu.Unlock()

	// Session cap
	if genCount >= cfg.Personality.InnerVoice.MaxPerSession {
		return false
	}

	// Rate limiting
	minInterval := time.Duration(cfg.Personality.InnerVoice.MinIntervalSecs) * time.Second
	if elapsed < minInterval {
		return false
	}

	// Trigger: error streak
	if consecutiveErrors >= cfg.Personality.InnerVoice.ErrorStreakMin {
		return true
	}

	// Trigger: task completed after errors (recovery) — consecutiveErrors is 0 when recovered,
	// so use totalErrors to check whether any errors occurred this session.
	if taskCompleted && totalErrors > 0 {
		return true
	}

	// Trigger: periodic — generate inner voice during normal conversation (no errors required).
	// Fire when no thought has been generated yet this session, or when the current thought
	// has aged past the decay threshold (meaning the last thought already faded from the prompt).
	periodicThreshold := cfg.Personality.InnerVoice.DecayTurns
	if periodicThreshold <= 0 {
		periodicThreshold = 3
	}
	if genCount == 0 || userTurnsSinceLast >= periodicThreshold {
		return true
	}

	return false
}

func normalizeInnerVoiceCategory(category string) string {
	category = strings.ToLower(strings.TrimSpace(category))
	if category == "" {
		return "reflection"
	}
	valid := strings.Split(memory.InnerVoiceNudgeCategories, ",")
	for _, candidate := range valid {
		if strings.TrimSpace(candidate) == category {
			return category
		}
	}
	return "reflection"
}

func innerVoiceCommandPhrases() []string {
	return []string{
		"you must", "you need to", "you have to", "you should",
		"let's ", "always ", "never ", "only right path",
		"definitely ", "certainly ", "ich muss", "ich sollte", "du solltest", "du musst",
	}
}

func normalizeInnerVoiceText(thought string) string {
	thought = strings.TrimSpace(thought)
	thought = strings.Trim(thought, `"'`)
	thought = innerVoiceWhitespaceRx.ReplaceAllString(thought, " ")
	runes := []rune(thought)
	if len(runes) > 160 {
		thought = strings.TrimSpace(string(runes[:157])) + "..."
	}
	return thought
}

func normalizeInnerVoice(sessionID, thought, category string, confidence float64) (string, string, float64, bool, string) {
	thought = normalizeInnerVoiceText(thought)
	if thought == "" {
		return "", "", 0, false, "empty"
	}

	if math.IsNaN(confidence) || math.IsInf(confidence, 0) || confidence <= 0 {
		confidence = 0.7
	}
	if confidence < 0.55 {
		return "", "", confidence, false, "low_confidence"
	}

	lower := strings.ToLower(thought)
	if strings.Contains(thought, "\n- ") || strings.Contains(thought, "\n1.") {
		return "", "", confidence, false, "checklist_like"
	}
	for _, phrase := range innerVoiceCommandPhrases() {
		if strings.Contains(lower, phrase) {
			return "", "", confidence, false, "command_tone"
		}
	}

	globalInnerVoiceStore.mu.Lock()
	currentThought := getInnerVoiceStateLocked(sessionID).currentThought
	globalInnerVoiceStore.mu.Unlock()
	if currentThought != "" && strings.EqualFold(normalizeInnerVoiceText(currentThought), thought) {
		return "", "", confidence, false, "duplicate"
	}

	return thought, normalizeInnerVoiceCategory(category), confidence, true, "accepted"
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
	recentToolNames []string,
) {
	input.InnerVoiceEnabled = true
	input.ConversationTurns = conversationTurns
	input.RecoveryAttempts = recoveryAttempts
	input.TaskStatus = deriveTaskStatus(consecutiveErrors, totalErrors, totalSuccesses)
	if len(lessons) > 3 {
		lessons = lessons[:3]
	}
	input.RelevantLessons = lessons
	input.RecentToolUsage = summarizeToolUsage(recentToolNames)
	input.ConversationPhase = deriveConversationPhase(conversationTurns, consecutiveErrors, totalSuccesses)
}

// summarizeToolUsage creates a compact summary of recently used tools.
func summarizeToolUsage(toolNames []string) string {
	if len(toolNames) == 0 {
		return "none"
	}
	counts := make(map[string]int)
	order := make([]string, 0, 8)
	for _, name := range toolNames {
		if _, ok := counts[name]; !ok {
			order = append(order, name)
		}
		counts[name]++
	}
	var parts []string
	for _, name := range order {
		if len(parts) >= 6 {
			break
		}
		if counts[name] > 1 {
			parts = append(parts, fmt.Sprintf("%s(%d)", name, counts[name]))
		} else {
			parts = append(parts, name)
		}
	}
	return strings.Join(parts, ", ")
}

// deriveConversationPhase estimates the current conversation phase from signals.
func deriveConversationPhase(turns, consecutiveErrors, totalSuccesses int) string {
	if turns == 0 {
		return "idle"
	}
	if turns <= 2 {
		return "opening"
	}
	if consecutiveErrors >= 2 {
		return "struggling"
	}
	if totalSuccesses > 0 && turns > 5 {
		return "closing"
	}
	if totalSuccesses > 0 {
		return "execution"
	}
	return "exploration"
}
