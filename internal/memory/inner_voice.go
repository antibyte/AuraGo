package memory

import (
	"fmt"
	"strings"
)

// InnerVoiceEntry represents a stored inner voice thought.
type InnerVoiceEntry struct {
	ID            int64  `json:"id"`
	InnerThought  string `json:"inner_thought"`
	NudgeCategory string `json:"nudge_category"`
	Timestamp     string `json:"timestamp"`
}

// InitInnerVoiceTables creates the inner voice tables (idempotent).
// Uses ALTER TABLE to add columns to existing emotion_history table.
func (s *SQLiteMemory) InitInnerVoiceTables() error {
	columns := []struct {
		Name    string
		TypeDef string
	}{
		{Name: "inner_thought", TypeDef: "TEXT DEFAULT ''"},
		{Name: "nudge_category", TypeDef: "TEXT DEFAULT ''"},
	}
	for _, column := range columns {
		var hasColumn bool
		if err := s.db.QueryRow("SELECT count(*) > 0 FROM pragma_table_info('emotion_history') WHERE name = ?", column.Name).Scan(&hasColumn); err != nil {
			return fmt.Errorf("inner voice check column %s: %w", column.Name, err)
		}
		if hasColumn {
			continue
		}
		if _, err := s.db.Exec("ALTER TABLE emotion_history ADD COLUMN " + column.Name + " " + column.TypeDef); err != nil {
			return fmt.Errorf("inner voice add column %s: %w", column.Name, err)
		}
	}
	return nil
}

// StoreInnerVoice persists an inner voice thought alongside the latest emotion history entry.
// It updates the most recent emotion_history row with the inner thought and nudge category.
func (s *SQLiteMemory) StoreInnerVoice(thought, category string) error {
	if thought == "" {
		return nil
	}
	_, err := s.db.Exec(
		`UPDATE emotion_history SET inner_thought = ?, nudge_category = ?
		 WHERE id = (SELECT MAX(id) FROM emotion_history)`,
		thought, category,
	)
	return err
}

// GetRecentInnerVoices returns the N most recent inner voice entries (non-empty thoughts).
func (s *SQLiteMemory) GetRecentInnerVoices(limit int) ([]InnerVoiceEntry, error) {
	if limit <= 0 {
		limit = 5
	}
	rows, err := s.db.Query(
		`SELECT id, inner_thought, COALESCE(nudge_category, ''), timestamp
		 FROM emotion_history
		 WHERE inner_thought IS NOT NULL AND inner_thought != ''
		 ORDER BY id DESC LIMIT ?`, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("get recent inner voices: %w", err)
	}
	defer rows.Close()

	var entries []InnerVoiceEntry
	for rows.Next() {
		var e InnerVoiceEntry
		if err := rows.Scan(&e.ID, &e.InnerThought, &e.NudgeCategory, &e.Timestamp); err != nil {
			return nil, fmt.Errorf("scan inner voice entry: %w", err)
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// GetTodayInnerVoiceSummary returns all inner voice entries from today for daily reflection.
func (s *SQLiteMemory) GetTodayInnerVoiceSummary() ([]InnerVoiceEntry, error) {
	rows, err := s.db.Query(
		`SELECT id, inner_thought, COALESCE(nudge_category, ''), timestamp
		 FROM emotion_history
		 WHERE inner_thought IS NOT NULL AND inner_thought != ''
		   AND DATE(timestamp) = DATE('now')
		 ORDER BY id ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("get today inner voices: %w", err)
	}
	defer rows.Close()

	var entries []InnerVoiceEntry
	for rows.Next() {
		var e InnerVoiceEntry
		if err := rows.Scan(&e.ID, &e.InnerThought, &e.NudgeCategory, &e.Timestamp); err != nil {
			return nil, fmt.Errorf("scan today inner voice: %w", err)
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// FormatInnerVoiceHistory formats recent inner voice entries as a brief summary for prompt context.
// Returns a compact string like: "thought1 [category] | thought2 [category]".
func FormatInnerVoiceHistory(entries []InnerVoiceEntry) string {
	if len(entries) == 0 {
		return ""
	}
	var b strings.Builder
	for i, e := range entries {
		if i >= 3 {
			break
		}
		if b.Len() > 0 {
			b.WriteString(" | ")
		}
		thought := e.InnerThought
		// Truncate long thoughts for prompt efficiency
		if len(thought) > 100 {
			thought = thought[:97] + "..."
		}
		b.WriteString(thought)
		if e.NudgeCategory != "" {
			b.WriteString(" [")
			b.WriteString(e.NudgeCategory)
			b.WriteString("]")
		}
	}
	return b.String()
}
