package flows

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
)

const durableApprovalGatesSchema = `CREATE TABLE flow_durable_gates (
	gate_id TEXT PRIMARY KEY,
	run_id TEXT NOT NULL REFERENCES flow_runs(id) ON DELETE CASCADE,
	flow_id TEXT NOT NULL,
	revision INTEGER NOT NULL CHECK (revision > 0),
	node_id TEXT NOT NULL,
	node_key TEXT NOT NULL,
	admission_owner TEXT NOT NULL,
	gate_generation INTEGER NOT NULL CHECK (gate_generation > 0),
	content_digest TEXT NOT NULL CHECK (length(content_digest) = 64),
	expires_at TEXT NOT NULL,
	state TEXT NOT NULL CHECK (state IN ('pending','approved','rejected','expired','cancelled')),
	created_at TEXT NOT NULL,
	decided_at TEXT NOT NULL DEFAULT '',
	decision TEXT NOT NULL DEFAULT '',
	actor TEXT NOT NULL DEFAULT ''
)`

var durableApprovalGateIndexes = []string{
	`CREATE UNIQUE INDEX idx_flow_durable_gates_pending_run ON flow_durable_gates(run_id) WHERE state='pending'`,
	`CREATE UNIQUE INDEX idx_flow_durable_gates_run_node ON flow_durable_gates(run_id,node_id)`,
	`CREATE INDEX idx_flow_durable_gates_inbox ON flow_durable_gates(state,expires_at,gate_id)`,
	`CREATE INDEX idx_flow_durable_gates_run ON flow_durable_gates(run_id,created_at DESC,gate_id DESC)`,
}

func verifyFlowsV8(db *sql.DB) error {
	version, err := flowSchemaVersion(db)
	if err != nil || version < 8 {
		return err
	}
	canonical := func(s string) string { return strings.ToLower(strings.Join(strings.Fields(s), "")) }
	var table string
	if err := db.QueryRow(`SELECT sql FROM sqlite_master WHERE type='table' AND name='flow_durable_gates'`).Scan(&table); err != nil {
		return fmt.Errorf("verify durable approval table: %w", err)
	}
	if canonical(table) != canonical(durableApprovalGatesSchema) {
		return errors.New("invalid durable approval table")
	}
	for _, index := range durableApprovalGateIndexes {
		fields := strings.Fields(index)
		name := fields[2]
		if strings.EqualFold(fields[1], "unique") {
			name = fields[3]
		}
		var got string
		if err := db.QueryRow(`SELECT sql FROM sqlite_master WHERE type='index' AND name=?`, name).Scan(&got); err != nil {
			return fmt.Errorf("verify durable approval index %s: %w", name, err)
		}
		if canonical(got) != canonical(index) {
			return fmt.Errorf("invalid durable approval index %s", name)
		}
	}
	return nil
}

func migrateFlowsV8(db *sql.DB, logger *slog.Logger, existing bool) error {
	version, err := flowSchemaVersion(db)
	if err != nil {
		return err
	}
	if version >= 8 {
		return verifyFlowsV8(db)
	}
	if version != 7 {
		return errors.New("flows v8 migration requires schema v7")
	}
	if existing {
		if err := backupFlowsSnapshot(db, logger, ".flows-v7-backup-*"); err != nil {
			return err
		}
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(durableApprovalGatesSchema); err != nil {
		return fmt.Errorf("create durable approval table: %w", err)
	}
	for _, index := range durableApprovalGateIndexes {
		if _, err := tx.Exec(index); err != nil {
			return fmt.Errorf("create durable approval index: %w", err)
		}
	}
	result, err := tx.Exec(`UPDATE flows_meta SET value='8' WHERE key='schema_version' AND value='7'`)
	if err != nil {
		return fmt.Errorf("record flows v8 schema: %w", err)
	}
	if n, err := result.RowsAffected(); err != nil || n != 1 {
		return errors.New("record flows v8 schema: version changed")
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return verifyFlowsV8(db)
}
