package memory

import (
	"context"
	"database/sql"
	"fmt"
)

// migrateMaintenanceProgress adds the durable, source-bound maintenance state.
// The caller owns the surrounding versioned transaction so the schema and its
// migration version commit atomically.
func migrateMaintenanceProgress(ctx context.Context, tx *sql.Tx) error {
	if tx == nil {
		return fmt.Errorf("maintenance progress migration transaction is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	statements := []string{
		`CREATE TABLE IF NOT EXISTS memory_consolidation_checkpoints (
			message_id INTEGER NOT NULL,
			source_hash TEXT NOT NULL,
			prompt_version TEXT NOT NULL,
			model_id TEXT NOT NULL,
			part_count INTEGER NOT NULL CHECK (part_count >= 0),
			state TEXT NOT NULL DEFAULT 'extracting' CHECK (state IN ('extracting','ready','publishing')),
			publish_cursor INTEGER NOT NULL DEFAULT 0 CHECK (publish_cursor >= 0),
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (message_id, source_hash, prompt_version, model_id)
		)`,
		`CREATE TABLE IF NOT EXISTS memory_consolidation_parts (
			message_id INTEGER NOT NULL,
			source_hash TEXT NOT NULL,
			prompt_version TEXT NOT NULL,
			model_id TEXT NOT NULL,
			part_index INTEGER NOT NULL CHECK (part_index >= 0),
			facts_json TEXT NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (message_id, source_hash, prompt_version, model_id, part_index),
			FOREIGN KEY (message_id, source_hash, prompt_version, model_id)
				REFERENCES memory_consolidation_checkpoints(message_id, source_hash, prompt_version, model_id)
				ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_memory_consolidation_checkpoints_state
			ON memory_consolidation_checkpoints(state, updated_at, message_id)`,
		`CREATE TRIGGER IF NOT EXISTS trg_memory_consolidation_checkpoint_insert_fence
			BEFORE INSERT ON memory_consolidation_checkpoints
			WHEN NOT EXISTS (
				SELECT 1 FROM archived_messages
				WHERE id=NEW.message_id AND consolidated=0 AND consolidation_status='in_progress'
				  AND NOT EXISTS(SELECT 1 FROM deleted_chat_sessions d WHERE d.session_id=archived_messages.session_id)
			)
			BEGIN SELECT RAISE(ABORT, 'consolidation source is not actively claimed'); END`,
		`CREATE TRIGGER IF NOT EXISTS trg_memory_consolidation_checkpoint_update_fence
			BEFORE UPDATE ON memory_consolidation_checkpoints
			WHEN NOT EXISTS (
				SELECT 1 FROM archived_messages
				WHERE id=NEW.message_id AND consolidated=0 AND consolidation_status='in_progress'
				  AND NOT EXISTS(SELECT 1 FROM deleted_chat_sessions d WHERE d.session_id=archived_messages.session_id)
			)
			BEGIN SELECT RAISE(ABORT, 'consolidation source is not actively claimed'); END`,
		`CREATE TRIGGER IF NOT EXISTS trg_memory_consolidation_part_insert_fence
			BEFORE INSERT ON memory_consolidation_parts
			WHEN NOT EXISTS (
				SELECT 1 FROM memory_consolidation_checkpoints c JOIN archived_messages a ON a.id=c.message_id
				WHERE c.message_id=NEW.message_id AND c.source_hash=NEW.source_hash
				  AND c.prompt_version=NEW.prompt_version AND c.model_id=NEW.model_id
				  AND a.consolidated=0 AND a.consolidation_status='in_progress'
				  AND NOT EXISTS(SELECT 1 FROM deleted_chat_sessions d WHERE d.session_id=a.session_id)
			)
			BEGIN SELECT RAISE(ABORT, 'consolidation checkpoint source is not actively claimed'); END`,
		`CREATE TRIGGER IF NOT EXISTS trg_memory_consolidation_part_update_fence
			BEFORE UPDATE ON memory_consolidation_parts
			WHEN NOT EXISTS (
				SELECT 1 FROM memory_consolidation_checkpoints c JOIN archived_messages a ON a.id=c.message_id
				WHERE c.message_id=NEW.message_id AND c.source_hash=NEW.source_hash
				  AND c.prompt_version=NEW.prompt_version AND c.model_id=NEW.model_id
				  AND a.consolidated=0 AND a.consolidation_status='in_progress'
				  AND NOT EXISTS(SELECT 1 FROM deleted_chat_sessions d WHERE d.session_id=a.session_id)
			)
			BEGIN SELECT RAISE(ABORT, 'consolidation checkpoint source is not actively claimed'); END`,
		`CREATE TABLE IF NOT EXISTS pending_memory_write_tombstones (
			fingerprint TEXT PRIMARY KEY,
			source_session_hash TEXT NOT NULL DEFAULT '',
			terminal_at DATETIME NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_pending_memory_write_tombstones_session
			ON pending_memory_write_tombstones(source_session_hash, terminal_at)`,
		`CREATE TABLE IF NOT EXISTS memory_maintenance_retry_cursors (
			action TEXT PRIMARY KEY,
			target_id TEXT NOT NULL DEFAULT '',
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_messages_session_id_id ON messages(session_id, id)`,
		`CREATE INDEX IF NOT EXISTS idx_archived_messages_consolidation_claim
			ON archived_messages(consolidation_status, consolidation_claimed_at, id)`,
	}
	for _, statement := range statements {
		if err := ctx.Err(); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("create maintenance progress schema: %w", err)
		}
	}
	for _, column := range []struct {
		name    string
		typeSQL string
	}{
		{name: "source_fingerprint", typeSQL: "TEXT NOT NULL DEFAULT ''"},
		{name: "source_session_hash", typeSQL: "TEXT NOT NULL DEFAULT ''"},
		{name: "terminal_at", typeSQL: "DATETIME"},
	} {
		if err := ctx.Err(); err != nil {
			return err
		}
		var exists int
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM pragma_table_info('pending_memory_writes') WHERE name=?)`, column.name).Scan(&exists); err != nil {
			return fmt.Errorf("inspect pending memory %s migration: %w", column.name, err)
		}
		if exists != 0 {
			continue
		}
		if _, err := tx.ExecContext(ctx, `ALTER TABLE pending_memory_writes ADD COLUMN `+column.name+` `+column.typeSQL); err != nil {
			return fmt.Errorf("add pending memory %s column: %w", column.name, err)
		}
	}
	if _, err := tx.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_pending_memory_writes_exhausted ON pending_memory_writes(status, terminal_at, id)`); err != nil {
		return fmt.Errorf("create exhausted pending memory write index: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE pending_memory_writes
		SET terminal_at=COALESCE(terminal_at, updated_at)
		WHERE status='exhausted' AND terminal_at IS NULL`); err != nil {
		return fmt.Errorf("backfill pending memory terminal timestamps: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		CREATE TRIGGER IF NOT EXISTS trg_archived_message_delete_maintenance_progress
		AFTER DELETE ON archived_messages
		BEGIN
			DELETE FROM memory_consolidation_checkpoints WHERE message_id=OLD.id;
			DELETE FROM memory_consolidation_parts WHERE message_id=OLD.id;
		END`); err != nil {
		return fmt.Errorf("create archive maintenance progress cleanup trigger: %w", err)
	}
	return nil
}

func maintenanceProgressTableExists(ctx context.Context, tx *sql.Tx, table string) (bool, error) {
	var exists bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE type='table' AND name=?)`, table).Scan(&exists); err != nil {
		return false, fmt.Errorf("inspect maintenance schema table %s: %w", table, err)
	}
	return exists, nil
}
