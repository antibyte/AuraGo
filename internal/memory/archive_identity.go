package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

func memoryDocumentIdentity(document, domain string) string {
	if concept, body, ok := AnalysisDocumentParts(document); ok {
		identity, _ := AnalysisMemoryIdentity(concept, body, domain)
		return identity
	}
	encoded, _ := json.Marshal([]string{normalizeMemoryDocumentPart(document), normalizeMemoryDocumentPart(domain)})
	return contentSHA256(string(encoded))
}

func (s *SQLiteMemory) bindArchiveVector(vector VectorDB) {
	if s == nil || vector == nil {
		return
	}
	s.archiveVectorMu.Lock()
	s.archiveVector = vector
	s.archiveVectorMu.Unlock()
}

func (s *SQLiteMemory) boundArchiveVector() VectorDB {
	s.archiveVectorMu.RLock()
	defer s.archiveVectorMu.RUnlock()
	return s.archiveVector
}

// archiveDocumentIdentity reads only canonical content and metadata, never the
// embedding. A missing source cannot be assigned an invented identity.
func archiveDocumentIdentity(ctx context.Context, vector VectorDB, id string) (identity, hash string, err error) {
	if vector == nil {
		return "", "", nil
	}
	var content, domain string
	if cv, ok := vector.(*ChromemVectorDB); ok && cv.corpus != nil {
		var metadata string
		err = cv.corpus.db.QueryRowContext(ctx, `SELECT content,metadata FROM documents WHERE collection='aurago_memories' AND id=?`, id).Scan(&content, &metadata)
		if errors.Is(err, sql.ErrNoRows) {
			return "", "", nil
		}
		if err != nil {
			return "", "", err
		}
		var meta map[string]string
		if err := json.Unmarshal([]byte(metadata), &meta); err != nil {
			return "", "", fmt.Errorf("read archive document metadata: %w", err)
		}
		domain = meta["domain"]
	} else {
		content, err = vector.GetByID(id)
		if err != nil {
			return "", "", err
		}
	}
	if strings.TrimSpace(content) == "" {
		return "", "", nil
	}
	return memoryDocumentIdentity(content, domain), contentSHA256(content), nil
}

// Automatic chunks retain the identity of their complete source document.
// The hash still describes the individual chunk for conditional cleanup.
func (s *SQLiteMemory) archiveDocumentIdentity(ctx context.Context, vector VectorDB, id string) (identity, hash string, err error) {
	identity, hash, err = archiveDocumentIdentity(ctx, vector, id)
	if err != nil || hash == "" {
		return identity, hash, err
	}
	var original string
	err = s.db.QueryRowContext(ctx, `SELECT identity FROM memory_document_identities WHERE doc_id=?`, id).Scan(&original)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return "", "", fmt.Errorf("read original archive identity: %w", err)
	}
	if original != "" {
		identity = original
	}
	return identity, hash, nil
}

func (s *SQLiteMemory) recordDocumentIdentity(ctx context.Context, id, identity string) error {
	if identity == "" {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO memory_document_identities(doc_id,identity) VALUES(?,?)
		ON CONFLICT(doc_id) DO UPDATE SET identity=excluded.identity`, id, identity)
	return err
}

func (s *SQLiteMemory) archivedIdentityBlocked(ctx context.Context, identity string) (bool, error) {
	var blocked bool
	err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM memory_archive_identities WHERE identity=?)`, identity).Scan(&blocked)
	return blocked, err
}

// backfillArchiveIdentities is called under automaticMemoryMu. A durable cursor
// bounds each query; cancellation resumes at the last committed page.
func (s *SQLiteMemory) backfillArchiveIdentities(ctx context.Context, vector VectorDB) error {
	const key = "archive_identity_backfill_v1"
	var cursor string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM memory_maintenance_meta WHERE key=?`, key).Scan(&cursor)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if cursor == "completed" || vector == nil {
		return nil
	}
	for {
		rows, err := s.db.QueryContext(ctx, `SELECT doc_id FROM memory_meta
			WHERE doc_id>? AND (verification_status='archived' OR COALESCE(archived_at,'')<>'')
			ORDER BY doc_id LIMIT 200`, cursor)
		if err != nil {
			return err
		}
		var ids []string
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return err
			}
			ids = append(ids, id)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		for _, id := range ids {
			identity, _, err := s.archiveDocumentIdentity(ctx, vector, id)
			if err != nil {
				return fmt.Errorf("backfill archive identity: %w", err)
			}
			if identity != "" {
				if _, err := s.db.ExecContext(ctx, `INSERT INTO memory_archive_identities(doc_id,identity) VALUES(?,?) ON CONFLICT(doc_id) DO NOTHING`, id, identity); err != nil {
					return err
				}
			}
		}
		if len(ids) < 200 {
			cursor = "completed"
		} else {
			cursor = ids[len(ids)-1]
		}
		if _, err := s.db.ExecContext(ctx, `INSERT INTO memory_maintenance_meta(key,value) VALUES(?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`, key, cursor); err != nil {
			return err
		}
		if cursor == "completed" {
			return nil
		}
	}
}
