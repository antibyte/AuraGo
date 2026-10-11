package cydcreator

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"aurago/internal/dbutil"

	_ "modernc.org/sqlite"
)

// timeLayout is the fixed-width RFC 3339 UTC form of every stored time, so
// lexical order in SQL equals chronological order.
const timeLayout = "2006-01-02T15:04:05.000Z07:00"

func formatTime(t time.Time) string { return t.UTC().Format(timeLayout) }

// storeMigrations[i] migrates schema version i to i+1. Append new migrations;
// never edit a released one. Version 0 is an empty database.
var storeMigrations = []func(tx *sql.Tx) error{migrateStoreV1}

// openStore opens (and creates) the CYD Creator database, migrates it and
// returns the handle. dbutil applies WAL, foreign keys and a busy timeout.
func openStore(path string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, fmt.Errorf("create cyd creator data directory: %w", err)
	}
	db, err := dbutil.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open cyd creator database: %w", err)
	}
	if err := migrateStore(db, path); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

// storeSchemaVersion reads cc_meta.schema_version (0 when missing).
func storeSchemaVersion(db *sql.DB) (int, error) {
	var raw string
	err := db.QueryRow(`SELECT value FROM cc_meta WHERE key='schema_version'`).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("read cyd creator schema version: %w", err)
	}
	version, err := strconv.Atoi(raw)
	if err != nil || version < 0 {
		return 0, fmt.Errorf("invalid cyd creator schema version %q", raw)
	}
	return version, nil
}

