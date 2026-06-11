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

func ShouldHideAutonomousMessage(sessionID, role, content string) bool {
	if sessionID == "heartbeat" || sessionID == "space-agent-bridge" {
		return true
	}
	if role != "user" {
		return false
	}
	return strings.Contains(content, "[SYSTEM HEARTBEAT]") ||
		strings.Contains(content, "Space Agent sent this bridge question to AuraGo.")
}

// CreateChatSession creates a new chat session and returns its ID.
// It also runs rotation to ensure we don't exceed MaxChatSessions.
func (s *SQLiteMemory) CreateChatSession() (*ChatSession, error) {
	return s.CreateChatSessionWithLimit(MaxChatSessions)
}

// CreateChatSessionWithLimit creates a new chat session and rotates older
// sessions using the supplied retention limit.
func (s *SQLiteMemory) CreateChatSessionWithLimit(retainLimit int) (*ChatSession, error) {
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
	if err := s.RotateChatSessionsWithLimit(retainLimit); err != nil {
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
	return s.ListChatSessionsWithLimit(MaxChatSessions)
}

// ListChatSessionsWithLimit returns the most recent chat sessions (newest first).
func (s *SQLiteMemory) ListChatSessionsWithLimit(limit int) ([]ChatSession, error) {
	if limit <= 0 {
		limit = MaxChatSessions
	}
	rows, err := s.db.Query(
		`SELECT id, COALESCE(preview, ''), created_at, last_active_at, message_count
		 FROM chat_sessions
		 WHERE id != 'heartbeat'
		 ORDER BY last_active_at DESC
		 LIMIT ?`, limit,
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
	if sessionID == "heartbeat" {
		_, err := s.db.Exec(
			`UPDATE chat_sessions SET preview = '', message_count = 0 WHERE id = ?`,
			sessionID,
		)
		if err != nil {
			return fmt.Errorf("failed to update heartbeat chat session preview: %w", err)
		}
		return nil
	}

	// Get first non-internal user message for preview
	var content string
	err := s.db.QueryRow(
		`SELECT content FROM messages
		 WHERE session_id = ? AND role = 'user' AND is_internal = 0
		 AND content NOT LIKE '%[SYSTEM HEARTBEAT]%'
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
		`SELECT COUNT(*) FROM messages
		 WHERE session_id = ? AND is_internal = 0
		 AND NOT (role = 'user' AND content LIKE '%[SYSTEM HEARTBEAT]%')`, sessionID,
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
	return s.RotateChatSessionsWithLimit(MaxChatSessions)
}

// RotateChatSessionsWithLimit deletes the oldest sessions that exceed limit.
// It keeps the most recently active sessions and archives user/assistant messages
// in the same transaction before deleting each rotated session.
func (s *SQLiteMemory) RotateChatSessionsWithLimit(limit int) error {
	if limit <= 0 {
		return fmt.Errorf("chat session rotation limit must be positive, got %d", limit)
	}
	// Find sessions to delete (all except the newest MaxChatSessions)
	rows, err := s.db.Query(
		`SELECT id FROM chat_sessions
		 WHERE id != 'heartbeat'
		 ORDER BY last_active_at DESC
		 LIMIT -1 OFFSET ?`, limit,
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
		if err := s.archiveAndDeleteChatSession(id); err != nil {
			s.logger.Warn("Failed to rotate chat session", "session_id", id, "error", err)
			return err
		}
		s.logger.Info("Rotated old chat session", "session_id", id)
	}
	return nil
}

// GetSessionMessages returns all visible (non-internal) messages for a session,
// ordered chronologically.
func (s *SQLiteMemory) GetSessionMessages(sessionID string) ([]HistoryMessage, error) {
	return s.querySessionMessages(sessionID, false)
}

// GetSessionMessagesForBridge returns session messages with SQLite IDs for agent
// context rebuilds (e.g. MCP debug bridge). Internal tool messages are included
// so debugging turns retain tool-call context.
func (s *SQLiteMemory) GetSessionMessagesForBridge(sessionID string) ([]HistoryMessage, error) {
	return s.querySessionMessages(sessionID, true)
}

func (s *SQLiteMemory) querySessionMessages(sessionID string, includeInternal bool) ([]HistoryMessage, error) {
	query := `SELECT id, role, content, is_pinned, is_internal, timestamp FROM messages
		 WHERE session_id = ?`
	if !includeInternal {
		query += ` AND is_internal = 0`
	}
	query += ` ORDER BY timestamp ASC, id ASC`

	rows, err := s.db.Query(query, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session messages: %w", err)
	}
	defer rows.Close()

	var messages []HistoryMessage
	for rows.Next() {
		var msg HistoryMessage
		var pinned, internal bool
		if err := rows.Scan(&msg.ID, &msg.Role, &msg.Content, &pinned, &internal, &msg.Timestamp); err != nil {
			return nil, fmt.Errorf("failed to scan session message: %w", err)
		}
		msg.Pinned = pinned
		msg.IsInternal = internal
		msg.Timestamp = sqliteDatetimeToRFC3339(msg.Timestamp)
		if ShouldHideAutonomousMessage(sessionID, msg.Role, msg.Content) {
			continue
		}
		messages = append(messages, msg)
	}
	return messages, rows.Err()
}

// ClearSession archives consolidatable messages, removes all session messages, and keeps session metadata.
func (s *SQLiteMemory) ClearSession(sessionID string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin clear session transaction: %w", err)
	}
	defer tx.Rollback()

	archived, err := s.archiveMessagesInTx(tx, sessionID, "")
	if err != nil {
		return err
	}
	if _, err := s.deleteMessagesInTx(tx, sessionID, ""); err != nil {
		return err
	}
	if _, err := tx.Exec(
		`UPDATE chat_sessions SET preview = '', message_count = 0 WHERE id = ?`, sessionID,
	); err != nil {
		return fmt.Errorf("failed to reset session metadata: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit clear session transaction: %w", err)
	}
	s.logger.Info("Cleared session messages", "session_id", sessionID, "archived", archived)
	return nil
}

// DeleteChatSession removes a session and all its messages.
func (s *SQLiteMemory) DeleteChatSession(sessionID string) error {
	if err := s.archiveAndDeleteChatSession(sessionID); err != nil {
		return err
	}
	s.logger.Info("Deleted chat session", "session_id", sessionID)
	return nil
}

func (s *SQLiteMemory) archiveAndDeleteChatSession(sessionID string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin chat session delete transaction: %w", err)
	}
	defer tx.Rollback()

	if _, err := s.archiveMessagesInTx(tx, sessionID, ""); err != nil {
		return err
	}
	if _, err := s.deleteMessagesInTx(tx, sessionID, ""); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM chat_sessions WHERE id = ?`, sessionID); err != nil {
		return fmt.Errorf("failed to delete chat session: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit chat session delete transaction: %w", err)
	}
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
