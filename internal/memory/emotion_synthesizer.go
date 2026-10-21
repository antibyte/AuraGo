package memory

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"time"
	"unicode/utf8"

	"aurago/internal/security"
)

// ── Emotion Synthesizer ──────────────────────────────────────────────────────
// Emotion state produced by the budgeted Personality Engine V2 analysis.

// EmotionState represents the synthesized emotional state of the agent.
type EmotionState struct {
	DynamicsRevision         uint64                `json:"dynamics_revision,omitempty"`
	DynamicsEpoch            uint64                `json:"-"`
	Basis                    *PersonalitySnapshot  `json:"-"`
	ObservationID            string                `json:"-"`
	Appraisal                *PersonalityAppraisal `json:"-"`
	Description              string                `json:"description"`                // 1-2 sentences describing the agent's current emotional state
	PrimaryMood              Mood                  `json:"primary_mood"`               // Primary mood from the existing mood system
	SecondaryMood            string                `json:"secondary_mood"`             // Optional secondary nuance
	Valence                  float64               `json:"valence"`                    // -1.0..1.0
	Arousal                  float64               `json:"arousal"`                    // 0.0..1.0
	Confidence               float64               `json:"confidence"`                 // 0.0..1.0
	Cause                    string                `json:"cause"`                      // Short explanation of the trigger
	Source                   string                `json:"source"`                     // llm_structured | llm_text_fallback
	RecommendedResponseStyle string                `json:"recommended_response_style"` // Short style hint for UI/behavior
	Timestamp                time.Time             `json:"timestamp"`                  // When the emotion was synthesized
}

type EmotionTriggerType string

const (
	EmotionTriggerConversation           EmotionTriggerType = "conversation"
	EmotionTriggerPositiveFeedback       EmotionTriggerType = "positive_feedback"
	EmotionTriggerNegativeFeedback       EmotionTriggerType = "negative_feedback"
	EmotionTriggerUserReturn             EmotionTriggerType = "user_return_after_absence"
	EmotionTriggerPlanCreated            EmotionTriggerType = "plan_created"
	EmotionTriggerPlanAdvanced           EmotionTriggerType = "plan_advanced"
	EmotionTriggerPlanBlocked            EmotionTriggerType = "plan_blocked"
	EmotionTriggerPlanUnblocked          EmotionTriggerType = "plan_unblocked"
	EmotionTriggerPlanCompleted          EmotionTriggerType = "plan_completed"
	EmotionTriggerToolErrorStreak        EmotionTriggerType = "tool_error_streak"
	EmotionTriggerToolSuccessStreak      EmotionTriggerType = "tool_success_streak"
	EmotionTriggerOpsIssueOpened         EmotionTriggerType = "ops_issue_opened"
	EmotionTriggerOpsIssueResolved       EmotionTriggerType = "ops_issue_resolved"
	EmotionTriggerAutonomousRunFailed    EmotionTriggerType = "autonomous_run_failed"
	EmotionTriggerAutonomousRunSucceeded EmotionTriggerType = "autonomous_run_succeeded"
	EmotionTriggerQuietHours             EmotionTriggerType = "quiet_hours"
)

// InnerVoiceNudgeCategories is the canonical set of nudge categories for the inner voice system.
// This must be used consistently across all inner voice prompt paths to ensure valid categories.
const InnerVoiceNudgeCategories = "reflection, patience, focus, creativity, caution, recovery, encouragement, grounding, curiosity, concern, satisfaction, anticipation"

// EmotionInput collects the relevant data for emotion synthesis.
type EmotionInput struct {
	Snapshot           *PersonalitySnapshot
	ObservationID      string
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
	InnerVoiceHistory string   // Brief summary of recent inner voice thoughts for continuity
	// Predictive context fields — enable forward-looking inner voice nudges
	RecentToolUsage     string // Summary of recently used tools (e.g. "shell(2), docker(1)")
	ConversationPhase   string // Detected phase: "opening", "exploration", "execution", "closing", "idle"
	UserTopics          string // Brief summary of recent user topics/interests
	PredictedNextAction string // Hint about likely next action based on trajectory
	// Active persona context — shapes the tone of emotion descriptions & inner voice
	PersonaName   string // Active persona ID (e.g. "punk", "friend"); empty or "neutral" = generic
	PersonaPrompt string // Short persona description (max ~300 chars) for tone guidance
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

// EmotionSynthesizer shares validated emotion state, persistence and cooldown across runs.
type EmotionSynthesizer struct {
	*emotionSynthesisRuntime
	memory      *SQLiteMemory
	minInterval time.Duration
	maxHistory  int
	language    string
	logger      *slog.Logger
}

// NewEmotionSynthesizer creates a new emotion synthesizer.
// Client and model arguments remain compatible with callers; this state store performs no LLM calls.
func NewEmotionSynthesizer(_ PersonalityAnalyzerClient, _ string, minIntervalSecs int, maxHistory int, language string, logger *slog.Logger) *EmotionSynthesizer {
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
		emotionSynthesisRuntime: &emotionSynthesisRuntime{},
		minInterval:             time.Duration(minIntervalSecs) * time.Second,
		maxHistory:              maxHistory,
		language:                language,
		logger:                  logger,
	}
}

