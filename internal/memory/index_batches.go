package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	chromem "github.com/philippgille/chromem-go"
)

// indexingContext returns the parent context of a long indexing run. It ends
// when Close starts, so shutdown aborts an in-flight embedding call instead of
// waiting for it. The run itself is bounded per batch, never as a whole.
func (cv *ChromemVectorDB) indexingContext() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if cv.closed.Load() {
					cancel()
					return
				}
			}
		}
	}()
	return ctx, cancel
}

// indexFileTimeout bounds the replacement of one indexed file: every one of its
// batches gets the deadline of a full batch, plus a fixed allowance for the
// metadata-store publication that follows. The deadline grows with the file;
// calculateBatchTimeout caps a single batch, but a file of many batches would be cut
// off by that cap even though no batch is slow.
func indexFileTimeout(chunks int) time.Duration {
	if chunks < 0 {
		chunks = 0
	}
	batches := (chunks + indexDocumentBatchSize - 1) / indexDocumentBatchSize
	return time.Duration(batches)*calculateBatchTimeout(indexDocumentBatchSize) + 30*time.Second
}

// indexBatchStats reports what addDocumentsInBatches did with its documents.
type indexBatchStats struct {
	embedded  int // written after a provider call
	rewritten int // written again with the stored vector, no provider call
	unchanged int // already published exactly as wanted, left alone
	// rejected lists the IDs of documents the credential policy kept out: they were
	// neither embedded nor written, so callers must not count them as indexed.
	rejected []string
}

// maxLoggedRejectedIDs bounds the document IDs named in the log of one run.
const maxLoggedRejectedIDs = 20

// addDocumentsInBatches embeds and publishes docs in batches of
// indexDocumentBatchSize. Every batch gets its own deadline sized for the work it
// still has to do, so one slow provider call cannot exhaust a budget shared with
// hundreds of other documents. Finished batches stay written when a later one
// fails; the caller decides what a partial run means.
//
// Every document is checked against the credential policy before any of its content
// is sent to the embedding provider. A rejected document is skipped on its own: it
// is neither embedded nor written, it is reported in the stats and logged by ID and
// count, and the rest of the run continues. With reuseIndexed, documents whose
// stored copy has the same content and the current embedding fingerprint are not
// embedded again: an identical stored copy is left alone, a copy with changed
// metadata is rewritten with the stored vector.
func (cv *ChromemVectorDB) addDocumentsInBatches(ctx context.Context, col *chromem.Collection, docs []chromem.Document, reuseIndexed bool) (stats indexBatchStats, err error) {
	defer func() {
		if len(stats.rejected) == 0 || cv.logger == nil {
			return
		}
		ids := stats.rejected
		if len(ids) > maxLoggedRejectedIDs {
			ids = ids[:maxLoggedRejectedIDs]
		}
		cv.logger.Warn("Skipped documents that the credential policy rejects", "collection", col.Name, "count", len(stats.rejected), "ids", ids)
	}()
	batches := (len(docs) + indexDocumentBatchSize - 1) / indexDocumentBatchSize
	for start := 0; start < len(docs); start += indexDocumentBatchSize {
		if err := ctx.Err(); err != nil {
			return stats, err
		}
		batch, rejected := splitRejectedDocuments(docs[start:min(start+indexDocumentBatchSize, len(docs))])
		stats.rejected = append(stats.rejected, rejected...)
		if len(batch) == 0 {
			continue
		}
		pending := batch
		var unchanged, rewritten int
		if reuseIndexed {
			pending, unchanged, rewritten = cv.dropIndexedDocuments(ctx, col, batch)
		}
		stats.unchanged += unchanged
		if len(pending) == 0 {
			continue
		}
		batchCtx, cancel := context.WithTimeout(ctx, calculateBatchTimeout(len(pending)))
		err := cv.addExistingDocumentsWithPrecomputedEmbeddings(batchCtx, col, pending)
		cancel()
		if err != nil {
			return stats, fmt.Errorf("index batch %d/%d: %w", start/indexDocumentBatchSize+1, batches, err)
		}
		stats.rewritten += rewritten
		stats.embedded += len(pending) - rewritten
	}
	return stats, nil
}

