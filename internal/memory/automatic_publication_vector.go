package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	chromem "github.com/philippgille/chromem-go"
)

func (cv *ChromemVectorDB) PrepareAutomaticMemoryWriteContext(ctx context.Context, concept, content string) (*PreparedAutomaticMemoryWrite, error) {
	if err := validateMemoryDocument(concept, content); err != nil {
		return nil, err
	}
	if err := cv.requireReadyForStore(); err != nil {
		return nil, err
	}
	if cv.corpus == nil {
		return nil, fmt.Errorf("automatic memory publication requires a canonical document corpus")
	}
	done, err := cv.beginTrackedOperation(&cv.storeWg)
	if err != nil {
		return nil, err
	}
	conceptLock := cv.getConceptMutex(concept)
	if err := lockMemoryMutex(ctx, conceptLock); err != nil {
		done()
		return nil, err
	}
	release := func() { conceptLock.Unlock(); done() }
	if err := cv.requireReadyForStore(); err != nil {
		release()
		return nil, err
	}
	fullContent := buildContentString(concept, content)
	metadata := map[string]string{"concept": concept, "timestamp": fmt.Sprintf("%d", time.Now().Unix())}
	if len(fullContent) <= 4000 {
		ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		docs := []chromem.Document{{Content: fullContent}}
		if err := cv.embedDocumentsOutsideLock(ctx, docs); err != nil {
			release()
			return nil, fmt.Errorf("prepare memory embedding: %w", err)
		}
		if err := ctx.Err(); err != nil {
			release()
			return nil, err
		}
		docID, err := cv.findIdenticalMemory(ctx, concept, content, "", docs[0].Embedding)
		if err != nil {
			release()
			return nil, err
		}
		if docID != "" {
			return &PreparedAutomaticMemoryWrite{Result: VectorStoreResult{ReusedIDs: []string{docID}}, release: release}, nil
		}
		docs[0].ID = fmt.Sprintf("mem_%d_%d", time.Now().UnixMilli(), cv.idCounter.Add(1))
		docs[0].Metadata = cv.addEmbeddingMetadata(metadata)
		if err := validateNewVectorDocuments(docs); err != nil {
			release()
			return nil, err
		}
		return &PreparedAutomaticMemoryWrite{
			Result:    VectorStoreResult{CreatedIDs: []string{docs[0].ID}},
			Documents: []PreparedMemoryDocument{preparedMemoryDocument(docs[0])},
			release:   release,
		}, nil
	}

	const maxChunks = 200
	chunks := chunkText(content, 3500, 200)
	if len(chunks) > maxChunks {
		chunks = chunks[:maxChunks]
	}
	if len(chunks) == 0 {
		release()
		return nil, fmt.Errorf("automatic memory produced no chunks")
	}
	baseCounter := cv.idCounter.Add(int64(len(chunks)))
	docs := make([]chromem.Document, 0, len(chunks))
	for i, chunk := range chunks {
		chunkMetadata := map[string]string{
			"concept":     concept,
			"chunk_index": fmt.Sprintf("%d/%d", i+1, len(chunks)),
			"timestamp":   fmt.Sprintf("%d", time.Now().Unix()),
		}
		docs = append(docs, chromem.Document{
			ID:       fmt.Sprintf("mem_%d_%d_chunk_%d", time.Now().UnixMilli(), baseCounter-int64(len(chunks))+int64(i)+1, i),
			Content:  buildContentString(concept, chunk),
			Metadata: cv.addEmbeddingMetadata(chunkMetadata),
		})
	}
	chunkCtx, cancel := context.WithTimeout(ctx, calculateBatchTimeout(len(docs)))
	defer cancel()
	if err := cv.embedDocumentsOutsideLock(chunkCtx, docs); err != nil {
		release()
		return nil, fmt.Errorf("prepare chunked memory embeddings: %w", err)
	}
	if err := validateNewVectorDocuments(docs); err != nil {
		release()
		return nil, err
	}
	prepared := make([]PreparedMemoryDocument, 0, len(docs))
	ids := make([]string, 0, len(docs))
	for _, doc := range docs {
		prepared = append(prepared, preparedMemoryDocument(doc))
		ids = append(ids, doc.ID)
	}
	return &PreparedAutomaticMemoryWrite{Result: VectorStoreResult{CreatedIDs: ids}, Documents: prepared, release: release}, nil
}

