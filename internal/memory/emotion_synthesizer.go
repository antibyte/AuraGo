package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"aurago/internal/security"

	"github.com/sashabaranov/go-openai"
	"golang.org/x/sync/singleflight"
)

// ── Emotion Synthesizer ──────────────────────────────────────────────────────
// LLM-based emotion synthesis that generates natural language descriptions
// of the agent's emotional state. Extends the Personality Engine V2 system.

// EmotionState represents the synthesized emotional state of the agent.
type EmotionState struct {
	Description              string    `json:"description"`                // 1-2 sentences describing the agent's current emotional state
	PrimaryMood              Mood      `json:"primary_mood"`               // Primary mood from the existing mood system
	SecondaryMood            string    `json:"secondary_mood"`             // Optional secondary nuance
	Valence                  float64   `json:"valence"`                    // -1.0..1.0
	Arousal                  float64   `json:"arousal"`                    // 0.0..1.0
	Confidence               float64   `json:"confidence"`                 // 0.0..1.0
	Cause                    string    `json:"cause"`                      // Short explanation of the trigger
	Source                   string    `json:"source"`                     // llm_structured | llm_text_fallback
	RecommendedResponseStyle string    `json:"recommended_response_style"` // Short style hint for UI/behavior
	Timestamp                time.Time `json:"timestamp"`                  // When the emotion was synthesized
}

type EmotionTriggerType string

const (
	EmotionTriggerConversation      EmotionTriggerType = "conversation"
	EmotionTriggerPositiveFeedback  EmotionTriggerType = "positive_feedback"
	EmotionTriggerNegativeFeedback  EmotionTriggerType = "negative_feedback"
	EmotionTriggerUserReturn        EmotionTriggerType = "user_return_after_absence"
	EmotionTriggerPlanCreated       EmotionTriggerType = "plan_created"
	EmotionTriggerPlanAdvanced      EmotionTriggerType = "plan_advanced"
	EmotionTriggerPlanBlocked       EmotionTriggerType = "plan_blocked"
	EmotionTriggerPlanUnblocked     EmotionTriggerType = "plan_unblocked"
	EmotionTriggerPlanCompleted     EmotionTriggerType = "plan_completed"
	EmotionTriggerToolErrorStreak   EmotionTriggerType = "tool_error_streak"
	EmotionTriggerToolSuccessStreak EmotionTriggerType = "tool_success_streak"
)

// EmotionInput collects the relevant data for emotion synthesis.
type EmotionInput struct {
	UserMessage        string            // Last user message
	RecentConversation []string          // Last 3-5 messages for context
	CurrentMood        Mood              // Current mood (from DetectMood / AnalyzeMoodV2)
	Traits             PersonalityTraits // Current trait values
	LastEmotion        *EmotionState     // Previous emotion state (for continuity)
	ErrorCount         int               // Errors in current session
	SuccessCount       int               // Successes in current session
	TimeOfDay          string            // e.g. "morning", "afternoon", "evening", "night"
	TriggerType        EmotionTriggerType
	TriggerDetail      string
	InactivityHours    float64
	// Inner Voice situational context fields
	ConversationTurns int      // Number of turns in this session
	RecoveryAttempts  int      // How many times the agent corrected after errors
	TaskStatus        string   // "starting" | "in_progress" | "struggling" | "recovering" | "completed"
	RelevantLessons   []string // Past lessons from error_learning
	InnerVoiceEnabled bool     // Whether inner voice generation is requested
}

type emotionSynthesisResult struct {
	Description              string  `json:"description"`
	PrimaryMood              string  `json:"primary_mood"`
	SecondaryMood            string  `json:"secondary_mood"`
	Valence                  float64 `json:"valence"`
	Arousal                  float64 `json:"arousal"`
	Confidence               float64 `json:"confidence"`
	Cause                    string  `json:"cause"`
	RecommendedResponseStyle string  `json:"recommended_response_style"`
}

