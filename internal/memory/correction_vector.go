package memory

import (
	"context"
	"fmt"
	"strings"

	chromem "github.com/philippgille/chromem-go"
)

type correctionVectorWriter interface {
	StoreCorrectionDocument(context.Context, string, string) error
}

type correctionPreparedVectorsKey struct{}

func (cv *ChromemVectorDB) prepareCorrectionVectors(ctx context.Context, correction *MemoryCorrection) (context.Context, error) {
	docs := []chromem.Document{}
	for _, item := range correction.Items {
		if item.Reference.Store == "ltm" && item.Operation == "replace" && item.State != "done" {
			docs = append(docs, chromem.Document{ID: item.ReplacementID, Content: item.Value})
		}
	}
	if err := cv.embedDocumentsOutsideLock(ctx, docs); err != nil {
		return ctx, err
	}
	prepared := map[string]chromem.Document{}
	for _, doc := range docs {
		prepared[doc.ID] = doc
	}
	return context.WithValue(ctx, correctionPreparedVectorsKey{}, prepared), nil
}

// StoreCorrectionDocument uses the ID journaled as archived before the provider call.
func (cv *ChromemVectorDB) StoreCorrectionDocument(ctx context.Context, id, content string) error {
	if !strings.HasPrefix(id, "mc_") || strings.TrimSpace(content) == "" {
		return fmt.Errorf("invalid staged memory correction")
	}
	done, err := cv.beginTrackedOperation(&cv.storeWg)
	if err != nil {
		return err
	}
	defer done()
	if err := cv.requireReadyForStore(); err != nil {
		return err
	}
	lock := cv.getConceptMutex(id)
	lock.Lock()
	defer lock.Unlock()
	cv.mu.RLock()
	existing, readErr := cv.collection.GetByID(ctx, id)
	cv.mu.RUnlock()
	if readErr == nil {
		if existing.Content != content || existing.Metadata["correction_id"] != id {
			return fmt.Errorf("correction vector ownership changed")
		}
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if readErr.Error() != fmt.Sprintf("document with ID '%v' not found", id) {
		return fmt.Errorf("read staged correction: %w", readErr)
	}
	docs := []chromem.Document{{ID: id, Content: content, Metadata: cv.addEmbeddingMetadata(map[string]string{"source_type": "memory_correction", "correction_id": id, "collection": "aurago_memories"})}}
	if prepared, ok := ctx.Value(correctionPreparedVectorsKey{}).(map[string]chromem.Document); ok {
		if doc, found := prepared[id]; found && doc.Content == content {
			docs[0].Embedding = doc.Embedding
		}
	}
	return cv.addDocumentsWithPrecomputedEmbeddings(ctx, cv.collection, docs)
}

// deleteUnpublishedCorrectionDocument removes a vector only while its stored
// metadata proves the exact correction owner. Missing vectors are idempotent.
func (cv *ChromemVectorDB) deleteUnpublishedCorrectionDocument(ctx context.Context, id, correctionID string) (bool, error) {
	if cv == nil || id == "" || correctionID == "" {
		return false, fmt.Errorf("correction vector identity is required")
	}
	done, err := cv.beginTrackedOperation(&cv.storeWg)
	if err != nil {
		return false, err
	}
	defer done()
	if err := cv.requireReadyForDelete(); err != nil {
		return false, err
	}
	lock := cv.getConceptMutex(id)
	lock.Lock()
	defer lock.Unlock()
	cv.mu.Lock()
	defer cv.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return false, err
	}
	var doc chromem.Document
	var readErr error
	if cv.corpus != nil {
		doc, readErr = cv.corpusDocument(ctx, "aurago_memories", id)
	} else if cv.collection != nil {
		doc, readErr = cv.collection.GetByID(ctx, id)
	} else {
		readErr = fmt.Errorf("correction vector collection is unavailable")
	}
	if readErr != nil {
		if readErr.Error() == fmt.Sprintf("document with ID '%v' not found", id) {
			return true, nil
		}
		return false, fmt.Errorf("read staged correction vector: %w", readErr)
	}
	if doc.Metadata["correction_id"] != id || doc.Metadata["source_type"] != "memory_correction" || doc.Metadata["collection"] != "aurago_memories" {
		return false, fmt.Errorf("staged correction vector ownership is uncertain")
	}
	if err := cv.deleteStoredDocuments(ctx, "aurago_memories", cv.collection, nil, id); err != nil {
		return false, fmt.Errorf("delete staged correction vector: %w", err)
	}
	return true, nil
}
