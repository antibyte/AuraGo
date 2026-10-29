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

// ArchiveExpiredMemoriesContext archives memories whose stated validity ended
// and removes their vectors through the archive cleanup queue. Unlike budget
// archives, an expired fact keeps no archive identity: the user may state the
// same short-lived fact again later. Protected and keep-forever memories only
// lose their expiry.
func (s *SQLiteMemory) ArchiveExpiredMemoriesContext(ctx context.Context, vector VectorDB) (int, error) {
	if vector == nil {
		return 0, nil
	}
	if err := lockMemoryMutex(ctx, &s.automaticMemoryMu); err != nil {
		return 0, err
	}
	defer s.automaticMemoryMu.Unlock()
	s.bindArchiveVector(vector)
	ctx, release, err := s.BeginMemoryPublication(ctx)
	if err != nil {
		return 0, err
	}
	defer release()
	if _, err := s.db.ExecContext(ctx, `DELETE FROM memory_expiry WHERE doc_id NOT IN (SELECT doc_id FROM memory_meta)
		OR doc_id IN (SELECT doc_id FROM memory_meta WHERE protected=1 OR keep_forever=1
			OR verification_status='archived' OR COALESCE(archived_at,'')<>'')`); err != nil {
		return 0, fmt.Errorf("prune memory expiry: %w", err)
	}
	rows, err := s.db.QueryContext(ctx, `SELECT doc_id FROM memory_expiry WHERE expires_at<=CURRENT_TIMESTAMP ORDER BY expires_at, doc_id LIMIT 500`)
	if err != nil {
		return 0, err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return 0, err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return 0, err
	}
	candidates := make([]memoryArchiveCandidate, 0, len(ids))
	for _, id := range ids {
		var candidate memoryArchiveCandidate
		if err := scanMemoryMeta(s.db.QueryRowContext(ctx, `SELECT `+memoryMetaSelectColumns+` FROM memory_meta WHERE doc_id=?`, id), &candidate.meta); err != nil {
			return 0, err
		}
		candidate.identity, candidate.hash, err = s.archiveDocumentIdentity(ctx, vector, id)
		if err != nil {
			return 0, err
		}
		candidates = append(candidates, candidate)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	applied := 0
	for _, candidate := range candidates {
		var current MemoryMeta
		err := scanMemoryMeta(tx.QueryRowContext(ctx, `SELECT `+memoryMetaSelectColumns+` FROM memory_meta WHERE doc_id=?`, candidate.meta.DocID), &current)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return 0, err
		}
		if current.Protected || current.KeepForever || IsMemoryArchived(current) || !memoryMetaEqual(current, candidate.meta) {
			continue
		}
		// Without source content there is no hash that could authorize a deletion.
		if err := archiveMemoryCandidate(ctx, tx, candidate, "expired", "system", candidate.hash != ""); err != nil {
			return 0, err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM memory_archive_identities WHERE doc_id=?`, candidate.meta.DocID); err != nil {
			return 0, err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM memory_expiry WHERE doc_id=?`, candidate.meta.DocID); err != nil {
			return 0, err
		}
		applied++
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit memory expiry archive: %w", err)
	}
	if err := s.cleanupMemoryArchives(ctx, vector, 500); err != nil {
		return applied, err
	}
	return applied, nil
}