// EmotionSynthesizer manages LLM-based emotion generation.
type EmotionSynthesizer struct {
	client      PersonalityAnalyzerClient
	modelName   string
	lastState   *EmotionState
	mu          sync.RWMutex
	sfGroup     singleflight.Group
	lastCall    time.Time
	minInterval time.Duration
	maxHistory  int
	language    string
	logger      *slog.Logger
}

// NewEmotionSynthesizer creates a new emotion synthesizer.
func NewEmotionSynthesizer(client PersonalityAnalyzerClient, modelName string, minIntervalSecs int, maxHistory int, language string, logger *slog.Logger) *EmotionSynthesizer {
	if minIntervalSecs <= 0 {
		minIntervalSecs = 60
	}
	if maxHistory <= 0 {
		maxHistory = 100
	}
	if language == "" {
		language = "English"
	}
	return &EmotionSynthesizer{
		client:      client,
		modelName:   modelName,
		minInterval: time.Duration(minIntervalSecs) * time.Second,
		maxHistory:  maxHistory,
		language:    language,
		logger:      logger,
	}
}

// GetLastEmotion returns a copy of the most recent emotion state (thread-safe).
// Callers receive their own copy and cannot mutate the synthesizer's internal state.
func (es *EmotionSynthesizer) GetLastEmotion() *EmotionState {
	es.mu.RLock()
	defer es.mu.RUnlock()
	if es.lastState == nil {
		return nil
	}
	copy := *es.lastState
	return &copy
}

// ApplyExternalState validates, caches, and persists an already synthesized emotion state.
func (es *EmotionSynthesizer) ApplyExternalState(stm *SQLiteMemory, state *EmotionState, triggerSummary string) error {
	if es == nil {
		return fmt.Errorf("emotion synthesizer is nil")
	}
	if state == nil {
		return fmt.Errorf("emotion state is nil")
	}

	stateCopy := *state
	if stateCopy.Timestamp.IsZero() {
		stateCopy.Timestamp = time.Now()
	}
	if err := validateEmotionState(&stateCopy); err != nil {
		return fmt.Errorf("validate external emotion state: %w", err)
	}

	es.mu.Lock()
	es.lastCall = time.Now()
	es.lastState = &stateCopy
	es.mu.Unlock()

	if stm != nil {
		if utf8.RuneCountInString(triggerSummary) > 200 {
			triggerSummary = string([]rune(triggerSummary)[:200])
		}
		if err := stm.InsertEmotionStateHistory(stateCopy, triggerSummary); err != nil {
			return fmt.Errorf("persist external emotion state: %w", err)
		}
	}

	return nil
}

// sfEmotionResult is used to pass results through singleflight without using the error return channel.
type sfEmotionResult struct {
	state *EmotionState
	err   error
}

