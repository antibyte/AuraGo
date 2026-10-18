package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"

	"aurago/internal/dbutil"
	"aurago/internal/fileutil"
	"aurago/internal/memory/kgsemantic"
	chromem "github.com/philippgille/chromem-go"
)

// documentCorpus is the source of truth. Chromem is a disposable in-memory index.
type documentCorpus struct{ db *sql.DB }

const corpusSchema = `
 CREATE TABLE IF NOT EXISTS corpus_meta(key TEXT PRIMARY KEY,value TEXT NOT NULL);
 CREATE TABLE IF NOT EXISTS collections(name TEXT PRIMARY KEY, metadata TEXT NOT NULL DEFAULT '{}');
 CREATE TABLE IF NOT EXISTS documents(
  collection TEXT NOT NULL REFERENCES collections(name), id TEXT NOT NULL,
  content TEXT NOT NULL, metadata TEXT NOT NULL, vector BLOB NOT NULL,
  dimension INTEGER NOT NULL, fingerprint TEXT NOT NULL, pending INTEGER NOT NULL DEFAULT 1,
  blocked_reason TEXT NOT NULL DEFAULT '',
  PRIMARY KEY(collection,id));
 CREATE INDEX IF NOT EXISTS documents_pending ON documents(pending,collection,id);`

// PrepareDocumentCorpus verifies/migrates durable sources before applying a
// model-change reset. It does not initialize an embedding provider.
func PrepareDocumentCorpus(dir string) error {
	c, err := openDocumentCorpus(dir)
	if err != nil {
		return err
	}
	return c.db.Close()
}

// BindMemoryMetadata provides current curation and file-generation eligibility.
func (cv *ChromemVectorDB) BindMemoryMetadata(stm *SQLiteMemory) { cv.fileIndexMemory.Store(stm) }

func (cv *ChromemVectorDB) readCollectionDocument(ctx context.Context, col *chromem.Collection, id string) (chromem.Document, error) {
	if cv.corpus != nil {
		return cv.corpusDocument(ctx, col.Name, id)
	}
	return col.GetByID(ctx, id)
}

func (cv *ChromemVectorDB) memoryRetrievalEligible(ctx context.Context, id string) (bool, error) {
	stm := cv.fileIndexMemory.Load()
	if stm == nil {
		if cv.corpus == nil {
			return true, nil
		}
		return false, fmt.Errorf("memory metadata is unavailable")
	}
	var eligible bool
	err := stm.db.QueryRowContext(ctx, `SELECT NOT EXISTS(SELECT 1 FROM memory_meta WHERE doc_id=? AND (verification_status='archived' OR COALESCE(archived_at,'')<>''))`, id).Scan(&eligible)
	return eligible, err
}

func openDocumentCorpus(dir string) (*documentCorpus, error) {
	if strings.TrimSpace(dir) == "" {
		return nil, fmt.Errorf("document corpus directory is required")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("create corpus directory: %w", err)
	}
	if err := fileutil.RestrictToOwner(dir); err != nil {
		return nil, fmt.Errorf("protect corpus directory: %w", err)
	}
	path := filepath.Join(dir, "documents.sqlite")
	if info, err := os.Lstat(path); err == nil {
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("document corpus must be regular")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err == nil {
		if err = file.Close(); err != nil {
			return nil, err
		}
	} else if !errors.Is(err, os.ErrExist) {
		return nil, fmt.Errorf("create corpus: %w", err)
	}
	db, err := dbutil.Open(path, dbutil.WithImmediateTransactions(), dbutil.WithSynchronous("FULL"))
	if err != nil {
		return nil, fmt.Errorf("open corpus: %w", err)
	}
	fail := func(err error) (*documentCorpus, error) { db.Close(); return nil, err }
	if err := protectMemoryDatabase(db); err != nil {
		return fail(err)
	}
	if _, err := db.Exec(corpusSchema); err != nil {
		return fail(fmt.Errorf("initialize corpus: %w", err))
	}
	c := &documentCorpus{db: db}
	var verified string
	err = db.QueryRow(`SELECT value FROM corpus_meta WHERE key='verified'`).Scan(&verified)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fail(err)
	}
	if verified != "1" {
		if err := c.migrateLegacy(dir); err != nil {
			return fail(err)
		}
	}
	return c, nil
}

