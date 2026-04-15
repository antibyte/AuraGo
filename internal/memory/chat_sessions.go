package memory

import (
	"aurago/internal/uid"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// ChatSession represents a persisted chat session with metadata.
type ChatSession struct {
	ID           string `json:"id"`
	Preview      string `json:"preview"`
	CreatedAt    string `json:"created_at"`
	LastActiveAt string `json:"last_active_at"`
	MessageCount int    `json:"message_count"`
}

// MaxChatSessions is the maximum number of chat sessions retained.
const MaxChatSessions = 10

// sqliteDatetimeToRFC3339 converts a SQLite datetime string
// ("2006-01-02 15:04:05") to RFC 3339 ("2006-01-02T15:04:05Z").
// If parsing fails, the original string is returned unchanged.
func sqliteDatetimeToRFC3339(dt string) string {
	if dt == "" {
		return ""
	}
	// Already RFC3339?
	if len(dt) > 0 && (dt[len(dt)-1] == 'Z' || strings.Contains(dt, "T")) {
		return dt
	}
	t, err := time.Parse("2006-01-02 15:04:05", dt)
	if err != nil {
		return dt
	}
	return t.UTC().Format(time.RFC3339)
}

// CreateChatSession creates a new chat session and returns its ID.
// It also runs rotation to ensure we don't exceed MaxChatSessions.
func (s *SQLiteMemory) CreateChatSession() (*ChatSession, error) {
	id := "sess-" + uid.New()
	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	_, err := s.db.Exec(
		`INSERT INTO chat_sessions (id, preview, created_at, last_active_at, message_count) VALUES (?, '', ?, ?, 0)`,
		id, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create chat session: %w", err)
	}
	s.logger.Info("Created new chat session", "id", id)

	// Rotate old sessions
	if err := s.RotateChatSessions(); err != nil {
		s.logger.Warn("Failed to rotate chat sessions", "error", err)
	}

	return &ChatSession{
		ID:           id,
		Preview:      "",
		CreatedAt:    now,
		LastActiveAt: now,
		MessageCount: 0,
	}, nil
}

// ListChatSessions returns the most recent chat sessions (newest first), up to MaxChatSessions.
func (s *SQLiteMemory) ListChatSessions() ([]ChatSession, error) {
	rows, err := s.db.Query(
		`SELECT id, COALESCE(preview, ''), created_at, last_active_at, message_count
		 FROM chat_sessions
		 ORDER BY last_active_at DESC
		 LIMIT ?`, MaxChatSessions,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list chat sessions: %w", err)
	}
	defer rows.Close()

	var sessions []ChatSession
	for rows.Next() {
		var sess ChatSession
		if err := rows.Scan(&sess.ID, &sess.Preview, &sess.CreatedAt, &sess.LastActiveAt, &sess.MessageCount); err != nil {
			return nil, fmt.Errorf("failed to scan chat session: %w", err)
		}
		sess.CreatedAt = sqliteDatetimeToRFC3339(sess.CreatedAt)
		sess.LastActiveAt = sqliteDatetimeToRFC3339(sess.LastActiveAt)
		sessions = append(sessions, sess)
	}
	return sessions, rows.Err()
}

// GetChatSession returns a single chat session by ID.
func (s *SQLiteMemory) GetChatSession(id string) (*ChatSession, error) {
	var sess ChatSession
	err := s.db.QueryRow(
		`SELECT id, COALESCE(preview, ''), created_at, last_active_at, message_count
		 FROM chat_sessions WHERE id = ?`, id,
	).Scan(&sess.ID, &sess.Preview, &sess.CreatedAt, &sess.LastActiveAt, &sess.MessageCount)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get chat session: %w", err)
	}
	sess.CreatedAt = sqliteDatetimeToRFC3339(sess.CreatedAt)
	sess.LastActiveAt = sqliteDatetimeToRFC3339(sess.LastActiveAt)
	return &sess, nil
}

// UpdateChatSessionPreview updates the preview text and message count for a session.
// The preview is derived from the first non-empty user message in the session.
func (s *SQLiteMemory) UpdateChatSessionPreview(sessionID string) error {
	// Get first non-internal user message for preview
	var content string
	err := s.db.QueryRow(
		`SELECT content FROM messages
		 WHERE session_id = ? AND role = 'user' AND is_internal = 0
		 ORDER BY timestamp ASC, id ASC LIMIT 1`, sessionID,
	).Scan(&content)
	if err != nil {
		if err == sql.ErrNoRows {
			// No user messages yet, try assistant
			_ = s.db.QueryRow(
				`SELECT content FROM messages
				 WHERE session_id = ? AND role = 'assistant' AND is_internal = 0
				 ORDER BY timestamp ASC, id ASC LIMIT 1`, sessionID,
			).Scan(&content)
		}
	}

	// Truncate preview to first line, max 80 chars
	preview := ""
	if content != "" {
		lines := strings.SplitN(content, "\n", 2)
		preview = strings.TrimSpace(lines[0])
		if len(preview) > 80 {
			preview = preview[:77] + "..."
		}
	}

	// Count visible messages
	var count int
	_ = s.db.QueryRow(
		`SELECT COUNT(*) FROM messages WHERE session_id = ? AND is_internal = 0`, sessionID,
	).Scan(&count)

	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	_, err = s.db.Exec(
		`UPDATE chat_sessions SET preview = ?, message_count = ?, last_active_at = ? WHERE id = ?`,
		preview, count, now, sessionID,
	)
	if err != nil {
		return fmt.Errorf("failed to update chat session preview: %w", err)
	}
	return nil
}

