package manus

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"aurago/internal/dbutil"

	_ "modernc.org/sqlite"
)

const ledgerSchemaVersion = 1

// Ledger is the local allowlist and metadata store for AuraGo-created tasks.
type Ledger struct {
	db *sql.DB
}

// TaskStore is the minimal local persistence boundary used by the Manus
// runtime. The interface keeps remote mutation outcomes independently testable.
type TaskStore interface {
	PreflightWrite(context.Context) error
	Upsert(context.Context, TaskRecord) error
	Get(context.Context, string) (TaskRecord, bool, error)
	List(context.Context, int) ([]TaskRecord, error)
}

// TaskRecord is deliberately limited to non-content task metadata.
type TaskRecord struct {
	TaskID       string    `json:"task_id"`
	Title        string    `json:"title"`
	TaskURL      string    `json:"task_url"`
	Status       string    `json:"status"`
	AgentProfile string    `json:"agent_profile"`
	CreditUsage  int       `json:"credit_usage"`
	LastCursor   string    `json:"last_cursor"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// OpenLedger opens or creates the Manus task metadata database.
func OpenLedger(path string) (*Ledger, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("Manus ledger path is empty")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create Manus ledger directory: %w", err)
	}
	db, err := dbutil.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open Manus ledger: %w", err)
	}
	ledger := &Ledger{db: db}
	if err := ledger.migrate(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return ledger, nil
}

func (l *Ledger) migrate(ctx context.Context) error {
	for _, statement := range []string{
		`CREATE TABLE IF NOT EXISTS schema_meta (
			key TEXT PRIMARY KEY,
			value INTEGER NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS manus_tasks (
			task_id TEXT PRIMARY KEY,
			title TEXT NOT NULL DEFAULT '',
			task_url TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT '',
			agent_profile TEXT NOT NULL DEFAULT '',
			credit_usage INTEGER NOT NULL DEFAULT 0,
			last_cursor TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`INSERT INTO schema_meta(key, value) VALUES ('schema_version', 1)
			ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
	} {
		if _, err := l.db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("migrate Manus ledger to version %d: %w", ledgerSchemaVersion, err)
		}
	}
	return nil
}

// Close closes the underlying SQLite database.
func (l *Ledger) Close() error {
	if l == nil || l.db == nil {
		return nil
	}
	return l.db.Close()
}

// PreflightWrite verifies ledger writability using a real SQLite transaction.
func (l *Ledger) PreflightWrite(ctx context.Context) error {
	tx, err := l.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin Manus ledger write preflight: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE schema_meta SET value = value WHERE key = 'schema_version'`); err != nil {
		return fmt.Errorf("write Manus ledger preflight: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit Manus ledger write preflight: %w", err)
	}
	return nil
}

// Upsert records safe task metadata and makes the task available to the agent.
func (l *Ledger) Upsert(ctx context.Context, record TaskRecord) error {
	record.TaskID = strings.TrimSpace(record.TaskID)
	if record.TaskID == "" {
		return fmt.Errorf("Manus task ID is required")
	}
	now := time.Now().UTC()
	if record.CreatedAt.IsZero() {
		record.CreatedAt = now
	}
	if record.UpdatedAt.IsZero() {
		record.UpdatedAt = now
	}
	_, err := l.db.ExecContext(ctx, `INSERT INTO manus_tasks
		(task_id, title, task_url, status, agent_profile, credit_usage, last_cursor, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(task_id) DO UPDATE SET
			title = excluded.title,
			task_url = excluded.task_url,
			status = excluded.status,
			agent_profile = excluded.agent_profile,
			credit_usage = excluded.credit_usage,
			last_cursor = excluded.last_cursor,
			updated_at = excluded.updated_at`,
		record.TaskID, record.Title, record.TaskURL, record.Status, record.AgentProfile,
		record.CreditUsage, record.LastCursor, record.CreatedAt.Format(time.RFC3339Nano), record.UpdatedAt.Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("upsert Manus task metadata: %w", err)
	}
	return nil
}

// Get returns one locally tracked task.
func (l *Ledger) Get(ctx context.Context, taskID string) (TaskRecord, bool, error) {
	row := l.db.QueryRowContext(ctx, `SELECT task_id, title, task_url, status, agent_profile,
		credit_usage, last_cursor, created_at, updated_at FROM manus_tasks WHERE task_id = ?`, strings.TrimSpace(taskID))
	record, err := scanTaskRecord(row)
	if err == sql.ErrNoRows {
		return TaskRecord{}, false, nil
	}
	if err != nil {
		return TaskRecord{}, false, fmt.Errorf("read Manus task metadata: %w", err)
	}
	return record, true, nil
}

// List returns tracked tasks newest first.
func (l *Ledger) List(ctx context.Context, limit int) ([]TaskRecord, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	rows, err := l.db.QueryContext(ctx, `SELECT task_id, title, task_url, status, agent_profile,
		credit_usage, last_cursor, created_at, updated_at FROM manus_tasks ORDER BY updated_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("list Manus task metadata: %w", err)
	}
	defer rows.Close()
	result := make([]TaskRecord, 0)
	for rows.Next() {
		record, err := scanTaskRecord(rows)
		if err != nil {
			return nil, fmt.Errorf("scan Manus task metadata: %w", err)
		}
		result = append(result, record)
	}
	return result, rows.Err()
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanTaskRecord(row rowScanner) (TaskRecord, error) {
	var record TaskRecord
	var created, updated string
	err := row.Scan(&record.TaskID, &record.Title, &record.TaskURL, &record.Status, &record.AgentProfile,
		&record.CreditUsage, &record.LastCursor, &created, &updated)
	if err != nil {
		return TaskRecord{}, err
	}
	record.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
	record.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updated)
	return record, nil
}
