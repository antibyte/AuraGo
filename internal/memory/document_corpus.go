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

const corpusPageSize = 128
const corpusSQLChunkSize = 300

type corpusDocumentSnapshot struct {
	document      chromem.Document
	pending       int
	metadata      string
	vectorJSON    []byte
	dimension     int
	fingerprint   string
	blockedReason string
}

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
func (cv *ChromemVectorDB) BindMemoryMetadata(stm *SQLiteMemory) {
	cv.fileIndexMemory.Store(stm)
	if stm != nil {
		stm.bindArchiveVector(cv)
	}
}

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

func (c *documentCorpus) documentPage(ctx context.Context, collection, afterID string, pendingOnly bool) ([]corpusDocumentSnapshot, error) {
	query := `SELECT id,content,metadata,vector,dimension,fingerprint,pending,blocked_reason FROM documents WHERE collection=? AND id>? ORDER BY id LIMIT ?`
	args := []any{collection, afterID, corpusPageSize}
	if pendingOnly {
		query = `SELECT id,content,metadata,vector,dimension,fingerprint,pending,blocked_reason FROM documents WHERE collection=? AND id>? AND pending<>0 ORDER BY id LIMIT ?`
	}
	rows, err := c.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return scanCorpusSnapshots(rows)
}

// blockedDocumentPage pages the pending documents of one collection that are
// blocked for a single reason, e.g. a failed embedding.
func (c *documentCorpus) blockedDocumentPage(ctx context.Context, collection, afterID, reason string) ([]corpusDocumentSnapshot, error) {
	rows, err := c.db.QueryContext(ctx, `SELECT id,content,metadata,vector,dimension,fingerprint,pending,blocked_reason FROM documents WHERE collection=? AND id>? AND pending<>0 AND blocked_reason=? ORDER BY id LIMIT ?`, collection, afterID, reason, corpusPageSize)
	if err != nil {
		return nil, err
	}
	return scanCorpusSnapshots(rows)
}

// scanCorpusSnapshots reads and closes a page of documents selected in the
// column order id,content,metadata,vector,dimension,fingerprint,pending,blocked_reason.
func scanCorpusSnapshots(rows *sql.Rows) ([]corpusDocumentSnapshot, error) {
	defer rows.Close()
	var docs []corpusDocumentSnapshot
	for rows.Next() {
		var snapshot corpusDocumentSnapshot
		var vector []byte
		if err := rows.Scan(&snapshot.document.ID, &snapshot.document.Content, &snapshot.metadata, &vector, &snapshot.dimension, &snapshot.fingerprint, &snapshot.pending, &snapshot.blockedReason); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(snapshot.metadata), &snapshot.document.Metadata); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(vector, &snapshot.document.Embedding); err != nil {
			return nil, err
		}
		if len(snapshot.document.Embedding) != snapshot.dimension {
			return nil, fmt.Errorf("stored vector dimension differs from its source descriptor")
		}
		for _, v := range snapshot.document.Embedding {
			if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
				return nil, fmt.Errorf("stored vector is not finite")
			}
		}
		snapshot.vectorJSON = append([]byte(nil), vector...)
		docs = append(docs, snapshot)
	}
	return docs, rows.Err()
}

func corpusMatchesMetadata(metadata map[string]string, where map[string]string) bool {
	for key, value := range where {
		if metadata[key] != value {
			return false
		}
	}
	return true
}

func corpusPlaceholders(count int) string {
	return strings.TrimSuffix(strings.Repeat("?,", count), ",")
}

type corpusMetadataSnapshot struct {
	id       string
	metadata map[string]string
}

