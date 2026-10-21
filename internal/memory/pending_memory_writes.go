package memory

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"aurago/internal/dbutil"
	"aurago/internal/security"
)

const pendingMemoryWriteMaxAttempts = 6
const pendingMemoryWriteLastErrorMaxBytes = 1000

type PendingMemoryWrite struct {
	ID            int64             `json:"id"`
	Concept       string            `json:"concept"`
	Content       string            `json:"content"`
	Domain        string            `json:"domain,omitempty"`
	Attempts      int               `json:"attempts"`
	NextAttemptAt time.Time         `json:"next_attempt_at"`
	LastError     string            `json:"last_error"`
	CreatedAt     time.Time         `json:"created_at"`
	Metadata      *MemoryMetaUpdate `json:"metadata,omitempty"`
	MetadataError error             `json:"-"`
}

type pendingMemoryWriteMetadata struct {
	Version int               `json:"version"`
	Details *MemoryMetaUpdate `json:"details"`
}

func (s *SQLiteMemory) InitPendingMemoryWritesTable() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS pending_memory_writes (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			concept TEXT NOT NULL,
			content TEXT NOT NULL,
			domain TEXT NOT NULL DEFAULT '',
			metadata_json TEXT,
			attempts INTEGER NOT NULL DEFAULT 0,
			next_attempt_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			last_error TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'pending',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(concept, content, domain)
		);
		CREATE INDEX IF NOT EXISTS idx_pending_memory_writes_due
			ON pending_memory_writes(status, next_attempt_at);
	`)
	if err != nil {
		return fmt.Errorf("pending memory writes schema: %w", err)
	}
	var hasMetadata bool
	if err := s.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM pragma_table_info('pending_memory_writes') WHERE name='metadata_json')`).Scan(&hasMetadata); err != nil {
		return fmt.Errorf("inspect pending memory metadata migration: %w", err)
	}
	if hasMetadata {
		return s.normalizeLegacyPendingMemoryWrites()
	}
	var populated bool
	if err := s.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM pending_memory_writes) OR EXISTS(SELECT 1 FROM memory_meta) OR EXISTS(SELECT 1 FROM messages)`).Scan(&populated); err != nil {
		return fmt.Errorf("inspect memory before queue migration: %w", err)
	}
	if populated {
		var path string
		if err := s.db.QueryRow(`SELECT file FROM pragma_database_list WHERE name='main'`).Scan(&path); err != nil {
			return fmt.Errorf("locate pending memory database: %w", err)
		}
		if path != "" {
			backup := path + ".pending-memory-metadata-v1-" + rand.Text() + ".bak"
			if _, err := s.db.Exec(`VACUUM main INTO ?`, backup); err != nil {
				return fmt.Errorf("back up pending memory metadata migration: %w", err)
			}
			if err := os.Chmod(backup, 0600); err != nil {
				return fmt.Errorf("protect pending memory migration backup: %w", err)
			}
		}
	}
	if err := dbutil.MigrateAddColumn(s.db, "pending_memory_writes", "metadata_json", "TEXT", s.logger); err != nil {
		return err
	}
	return s.normalizeLegacyPendingMemoryWrites()
}

func (s *SQLiteMemory) EnqueuePendingMemoryWrite(write PendingMemoryWrite, cause error) error {
	ctx, cancel := maintenanceCompatibilityContext()
	defer cancel()
	return s.EnqueuePendingMemoryWriteContext(ctx, write, cause)
}

func (s *SQLiteMemory) EnqueuePendingMemoryWriteContext(ctx context.Context, write PendingMemoryWrite, cause error) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("pending memory write store is unavailable")
	}
	ctx = maintenanceContext(ctx)
	if err := ctx.Err(); err != nil {
		return err
	}
	if session, ok := MemoryAnalysisSession(write.Content); ok && session != "" && (write.Metadata == nil || write.Metadata.SourceSessionID == "") {
		details := MemoryMetaUpdate{SourceType: "memory_analysis", SourceRole: "unknown"}
		if write.Metadata != nil {
			details = *write.Metadata
		}
		details.SourceSessionID = session
		write.Metadata = &details
	}
	sessionHash := ""
	if write.Metadata != nil && write.Metadata.SourceSessionID != "" {
		if err := lockMemoryMutex(ctx, &s.sessionWriteMu); err != nil {
			return err
		}
		defer s.sessionWriteMu.Unlock()
		if err := s.checkSessionWritable(ctx, write.Metadata.SourceSessionID); err != nil {
			return err
		}
		sessionHash = PendingMemorySessionFingerprint(write.Metadata.SourceSessionID)
	}
	if HasDurableCredential(write.Concept + "\n" + write.Content) {
		return fmt.Errorf("%w: credentials cannot enter pending memory", ErrAutomaticMemorySkipped)
	}
	if write.Concept == "" || write.Content == "" {
		return fmt.Errorf("pending memory write requires concept and content")
	}
	fingerprint := pendingMemoryWriteFingerprint(write, sessionHash)
	var tombstoned bool
	if err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM pending_memory_write_tombstones WHERE fingerprint=?)`, fingerprint).Scan(&tombstoned); err != nil {
		return fmt.Errorf("check pending memory write tombstone: %w", err)
	}
	if tombstoned {
		return fmt.Errorf("%w: exhausted memory write was retained as a tombstone", ErrAutomaticMemorySkipped)
	}
	lastError := ""
	if cause != nil {
		lastError = truncateUTF8Bytes(security.RedactSensitiveInfo(security.Scrub(cause.Error())), pendingMemoryWriteLastErrorMaxBytes)
	}
	var metadata any
	if write.Metadata != nil {
		encoded, err := json.Marshal(pendingMemoryWriteMetadata{Version: 1, Details: write.Metadata})
		if err != nil {
			return fmt.Errorf("encode pending memory metadata: %w", err)
		}
		metadata = string(encoded)
	}
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO pending_memory_writes (concept, content, domain, last_error, metadata_json, source_fingerprint, source_session_hash)
		SELECT ?, ?, ?, ?, ?, ?, ?
		WHERE NOT EXISTS (
			SELECT 1 FROM pending_memory_write_tombstones WHERE fingerprint = ?
		)
		ON CONFLICT(concept, content, domain) DO UPDATE SET
			metadata_json = COALESCE(pending_memory_writes.metadata_json, excluded.metadata_json),
			last_error = excluded.last_error,
			updated_at = CURRENT_TIMESTAMP
	`, write.Concept, write.Content, write.Domain, lastError, metadata, fingerprint, sessionHash, fingerprint)
	if err != nil {
		return fmt.Errorf("enqueue pending memory write: %w", err)
	}
	if affected, err := res.RowsAffected(); err != nil {
		return fmt.Errorf("count enqueued pending memory writes: %w", err)
	} else if affected == 0 {
		return fmt.Errorf("%w: exhausted memory write was retained as a tombstone", ErrAutomaticMemorySkipped)
	}
	return nil
}

func (s *SQLiteMemory) GetDuePendingMemoryWrites(now time.Time, limit int) ([]PendingMemoryWrite, error) {
	ctx, cancel := maintenanceCompatibilityContext()
	defer cancel()
	return s.GetDuePendingMemoryWritesContext(ctx, now, limit)
}

func (s *SQLiteMemory) GetDuePendingMemoryWritesContext(ctx context.Context, now time.Time, limit int) ([]PendingMemoryWrite, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("pending memory write store is unavailable")
	}
	ctx = maintenanceContext(ctx)
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, concept, content, domain, attempts, next_attempt_at, last_error, created_at, metadata_json
		FROM pending_memory_writes
		WHERE status = 'pending' AND attempts < ? AND next_attempt_at <= ?
		ORDER BY next_attempt_at, id
		LIMIT ?
	`, pendingMemoryWriteMaxAttempts, now.UTC(), limit)
	if err != nil {
		return nil, fmt.Errorf("get due pending memory writes: %w", err)
	}
	defer rows.Close()
	writes := make([]PendingMemoryWrite, 0)
	for rows.Next() {
		var write PendingMemoryWrite
		var encoded sql.NullString
		if err := rows.Scan(&write.ID, &write.Concept, &write.Content, &write.Domain, &write.Attempts, &write.NextAttemptAt, &write.LastError, &write.CreatedAt, &encoded); err != nil {
			return nil, fmt.Errorf("scan pending memory write: %w", err)
		}
		if encoded.Valid {
			var payload pendingMemoryWriteMetadata
			if err := json.Unmarshal([]byte(encoded.String), &payload); err != nil {
				write.MetadataError = fmt.Errorf("decode pending memory metadata for %d: %w", write.ID, err)
			} else if payload.Version != 1 || payload.Details == nil {
				write.MetadataError = fmt.Errorf("invalid pending memory metadata version or details for %d", write.ID)
			} else {
				write.Metadata = payload.Details
			}
		}
		writes = append(writes, write)
	}
	return writes, rows.Err()
}