func preparedMemoryDocument(doc chromem.Document) PreparedMemoryDocument {
	metadata := make(map[string]string, len(doc.Metadata))
	for key, value := range doc.Metadata {
		metadata[key] = value
	}
	return PreparedMemoryDocument{ID: doc.ID, Content: doc.Content, Embedding: cloneFloat32Slice(doc.Embedding), Metadata: metadata}
}

func (cv *ChromemVectorDB) hasCanonicalMemoryCorpus() bool {
	return cv != nil && cv.corpus != nil
}

func (cv *ChromemVectorDB) StageAutomaticMemoryWriteContext(ctx context.Context, publicationID string, documents []PreparedMemoryDocument) error {
	if cv.corpus == nil || publicationID == "" || len(documents) == 0 {
		return fmt.Errorf("automatic memory publication stage is incomplete")
	}
	if !strings.HasPrefix(publicationID, automaticMemoryPublicationPrefix) {
		return fmt.Errorf("automatic memory publication ID is invalid")
	}
	cv.mu.Lock()
	defer cv.mu.Unlock()
	tx, err := cv.corpus.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO collections(name) VALUES('aurago_memories')`); err != nil {
		return err
	}
	for _, document := range documents {
		if document.ID == "" || document.Content == "" || len(document.Embedding) == 0 {
			return fmt.Errorf("prepared automatic memory document is incomplete")
		}
		if err := validateDurableDocument(document.Content, ""); err != nil {
			return err
		}
		metadata := make(map[string]string, len(document.Metadata)+1)
		for key, value := range document.Metadata {
			metadata[key] = value
		}
		metadata["memory_publication_id"] = publicationID
		metadataJSON, err := json.Marshal(metadata)
		if err != nil {
			return err
		}
		vectorJSON, err := json.Marshal(document.Embedding)
		if err != nil {
			return err
		}
		result, err := tx.ExecContext(ctx, `INSERT INTO documents(collection,id,content,metadata,vector,dimension,fingerprint,pending,blocked_reason)
			VALUES('aurago_memories',?,?,?,?,?,?,1,?)`, document.ID, document.Content, string(metadataJSON), vectorJSON, len(document.Embedding), metadata["embedding_fingerprint"], automaticMemoryPublicationPending)
		if err != nil {
			return fmt.Errorf("stage automatic memory document %s: %w", document.ID, err)
		}
		if count, err := result.RowsAffected(); err != nil || count != 1 {
			return errors.Join(fmt.Errorf("stage automatic memory document %s did not insert exactly one row", document.ID), err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit staged automatic memory documents: %w", err)
	}
	return nil
}

func (cv *ChromemVectorDB) ActivateAutomaticMemoryWriteContext(ctx context.Context, publicationID string, receipts []MemoryPublicationDocument) error {
	return cv.activateAutomaticMemoryWriteContext(ctx, publicationID, receipts)
}

func (cv *ChromemVectorDB) ReadAutomaticMemoryPublicationDocumentContext(ctx context.Context, id string) (string, error) {
	if cv.corpus == nil || strings.TrimSpace(id) == "" {
		return "", fmt.Errorf("automatic memory publication document is unavailable")
	}
	document, err := cv.corpusDocument(ctx, "aurago_memories", id)
	if err != nil {
		return "", err
	}
	return document.Content, nil
}

func (cv *ChromemVectorDB) ReadAutomaticMemoryPublicationCorpusStateContext(ctx context.Context, ids []string) (map[string]memoryPublicationCorpusState, error) {
	if cv.corpus == nil {
		return nil, fmt.Errorf("automatic memory publication corpus is unavailable")
	}
	states := make(map[string]memoryPublicationCorpusState)
	for start := 0; start < len(ids); start += maxMemoryEligibilityIDs {
		end := min(start+maxMemoryEligibilityIDs, len(ids))
		chunk := ids[start:end]
		if len(chunk) == 0 {
			continue
		}
		query := `SELECT id,metadata,pending,blocked_reason FROM documents WHERE collection='aurago_memories' AND id IN (` + corpusPlaceholders(len(chunk)) + `)`
		rows, err := cv.corpus.db.QueryContext(ctx, query, stringsToAny(chunk)...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var id, metadataRaw, blocked string
			var pending int
			if err := rows.Scan(&id, &metadataRaw, &pending, &blocked); err != nil {
				rows.Close()
				return nil, err
			}
			var metadata map[string]string
			if err := json.Unmarshal([]byte(metadataRaw), &metadata); err != nil {
				rows.Close()
				return nil, fmt.Errorf("decode memory publication corpus metadata: %w", err)
			}
			publicationID := metadata["memory_publication_id"]
			if publicationID != "" || blocked == automaticMemoryPublicationPending {
				states[id] = memoryPublicationCorpusState{publicationID: publicationID, pending: pending != 0, blockedReason: blocked}
			}
		}
		if err := errors.Join(rows.Err(), rows.Close()); err != nil {
			return nil, err
		}
	}
	return states, nil
}

func (cv *ChromemVectorDB) activateAutomaticMemoryWriteContext(ctx context.Context, publicationID string, receipts []MemoryPublicationDocument) error {
	if cv.corpus == nil || cv.collection == nil || publicationID == "" || len(receipts) == 0 {
		return fmt.Errorf("automatic memory publication activation is incomplete")
	}
	if err := cv.requireReadyForStore(); err != nil {
		return err
	}
	done, err := cv.beginTrackedOperation(&cv.storeWg)
	if err != nil {
		return err
	}
	defer done()
	if err := cv.requireReadyForStore(); err != nil {
		return err
	}
	for !cv.mu.TryLock() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(10 * time.Millisecond):
		}
	}
	defer cv.mu.Unlock()
	ids := make([]string, 0, len(receipts))
	wanted := make(map[string]string, len(receipts))
	for _, receipt := range receipts {
		if receipt.ID == "" || receipt.Hash == "" {
			return fmt.Errorf("automatic memory publication receipt is incomplete")
		}
		if _, exists := wanted[receipt.ID]; exists {
			return fmt.Errorf("duplicate automatic memory publication receipt %s", receipt.ID)
		}
		wanted[receipt.ID] = receipt.Hash
		ids = append(ids, receipt.ID)
	}
	args := append([]any{"aurago_memories"}, stringsToAny(ids)...)
	rows, err := cv.corpus.db.QueryContext(ctx, `SELECT id,content,metadata,vector,dimension,fingerprint,pending,blocked_reason FROM documents WHERE collection=? AND id IN (`+corpusPlaceholders(len(ids))+`)`, args...)
	if err != nil {
		return fmt.Errorf("read staged automatic memory documents: %w", err)
	}
	docsByID := make(map[string]chromem.Document, len(ids))
	metadataByID := make(map[string]string, len(ids))
	pendingByID := make(map[string]bool, len(ids))
	for rows.Next() {
		var id, content, metadata, fingerprint, blocked string
		var vector []byte
		var dimension, pending int
		if err := rows.Scan(&id, &content, &metadata, &vector, &dimension, &fingerprint, &pending, &blocked); err != nil {
			rows.Close()
			return err
		}
		var decodedMetadata map[string]string
		var embedding []float32
		if err := json.Unmarshal([]byte(metadata), &decodedMetadata); err != nil {
			rows.Close()
			return err
		}
		if err := json.Unmarshal(vector, &embedding); err != nil {
			rows.Close()
			return err
		}
		if decodedMetadata["memory_publication_id"] != publicationID || contentSHA256(content) != wanted[id] || fingerprint != cv.EmbeddingFingerprint() || decodedMetadata["embedding_fingerprint"] != fingerprint || len(embedding) != dimension || dimension != int(cv.embeddingDimension.Load()) {
			rows.Close()
			return fmt.Errorf("staged automatic memory document %s changed or uses an incompatible embedding model", id)
		}
		if pending != 0 && blocked != automaticMemoryPublicationPending {
			rows.Close()
			return fmt.Errorf("staged automatic memory document %s is not pending publication", id)
		}
		docsByID[id] = chromem.Document{ID: id, Content: content, Metadata: decodedMetadata, Embedding: embedding}
		metadataByID[id] = metadata
		pendingByID[id] = pending != 0
	}
	if err := errors.Join(rows.Err(), rows.Close()); err != nil {
		return fmt.Errorf("finish staged automatic memory documents: %w", err)
	}
	if len(docsByID) != len(ids) {
		return fmt.Errorf("staged automatic memory publication is incomplete")
	}
	docs := make([]chromem.Document, 0, len(ids))
	for _, id := range ids {
		docs = append(docs, docsByID[id])
	}
	// The corpus fence stays pending while the precomputed vectors are added.
	// Removing any partial in-memory attempt first makes retries deterministic.
	if err := cv.collection.Delete(ctx, nil, nil, ids...); err != nil {
		return fmt.Errorf("clear partial automatic memory index: %w", err)
	}
	if err := cv.collection.AddDocuments(ctx, docs, 8); err != nil {
		return fmt.Errorf("add prepared automatic memory vectors: %w", err)
	}
	tx, err := cv.corpus.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin automatic memory activation marker: %w", err)
	}
	defer tx.Rollback()
	for _, id := range ids {
		if !pendingByID[id] {
			continue
		}
		result, err := tx.ExecContext(ctx, `UPDATE documents SET pending=0,blocked_reason='' WHERE collection='aurago_memories' AND id=? AND metadata=? AND content=? AND pending=1 AND blocked_reason=?`, id, metadataByID[id], docsByID[id].Content, automaticMemoryPublicationPending)
		if err != nil {
			return fmt.Errorf("activate corpus document %s: %w", id, err)
		}
		if count, err := result.RowsAffected(); err != nil || count != 1 {
			return errors.Join(fmt.Errorf("activate corpus document %s changed during publication", id), err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit automatic memory activation markers: %w", err)
	}
	return nil
}

func (cv *ChromemVectorDB) CleanupAutomaticMemoryWriteContext(ctx context.Context, publicationID string, receipts []MemoryPublicationDocument) error {
	if cv.corpus == nil || publicationID == "" {
		return fmt.Errorf("automatic memory publication cleanup is incomplete")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	cv.mu.Lock()
	defer cv.mu.Unlock()
	tx, err := cv.corpus.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var deleteIDs []string
	for _, receipt := range receipts {
		if receipt.ID == "" || receipt.Hash == "" {
			return fmt.Errorf("automatic memory cleanup receipt is incomplete")
		}
		var content, metadataRaw string
		err := tx.QueryRowContext(ctx, `SELECT content,metadata FROM documents WHERE collection='aurago_memories' AND id=?`, receipt.ID).Scan(&content, &metadataRaw)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return err
		}
		var metadata map[string]string
		if err := json.Unmarshal([]byte(metadataRaw), &metadata); err != nil {
			return fmt.Errorf("read staged automatic memory metadata: %w", err)
		}
		if metadata["memory_publication_id"] != publicationID || contentSHA256(content) != receipt.Hash {
			continue
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM documents WHERE collection='aurago_memories' AND id=? AND metadata=? AND content=?`, receipt.ID, metadataRaw, content); err != nil {
			return err
		}
		deleteIDs = append(deleteIDs, receipt.ID)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit staged automatic memory cleanup: %w", err)
	}
	if len(deleteIDs) == 0 {
		return nil
	}
	if err := cv.collection.Delete(ctx, nil, nil, deleteIDs...); err != nil {
		return fmt.Errorf("delete partial automatic memory index: %w", err)
	}
	return nil
}