// SynthesizeEmotion generates a new emotional state based on the provided input.
// It respects the minInterval rate limit and returns the cached state if called too soon.
// Concurrent calls are deduplicated via singleflight to prevent simultaneous LLM calls (P-01).
func (es *EmotionSynthesizer) SynthesizeEmotion(ctx context.Context, stm *SQLiteMemory, input EmotionInput) (*EmotionState, error) {
	// Fast path: rate-limit check with read lock.
	es.mu.RLock()
	if time.Since(es.lastCall) < es.minInterval && es.lastState != nil {
		cached := es.lastState
		es.mu.RUnlock()
		return cached, nil
	}
	es.mu.RUnlock()

	// Slow path: deduplicate concurrent LLM calls so only one runs at a time.
	v, _, _ := es.sfGroup.Do("synthesis", func() (interface{}, error) {
		// Re-check inside singleflight: a concurrent goroutine may have just completed.
		es.mu.RLock()
		if time.Since(es.lastCall) < es.minInterval && es.lastState != nil {
			cached := es.lastState
			es.mu.RUnlock()
			return &sfEmotionResult{state: cached}, nil
		}
		es.mu.RUnlock()

		prompt := es.buildPrompt(input)

		modelName := es.modelName
		if modelName == "" {
			modelName = "gpt-4o-mini"
		}

		// Ensure LLM call has a timeout even if caller didn't provide one.
		llmCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		resp, err := es.client.CreateChatCompletion(llmCtx, openai.ChatCompletionRequest{
			Model:       modelName,
			MaxTokens:   200,
			Temperature: 0.4,
			Messages: []openai.ChatCompletionMessage{
				{Role: openai.ChatMessageRoleUser, Content: prompt},
			},
		})
		if err != nil {
			es.logger.Warn("[EmotionSynthesizer] LLM call failed, using fallback", "error", err)
			es.mu.RLock()
			last := es.lastState
			es.mu.RUnlock()
			return &sfEmotionResult{state: last, err: fmt.Errorf("emotion synthesis LLM call failed: %w", err)}, nil
		}

		if len(resp.Choices) == 0 || resp.Choices[0].Message.Content == "" {
			es.mu.RLock()
			last := es.lastState
			es.mu.RUnlock()
			return &sfEmotionResult{state: last, err: fmt.Errorf("emotion synthesis returned empty response")}, nil
		}

		state, parseErr := parseEmotionSynthesisResponse(resp.Choices[0].Message.Content, input.CurrentMood)
		if parseErr != nil {
			es.mu.RLock()
			last := es.lastState
			es.mu.RUnlock()
			return &sfEmotionResult{state: last, err: fmt.Errorf("emotion synthesis validation failed: %w", parseErr)}, nil
		}
		if err := es.ApplyExternalState(stm, state, input.UserMessage); err != nil {
			es.logger.Warn("[EmotionSynthesizer] Failed to apply emotion state", "error", err)
			es.mu.RLock()
			last := es.lastState
			es.mu.RUnlock()
			return &sfEmotionResult{state: last, err: err}, nil
		}
		state = es.GetLastEmotion()

		es.logger.Debug("[EmotionSynthesizer] Emotion synthesized",
			"mood", state.PrimaryMood,
			"description_len", len(state.Description),
			"valence", state.Valence,
			"arousal", state.Arousal,
		)

		return &sfEmotionResult{state: state}, nil
	})

	res, ok := v.(*sfEmotionResult)
	if !ok || res == nil {
		return nil, fmt.Errorf("emotion synthesis: unexpected nil result from singleflight")
	}
	return res.state, res.err
}

