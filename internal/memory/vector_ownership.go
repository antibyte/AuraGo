package memory

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	chromem "github.com/philippgille/chromem-go"
)

// ContextOwnershipAwareVectorDB keeps bounded automatic writes cancellable while
// retaining the ownership receipts required for safe rollback.
type ContextOwnershipAwareVectorDB interface {
	OwnershipAwareVectorDB
	StoreDocumentOwnedContext(context.Context, string, string, VectorStoreMode) (VectorStoreResult, error)
}

// StoreDocumentWithOwnership is the compatibility entry point for callers
// that need ownership information. Legacy VectorDB implementations remain
// usable for deduplicating writes, but their returned IDs are explicitly
// marked unknown and therefore must never be used for destructive rollback.
// Force-create is intentionally exposed only through OwnershipAwareVectorDB;
// this compatibility helper always performs the existing deduplicating write.
func StoreDocumentWithOwnership(ltm VectorDB, concept, content string) (VectorStoreResult, error) {
	if err := validateMemoryDocument(concept, content); err != nil {
		return VectorStoreResult{}, err
	}
	if ltm == nil {
		return VectorStoreResult{}, fmt.Errorf("vector store is required")
	}
	if owned, ok := ltm.(OwnershipAwareVectorDB); ok {
		return owned.StoreDocumentOwned(concept, content, VectorStoreDeduplicate)
	}
	ids, err := ltm.StoreDocument(concept, content)
	return VectorStoreResult{UnknownIDs: append([]string(nil), ids...)}, err
}

// StoreDocumentWithOwnershipContext prefers cancellable writes. The legacy
// entry point keeps its original dispatch, including wrapper overrides.
func StoreDocumentWithOwnershipContext(ctx context.Context, ltm VectorDB, concept, content string) (VectorStoreResult, error) {
	if err := validateMemoryDocument(concept, content); err != nil {
		return VectorStoreResult{}, err
	}
	if err := ctx.Err(); err != nil {
		return VectorStoreResult{}, err
	}
	if owned, ok := ltm.(ContextOwnershipAwareVectorDB); ok {
		return owned.StoreDocumentOwnedContext(ctx, concept, content, VectorStoreDeduplicate)
	}
	return StoreDocumentWithOwnership(ltm, concept, content)
}

// StoreDocumentOwned stores a document while preserving whether the returned
// IDs were created by this operation or came from similarity deduplication.
// Force-create intentionally skips the similarity check, but keeps the same
// striped concept lock and collection mutation lock as normal writes.
func (cv *ChromemVectorDB) StoreDocumentOwned(concept, content string, mode VectorStoreMode) (VectorStoreResult, error) {
	return cv.StoreDocumentOwnedContext(context.Background(), concept, content, mode)
}

func (cv *ChromemVectorDB) StoreDocumentOwnedContext(ctx context.Context, concept, content string, mode VectorStoreMode) (VectorStoreResult, error) {
	var result VectorStoreResult
	if err := ctx.Err(); err != nil {
		return result, err
	}
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
	if err := lockMemoryMutex(ctx, mu); err != nil {
		return result, err
	}
	defer mu.Unlock()
	return cv.storeMemoryDocumentLocked(ctx, concept, content, "", mode)
}

// The caller holds the concept stripe. A small document needs one embedding:
// use it for both the identity probe and the write, without an extra title query.
func (cv *ChromemVectorDB) storeMemoryDocumentLocked(ctx context.Context, concept, content, domain string, mode VectorStoreMode) (VectorStoreResult, error) {
	if err := validateMemoryDocument(concept+"\n"+domain, content); err != nil {
		return VectorStoreResult{}, err
	}
	var result VectorStoreResult
	if err := ctx.Err(); err != nil {
		return result, err
	}
	var embedding []float32
	if fullContent := buildContentString(concept, content); len(fullContent) <= 4000 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		docs := []chromem.Document{{Content: fullContent}}
		if err := cv.embedDocumentsOutsideLock(ctx, docs); err != nil {
			return result, fmt.Errorf("prepare memory embedding: %w", err)
		}
		embedding = docs[0].Embedding
		if err := startMemoryPublication(ctx); err != nil {
			return result, err
		}
		if mode == VectorStoreDeduplicate {
			docID, err := cv.findIdenticalMemory(ctx, concept, content, domain, embedding)
			if err != nil {
				return result, err
			}
			if docID != "" {
				result.ReusedIDs = []string{docID}
				return result, nil
			}
		}
	}

	if err := ctx.Err(); err != nil {
		return result, err
	}
	ids, err := cv.storeDocumentLocked(ctx, concept, content, domain, embedding)
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

