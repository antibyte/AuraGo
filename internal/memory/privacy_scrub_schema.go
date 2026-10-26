package memory

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

var ErrChatSessionDeleted = errors.New("chat session has been deleted")

type privacySessionQueryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func rejectDeletedChatSessionEvidence(ctx context.Context, queryer privacySessionQueryer, sessionID string) error {
	if sessionID == "" {
		return nil
	}
	var deleted bool
	if err := queryer.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM deleted_chat_sessions WHERE session_id=?)`, sessionID).Scan(&deleted); err != nil {
		return fmt.Errorf("check deleted chat session fence: %w", err)
	}
	if deleted {
		return ErrChatSessionDeleted
	}
	return nil
}

// migratePrivacyScrub installs database-level fences for session evidence that
// can arrive through learning and correction payloads after an accepted delete.
// It runs inside the versioned schema transaction owned by the memory store.
func migratePrivacyScrub(ctx context.Context, tx *sql.Tx) error {
	if tx == nil {
		return fmt.Errorf("privacy scrub migration requires a transaction")
	}
	if _, err := tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS deleted_chat_sessions (
		session_id TEXT PRIMARY KEY,
		deleted_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
		privacy_job_id TEXT NOT NULL DEFAULT ''
	)`); err != nil {
		return fmt.Errorf("create deleted session fence: %w", err)
	}
	var hasJobID bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM pragma_table_info('deleted_chat_sessions') WHERE name='privacy_job_id')`).Scan(&hasJobID); err != nil {
		return fmt.Errorf("inspect deleted session fence: %w", err)
	}
	if !hasJobID {
		if _, err := tx.ExecContext(ctx, `ALTER TABLE deleted_chat_sessions ADD COLUMN privacy_job_id TEXT NOT NULL DEFAULT ''`); err != nil {
			return fmt.Errorf("add deleted session privacy job reference: %w", err)
		}
	}

	for _, table := range []string{"memory_learnings", "memory_learning_revisions", "memory_corrections"} {
		var exists bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE type='table' AND name=?)`, table).Scan(&exists); err != nil {
			return fmt.Errorf("inspect %s privacy fence: %w", table, err)
		}
		if !exists {
			continue
		}
		var payloadFence string
		switch table {
		case "memory_learnings":
			payloadFence = `EXISTS(
				SELECT 1 FROM deleted_chat_sessions d, json_tree(CASE WHEN json_valid(NEW.payload) THEN NEW.payload ELSE '{}' END) p
				WHERE p.key IN ('session','session_id') AND p.value=d.session_id
			)`
		case "memory_learning_revisions":
			payloadFence = `EXISTS(
				SELECT 1 FROM deleted_chat_sessions d, json_tree(CASE WHEN json_valid(NEW.payload) THEN NEW.payload ELSE '{}' END) p
				WHERE p.key IN ('session','session_id') AND p.value=d.session_id
			)`
		case "memory_corrections":
			payloadFence = `EXISTS(
				SELECT 1 FROM deleted_chat_sessions d, json_tree(CASE WHEN json_valid(NEW.payload) THEN NEW.payload ELSE '{}' END) p
				WHERE p.key IN ('session','session_id') AND p.value=d.session_id
			)`
		}
		for _, operation := range []string{"INSERT", "UPDATE"} {
			name := "deleted_session_" + table + "_" + operation
			condition := `NOT json_valid(NEW.payload) OR ` + payloadFence
			if table == "memory_corrections" {
				condition = `NOT json_valid(NEW.payload) OR EXISTS(SELECT 1 FROM deleted_chat_sessions WHERE session_id=NEW.session_id) OR ` + payloadFence
			}
			if _, err := tx.ExecContext(ctx, `CREATE TRIGGER IF NOT EXISTS `+name+` BEFORE `+operation+` ON `+table+`
				WHEN `+condition+`
				BEGIN SELECT RAISE(ABORT,'chat session deleted'); END`); err != nil {
				return fmt.Errorf("create %s privacy fence: %w", table, err)
			}
		}
	}
	return nil
}
