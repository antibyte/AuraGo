package memory

import "fmt"

// PurgeChatSession permanently removes a privacy-sensitive transient session
// without first copying its messages into the consolidation archive.
func (s *SQLiteMemory) PurgeChatSession(sessionID string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin chat session purge: %w", err)
	}
	defer tx.Rollback()
	for _, table := range []string{
		"messages", "archived_messages", "archive_events", "memory_usage_log",
		"compressed_tool_outputs", "activity_turns", "audit_events", "journal_entries", "episodic_memories",
	} {
		var exists bool
		if err := tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE type = 'table' AND name = ?)`, table).Scan(&exists); err != nil {
			return fmt.Errorf("inspect transient session table %s: %w", table, err)
		}
		if !exists {
			continue
		}
		if _, err := tx.Exec(`DELETE FROM `+table+` WHERE session_id = ?`, sessionID); err != nil {
			return fmt.Errorf("purge transient session from %s: %w", table, err)
		}
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