// TouchChatSession updates the last_active_at timestamp for a session.
func (s *SQLiteMemory) TouchChatSession(sessionID string) error {
	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	_, err := s.db.Exec(
		`UPDATE chat_sessions SET last_active_at = ? WHERE id = ?`, now, sessionID,
	)
	if err != nil {
		return fmt.Errorf("failed to touch chat session: %w", err)
	}
	return nil
}

// RotateChatSessions deletes the oldest sessions that exceed MaxChatSessions.
// It keeps the most recently active sessions.
func (s *SQLiteMemory) RotateChatSessions() error {
	// Find sessions to delete (all except the newest MaxChatSessions)
	rows, err := s.db.Query(
		`SELECT id FROM chat_sessions
		 ORDER BY last_active_at DESC
		 LIMIT -1 OFFSET ?`, MaxChatSessions,
	)
	if err != nil {
		return fmt.Errorf("failed to find sessions to rotate: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return fmt.Errorf("failed to scan session id for rotation: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	if len(ids) == 0 {
		return nil
	}

	for _, id := range ids {
		// Delete messages for this session
		if _, err := s.db.Exec(`DELETE FROM messages WHERE session_id = ?`, id); err != nil {
			s.logger.Warn("Failed to delete messages for rotated session", "session_id", id, "error", err)
		}
		// Delete the session itself
		if _, err := s.db.Exec(`DELETE FROM chat_sessions WHERE id = ?`, id); err != nil {
			s.logger.Warn("Failed to delete rotated session", "session_id", id, "error", err)
		}
		s.logger.Info("Rotated old chat session", "session_id", id)
	}
	return nil
}

// GetSessionMessages returns all visible (non-internal) messages for a session,
// ordered chronologically.
func (s *SQLiteMemory) GetSessionMessages(sessionID string) ([]HistoryMessage, error) {
	rows, err := s.db.Query(
		`SELECT id, role, content, is_pinned, is_internal FROM messages
		 WHERE session_id = ?
		 ORDER BY timestamp ASC, id ASC`, sessionID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get session messages: %w", err)
	}
	defer rows.Close()

	var messages []HistoryMessage
	for rows.Next() {
		var msg HistoryMessage
		var pinned, internal bool
		if err := rows.Scan(&msg.ID, &msg.Role, &msg.Content, &pinned, &internal); err != nil {
			return nil, fmt.Errorf("failed to scan session message: %w", err)
		}
		msg.Pinned = pinned
		msg.IsInternal = internal
		messages = append(messages, msg)
	}
	return messages, rows.Err()
}

// ClearSession removes all messages for a specific session but keeps the session metadata.
func (s *SQLiteMemory) ClearSession(sessionID string) error {
	_, err := s.db.Exec(`DELETE FROM messages WHERE session_id = ?`, sessionID)
	if err != nil {
		return fmt.Errorf("failed to clear session messages: %w", err)
	}
	// Reset preview and count
	_, err = s.db.Exec(
		`UPDATE chat_sessions SET preview = '', message_count = 0 WHERE id = ?`, sessionID,
	)
	if err != nil {
		return fmt.Errorf("failed to reset session metadata: %w", err)
	}
	s.logger.Info("Cleared session messages", "session_id", sessionID)
	return nil
}

// DeleteChatSession removes a session and all its messages.
func (s *SQLiteMemory) DeleteChatSession(sessionID string) error {
	if _, err := s.db.Exec(`DELETE FROM messages WHERE session_id = ?`, sessionID); err != nil {
		return fmt.Errorf("failed to delete session messages: %w", err)
	}
	if _, err := s.db.Exec(`DELETE FROM chat_sessions WHERE id = ?`, sessionID); err != nil {
		return fmt.Errorf("failed to delete chat session: %w", err)
	}
	s.logger.Info("Deleted chat session", "session_id", sessionID)
	return nil
}

// EnsureDefaultSession creates a "default" chat session if no sessions exist yet.
// This is called during startup for backward compatibility with existing data.
func (s *SQLiteMemory) EnsureDefaultSession() error {
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM chat_sessions`).Scan(&count); err != nil {
		return fmt.Errorf("failed to count chat sessions: %w", err)
	}
	if count > 0 {
		return nil
	}

	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	_, err := s.db.Exec(
		`INSERT INTO chat_sessions (id, preview, created_at, last_active_at, message_count) VALUES (?, '', ?, ?, 0)`,
		"default", now, now,
	)
	if err != nil {
		return fmt.Errorf("failed to create default chat session: %w", err)
	}
	s.logger.Info("Created default chat session for backward compatibility")
	return nil
}