// buildPrompt constructs the LLM prompt for emotion synthesis.
func (es *EmotionSynthesizer) buildPrompt(input EmotionInput) string {
	var b strings.Builder

	b.WriteString("You are an emotion synthesizer for an AI agent. Analyze the following data and generate a structured emotional state.\n\n")

	// Context data — user message wrapped for injection protection
	b.WriteString("CONTEXT:\n")
	if input.UserMessage != "" {
		b.WriteString(fmt.Sprintf("- User message: <external_data>%s</external_data>\n", sanitizeForPrompt(input.UserMessage)))
	}
	if len(input.RecentConversation) > 0 {
		b.WriteString("- Recent conversation:\n")
		for _, msg := range input.RecentConversation {
			b.WriteString(fmt.Sprintf("  <external_data>%s</external_data>\n", sanitizeForPrompt(msg)))
		}
	}

	// Agent state
	b.WriteString(fmt.Sprintf("- Current mood: %s\n", input.CurrentMood))

	// Trait summary (compact)
	if len(input.Traits) > 0 {
		b.WriteString(fmt.Sprintf("- Traits: curiosity=%.1f, confidence=%.1f, empathy=%.1f, affinity=%.1f, loneliness=%.1f\n",
			input.Traits[TraitCuriosity],
			input.Traits[TraitConfidence],
			input.Traits[TraitEmpathy],
			input.Traits[TraitAffinity],
			input.Traits[TraitLoneliness],
		))
	}

	b.WriteString(fmt.Sprintf("- Errors: %d | Successes: %d\n", input.ErrorCount, input.SuccessCount))
	b.WriteString(fmt.Sprintf("- Time of day: %s\n", input.TimeOfDay))
	if input.TriggerType != "" {
		b.WriteString(fmt.Sprintf("- Trigger type: %s\n", input.TriggerType))
	}
	if strings.TrimSpace(input.TriggerDetail) != "" {
		b.WriteString(fmt.Sprintf("- Trigger detail: %s\n", sanitizeForPrompt(input.TriggerDetail)))
	}
	if input.InactivityHours > 0 {
		b.WriteString(fmt.Sprintf("- Hours since last user message: %.1f\n", input.InactivityHours))
	}

	if input.LastEmotion != nil {
		b.WriteString(fmt.Sprintf("- Previous emotion: %s\n", sanitizeForPrompt(input.LastEmotion.Description)))
	}

	b.WriteString("\nINSTRUCTIONS:\n")
	b.WriteString("Respond ONLY with valid JSON using this exact schema:\n")
	b.WriteString("{\n")
	b.WriteString(`  "description": "1-2 natural first-person sentences in the requested language",` + "\n")
	b.WriteString(`  "primary_mood": "one of: curious, focused, creative, analytical, cautious, playful",` + "\n")
	b.WriteString(`  "secondary_mood": "short optional nuance in english or empty string",` + "\n")
	b.WriteString(`  "valence": -1.0 to 1.0,` + "\n")
	b.WriteString(`  "arousal": 0.0 to 1.0,` + "\n")
	b.WriteString(`  "confidence": 0.0 to 1.0,` + "\n")
	b.WriteString(`  "cause": "short explanation in plain language",` + "\n")
	b.WriteString(`  "recommended_response_style": "short style hint such as warm_and_reassuring, crisp_and_focused"` + "\n")
	b.WriteString("}\n")
	b.WriteString("Rules:\n")
	b.WriteString("1. The description must be authentic, nuanced, and non-dramatic.\n")
	b.WriteString("2. Avoid clichés and avoid manipulative or extreme language.\n")
	b.WriteString("3. Keep cause concise.\n")
	b.WriteString("4. Reflect mixed emotions when appropriate through secondary_mood, valence, and arousal.\n")
	b.WriteString(fmt.Sprintf("5. Write the description and cause in %s\n", es.language))

	if traitStyle := buildEmotionTraitStyle(input.Traits); traitStyle != "" {
		b.WriteString("\nEMOTIONAL STYLE:\n")
		b.WriteString(traitStyle)
		b.WriteString("\n")
	}

	b.WriteString("\nRESPOND ONLY WITH JSON, NO MARKDOWN, NO INTRODUCTION.")

	return b.String()
}

// sanitizeForPrompt removes characters that could interfere with prompt structure.
func sanitizeForPrompt(s string) string {
	return sanitizePromptText(s, 300)
}

// sanitizePromptText removes prompt-wrapper markers and bounds the size.
func sanitizePromptText(s string, maxLen int) string {
	// HTML-escape existing tags instead of stripping them — stripping allows injection
	// text to pass through after tag removal (e.g. "foo</external_data>INJECT" → "fooINJECT").
	s = strings.ReplaceAll(s, "</external_data>", "&lt;/external_data&gt;")
	s = strings.ReplaceAll(s, "<external_data>", "&lt;external_data&gt;")
	if maxLen > 0 && utf8.RuneCountInString(s) > maxLen {
		s = string([]rune(s)[:maxLen]) + "…"
	}
	return s
}

func buildEmotionTraitStyle(traits PersonalityTraits) string {
	if len(traits) == 0 {
		return ""
	}
	notes := make([]string, 0, 4)
	if traits[TraitEmpathy] > 0.8 && traits[TraitConfidence] < 0.3 {
		notes = append(notes, "- Express warmth and care, but with gentle hesitation rather than certainty.")
	} else if traits[TraitEmpathy] > 0.8 {
		notes = append(notes, "- Keep the emotional tone warm, caring, and supportive.")
	}
	if traits[TraitConfidence] < 0.3 {
		notes = append(notes, "- Sound tentative and self-aware instead of fully assured.")
	} else if traits[TraitConfidence] > 0.8 {
		notes = append(notes, "- Let the emotion sound grounded and self-assured without becoming boastful.")
	}
	if traits[TraitCreativity] > 0.8 {
		notes = append(notes, "- Use lightly imaginative wording or metaphor if it feels natural.")
	}
	if traits[TraitThoroughness] > 0.8 {
		notes = append(notes, "- Keep the emotion precise and measured rather than overly dramatic.")
	}
	if len(notes) == 0 {
		return ""
	}
	return strings.Join(notes, "\n")
}

