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
	return cv.addDocumentsWithPrecomputedEmbeddings(ctx, cv.collection, docs)
}
