package infra

import (
	"context"
	"crypto/rand"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

const schemaVersion = 2

const timeLayout = "2006-01-02T15:04:05.000000000Z"

const RemovedSourceRetention = 90 * 24 * time.Hour

func formatTime(t time.Time) string { return t.UTC().Format(timeLayout) }

type Store struct {
	db *sql.DB
}

// New migrates the shared inventory database and returns a store that never owns it.
func New(db *sql.DB) (*Store, error) {
	if db == nil {
		return nil, fmt.Errorf("open infra store: nil database")
	}
	if err := migrate(db); err != nil {
		return nil, err
	}
	// Persisted provider freshness never survives process startup as current.
	if _, err := db.Exec(`UPDATE infra_sources SET state='stale' WHERE state='current'`); err != nil {
		return nil, fmt.Errorf("mark prior infra observations stale: %w", err)
	}
	if _, err := db.Exec(`UPDATE infra_action_requests SET status='unknown', updated_at=? WHERE status='running'`, formatTime(time.Now())); err != nil {
		return nil, fmt.Errorf("mark interrupted infra actions unknown: %w", err)
	}
	return &Store{db: db}, nil
}

func migrate(db *sql.DB) error {
	var hasMeta bool
	if err := db.QueryRow(`SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE type='table' AND name='infra_meta')`).Scan(&hasMeta); err != nil {
		return fmt.Errorf("inspect infra schema version: %w", err)
	}
	version := 0
	if hasMeta {
		if err := db.QueryRow(`SELECT version FROM infra_meta WHERE id=1`).Scan(&version); err != nil && err != sql.ErrNoRows {
			return fmt.Errorf("read infra schema version: %w", err)
		}
	}
	if version > schemaVersion {
		return fmt.Errorf("infra schema %d is newer than supported version %d", version, schemaVersion)
	}
	if version == schemaVersion {
		return nil
	}
	if err := backupExistingDatabase(db, schemaVersion); err != nil {
		return err
	}
	if version == 1 {
		return migrateActionLedgerV2(db)
	}
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin infra migration: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`
		CREATE TABLE IF NOT EXISTS infra_meta (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			version INTEGER NOT NULL
		);
		CREATE TABLE IF NOT EXISTS infra_nodes (
			id TEXT PRIMARY KEY,
			version INTEGER NOT NULL CHECK (version > 0),
			kind TEXT NOT NULL,
			name TEXT NOT NULL,
			icon TEXT NOT NULL,
			registry_id TEXT NOT NULL DEFAULT '',
			parent_id TEXT,
			profile_json TEXT NOT NULL,
			placements_json TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			FOREIGN KEY(parent_id) REFERENCES infra_nodes(id) ON DELETE RESTRICT
		);
		CREATE UNIQUE INDEX IF NOT EXISTS idx_infra_registry_id
			ON infra_nodes(registry_id) WHERE registry_id <> '';
		CREATE INDEX IF NOT EXISTS idx_infra_nodes_parent ON infra_nodes(parent_id);
		CREATE TABLE IF NOT EXISTS infra_sources (
			kind TEXT NOT NULL,
			instance TEXT NOT NULL,
			source_key TEXT NOT NULL,
			node_id TEXT,
			name TEXT NOT NULL DEFAULT '',
			state TEXT NOT NULL,
			fetched_at TEXT NOT NULL DEFAULT '',
			removed_at TEXT NOT NULL DEFAULT '',
			observed_json TEXT NOT NULL,
			PRIMARY KEY(kind, instance, source_key),
			FOREIGN KEY(node_id) REFERENCES infra_nodes(id) ON DELETE SET NULL
		);
		CREATE INDEX IF NOT EXISTS idx_infra_sources_node ON infra_sources(node_id);
		CREATE INDEX IF NOT EXISTS idx_infra_sources_freshness ON infra_sources(kind, instance, state, fetched_at);
		CREATE TABLE IF NOT EXISTS infra_edges (
			id TEXT PRIMARY KEY,
			version INTEGER NOT NULL CHECK (version > 0),
			from_id TEXT NOT NULL,
			to_id TEXT NOT NULL,
			kind TEXT NOT NULL,
			label TEXT NOT NULL DEFAULT '',
			from_port TEXT NOT NULL DEFAULT '',
			to_port TEXT NOT NULL DEFAULT '',
			speed TEXT NOT NULL DEFAULT '',
			source_kind TEXT NOT NULL DEFAULT '',
			source_instance TEXT NOT NULL DEFAULT '',
			source_key TEXT NOT NULL DEFAULT '',
			updated_at TEXT NOT NULL,
			FOREIGN KEY(from_id) REFERENCES infra_nodes(id) ON DELETE CASCADE,
			FOREIGN KEY(to_id) REFERENCES infra_nodes(id) ON DELETE CASCADE
		);
		CREATE INDEX IF NOT EXISTS idx_infra_edges_from ON infra_edges(from_id);
		CREATE INDEX IF NOT EXISTS idx_infra_edges_to ON infra_edges(to_id);
		CREATE INDEX IF NOT EXISTS idx_infra_edges_source ON infra_edges(source_kind, source_instance, source_key);
		CREATE TABLE IF NOT EXISTS infra_action_requests (
			request_id TEXT PRIMARY KEY,
			fingerprint TEXT NOT NULL,
			action TEXT NOT NULL,
			status TEXT NOT NULL CHECK(status IN ('running','accepted','completed','failed','unknown')),
			upid TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_infra_actions_updated ON infra_action_requests(updated_at);
		INSERT INTO infra_meta(id, version) VALUES(1, 2)
			ON CONFLICT(id) DO UPDATE SET version=excluded.version;
	`); err != nil {
		return fmt.Errorf("create infra schema: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit infra migration: %w", err)
	}
	return nil
}

