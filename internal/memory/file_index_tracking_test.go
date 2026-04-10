package memory

import (
	"io"
	"log/slog"
	"reflect"
	"testing"
	"time"
)

func TestFileIndexTrackingStoresAndDeletesDocIDs(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	defer stm.Close()

	path := "knowledge/note.txt"
	collection := "file_index"
	modTime := time.Now().UTC().Truncate(time.Second)
	if err := stm.UpdateFileIndexWithDocs(path, collection, modTime, []string{"doc-2", "doc-1"}); err != nil {
		t.Fatalf("UpdateFileIndexWithDocs: %v", err)
	}

	gotTime, err := stm.GetFileIndex(path, collection)
	if err != nil {
		t.Fatalf("GetFileIndex: %v", err)
	}
	if gotTime.IsZero() {
		t.Fatal("expected file index timestamp to be stored")
	}

	docIDs, err := stm.GetFileEmbeddingDocIDs(path, collection)
	if err != nil {
		t.Fatalf("GetFileEmbeddingDocIDs: %v", err)
	}
	wantDocIDs := []string{"doc-1", "doc-2"}
	if !reflect.DeepEqual(docIDs, wantDocIDs) {
		t.Fatalf("GetFileEmbeddingDocIDs = %v, want %v", docIDs, wantDocIDs)
	}

	paths, err := stm.ListIndexedFiles(collection)
	if err != nil {
		t.Fatalf("ListIndexedFiles: %v", err)
	}
	if !reflect.DeepEqual(paths, []string{path}) {
		t.Fatalf("ListIndexedFiles = %v, want [%s]", paths, path)
	}

	if err := stm.DeleteFileIndex(path, collection); err != nil {
		t.Fatalf("DeleteFileIndex: %v", err)
	}

	gotTime, err = stm.GetFileIndex(path, collection)
	if err != nil {
		t.Fatalf("GetFileIndex after delete: %v", err)
	}
	if !gotTime.IsZero() {
		t.Fatalf("expected file index timestamp cleared, got %v", gotTime)
	}

	docIDs, err = stm.GetFileEmbeddingDocIDs(path, collection)
	if err != nil {
		t.Fatalf("GetFileEmbeddingDocIDs after delete: %v", err)
	}
	if len(docIDs) != 0 {
		t.Fatalf("expected tracked doc IDs cleared, got %v", docIDs)
	}
}

