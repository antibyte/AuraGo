package dockerstacks

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

const journalSchema = `
CREATE TABLE IF NOT EXISTS docker_operations (
    id          TEXT PRIMARY KEY,
    host_id     TEXT NOT NULL,
    stack       TEXT NOT NULL DEFAULT '',
    kind        TEXT NOT NULL,
    options     TEXT NOT NULL DEFAULT '{}',
    state       TEXT NOT NULL,
    runner_id   TEXT NOT NULL DEFAULT '',
    exit_code   INTEGER,
    code        TEXT NOT NULL DEFAULT '',
    error       TEXT NOT NULL DEFAULT '',
    error_line  INTEGER NOT NULL DEFAULT 0,
    warning     TEXT NOT NULL DEFAULT '',
    output      TEXT NOT NULL DEFAULT '',
    ports       TEXT NOT NULL DEFAULT '[]',
    created_at  TEXT NOT NULL,
    started_at  TEXT NOT NULL DEFAULT '',
    finished_at TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_docker_operations_stack ON docker_operations(host_id, stack, created_at);
CREATE INDEX IF NOT EXISTS idx_docker_operations_state ON docker_operations(state);
`

// journalKeep is how many operations of one (host, stack) the journal keeps.
const journalKeep = 50

// journalTime is a fixed-width UTC layout, so that the text column sorts in time order.
const journalTime = "2006-01-02T15:04:05.000000000Z"

const (
	journalColumns     = "id, host_id, stack, kind, options, state, runner_id, exit_code, code, error, error_line, warning, ports, created_at, started_at, finished_at"
	journalListLimit   = 20
	journalListMaxRows = 100
	activeStates       = "('" + StateQueued + "','" + StateRunning + "','" + StateHealthWait + "')"
)

// journal is the table docker_operations in docker_manager.db.
type journal struct{ db *sql.DB }

func newJournal(db *sql.DB) (*journal, error) {
	if _, err := db.Exec(journalSchema); err != nil {
		return nil, fmt.Errorf("create docker operations schema: %w", err)
	}
	return &journal{db: db}, nil
}

func formatJournalTime(t time.Time) string { return t.UTC().Format(journalTime) }

func parseJournalTime(s string) time.Time {
	t, _ := time.Parse(journalTime, s)
	return t
}

func optionalTime(t *time.Time) string {
	if t == nil || t.IsZero() {
		return ""
	}
	return formatJournalTime(*t)
}

