package memory

import (
	"fmt"
	"time"
	"unicode/utf8"
)

// LearnedRule represents a concrete action rule learned from recurring errors
// or successful recovery patterns.
type LearnedRule struct {
	ID         int64     `json:"id"`
	ToolName   string    `json:"tool_name"`
	Pattern    string    `json:"pattern"`    // normalised error pattern
	Rule       string    `json:"rule"`       // concise human-readable rule
	Confidence float64   `json:"confidence"` // 0.0–1.0
	Hits       int       `json:"hits"`
	Misses     int       `json:"misses"`
	Active     bool      `json:"active"`
	CreatedAt  time.Time `json:"created_at"`
}

// InitLearnedRulesTable creates the learned_rules table and indexes.
func (s *SQLiteMemory) InitLearnedRulesTable() error {
	schema := `
	CREATE TABLE IF NOT EXISTS learned_rules (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		tool_name TEXT NOT NULL,
		pattern TEXT NOT NULL,
		rule TEXT NOT NULL,
		confidence REAL DEFAULT 0.5,
		hits INTEGER DEFAULT 0,
		misses INTEGER DEFAULT 0,
		active BOOLEAN DEFAULT 1,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(tool_name, pattern)
	);
	CREATE INDEX IF NOT EXISTS idx_learned_tool ON learned_rules(tool_name);
	CREATE INDEX IF NOT EXISTS idx_learned_active ON learned_rules(active);`

	if _, err := s.db.Exec(schema); err != nil {
		return fmt.Errorf("learned_rules schema: %w", err)
	}
	return nil
}

// UpsertLearnedRule inserts a new rule or updates an existing one when the
// same (tool_name, pattern) pair already exists. The rule text is updated
// and confidence is bumped slightly on every re-occurrence.
// Uses SQLite's native ON CONFLICT for atomic upsert.
func (s *SQLiteMemory) UpsertLearnedRule(rule *LearnedRule) error {
	if rule == nil || rule.ToolName == "" || rule.Pattern == "" || rule.Rule == "" {
		return nil
	}
	// Truncate long fields (rune-safe)
	const maxFieldLen = 500
	if utf8.RuneCountInString(rule.Pattern) > maxFieldLen {
		rule.Pattern = string([]rune(rule.Pattern)[:maxFieldLen])
	}
	if utf8.RuneCountInString(rule.Rule) > maxFieldLen {
		rule.Rule = string([]rune(rule.Rule)[:maxFieldLen])
	}

	now := time.Now().UTC()

	_, err := s.db.Exec(`
		INSERT INTO learned_rules (tool_name, pattern, rule, confidence, hits, misses, active, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(tool_name, pattern) DO UPDATE SET
			rule = excluded.rule,
			confidence = CASE WHEN learned_rules.confidence + 0.05 > 0.95 THEN 0.95 ELSE learned_rules.confidence + 0.05 END,
			hits = learned_rules.hits + 1,
			created_at = excluded.created_at
	`, rule.ToolName, rule.Pattern, rule.Rule, rule.Confidence, rule.Hits, rule.Misses, rule.Active, now)
	if err != nil {
		return fmt.Errorf("upsert learned rule: %w", err)
	}
	return nil
}

// GetActiveLearnedRules returns the top N active learned rules, ordered by
// confidence desc, hits desc, recency desc.
func (s *SQLiteMemory) GetActiveLearnedRules(limit int) ([]LearnedRule, error) {
	if limit <= 0 || limit > 50 {
		limit = 5
	}

	rows, err := s.db.Query(`
		SELECT id, tool_name, pattern, rule, confidence, hits, misses, active, created_at
		FROM learned_rules
		WHERE active = 1
		ORDER BY confidence DESC, hits DESC, created_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("get active learned rules: %w", err)
	}
	defer rows.Close()

	var rules []LearnedRule
	for rows.Next() {
		var r LearnedRule
		var active int
		if err := rows.Scan(&r.ID, &r.ToolName, &r.Pattern, &r.Rule, &r.Confidence, &r.Hits, &r.Misses, &active, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan learned rule: %w", err)
		}
		r.Active = active != 0
		rules = append(rules, r)
	}
	return rules, rows.Err()
}

// GetLearnedRulesForTools returns active learned rules filtered by a set of
// tool names. This is used for adaptive injection (only rules relevant to the
// current tool set).
func (s *SQLiteMemory) GetLearnedRulesForTools(toolNames []string, limit int) ([]LearnedRule, error) {
	if len(toolNames) == 0 {
		return s.GetActiveLearnedRules(limit)
	}
	if limit <= 0 || limit > 50 {
		limit = 5
	}

	query := `
		SELECT id, tool_name, pattern, rule, confidence, hits, misses, active, created_at
		FROM learned_rules
		WHERE active = 1 AND tool_name IN (`
	args := make([]interface{}, 0, len(toolNames)+1)
	for i, name := range toolNames {
		if i > 0 {
			query += ", "
		}
		query += "?"
		args = append(args, name)
	}
	query += `)
		ORDER BY confidence DESC, hits DESC, created_at DESC
		LIMIT ?`
	args = append(args, limit)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("get learned rules for tools: %w", err)
	}
	defer rows.Close()

	var rules []LearnedRule
	for rows.Next() {
		var r LearnedRule
		var active int
		if err := rows.Scan(&r.ID, &r.ToolName, &r.Pattern, &r.Rule, &r.Confidence, &r.Hits, &r.Misses, &active, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan learned rule: %w", err)
		}
		r.Active = active != 0
		rules = append(rules, r)
	}
	return rules, rows.Err()
}

// RecordLearnedRuleHit increments the hit counter and slightly boosts confidence.
func (s *SQLiteMemory) RecordLearnedRuleHit(ruleID int64) error {
	_, err := s.db.Exec(`
		UPDATE learned_rules
		SET hits = hits + 1,
		    confidence = CASE WHEN confidence + 0.02 > 0.99 THEN 0.99 ELSE confidence + 0.02 END
		WHERE id = ?
	`, ruleID)
	return err
}

// RecordLearnedRuleMiss increments the miss counter and slightly reduces confidence.
func (s *SQLiteMemory) RecordLearnedRuleMiss(ruleID int64) error {
	_, err := s.db.Exec(`
		UPDATE learned_rules
		SET misses = misses + 1,
		    confidence = CASE WHEN confidence - 0.05 < 0.1 THEN 0.1 ELSE confidence - 0.05 END
		WHERE id = ?
	`, ruleID)
	return err
}

// GetLearnedRulesCount returns the total number of learned rules.
func (s *SQLiteMemory) GetLearnedRulesCount() (int, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM learned_rules`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("learned rules count: %w", err)
	}
	return count, nil
}

// CleanOldLearnedRules deletes learned rules whose confidence has fallen below
// the threshold or that are older than the given number of days and have
// never been hit. Returns the number of rows deleted.
func (s *SQLiteMemory) CleanOldLearnedRules(minConfidence float64, maxAgeDays int) (int, error) {
	if minConfidence <= 0 {
		minConfidence = 0.1
	}
	if maxAgeDays <= 0 {
		maxAgeDays = 90
	}
	cutoff := time.Now().UTC().AddDate(0, 0, -maxAgeDays)

	res, err := s.db.Exec(`
		DELETE FROM learned_rules
		WHERE confidence < ?
		   OR (hits = 0 AND created_at < ?)
	`, minConfidence, cutoff)
	if err != nil {
		return 0, fmt.Errorf("clean old learned rules: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("clean old learned rules rows: %w", err)
	}
	return int(n), nil
}