func writeCorpusDocument(ctx context.Context, tx *sql.Tx, collection string, doc chromem.Document) error {
	if doc.ID == "" || doc.Content == "" {
		return fmt.Errorf("document ID and content are required")
	}
	metadata, err := json.Marshal(doc.Metadata)
	if err != nil {
		return err
	}
	vector, err := json.Marshal(doc.Embedding)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO collections(name) VALUES(?)`, collection); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO documents(collection,id,content,metadata,vector,dimension,fingerprint,pending) VALUES(?,?,?,?,?,?,?,1)
 ON CONFLICT(collection,id) DO UPDATE SET content=excluded.content,metadata=excluded.metadata,vector=excluded.vector,dimension=excluded.dimension,fingerprint=excluded.fingerprint,pending=1,blocked_reason=''`, collection, doc.ID, doc.Content, string(metadata), vector, len(doc.Embedding), doc.Metadata["embedding_fingerprint"])
	return err
}

func (c *documentCorpus) documents(ctx context.Context, collection string) ([]chromem.Document, error) {
	rows, err := c.db.QueryContext(ctx, `SELECT id,content,metadata,vector,dimension FROM documents WHERE collection=? ORDER BY id`, collection)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var docs []chromem.Document
	for rows.Next() {
		var doc chromem.Document
		var metadata string
		var vector []byte
		var dimension int
		if err := rows.Scan(&doc.ID, &doc.Content, &metadata, &vector, &dimension); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(metadata), &doc.Metadata); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(vector, &doc.Embedding); err != nil {
			return nil, err
		}
		if len(doc.Embedding) != dimension {
			return nil, fmt.Errorf("stored vector dimension differs from its source descriptor")
		}
		for _, v := range doc.Embedding {
			if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
				return nil, fmt.Errorf("stored vector is not finite")
			}
		}
		docs = append(docs, doc)
	}
	return docs, rows.Err()
}

