package memory

import (
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"aurago/internal/dbutil"
)

const pendingMemoryWriteMaxAttempts = 6

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
		return nil
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
	return dbutil.MigrateAddColumn(s.db, "pending_memory_writes", "metadata_json", "TEXT", s.logger)
}

func (s *SQLiteMemory) EnqueuePendingMemoryWrite(write PendingMemoryWrite, cause error) error {
	if write.Concept == "" || write.Content == "" {
		return fmt.Errorf("pending memory write requires concept and content")
	}
	lastError := ""
	if cause != nil {
		lastError = cause.Error()
	}
	var metadata any
	if write.Metadata != nil {
		encoded, err := json.Marshal(pendingMemoryWriteMetadata{Version: 1, Details: write.Metadata})
		if err != nil {
			return fmt.Errorf("encode pending memory metadata: %w", err)
		}
		metadata = string(encoded)
	}
	_, err := s.db.Exec(`
		INSERT INTO pending_memory_writes (concept, content, domain, last_error, metadata_json)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(concept, content, domain) DO UPDATE SET
			metadata_json = COALESCE(pending_memory_writes.metadata_json, excluded.metadata_json),
			last_error = excluded.last_error,
			updated_at = CURRENT_TIMESTAMP
	`, write.Concept, write.Content, write.Domain, lastError, metadata)
	if err != nil {
		return fmt.Errorf("enqueue pending memory write: %w", err)
	}
	return nil
}

func (s *SQLiteMemory) GetDuePendingMemoryWrites(now time.Time, limit int) ([]PendingMemoryWrite, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	rows, err := s.db.Query(`
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
	lastError := ""
	if cause != nil {
		lastError = cause.Error()
	}
	var attempts int
	if err := s.db.QueryRow(`SELECT attempts + 1 FROM pending_memory_writes WHERE id = ?`, id).Scan(&attempts); err != nil {
		return fmt.Errorf("read pending memory write attempts: %w", err)
	}
	status := "pending"
	if attempts >= pendingMemoryWriteMaxAttempts {
		status = "exhausted"
	}
	delay := 5 * time.Minute * time.Duration(1<<min(attempts-1, 8))
	if delay > 24*time.Hour {
		delay = 24 * time.Hour
	}
	_, err := s.db.Exec(`
		UPDATE pending_memory_writes
		SET attempts = ?, next_attempt_at = ?, last_error = ?, status = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, attempts, now.UTC().Add(delay), lastError, status, id)
	if err != nil {
		return fmt.Errorf("mark pending memory write failed: %w", err)
	}
	return nil
}

func (s *SQLiteMemory) CompletePendingMemoryWrite(id int64) error {
	if _, err := s.db.Exec(`DELETE FROM pending_memory_writes WHERE id = ?`, id); err != nil {
		return fmt.Errorf("complete pending memory write: %w", err)
	}
	return nil
}

func (s *SQLiteMemory) CountPendingMemoryWrites() (int, error) {
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM pending_memory_writes`).Scan(&count); err != nil {
		return 0, fmt.Errorf("count pending memory writes: %w", err)
	}
	return count, nil
}
