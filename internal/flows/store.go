package flows

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"aurago/internal/dbutil"

	_ "modernc.org/sqlite" // registers the "sqlite" driver that dbutil.Open uses
)

var (
	ErrNotFound         = errors.New("flow not found")
	ErrRunNotFound      = errors.New("flow run not found")
	ErrRevisionConflict = errors.New("the flow was changed elsewhere")
	// ErrFlowExists is returned (wrapped, with the flow id) by CreateFlow when a flow with that id is already stored.
	ErrFlowExists = errors.New("a flow with this id already exists")
)

const (
	// timeLayout has a fixed width so stored times sort correctly as text.
	timeLayout        = "2006-01-02T15:04:05.000000Z"
	maxStoredVersions = 50
)

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(timeLayout)
}

func parseTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, err := time.Parse(timeLayout, s)
	if err != nil {
		return time.Time{}
	}
	return t
}

var schemaStatements = []string{
	`CREATE TABLE IF NOT EXISTS flows_meta (key TEXT PRIMARY KEY, value TEXT NOT NULL)`,
	`CREATE TABLE IF NOT EXISTS flows (
		id TEXT PRIMARY KEY,
		kind TEXT NOT NULL DEFAULT 'flow',
		name TEXT NOT NULL,
		description TEXT NOT NULL DEFAULT '',
		mission_id TEXT NOT NULL DEFAULT '',
		draft_json TEXT NOT NULL,
		draft_revision INTEGER NOT NULL DEFAULT 1,
		live_json TEXT NOT NULL DEFAULT '',
		live_revision INTEGER NOT NULL DEFAULT 0,
		published_draft_revision INTEGER NOT NULL DEFAULT 0,
		published_at TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS flow_versions (
		flow_id TEXT NOT NULL REFERENCES flows(id) ON DELETE CASCADE,
		revision INTEGER NOT NULL,
		json TEXT NOT NULL,
		published_at TEXT NOT NULL,
		PRIMARY KEY (flow_id, revision)
	)`,
	`CREATE TABLE IF NOT EXISTS flow_runs (
		id TEXT PRIMARY KEY,
		flow_id TEXT NOT NULL REFERENCES flows(id) ON DELETE CASCADE,
		revision INTEGER NOT NULL DEFAULT 0,
		mode TEXT NOT NULL,
		trigger_node TEXT NOT NULL DEFAULT '',
		trigger_type TEXT NOT NULL DEFAULT '',
		trigger_data_json TEXT NOT NULL DEFAULT '{}',
		status TEXT NOT NULL,
		error_code TEXT NOT NULL DEFAULT '',
		error_message TEXT NOT NULL DEFAULT '',
		started_at TEXT NOT NULL,
		finished_at TEXT NOT NULL DEFAULT '',
		duration_ms INTEGER NOT NULL DEFAULT 0,
		parent_run_id TEXT NOT NULL DEFAULT '',
		parent_node_id TEXT NOT NULL DEFAULT '',
		doc_json TEXT NOT NULL DEFAULT '',
		checkpoint_json TEXT NOT NULL DEFAULT ''
	)`,
	`CREATE INDEX IF NOT EXISTS idx_flow_runs_flow_started ON flow_runs(flow_id, started_at DESC)`,
	`CREATE INDEX IF NOT EXISTS idx_flow_runs_status ON flow_runs(status)`,
	`CREATE TABLE IF NOT EXISTS flow_run_steps (
		run_id TEXT NOT NULL REFERENCES flow_runs(id) ON DELETE CASCADE,
		seq INTEGER NOT NULL,
		node_id TEXT NOT NULL,
		node_key TEXT NOT NULL DEFAULT '',
		attempt INTEGER NOT NULL DEFAULT 1,
		status TEXT NOT NULL,
		started_at TEXT NOT NULL DEFAULT '',
		finished_at TEXT NOT NULL DEFAULT '',
		duration_ms INTEGER NOT NULL DEFAULT 0,
		params_json TEXT NOT NULL DEFAULT '{}',
		params_truncated INTEGER NOT NULL DEFAULT 0,
		output_json TEXT NOT NULL DEFAULT '{}',
		output_truncated INTEGER NOT NULL DEFAULT 0,
		item_count INTEGER NOT NULL DEFAULT 0,
		ports_json TEXT NOT NULL DEFAULT 'null',
		error_code TEXT NOT NULL DEFAULT '',
		error_message TEXT NOT NULL DEFAULT '',
		PRIMARY KEY (run_id, seq)
	)`,
	`CREATE TABLE IF NOT EXISTS flow_timers (
		flow_id TEXT NOT NULL REFERENCES flows(id) ON DELETE CASCADE,
		node_id TEXT NOT NULL,
		fire_at TEXT NOT NULL,
		repeat TEXT NOT NULL DEFAULT '',
		PRIMARY KEY (flow_id, node_id)
	)`,
	`CREATE TABLE IF NOT EXISTS flow_test_data (
		flow_id TEXT NOT NULL REFERENCES flows(id) ON DELETE CASCADE,
		node_id TEXT NOT NULL,
		kind TEXT NOT NULL,
		json TEXT NOT NULL,
		updated_at TEXT NOT NULL,
		PRIMARY KEY (flow_id, node_id, kind)
	)`,
	`INSERT OR IGNORE INTO flows_meta (key, value) VALUES ('schema_version', '1')`,
}