func (cv *ChromemVectorDB) corpusCollectionNames(ctx context.Context) ([]string, error) {
	rows, err := cv.corpus.db.QueryContext(ctx, `SELECT name FROM collections ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	return names, rows.Err()
}

// loadCorpusIndex never puts vectors from an unknown or different model into a
// queryable collection. Content remains readable while pending reindexing.
func (cv *ChromemVectorDB) loadCorpusIndex(ctx context.Context) error {
	cv.mu.Lock()
	defer cv.mu.Unlock()
	names, err := cv.corpusCollectionNames(ctx)
	if err != nil {
		return err
	}
	for _, name := range names {
		// KG's SQL tables are authoritative; retained legacy index snapshots must
		// never overwrite the fresh shared KG index or suppress its SQL rebuild.
		if name == kgsemantic.CollectionName {
			continue
		}
		col, err := cv.db.GetOrCreateCollection(name, nil, cv.embeddingFunc)
		if err != nil {
			return err
		}
		cv.registerFileIndexerCollection(name)
		docs, err := cv.corpus.documents(ctx, name)
		if err != nil {
			return err
		}
		for _, doc := range docs {
			var pending int
			if err := cv.corpus.db.QueryRowContext(ctx, `SELECT pending FROM documents WHERE collection=? AND id=?`, name, doc.ID).Scan(&pending); err != nil {
				return err
			}
			if pending == 0 && len(doc.Embedding) == int(cv.embeddingDimension.Load()) && len(doc.Embedding) > 0 && doc.Metadata["embedding_fingerprint"] == cv.EmbeddingFingerprint() {
				if err := col.AddDocument(ctx, doc); err != nil {
					return err
				}
			} else {
				if _, err := cv.corpus.db.ExecContext(ctx, `UPDATE documents SET pending=1 WHERE collection=? AND id=?`, name, doc.ID); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (cv *ChromemVectorDB) reconcileCorpus(ctx context.Context) error {
	if cv.corpus == nil {
		return nil
	}
	names, err := cv.corpusCollectionNames(ctx)
	if err != nil {
		return err
	}
	var indexErrors error
	for _, name := range names {
		if name == kgsemantic.CollectionName {
			continue
		}
		docs, err := cv.corpus.documents(ctx, name)
		if err != nil {
			return err
		}
		col, err := cv.db.GetOrCreateCollection(name, nil, cv.embeddingFunc)
		if err != nil {
			return err
		}
		for _, doc := range docs {
			if err := ctx.Err(); err != nil {
				return err
			}
			var pending int
			if err := cv.corpus.db.QueryRowContext(ctx, `SELECT pending FROM documents WHERE collection=? AND id=?`, name, doc.ID).Scan(&pending); err != nil {
				return err
			}
			if pending == 0 {
				continue
			}
			oldContent := doc.Content
			oldMeta, _ := json.Marshal(doc.Metadata)
			oldVector, _ := json.Marshal(doc.Embedding)
			if doc.Metadata["multimodal"] == "true" || doc.Metadata["index_mode"] == "multimodal" || (doc.Metadata["index_mode"] == "pdf" && strings.Contains(doc.Content, "PDF (gescannt):")) {
				if _, err := cv.corpus.db.ExecContext(ctx, `UPDATE documents SET blocked_reason='multimodal_source_reindex_required' WHERE collection=? AND id=?`, name, doc.ID); err != nil {
					return err
				}
				cv.logger.Warn("Multimodal memory retained pending source reindex", "collection", name, "doc_id", doc.ID)
				continue
			}
			doc.Embedding = nil
			// Keep the embedding result, then compare the complete canonical snapshot.
			embedded := []chromem.Document{doc}
			if err := cv.embedDocumentsOutsideLock(ctx, embedded); err != nil {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				if _, saveErr := cv.corpus.db.ExecContext(ctx, `UPDATE documents SET blocked_reason='embedding_failed' WHERE collection=? AND id=?`, name, doc.ID); saveErr != nil {
					return errors.Join(err, saveErr)
				}
				indexErrors = errors.Join(indexErrors, err)
				continue
			}
			doc = embedded[0]
			doc.Metadata = cv.addEmbeddingMetadata(doc.Metadata)
			cv.mu.Lock()
			var currentContent, currentMeta string
			var currentVector []byte
			err := cv.corpus.db.QueryRowContext(ctx, `SELECT content,metadata,vector FROM documents WHERE collection=? AND id=?`, name, doc.ID).Scan(&currentContent, &currentMeta, &currentVector)
			if errors.Is(err, sql.ErrNoRows) {
				cv.mu.Unlock()
				continue
			}
			if err == nil && currentContent == oldContent && currentMeta == string(oldMeta) && string(currentVector) == string(oldVector) {
				err = cv.persistDocumentsLocked(ctx, col, []chromem.Document{doc})
			}
			cv.mu.Unlock()
			if err != nil {
				return err
			}
		}
	}
	return indexErrors
}

func (cv *ChromemVectorDB) deleteDocumentsWithCorpus(ctx context.Context, col *chromem.Collection, where map[string]string, ids ...string) error {
	cv.mu.Lock()
	defer cv.mu.Unlock()
	return cv.deleteCorpusDocuments(ctx, col, where, ids...)
}

// persistDocumentsLocked commits the source before mutating the derived index.
// Callers hold cv.mu; pending work survives any index failure or process exit.
func (cv *ChromemVectorDB) persistDocumentsLocked(ctx context.Context, col *chromem.Collection, docs []chromem.Document) error {
	return cv.replaceCorpusDocumentsLocked(ctx, col, docs, nil)
}

func (cv *ChromemVectorDB) replaceCorpusDocumentsLocked(ctx context.Context, col *chromem.Collection, docs []chromem.Document, replaceWhere map[string]string) error {
	if cv.corpus != nil {
		var old []chromem.Document
		if len(replaceWhere) > 0 {
			var err error
			old, err = cv.corpus.documents(ctx, col.Name)
			if err != nil {
				return err
			}
		}
		tx, err := cv.corpus.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer tx.Rollback()
		for _, doc := range old {
			matches := true
			for k, v := range replaceWhere {
				matches = matches && doc.Metadata[k] == v
			}
			if matches {
				if _, err := tx.ExecContext(ctx, `DELETE FROM documents WHERE collection=? AND id=?`, col.Name, doc.ID); err != nil {
					return err
				}
			}
		}
		for _, doc := range docs {
			if err := writeCorpusDocument(ctx, tx, col.Name, doc); err != nil {
				return err
			}
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit source documents: %w", err)
		}
	}
	if len(replaceWhere) > 0 {
		if err := col.Delete(ctx, replaceWhere, nil); err != nil {
			return err
		}
	}
	for _, doc := range docs {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := col.AddDocument(ctx, doc); err != nil {
			return err
		}
	}
	if cv.corpus != nil {
		for _, doc := range docs {
			if _, err := cv.corpus.db.ExecContext(ctx, `UPDATE documents SET pending=0 WHERE collection=? AND id=?`, col.Name, doc.ID); err != nil {
				return err
			}
		}
	}
	return nil
}

func (cv *ChromemVectorDB) deleteCorpusDocuments(ctx context.Context, col *chromem.Collection, where map[string]string, ids ...string) error {
	if col == nil {
		return fmt.Errorf("derived collection is unavailable")
	}
	return cv.deleteStoredDocuments(ctx, col.Name, col, where, ids...)
}

func (cv *ChromemVectorDB) deleteStoredDocuments(ctx context.Context, collection string, col *chromem.Collection, where map[string]string, ids ...string) error {
	if len(where) == 0 && len(ids) == 0 {
		return fmt.Errorf("document deletion requires a filter or IDs")
	}
	if cv.corpus != nil {
		var docs []chromem.Document
		if len(where) > 0 {
			var err error
			docs, err = cv.corpus.documents(ctx, collection)
			if err != nil {
				return err
			}
		}
		tx, err := cv.corpus.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer tx.Rollback()
		if len(where) == 0 {
			for _, id := range ids {
				if _, err := tx.ExecContext(ctx, `DELETE FROM documents WHERE collection=? AND id=?`, collection, id); err != nil {
					return err
				}
			}
		}
		for _, doc := range docs {
			match := len(ids) == 0
			for _, id := range ids {
				match = match || id == doc.ID
			}
			for key, value := range where {
				match = match && doc.Metadata[key] == value
			}
			if match {
				if _, err := tx.ExecContext(ctx, `DELETE FROM documents WHERE collection=? AND id=?`, collection, doc.ID); err != nil {
					return err
				}
			}
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	if col == nil {
		if cv.corpus != nil {
			return nil
		}
		return fmt.Errorf("document collection is unavailable")
	}
	return col.Delete(ctx, where, nil, ids...)
}

func (cv *ChromemVectorDB) corpusDocument(ctx context.Context, collection, id string) (chromem.Document, error) {
	var doc chromem.Document
	doc.ID = id
	var metadata string
	var vector []byte
	err := cv.corpus.db.QueryRowContext(ctx, `SELECT content,metadata,vector FROM documents WHERE collection=? AND id=?`, collection, id).Scan(&doc.Content, &metadata, &vector)
	if err != nil {
		return doc, err
	}
	if err := json.Unmarshal([]byte(metadata), &doc.Metadata); err != nil {
		return doc, err
	}
	err = json.Unmarshal(vector, &doc.Embedding)
	return doc, err
}

func (cv *ChromemVectorDB) corpusIndexEligible(ctx context.Context, collection string, result chromem.Result) (bool, error) {
	var pending int
	var content, fingerprint, metadata string
	err := cv.corpus.db.QueryRowContext(ctx, `SELECT pending,content,metadata,fingerprint FROM documents WHERE collection=? AND id=?`, collection, result.ID).Scan(&pending, &content, &metadata, &fingerprint)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	actualMeta, err := json.Marshal(result.Metadata)
	if err != nil {
		return false, err
	}
	return pending == 0 && content == result.Content && metadata == string(actualMeta) && fingerprint == cv.EmbeddingFingerprint(), nil
}
