package memory

import (
	"fmt"
	"os"
	"path/filepath"

	"aurago/internal/dbutil"
)

// ScheduleDocumentReindex refuses legacy-only stores and changes derived work
// state only after the canonical corpus completed its verified migration.
func ScheduleDocumentReindex(vectorDir string) error {
	path := filepath.Join(vectorDir, "documents.sqlite")
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("inspect document corpus: %w", err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("document corpus must be a regular file")
	}
	db, err := dbutil.Open(path, dbutil.WithImmediateTransactions())
	if err != nil {
		return fmt.Errorf("open document corpus: %w", err)
	}
	defer db.Close()
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin corpus reset: %w", err)
	}
	defer tx.Rollback()
	var verified string
	if err := tx.QueryRow(`SELECT value FROM corpus_meta WHERE key='verified'`).Scan(&verified); err != nil {
		return fmt.Errorf("document corpus is not verified: %w", err)
	}
	if verified != "1" {
		return fmt.Errorf("document corpus is not verified")
	}
	if _, err := tx.Exec(`UPDATE documents SET pending=1 WHERE collection<>'kg_embeddings'`); err != nil {
		return fmt.Errorf("schedule corpus reindex: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit corpus reindex: %w", err)
	}
	return nil
}

// ScheduleFileSourceReindex invalidates only the derived fingerprint. The
// existing authorized indexer reopens available sources and atomically publishes
// a fresh generation while retaining all previous source/tracking evidence.
func (s *SQLiteMemory) ScheduleFileSourceReindex() error {
	if s == nil || s.db == nil {
		return fmt.Errorf("file metadata store is unavailable")
	}
	if _, err := s.db.Exec(`UPDATE file_indices SET index_fingerprint=''`); err != nil {
		return fmt.Errorf("schedule file source reindex: %w", err)
	}
	return nil
}