func migrateActionLedgerV2(db *sql.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin infra action ledger migration: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`
		DROP INDEX IF EXISTS idx_infra_actions_updated;
		ALTER TABLE infra_action_requests RENAME TO infra_action_requests_v1;
		CREATE TABLE infra_action_requests (
			request_id TEXT PRIMARY KEY,
			fingerprint TEXT NOT NULL,
			action TEXT NOT NULL,
			status TEXT NOT NULL CHECK(status IN ('running','accepted','completed','failed','unknown')),
			upid TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);
		CREATE INDEX idx_infra_actions_updated ON infra_action_requests(updated_at);
		INSERT INTO infra_action_requests(request_id,fingerprint,action,status,upid,created_at,updated_at)
			SELECT request_id,fingerprint,action,
				CASE WHEN status='accepted' THEN 'unknown' ELSE status END,
				upid,created_at,updated_at
			FROM infra_action_requests_v1;
		DROP TABLE infra_action_requests_v1;
		UPDATE infra_meta SET version=2 WHERE id=1;
	`); err != nil {
		return fmt.Errorf("migrate infra action ledger: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit infra action ledger migration: %w", err)
	}
	return nil
}

func backupExistingDatabase(db *sql.DB, targetVersion int) error {
	rows, err := db.Query(`PRAGMA database_list`)
	if err != nil {
		return fmt.Errorf("locate inventory database for infra migration backup: %w", err)
	}
	path := ""
	for rows.Next() {
		var seq int
		var name, file string
		if err := rows.Scan(&seq, &name, &file); err != nil {
			_ = rows.Close()
			return fmt.Errorf("read inventory database path: %w", err)
		}
		if name == "main" {
			path = file
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return fmt.Errorf("read inventory database path: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close inventory database path query: %w", err)
	}
	if path == "" {
		return nil
	}
	backup := fmt.Sprintf("%s.infra-v%d-%s.bak", path, targetVersion, rand.Text())
	if _, err := db.Exec(`VACUUM main INTO ?`, backup); err != nil {
		return fmt.Errorf("back up inventory database before infra migration: %w", err)
	}
	if err := os.Chmod(backup, 0o600); err != nil {
		return fmt.Errorf("protect inventory database migration backup: %w", err)
	}
	pruneMigrationBackups(path)
	return nil
}

// migrationBackupRetention bounds the full database copies a failed or repeated
// migration can leave next to the inventory database.
const migrationBackupRetention = 3

func pruneMigrationBackups(databasePath string) {
	matches, err := filepath.Glob(databasePath + ".infra-v*.bak")
	if err != nil || len(matches) <= migrationBackupRetention {
		return
	}
	type backupFile struct {
		path    string
		modTime time.Time
	}
	files := make([]backupFile, 0, len(matches))
	for _, match := range matches {
		info, statErr := os.Stat(match)
		if statErr != nil || !info.Mode().IsRegular() {
			continue
		}
		files = append(files, backupFile{path: match, modTime: info.ModTime()})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].modTime.After(files[j].modTime) })
	for _, stale := range files[migrationBackupRetention:] {
		_ = os.Remove(stale.path)
	}
}

func (s *Store) PruneSources(ctx context.Context, olderThan time.Time) (int64, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM infra_sources
		WHERE node_id IS NULL AND state='removed' AND removed_at<>'' AND removed_at<?`, formatTime(olderThan))
	if err != nil {
		return 0, fmt.Errorf("prune expired infra source observations: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("count expired infra source observations: %w", err)
	}
	return n, nil
}
