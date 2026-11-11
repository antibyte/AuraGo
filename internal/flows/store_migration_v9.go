package flows

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
)

// versionPurgeSchema marks published versions that the credential purge removes (or removes as soon
// as nothing pins them). A mark goes with its version (ON DELETE CASCADE), so the table never grows
// stale rows, and notes go with the version the same way.
const versionPurgeSchema = `CREATE TABLE flow_version_purge (
	flow_id TEXT NOT NULL,
	revision INTEGER NOT NULL,
	marked_at TEXT NOT NULL,
	PRIMARY KEY (flow_id, revision),
	FOREIGN KEY (flow_id, revision) REFERENCES flow_versions(flow_id, revision) ON DELETE CASCADE
)`

func verifyFlowsV9(db *sql.DB) error {
	version, err := flowSchemaVersion(db)
	if err != nil || version < 9 {
		return err
	}
	var table string
	if err := db.QueryRow(`SELECT sql FROM sqlite_master WHERE type='table' AND name='flow_version_purge'`).Scan(&table); err != nil {
		return fmt.Errorf("verify version purge table: %w", err)
	}
	canonical := func(s string) string { return strings.ToLower(strings.Join(strings.Fields(s), "")) }
	if canonical(table) != canonical(versionPurgeSchema) {
		return errors.New("invalid version purge table")
	}
	return nil
}

func migrateFlowsV9(db *sql.DB, logger *slog.Logger, existing bool) error {
	version, err := flowSchemaVersion(db)
	if err != nil {
		return err
	}
	if version >= 9 {
		return verifyFlowsV9(db)
	}
	if version != 8 {
		return errors.New("flows v9 migration requires schema v8")
	}
	if existing {
		if err := backupFlowsSnapshot(db, logger, ".flows-v8-backup-*"); err != nil {
			return err
		}
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(versionPurgeSchema); err != nil {
		return fmt.Errorf("create version purge table: %w", err)
	}
	result, err := tx.Exec(`UPDATE flows_meta SET value='9' WHERE key='schema_version' AND value='8'`)
	if err != nil {
		return fmt.Errorf("record flows v9 schema: %w", err)
	}
	if n, err := result.RowsAffected(); err != nil || n != 1 {
		return errors.New("record flows v9 schema: version changed")
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return verifyFlowsV9(db)
}