// Store persists flows, versions, runs, timers and test data in SQLite.
type Store struct {
	db     *sql.DB
	logger *slog.Logger
}

// OpenStore opens (and migrates) the flows database at path.
func OpenStore(path string, logger *slog.Logger) (*Store, error) {
	if logger == nil {
		logger = slog.Default()
	}
	db, err := dbutil.Open(path, dbutil.WithCorruptionRecovery(logger))
	if err != nil {
		return nil, fmt.Errorf("open flows database: %w", err)
	}
	s := &Store{db: db, logger: logger}
	for _, stmt := range schemaStatements {
		if _, err := db.Exec(stmt); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("migrate flows database: %w", err)
		}
	}
	return s, nil
}

// Close closes the database.
func (s *Store) Close() error { return s.db.Close() }

// FlowRecord is a stored flow with its draft and published revision.
type FlowRecord struct {
	ID                     string    `json:"id"`
	Kind                   Kind      `json:"kind"`
	Name                   string    `json:"name"`
	Description            string    `json:"description"`
	MissionID              string    `json:"mission_id"`
	Draft                  *Flow     `json:"draft"`
	DraftRevision          int       `json:"draft_revision"`
	Live                   *Flow     `json:"live,omitempty"`
	LiveRevision           int       `json:"live_revision"`
	PublishedDraftRevision int       `json:"published_draft_revision"`
	PublishedAt            time.Time `json:"published_at"`
	CreatedAt              time.Time `json:"created_at"`
	UpdatedAt              time.Time `json:"updated_at"`
}

// Published reports whether the flow has a live revision.
func (r *FlowRecord) Published() bool { return r.Live != nil }

// HasUnpublishedChanges reports whether the draft differs from the published revision.
func (r *FlowRecord) HasUnpublishedChanges() bool {
	return r.Live == nil || r.DraftRevision != r.PublishedDraftRevision
}

type scanner interface {
	Scan(dest ...any) error
}

const flowColumns = `id, kind, name, description, mission_id, draft_json, draft_revision, live_json,
	live_revision, published_draft_revision, published_at, created_at, updated_at`