func corpusMetadataPage(ctx context.Context, tx *sql.Tx, collection, afterID string) ([]corpusMetadataSnapshot, error) {
	rows, err := tx.QueryContext(ctx, `SELECT id,metadata FROM documents WHERE collection=? AND id>? ORDER BY id LIMIT ?`, collection, afterID, corpusPageSize)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	page := make([]corpusMetadataSnapshot, 0, corpusPageSize)
	for rows.Next() {
		var item corpusMetadataSnapshot
		var raw string
		if err := rows.Scan(&item.id, &raw); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(raw), &item.metadata); err != nil {
			return nil, fmt.Errorf("decode corpus metadata %q: %w", item.id, err)
		}
		page = append(page, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return page, rows.Close()
}

type corpusSQLExecer interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func deleteCorpusIDs(ctx context.Context, execer corpusSQLExecer, collection string, ids []string) error {
	for start := 0; start < len(ids); start += corpusSQLChunkSize {
		end := min(start+corpusSQLChunkSize, len(ids))
		chunk := ids[start:end]
		args := make([]any, 0, len(chunk)+1)
		args = append(args, collection)
		for _, id := range chunk {
			args = append(args, id)
		}
		if _, err := execer.ExecContext(ctx, `DELETE FROM documents WHERE collection=? AND id IN (`+corpusPlaceholders(len(chunk))+`)`, args...); err != nil {
			return fmt.Errorf("delete corpus document batch: %w", err)
		}
	}
	return nil
}

func updateCorpusPending(ctx context.Context, execer corpusSQLExecer, collection string, ids []string, pending int) error {
	for start := 0; start < len(ids); start += corpusSQLChunkSize {
		end := min(start+corpusSQLChunkSize, len(ids))
		chunk := ids[start:end]
		args := make([]any, 0, len(chunk)+2)
		args = append(args, pending, collection)
		for _, id := range chunk {
			args = append(args, id)
		}
		if _, err := execer.ExecContext(ctx, `UPDATE documents SET pending=? WHERE collection=? AND id IN (`+corpusPlaceholders(len(chunk))+`)`, args...); err != nil {
			return fmt.Errorf("update corpus document batch status: %w", err)
		}
	}
	return nil
}

func deleteCorpusMatchingMetadata(ctx context.Context, tx *sql.Tx, collection string, where map[string]string) error {
	afterID := ""
	for {
		page, err := corpusMetadataPage(ctx, tx, collection, afterID)
		if err != nil {
			return fmt.Errorf("page corpus metadata for deletion: %w", err)
		}
		if len(page) == 0 {
			return nil
		}
		ids := make([]string, 0, len(page))
		for _, item := range page {
			if corpusMatchesMetadata(item.metadata, where) {
				ids = append(ids, item.id)
			}
		}
		if err := deleteCorpusIDs(ctx, tx, collection, ids); err != nil {
			return err
		}
		if len(page) < corpusPageSize {
			return nil
		}
		afterID = page[len(page)-1].id
	}
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
		afterID := ""
		for {
			page, err := cv.corpus.documentPage(ctx, name, afterID, false)
			if err != nil {
				return fmt.Errorf("page corpus collection %s: %w", name, err)
			}
			if len(page) == 0 {
				break
			}
			var docs []chromem.Document
			var invalidate []string
			for _, snapshot := range page {
				doc := snapshot.document
				if snapshot.pending == 0 && len(doc.Embedding) == int(cv.embeddingDimension.Load()) && len(doc.Embedding) > 0 && doc.Metadata["embedding_fingerprint"] == cv.EmbeddingFingerprint() {
					docs = append(docs, doc)
				} else if snapshot.pending == 0 {
					invalidate = append(invalidate, doc.ID)
				}
			}
			if len(docs) > 0 {
				if err := col.AddDocuments(ctx, docs, 8); err != nil {
					return fmt.Errorf("load corpus vectors for %s: %w", name, err)
				}
			}
			if len(invalidate) > 0 {
				tx, err := cv.corpus.db.BeginTx(ctx, nil)
				if err != nil {
					return err
				}
				if _, err := tx.ExecContext(ctx, `UPDATE documents SET pending=1 WHERE collection=? AND id IN (`+corpusPlaceholders(len(invalidate))+`)`, append([]any{name}, stringsToAny(invalidate)...)...); err != nil {
					tx.Rollback()
					return err
				}
				if err := tx.Commit(); err != nil {
					return err
				}
			}
			afterID = page[len(page)-1].document.ID
			if len(page) < corpusPageSize {
				break
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
		col, err := cv.db.GetOrCreateCollection(name, nil, cv.embeddingFunc)
		if err != nil {
			return err
		}
		afterID := ""
		for {
			page, err := cv.corpus.documentPage(ctx, name, afterID, true)
			if err != nil {
				return fmt.Errorf("page pending corpus collection %s: %w", name, err)
			}
			if len(page) == 0 {
				break
			}
			var blockedMultimodal, blockedEmbedding []string
			var candidates []corpusReconcileCandidate
			for _, snapshot := range page {
				if err := ctx.Err(); err != nil {
					return err
				}
				doc := snapshot.document
				if doc.Metadata["multimodal"] == "true" || doc.Metadata["index_mode"] == "multimodal" || (doc.Metadata["index_mode"] == "pdf" && strings.Contains(doc.Content, "PDF (gescannt):")) {
					blockedMultimodal = append(blockedMultimodal, doc.ID)
					cv.logger.Warn("Multimodal memory retained pending source reindex", "collection", name, "doc_id", doc.ID)
					continue
				}
				doc.Embedding = nil
				embedded := []chromem.Document{doc}
				if err := cv.embedDocumentsOutsideLock(ctx, embedded); err != nil {
					if ctx.Err() != nil {
						return ctx.Err()
					}
					blockedEmbedding = append(blockedEmbedding, doc.ID)
					indexErrors = errors.Join(indexErrors, err)
					continue
				}
				doc = embedded[0]
				doc.Metadata = cv.addEmbeddingMetadata(doc.Metadata)
				candidates = append(candidates, corpusReconcileCandidate{snapshot: snapshot, document: doc})
			}
			if err := cv.setCorpusBlocked(ctx, name, blockedMultimodal, "multimodal_source_reindex_required"); err != nil {
				return err
			}
			if err := cv.setCorpusBlocked(ctx, name, blockedEmbedding, blockedEmbeddingReason); err != nil {
				return errors.Join(indexErrors, err)
			}
			if err := cv.persistReconcileCandidates(ctx, name, col, candidates); err != nil {
				return err
			}
			afterID = page[len(page)-1].document.ID
			if len(page) < corpusPageSize {
				break
			}
		}
	}
	return indexErrors
}

// persistReconcileCandidates publishes freshly embedded documents, but only those
// whose corpus row is still exactly what was read before the embedding call: a
// concurrent writer's newer version must win.
func (cv *ChromemVectorDB) persistReconcileCandidates(ctx context.Context, collection string, col *chromem.Collection, candidates []corpusReconcileCandidate) error {
	if len(candidates) == 0 {
		return nil
	}
	cv.mu.Lock()
	defer cv.mu.Unlock()
	unchanged, err := cv.currentCorpusSnapshots(ctx, collection, candidates)
	if err != nil || len(unchanged) == 0 {
		return err
	}
	docs := make([]chromem.Document, 0, len(unchanged))
	for _, item := range unchanged {
		docs = append(docs, item.document)
	}
	return cv.persistDocumentsLocked(ctx, col, docs)
}

func stringsToAny(values []string) []any {
	result := make([]any, len(values))
	for i, value := range values {
		result[i] = value
	}
	return result
}

func (cv *ChromemVectorDB) setCorpusBlocked(ctx context.Context, collection string, ids []string, reason string) error {
	if len(ids) == 0 {
		return nil
	}
	tx, err := cv.corpus.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE documents SET blocked_reason=? WHERE collection=? AND id IN (`+corpusPlaceholders(len(ids))+`)`, append([]any{reason, collection}, stringsToAny(ids)...)...); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

type corpusReconcileCandidate struct {
	snapshot corpusDocumentSnapshot
	document chromem.Document
}

func (cv *ChromemVectorDB) currentCorpusSnapshots(ctx context.Context, collection string, candidates []corpusReconcileCandidate) ([]corpusReconcileCandidate, error) {
	byID := make(map[string]corpusReconcileCandidate, len(candidates))
	ids := make([]string, len(candidates))
	for i, item := range candidates {
		ids[i] = item.snapshot.document.ID
		byID[ids[i]] = item
	}
	unchanged := make([]corpusReconcileCandidate, 0, len(candidates))
	for start := 0; start < len(ids); start += corpusSQLChunkSize {
		end := min(start+corpusSQLChunkSize, len(ids))
		chunk := ids[start:end]
		args := append([]any{collection}, stringsToAny(chunk)...)
		rows, err := cv.corpus.db.QueryContext(ctx, `SELECT id,content,metadata,vector,dimension,fingerprint,pending,blocked_reason FROM documents WHERE collection=? AND id IN (`+corpusPlaceholders(len(chunk))+`)`, args...)
		if err != nil {
			return nil, fmt.Errorf("recheck current corpus snapshot: %w", err)
		}
		current := make(map[string]corpusDocumentSnapshot, len(chunk))
		for rows.Next() {
			var snapshot corpusDocumentSnapshot
			if err := rows.Scan(&snapshot.document.ID, &snapshot.document.Content, &snapshot.metadata, &snapshot.vectorJSON, &snapshot.dimension, &snapshot.fingerprint, &snapshot.pending, &snapshot.blockedReason); err != nil {
				rows.Close()
				return nil, fmt.Errorf("scan current corpus snapshot: %w", err)
			}
			current[snapshot.document.ID] = snapshot
		}
		rowsErr := rows.Err()
		closeErr := rows.Close()
		if err := errors.Join(rowsErr, closeErr); err != nil {
			return nil, fmt.Errorf("finish current corpus snapshot: %w", err)
		}
		for _, id := range chunk {
			actual, ok := current[id]
			if !ok {
				continue
			}
			expected := byID[id].snapshot
			if actual.document.Content == expected.document.Content && actual.metadata == expected.metadata && string(actual.vectorJSON) == string(expected.vectorJSON) && actual.dimension == expected.dimension && actual.fingerprint == expected.fingerprint && actual.pending == expected.pending && actual.blockedReason == expected.blockedReason {
				unchanged = append(unchanged, byID[id])
			}
		}
	}
	return unchanged, nil
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
		tx, err := cv.corpus.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer tx.Rollback()
		if len(replaceWhere) > 0 {
			afterID := ""
			for {
				page, err := corpusMetadataPage(ctx, tx, col.Name, afterID)
				if err != nil {
					return fmt.Errorf("page corpus replacement metadata: %w", err)
				}
				if len(page) == 0 {
					break
				}
				ids := make([]string, 0, len(page))
				for _, item := range page {
					if corpusMatchesMetadata(item.metadata, replaceWhere) {
						ids = append(ids, item.id)
					}
				}
				if err := deleteCorpusIDs(ctx, tx, col.Name, ids); err != nil {
					return err
				}
				afterID = page[len(page)-1].id
				if len(page) < corpusPageSize {
					break
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
	if len(docs) > 0 {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := col.AddDocuments(ctx, docs, 8); err != nil {
			return err
		}
	}
	if cv.corpus != nil && len(docs) > 0 {
		tx, err := cv.corpus.db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin corpus publication marker transaction: %w", err)
		}
		ids := make([]string, 0, len(docs))
		for _, doc := range docs {
			ids = append(ids, doc.ID)
		}
		if err := updateCorpusPending(ctx, tx, col.Name, ids, 0); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("publish corpus document markers: %w", err)
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
		tx, err := cv.corpus.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer tx.Rollback()
		switch {
		case len(ids) == 0:
			if err := deleteCorpusMatchingMetadata(ctx, tx, collection, where); err != nil {
				return err
			}
		case len(where) == 0:
			if err := deleteCorpusIDs(ctx, tx, collection, ids); err != nil {
				return err
			}
		default:
			for start := 0; start < len(ids); start += corpusSQLChunkSize {
				end := min(start+corpusSQLChunkSize, len(ids))
				chunk := ids[start:end]
				args := append([]any{collection}, stringsToAny(chunk)...)
				rows, err := tx.QueryContext(ctx, `SELECT id,metadata FROM documents WHERE collection=? AND id IN (`+corpusPlaceholders(len(chunk))+`)`, args...)
				if err != nil {
					return fmt.Errorf("read requested corpus deletion metadata: %w", err)
				}
				matched := make([]string, 0, len(chunk))
				for rows.Next() {
					var id, raw string
					var metadata map[string]string
					if err := rows.Scan(&id, &raw); err != nil {
						rows.Close()
						return err
					}
					if err := json.Unmarshal([]byte(raw), &metadata); err != nil {
						rows.Close()
						return fmt.Errorf("decode corpus deletion metadata %q: %w", id, err)
					}
					if corpusMatchesMetadata(metadata, where) {
						matched = append(matched, id)
					}
				}
				rowsErr := rows.Err()
				closeErr := rows.Close()
				if err := errors.Join(rowsErr, closeErr); err != nil {
					return fmt.Errorf("finish requested corpus deletion metadata: %w", err)
				}
				if err := deleteCorpusIDs(ctx, tx, collection, matched); err != nil {
					return err
				}
			}
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit corpus deletions: %w", err)
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
