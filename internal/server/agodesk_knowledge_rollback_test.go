package server

import (
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"aurago/internal/memory"
)

// deleteFailingKnowledgeVectorDB fails every vector delete so rollback cannot
// remove embeddings.
type deleteFailingKnowledgeVectorDB struct {
	knowledgeUploadVectorDB
}

func (v *deleteFailingKnowledgeVectorDB) DeleteDocumentFromCollection(id, collection string) error {
	return errors.New("vector store unavailable")
}

func TestAgodeskKnowledgeRollbackKeepsTrackingUntilVectorsAreDeleted(t *testing.T) {
	for _, tc := range []struct {
		name     string
		vectors  memory.VectorDB
		wantKept int
	}{
		{name: "delete fails", vectors: &deleteFailingKnowledgeVectorDB{}, wantKept: 2},
		{name: "delete succeeds", vectors: &knowledgeUploadVectorDB{}, wantKept: 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stm := newAgodeskTestMemory(t)
			s := &Server{
				Logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
				ShortTermMem: stm,
				LongTermMem:  tc.vectors,
			}
			path := filepath.Join(t.TempDir(), "doc-1.txt")
			if err := stm.UpdateFileIndexWithDocs(path, "file_index", time.Now(), []string{"vec-1", "vec-2"}); err != nil {
				t.Fatal(err)
			}
			(&agodeskKnowledgeCoordinator{server: s}).rollbackIndexedDocument(memory.AgoDeskKnowledgeDocument{
				DocumentID:  "doc-1",
				StoragePath: path,
				Collection:  "file_index",
			})
			kept, err := stm.GetFileEmbeddingDocIDs(path, "file_index")
			if err != nil {
				t.Fatal(err)
			}
			if len(kept) != tc.wantKept {
				t.Fatalf("tracked doc ids after rollback = %v, want %d", kept, tc.wantKept)
			}
		})
	}
}
