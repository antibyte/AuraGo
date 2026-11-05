package flows

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
)

// V5 is a compatibility marker: older binaries must not rewrite groups away.
func migrateFlowsV5(db *sql.DB, logger *slog.Logger, existing bool) error {
	version, err := flowSchemaVersion(db)
	if err != nil {
		return err
	}
	if version >= 5 {
		return nil
	}
	if version != 4 {
		return errors.New("flows v5 migration requires schema v4")
	}
	if existing {
		if err := backupFlowsSnapshot(db, logger, ".flows-v4-backup-*"); err != nil {
			return err
		}
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.Exec(`UPDATE flows_meta SET value='5' WHERE key='schema_version' AND value='4'`)
	if err != nil {
		return fmt.Errorf("record flows v5 schema: %w", err)
	}
	if n, err := result.RowsAffected(); err != nil || n != 1 {
		return errors.New("record flows v5 schema: version changed")
	}
	return tx.Commit()
}