func (cv *ChromemVectorDB) RetireCompletedAutomaticMemoryPublicationTokenContext(ctx context.Context, publicationID string, receipt MemoryPublicationDocument, keepIndexed bool) (bool, error) {
	if cv.corpus == nil || publicationID == "" || receipt.ID == "" || receipt.Hash == "" {
		return false, fmt.Errorf("completed memory publication token is incomplete")
	}
	if err := ctx.Err(); err != nil {
		return false, err
	}
	cv.mu.Lock()
	defer cv.mu.Unlock()
	tx, err := cv.corpus.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	var content, metadataRaw, fingerprint string
	var vectorJSON []byte
	var dimension, pending int
	var blocked string
	err = tx.QueryRowContext(ctx, `SELECT content,metadata,vector,dimension,fingerprint,pending,blocked_reason FROM documents WHERE collection='aurago_memories' AND id=?`, receipt.ID).Scan(&content, &metadataRaw, &vectorJSON, &dimension, &fingerprint, &pending, &blocked)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var metadata map[string]string
	if err := json.Unmarshal([]byte(metadataRaw), &metadata); err != nil {
		return false, err
	}
	if metadata["memory_publication_id"] != publicationID || contentSHA256(content) != receipt.Hash || pending != 0 {
		return false, nil
	}
	var embedding []float32
	if err := json.Unmarshal(vectorJSON, &embedding); err != nil {
		return false, err
	}
	delete(metadata, "memory_publication_id")
	newMetadata, err := json.Marshal(metadata)
	if err != nil {
		return false, err
	}
	reindex := keepIndexed && (fingerprint != cv.EmbeddingFingerprint() || dimension != int(cv.embeddingDimension.Load()) || len(embedding) != dimension)
	newPending := pending
	newBlocked := blocked
	if reindex {
		newPending = 1
		newBlocked = ""
	}
	result, err := tx.ExecContext(ctx, `UPDATE documents SET metadata=?,pending=?,blocked_reason=? WHERE collection='aurago_memories' AND id=? AND content=? AND metadata=? AND pending=0 AND blocked_reason=?`, string(newMetadata), newPending, newBlocked, receipt.ID, content, metadataRaw, blocked)
	if err != nil {
		return false, err
	}
	if count, err := result.RowsAffected(); err != nil || count != 1 {
		return false, errors.Join(fmt.Errorf("completed memory publication token changed before retirement"), err)
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	if cv.collection != nil {
		if err := cv.collection.Delete(ctx, nil, nil, receipt.ID); err != nil {
			return reindex, err
		}
		if keepIndexed && !reindex {
			document := chromem.Document{ID: receipt.ID, Content: content, Metadata: metadata, Embedding: embedding}
			if err := cv.collection.AddDocuments(ctx, []chromem.Document{document}, 8); err != nil {
				return false, err
			}
		}
	}
	return reindex, nil
}

func (cv *ChromemVectorDB) BlockCompletedAutomaticMemoryPublicationTokenContext(ctx context.Context, publicationID string, receipt MemoryPublicationDocument) error {
	if cv.corpus == nil || publicationID == "" || receipt.ID == "" || receipt.Hash == "" {
		return fmt.Errorf("memory publication quarantine receipt is incomplete")
	}
	cv.mu.Lock()
	defer cv.mu.Unlock()
	tx, err := cv.corpus.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var content, metadataRaw string
	var pending int
	err = tx.QueryRowContext(ctx, `SELECT content,metadata,pending FROM documents WHERE collection='aurago_memories' AND id=?`, receipt.ID).Scan(&content, &metadataRaw, &pending)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	var metadata map[string]string
	if err := json.Unmarshal([]byte(metadataRaw), &metadata); err != nil {
		return err
	}
	if metadata["memory_publication_id"] != publicationID || contentSHA256(content) != receipt.Hash || pending != 0 {
		return nil
	}
	result, err := tx.ExecContext(ctx, `UPDATE documents SET pending=1,blocked_reason=? WHERE collection='aurago_memories' AND id=? AND content=? AND metadata=? AND pending=0`, automaticMemoryPublicationPending, receipt.ID, content, metadataRaw)
	if err != nil {
		return err
	}
	if count, err := result.RowsAffected(); err != nil || count != 1 {
		return errors.Join(fmt.Errorf("memory publication quarantine receipt changed"), err)
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	if cv.collection != nil {
		if err := cv.collection.Delete(ctx, nil, nil, receipt.ID); err != nil {
			return err
		}
	}
	return nil
}

func (cv *ChromemVectorDB) waitMemoryPublicationReady(ctx context.Context) error {
	if cv == nil {
		return fmt.Errorf("vector store is unavailable")
	}
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		if cv.closed.Load() {
			return ErrVectorDBClosed
		}
		if cv.ready.Load() {
			if cv.disabled.Load() {
				return ErrVectorDBDisabled
			}
			return nil
		}
		select {
		case <-ctx.Done():
			return errors.Join(ErrVectorDBNotReady, ctx.Err())
		case <-ticker.C:
		}
	}
}

var _ PreparedAutomaticMemoryVectorDB = (*ChromemVectorDB)(nil)