func TestFileIndexTrackingMultiCollectionIsolation(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	defer stm.Close()

	path := "knowledge/shared.txt"
	modTime := time.Now().UTC().Truncate(time.Second)

	// Store same file in two different collections with different doc IDs
	if err := stm.UpdateFileIndexWithDocs(path, "collection1", modTime, []string{"doc-c1-a", "doc-c1-b"}); err != nil {
		t.Fatalf("UpdateFileIndexWithDocs collection1: %v", err)
	}
	if err := stm.UpdateFileIndexWithDocs(path, "collection2", modTime, []string{"doc-c2-only"}); err != nil {
		t.Fatalf("UpdateFileIndexWithDocs collection2: %v", err)
	}

	// Verify collection1 has its own tracking
	c1IDs, err := stm.GetFileEmbeddingDocIDs(path, "collection1")
	if err != nil {
		t.Fatalf("GetFileEmbeddingDocIDs collection1: %v", err)
	}
	if !reflect.DeepEqual(c1IDs, []string{"doc-c1-a", "doc-c1-b"}) {
		t.Fatalf("collection1 doc IDs = %v, want [doc-c1-a doc-c1-b]", c1IDs)
	}

	// Verify collection2 has its own tracking
	c2IDs, err := stm.GetFileEmbeddingDocIDs(path, "collection2")
	if err != nil {
		t.Fatalf("GetFileEmbeddingDocIDs collection2: %v", err)
	}
	if !reflect.DeepEqual(c2IDs, []string{"doc-c2-only"}) {
		t.Fatalf("collection2 doc IDs = %v, want [doc-c2-only]", c2IDs)
	}

	// Verify ListIndexedFiles returns correct paths per collection
	c1Paths, err := stm.ListIndexedFiles("collection1")
	if err != nil {
		t.Fatalf("ListIndexedFiles collection1: %v", err)
	}
	if !reflect.DeepEqual(c1Paths, []string{path}) {
		t.Fatalf("collection1 paths = %v, want [%s]", c1Paths, path)
	}

	c2Paths, err := stm.ListIndexedFiles("collection2")
	if err != nil {
		t.Fatalf("ListIndexedFiles collection2: %v", err)
	}
	if !reflect.DeepEqual(c2Paths, []string{path}) {
		t.Fatalf("collection2 paths = %v, want [%s]", c2Paths, path)
	}

	// Delete from collection1 only
	if err := stm.DeleteFileIndex(path, "collection1"); err != nil {
		t.Fatalf("DeleteFileIndex collection1: %v", err)
	}

	// Verify collection1 is cleared
	c1IDs, err = stm.GetFileEmbeddingDocIDs(path, "collection1")
	if err != nil {
		t.Fatalf("GetFileEmbeddingDocIDs collection1 after delete: %v", err)
	}
	if len(c1IDs) != 0 {
		t.Fatalf("expected collection1 cleared, got %v", c1IDs)
	}

	// Verify collection2 is unaffected
	c2IDs, err = stm.GetFileEmbeddingDocIDs(path, "collection2")
	if err != nil {
		t.Fatalf("GetFileEmbeddingDocIDs collection2 after delete: %v", err)
	}
	if !reflect.DeepEqual(c2IDs, []string{"doc-c2-only"}) {
		t.Fatalf("collection2 doc IDs after delete = %v, want [doc-c2-only]", c2IDs)
	}

	// ListIndexedFiles should only show path in collection2 now
	c1Paths, err = stm.ListIndexedFiles("collection1")
	if err != nil {
		t.Fatalf("ListIndexedFiles collection1 after delete: %v", err)
	}
	if len(c1Paths) != 0 {
		t.Fatalf("expected collection1 empty after delete, got %v", c1Paths)
	}

	c2Paths, err = stm.ListIndexedFiles("collection2")
	if err != nil {
		t.Fatalf("ListIndexedFiles collection2 after delete: %v", err)
	}
	if !reflect.DeepEqual(c2Paths, []string{path}) {
		t.Fatalf("collection2 paths after delete = %v, want [%s]", c2Paths, path)
	}
}

func TestClearFileIndicesRemovesTrackedDocIDs(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	defer stm.Close()

	if err := stm.UpdateFileIndexWithDocs("knowledge/a.txt", "file_index", time.Now().UTC(), []string{"doc-a"}); err != nil {
		t.Fatalf("UpdateFileIndexWithDocs a: %v", err)
	}
	if err := stm.UpdateFileIndexWithDocs("knowledge/b.txt", "file_index", time.Now().UTC(), []string{"doc-b"}); err != nil {
		t.Fatalf("UpdateFileIndexWithDocs b: %v", err)
	}

	if err := stm.ClearFileIndices(); err != nil {
		t.Fatalf("ClearFileIndices: %v", err)
	}

	paths, err := stm.ListIndexedFiles("file_index")
	if err != nil {
		t.Fatalf("ListIndexedFiles after clear: %v", err)
	}
	if len(paths) != 0 {
		t.Fatalf("expected no indexed files after clear, got %v", paths)
	}

	docIDs, err := stm.GetFileEmbeddingDocIDs("knowledge/a.txt", "file_index")
	if err != nil {
		t.Fatalf("GetFileEmbeddingDocIDs after clear: %v", err)
	}
	if len(docIDs) != 0 {
		t.Fatalf("expected no tracked doc IDs after clear, got %v", docIDs)
	}
}
