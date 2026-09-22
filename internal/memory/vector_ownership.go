package memory

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// StoreDocumentWithOwnership is the compatibility entry point for callers
// that need ownership information. Legacy VectorDB implementations remain
// usable for deduplicating writes, but their returned IDs are explicitly
// marked unknown and therefore must never be used for destructive rollback.
// Force-create is intentionally exposed only through OwnershipAwareVectorDB;
// this compatibility helper always performs the existing deduplicating write.
func StoreDocumentWithOwnership(ltm VectorDB, concept, content string) (VectorStoreResult, error) {
	if ltm == nil {
		return VectorStoreResult{}, fmt.Errorf("vector store is required")
	}
	if owned, ok := ltm.(OwnershipAwareVectorDB); ok {
		return owned.StoreDocumentOwned(concept, content, VectorStoreDeduplicate)
	}
	ids, err := ltm.StoreDocument(concept, content)
	return VectorStoreResult{UnknownIDs: append([]string(nil), ids...)}, err
}

// StoreDocumentOwned stores a document while preserving whether the returned
// IDs were created by this operation or came from similarity deduplication.
// Force-create intentionally skips the similarity check, but keeps the same
// striped concept lock and collection mutation lock as normal writes.
func (cv *ChromemVectorDB) StoreDocumentOwned(concept, content string, mode VectorStoreMode) (VectorStoreResult, error) {
	var result VectorStoreResult
	if mode != VectorStoreDeduplicate && mode != VectorStoreForceCreate {
		return result, fmt.Errorf("unsupported vector store mode %d", mode)
	}
	doneStore, err := cv.beginTrackedOperation(&cv.storeWg)
	if err != nil {
		return result, err
	}
	defer doneStore()

	if err := cv.requireReadyForStore(); err != nil {
		return result, err
	}

	mu := cv.getConceptMutex(concept)
	mu.Lock()
	defer mu.Unlock()

	if mode == VectorStoreDeduplicate {
		if docID, sim := cv.searchTopSimilarMemory(concept); sim > 0.95 && docID != "" {
			result.ReusedIDs = []string{docID}
			return result, nil
		}
	}

	ids, err := cv.storeDocumentLocked(concept, content, "")
	// storeDocumentLocked returns every generated ID even when Chromem reports
	// a partial chunk/persistence failure. Those IDs are owned by this call and
	// must remain available to the caller for bounded rollback.
	result.CreatedIDs = append(result.CreatedIDs, ids...)
	if err != nil {
		return result, err
	}
	if len(result.CreatedIDs) == 0 {
		return result, fmt.Errorf("vector store created no document IDs")
	}
	return result, nil
}

// DeleteDocumentIfContentMatches removes a memory document only if the
// content hash still matches the value captured before replacement. The
// check and delete share cv.mu, so a concurrent Chromem mutation cannot pass
// the check and replace the document before deletion.
func (cv *ChromemVectorDB) DeleteDocumentIfContentMatches(id, expectedSHA256 string) (bool, error) {
	id = strings.TrimSpace(id)
	expectedSHA256 = strings.TrimSpace(expectedSHA256)
	if id == "" || expectedSHA256 == "" {
		return false, fmt.Errorf("document ID and expected content hash are required")
	}
	doneStore, err := cv.beginTrackedOperation(&cv.storeWg)
	if err != nil {
		return false, err
	}
	defer doneStore()
	if err := cv.requireReadyForStore(); err != nil {
		return false, err
	}

	cv.mu.Lock()
	defer cv.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	doc, err := cv.collection.GetByID(ctx, id)
	if err != nil {
		return false, fmt.Errorf("read document %s before conditional delete: %w", id, err)
	}
	actual := sha256.Sum256([]byte(doc.Content))
	if !strings.EqualFold(hex.EncodeToString(actual[:]), expectedSHA256) {
		return false, nil
	}
	if err := cv.collection.Delete(ctx, nil, nil, id); err != nil {
		return false, fmt.Errorf("delete unchanged document %s: %w", id, err)
	}
	return true, nil
}