// splitRejectedDocuments separates the documents that pass the credential policy
// from the IDs of those that do not. Without a rejection it returns docs itself.
func splitRejectedDocuments(docs []chromem.Document) (accepted []chromem.Document, rejected []string) {
	for i, doc := range docs {
		if validateNewVectorDocuments([]chromem.Document{doc}) == nil {
			if accepted != nil {
				accepted = append(accepted, doc)
			}
			continue
		}
		if accepted == nil {
			accepted = append(make([]chromem.Document, 0, len(docs)-1), docs[:i]...)
		}
		rejected = append(rejected, doc.ID)
	}
	if accepted == nil {
		return docs, nil
	}
	return accepted, rejected
}

// dropIndexedDocuments removes the documents of a batch that need no work
// (unchanged) and fills in the stored vector of those that only need a rewrite
// (rewritten, so no provider call). A document counts as already indexed when its
// stored copy is published, has the same content, was embedded under the current
// fingerprint and carries a usable vector.
func (cv *ChromemVectorDB) dropIndexedDocuments(ctx context.Context, col *chromem.Collection, batch []chromem.Document) (pending []chromem.Document, unchanged, rewritten int) {
	fingerprint := cv.EmbeddingFingerprint()
	if fingerprint == "" {
		// Without a fingerprint nothing proves the stored vector matches the model.
		return batch, 0, 0
	}
	pending = make([]chromem.Document, 0, len(batch))
	for _, doc := range batch {
		stored, ok := cv.storedIndexedDocument(ctx, col, doc.ID)
		if !ok || stored.Content != doc.Content || stored.Metadata["embedding_fingerprint"] != fingerprint || !cv.usableStoredVector(stored.Embedding) {
			pending = append(pending, doc)
			continue
		}
		if live, err := col.GetByID(ctx, doc.ID); err == nil && live.Content == doc.Content && sameIndexedMetadata(live.Metadata, doc.Metadata, fingerprint) {
			unchanged++
			continue
		}
		doc.Embedding = stored.Embedding
		pending = append(pending, doc)
		rewritten++
	}
	return pending, unchanged, rewritten
}

// storedIndexedDocument returns the durable copy of a published document: the
// corpus row once its index publication finished, or the in-memory copy when
// there is no corpus. The corpus is consulted because the derived index may not
// be loaded yet when indexing starts right after process start.
func (cv *ChromemVectorDB) storedIndexedDocument(ctx context.Context, col *chromem.Collection, id string) (chromem.Document, bool) {
	if cv.corpus == nil {
		doc, err := col.GetByID(ctx, id)
		return doc, err == nil
	}
	var doc chromem.Document
	var metadata string
	var vector []byte
	var dimension, pending int
	err := cv.corpus.db.QueryRowContext(ctx, `SELECT content,metadata,vector,dimension,pending FROM documents WHERE collection=? AND id=?`, col.Name, id).Scan(&doc.Content, &metadata, &vector, &dimension, &pending)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			cv.logger.Debug("Stored document lookup failed; embedding it again", "collection", col.Name, "id", id, "error", err)
		}
		return chromem.Document{}, false
	}
	if pending != 0 {
		return chromem.Document{}, false
	}
	if err := json.Unmarshal([]byte(metadata), &doc.Metadata); err != nil {
		return chromem.Document{}, false
	}
	if err := json.Unmarshal(vector, &doc.Embedding); err != nil || len(doc.Embedding) != dimension {
		return chromem.Document{}, false
	}
	doc.ID = id
	return doc, true
}

func (cv *ChromemVectorDB) usableStoredVector(embedding []float32) bool {
	if len(embedding) == 0 {
		return false
	}
	if expected := cv.embeddingDimension.Load(); expected > 0 && len(embedding) != int(expected) {
		return false
	}
	for _, v := range embedding {
		if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
			return false
		}
	}
	return true
}

// sameIndexedMetadata reports whether a published copy already carries exactly
// the metadata a re-index would write: the desired keys plus the fingerprint.
func sameIndexedMetadata(live, desired map[string]string, fingerprint string) bool {
	if live["embedding_fingerprint"] != fingerprint {
		return false
	}
	for key, value := range desired {
		if key == "embedding_fingerprint" {
			continue
		}
		if got, ok := live[key]; !ok || got != value {
			return false
		}
	}
	for key := range live {
		if key == "embedding_fingerprint" {
			continue
		}
		if _, ok := desired[key]; !ok {
			return false
		}
	}
	return true
}
