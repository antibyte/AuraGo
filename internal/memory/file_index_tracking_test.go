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
	modTime := time.Now().UTC().Truncate(time.Second)
	if err := stm.UpdateFileIndexWithDocs(path, "file_index", modTime, []string{"doc-2", "doc-1"}); err != nil {
		t.Fatalf("UpdateFileIndexWithDocs: %v", err)
	}

	gotTime, err := stm.GetFileIndex(path)
	if err != nil {
		t.Fatalf("GetFileIndex: %v", err)
	}
	if gotTime.IsZero() {
		t.Fatal("expected file index timestamp to be stored")
	}

	docIDs, err := stm.GetFileEmbeddingDocIDs(path)
	if err != nil {
		t.Fatalf("GetFileEmbeddingDocIDs: %v", err)
	}
	wantDocIDs := []string{"doc-1", "doc-2"}
	if !reflect.DeepEqual(docIDs, wantDocIDs) {
		t.Fatalf("GetFileEmbeddingDocIDs = %v, want %v", docIDs, wantDocIDs)
	}

	paths, err := stm.ListIndexedFiles("file_index")
	if err != nil {
		t.Fatalf("ListIndexedFiles: %v", err)
	}
	if !reflect.DeepEqual(paths, []string{path}) {
		t.Fatalf("ListIndexedFiles = %v, want [%s]", paths, path)
	}

	if err := stm.DeleteFileIndex(path); err != nil {
		t.Fatalf("DeleteFileIndex: %v", err)
	}

	gotTime, err = stm.GetFileIndex(path)
	if err != nil {
		t.Fatalf("GetFileIndex after delete: %v", err)
	}
	if !gotTime.IsZero() {
		t.Fatalf("expected file index timestamp cleared, got %v", gotTime)
	}

	docIDs, err = stm.GetFileEmbeddingDocIDs(path)
	if err != nil {
		t.Fatalf("GetFileEmbeddingDocIDs after delete: %v", err)
	}
	if len(docIDs) != 0 {
		t.Fatalf("expected tracked doc IDs cleared, got %v", docIDs)
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

	docIDs, err := stm.GetFileEmbeddingDocIDs("knowledge/a.txt")
	if err != nil {
		t.Fatalf("GetFileEmbeddingDocIDs after clear: %v", err)
	}
	if len(docIDs) != 0 {
		t.Fatalf("expected no tracked doc IDs after clear, got %v", docIDs)
	}
}
