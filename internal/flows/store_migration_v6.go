package flows

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
)

func verifyFlowsV6(db *sql.DB) error {
	version, err := flowSchemaVersion(db)
	if err != nil || version < 6 {
		return err
	}
	var table string
	if err := db.QueryRow(`SELECT sql FROM sqlite_master WHERE type='table' AND name='flow_runs'`).Scan(&table); err != nil {
		return fmt.Errorf("verify durable run schema: %w", err)
	}
	canonical := func(s string) string { return strings.ToLower(strings.Join(strings.Fields(s), "")) }
	if !strings.Contains(canonical(table), `durableintegernotnulldefault0check(durablein(0,1))`) {
		return errors.New("invalid durable run schema")
	}
	if !strings.Contains(canonical(table), `durable_mission_idtextnotnulldefault''check(length(durable_mission_id)<=128)`) {
		return errors.New("invalid durable mission owner schema")
	}
	return nil
}

func migrateFlowsV6(db *sql.DB, logger *slog.Logger, existing bool) error {
	version, err := flowSchemaVersion(db)
	if err != nil {
		return err
	}
	if version >= 6 {
		return verifyFlowsV6(db)
	}
	if version != 5 {
		return errors.New("flows v6 migration requires schema v5")
	}
	if existing {
		if err := backupFlowsSnapshot(db, logger, ".flows-v5-backup-*"); err != nil {
			return err
		}
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`ALTER TABLE flow_runs ADD COLUMN durable INTEGER NOT NULL DEFAULT 0 CHECK (durable IN (0,1))`); err != nil {
		return fmt.Errorf("add durable run marker: %w", err)
	}
	if _, err := tx.Exec(`ALTER TABLE flow_runs ADD COLUMN durable_mission_id TEXT NOT NULL DEFAULT '' CHECK (length(durable_mission_id)<=128)`); err != nil {
		return fmt.Errorf("add durable mission owner: %w", err)
	}
	result, err := tx.Exec(`UPDATE flows_meta SET value='6' WHERE key='schema_version' AND value='5'`)
	if err != nil {
		return fmt.Errorf("record flows v6 schema: %w", err)
	}
	if n, err := result.RowsAffected(); err != nil || n != 1 {
		return errors.New("record flows v6 schema: version changed")
	}
	return tx.Commit()
}
