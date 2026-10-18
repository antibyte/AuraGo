package memory

import (
	"context"
	"errors"
	"fmt"

	chromem "github.com/philippgille/chromem-go"
)

// reindexUnchangedFileDocuments preserves raw IDs and provenance when the
// authorized file indexer reproduced the same complete source chunks. Changed
// content/chunk boundaries continue through the existing generation protocol.
func (cv *ChromemVectorDB) reindexUnchangedFileDocuments(ctx context.Context, stm *SQLiteMemory, col *chromem.Collection, path, collection string, before, after FileIndexState, oldIDs []string, docs []chromem.Document) (resultIDs []string, reindexed bool, resultErr error) {
	if cv.corpus == nil || before.ContentHash == "" || before.ContentHash != after.ContentHash || len(oldIDs) == 0 || len(oldIDs) != len(docs) {
		return nil, false, nil
	}
	old := make([]chromem.Document, 0, len(oldIDs))
	for _, id := range oldIDs {
		doc, err := cv.readCollectionDocument(ctx, col, id)
		if err != nil {
			return oldIDs, true, fmt.Errorf("retain unchanged file until source documents are readable: %w", err)
		}
		old = append(old, doc)
	}
	used := map[string]bool{}
	stable := make([]chromem.Document, 0, len(docs))
	ids := make([]string, 0, len(docs))
	for _, doc := range docs {
		matched := false
		for _, previous := range old {
			if used[previous.ID] || previous.Content != doc.Content || previous.Metadata["domain"] != doc.Metadata["domain"] {
				continue
			}
			if previous.Metadata["multimodal"] == "true" && doc.Metadata["multimodal"] != "true" {
				continue
			}
			doc.ID = previous.ID
			doc.Metadata = cv.addEmbeddingMetadata(previous.Metadata)
			if len(doc.Embedding) > 0 {
				doc.Metadata["multimodal"] = "true"
			}
			used[doc.ID] = true
			stable = append(stable, doc)
			ids = append(ids, doc.ID)
			matched = true
			break
		}
		if !matched {
			return nil, false, nil
		}
	}
	ctx, publication := prepareMemoryPublication(ctx, stm, nil)
	complete := false
	defer func() { resultErr = errors.Join(resultErr, publication.finish(complete)) }()
	if err := cv.addExistingDocumentsWithPrecomputedEmbeddings(ctx, col, stable); err != nil {
		return ids, true, err
	}
	if err := stm.PublishFileIndex(path, collection, before, oldIDs, after, ids); err != nil {
		return ids, true, err
	}
	complete = true
	return ids, true, nil
}