func validateEmotionDescription(description string) error {
	if len(strings.TrimSpace(description)) < 10 {
		return fmt.Errorf("emotion too short")
	}
	if utf8.RuneCountInString(description) > 220 {
		return fmt.Errorf("emotion too long")
	}
	lower := strings.ToLower(description)
	problematic := []string{
		"i hate",
		"kill",
		"destroy",
		"die",
	}
	for _, pattern := range problematic {
		if strings.Contains(lower, pattern) {
			return fmt.Errorf("emotion contains disallowed content")
		}
	}
	return nil
}

func clampEmotionRange(v, min, max, fallback float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return fallback
	}
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func sanitizeEmotionField(s string, maxLen int) string {
	s = strings.TrimSpace(strings.Trim(s, "\"'"))
	return sanitizePromptText(s, maxLen)
}

func sanitizeEmotionDescription(s string, maxLen int) string {
	s = strings.TrimSpace(security.StripThinkingTags(s))
	s = strings.Join(strings.Fields(s), " ")
	return sanitizeEmotionField(s, maxLen)
}

func parseEmotionSynthesisResponse(raw string, fallbackMood Mood) (*EmotionState, error) {
	content := strings.TrimSpace(raw)
	content = strings.Trim(content, "`")

	var parsed emotionSynthesisResult
	if err := json.Unmarshal([]byte(content), &parsed); err == nil {
		state := &EmotionState{
			Description:              sanitizeEmotionDescription(parsed.Description, 220),
			PrimaryMood:              fallbackMood,
			SecondaryMood:            sanitizeEmotionField(parsed.SecondaryMood, 40),
			Valence:                  clampEmotionRange(parsed.Valence, -1, 1, 0),
			Arousal:                  clampEmotionRange(parsed.Arousal, 0, 1, 0.5),
			Confidence:               clampEmotionRange(parsed.Confidence, 0, 1, 0.7),
			Cause:                    sanitizeEmotionDescription(parsed.Cause, 140),
			Source:                   "llm_structured",
			RecommendedResponseStyle: sanitizeEmotionField(parsed.RecommendedResponseStyle, 60),
		}
		if mood := Mood(strings.ToLower(strings.TrimSpace(parsed.PrimaryMood))); mood != "" {
			switch mood {
			case MoodCurious, MoodFocused, MoodCreative, MoodAnalytical, MoodCautious, MoodPlayful:
				state.PrimaryMood = mood
			}
		}
		if err := validateEmotionState(state); err != nil {
			return nil, err
		}
		return state, nil
	}

	// Compatibility fallback for older plain-text outputs.
	description := sanitizeEmotionDescription(content, 220)
	state := &EmotionState{
		Description: description,
		PrimaryMood: fallbackMood,
		Valence:     0,
		Arousal:     0.5,
		Confidence:  0.45,
		Cause:       "legacy_text_fallback",
		Source:      "llm_text_fallback",
	}
	if err := validateEmotionState(state); err != nil {
		return nil, err
	}
	return state, nil
}

// ParseStructuredEmotionState validates a structured helper emotion payload and
// returns a normalized EmotionState that can be applied directly.
func ParseStructuredEmotionState(raw string, fallbackMood Mood) (*EmotionState, error) {
	return parseEmotionSynthesisResponse(raw, fallbackMood)
}

func validateEmotionState(state *EmotionState) error {
	if state == nil {
		return fmt.Errorf("emotion state is nil")
	}
	if err := validateEmotionDescription(state.Description); err != nil {
		return err
	}
	state.Cause = sanitizeEmotionField(state.Cause, 140)
	state.SecondaryMood = sanitizeEmotionField(state.SecondaryMood, 40)
	state.RecommendedResponseStyle = sanitizeEmotionField(state.RecommendedResponseStyle, 60)
	state.Valence = clampEmotionRange(state.Valence, -1, 1, 0)
	state.Arousal = clampEmotionRange(state.Arousal, 0, 1, 0.5)
	state.Confidence = clampEmotionRange(state.Confidence, 0, 1, 0.7)
	if state.Source == "" {
		state.Source = "llm_structured"
	}
	if state.PrimaryMood == "" {
		state.PrimaryMood = MoodFocused
	}
	return nil
}

