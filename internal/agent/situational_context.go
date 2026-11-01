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

	"github.com/sashabaranov/go-openai"
)

// innerVoiceState tracks runtime state for the inner voice system within a session.
type innerVoiceState struct {
	lastGenerated      time.Time
	lastRequested      time.Time
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

const (
	desktopInnerVoiceSessionID     = "virtual-desktop"
	desktopInnerVoiceMinInterval   = 15 * time.Minute
	desktopInnerVoiceMaxPerSession = 2
	desktopInnerVoiceRecoveryTurns = 1
)

func normalizeInnerVoiceSessionID(sessionID string) string {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return "default"
	}
	return sessionID
}

func isDesktopInnerVoiceSession(sessionID string) bool {
	return normalizeInnerVoiceSessionID(sessionID) == desktopInnerVoiceSessionID
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

// ResetInnerVoiceSession resets the inner voice state for a single session.
func ResetInnerVoiceSession(sessionID string) {
	globalInnerVoiceStore.mu.Lock()
	defer globalInnerVoiceStore.mu.Unlock()
	delete(globalInnerVoiceStore.states, normalizeInnerVoiceSessionID(sessionID))
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

func reserveInnerVoiceGeneration(sessionID string, maxPerSession int, minInterval time.Duration, now time.Time) bool {
	globalInnerVoiceStore.mu.Lock()
	defer globalInnerVoiceStore.mu.Unlock()
	state := getInnerVoiceStateLocked(sessionID)
	if maxPerSession > 0 && state.generationCount >= maxPerSession {
		return false
	}
	if minInterval > 0 {
		if !state.lastGenerated.IsZero() && now.Sub(state.lastGenerated) < minInterval {
			return false
		}
		if !state.lastRequested.IsZero() && now.Sub(state.lastRequested) < minInterval {
			return false
		}
	}
	state.lastRequested = now
	return true
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

func shouldInjectInnerVoiceIntoPrompt(cfg *config.Config, consecutiveErrors int, isMission bool, isCoAgent bool, lastResponseWasTool bool) bool {
	if cfg == nil || !cfg.Personality.InnerVoice.Enabled {
		return false
	}
	if isMission || isCoAgent {
		return false
	}
	if lastResponseWasTool {
		return false
	}
	return consecutiveErrors == 0
}

// releaseInnerVoiceReservation frees the generation slot reserved for a turn whose
// helper batch failed, so the V2 fallback of the same turn can request the thought.
func releaseInnerVoiceReservation(sessionID string) {
	globalInnerVoiceStore.mu.Lock()
	defer globalInnerVoiceStore.mu.Unlock()
	getInnerVoiceStateLocked(sessionID).lastRequested = time.Time{}
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

	desktopSession := isDesktopInnerVoiceSession(sessionID)
	now := time.Now()
	maxPerSession := cfg.Personality.InnerVoice.MaxPerSession
	if desktopSession && (maxPerSession <= 0 || maxPerSession > desktopInnerVoiceMaxPerSession) {
		maxPerSession = desktopInnerVoiceMaxPerSession
	}
	minInterval := time.Duration(cfg.Personality.InnerVoice.MinIntervalSecs) * time.Second
	if desktopSession && minInterval < desktopInnerVoiceMinInterval {
		minInterval = desktopInnerVoiceMinInterval
	}

	globalInnerVoiceStore.mu.Lock()
	state := getInnerVoiceStateLocked(sessionID)
	genCount := state.generationCount
	userTurnsSinceLast := state.userTurnsSinceLast
	globalInnerVoiceStore.mu.Unlock()

	// Session cap
	if maxPerSession > 0 && genCount >= maxPerSession {
		return false
	}

	// Active error recovery must stay procedural. Inner voice text is injected into
	// the next system prompt, so generating it during a failing tool chain can turn
	// transient frustration into instructions for the next recovery step.
	if consecutiveErrors > 0 {
		return false
	}

	// Trigger: task completed after errors (recovery) — consecutiveErrors is 0 when recovered,
	// so use totalErrors to check whether any errors occurred this session.
	if taskCompleted && totalErrors > 0 && successCount > 0 {
		if desktopSession && userTurnsSinceLast < desktopInnerVoiceRecoveryTurns {
			return false
		}
		return reserveInnerVoiceGeneration(sessionID, maxPerSession, minInterval, now)
	}

	// Desktop chat is a high-frequency tool surface. Keep inner voice out of normal
	// Desktop turns and allow it only for rare post-error recovery summaries.
	if desktopSession {
		return false
	}

	// Trigger: periodic — generate inner voice during normal conversation (no errors required).
	// Fire when no thought has been generated yet this session, or when the current thought
	// has aged past the decay threshold (meaning the last thought already faded from the prompt).
	periodicThreshold := cfg.Personality.InnerVoice.DecayTurns
	if periodicThreshold <= 0 {
		periodicThreshold = 3
	}
	if genCount == 0 || userTurnsSinceLast >= periodicThreshold {
		return reserveInnerVoiceGeneration(sessionID, maxPerSession, minInterval, now)
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

// innerVoiceCommandRx matches obligations and imperatives only. Word boundaries keep
// words like "whenever" out, and plain intensifiers ("never", "always") are not commands.
var innerVoiceCommandRx = regexp.MustCompile(`(?i)\b(you\s+(must|need\s+to|have\s+to|should)|let['’]s|let\s+us|i\s+(must|should)|only\s+right\s+path|ich\s+(muss|sollte)|du\s+(musst|solltest)|wir\s+(müssen|sollten))\b`)

func innerVoiceUnsafeTonePhrases() []string {
	return []string{
		"fuck", "shit", "scheisse", "scheiße", "verdammt", "wahnsinnig",
		"losing it", "i am losing", "i'm losing", "ich verliere", "ich raste",
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
	// List markers are line-based: check the raw text, the normalizer joins the lines.
	checklistLike := strings.Contains(thought, "\n- ") || strings.Contains(thought, "\n1.")
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
	if checklistLike {
		return "", "", confidence, false, "checklist_like"
	}
	if innerVoiceCommandRx.MatchString(thought) {
		return "", "", confidence, false, "command_tone"
	}
	for _, phrase := range innerVoiceUnsafeTonePhrases() {
		if strings.Contains(lower, phrase) {
			return "", "", confidence, false, "unsafe_tone"
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
	input.ConversationPhase = deriveConversationPhase(conversationTurns, totalSuccesses)
}

// innerVoiceTurnToolNames returns the function names of the assistant tool calls
// after the last user message, i.e. the tools used in the current turn.
func innerVoiceTurnToolNames(msgs []openai.ChatCompletionMessage) []string {
	start := 0
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == openai.ChatMessageRoleUser && !isTextModeToolResult(msgs[i]) {
			start = i + 1
			break
		}
	}
	var names []string
	for _, msg := range msgs[start:] {
		if msg.Role != openai.ChatMessageRoleAssistant {
			continue
		}
		for _, call := range msg.ToolCalls {
			if call.Function.Name != "" {
				names = append(names, call.Function.Name)
			}
		}
	}
	return names
}

// innerVoiceUserTurns counts the user turns in msgs. Text-mode tool results are
// user-role messages but not turns.
func innerVoiceUserTurns(msgs []openai.ChatCompletionMessage) int {
	turns := 0
	for _, msg := range msgs {
		if msg.Role == openai.ChatMessageRoleUser && !isTextModeToolResult(msg) {
			turns++
		}
	}
	return turns
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

// deriveConversationPhase estimates the conversation phase from the user-turn count
// and the successful tool calls. Generation is blocked while consecutive errors
// exist, so there is no "struggling" phase here, and nothing signals a "closing".
func deriveConversationPhase(turns, totalSuccesses int) string {
	if turns <= 1 {
		return "opening"
	}
	if totalSuccesses > 0 {
		return "execution"
	}
	return "exploration"
}