// GetLastEmotion returns a copy of the most recent emotion state (thread-safe).
// Callers receive their own copy and cannot mutate the synthesizer's internal state.
func (es *EmotionSynthesizer) GetLastEmotion() *EmotionState {
	if es == nil {
		return nil
	}
	es.mu.RLock()
	if es.lastState == nil {
		es.mu.RUnlock()
		return nil
	}
	stateCopy := *es.lastState
	es.mu.RUnlock()
	if es.memory != nil {
		latest, err := es.memory.GetLatestEmotion()
		if err != nil || latest == nil || latest.DynamicsRevision != stateCopy.DynamicsRevision {
			return nil
		}
		snapshot, err := es.memory.GetPersonalitySnapshotAt(time.Now())
		if err != nil || stateCopy.DynamicsEpoch != snapshot.Epoch {
			return nil
		}
	}
	return &stateCopy
}

// CanSynthesizeNow reports whether a new emotion state may be synthesized at now.
// It returns a copy of the last cached emotion state when one exists.
func (es *EmotionSynthesizer) CanSynthesizeNow(now time.Time) (bool, *EmotionState) {
	if es == nil {
		return false, nil
	}
	if now.IsZero() {
		now = time.Now()
	}
	es.mu.RLock()
	allowed := es.lastCall.IsZero() || now.Sub(es.lastCall) >= es.minInterval
	es.mu.RUnlock()
	return allowed, es.GetLastEmotion()
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
	defer es.mu.Unlock()
	if !es.lastCall.IsZero() && time.Since(es.lastCall) < es.minInterval {
		return nil
	}
	if stm != nil {
		if utf8.RuneCountInString(triggerSummary) > 200 {
			triggerSummary = string([]rune(triggerSummary)[:200])
		}
		if err := stm.persistSynthesizedEmotion(&stateCopy, triggerSummary); err != nil {
			return fmt.Errorf("persist external emotion state: %w", err)
		}
	}

	es.lastCall = time.Now()
	stateCopy.Basis, stateCopy.ObservationID = nil, ""
	es.lastState = &stateCopy

	return nil
}

// sanitizePromptText bounds untrusted text without trying to implement prompt isolation.
func sanitizePromptText(s string, maxLen int) string {
	if maxLen > 0 && utf8.RuneCountInString(s) > maxLen {
		s = string([]rune(s)[:maxLen]) + "…"
	}
	return s
}

func isolatedPromptText(s string, maxLen int) string {
	return security.IsolateExternalData(sanitizePromptText(s, maxLen))
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
		"want to die",
		"wish i were dead",
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
	// Strip markdown JSON code block formatting if present
	if strings.HasPrefix(strings.ToLower(content), "```json") {
		content = content[7:]
	} else if strings.HasPrefix(content, "```") {
		content = content[3:]
	}
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

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
		if mood, ok := normalizeMoodValue(parsed.PrimaryMood); ok {
			state.PrimaryMood = mood
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
	var object map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &object); err != nil || object == nil {
		return nil, fmt.Errorf("emotion_state must be a JSON object")
	}
	var payload emotionSynthesisResult
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil, fmt.Errorf("invalid emotion_state: %w", err)
	}
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
		{Name: "dynamics_revision", TypeDef: "INTEGER NOT NULL DEFAULT 0"},
		{Name: "dynamics_epoch", TypeDef: "INTEGER NOT NULL DEFAULT 0"},
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
	return insertEmotionStateHistoryWith(s.db, state, triggerSummary)
}

func insertEmotionStateHistoryWith(db affectExecer, state EmotionState, triggerSummary string) error {
	if state.Timestamp.IsZero() {
		state.Timestamp = time.Now().UTC()
	}
	_, err := db.Exec(
		`INSERT INTO emotion_history (
			description, primary_mood, secondary_mood, valence, arousal, confidence,
			cause, source, recommended_response_style, trigger_summary, dynamics_revision, dynamics_epoch,timestamp
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?,?)`,
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
		state.DynamicsRevision,
		state.DynamicsEpoch,
		state.Timestamp.UTC().Format("2006-01-02 15:04:05"),
	)
	return err
}

// EmotionHistoryEntry is a single row from the emotion_history table.
type EmotionHistoryEntry struct {
	DynamicsRevision         uint64  `json:"dynamics_revision,omitempty"`
	DynamicsEpoch            uint64  `json:"-"`
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
		        COALESCE(source, ''), COALESCE(recommended_response_style, ''), COALESCE(trigger_summary, ''), timestamp, dynamics_revision, dynamics_epoch
		 FROM emotion_history
		 WHERE timestamp >= datetime('now', ?)
		 ORDER BY timestamp DESC, id DESC`,
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
			&e.DynamicsRevision,
			&e.DynamicsEpoch,
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
		        COALESCE(source, ''), COALESCE(recommended_response_style, ''), COALESCE(trigger_summary, ''), timestamp, dynamics_revision, dynamics_epoch
		 FROM emotion_history ORDER BY timestamp DESC, id DESC LIMIT 1`,
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
		&e.DynamicsRevision,
		&e.DynamicsEpoch,
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