// insert stores a new operation.
func (j *journal) insert(ctx context.Context, op Operation) error {
	options, _ := json.Marshal(op.Options)
	ports, _ := json.Marshal(nonNilPorts(op.Ports))
	var exit any
	if op.ExitCode != nil {
		exit = *op.ExitCode
	}
	_, err := j.db.ExecContext(ctx, `INSERT INTO docker_operations
		(id, host_id, stack, kind, options, state, runner_id, exit_code, code, error, error_line, warning, output, ports, created_at, started_at, finished_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		op.ID, op.HostID, op.Stack, op.Kind, string(options), op.State, op.RunnerID, exit, op.Code, op.Error, op.ErrorLine,
		op.Warning, op.Output, string(ports), formatJournalTime(op.CreatedAt), optionalTime(op.StartedAt), optionalTime(op.FinishedAt))
	if err != nil {
		return fmt.Errorf("store docker operation: %w", err)
	}
	return nil
}

// save writes every mutable column of an operation.
func (j *journal) save(ctx context.Context, op Operation) error {
	ports, _ := json.Marshal(nonNilPorts(op.Ports))
	var exit any
	if op.ExitCode != nil {
		exit = *op.ExitCode
	}
	_, err := j.db.ExecContext(ctx, `UPDATE docker_operations SET state = ?, runner_id = ?, exit_code = ?, code = ?, error = ?,
		error_line = ?, warning = ?, output = ?, ports = ?, started_at = ?, finished_at = ? WHERE id = ?`,
		op.State, op.RunnerID, exit, op.Code, op.Error, op.ErrorLine, op.Warning, op.Output, string(ports),
		optionalTime(op.StartedAt), optionalTime(op.FinishedAt), op.ID)
	if err != nil {
		return fmt.Errorf("update docker operation: %w", err)
	}
	return nil
}

func nonNilPorts(p []PortLink) []PortLink {
	if p == nil {
		return []PortLink{}
	}
	return p
}

type journalScanner interface{ Scan(dest ...any) error }

// scanOperation reads the columns of journalColumns, plus the output when withOutput.
func scanOperation(row journalScanner, withOutput bool) (Operation, error) {
	var op Operation
	var options, ports, created, started, finished string
	var exit sql.NullInt64
	dest := []any{&op.ID, &op.HostID, &op.Stack, &op.Kind, &options, &op.State, &op.RunnerID, &exit, &op.Code, &op.Error,
		&op.ErrorLine, &op.Warning, &ports, &created, &started, &finished}
	if withOutput {
		dest = append(dest, &op.Output)
	}
	if err := row.Scan(dest...); err != nil {
		return Operation{}, err
	}
	_ = json.Unmarshal([]byte(options), &op.Options)
	_ = json.Unmarshal([]byte(ports), &op.Ports)
	if exit.Valid {
		code := int(exit.Int64)
		op.ExitCode = &code
	}
	op.CreatedAt = parseJournalTime(created)
	if started != "" {
		t := parseJournalTime(started)
		op.StartedAt = &t
	}
	if finished != "" {
		t := parseJournalTime(finished)
		op.FinishedAt = &t
	}
	return op, nil
}

// get returns one operation with its output, or a CodeOperationNotFound error.
func (j *journal) get(ctx context.Context, id string) (Operation, error) {
	op, err := scanOperation(j.db.QueryRowContext(ctx, "SELECT "+journalColumns+", output FROM docker_operations WHERE id = ?", id), true)
	if errors.Is(err, sql.ErrNoRows) {
		return Operation{}, newError(CodeOperationNotFound, "no operation %q", id)
	}
	return op, err
}

// list returns operations newest first, without their output (use get for it).
func (j *journal) list(ctx context.Context, f OperationFilter) ([]Operation, error) {
	query := "SELECT " + journalColumns + " FROM docker_operations WHERE 1=1"
	var args []any
	if f.HostID != "" {
		query += " AND host_id = ?"
		args = append(args, f.HostID)
	}
	if f.Stack != "" {
		query += " AND stack = ?"
		args = append(args, f.Stack)
	}
	if f.Active {
		query += " AND state IN " + activeStates
	}
	limit := f.Limit
	if limit <= 0 {
		limit = journalListLimit
	}
	if limit > journalListMaxRows {
		limit = journalListMaxRows
	}
	query += " ORDER BY created_at DESC, id DESC LIMIT ?"
	args = append(args, limit)
	rows, err := j.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list docker operations: %w", err)
	}
	defer rows.Close()
	ops := []Operation{}
	for rows.Next() {
		op, err := scanOperation(rows, false)
		if err != nil {
			return nil, err
		}
		ops = append(ops, op)
	}
	return ops, rows.Err()
}

// active returns every queued, running or health_wait operation with its
// output, oldest first. Open uses it to reattach after a restart.
func (j *journal) active(ctx context.Context) ([]Operation, error) {
	rows, err := j.db.QueryContext(ctx, "SELECT "+journalColumns+", output FROM docker_operations WHERE state IN "+activeStates+" ORDER BY created_at, id")
	if err != nil {
		return nil, fmt.Errorf("list active docker operations: %w", err)
	}
	defer rows.Close()
	var ops []Operation
	for rows.Next() {
		op, err := scanOperation(rows, true)
		if err != nil {
			return nil, err
		}
		ops = append(ops, op)
	}
	return ops, rows.Err()
}

// prune deletes the finished operations of one (host, stack) beyond the newest keep.
func (j *journal) prune(ctx context.Context, hostID, stack string, keep int) error {
	_, err := j.db.ExecContext(ctx, `DELETE FROM docker_operations WHERE host_id = ? AND stack = ? AND state NOT IN `+activeStates+`
		AND id NOT IN (SELECT id FROM docker_operations WHERE host_id = ? AND stack = ? ORDER BY created_at DESC, id DESC LIMIT ?)`,
		hostID, stack, hostID, stack, keep)
	if err != nil {
		return fmt.Errorf("prune docker operations: %w", err)
	}
	return nil
}
