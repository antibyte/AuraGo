package memory

import (
	"fmt"
	"time"
)

// ErrorPattern represents a learned error pattern with resolution info.
type ErrorPattern struct {
	ID              int64  `json:"id"`
	ToolName        string `json:"tool_name"`
	ErrorMessage    string `json:"error_message"`
	Resolution      string `json:"resolution"`
	OccurrenceCount int    `json:"occurrence_count"`
	LastSeen        string `json:"last_seen"`
	CreatedAt       string `json:"created_at"`
}

// InitErrorLearningTable creates the error_patterns table if it does not exist.
func (s *SQLiteMemory) InitErrorLearningTable() error {
	schema := `
	CREATE TABLE IF NOT EXISTS error_patterns (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		tool_name TEXT NOT NULL,
		error_message TEXT NOT NULL,
		resolution TEXT DEFAULT '',
		occurrence_count INTEGER DEFAULT 1,
		last_seen DATETIME DEFAULT CURRENT_TIMESTAMP,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_error_tool ON error_patterns(tool_name);`

	if _, err := s.db.Exec(schema); err != nil {
		return fmt.Errorf("error_patterns schema: %w", err)
	}
	return nil
}

// RecordError logs a tool error. If a similar pattern exists, it increments the count.
func (s *SQLiteMemory) RecordError(toolName, errorMsg string) error {
	if toolName == "" || errorMsg == "" {
		return nil
	}
	// Truncate long error messages
	if len(errorMsg) > 500 {
		errorMsg = errorMsg[:500]
	}

	now := time.Now().UTC().Format(time.RFC3339)

	// Check for existing pattern (same tool, similar error)
	var existingID int64
	err := s.db.QueryRow(
		`SELECT id FROM error_patterns WHERE tool_name = ? AND error_message = ? LIMIT 1`,
		toolName, errorMsg,
	).Scan(&existingID)

	if err == nil {
		// Pattern exists — increment count
		_, err = s.db.Exec(
			`UPDATE error_patterns SET occurrence_count = occurrence_count + 1, last_seen = ? WHERE id = ?`,
			now, existingID,
		)
		return err
	}

	// New pattern
	_, err = s.db.Exec(
		`INSERT INTO error_patterns (tool_name, error_message, last_seen, created_at) VALUES (?, ?, ?, ?)`,
		toolName, errorMsg, now, now,
	)
	return err
}

// RecordResolution attaches a resolution to an error pattern.
func (s *SQLiteMemory) RecordResolution(toolName, errorMsg, resolution string) error {
	_, err := s.db.Exec(
		`UPDATE error_patterns SET resolution = ? WHERE tool_name = ? AND error_message = ?`,
		resolution, toolName, errorMsg,
	)
	return err
}

// GetFrequentErrors returns the most common error patterns, optionally filtered by tool.
func (s *SQLiteMemory) GetFrequentErrors(toolName string, limit int) ([]ErrorPattern, error) {
	if limit <= 0 || limit > 50 {
		limit = 10
	}

	var query string
	var args []interface{}
	if toolName != "" {
		query = `SELECT id, tool_name, error_message, resolution, occurrence_count, last_seen, created_at
		         FROM error_patterns WHERE tool_name = ? ORDER BY occurrence_count DESC LIMIT ?`
		args = []interface{}{toolName, limit}
	} else {
		query = `SELECT id, tool_name, error_message, resolution, occurrence_count, last_seen, created_at
		         FROM error_patterns ORDER BY occurrence_count DESC LIMIT ?`
		args = []interface{}{limit}
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("get frequent errors: %w", err)
	}
	defer rows.Close()

	var patterns []ErrorPattern
	for rows.Next() {
		var p ErrorPattern
		if err := rows.Scan(&p.ID, &p.ToolName, &p.ErrorMessage, &p.Resolution, &p.OccurrenceCount, &p.LastSeen, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan error pattern: %w", err)
		}
		patterns = append(patterns, p)
	}
	return patterns, rows.Err()
}

// GetRecentErrors returns the most recently seen error patterns.
func (s *SQLiteMemory) GetRecentErrors(limit int) ([]ErrorPattern, error) {
	if limit <= 0 || limit > 50 {
		limit = 10
	}

	rows, err := s.db.Query(
		`SELECT id, tool_name, error_message, resolution, occurrence_count, last_seen, created_at
		 FROM error_patterns ORDER BY last_seen DESC LIMIT ?`, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("get recent errors: %w", err)
	}
	defer rows.Close()

	var patterns []ErrorPattern
	for rows.Next() {
		var p ErrorPattern
		if err := rows.Scan(&p.ID, &p.ToolName, &p.ErrorMessage, &p.Resolution, &p.OccurrenceCount, &p.LastSeen, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan error pattern: %w", err)
		}
		patterns = append(patterns, p)
	}
	return patterns, rows.Err()
}

// LookupErrorResolution returns the resolution for a matching error pattern, if any.
func (s *SQLiteMemory) LookupErrorResolution(toolName, errorMsg string) (string, error) {
	var resolution string
	err := s.db.QueryRow(
		`SELECT resolution FROM error_patterns WHERE tool_name = ? AND error_message = ? AND resolution != ''`,
		toolName, errorMsg,
	).Scan(&resolution)
	if err != nil {
		return "", nil // no resolution found is not an error
	}
	return resolution, nil
}

// GetErrorPatternsCount returns the total number of distinct error patterns.
func (s *SQLiteMemory) GetErrorPatternsCount() (int, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM error_patterns`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("error patterns count: %w", err)
	}
	return count, nil
}
