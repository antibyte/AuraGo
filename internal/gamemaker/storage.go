package gamemaker

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"aurago/internal/dbutil"

	_ "modernc.org/sqlite"
)

func openDatabase(path string) (*sql.DB, error) {
	db, err := dbutil.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open game maker database: %w", err)
	}
	if err := migrate(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func migrate(db *sql.DB) error {
	if err := migrateVoxelVariant(db); err != nil {
		return err
	}
	const schema = `
CREATE TABLE IF NOT EXISTS gm_projects (
 id TEXT PRIMARY KEY, name TEXT NOT NULL, slug TEXT NOT NULL UNIQUE,
 project_key TEXT NOT NULL UNIQUE, dimension TEXT NOT NULL, description TEXT NOT NULL,
 variant TEXT NOT NULL DEFAULT '',
 provider_id TEXT NOT NULL DEFAULT '', model TEXT NOT NULL DEFAULT '',
 use_image INTEGER NOT NULL DEFAULT 0, use_music INTEGER NOT NULL DEFAULT 0,
 status TEXT NOT NULL DEFAULT 'draft', current_revision INTEGER NOT NULL DEFAULT 0,
 created_at DATETIME NOT NULL, updated_at DATETIME NOT NULL
);
CREATE TABLE IF NOT EXISTS gm_messages (
 id INTEGER PRIMARY KEY AUTOINCREMENT, project_id TEXT NOT NULL, job_id TEXT NOT NULL DEFAULT '',
 role TEXT NOT NULL, content TEXT NOT NULL, created_at DATETIME NOT NULL,
 FOREIGN KEY(project_id) REFERENCES gm_projects(id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS gm_jobs (
 id TEXT PRIMARY KEY, project_id TEXT NOT NULL, kind TEXT NOT NULL, prompt TEXT NOT NULL,
 status TEXT NOT NULL, phase TEXT NOT NULL, provider_id TEXT NOT NULL DEFAULT '',
 model TEXT NOT NULL DEFAULT '', error TEXT NOT NULL DEFAULT '',
 error_code TEXT NOT NULL DEFAULT '', gameplay_status TEXT NOT NULL DEFAULT '',
 checks_passed INTEGER, checks_failed INTEGER, repair_passes INTEGER,
 open_checks TEXT NOT NULL DEFAULT '',
 small_model INTEGER NOT NULL DEFAULT 0, building_mode TEXT NOT NULL DEFAULT '',
 plan_accepted INTEGER NOT NULL DEFAULT 0, requested_selection TEXT NOT NULL DEFAULT '',
 base_revision INTEGER NOT NULL DEFAULT 0, result_revision INTEGER NOT NULL DEFAULT 0,
 created_at DATETIME NOT NULL, started_at DATETIME, finished_at DATETIME,
 FOREIGN KEY(project_id) REFERENCES gm_projects(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS gm_jobs_project_idx ON gm_jobs(project_id, created_at DESC);
CREATE TABLE IF NOT EXISTS gm_agent_context (
 project_id TEXT PRIMARY KEY, job_id TEXT NOT NULL, provider_id TEXT NOT NULL, model TEXT NOT NULL,
 messages BLOB NOT NULL, updated_at DATETIME NOT NULL,
 FOREIGN KEY(project_id) REFERENCES gm_projects(id) ON DELETE CASCADE,
 FOREIGN KEY(job_id) REFERENCES gm_jobs(id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS gm_events (
 id INTEGER PRIMARY KEY AUTOINCREMENT, project_id TEXT NOT NULL, job_id TEXT NOT NULL DEFAULT '',
 event_type TEXT NOT NULL, payload TEXT NOT NULL DEFAULT '{}', created_at DATETIME NOT NULL,
 FOREIGN KEY(project_id) REFERENCES gm_projects(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS gm_events_project_idx ON gm_events(project_id, id);
CREATE TABLE IF NOT EXISTS gm_assets (
 id INTEGER PRIMARY KEY AUTOINCREMENT, project_id TEXT NOT NULL, job_id TEXT NOT NULL DEFAULT '',
 path TEXT NOT NULL, kind TEXT NOT NULL, generator TEXT NOT NULL, provenance TEXT NOT NULL,
 content_hash TEXT NOT NULL, created_at DATETIME NOT NULL,
 FOREIGN KEY(project_id) REFERENCES gm_projects(id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS gm_revisions (
 id INTEGER PRIMARY KEY AUTOINCREMENT, project_id TEXT NOT NULL, number INTEGER NOT NULL,
 parent INTEGER NOT NULL DEFAULT 0, source TEXT NOT NULL, summary TEXT NOT NULL,
 file_count INTEGER NOT NULL, total_bytes INTEGER NOT NULL, created_at DATETIME NOT NULL,
 UNIQUE(project_id, number), FOREIGN KEY(project_id) REFERENCES gm_projects(id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS gm_revision_files (
 revision_id INTEGER NOT NULL, path TEXT NOT NULL, content_hash TEXT NOT NULL, size INTEGER NOT NULL,
 PRIMARY KEY(revision_id, path), FOREIGN KEY(revision_id) REFERENCES gm_revisions(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS gm_revision_files_hash_idx ON gm_revision_files(content_hash);
CREATE TABLE IF NOT EXISTS gm_play_states (
 project_id TEXT NOT NULL, compatibility TEXT NOT NULL, version INTEGER NOT NULL,
 revision INTEGER NOT NULL, payload BLOB, updated_at DATETIME NOT NULL,
 PRIMARY KEY(project_id,compatibility),
 FOREIGN KEY(project_id) REFERENCES gm_projects(id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS gm_blobs (
 content_hash TEXT PRIMARY KEY, size INTEGER NOT NULL, created_at DATETIME NOT NULL
);`
	if _, err := db.Exec(schema); err != nil {
		return fmt.Errorf("migrate game maker database: %w", err)
	}
	if err := ensureJobOutcomeColumns(db); err != nil {
		return err
	}
	_, err := db.Exec(`UPDATE gm_jobs SET status='interrupted', phase='interrupted',
		error='Server restarted while the job was active', finished_at=?
		WHERE status IN ('queued','planning','building','validating','polishing')`, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("mark interrupted game maker jobs: %w", err)
	}
	return nil
}

func scanProject(row interface{ Scan(...any) error }) (Project, error) {
	var p Project
	var image, music int
	err := row.Scan(&p.ID, &p.Name, &p.Slug, &p.ProjectKey, &p.Dimension, &p.Description,
		&p.ProviderID, &p.Model, &image, &music, &p.Status, &p.CurrentRevision, &p.CreatedAt, &p.UpdatedAt, &p.Variant)
	p.UseImageGeneration = image != 0
	p.UseMusicGeneration = music != 0
	return p, err
}

const jobSelectColumns = `id,project_id,kind,prompt,status,phase,provider_id,model,error,error_code,base_revision,result_revision,created_at,started_at,finished_at,gameplay_status,checks_passed,checks_failed,repair_passes,open_checks,small_model,building_mode,plan_accepted`

func scanJob(row interface{ Scan(...any) error }) (Job, error) {
	var j Job
	var started, finished sql.NullTime
	var passed, failed, repairs sql.NullInt64
	var openChecks string
	var smallModel, planAccepted int
	err := row.Scan(&j.ID, &j.ProjectID, &j.Kind, &j.Prompt, &j.Status, &j.Phase,
		&j.ProviderID, &j.Model, &j.Error, &j.ErrorCode, &j.BaseRevision, &j.ResultRevision,
		&j.CreatedAt, &started, &finished, &j.GameplayStatus, &passed, &failed, &repairs, &openChecks,
		&smallModel, &j.BuildingMode, &planAccepted)
	if started.Valid {
		j.StartedAt = &started.Time
	}
	if finished.Valid {
		j.FinishedAt = &finished.Time
	}
	j.SmallModel = smallModel != 0
	j.PlanAccepted = planAccepted != 0
	j.ChecksPassed = nullInt(passed)
	j.ChecksFailed = nullInt(failed)
	j.RepairPasses = nullInt(repairs)
	j.OpenChecks = decodeOpenChecks(openChecks)
	return j, err
}

func nullInt(value sql.NullInt64) *int {
	if !value.Valid {
		return nil
	}
	number := int(value.Int64)
	return &number
}

// ensureJobOutcomeColumns adds outcome fields to databases created before them.
// A backup is written only when a column is missing. Fresh databases already
// have the columns from CREATE TABLE and skip the copy.
func ensureJobOutcomeColumns(db *sql.DB) error {
	rows, err := db.Query(`PRAGMA table_info(gm_jobs)`)
	if err != nil {
		return fmt.Errorf("inspect game maker job columns: %w", err)
	}
	have := map[string]bool{}
	exists := false
	for rows.Next() {
		var cid, notnull, pk int
		var name, kind string
		var def any
		if err := rows.Scan(&cid, &name, &kind, &notnull, &def, &pk); err != nil {
			rows.Close()
			return err
		}
		exists = true
		have[name] = true
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	columns := []struct{ name, ddl string }{
		{"error_code", `ALTER TABLE gm_jobs ADD COLUMN error_code TEXT NOT NULL DEFAULT ''`},
		{"gameplay_status", `ALTER TABLE gm_jobs ADD COLUMN gameplay_status TEXT NOT NULL DEFAULT ''`},
		{"checks_passed", `ALTER TABLE gm_jobs ADD COLUMN checks_passed INTEGER`},
		{"checks_failed", `ALTER TABLE gm_jobs ADD COLUMN checks_failed INTEGER`},
		{"repair_passes", `ALTER TABLE gm_jobs ADD COLUMN repair_passes INTEGER`},
		{"open_checks", `ALTER TABLE gm_jobs ADD COLUMN open_checks TEXT NOT NULL DEFAULT ''`},
		{"small_model", `ALTER TABLE gm_jobs ADD COLUMN small_model INTEGER NOT NULL DEFAULT 0`},
		{"building_mode", `ALTER TABLE gm_jobs ADD COLUMN building_mode TEXT NOT NULL DEFAULT ''`},
		{"plan_accepted", `ALTER TABLE gm_jobs ADD COLUMN plan_accepted INTEGER NOT NULL DEFAULT 0`},
		{"requested_selection", `ALTER TABLE gm_jobs ADD COLUMN requested_selection TEXT NOT NULL DEFAULT ''`},
	}
	missing := false
	for _, column := range columns {
		if !have[column.name] {
			missing = true
			break
		}
	}
	if !missing {
		return nil
	}
	var seq int
	var name, path string
	if err := db.QueryRow(`PRAGMA database_list`).Scan(&seq, &name, &path); err != nil {
		return fmt.Errorf("locate game maker database: %w", err)
	}
	if path != "" {
		backup := path + ".before-job-outcome-" + time.Now().UTC().Format("20060102T150405.000000000") + ".bak"
		if _, err := db.Exec(`VACUUM INTO '` + strings.ReplaceAll(backup, "'", "''") + `'`); err != nil {
			return fmt.Errorf("backup before game maker job outcome migration: %w", err)
		}
	}
	for _, column := range columns {
		if have[column.name] {
			continue
		}
		if _, err := db.Exec(column.ddl); err != nil {
			return fmt.Errorf("migrate game maker job outcome: %w", err)
		}
	}
	return nil
}

func scanRevision(row interface{ Scan(...any) error }) (Revision, error) {
	var revision Revision
	err := row.Scan(&revision.ID, &revision.ProjectID, &revision.Number, &revision.Parent,
		&revision.Source, &revision.Summary, &revision.FileCount, &revision.TotalBytes, &revision.CreatedAt)
	return revision, err
}

func decodeEventPayload(raw string) map[string]any {
	out := map[string]any{}
	_ = json.Unmarshal([]byte(raw), &out)
	return out
}

func ensureStorageDirs(opts Options) (string, string, error) {
	if err := os.MkdirAll(filepath.Dir(opts.DBPath), 0o750); err != nil {
		return "", "", fmt.Errorf("create game maker data directory: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(opts.WorkspacePath, "Games"), 0o750); err != nil {
		return "", "", fmt.Errorf("create game maker workspace: %w", err)
	}
	staging := filepath.Join(opts.WorkspacePath, ".game-maker-staging")
	if err := os.MkdirAll(staging, 0o750); err != nil {
		return "", "", fmt.Errorf("create game maker staging directory: %w", err)
	}
	blobs := filepath.Join(filepath.Dir(opts.DBPath), "game_maker_blobs")
	if err := os.MkdirAll(blobs, 0o750); err != nil {
		return "", "", fmt.Errorf("create game maker blob directory: %w", err)
	}
	return staging, blobs, nil
}
