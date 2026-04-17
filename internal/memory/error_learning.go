package memory

import (
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"time"
	"unicode/utf8"
)

// Compiled regexp patterns for error message normalization.
// Variable parts such as paths, timestamps, and IDs are replaced with
// stable placeholders so similar errors group together correctly.
var (
	reErrorPath        = regexp.MustCompile(`(/[a-zA-Z0-9_.\-/]+){2,}`)                               // file/directory paths
	reErrorPathWindows = regexp.MustCompile(`[A-Za-z]:\\[\\A-Za-z0-9_.\-\\]+|\\\\[A-Za-z0-9_.\-\\]+`) // Windows paths
	reErrorTimestamp   = regexp.MustCompile(`\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}`)                 // ISO timestamps
	reErrorNumber      = regexp.MustCompile(`\b\d{4,}\b`)                                             // long numbers / IDs
	reErrorHex         = regexp.MustCompile(`\b[0-9a-fA-F]{8,}\b`)                                    // hex digests / UUIDs
)

// normalizeErrorMsg replaces variable parts of an error message with placeholders
// so that semantically identical errors with different paths/IDs/timestamps match.
func normalizeErrorMsg(msg string) string {
	// Cap input length to prevent excessive regex processing
	const maxErrorMsgLen = 10 * 1024 // 10KB
	if len(msg) > maxErrorMsgLen {
		msg = msg[:maxErrorMsgLen]
	}
	msg = reErrorPathWindows.ReplaceAllString(msg, "<PATH>")
	msg = reErrorPath.ReplaceAllString(msg, "<PATH>")
	msg = reErrorTimestamp.ReplaceAllString(msg, "<TIMESTAMP>")
	msg = reErrorNumber.ReplaceAllString(msg, "<ID>")
	msg = reErrorHex.ReplaceAllString(msg, "<HEX>")
	return msg
}

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
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(tool_name, error_message)
	);
	CREATE INDEX IF NOT EXISTS idx_error_tool ON error_patterns(tool_name);`

	if _, err := s.db.Exec(schema); err != nil {
		return fmt.Errorf("error_patterns schema: %w", err)
	}
	// Add UNIQUE constraint to existing databases that were created before this migration.
	// SQLite cannot ADD CONSTRAINT to existing tables, but we can create a unique index
	// (idempotent via IF NOT EXISTS).
	if _, err := s.db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_error_unique ON error_patterns(tool_name, error_message)`); err != nil {
		return fmt.Errorf("error_patterns unique index: %w", err)
	}
	return nil
}

// RecordError logs a tool error. If a similar pattern exists, it increments the count.
// The error message is normalized before storage so variable parts (paths, IDs, timestamps)
// do not prevent grouping of semantically identical errors.
func (s *SQLiteMemory) RecordError(toolName, errorMsg string) error {
	if toolName == "" || errorMsg == "" {
		return nil
	}
	// Truncate long error messages (rune-safe)
	const maxErrorMsgLen = 500
	if utf8.RuneCountInString(errorMsg) > maxErrorMsgLen {
		errorMsg = string([]rune(errorMsg)[:maxErrorMsgLen])
	}
	errorMsg = normalizeErrorMsg(errorMsg)

	now := time.Now().UTC().Format(time.RFC3339)

	// Atomic upsert: if the pattern exists increment count, otherwise insert.
	// The UNIQUE index on (tool_name, error_message) makes this race-free.
	_, err := s.db.Exec(`
		INSERT INTO error_patterns (tool_name, error_message, last_seen, created_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(tool_name, error_message) DO UPDATE SET
			occurrence_count = occurrence_count + 1,
			last_seen = excluded.last_seen
	`, toolName, errorMsg, now, now)
	return err
}

// RecordResolution attaches a resolution to an error pattern.
func (s *SQLiteMemory) RecordResolution(toolName, errorMsg, resolution string) error {
	errorMsg = normalizeErrorMsg(errorMsg)
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
	errorMsg = normalizeErrorMsg(errorMsg)
	var resolution string
	err := s.db.QueryRow(
		`SELECT resolution FROM error_patterns WHERE tool_name = ? AND error_message = ? AND resolution != ''`,
		toolName, errorMsg,
	).Scan(&resolution)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", fmt.Errorf("lookup error resolution: %w", err)
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