func (s *SQLiteMemory) MarkPendingMemoryWriteFailed(id int64, cause error, now time.Time) error {
	ctx, cancel := maintenanceCompatibilityContext()
	defer cancel()
	return s.MarkPendingMemoryWriteFailedContext(ctx, id, cause, now)
}

func (s *SQLiteMemory) MarkPendingMemoryWriteFailedContext(ctx context.Context, id int64, cause error, now time.Time) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("pending memory write store is unavailable")
	}
	ctx = maintenanceContext(ctx)
	lastError := ""
	if cause != nil {
		lastError = truncateUTF8Bytes(security.RedactSensitiveInfo(security.Scrub(cause.Error())), pendingMemoryWriteLastErrorMaxBytes)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin pending memory write failure: %w", err)
	}
	defer tx.Rollback()
	var attempts int
	var status string
	if err := tx.QueryRowContext(ctx, `SELECT attempts + 1, status FROM pending_memory_writes WHERE id = ?`, id).Scan(&attempts, &status); err != nil {
		return fmt.Errorf("read pending memory write attempts: %w", err)
	}
	if status != "pending" {
		return nil
	}
	status = "pending"
	if attempts >= pendingMemoryWriteMaxAttempts {
		status = "exhausted"
	}
	delay := 5 * time.Minute * time.Duration(1<<min(attempts-1, 8))
	if delay > 24*time.Hour {
		delay = 24 * time.Hour
	}
	_, err = tx.ExecContext(ctx, `
		UPDATE pending_memory_writes
		SET attempts = ?, next_attempt_at = ?, last_error = ?, status = ?,
		    terminal_at = CASE WHEN ?='exhausted' THEN COALESCE(terminal_at, ?) ELSE terminal_at END,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, attempts, now.UTC().Add(delay), lastError, status, status, now.UTC(), id)
	if err != nil {
		return fmt.Errorf("mark pending memory write failed: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit pending memory write failure: %w", err)
	}
	return nil
}

// DeferPendingMemoryWrite retains the validation retry budget during provider outages.
func (s *SQLiteMemory) DeferPendingMemoryWrite(id int64, now time.Time) error {
	ctx, cancel := maintenanceCompatibilityContext()
	defer cancel()
	return s.DeferPendingMemoryWriteContext(ctx, id, now)
}

func (s *SQLiteMemory) DeferPendingMemoryWriteContext(ctx context.Context, id int64, now time.Time) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("pending memory write store is unavailable")
	}
	ctx = maintenanceContext(ctx)
	_, err := s.db.ExecContext(ctx, `UPDATE pending_memory_writes SET next_attempt_at=?, last_error='infrastructure_unavailable', updated_at=CURRENT_TIMESTAMP WHERE id=? AND status='pending'`, now.UTC().Add(5*time.Minute), id)
	if err != nil {
		return fmt.Errorf("defer pending memory write: %w", err)
	}
	return nil
}

func (s *SQLiteMemory) CompletePendingMemoryWrite(id int64) error {
	ctx, cancel := maintenanceCompatibilityContext()
	defer cancel()
	return s.CompletePendingMemoryWriteContext(ctx, id)
}

func (s *SQLiteMemory) CompletePendingMemoryWriteContext(ctx context.Context, id int64) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("pending memory write store is unavailable")
	}
	ctx = maintenanceContext(ctx)
	if _, err := s.db.ExecContext(ctx, `DELETE FROM pending_memory_writes WHERE id = ?`, id); err != nil {
		return fmt.Errorf("complete pending memory write: %w", err)
	}
	return nil
}

func (s *SQLiteMemory) CountPendingMemoryWrites() (int, error) {
	ctx, cancel := maintenanceCompatibilityContext()
	defer cancel()
	return s.CountPendingMemoryWritesContext(ctx)
}

func (s *SQLiteMemory) CountPendingMemoryWritesContext(ctx context.Context) (int, error) {
	if s == nil || s.db == nil {
		return 0, fmt.Errorf("pending memory write store is unavailable")
	}
	ctx = maintenanceContext(ctx)
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM pending_memory_writes WHERE status='pending' AND attempts < ?`, pendingMemoryWriteMaxAttempts).Scan(&count); err != nil {
		return 0, fmt.Errorf("count pending memory writes: %w", err)
	}
	return count, nil
}

