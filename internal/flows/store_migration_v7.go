package flows

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
)

const childTracesSchema = `CREATE TABLE flow_child_traces (
	run_id TEXT NOT NULL,
	parent_seq INTEGER NOT NULL CHECK(parent_seq > 0),
	summary_json TEXT NOT NULL CHECK(length(CAST(summary_json AS BLOB)) <= 2048),
	trace_json TEXT CHECK(trace_json IS NULL OR length(CAST(trace_json AS BLOB)) <= 262144),
	PRIMARY KEY(run_id,parent_seq),
	FOREIGN KEY(run_id,parent_seq) REFERENCES flow_run_steps(run_id,seq) ON DELETE CASCADE
)`

var childHistoryColumns = []string{
	`child_history_state TEXT NOT NULL DEFAULT 'legacy_unknown' CHECK(child_history_state IN ('legacy_unknown','pending','complete','truncated','incomplete'))`,
	`child_history_bytes INTEGER NOT NULL DEFAULT 0 CHECK(child_history_bytes BETWEEN 0 AND 4194304)`,
	`child_history_metadata INTEGER NOT NULL DEFAULT 0 CHECK(child_history_metadata BETWEEN 0 AND 524288)`,
}

func verifyFlowsV7(db *sql.DB) error {
	version, err := flowSchemaVersion(db)
	if err != nil || version < 7 {
		return err
	}
	canonical := func(s string) string { return strings.ToLower(strings.Join(strings.Fields(s), "")) }
	var table string
	if err := db.QueryRow(`SELECT sql FROM sqlite_master WHERE type='table' AND name='flow_child_traces'`).Scan(&table); err != nil {
		return fmt.Errorf("verify child history table: %w", err)
	}
	if canonical(table) != canonical(childTracesSchema) {
		return errors.New("invalid child history table")
	}
	if err := db.QueryRow(`SELECT sql FROM sqlite_master WHERE type='table' AND name='flow_runs'`).Scan(&table); err != nil {
		return err
	}
	for _, column := range childHistoryColumns {
		if !strings.Contains(canonical(table), canonical(column)) {
			return errors.New("invalid child history run schema")
		}
	}
	return nil
}

func migrateFlowsV7(db *sql.DB, logger *slog.Logger, existing bool) error {
	version, err := flowSchemaVersion(db)
	if err != nil {
		return err
	}
	if version >= 7 {
		return verifyFlowsV7(db)
	}
	if version != 6 {
		return errors.New("flows v7 migration requires schema v6")
	}
	if existing {
		if err := backupFlowsSnapshot(db, logger, ".flows-v6-backup-*"); err != nil {
			return err
		}
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, column := range childHistoryColumns {
		if _, err := tx.Exec(`ALTER TABLE flow_runs ADD COLUMN ` + column); err != nil {
			return fmt.Errorf("add child history column: %w", err)
		}
	}
	if _, err := tx.Exec(childTracesSchema); err != nil {
		return fmt.Errorf("add child history table: %w", err)
	}
	result, err := tx.Exec(`UPDATE flows_meta SET value='7' WHERE key='schema_version' AND value='6'`)
	if err != nil {
		return fmt.Errorf("record flows v7 schema: %w", err)
	}
	if n, err := result.RowsAffected(); err != nil || n != 1 {
		return errors.New("record flows v7 schema: version changed")
	}
	return tx.Commit()
}
