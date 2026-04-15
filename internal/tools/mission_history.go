package tools

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// MissionRun represents a single mission execution record in the history.
type MissionRun struct {
	ID          string     `json:"id"`           // Unique run ID (auto-generated)
	MissionID   string     `json:"mission_id"`   // Reference to the mission
	MissionName string     `json:"mission_name"` // Snapshot of mission name at run time
	TriggerType string     `json:"trigger_type"` // manual, cron, webhook, email, mqtt, daemon_wake, etc.
	TriggerData string     `json:"trigger_data"` // JSON blob with trigger context
	Status      string     `json:"status"`       // running, success, error
	Output      string     `json:"output"`       // Truncated output (max 2000 chars)
	ErrorMsg    string     `json:"error_msg"`    // Error message if status=error
	StartedAt   time.Time  `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	DurationMS  int64      `json:"duration_ms"` // Duration in milliseconds
}

// MissionHistoryFilter holds structured filter parameters for querying mission history.
type MissionHistoryFilter struct {
	MissionID   string `json:"mission_id,omitempty"`   // Filter by mission ID
	Result      string `json:"result,omitempty"`       // Filter by result: success, error
	TriggerType string `json:"trigger_type,omitempty"` // Filter by trigger type
	From        string `json:"from,omitempty"`         // ISO 8601 start date
	To          string `json:"to,omitempty"`           // ISO 8601 end date
	Limit       int    `json:"limit,omitempty"`        // Page size (default 10)
	Offset      int    `json:"offset,omitempty"`       // Page offset
}

// MissionHistoryPage represents a paginated result of mission history entries.
type MissionHistoryPage struct {
	Entries []*MissionRun `json:"entries"`
	Total   int           `json:"total"`
	Limit   int           `json:"limit"`
	Offset  int           `json:"offset"`
}

// InitMissionHistoryDB initializes the mission history SQLite database with
// WAL journal mode, busy timeout and schema creation.
func InitMissionHistoryDB(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open mission history database: %w", err)
	}

	// SQLite hardening: WAL mode for concurrent read/write, busy timeout for lock contention
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to set WAL mode: %w", err)
	}
	if _, err := db.Exec("PRAGMA busy_timeout=5000"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to set busy_timeout: %w", err)
	}

	schema := `
	CREATE TABLE IF NOT EXISTS mission_history (
		id            TEXT PRIMARY KEY,
		mission_id    TEXT NOT NULL,
		mission_name  TEXT NOT NULL DEFAULT '',
		trigger_type  TEXT NOT NULL DEFAULT '',
		trigger_data  TEXT NOT NULL DEFAULT '',
		status        TEXT NOT NULL DEFAULT 'running',
		output        TEXT NOT NULL DEFAULT '',
		error_msg     TEXT NOT NULL DEFAULT '',
		started_at    DATETIME NOT NULL,
		completed_at  DATETIME,
		duration_ms   INTEGER NOT NULL DEFAULT 0
	);
	CREATE INDEX IF NOT EXISTS idx_mh_mission_id ON mission_history(mission_id);
	CREATE INDEX IF NOT EXISTS idx_mh_status ON mission_history(status);
	CREATE INDEX IF NOT EXISTS idx_mh_trigger_type ON mission_history(trigger_type);
	CREATE INDEX IF NOT EXISTS idx_mh_started_at ON mission_history(started_at);`

	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create mission_history schema: %w", err)
	}

	return db, nil
}

// ReconcileStaleRunningMarks marks all mission_history entries that are still
// "running" but started more than maxAge ago as "error" with an abort message.
// This should be called once at server startup to clean up zombie entries left
// behind by crashes or unclean shutdowns.
func ReconcileStaleRunningMarks(db *sql.DB, maxAge time.Duration, logger *slog.Logger) (int64, error) {
	if db == nil {
		return 0, nil
	}
	cutoff := time.Now().Add(-maxAge).Format(time.RFC3339)
	result, err := db.Exec(`
		UPDATE mission_history
		SET status = 'error', error_msg = 'aborted: server restart', completed_at = ?
		WHERE status = 'running' AND started_at < ?`,
		time.Now().Format(time.RFC3339), cutoff)
	if err != nil {
		return 0, fmt.Errorf("failed to reconcile stale running entries: %w", err)
	}
	updated, _ := result.RowsAffected()
	if updated > 0 {
		logger.Info("[MissionHistory] Reconciled stale running entries", "updated", updated)
	}
	return updated, nil
}

// RecordMissionStart inserts a new mission run record with status "running".
func RecordMissionStart(db *sql.DB, missionID, missionName, triggerType, triggerData string) (string, error) {
	if db == nil {
		return "", fmt.Errorf("mission history database not available")
	}

	id := fmt.Sprintf("run_%d", time.Now().UnixNano())
	now := time.Now()

	_, err := db.Exec(`
		INSERT INTO mission_history (id, mission_id, mission_name, trigger_type, trigger_data, status, started_at)
		VALUES (?, ?, ?, ?, ?, 'running', ?)`,
		id, missionID, missionName, triggerType, triggerData, now.Format(time.RFC3339))

	if err != nil {
		return "", fmt.Errorf("failed to record mission start: %w", err)
	}

	return id, nil
}

// RecordMissionCompletion updates a mission run record with the final result.
func RecordMissionCompletion(db *sql.DB, runID, status, output string) error {
	if db == nil {
		return fmt.Errorf("mission history database not available")
	}

	now := time.Now()
	nowStr := now.Format(time.RFC3339)

	// Truncate output to 2000 characters
	if len(output) > 2000 {
		output = output[:1997] + "..."
	}

	// Calculate duration from started_at
	var durationMS int64
	var startedAtStr string
	row := db.QueryRow(`SELECT started_at FROM mission_history WHERE id = ?`, runID)
	if err := row.Scan(&startedAtStr); err == nil {
		if startedAt, err := time.Parse(time.RFC3339, startedAtStr); err == nil {
			durationMS = now.Sub(startedAt).Milliseconds()
		}
	}

	result, err := db.Exec(`
		UPDATE mission_history SET status = ?, output = ?, completed_at = ?, duration_ms = ? WHERE id = ?`,
		status, output, nowStr, durationMS, runID)

	if err != nil {
		return fmt.Errorf("failed to record mission completion: %w", err)
	}

	if n, _ := result.RowsAffected(); n == 0 {
		slog.Warn("[MissionV2] RecordMissionCompletion updated 0 rows, run ID may not exist", "run_id", runID)
	}

	return nil
}

// RecordMissionError updates a mission run record with an error result.
func RecordMissionError(db *sql.DB, runID, errorMsg string) error {
	if db == nil {
		return fmt.Errorf("mission history database not available")
	}

	now := time.Now()
	nowStr := now.Format(time.RFC3339)

	// Truncate error message
	if len(errorMsg) > 500 {
		errorMsg = errorMsg[:497] + "..."
	}

	// Calculate duration from started_at
	var durationMS int64
	var startedAtStr string
	row := db.QueryRow(`SELECT started_at FROM mission_history WHERE id = ?`, runID)
	if err := row.Scan(&startedAtStr); err == nil {
		if startedAt, err := time.Parse(time.RFC3339, startedAtStr); err == nil {
			durationMS = now.Sub(startedAt).Milliseconds()
		}
	}

	result, err := db.Exec(`
		UPDATE mission_history SET status = 'error', error_msg = ?, completed_at = ?, duration_ms = ? WHERE id = ?`,
		errorMsg, nowStr, durationMS, runID)

	if err != nil {
		return fmt.Errorf("failed to record mission error: %w", err)
	}

	if n, _ := result.RowsAffected(); n == 0 {
		slog.Warn("[MissionV2] RecordMissionError updated 0 rows, run ID may not exist", "run_id", runID)
	}

	return nil
}

// QueryMissionHistory queries mission history with structured filters and pagination.
func QueryMissionHistory(db *sql.DB, filter MissionHistoryFilter) (*MissionHistoryPage, error) {
	if db == nil {
		return nil, fmt.Errorf("mission history database not available")
	}

	// Build WHERE clause
	var conditions []string
	var args []interface{}

	if filter.MissionID != "" {
		conditions = append(conditions, "mission_id = ?")
		args = append(args, filter.MissionID)
	}
	if filter.Result != "" {
		conditions = append(conditions, "status = ?")
		args = append(args, filter.Result)
	}
	if filter.TriggerType != "" {
		conditions = append(conditions, "trigger_type = ?")
		args = append(args, filter.TriggerType)
	}
	if filter.From != "" {
		conditions = append(conditions, "started_at >= ?")
		args = append(args, filter.From)
	}
	if filter.To != "" {
		conditions = append(conditions, "started_at <= ?")
		args = append(args, filter.To)
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	// Count total
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM mission_history %s", whereClause)
	var total int
	if err := db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("failed to count mission history: %w", err)
	}

	// Apply pagination defaults
	limit := filter.Limit
	if limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	// Query entries
	query := fmt.Sprintf(
		`SELECT id, mission_id, mission_name, trigger_type, trigger_data, status, output, error_msg, started_at, completed_at, duration_ms
		 FROM mission_history %s ORDER BY started_at DESC LIMIT ? OFFSET ?`,
		whereClause,
	)
	queryArgs := append(args, limit, offset)

	rows, err := db.Query(query, queryArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to query mission history: %w", err)
	}
	defer rows.Close()

	var entries []*MissionRun
	for rows.Next() {
		run := &MissionRun{}
		var startedAtStr, completedAtStr sql.NullString
		var triggerData, output, errorMsg sql.NullString

		if err := rows.Scan(
			&run.ID, &run.MissionID, &run.MissionName, &run.TriggerType,
			&triggerData, &run.Status, &output, &errorMsg,
			&startedAtStr, &completedAtStr, &run.DurationMS,
		); err != nil {
			return nil, fmt.Errorf("failed to scan mission history row: %w", err)
		}

		if startedAtStr.Valid {
			if t, err := time.Parse(time.RFC3339, startedAtStr.String); err == nil {
				run.StartedAt = t
			}
		}
		if completedAtStr.Valid && completedAtStr.String != "" {
			if t, err := time.Parse(time.RFC3339, completedAtStr.String); err == nil {
				run.CompletedAt = &t
			}
		}
		run.TriggerData = triggerData.String
		run.Output = output.String
		run.ErrorMsg = errorMsg.String

		entries = append(entries, run)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating mission history rows: %w", err)
	}

	return &MissionHistoryPage{
		Entries: entries,
		Total:   total,
		Limit:   limit,
		Offset:  offset,
	}, nil
}

// GetMissionRun retrieves a single mission run by ID.
func GetMissionRun(db *sql.DB, runID string) (*MissionRun, error) {
	if db == nil {
		return nil, fmt.Errorf("mission history database not available")
	}

	run := &MissionRun{}
	var startedAtStr, completedAtStr sql.NullString
	var triggerData, output, errorMsg sql.NullString

	err := db.QueryRow(`
		SELECT id, mission_id, mission_name, trigger_type, trigger_data, status, output, error_msg, started_at, completed_at, duration_ms
		FROM mission_history WHERE id = ?`, runID).Scan(
		&run.ID, &run.MissionID, &run.MissionName, &run.TriggerType,
		&triggerData, &run.Status, &output, &errorMsg,
		&startedAtStr, &completedAtStr, &run.DurationMS,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get mission run: %w", err)
	}

	if startedAtStr.Valid {
		if t, err := time.Parse(time.RFC3339, startedAtStr.String); err == nil {
			run.StartedAt = t
		}
	}
	if completedAtStr.Valid && completedAtStr.String != "" {
		if t, err := time.Parse(time.RFC3339, completedAtStr.String); err == nil {
			run.CompletedAt = &t
		}
	}
	run.TriggerData = triggerData.String
	run.Output = output.String
	run.ErrorMsg = errorMsg.String

	return run, nil
}

// CleanOldMissionHistory removes history entries older than the specified retention period.
// Set maxAge to 0 to keep all entries.
func CleanOldMissionHistory(db *sql.DB, maxAge time.Duration, logger *slog.Logger) (int64, error) {
	if db == nil || maxAge <= 0 {
		return 0, nil
	}

	cutoff := time.Now().Add(-maxAge).Format(time.RFC3339)
	result, err := db.Exec(`DELETE FROM mission_history WHERE started_at < ?`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("failed to clean old mission history: %w", err)
	}

	deleted, _ := result.RowsAffected()
	if deleted > 0 {
		logger.Info("[MissionHistory] Cleaned old entries", "deleted", deleted, "cutoff", cutoff)
	}

	return deleted, nil
}

// FormatMissionHistoryJSON marshals mission history entries to JSON string.
func FormatMissionHistoryJSON(page *MissionHistoryPage) string {
	b, err := json.Marshal(page)
	if err != nil {
		return `{"entries":[],"total":0,"limit":10,"offset":0}`
	}
	return string(b)
}