// CountExhaustedMemoryWritesContext includes retained exhausted queue rows and
// content-free tombstones left after their payloads expire.
func (s *SQLiteMemory) CountExhaustedMemoryWritesContext(ctx context.Context) (int, error) {
	if s == nil || s.db == nil {
		return 0, fmt.Errorf("pending memory write store is unavailable")
	}
	ctx = maintenanceContext(ctx)
	var count int
	if err := s.db.QueryRowContext(ctx, `
		SELECT
			(SELECT COUNT(*) FROM pending_memory_writes WHERE status='exhausted') +
			(SELECT COUNT(*) FROM pending_memory_write_tombstones)
	`).Scan(&count); err != nil {
		return 0, fmt.Errorf("count exhausted memory writes: %w", err)
	}
	return count, nil
}

// PendingMemorySessionFingerprint returns an opaque stable digest suitable for
// privacy tombstone lookup without retaining a raw session identifier.
func PendingMemorySessionFingerprint(sessionID string) string {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return ""
	}
	digest := sha256.Sum256([]byte("aurago/pending-memory-session/v1\x00" + sessionID))
	return hex.EncodeToString(digest[:])
}

func pendingMemoryWriteFingerprint(write PendingMemoryWrite, sessionHash string) string {
	identity := strings.Join([]string{
		strings.TrimSpace(write.Concept), strings.TrimSpace(write.Content), strings.TrimSpace(write.Domain), sessionHash,
	}, "\x00")
	digest := sha256.Sum256([]byte("aurago/pending-memory-write/v1\x00" + identity))
	return hex.EncodeToString(digest[:])
}