// findIdenticalMemory uses similarity only to nominate a candidate. A topic
// match must never discard a changed fact or knowledge from another domain.
func (cv *ChromemVectorDB) findIdenticalMemory(ctx context.Context, concept, content, domain string, embedding []float32) (string, error) {
	expected := buildContentString(normalizeMemoryDocumentPart(concept), normalizeMemoryDocumentPart(content))
	// ponytail: chunks are not complete documents; store them again until a
	// measured duplication problem justifies whole-document fingerprints.
	if len(buildContentString(concept, content)) > 4000 {
		return "", nil
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	count := cv.collection.Count()
	if count == 0 {
		return "", nil
	}
	// Retrieval age decay is not evidence that identical factual content changed.
	candidates, err := cv.queryVisibleCollection(ctx, cv.collection, embedding, min(count, 8))
	if err != nil {
		return "", fmt.Errorf("find identical memory: %w", err)
	}
	for _, doc := range candidates {
		if doc.Similarity <= 0.95 || doc.Metadata["chunk_index"] != "" ||
			normalizeMemoryDocumentPart(doc.Metadata["domain"]) != normalizeMemoryDocumentPart(domain) {
			continue
		}
		if normalizeMemoryDocumentPart(doc.Content) != expected {
			incoming, incomingOK := AnalysisMemoryIdentity(concept, content, domain)
			oldConcept, oldContent, complete := AnalysisDocumentParts(doc.Content)
			existing, existingOK := AnalysisMemoryIdentity(oldConcept, oldContent, doc.Metadata["domain"])
			if !incomingOK || !complete || !existingOK || incoming != existing {
				continue
			}
		}
		return doc.ID, nil
	}
	return "", nil
}

func normalizeMemoryDocumentPart(text string) string {
	return strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n"))
}

// DeleteDocumentIfContentMatches removes a memory document only if the
// content hash still matches the value captured before replacement. The
// check and delete share cv.mu, so a concurrent Chromem mutation cannot pass
// the check and replace the document before deletion.
func (cv *ChromemVectorDB) DeleteDocumentIfContentMatches(id, expectedSHA256 string) (bool, error) {

	expectedSHA256 = strings.TrimSpace(expectedSHA256)
	if id == "" || expectedSHA256 == "" {
		return false, fmt.Errorf("document ID and expected content hash are required")
	}
	doneStore, err := cv.beginTrackedOperation(&cv.storeWg)
	if err != nil {
		return false, err
	}
	defer doneStore()
	if err := cv.requireReadyForDelete(); err != nil {
		return false, err
	}

	cv.mu.Lock()
	defer cv.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	var doc chromem.Document
	var readErr error
	if cv.corpus != nil {
		doc, readErr = cv.corpusDocument(ctx, "aurago_memories", id)
	} else if cv.collection != nil {
		doc, readErr = cv.collection.GetByID(ctx, id)
	} else {
		readErr = fmt.Errorf("document collection is unavailable")
	}
	err = readErr
	if err != nil {
		return false, fmt.Errorf("read document %s before conditional delete: %w", id, err)
	}
	actual := sha256.Sum256([]byte(doc.Content))
	if !strings.EqualFold(hex.EncodeToString(actual[:]), expectedSHA256) {
		return false, nil
	}
	if err := cv.deleteStoredDocuments(ctx, "aurago_memories", cv.collection, nil, id); err != nil {
		return false, fmt.Errorf("delete unchanged document %s: %w", id, err)
	}
	return true, nil
}