// migrateStore brings the schema to len(storeMigrations). Every step runs in
// one transaction together with the new cc_meta version. An existing database
// (version >= 1) is copied to "<path>.v<version>.bak" with VACUUM INTO before
// the first step, so a failed upgrade never loses data.
func migrateStore(db *sql.DB, path string) error {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS cc_meta (key TEXT PRIMARY KEY, value TEXT NOT NULL)`); err != nil {
		return fmt.Errorf("create cc_meta: %w", err)
	}
	version, err := storeSchemaVersion(db)
	if err != nil {
		return err
	}
	target := len(storeMigrations)
	if version > target {
		return fmt.Errorf("cyd creator database schema %d is newer than this AuraGo supports (%d)", version, target)
	}
	if version >= 1 && version < target && path != "" && path != ":memory:" {
		backup := fmt.Sprintf("%s.v%d.bak", path, version)
		_ = os.Remove(backup)
		if _, err := db.Exec(`VACUUM INTO ?`, backup); err != nil {
			return fmt.Errorf("back up cyd creator database before migration: %w", err)
		}
	}
	for v := version; v < target; v++ {
		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("begin cyd creator migration %d: %w", v+1, err)
		}
		if err := storeMigrations[v](tx); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("migrate cyd creator database to version %d: %w", v+1, err)
		}
		if _, err := tx.Exec(`INSERT INTO cc_meta(key,value) VALUES('schema_version',?)
			ON CONFLICT(key) DO UPDATE SET value=excluded.value`, strconv.Itoa(v+1)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("record cyd creator schema version %d: %w", v+1, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit cyd creator migration %d: %w", v+1, err)
		}
	}
	return nil
}

// migrateStoreV1 creates every table of the overview "Storage schema"; plans 3
// and 5 use cc_jobs, cc_messages, cc_agent_context, cc_revisions,
// cc_revision_files and cc_devices without changing them.
func migrateStoreV1(tx *sql.Tx) error {
	const schema = `
CREATE TABLE cc_projects (
  id TEXT PRIMARY KEY, slug TEXT NOT NULL UNIQUE, name TEXT NOT NULL, board TEXT NOT NULL, theme TEXT NOT NULL,
  version INTEGER NOT NULL DEFAULT 1, locked_by_job TEXT NOT NULL DEFAULT '', draft_job TEXT NOT NULL DEFAULT '',
  last_build TEXT NOT NULL DEFAULT '', app_changed_since_job INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL, updated_at TEXT NOT NULL);
CREATE TABLE cc_builds (
  id TEXT PRIMARY KEY, project_id TEXT NOT NULL, job_id TEXT NOT NULL DEFAULT '', origin TEXT NOT NULL,
  status TEXT NOT NULL, board TEXT NOT NULL, project_version INTEGER NOT NULL, error_code TEXT NOT NULL DEFAULT '',
  diagnostics TEXT NOT NULL DEFAULT '[]', usage TEXT NOT NULL DEFAULT '{}', artifacts TEXT NOT NULL DEFAULT '[]',
  created_at TEXT NOT NULL, started_at TEXT NOT NULL DEFAULT '', finished_at TEXT NOT NULL DEFAULT '');
CREATE INDEX cc_builds_project ON cc_builds(project_id, created_at);
CREATE TABLE cc_events (
  seq INTEGER PRIMARY KEY AUTOINCREMENT, project_id TEXT NOT NULL, type TEXT NOT NULL,
  job_id TEXT NOT NULL DEFAULT '', build_id TEXT NOT NULL DEFAULT '', data TEXT NOT NULL, created_at TEXT NOT NULL);
CREATE INDEX cc_events_project ON cc_events(project_id, seq);
CREATE TABLE cc_jobs (
  id TEXT PRIMARY KEY, project_id TEXT NOT NULL, mode TEXT NOT NULL, status TEXT NOT NULL, phase TEXT NOT NULL,
  prompt TEXT NOT NULL, plan TEXT NOT NULL DEFAULT '', repair_attempt INTEGER NOT NULL DEFAULT 0,
  error_code TEXT NOT NULL DEFAULT '', error_message TEXT NOT NULL DEFAULT '', build_id TEXT NOT NULL DEFAULT '',
  staging_dir TEXT NOT NULL DEFAULT '', resume_of TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL, started_at TEXT NOT NULL DEFAULT '', finished_at TEXT NOT NULL DEFAULT '');
CREATE INDEX cc_jobs_project ON cc_jobs(project_id, created_at);
CREATE TABLE cc_messages (
  id TEXT PRIMARY KEY, project_id TEXT NOT NULL, job_id TEXT NOT NULL DEFAULT '', role TEXT NOT NULL,
  text TEXT NOT NULL, attachments TEXT NOT NULL DEFAULT '[]', created_at TEXT NOT NULL);
CREATE INDEX cc_messages_project ON cc_messages(project_id, created_at);
CREATE TABLE cc_agent_context (job_id TEXT PRIMARY KEY, project_id TEXT NOT NULL, data BLOB NOT NULL, updated_at TEXT NOT NULL);
CREATE TABLE cc_revisions (
  id TEXT PRIMARY KEY, project_id TEXT NOT NULL, label TEXT NOT NULL, reason TEXT NOT NULL,
  job_id TEXT NOT NULL DEFAULT '', build_id TEXT NOT NULL DEFAULT '', layout_sha256 TEXT NOT NULL, created_at TEXT NOT NULL);
CREATE INDEX cc_revisions_project ON cc_revisions(project_id, created_at);
CREATE TABLE cc_revision_files (revision_id TEXT NOT NULL, path TEXT NOT NULL, sha256 TEXT NOT NULL, size INTEGER NOT NULL,
  PRIMARY KEY (revision_id, path));
CREATE TABLE cc_devices (
  id TEXT PRIMARY KEY, project_id TEXT NOT NULL, name TEXT NOT NULL, board TEXT NOT NULL, status TEXT NOT NULL,
  token_hash TEXT NOT NULL, build_id TEXT NOT NULL DEFAULT '', target_build_id TEXT NOT NULL DEFAULT '',
  ip TEXT NOT NULL DEFAULT '', rssi INTEGER NOT NULL DEFAULT 0, free_heap INTEGER NOT NULL DEFAULT 0,
  template_version TEXT NOT NULL DEFAULT '', ota_state TEXT NOT NULL DEFAULT '', last_error TEXT NOT NULL DEFAULT '',
  last_seen TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL, expires_at TEXT NOT NULL DEFAULT '');
CREATE INDEX cc_devices_project ON cc_devices(project_id);
CREATE UNIQUE INDEX cc_devices_token ON cc_devices(token_hash);`
	_, err := tx.Exec(schema)
	return err
}

// recoveredBuild is a build that was queued or running when the previous
// process stopped.
type recoveredBuild struct{ ID, ProjectID, JobID string }

// recoverStore runs once at startup: queued and running builds become
// interrupted (CYD_BUILD_INTERRUPTED), queued and running jobs become
// interrupted, and project locks held by a job that is no longer active
// (awaiting_confirmation stays active) are released. Everything runs in one
// transaction: a crash midway leaves the rows unchanged, so the next start
// recovers the same builds.
func recoverStore(ctx context.Context, db *sql.DB, now string) ([]recoveredBuild, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin cyd creator recovery: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	rows, err := tx.QueryContext(ctx, `SELECT id, project_id, job_id FROM cc_builds WHERE status IN ('queued','running') ORDER BY created_at, id`)
	if err != nil {
		return nil, fmt.Errorf("list unfinished cyd creator builds: %w", err)
	}
	var builds []recoveredBuild
	for rows.Next() {
		var b recoveredBuild
		if err := rows.Scan(&b.ID, &b.ProjectID, &b.JobID); err != nil {
			_ = rows.Close()
			return nil, err
		}
		builds = append(builds, b)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if len(builds) > 0 {
		args := make([]any, 0, len(builds)+3)
		args = append(args, BuildInterrupted, CodeBuildInterrupted, now)
		for _, b := range builds {
			args = append(args, b.ID)
		}
		if _, err := tx.ExecContext(ctx, `UPDATE cc_builds SET status=?, error_code=?, finished_at=?
			WHERE id IN (`+sqlPlaceholders(len(builds))+`)`, args...); err != nil {
			return nil, fmt.Errorf("mark interrupted cyd creator builds: %w", err)
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE cc_jobs SET status='interrupted',
		error_message='The server restarted while the job was active.', finished_at=?
		WHERE status IN ('queued','running')`, now); err != nil {
		return nil, fmt.Errorf("mark interrupted cyd creator jobs: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE cc_projects SET locked_by_job=''
		WHERE locked_by_job<>'' AND locked_by_job NOT IN (SELECT id FROM cc_jobs WHERE status='awaiting_confirmation')`); err != nil {
		return nil, fmt.Errorf("release stale cyd creator project locks: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit cyd creator recovery: %w", err)
	}
	return builds, nil
}

// projectRow mirrors one cc_projects row.
type projectRow struct {
	ID, Slug, Name, Board, Theme string
	Version                      int64
	LockedByJob, DraftJob        string
	LastBuild                    string
	AppChangedSinceJob           bool
	CreatedAt, UpdatedAt         string
}

const projectColumns = `id,slug,name,board,theme,version,locked_by_job,draft_job,last_build,app_changed_since_job,created_at,updated_at`

type rowScanner interface{ Scan(dest ...any) error }

func scanProjectRow(row rowScanner) (projectRow, error) {
	var p projectRow
	var changed int
	err := row.Scan(&p.ID, &p.Slug, &p.Name, &p.Board, &p.Theme, &p.Version, &p.LockedByJob, &p.DraftJob,
		&p.LastBuild, &changed, &p.CreatedAt, &p.UpdatedAt)
	p.AppChangedSinceJob = changed != 0
	return p, err
}

const buildColumns = `id,project_id,job_id,origin,status,board,project_version,error_code,diagnostics,usage,artifacts,created_at,started_at,finished_at`

// scanBuild reads one cc_builds row. The JSON columns are written only by the
// service, so a column that does not decode means a damaged row: it is logged
// (build id and column, never the content) and read as empty, so one bad row
// cannot break a build list or lookup.
func scanBuild(row rowScanner) (Build, error) {
	var b Build
	var diagnostics, usage, artifacts string
	if err := row.Scan(&b.ID, &b.ProjectID, &b.JobID, &b.Origin, &b.Status, &b.Board, &b.ProjectVersion, &b.ErrorCode,
		&diagnostics, &usage, &artifacts, &b.CreatedAt, &b.StartedAt, &b.FinishedAt); err != nil {
		return Build{}, err
	}
	if err := json.Unmarshal([]byte(diagnostics), &b.Diagnostics); err != nil {
		warnBadBuildColumn(b.ID, "diagnostics", err)
		b.Diagnostics = nil
	}
	if err := json.Unmarshal([]byte(usage), &b.Usage); err != nil {
		warnBadBuildColumn(b.ID, "usage", err)
		b.Usage = BuildUsage{}
	}
	if err := json.Unmarshal([]byte(artifacts), &b.Artifacts); err != nil {
		warnBadBuildColumn(b.ID, "artifacts", err)
		b.Artifacts = nil
	}
	if b.Diagnostics == nil {
		b.Diagnostics = []Diagnostic{}
	}
	if b.Artifacts == nil {
		b.Artifacts = []ArtifactInfo{}
	}
	return b, nil
}

// warnBadBuildColumn logs an undecodable JSON column of a build row. Only the
// error's type is logged: a decoder message can quote part of the content.
func warnBadBuildColumn(buildID, column string, err error) {
	slog.Warn("CYD Creator build record has an unreadable column; reading it as empty",
		"build_id", buildID, "column", column, "error_type", fmt.Sprintf("%T", err))
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func mustJSON(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		return "null"
	}
	return string(data)
}

// sqlPlaceholders returns "?,?,…" for n values.
func sqlPlaceholders(n int) string {
	if n <= 0 {
		return ""
	}
	return strings.TrimSuffix(strings.Repeat("?,", n), ",")
}