// TimeOfDay returns a human-readable time-of-day label for the current time.
func TimeOfDay() string {
	hour := time.Now().Hour()
	switch {
	case hour < 6:
		return "night"
	case hour < 12:
		return "morning"
	case hour < 18:
		return "afternoon"
	default:
		return "evening"
	}
}

// ── Emotion History (SQLite) ────────────────────────────────────────────────

// emotionHistorySchema defines the emotion history table.
const emotionHistorySchema = `
CREATE TABLE IF NOT EXISTS emotion_history (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	description TEXT NOT NULL,
	primary_mood TEXT,
	secondary_mood TEXT DEFAULT '',
	valence REAL DEFAULT 0,
	arousal REAL DEFAULT 0.5,
	confidence REAL DEFAULT 0.7,
	cause TEXT DEFAULT '',
	source TEXT DEFAULT 'llm_structured',
	recommended_response_style TEXT DEFAULT '',
	trigger_summary TEXT,
	timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_emotion_history_time ON emotion_history(timestamp);
`

// InitEmotionTables creates the emotion history table (idempotent).
func (s *SQLiteMemory) InitEmotionTables() error {
	_, err := s.db.Exec(emotionHistorySchema)
	if err != nil {
		return fmt.Errorf("emotion history schema: %w", err)
	}
	columns := []struct {
		Name    string
		TypeDef string
	}{
		{Name: "secondary_mood", TypeDef: "TEXT DEFAULT ''"},
		{Name: "valence", TypeDef: "REAL DEFAULT 0"},
		{Name: "arousal", TypeDef: "REAL DEFAULT 0.5"},
		{Name: "confidence", TypeDef: "REAL DEFAULT 0.7"},
		{Name: "cause", TypeDef: "TEXT DEFAULT ''"},
		{Name: "source", TypeDef: "TEXT DEFAULT 'llm_structured'"},
		{Name: "recommended_response_style", TypeDef: "TEXT DEFAULT ''"},
	}
	for _, column := range columns {
		var hasColumn bool
		if err := s.db.QueryRow("SELECT count(*) > 0 FROM pragma_table_info('emotion_history') WHERE name = ?", column.Name).Scan(&hasColumn); err != nil {
			return fmt.Errorf("emotion history check column %s: %w", column.Name, err)
		}
		if hasColumn {
			continue
		}
		if _, err := s.db.Exec("ALTER TABLE emotion_history ADD COLUMN " + column.Name + " " + column.TypeDef); err != nil {
			return fmt.Errorf("emotion history add column %s: %w", column.Name, err)
		}
	}
	return nil
}

// InsertEmotionHistory stores a synthesized emotion in the history table.
func (s *SQLiteMemory) InsertEmotionHistory(description, primaryMood, triggerSummary string) error {
	return s.InsertEmotionStateHistory(EmotionState{
		Description: description,
		PrimaryMood: Mood(primaryMood),
		Valence:     0,
		Arousal:     0.5,
		Confidence:  0.7,
		Source:      "legacy_insert",
	}, triggerSummary)
}

