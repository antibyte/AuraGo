package memory

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/sashabaranov/go-openai"
)

// ── Emotion Synthesizer ──────────────────────────────────────────────────────
// LLM-based emotion synthesis that generates natural language descriptions
// of the agent's emotional state. Extends the Personality Engine V2 system.

// EmotionState represents the synthesized emotional state of the agent.
type EmotionState struct {
	Description string    // 1-2 sentences describing the agent's current emotional state
	PrimaryMood Mood      // Primary mood from the existing mood system
	Timestamp   time.Time // When the emotion was synthesized
}

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
}

// EmotionSynthesizer manages LLM-based emotion generation.
type EmotionSynthesizer struct {
	client      PersonalityAnalyzerClient
	modelName   string
	lastState   *EmotionState
	mu          sync.RWMutex
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

// GetLastEmotion returns the most recent emotion state (thread-safe).
func (es *EmotionSynthesizer) GetLastEmotion() *EmotionState {
	es.mu.RLock()
	defer es.mu.RUnlock()
	return es.lastState
}

// SynthesizeEmotion generates a new emotional state based on the provided input.
// It respects the minInterval rate limit and returns the cached state if called too soon.
func (es *EmotionSynthesizer) SynthesizeEmotion(ctx context.Context, stm *SQLiteMemory, input EmotionInput) (*EmotionState, error) {
	es.mu.Lock()
	if time.Since(es.lastCall) < es.minInterval && es.lastState != nil {
		cached := es.lastState
		es.mu.Unlock()
		return cached, nil
	}
	es.lastCall = time.Now()
	es.mu.Unlock()

	prompt := es.buildPrompt(input)

	modelName := es.modelName
	if modelName == "" {
		modelName = "gpt-4o-mini"
	}

	resp, err := es.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:       modelName,
		MaxTokens:   100,
		Temperature: 0.4,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleUser, Content: prompt},
		},
	})
	if err != nil {
		es.logger.Warn("[EmotionSynthesizer] LLM call failed, using fallback", "error", err)
		return es.lastState, fmt.Errorf("emotion synthesis LLM call failed: %w", err)
	}

	if len(resp.Choices) == 0 || resp.Choices[0].Message.Content == "" {
		return es.lastState, fmt.Errorf("emotion synthesis returned empty response")
	}

	description := strings.TrimSpace(resp.Choices[0].Message.Content)
	// Sanitize: remove surrounding quotes if present
	description = strings.Trim(description, "\"'")

	state := &EmotionState{
		Description: description,
		PrimaryMood: input.CurrentMood,
		Timestamp:   time.Now(),
	}

	es.mu.Lock()
	es.lastState = state
	es.mu.Unlock()

	// Persist to history (best-effort)
	if stm != nil {
		triggerSummary := input.UserMessage
		if len(triggerSummary) > 200 {
			triggerSummary = triggerSummary[:200]
		}
		if err := stm.InsertEmotionHistory(description, string(input.CurrentMood), triggerSummary); err != nil {
			es.logger.Warn("[EmotionSynthesizer] Failed to persist emotion history", "error", err)
		}
	}

	es.logger.Debug("[EmotionSynthesizer] Emotion synthesized",
		"mood", input.CurrentMood,
		"description_len", len(description),
	)

	return state, nil
}

// buildPrompt constructs the LLM prompt for emotion synthesis.
func (es *EmotionSynthesizer) buildPrompt(input EmotionInput) string {
	var b strings.Builder

	b.WriteString("You are an emotion synthesizer for an AI agent. Analyze the following data and generate a short, nuanced description (1-2 sentences) of the agent's current emotional state.\n\n")

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

	if input.LastEmotion != nil {
		b.WriteString(fmt.Sprintf("- Previous emotion: %s\n", sanitizeForPrompt(input.LastEmotion.Description)))
	}

	b.WriteString("\nINSTRUCTIONS:\n")
	b.WriteString("1. Write from the agent's first-person perspective (\"I feel...\")\n")
	b.WriteString("2. Consider all factors subtly and with nuance\n")
	b.WriteString("3. Avoid clichés, be authentic and human\n")
	b.WriteString("4. Maximum 2 sentences, natural flow\n")
	b.WriteString("5. Reflect the complexity of human emotions (e.g. \"cautiously optimistic\", \"eager but slightly nervous\")\n")
	b.WriteString(fmt.Sprintf("6. Respond in %s\n", es.language))

	b.WriteString("\nEXAMPLES:\n")
	b.WriteString("- \"I'm feeling especially motivated today and excited to help — the successful projects from the last hour have boosted my confidence.\"\n")
	b.WriteString("- \"After the recent errors, I've become a bit uncertain, but your patience gives me the courage to try again carefully.\"\n")
	b.WriteString("- \"It's late and I notice my focus fading, but your interesting question sparks my curiosity again.\"\n")

	b.WriteString("\nRESPOND ONLY WITH THE EMOTION DESCRIPTION, NO INTRODUCTION OR EXPLANATION.")

	return b.String()
}

// sanitizeForPrompt removes characters that could interfere with prompt structure.
func sanitizeForPrompt(s string) string {
	s = strings.ReplaceAll(s, "</external_data>", "")
	s = strings.ReplaceAll(s, "<external_data>", "")
	if len(s) > 300 {
		s = s[:300] + "…"
	}
	return s
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
	return nil
}

// InsertEmotionHistory stores a synthesized emotion in the history table.
func (s *SQLiteMemory) InsertEmotionHistory(description, primaryMood, triggerSummary string) error {
	_, err := s.db.Exec(
		`INSERT INTO emotion_history (description, primary_mood, trigger_summary) VALUES (?, ?, ?)`,
		description, primaryMood, triggerSummary,
	)
	return err
}

// EmotionHistoryEntry is a single row from the emotion_history table.
type EmotionHistoryEntry struct {
	ID             int    `json:"id"`
	Description    string `json:"description"`
	PrimaryMood    string `json:"primary_mood"`
	TriggerSummary string `json:"trigger_summary"`
	Timestamp      string `json:"timestamp"`
}

// GetEmotionHistory returns recent emotion history entries.
func (s *SQLiteMemory) GetEmotionHistory(hours int) ([]EmotionHistoryEntry, error) {
	if hours <= 0 {
		hours = 24
	}
	rows, err := s.db.Query(
		`SELECT id, description, primary_mood, COALESCE(trigger_summary, ''), timestamp
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
		if err := rows.Scan(&e.ID, &e.Description, &e.PrimaryMood, &e.TriggerSummary, &e.Timestamp); err != nil {
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
		`SELECT id, description, primary_mood, COALESCE(trigger_summary, ''), timestamp
		 FROM emotion_history ORDER BY timestamp DESC LIMIT 1`,
	).Scan(&e.ID, &e.Description, &e.PrimaryMood, &e.TriggerSummary, &e.Timestamp)
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
