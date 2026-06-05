package memory

import (
	"database/sql"
	"strings"
	"time"
)

func (s *SQLiteMemory) GetFileIndex(path, collection string) (time.Time, error) {
	var lastMod time.Time
	// Try path+collection first (collection-aware)
	err := s.db.QueryRow("SELECT last_modified FROM file_indices WHERE file_path = ? AND collection = ?", path, collection).Scan(&lastMod)
	if err == nil {
		return lastMod, nil
	}
	if err != sql.ErrNoRows {
		return time.Time{}, err
	}
	// Fallback to path-only for backward compatibility with pre-migration data
	err = s.db.QueryRow("SELECT last_modified FROM file_indices WHERE file_path = ? AND collection = ''", path).Scan(&lastMod)
	if err == sql.ErrNoRows {
		return time.Time{}, nil
	}
	return lastMod, err
}

// UpdateFileIndex updates the last modified time for a given file path within a collection.
func (s *SQLiteMemory) UpdateFileIndex(path, collection string, modTime time.Time) error {
	_, err := s.db.Exec(`
		INSERT INTO file_indices (file_path, collection, last_modified)
		VALUES (?, ?, ?)
		ON CONFLICT(file_path, collection) DO UPDATE SET
			last_modified = excluded.last_modified
	`, path, collection, modTime)
	return err
}

// FileIndexState contains the persisted change-detection metadata for an indexed file.
type FileIndexState struct {
	LastModified     time.Time
	ContentHash      string
	IndexFingerprint string
}

// GetFileIndexState returns timestamp and content-fingerprint metadata for a file.
func (s *SQLiteMemory) GetFileIndexState(path, collection string) (FileIndexState, error) {
	var state FileIndexState
	err := s.db.QueryRow(`
		SELECT last_modified, COALESCE(content_hash, ''), COALESCE(index_fingerprint, '')
		FROM file_indices
		WHERE file_path = ? AND collection = ?
	`, path, collection).Scan(&state.LastModified, &state.ContentHash, &state.IndexFingerprint)
	if err == sql.ErrNoRows {
		err = s.db.QueryRow(`
			SELECT last_modified, COALESCE(content_hash, ''), COALESCE(index_fingerprint, '')
			FROM file_indices
			WHERE file_path = ? AND collection = ''
		`, path).Scan(&state.LastModified, &state.ContentHash, &state.IndexFingerprint)
		if err == sql.ErrNoRows {
			return FileIndexState{}, nil
		}
	}
	return state, err
}

// UpdateFileIndexWithDocs updates the last modified time for a given file path within a collection
// and replaces the tracked VectorDB document IDs generated from that file.
func (s *SQLiteMemory) UpdateFileIndexWithDocs(path, collection string, modTime time.Time, docIDs []string) error {
	return s.UpdateFileIndexWithDocsAndState(path, collection, modTime, "", "", docIDs)
}

// UpdateFileIndexWithDocsAndState updates file index metadata and replaces tracked VectorDB document IDs.
func (s *SQLiteMemory) UpdateFileIndexWithDocsAndState(path, collection string, modTime time.Time, contentHash, indexFingerprint string, docIDs []string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`
		INSERT INTO file_indices (file_path, collection, last_modified, content_hash, index_fingerprint)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(file_path, collection) DO UPDATE SET
			last_modified = excluded.last_modified,
			content_hash = excluded.content_hash,
			index_fingerprint = excluded.index_fingerprint
	`, path, collection, modTime, contentHash, indexFingerprint); err != nil {
		return err
	}

	if _, err := tx.Exec(`DELETE FROM file_embedding_docs WHERE file_path = ? AND collection = ?`, path, collection); err != nil {
		return err
	}

	for _, docID := range docIDs {
		docID = strings.TrimSpace(docID)
		if docID == "" {
			continue
		}
		if _, err := tx.Exec(`
			INSERT INTO file_embedding_docs (file_path, collection, doc_id)
			VALUES (?, ?, ?)
		`, path, collection, docID); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// GetFileEmbeddingDocIDs returns the tracked VectorDB document IDs for a file path within a collection.
// For backward compatibility, if no entry exists for path+collection, falls back to
// path-only lookup (handles pre-migration data with collection=”).
func (s *SQLiteMemory) GetFileEmbeddingDocIDs(path, collection string) ([]string, error) {
	// Try path+collection first (collection-aware)
	rows, err := s.db.Query(`
		SELECT doc_id
		FROM file_embedding_docs
		WHERE file_path = ? AND collection = ?
		ORDER BY doc_id
	`, path, collection)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var docIDs []string
	for rows.Next() {
		var docID string
		if err := rows.Scan(&docID); err != nil {
			return nil, err
		}
		docIDs = append(docIDs, docID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// If we found docs, return them
	if len(docIDs) > 0 {
		return docIDs, nil
	}
	// Fallback to path-only for backward compatibility with pre-migration data
	rows2, err := s.db.Query(`
		SELECT doc_id
		FROM file_embedding_docs
		WHERE file_path = ? AND collection = ''
		ORDER BY doc_id
	`, path)
	if err != nil {
		return nil, err
	}
	defer rows2.Close()
	for rows2.Next() {
		var docID string
		if err := rows2.Scan(&docID); err != nil {
			return nil, err
		}
		docIDs = append(docIDs, docID)
	}
	return docIDs, rows2.Err()
}

// ListIndexedFiles returns all tracked file paths for a given collection.
func (s *SQLiteMemory) ListIndexedFiles(collection string) ([]string, error) {
	rows, err := s.db.Query(`
		SELECT file_path
		FROM file_indices
		WHERE collection = ?
		ORDER BY file_path
	`, collection)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var paths []string
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			return nil, err
		}
		paths = append(paths, path)
	}
	return paths, rows.Err()
}

// DeleteFileIndex removes file-index metadata and tracked VectorDB document IDs
// for a file path within a specific collection. This is used when a file is removed
// or before a full reindex.
func (s *SQLiteMemory) DeleteFileIndex(path, collection string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM file_embedding_docs WHERE file_path = ? AND collection = ?`, path, collection); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM file_indices WHERE file_path = ? AND collection = ?`, path, collection); err != nil {
		return err
	}

	return tx.Commit()
}

// ClearFileIndices removes all persisted file-index timestamps so the indexer
// treats knowledge/doc files as new and rebuilds their embeddings. It also
// clears the tracked file-to-vector document mappings.
func (s *SQLiteMemory) ClearFileIndices() error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM file_embedding_docs`); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM file_indices`); err != nil {
		return err
	}

	return tx.Commit()
}

// ClearMemoryMeta removes all long-term memory metadata. This is needed when
// the embedding database is rebuilt from scratch so stale chunk references do
// not remain in SQLite.
