package flows

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const flowScenarioSchema = `CREATE TABLE flow_test_scenarios (
	flow_id TEXT NOT NULL REFERENCES flows(id) ON DELETE CASCADE,
	id TEXT NOT NULL,
	ciphertext BLOB NOT NULL,
	updated_at TEXT NOT NULL,
	PRIMARY KEY (flow_id,id)
)`

func flowSchemaVersion(db *sql.DB) (int, error) {
	version, _, err := flowSchemaVersionAndMarker(db)
	return version, err
}

func flowSchemaVersionAndMarker(db *sql.DB) (int, string, error) {
	var exists int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='flows_meta'`).Scan(&exists); err != nil {
		return 0, "", err
	}
	if exists == 0 {
		return 1, "", nil
	}
	var raw string
	err := db.QueryRow(`SELECT value FROM flows_meta WHERE key='schema_version'`).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return 1, "", nil
	}
	if err != nil {
		return 0, "", err
	}
	version, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || version < 1 {
		return 0, "", fmt.Errorf("invalid flows database schema version")
	}
	return version, raw, nil
}

// backupFlowsBeforeV2 preserves a verified, unique SQLite snapshot before any
// schema statement changes an existing on-disk database.
func backupFlowsBeforeV2(db *sql.DB, logger *slog.Logger) error {
	version, err := flowSchemaVersion(db)
	if err != nil {
		return fmt.Errorf("read flows schema for backup: %w", err)
	}
	if version >= 2 {
		for _, query := range []string{
			`SELECT run_id,generation,state,wake_at,deadline,ciphertext,lease_owner,lease_until,updated_at FROM flow_checkpoints LIMIT 0`,
			`SELECT flow_id,id,ciphertext,updated_at FROM flow_test_scenarios LIMIT 0`,
		} {
			rows, err := db.Query(query)
			if err != nil {
				return fmt.Errorf("verify flows schema: %w", err)
			}
			if err := rows.Close(); err != nil {
				return fmt.Errorf("verify flows schema: %w", err)
			}
		}
		return nil
	}
	var exists int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'`).Scan(&exists); err != nil {
		return fmt.Errorf("inspect flows database: %w", err)
	}
	if exists == 0 {
		return nil
	}
	return backupFlowsSnapshot(db, logger, ".flows-v1-backup-*")
}

func backupFlowsSnapshot(db *sql.DB, logger *slog.Logger, prefix string) error {
	var path string
	if err := db.QueryRow(`SELECT file FROM pragma_database_list WHERE name='main'`).Scan(&path); err != nil {
		return fmt.Errorf("locate flows database: %w", err)
	}
	if path == "" {
		return nil
	}
	dir, err := os.MkdirTemp(filepath.Dir(path), prefix)
	if err != nil {
		return fmt.Errorf("create flows migration backup directory: %w", err)
	}
	complete := false
	defer func() {
		if !complete {
			_ = os.RemoveAll(dir)
		}
	}()
	backup := filepath.Join(dir, "flows.db")
	if _, err := db.Exec(`VACUUM INTO ?`, backup); err != nil {
		return fmt.Errorf("back up flows database: %w", err)
	}
	if err := os.Chmod(backup, 0o600); err != nil {
		return fmt.Errorf("protect flows backup: %w", err)
	}
	copyDB, err := sql.Open("sqlite", backup)
	if err != nil {
		return fmt.Errorf("open flows backup: %w", err)
	}
	var integrity string
	err = copyDB.QueryRow(`PRAGMA integrity_check`).Scan(&integrity)
	closeErr := copyDB.Close()
	if err != nil || integrity != "ok" || closeErr != nil {
		return fmt.Errorf("verify flows backup: %w", errors.Join(err, closeErr, invalidBackupIntegrity(integrity)))
	}
	file, err := os.OpenFile(backup, os.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("open flows backup for sync: %w", err)
	}
	syncErr := file.Sync()
	closeErr = file.Close()
	if syncErr != nil || closeErr != nil {
		return fmt.Errorf("sync flows backup: %w", errors.Join(syncErr, closeErr))
	}
	complete = true
	logger.Info("flows database backed up before schema migration", "backup", backup)
	return nil
}

func invalidBackupIntegrity(result string) error {
	if result == "ok" {
		return nil
	}
	return errors.New("backup integrity check failed")
}

func migrateFlowsV2(db *sql.DB) error {
	version, marker, err := flowSchemaVersionAndMarker(db)
	if err != nil {
		return fmt.Errorf("read flows schema: %w", err)
	}
	if version >= 2 {
		return nil
	}
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin flows schema migration: %w", err)
	}
	defer tx.Rollback()
	for _, statement := range []string{durableCheckpointSchema, `CREATE INDEX idx_flow_checkpoints_due ON flow_checkpoints(state,wake_at)`, flowScenarioSchema} {
		if _, err := tx.Exec(statement); err != nil {
			return fmt.Errorf("migrate flows schema: %w", err)
		}
	}
	result, err := tx.Exec(`UPDATE flows_meta SET value='2' WHERE key='schema_version' AND value=?`, marker)
	if err != nil {
		return fmt.Errorf("record flows schema: %w", err)
	}
	if n, err := result.RowsAffected(); err != nil || n != 1 {
		return errors.New("record flows schema: version row missing")
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit flows schema: %w", err)
	}
	return nil
}
