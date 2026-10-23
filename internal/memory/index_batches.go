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

// indexFileTimeout bounds the replacement of one indexed file: embedding all of its
// chunks plus a fixed allowance for the metadata-store publication that follows.
func indexFileTimeout(chunks int) time.Duration {
	return calculateBatchTimeout(chunks) + 30*time.Second
}

// indexBatchStats reports what addDocumentsInBatches did with its documents.
type indexBatchStats struct {
	embedded  int // written after a provider call
	rewritten int // written again with the stored vector, no provider call
	unchanged int // already published exactly as wanted, left alone
}

// addDocumentsInBatches embeds and publishes docs in batches of
// indexDocumentBatchSize. Every batch gets its own deadline sized for the work it
// still has to do, so one slow provider call cannot exhaust a budget shared with
// hundreds of other documents. Finished batches stay written when a later one
// fails; the caller decides what a partial run means.
//
// Each batch is checked against the credential policy before any of its content
// is sent to the embedding provider. With reuseIndexed, documents whose stored
// copy has the same content and the current embedding fingerprint are not
// embedded again: an identical stored copy is left alone, a copy with changed
// metadata is rewritten with the stored vector.
func (cv *ChromemVectorDB) addDocumentsInBatches(ctx context.Context, col *chromem.Collection, docs []chromem.Document, reuseIndexed bool) (indexBatchStats, error) {
	var stats indexBatchStats
	batches := (len(docs) + indexDocumentBatchSize - 1) / indexDocumentBatchSize
	for start := 0; start < len(docs); start += indexDocumentBatchSize {
		if err := ctx.Err(); err != nil {
			return stats, err
		}
		batch := docs[start:min(start+indexDocumentBatchSize, len(docs))]
		if err := validateNewVectorDocuments(batch); err != nil {
			return stats, err
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
