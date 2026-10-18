package memory

import (
	"crypto/sha256"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"
)

// TransientSessionIDs lists the sessions whose ID starts with prefix and that
// still hold messages, archived messages, activity turns or session metadata.
// Owners of transient sessions (for example CYD Creator's cyd-creator-<job>)
// use it at startup to purge sessions a crash left behind before their
// deferred PurgeChatSession ran.
func (s *SQLiteMemory) TransientSessionIDs(prefix string) ([]string, error) {
	if prefix == "" {
		return nil, fmt.Errorf("a transient session prefix is required")
	}
	n := utf8.RuneCountInString(prefix)
	seen := map[string]bool{}
	for _, source := range []struct{ table, column string }{
		{"messages", "session_id"}, {"archived_messages", "session_id"}, {"activity_turns", "session_id"}, {"chat_sessions", "id"},
	} {
		var exists bool
		if err := s.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE type = 'table' AND name = ?)`, source.table).Scan(&exists); err != nil {
			return nil, fmt.Errorf("inspect transient session table %s: %w", source.table, err)
		}
		if !exists {
			continue
		}
		rows, err := s.db.Query(`SELECT DISTINCT `+source.column+` FROM `+source.table+` WHERE substr(`+source.column+`, 1, ?) = ?`, n, prefix)
		if err != nil {
			return nil, fmt.Errorf("list transient sessions in %s: %w", source.table, err)
		}
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return nil, fmt.Errorf("list transient sessions in %s: %w", source.table, err)
			}
			seen[id] = true
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, fmt.Errorf("list transient sessions in %s: %w", source.table, err)
		}
	}
	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return ids, nil
}

// PurgeChatSession permanently removes a privacy-sensitive transient session
// without first copying its messages into the consolidation archive.
func (s *SQLiteMemory) PurgeChatSession(sessionID string) error {
	s.cancelSessionRuns(sessionID)
	defer s.finishSessionDeletion(sessionID)
	s.sessionWriteMu.Lock()
	defer s.sessionWriteMu.Unlock()
	s.automaticMemoryMu.Lock()
	defer s.automaticMemoryMu.Unlock()
	return s.purgeChatSessionLocked(sessionID)
}

func (s *SQLiteMemory) purgeChatSessionLocked(sessionID string) error {
	if strings.TrimSpace(sessionID) == "" {
		return fmt.Errorf("a session ID is required")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin chat session purge: %w", err)
	}
	defer tx.Rollback()
	// Keep only the opaque identity, never deleted requests or transcript content.
	if _, err := tx.Exec(`CREATE TABLE IF NOT EXISTS deleted_chat_sessions (session_id TEXT PRIMARY KEY, deleted_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP)`); err != nil {
		return fmt.Errorf("create chat deletion fence: %w", err)
	}
	if _, err := tx.Exec(`INSERT OR IGNORE INTO deleted_chat_sessions(session_id) VALUES (?)`, sessionID); err != nil {
		return fmt.Errorf("record chat deletion fence: %w", err)
	}
	var pendingMetadata bool
	if err := tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM pragma_table_info('pending_memory_writes') WHERE name='metadata_json')`).Scan(&pendingMetadata); err != nil {
		return fmt.Errorf("inspect queued memory provenance: %w", err)
	}
	if pendingMetadata {
		if err := normalizeLegacyPendingMemoryWritesTx(tx, sessionID); err != nil {
			return err
		}
		if _, err := tx.Exec(`DELETE FROM pending_memory_writes WHERE CASE WHEN json_valid(metadata_json) THEN json_extract(metadata_json,'$.version')=1 AND json_extract(metadata_json,'$.details.SourceSessionID')=? ELSE 0 END`, sessionID); err != nil {
			return fmt.Errorf("purge queued session memory: %w", err)
		}
		for _, operation := range []string{"INSERT", "UPDATE"} {
			if _, err := tx.Exec(`CREATE TRIGGER IF NOT EXISTS deleted_session_pending_memory_` + operation + ` BEFORE ` + operation + ` ON pending_memory_writes WHEN EXISTS(SELECT 1 FROM deleted_chat_sessions WHERE session_id=CASE WHEN json_valid(NEW.metadata_json) THEN json_extract(NEW.metadata_json,'$.details.SourceSessionID') ELSE NULL END) BEGIN SELECT RAISE(ABORT,'chat session deleted'); END`); err != nil {
				return fmt.Errorf("fence queued session memory: %w", err)
			}
		}
	}
	// An anonymous retained observation prevents shared memories becoming falsely exclusive.
	digest := fmt.Sprintf("deleted:%x", sha256.Sum256([]byte(sessionID)))
	if _, err := tx.Exec(`INSERT OR IGNORE INTO memory_extraction_sources(doc_id,source_type,session_id,first_seen_at,last_seen_at)
	 SELECT doc_id,'deleted_source',?,first_seen_at,last_seen_at FROM memory_extraction_sources WHERE session_id=?`, digest, sessionID); err != nil {
		return fmt.Errorf("anonymize retained memory provenance: %w", err)
	}
	for _, table := range []string{"plan_tasks", "plan_events"} {
		var exists bool
		if err := tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE type='table' AND name=?)`, table).Scan(&exists); err != nil {
			return fmt.Errorf("inspect session plans: %w", err)
		}
		if exists {
			for _, operation := range []string{"INSERT", "UPDATE"} {
				if _, err := tx.Exec(`CREATE TRIGGER IF NOT EXISTS deleted_session_` + table + `_` + operation + ` BEFORE ` + operation + ` ON ` + table + ` WHEN NOT EXISTS(SELECT 1 FROM plans WHERE id=NEW.plan_id) BEGIN SELECT RAISE(ABORT,'plan no longer exists'); END`); err != nil {
					return fmt.Errorf("fence orphan plan writes: %w", err)
				}
			}
			if _, err := tx.Exec(`DELETE FROM `+table+` WHERE plan_id IN (SELECT id FROM plans WHERE session_id=?)`, sessionID); err != nil {
				return fmt.Errorf("purge session %s: %w", table, err)
			}
		}
	}
	for _, table := range []string{
		"messages", "archived_messages", "archive_events", "memory_usage_log",
		"compressed_tool_outputs", "activity_turns", "audit_events", "journal_entries", "episodic_memories", "memory_extraction_sources", "plans",
	} {
		var exists bool
		if err := tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE type = 'table' AND name = ?)`, table).Scan(&exists); err != nil {
			return fmt.Errorf("inspect transient session table %s: %w", table, err)
		}
		if !exists {
			continue
		}
		for _, operation := range []string{"INSERT", "UPDATE"} {
			if _, err := tx.Exec(`CREATE TRIGGER IF NOT EXISTS deleted_session_` + table + `_` + operation + ` BEFORE ` + operation + ` ON ` + table + ` WHEN EXISTS(SELECT 1 FROM deleted_chat_sessions WHERE session_id=NEW.session_id) BEGIN SELECT RAISE(ABORT, 'chat session deleted'); END`); err != nil {
				return fmt.Errorf("fence deleted session writes to %s: %w", table, err)
			}
		}
		if _, err := tx.Exec(`DELETE FROM `+table+` WHERE session_id = ?`, sessionID); err != nil {
			return fmt.Errorf("purge transient session from %s: %w", table, err)
		}
	}
	if _, err := tx.Exec(`CREATE TRIGGER IF NOT EXISTS deleted_session_chat_sessions BEFORE INSERT ON chat_sessions WHEN EXISTS(SELECT 1 FROM deleted_chat_sessions WHERE session_id=NEW.id) BEGIN SELECT RAISE(ABORT, 'chat session deleted'); END`); err != nil {
		return fmt.Errorf("fence deleted session metadata: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM chat_sessions WHERE id = ?`, sessionID); err != nil {
		return fmt.Errorf("purge chat session metadata: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit chat session purge: %w", err)
	}
	s.logger.Info("Purged transient chat session", "session_id", sessionID)
	return nil
}

// IsSessionDeleted rejects delayed work after a permanent privacy deletion.
func (s *SQLiteMemory) IsSessionDeleted(sessionID string) (bool, error) {
	var exists bool
	if err := s.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE type='table' AND name='deleted_chat_sessions')`).Scan(&exists); err != nil || !exists {
		return false, err
	}
	var deleted bool
	err := s.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM deleted_chat_sessions WHERE session_id=?)`, sessionID).Scan(&deleted)
	return deleted, err
}