// CleanupExpiredPendingMemoryWrites deletes exhausted payloads only after 30
// days from the terminal timestamp, retaining a content-free dedupe tombstone.
func (s *SQLiteMemory) CleanupExpiredPendingMemoryWrites(ctx context.Context, now time.Time, limit int) (int, error) {
	if s == nil || s.db == nil {
		return 0, nil
	}
	ctx = maintenanceContext(ctx)
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	cutoff := now.UTC().Add(-30 * 24 * time.Hour)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin exhausted pending memory cleanup: %w", err)
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `
		SELECT id, concept, content, domain, metadata_json, source_fingerprint, source_session_hash, terminal_at
		FROM pending_memory_writes
		WHERE status='exhausted' AND terminal_at IS NOT NULL AND terminal_at<=?
		ORDER BY terminal_at, id LIMIT ?`, cutoff, limit)
	if err != nil {
		return 0, fmt.Errorf("select expired exhausted memory writes: %w", err)
	}
	type expiredWrite struct {
		id          int64
		write       PendingMemoryWrite
		metadata    sql.NullString
		fingerprint string
		sessionHash string
		hadSession  bool
		terminalAt  time.Time
	}
	expired := make([]expiredWrite, 0, limit)
	for rows.Next() {
		var item expiredWrite
		var terminal sql.NullTime
		if err := rows.Scan(&item.id, &item.write.Concept, &item.write.Content, &item.write.Domain, &item.metadata, &item.fingerprint, &item.sessionHash, &terminal); err != nil {
			rows.Close()
			return 0, fmt.Errorf("scan expired exhausted memory write: %w", err)
		}
		if terminal.Valid {
			item.terminalAt = terminal.Time.UTC()
		}
		item.sessionHash = strings.TrimSpace(item.sessionHash)
		item.hadSession = item.sessionHash != ""
		expired = append(expired, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, fmt.Errorf("iterate expired exhausted memory writes: %w", err)
	}
	if err := rows.Close(); err != nil {
		return 0, fmt.Errorf("close expired exhausted memory writes: %w", err)
	}
	if len(expired) == 0 {
		if err := tx.Commit(); err != nil {
			return 0, fmt.Errorf("commit empty exhausted memory cleanup: %w", err)
		}
		return 0, nil
	}
	ids := make([]string, len(expired))
	args := make([]any, 0, len(expired))
	for i, item := range expired {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		sessionID := pendingMemoryWriteSourceSession(item.write.Content, item.metadata)
		if item.sessionHash == "" && sessionID != "" {
			item.sessionHash = PendingMemorySessionFingerprint(sessionID)
		}
		item.fingerprint = strings.TrimSpace(item.fingerprint)
		if item.fingerprint == "" || (!item.hadSession && item.sessionHash != "") {
			item.fingerprint = pendingMemoryWriteFingerprint(item.write, item.sessionHash)
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE pending_memory_writes
			SET source_fingerprint=?, source_session_hash=?
			WHERE id=? AND (COALESCE(source_fingerprint,'')<>? OR COALESCE(source_session_hash,'')<>?)`,
			item.fingerprint, item.sessionHash, item.id, item.fingerprint, item.sessionHash); err != nil {
			return 0, fmt.Errorf("normalize expired pending memory write provenance: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO pending_memory_write_tombstones(fingerprint,source_session_hash,terminal_at)
			VALUES(?,?,?) ON CONFLICT(fingerprint) DO UPDATE SET
				source_session_hash=excluded.source_session_hash,
				terminal_at=MIN(pending_memory_write_tombstones.terminal_at,excluded.terminal_at)`,
			item.fingerprint, item.sessionHash, item.terminalAt); err != nil {
			return 0, fmt.Errorf("persist pending memory tombstone: %w", err)
		}
		ids[i] = "?"
		args = append(args, item.id)
	}
	res, err := tx.ExecContext(ctx, `DELETE FROM pending_memory_writes WHERE status='exhausted' AND id IN (`+strings.Join(ids, ",")+")", args...)
	if err != nil {
		return 0, fmt.Errorf("delete expired exhausted memory payloads: %w", err)
	}
	deleted, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("count expired exhausted memory payloads: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit exhausted pending memory cleanup: %w", err)
	}
	return int(deleted), nil
}

func pendingMemoryWriteSourceSession(content string, encoded sql.NullString) string {
	if encoded.Valid {
		var metadata pendingMemoryWriteMetadata
		if err := json.Unmarshal([]byte(encoded.String), &metadata); err == nil && metadata.Version == 1 && metadata.Details != nil {
			if sessionID := strings.TrimSpace(metadata.Details.SourceSessionID); sessionID != "" {
				return sessionID
			}
		}
	}
	sessionID, _ := MemoryAnalysisSession(content)
	return strings.TrimSpace(sessionID)
}
