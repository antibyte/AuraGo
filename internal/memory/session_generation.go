package memory

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"strings"
)

// ResolveUserSession advances a deleted fixed channel only for a fresh human input.
// Background work must retain its captured session ID and never call this method.
func (s *SQLiteMemory) ResolveUserSession(ctx context.Context, baseID string) (string, error) {
	if strings.TrimSpace(baseID) == "" {
		baseID = "default"
	}
	if err := lockMemoryMutex(ctx, &s.sessionWriteMu); err != nil {
		return "", err
	}
	defer s.sessionWriteMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("begin incoming session resolution: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS chat_session_generations(base_hash TEXT PRIMARY KEY,session_id TEXT NOT NULL)`); err != nil {
		return "", fmt.Errorf("create incoming session generations: %w", err)
	}
	key := fmt.Sprintf("%x", sha256.Sum256([]byte(baseID)))
	sessionID := baseID
	if err := tx.QueryRowContext(ctx, `SELECT session_id FROM chat_session_generations WHERE base_hash=?`, key).Scan(&sessionID); err != nil && err != sql.ErrNoRows {
		return "", fmt.Errorf("resolve incoming session generation: %w", err)
	}
	s.sessionRuntimeMu.Lock()
	closing := s.closingSessions[sessionID] > 0
	s.sessionRuntimeMu.Unlock()
	if closing {
		return "", ErrSessionDeleted
	}
	var hasTombstones, deleted bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE type='table' AND name='deleted_chat_sessions')`).Scan(&hasTombstones); err != nil {
		return "", err
	}
	if hasTombstones {
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM deleted_chat_sessions WHERE session_id=?)`, sessionID).Scan(&deleted); err != nil {
			return "", err
		}
	}
	if deleted {
		sessionID = "sess_" + rand.Text()
		if _, err := tx.ExecContext(ctx, `INSERT INTO chat_session_generations(base_hash,session_id) VALUES (?,?) ON CONFLICT(base_hash) DO UPDATE SET session_id=excluded.session_id`, key, sessionID); err != nil {
			return "", fmt.Errorf("advance incoming session generation: %w", err)
		}
	}
	// The shared human-facing default channel stays selectable after generation changes.
	if baseID == "default" && sessionID != baseID {
		if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO chat_sessions(id) VALUES(?)`, sessionID); err != nil {
			return "", fmt.Errorf("publish incoming chat session: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("commit incoming session generation: %w", err)
	}
	return sessionID, nil
}
