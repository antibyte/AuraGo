package flows

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
)

func verifyFlowsV3(db *sql.DB) error {
	version, err := flowSchemaVersion(db)
	if err != nil || version < 3 {
		return err
	}
	for _, item := range []struct{ kind, name, definition string }{
		{"table", "flow_block_version_pins", blockVersionPinsSchema},
		{"table", "flow_block_run_pins", blockRunPinsSchema},
		{"index", "idx_block_version_pins_child", blockVersionPinsIndex},
		{"index", "idx_block_run_pins_child", blockRunPinsIndex},
	} {
		var definition string
		if err := db.QueryRow(`SELECT sql FROM sqlite_master WHERE type=? AND name=?`, item.kind, item.name).Scan(&definition); err != nil {
			return fmt.Errorf("verify block pin schema: %w", err)
		}
		canonical := func(s string) string { return strings.ToLower(strings.Join(strings.Fields(s), "")) }
		if canonical(definition) != canonical(item.definition) {
			return errors.New("invalid block pin schema")
		}
	}
	return nil
}

func migrateFlowsV3(db *sql.DB, logger *slog.Logger, existing bool) error {
	version, err := flowSchemaVersion(db)
	if err != nil {
		return fmt.Errorf("read flows schema: %w", err)
	}
	if version >= 3 {
		return verifyFlowsV3(db)
	}
	if version != 2 {
		return errors.New("flows v3 migration requires schema v2")
	}
	if existing {
		if err := backupFlowsSnapshot(db, logger, ".flows-v2-backup-*"); err != nil {
			return err
		}
	}
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin flows v3 migration: %w", err)
	}
	defer tx.Rollback()
	for _, statement := range []string{blockVersionPinsSchema, blockRunPinsSchema, blockVersionPinsIndex, blockRunPinsIndex} {
		if _, err := tx.Exec(statement); err != nil {
			return fmt.Errorf("migrate flows v3: %w", err)
		}
	}
	result, err := tx.Exec(`UPDATE flows_meta SET value='3' WHERE key='schema_version' AND value='2'`)
	if err != nil {
		return fmt.Errorf("record flows v3 schema: %w", err)
	}
	if n, err := result.RowsAffected(); err != nil || n != 1 {
		return errors.New("record flows v3 schema: version changed")
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit flows v3 schema: %w", err)
	}
	return nil
}
