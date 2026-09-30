package memory

import (
	"crypto/rand"
	"database/sql"
	"fmt"
	"os"
	"time"
)

const extractionSourcesSchema = `CREATE TABLE IF NOT EXISTS memory_extraction_sources (
	doc_id TEXT NOT NULL, source_type TEXT NOT NULL, session_id TEXT NOT NULL,
	first_seen_at TEXT NOT NULL, last_seen_at TEXT NOT NULL,
	PRIMARY KEY (doc_id, source_type, session_id)
)`

func (s *SQLiteMemory) initExtractionSources() error {
	var exists bool
	if err := s.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE name='memory_extraction_sources' AND type='table')`).Scan(&exists); err != nil {
		return fmt.Errorf("inspect extraction source migration: %w", err)
	}
	if exists {
		return nil
	}
	// Back up deployed stores before the additive migration, including WAL data.
	var populated bool
	if err := s.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM memory_meta) OR EXISTS(SELECT 1 FROM messages)`).Scan(&populated); err != nil {
		return fmt.Errorf("inspect memory before source migration: %w", err)
	}
	if populated {
		rows, err := s.db.Query(`PRAGMA database_list`)
		if err != nil {
			return fmt.Errorf("locate extraction source database: %w", err)
		}
		path := ""
		for rows.Next() {
			var sequence int
			var name, file string
			if err = rows.Scan(&sequence, &name, &file); err != nil {
				rows.Close()
				return err
			}
			if name == "main" {
				path = file
			}
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		if path != "" {
			backup := path + ".memory-sources-v1-" + rand.Text() + ".bak"
			if _, err = s.db.Exec(`VACUUM main INTO ?`, backup); err != nil {
				return fmt.Errorf("back up memory source migration: %w", err)
			}
			if err = os.Chmod(backup, 0600); err != nil {
				return fmt.Errorf("protect memory source backup: %w", err)
			}
		}
	}
	_, err := s.db.Exec(extractionSourcesSchema)
	if err != nil {
		return fmt.Errorf("create memory extraction sources: %w", err)
	}
	return nil
}

type MemoryExtractionSource struct {
	DocID       string
	SourceType  string
	SessionID   string
	FirstSeenAt string
	LastSeenAt  string
}

func (s *SQLiteMemory) RecordMemoryExtractionSource(docID, source, session string) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("memory source store is unavailable")
	}
	if session == "" {
		session = "default"
	}
	now := time.Now().UTC().Format("2006-01-02T15:04:05.000000000Z")
	_, err := s.db.Exec(`INSERT INTO memory_extraction_sources VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(doc_id,source_type,session_id) DO UPDATE SET
		first_seen_at=MIN(first_seen_at,excluded.first_seen_at),last_seen_at=MAX(last_seen_at,excluded.last_seen_at)`, docID, source, session, now, now)
	if err != nil {
		return fmt.Errorf("record memory extraction source: %w", err)
	}
	return nil
}

func readMemoryExtractionSources(db interface {
	Query(string, ...any) (*sql.Rows, error)
}, docID string) ([]MemoryExtractionSource, error) {
	rows, err := db.Query(`SELECT doc_id,source_type,session_id,first_seen_at,last_seen_at FROM memory_extraction_sources WHERE doc_id=? ORDER BY source_type,session_id`, docID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var sources []MemoryExtractionSource
	for rows.Next() {
		var source MemoryExtractionSource
		if err := rows.Scan(&source.DocID, &source.SourceType, &source.SessionID, &source.FirstSeenAt, &source.LastSeenAt); err != nil {
			return nil, err
		}
		sources = append(sources, source)
	}
	return sources, rows.Err()
}

func (s *SQLiteMemory) GetMemoryExtractionSources(docID string) ([]MemoryExtractionSource, error) {
	return readMemoryExtractionSources(s.db, docID)
}
