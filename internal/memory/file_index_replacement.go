package memory

import (
	"aurago/internal/chunking"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/philippgille/chromem-go"
)

const fileReplacementPrefix = "file_index.replacement."

type fileDocumentReceipt struct {
	ID           string
	Hash         string
	DeleteFailed bool
}

type fileReplacement struct {
	Version    int
	Path       string
	Collection string
	Old        []fileDocumentReceipt
	New        []fileDocumentReceipt
}

// ReplaceIndexedFile stages a fresh, invisible generation before publishing its SQLite pointer.
func (cv *ChromemVectorDB) ReplaceIndexedFile(ctx context.Context, stm *SQLiteMemory, path, collection, concept, content string, embedding []float32, options chunking.Options, metadata map[string]string, state FileIndexState) ([]string, error) {
	if stm == nil || stm.db == nil {
		return nil, fmt.Errorf("file replacement requires a metadata store")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	done, err := cv.beginTrackedOperation(&cv.storeWg)
	if err != nil {
		return nil, err
	}
	defer done()
	if err := cv.requireReadyForStore(); err != nil {
		return nil, err
	}
	cv.fileWriteMu.Lock()
	defer cv.fileWriteMu.Unlock()
	cv.fileIndexMemory.Store(stm)
	cv.registerFileIndexerCollection(collection)
	cv.mu.Lock()
	col, err := cv.db.GetOrCreateCollection(collection, nil, cv.embeddingFunc)
	cv.mu.Unlock()
	if err != nil {
		return nil, err
	}
	if err := cv.recoverIndexedFiles(ctx, stm, col, collection); err != nil {
		cv.logger.Warn("File index recovery remains pending", "collection", collection, "error", err)
	}
	if embedding != nil {
		if err := cv.validateStoredEmbedding(embedding); err != nil {
			return nil, err
		}
	}
	chunks := []chunking.Chunk{{Text: content, Index: 0, Total: 1}}
	options = chunking.NormalizeOptionsWithDefaults(options)
	if embedding == nil {
		chunks, err = chunking.ChunkText(content, options)
		if err != nil {
			return nil, err
		}
	}
	generation := rand.Text()
	docs := make([]chromem.Document, 0, len(chunks))
	for _, c := range chunks {
		meta := make(map[string]string, len(metadata)+8)
		for k, v := range metadata {
			meta[k] = v
		}
		meta["source_path"], meta["collection"], meta["source_type"] = path, collection, "file_indexer"
		if embedding != nil {
			meta["multimodal"] = "true"
		}
		meta["concept"], meta["timestamp"] = concept, fmt.Sprint(time.Now().Unix())
		meta["chunk_index"], meta["chunk_total"], meta["chunker"] = fmt.Sprint(c.Index+1), fmt.Sprint(c.Total), options.Strategy
		body := buildContentString(concept, c.Text)
		if len(chunks) > 1 {
			body = fmt.Sprintf("%s (%d/%d)\n\n%s", concept, c.Index+1, c.Total, c.Text)
		}
		docs = append(docs, chromem.Document{ID: fmt.Sprintf("file_%s_%d", generation, c.Index), Content: body, Embedding: embedding, Metadata: cv.addEmbeddingMetadata(meta)})
	}
	return cv.replaceIndexedDocuments(ctx, stm, col, path, collection, state, docs)
}

// RecoverIndexedFiles retries only journaled replacements; it never guesses at orphan ownership.
func (cv *ChromemVectorDB) RecoverIndexedFiles(ctx context.Context, stm *SQLiteMemory, collection string) error {
	if stm == nil || stm.db == nil {
		return fmt.Errorf("file recovery requires a metadata store")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	done, err := cv.beginTrackedOperation(&cv.storeWg)
	if err != nil {
		return err
	}
	defer done()
	cv.fileWriteMu.Lock()
	defer cv.fileWriteMu.Unlock()
	cv.fileIndexMemory.Store(stm)
	cv.mu.RLock()
	col := cv.db.GetCollection(collection, nil)
	cv.mu.RUnlock()
	if col == nil {
		return nil
	}
	return cv.recoverIndexedFiles(ctx, stm, col, collection)
}

func (cv *ChromemVectorDB) replaceIndexedDocuments(ctx context.Context, stm *SQLiteMemory, col *chromem.Collection, path, collection string, state FileIndexState, docs []chromem.Document) ([]string, error) {
	if stm == nil {
		return nil, fmt.Errorf("file replacement requires a metadata store")
	}
	if err := cv.embedDocumentsOutsideLock(ctx, docs); err != nil {
		return nil, err
	}
	ctx, release, err := stm.BeginMemoryPublication(ctx)
	if err != nil {
		return nil, err
	}
	defer release()
	before, err := stm.GetFileIndexState(path, collection)
	if err != nil {
		return nil, err
	}
	oldIDs, err := stm.GetFileEmbeddingDocIDs(path, collection)
	if err != nil {
		return nil, err
	}
	if ids, reindexed, err := cv.reindexUnchangedFileDocuments(ctx, stm, col, path, collection, before, state, oldIDs, docs); reindexed || err != nil {
		return ids, err
	}
	key := fileReplacementPrefix + contentSHA256(collection) + "." + rand.Text()
	record := fileReplacement{Version: 1, Path: path, Collection: collection}
	for _, id := range oldIDs {
		doc, e := cv.readCollectionDocument(ctx, col, id)
		if e != nil {
			continue
		} // chromem's GetByID has no I/O: absent means absent.
		if doc.Metadata["source_path"] != path && doc.Metadata["path"] != path {
			continue
		}
		record.Old = append(record.Old, fileDocumentReceipt{ID: id, Hash: contentSHA256(doc.Content)})
	}
	ids := make([]string, 0, len(docs))
	for i := range docs {
		if docs[i].Metadata == nil {
			docs[i].Metadata = map[string]string{}
		}
		docs[i].Metadata["index_generation"] = key
		ids = append(ids, docs[i].ID)
		record.New = append(record.New, fileDocumentReceipt{ID: docs[i].ID, Hash: contentSHA256(docs[i].Content)})
	}
	if err := stm.saveFileReplacement(key, record); err != nil {
		return nil, err
	}
	if len(docs) > 0 {
		writeCtx, cancel := context.WithTimeout(ctx, calculateBatchTimeout(len(docs)))
		writeErr := cv.addDocumentsWithPrecomputedEmbeddings(writeCtx, col, docs)
		cancel()
		if err := writeErr; err != nil {
			return ids, errors.Join(fmt.Errorf("stage file generation: %w", err), cv.finishFileReplacement(ctx, stm, col, key, record))
		}
	}
	if err := stm.PublishFileIndex(path, collection, before, oldIDs, state, ids); err != nil {
		// A failed commit can be ambiguous. Re-read the active IDs before deciding what to retain.
		return ids, errors.Join(err, cv.finishFileReplacement(ctx, stm, col, key, record))
	}
	return ids, cv.finishFileReplacement(ctx, stm, col, key, record)
}

func (s *SQLiteMemory) saveFileReplacement(key string, record fileReplacement) error {
	value, err := json.Marshal(record)
	if err != nil {
		return err
	}
	return s.SetMemoryMaintenanceState(key, string(value))
}

func (cv *ChromemVectorDB) recoverIndexedFiles(ctx context.Context, stm *SQLiteMemory, col *chromem.Collection, collection string) error {
	if stm == nil {
		return fmt.Errorf("file recovery requires a metadata store")
	}
	ctx, release, err := stm.BeginMemoryPublication(ctx)
	if err != nil {
		return err
	}
	defer release()
	rows, err := stm.db.QueryContext(ctx, `SELECT key,value FROM memory_maintenance_meta WHERE key LIKE ? ORDER BY key LIMIT 100`, fileReplacementPrefix+contentSHA256(collection)+".%")
	if err != nil {
		return err
	}
	type entry struct {
		key    string
		record fileReplacement
	}
	var entries []entry
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			rows.Close()
			return err
		}
		var record fileReplacement
		if err := json.Unmarshal([]byte(value), &record); err != nil || record.Version != 1 {
			rows.Close()
			return fmt.Errorf("invalid file recovery record %s", key)
		}
		if record.Collection == collection {
			entries = append(entries, entry{key, record})
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, e := range entries {
		if ctx.Err() != nil {
			return errors.Join(err, ctx.Err())
		}
		err = errors.Join(err, cv.finishFileReplacement(ctx, stm, col, e.key, e.record))
	}
	return err
}

func (cv *ChromemVectorDB) finishFileReplacement(ctx context.Context, stm *SQLiteMemory, col *chromem.Collection, key string, record fileReplacement) error {
	tx, err := stm.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// Hold SQLite's write lock while testing references and deleting proven owned vectors.
	if _, err = tx.ExecContext(ctx, `UPDATE memory_maintenance_meta SET value=value WHERE key=?`, key); err != nil {
		return err
	}
	active, err := fileIDsInTransaction(tx, record.Path, record.Collection)
	if err != nil {
		return err
	}
	newIDs := make([]string, 0, len(record.New))
	for _, r := range record.New {
		newIDs = append(newIDs, r.ID)
	}
	slices.Sort(newIDs)
	cleanup := record.New
	if slices.Equal(active, newIDs) {
		cleanup = record.Old
	}
	for i := range cleanup {
		r := &cleanup[i]
		var referenced bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM file_embedding_docs WHERE doc_id=?) OR EXISTS(SELECT 1 FROM memory_meta WHERE doc_id=?) OR EXISTS(SELECT 1 FROM memory_extraction_sources WHERE doc_id=?) OR EXISTS(SELECT 1 FROM memory_curation_events WHERE doc_id=?) OR EXISTS(SELECT 1 FROM memory_conflicts WHERE doc_id_left=? OR doc_id_right=?)`, r.ID, r.ID, r.ID, r.ID, r.ID, r.ID).Scan(&referenced); err != nil {
			return err
		}
		if referenced {
			continue
		}
		doc, getErr := cv.readCollectionDocument(ctx, col, r.ID)
		if getErr != nil {
			if r.DeleteFailed {
				return fmt.Errorf("retain failed vector deletion receipt %s", r.ID)
			}
			continue
		}
		if contentSHA256(doc.Content) != r.Hash {
			continue
		}
		if doc.Metadata["source_path"] != record.Path && doc.Metadata["path"] != record.Path {
			continue
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := cv.deleteDocumentsWithCorpus(ctx, col, nil, r.ID); err != nil {
			r.DeleteFailed = true
			value, _ := json.Marshal(record)
			if _, e := tx.ExecContext(context.WithoutCancel(ctx), `UPDATE memory_maintenance_meta SET value=? WHERE key=?`, string(value), key); e != nil {
				return errors.Join(err, e)
			}
			return errors.Join(err, tx.Commit())
		}
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM memory_maintenance_meta WHERE key=?`, key); err != nil {
		return err
	}
	return tx.Commit()
}

func fileIDsInTransaction(tx *sql.Tx, path, collection string) ([]string, error) {
	rows, err := tx.Query(`SELECT doc_id FROM file_embedding_docs WHERE file_path=? AND collection=? ORDER BY doc_id`, path, collection)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// PublishFileIndex compares the complete prior pointer before atomically installing a generation.
func (s *SQLiteMemory) PublishFileIndex(path, collection string, before FileIndexState, oldIDs []string, after FileIndexState, newIDs []string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`UPDATE file_indices SET file_path=file_path WHERE file_path=? AND collection IN (?, '')`, path, collection); err != nil {
		return err
	}
	var current FileIndexState
	actualCollection := collection
	err = tx.QueryRow(`SELECT last_modified,COALESCE(content_hash,''),COALESCE(index_fingerprint,'') FROM file_indices WHERE file_path=? AND collection=?`, path, collection).Scan(&current.LastModified, &current.ContentHash, &current.IndexFingerprint)
	if errors.Is(err, sql.ErrNoRows) {
		actualCollection = ""
		err = tx.QueryRow(`SELECT last_modified,COALESCE(content_hash,''),COALESCE(index_fingerprint,'') FROM file_indices WHERE file_path=? AND collection=''`, path).Scan(&current.LastModified, &current.ContentHash, &current.IndexFingerprint)
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	actualIDs, err := fileIDsInTransaction(tx, path, actualCollection)
	if err != nil {
		return err
	}
	wantIDs := slices.Clone(oldIDs)
	slices.Sort(wantIDs)
	if !current.LastModified.Equal(before.LastModified) || current.ContentHash != before.ContentHash || current.IndexFingerprint != before.IndexFingerprint || !slices.Equal(actualIDs, wantIDs) {
		return fmt.Errorf("file index changed during replacement")
	}
	if _, err := tx.Exec(`INSERT INTO file_indices(file_path,collection,last_modified,content_hash,index_fingerprint) VALUES(?,?,?,?,?) ON CONFLICT(file_path,collection) DO UPDATE SET last_modified=excluded.last_modified,content_hash=excluded.content_hash,index_fingerprint=excluded.index_fingerprint`, path, collection, after.LastModified, after.ContentHash, after.IndexFingerprint); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM file_embedding_docs WHERE file_path=? AND collection IN (?,?)`, path, collection, actualCollection); err != nil {
		return err
	}
	for _, id := range newIDs {
		if strings.TrimSpace(id) == "" {
			return fmt.Errorf("empty document ID in file generation")
		}
		if _, err := tx.Exec(`INSERT INTO file_embedding_docs(file_path,collection,doc_id) VALUES(?,?,?)`, path, collection, id); err != nil {
			return err
		}
	}
	if actualCollection == "" && collection != "" {
		if _, err := tx.Exec(`DELETE FROM file_indices WHERE file_path=? AND collection=''`, path); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (cv *ChromemVectorDB) indexedDocumentVisible(ctx context.Context, id string, metadata map[string]string) (bool, error) {
	if metadata["index_generation"] == "" {
		return true, nil
	}
	stm := cv.fileIndexMemory.Load()
	if stm == nil {
		return false, fmt.Errorf("file index metadata unavailable")
	}
	path := metadata["source_path"]
	if path == "" {
		path = metadata["path"]
	}
	var active bool
	err := stm.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM file_embedding_docs WHERE file_path=? AND collection=? AND doc_id=?)`, filepath.Clean(path), metadata["collection"], id).Scan(&active)
	return active, err
}

func (cv *ChromemVectorDB) queryVisibleCollection(ctx context.Context, col *chromem.Collection, embedding []float32, topK int) ([]chromem.Result, error) {
	return cv.queryVisibleCollectionWithBackfillHook(ctx, col, embedding, topK, nil)
}

// queryVisibleCollectionWithBackfillHook keeps the test seam narrow: callers
// normally leave afterBackfill nil, while focused tests can change metadata
// between candidate discovery and the final grouped eligibility validation.
func (cv *ChromemVectorDB) queryVisibleCollectionWithBackfillHook(ctx context.Context, col *chromem.Collection, embedding []float32, topK int, afterBackfill func(k int)) ([]chromem.Result, error) {
	count := col.Count()
	if topK <= 0 || count == 0 {
		return nil, nil
	}
	type checkedResult struct {
		content  string
		metadata string
		active   bool
	}
	checked := make(map[string]checkedResult)
	k := min(topK, count)
	for {
		results, err := col.QueryEmbedding(ctx, embedding, k, nil, nil)
		if err != nil {
			return nil, err
		}
		newResults := make([]chromem.Result, 0, len(results))
		newIndexes := make([]int, 0, len(results))
		metadataJSON := make([]string, len(results))
		for i, result := range results {
			encoded, _ := json.Marshal(result.Metadata)
			metadataJSON[i] = string(encoded)
			previous, ok := checked[result.ID]
			if ok && previous.content == result.Content && previous.metadata == metadataJSON[i] {
				continue
			}
			newResults = append(newResults, result)
			newIndexes = append(newIndexes, i)
		}
		var visibilityErr error
		if len(newResults) > 0 {
			newEligibility, err := cv.visibleCandidates(ctx, col.Name, newResults)
			visibilityErr = err
			for i, result := range newResults {
				index := newIndexes[i]
				checked[result.ID] = checkedResult{content: result.Content, metadata: metadataJSON[index], active: newEligibility[i]}
			}
		}
		if afterBackfill != nil {
			afterBackfill(k)
		}
		visible := make([]chromem.Result, 0, len(results))
		for _, result := range results {
			if checked[result.ID].active {
				visible = append(visible, result)
			}
		}
		if visibilityErr != nil || len(visible) >= topK || k == count {
			// Eligibility may change while the bounded backfill loop is running.
			// Recheck the candidate pool as one grouped operation before returning;
			// this keeps archive, generation, and pending-source state authoritative
			// without restoring per-result SQL lookups on every expansion round.
			finalEligibility, finalErr := cv.visibleCandidates(ctx, col.Name, visible)
			finalVisible := make([]chromem.Result, 0, len(visible))
			for i, result := range visible {
				if finalEligibility[i] {
					finalVisible = append(finalVisible, result)
				}
				if state, ok := checked[result.ID]; ok {
					state.active = finalEligibility[i]
					checked[result.ID] = state
				}
			}
			if finalErr != nil || visibilityErr != nil {
				return finalVisible[:min(topK, len(finalVisible))], errors.Join(visibilityErr, finalErr)
			}
			if len(finalVisible) >= topK || k == count {
				return finalVisible[:min(topK, len(finalVisible))], nil
			}
		}
		k = min(count, k*2)
	}
}

// visibleCandidates checks current generation, archive, and corpus state in
// bounded batches. It keeps the same candidate order and error boundary as the
// former per-result lookups.
func (cv *ChromemVectorDB) visibleCandidates(ctx context.Context, collection string, results []chromem.Result) ([]bool, error) {
	type candidateState struct {
		active bool
		err    error
	}
	states := make([]candidateState, len(results))
	for i := range states {
		states[i].active = true
	}
	var allErr error

	stm := cv.fileIndexMemory.Load()
	type generationKey struct{ path, collection, id string }
	generationCandidates := make([]int, 0, len(results))
	keys := make([]generationKey, len(results))
	for i, result := range results {
		if result.Metadata["index_generation"] == "" {
			continue
		}
		if stm == nil {
			states[i].active = false
			states[i].err = fmt.Errorf("file index metadata unavailable")
			allErr = errors.Join(allErr, states[i].err)
			continue
		}
		path := result.Metadata["source_path"]
		if path == "" {
			path = result.Metadata["path"]
		}
		keys[i] = generationKey{path: filepath.Clean(path), collection: result.Metadata["collection"], id: result.ID}
		generationCandidates = append(generationCandidates, i)
	}
	for start := 0; start < len(generationCandidates); start += 200 {
		end := min(start+200, len(generationCandidates))
		chunk := generationCandidates[start:end]
		query := `SELECT file_path,collection,doc_id FROM file_embedding_docs WHERE `
		args := make([]any, 0, len(chunk)*3)
		for j, index := range chunk {
			if j > 0 {
				query += ` OR `
			}
			query += `(file_path=? AND collection=? AND doc_id=?)`
			key := keys[index]
			args = append(args, key.path, key.collection, key.id)
		}
		rows, err := stm.db.QueryContext(ctx, query, args...)
		if err != nil {
			wrapped := fmt.Errorf("check current file generations: %w", err)
			for _, index := range chunk {
				states[index].active = false
				states[index].err = wrapped
				allErr = errors.Join(allErr, wrapped)
			}
			continue
		}
		found := make(map[generationKey]struct{}, len(chunk))
		for rows.Next() {
			var key generationKey
			if err := rows.Scan(&key.path, &key.collection, &key.id); err != nil {
				wrapped := fmt.Errorf("read current file generations: %w", err)
				allErr = errors.Join(allErr, wrapped)
				for _, index := range chunk {
					states[index].active = false
					states[index].err = wrapped
				}
				break
			}
			found[key] = struct{}{}
		}
		rowsErr := rows.Err()
		closeErr := rows.Close()
		if err := errors.Join(rowsErr, closeErr); err != nil {
			wrapped := fmt.Errorf("finish current file generation lookup: %w", err)
			allErr = errors.Join(allErr, wrapped)
			for _, index := range chunk {
				states[index].active = false
				states[index].err = wrapped
			}
			continue
		}
		for _, index := range chunk {
			if _, ok := found[keys[index]]; !ok {
				states[index].active = false
			}
		}
	}

	if collection == "aurago_memories" {
		archiveCandidates := make([]int, 0, len(results))
		for i := range results {
			if states[i].active {
				archiveCandidates = append(archiveCandidates, i)
			}
		}
		if stm == nil {
			if cv.corpus != nil {
				for _, index := range archiveCandidates {
					states[index].active = false
					states[index].err = fmt.Errorf("memory metadata is unavailable")
					allErr = errors.Join(allErr, states[index].err)
				}
			}
		} else {
			for start := 0; start < len(archiveCandidates); start += corpusSQLChunkSize {
				end := min(start+corpusSQLChunkSize, len(archiveCandidates))
				chunk := archiveCandidates[start:end]
				args := make([]any, 0, len(chunk))
				for _, index := range chunk {
					args = append(args, results[index].ID)
				}
				rows, err := stm.db.QueryContext(ctx, `SELECT doc_id FROM memory_meta WHERE doc_id IN (`+corpusPlaceholders(len(chunk))+`) AND (verification_status='archived' OR COALESCE(archived_at,'')<>'')`, args...)
				if err != nil {
					wrapped := fmt.Errorf("check archived memory candidates: %w", err)
					for _, index := range chunk {
						states[index].active = false
						states[index].err = wrapped
						allErr = errors.Join(allErr, wrapped)
					}
					continue
				}
				archived := make(map[string]struct{}, len(chunk))
				for rows.Next() {
					var id string
					if err := rows.Scan(&id); err != nil {
						wrapped := fmt.Errorf("read archived memory candidates: %w", err)
						allErr = errors.Join(allErr, wrapped)
						for _, index := range chunk {
							states[index].active = false
							states[index].err = wrapped
						}
						break
					}
					archived[id] = struct{}{}
				}
				rowsErr := rows.Err()
				closeErr := rows.Close()
				if err := errors.Join(rowsErr, closeErr); err != nil {
					wrapped := fmt.Errorf("finish archived memory lookup: %w", err)
					allErr = errors.Join(allErr, wrapped)
					for _, index := range chunk {
						states[index].active = false
						states[index].err = wrapped
					}
					continue
				}
				for _, index := range chunk {
					if _, ok := archived[results[index].ID]; ok {
						states[index].active = false
					}
				}
			}
		}
	}

	if cv.corpus != nil {
		corpusCandidates := make([]int, 0, len(results))
		metadataJSON := make([]string, len(results))
		for i, result := range results {
			if !states[i].active {
				continue
			}
			encoded, err := json.Marshal(result.Metadata)
			if err != nil {
				states[i].active = false
				states[i].err = err
				allErr = errors.Join(allErr, err)
				continue
			}
			metadataJSON[i] = string(encoded)
			corpusCandidates = append(corpusCandidates, i)
		}
		fingerprint := cv.EmbeddingFingerprint()
		for start := 0; start < len(corpusCandidates); start += corpusSQLChunkSize {
			end := min(start+corpusSQLChunkSize, len(corpusCandidates))
			chunk := corpusCandidates[start:end]
			args := make([]any, 0, len(chunk)+1)
			args = append(args, collection)
			for _, index := range chunk {
				args = append(args, results[index].ID)
			}
			rows, err := cv.corpus.db.QueryContext(ctx, `SELECT id,pending,content,metadata,fingerprint FROM documents WHERE collection=? AND id IN (`+corpusPlaceholders(len(chunk))+`)`, args...)
			if err != nil {
				wrapped := fmt.Errorf("check corpus index candidates: %w", err)
				for _, index := range chunk {
					states[index].active = false
					states[index].err = wrapped
					allErr = errors.Join(allErr, wrapped)
				}
				continue
			}
			type corpusSnapshot struct {
				pending     int
				content     string
				metadata    string
				fingerprint string
			}
			stored := make(map[string]corpusSnapshot, len(chunk))
			for rows.Next() {
				var id string
				var snapshot corpusSnapshot
				if err := rows.Scan(&id, &snapshot.pending, &snapshot.content, &snapshot.metadata, &snapshot.fingerprint); err != nil {
					wrapped := fmt.Errorf("read corpus index candidates: %w", err)
					allErr = errors.Join(allErr, wrapped)
					for _, index := range chunk {
						states[index].active = false
						states[index].err = wrapped
					}
					break
				}
				stored[id] = snapshot
			}
			rowsErr := rows.Err()
			closeErr := rows.Close()
			if err := errors.Join(rowsErr, closeErr); err != nil {
				wrapped := fmt.Errorf("finish corpus candidate lookup: %w", err)
				allErr = errors.Join(allErr, wrapped)
				for _, index := range chunk {
					states[index].active = false
					states[index].err = wrapped
				}
				continue
			}
			for _, index := range chunk {
				snapshot, ok := stored[results[index].ID]
				if !ok || snapshot.pending != 0 || snapshot.content != results[index].Content || snapshot.metadata != metadataJSON[index] || snapshot.fingerprint != fingerprint {
					states[index].active = false
				}
			}
		}
	}

	eligible := make([]bool, len(states))
	for i, state := range states {
		eligible[i] = state.active && state.err == nil
	}
	return eligible, allErr
}
