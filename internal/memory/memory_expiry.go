package memory

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// memoryExpiryLayout matches SQLite CURRENT_TIMESTAMP, so expires_at compares
// as text against it. Values are always UTC.
const memoryExpiryLayout = "2006-01-02 15:04:05"

func migrateMemoryExpiry(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS memory_expiry (
			doc_id TEXT PRIMARY KEY, expires_at TEXT NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_memory_expiry_at ON memory_expiry(expires_at);`)
	if err != nil {
		return fmt.Errorf("create memory expiry schema: %w", err)
	}
	return nil
}

// SetMemoryExpiry records when the given memories stop being true. A zero time
// removes the expiry: the latest statement of a fact decides how long it holds.
func (s *SQLiteMemory) SetMemoryExpiry(ctx context.Context, docIDs []string, expiresAt time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, id := range docIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if expiresAt.IsZero() {
			_, err = tx.ExecContext(ctx, `DELETE FROM memory_expiry WHERE doc_id=?`, id)
		} else {
			_, err = tx.ExecContext(ctx, `INSERT INTO memory_expiry(doc_id,expires_at) VALUES(?,?)
				ON CONFLICT(doc_id) DO UPDATE SET expires_at=excluded.expires_at`, id, expiresAt.UTC().Format(memoryExpiryLayout))
		}
		if err != nil {
			return fmt.Errorf("record memory expiry: %w", err)
		}
	}
	return tx.Commit()
}

// MemoryExpiry returns the recorded end of validity of one memory.
func (s *SQLiteMemory) MemoryExpiry(ctx context.Context, docID string) (time.Time, bool, error) {
	var raw string
	err := s.db.QueryRowContext(ctx, `SELECT expires_at FROM memory_expiry WHERE doc_id=?`, docID).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return time.Time{}, false, nil
	}
	if err != nil {
		return time.Time{}, false, err
	}
	at, err := time.ParseInLocation(memoryExpiryLayout, raw, time.UTC)
	if err != nil {
		return time.Time{}, false, fmt.Errorf("parse memory expiry: %w", err)
	}
	return at, true, nil
}
