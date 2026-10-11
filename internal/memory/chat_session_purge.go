package memory

import (
	"fmt"
	"slices"
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
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin chat session purge: %w", err)
	}
	defer tx.Rollback()
	for _, table := range []string{
		"messages", "archived_messages", "archive_events", "memory_usage_log",
		"compressed_tool_outputs", "activity_turns", "audit_events", "journal_entries", "episodic_memories", "memory_extraction_sources",
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