func scanFlow(row scanner) (*FlowRecord, error) {
	var r FlowRecord
	var kind, draftJSON, liveJSON, publishedAt, createdAt, updatedAt string
	if err := row.Scan(&r.ID, &kind, &r.Name, &r.Description, &r.MissionID, &draftJSON, &r.DraftRevision, &liveJSON,
		&r.LiveRevision, &r.PublishedDraftRevision, &publishedAt, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	r.Kind = Kind(kind)
	draft, err := ParseFlow([]byte(draftJSON))
	if err != nil {
		return nil, fmt.Errorf("flow %s draft: %w", quoteForError(r.ID), err)
	}
	r.Draft = draft
	if liveJSON != "" {
		live, err := ParseFlow([]byte(liveJSON))
		if err != nil {
			return nil, fmt.Errorf("flow %s live revision: %w", quoteForError(r.ID), err)
		}
		r.Live = live
	}
	r.PublishedAt = parseTime(publishedAt)
	r.CreatedAt = parseTime(createdAt)
	r.UpdatedAt = parseTime(updatedAt)
	return &r, nil
}

// CreateFlow stores a new flow as draft revision 1. It returns ErrFlowExists
// (wrapped) when a flow with the same id is already stored and leaves that flow
// untouched. It does not modify f.
func (s *Store) CreateFlow(ctx context.Context, f *Flow, missionID string, now time.Time) (*FlowRecord, error) {
	if f == nil || f.ID == "" {
		return nil, errors.New("flow id is required")
	}
	data, err := f.Marshal()
	if err != nil {
		return nil, err
	}
	if len(data) > MaxDocumentBytes {
		return nil, ErrDocumentTooLarge
	}
	ts := formatTime(now)
	res, err := s.db.ExecContext(ctx, `INSERT INTO flows (id, kind, name, description, mission_id, draft_json, draft_revision, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, 1, ?, ?) ON CONFLICT(id) DO NOTHING`, f.ID, string(f.Kind), f.Name, f.Description, missionID, string(data), ts, ts)
	if err != nil {
		return nil, fmt.Errorf("create flow: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, fmt.Errorf("%w: %s", ErrFlowExists, quoteForError(f.ID))
	}
	return s.GetFlow(ctx, f.ID)
}

// GetFlow loads one flow.
func (s *Store) GetFlow(ctx context.Context, id string) (*FlowRecord, error) {
	r, err := scanFlow(s.db.QueryRowContext(ctx, `SELECT `+flowColumns+` FROM flows WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return r, err
}

// ListFlows returns all flows of kind (all kinds when empty), newest first.
// Rows that cannot be decoded are logged and skipped.
func (s *Store) ListFlows(ctx context.Context, kind Kind) ([]*FlowRecord, error) {
	query := `SELECT ` + flowColumns + ` FROM flows`
	var args []any
	if kind != "" {
		query += ` WHERE kind = ?`
		args = append(args, string(kind))
	}
	query += ` ORDER BY updated_at DESC, id`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*FlowRecord
	for rows.Next() {
		r, err := scanFlow(rows)
		if err != nil {
			s.logger.Warn("skipping unreadable flow", "error", err)
			continue
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// SaveDraft replaces the draft if baseRevision is current and returns the new revision.
// On a conflict it returns the current revision and ErrRevisionConflict.
// The stored document must name the row it lives in, so f.ID has to equal id;
// SaveDraft rejects a mismatch rather than rewriting the id, and it does not
// modify f.
func (s *Store) SaveDraft(ctx context.Context, id string, f *Flow, baseRevision int, now time.Time) (int, error) {
	if f == nil {
		return 0, errors.New("flow is required")
	}
	if f.ID != id {
		return 0, fmt.Errorf("draft carries flow id %s but is saved under %s", quoteForError(f.ID), quoteForError(id))
	}
	data, err := f.Marshal()
	if err != nil {
		return 0, err
	}
	if len(data) > MaxDocumentBytes {
		return 0, ErrDocumentTooLarge
	}
	res, err := s.db.ExecContext(ctx, `UPDATE flows SET draft_json = ?, name = ?, description = ?,
		draft_revision = draft_revision + 1, updated_at = ? WHERE id = ? AND draft_revision = ?`,
		string(data), f.Name, f.Description, formatTime(now), id, baseRevision)
	if err != nil {
		return 0, fmt.Errorf("save draft: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 1 {
		return baseRevision + 1, nil
	}
	var current int
	err = s.db.QueryRowContext(ctx, `SELECT draft_revision FROM flows WHERE id = ?`, id).Scan(&current)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrNotFound
	}
	if err != nil {
		return 0, err
	}
	return current, ErrRevisionConflict
}

// Publish copies the draft to the live revision, stores a version and keeps the last 50 versions.
func (s *Store) Publish(ctx context.Context, id string, baseRevision int, now time.Time) (*FlowRecord, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	var draftJSON string
	var draftRev, liveRev int
	err = tx.QueryRowContext(ctx, `SELECT draft_json, draft_revision, live_revision FROM flows WHERE id = ?`, id).
		Scan(&draftJSON, &draftRev, &liveRev)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if draftRev != baseRevision {
		return nil, ErrRevisionConflict
	}
	next := liveRev + 1
	ts := formatTime(now)
	if _, err := tx.ExecContext(ctx, `UPDATE flows SET live_json = ?, live_revision = ?, published_draft_revision = ?,
		published_at = ?, updated_at = ? WHERE id = ?`, draftJSON, next, draftRev, ts, ts, id); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO flow_versions (flow_id, revision, json, published_at) VALUES (?, ?, ?, ?)`,
		id, next, draftJSON, ts); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM flow_versions WHERE flow_id = ? AND revision <= ?`, id, next-maxStoredVersions); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.GetFlow(ctx, id)
}

// GetVersion returns a published revision.
func (s *Store) GetVersion(ctx context.Context, id string, revision int) (*Flow, error) {
	var data string
	err := s.db.QueryRowContext(ctx, `SELECT json FROM flow_versions WHERE flow_id = ? AND revision = ?`, id, revision).Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return ParseFlow([]byte(data))
}

// SetMissionID links the flow to its mission.
func (s *Store) SetMissionID(ctx context.Context, id, missionID string) error {
	res, err := s.db.ExecContext(ctx, `UPDATE flows SET mission_id = ? WHERE id = ?`, missionID, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteFlow removes the flow with its versions, runs, steps, timers and test data.
func (s *Store) DeleteFlow(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM flows WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}