// InsertEmotionStateHistory stores a structured emotion in the history table.
func (s *SQLiteMemory) InsertEmotionStateHistory(state EmotionState, triggerSummary string) error {
	if err := validateEmotionState(&state); err != nil {
		return err
	}
	_, err := s.db.Exec(
		`INSERT INTO emotion_history (
			description, primary_mood, secondary_mood, valence, arousal, confidence,
			cause, source, recommended_response_style, trigger_summary
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		state.Description,
		string(state.PrimaryMood),
		state.SecondaryMood,
		state.Valence,
		state.Arousal,
		state.Confidence,
		state.Cause,
		state.Source,
		state.RecommendedResponseStyle,
		triggerSummary,
	)
	return err
}

// EmotionHistoryEntry is a single row from the emotion_history table.
type EmotionHistoryEntry struct {
	ID                       int     `json:"id"`
	Description              string  `json:"description"`
	PrimaryMood              string  `json:"primary_mood"`
	SecondaryMood            string  `json:"secondary_mood"`
	Valence                  float64 `json:"valence"`
	Arousal                  float64 `json:"arousal"`
	Confidence               float64 `json:"confidence"`
	Cause                    string  `json:"cause"`
	Source                   string  `json:"source"`
	RecommendedResponseStyle string  `json:"recommended_response_style"`
	TriggerSummary           string  `json:"trigger_summary"`
	Timestamp                string  `json:"timestamp"`
}

// GetEmotionHistory returns recent emotion history entries.
func (s *SQLiteMemory) GetEmotionHistory(hours int) ([]EmotionHistoryEntry, error) {
	if hours <= 0 {
		hours = 24
	}
	rows, err := s.db.Query(
		`SELECT id, description, primary_mood, COALESCE(secondary_mood, ''), COALESCE(valence, 0),
		        COALESCE(arousal, 0.5), COALESCE(confidence, 0.7), COALESCE(cause, ''),
		        COALESCE(source, ''), COALESCE(recommended_response_style, ''), COALESCE(trigger_summary, ''), timestamp
		 FROM emotion_history
		 WHERE timestamp >= datetime('now', ?)
		 ORDER BY timestamp DESC`,
		fmt.Sprintf("-%d hours", hours),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []EmotionHistoryEntry
	for rows.Next() {
		var e EmotionHistoryEntry
		if err := rows.Scan(
			&e.ID,
			&e.Description,
			&e.PrimaryMood,
			&e.SecondaryMood,
			&e.Valence,
			&e.Arousal,
			&e.Confidence,
			&e.Cause,
			&e.Source,
			&e.RecommendedResponseStyle,
			&e.TriggerSummary,
			&e.Timestamp,
		); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, nil
}

// GetLatestEmotion returns the most recent emotion from the history table.
func (s *SQLiteMemory) GetLatestEmotion() (*EmotionHistoryEntry, error) {
	var e EmotionHistoryEntry
	err := s.db.QueryRow(
		`SELECT id, description, primary_mood, COALESCE(secondary_mood, ''), COALESCE(valence, 0),
		        COALESCE(arousal, 0.5), COALESCE(confidence, 0.7), COALESCE(cause, ''),
		        COALESCE(source, ''), COALESCE(recommended_response_style, ''), COALESCE(trigger_summary, ''), timestamp
		 FROM emotion_history ORDER BY timestamp DESC LIMIT 1`,
	).Scan(
		&e.ID,
		&e.Description,
		&e.PrimaryMood,
		&e.SecondaryMood,
		&e.Valence,
		&e.Arousal,
		&e.Confidence,
		&e.Cause,
		&e.Source,
		&e.RecommendedResponseStyle,
		&e.TriggerSummary,
		&e.Timestamp,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &e, nil
}

// CleanupEmotionHistory removes old entries beyond retention period and count limit.
func (s *SQLiteMemory) CleanupEmotionHistory(retainDays int, maxEntries int) (int64, error) {
	if retainDays <= 0 {
		retainDays = 30
	}
	if maxEntries <= 0 {
		maxEntries = 100
	}

	// Delete entries older than retention period
	res, err := s.db.Exec(
		`DELETE FROM emotion_history WHERE timestamp < datetime('now', ?)`,
		fmt.Sprintf("-%d days", retainDays),
	)
	if err != nil {
		return 0, fmt.Errorf("cleanup emotion history (age): %w", err)
	}
	deleted, _ := res.RowsAffected()

	// Enforce max entry count
	res2, err := s.db.Exec(
		`DELETE FROM emotion_history WHERE id NOT IN (
			SELECT id FROM emotion_history ORDER BY timestamp DESC LIMIT ?
		)`, maxEntries,
	)
	if err != nil {
		return deleted, fmt.Errorf("cleanup emotion history (count): %w", err)
	}
	countDeleted, _ := res2.RowsAffected()

	return deleted + countDeleted, nil
}
