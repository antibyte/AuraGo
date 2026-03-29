package services

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/memory"
)

type fakeIndexerVectorDB struct {
	mu      sync.Mutex
	nextID  int
	deleted []string
}

func (f *fakeIndexerVectorDB) StoreDocument(concept, content string) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextID++
	return []string{fmt.Sprintf("doc-%d", f.nextID)}, nil
}

func (f *fakeIndexerVectorDB) StoreDocumentWithEmbedding(concept, content string, embedding []float32) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextID++
	return fmt.Sprintf("doc-%d", f.nextID), nil
}

func (f *fakeIndexerVectorDB) StoreBatch(items []memory.ArchiveItem) ([]string, error) {
	return nil, nil
}

func (f *fakeIndexerVectorDB) SearchSimilar(query string, topK int, excludeCollections ...string) ([]string, []string, error) {
	return nil, nil, nil
}

func (f *fakeIndexerVectorDB) SearchMemoriesOnly(query string, topK int) ([]string, []string, error) {
	return nil, nil, nil
}

func (f *fakeIndexerVectorDB) GetByID(id string) (string, error) { return "", nil }

func (f *fakeIndexerVectorDB) DeleteDocument(id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deleted = append(f.deleted, id)
	return nil
}

func (f *fakeIndexerVectorDB) Count() int       { return 0 }
func (f *fakeIndexerVectorDB) IsDisabled() bool { return false }
func (f *fakeIndexerVectorDB) Close() error     { return nil }

func TestFileIndexerReplacesTrackedEmbeddingsOnReindex(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	defer stm.Close()

	dir := t.TempDir()
	path := filepath.Join(dir, "note.txt")
	if err := os.WriteFile(path, []byte("first version"), 0644); err != nil {
		t.Fatalf("WriteFile first version: %v", err)
	}
	firstMod := time.Now().UTC().Add(-2 * time.Minute).Truncate(time.Second)
	if err := os.Chtimes(path, firstMod, firstMod); err != nil {
		t.Fatalf("Chtimes first version: %v", err)
	}

	cfg := &config.Config{}
	cfg.Indexing.Directories = []string{dir}
	cfg.Indexing.Extensions = []string{".txt"}
	cfg.Indexing.PollIntervalSeconds = 60

	cfgMu := &sync.RWMutex{}
	vdb := &fakeIndexerVectorDB{}
	fi := NewFileIndexer(cfg, cfgMu, vdb, stm, logger)

	_, indexed, errs := fi.scanDirectory(dir)
	if indexed != 1 {
		t.Fatalf("first scan indexed = %d, want 1", indexed)
	}
	if len(errs) != 0 {
		t.Fatalf("first scan errors = %v", errs)
	}

	firstIDs, err := stm.GetFileEmbeddingDocIDs(path)
	if err != nil {
		t.Fatalf("GetFileEmbeddingDocIDs first: %v", err)
	}
	if !reflect.DeepEqual(firstIDs, []string{"doc-1"}) {
		t.Fatalf("first tracked IDs = %v, want [doc-1]", firstIDs)
	}

	if err := os.WriteFile(path, []byte("second version"), 0644); err != nil {
		t.Fatalf("WriteFile second version: %v", err)
	}
	secondMod := firstMod.Add(2 * time.Minute)
	if err := os.Chtimes(path, secondMod, secondMod); err != nil {
		t.Fatalf("Chtimes second version: %v", err)
	}

	_, indexed, errs = fi.scanDirectory(dir)
	if indexed != 1 {
		t.Fatalf("second scan indexed = %d, want 1", indexed)
	}
	if len(errs) != 0 {
		t.Fatalf("second scan errors = %v", errs)
	}

	if !reflect.DeepEqual(vdb.deleted, []string{"doc-1"}) {
		t.Fatalf("deleted docs = %v, want [doc-1]", vdb.deleted)
	}

	secondIDs, err := stm.GetFileEmbeddingDocIDs(path)
	if err != nil {
		t.Fatalf("GetFileEmbeddingDocIDs second: %v", err)
	}
	if !reflect.DeepEqual(secondIDs, []string{"doc-2"}) {
		t.Fatalf("second tracked IDs = %v, want [doc-2]", secondIDs)
	}
}

func TestFileIndexerRemovesEmbeddingsForDeletedFiles(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	defer stm.Close()

	dir := t.TempDir()
	path := filepath.Join(dir, "note.txt")
	if err := os.WriteFile(path, []byte("keep me indexed"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	modTime := time.Now().UTC().Add(-time.Minute).Truncate(time.Second)
	if err := os.Chtimes(path, modTime, modTime); err != nil {
		t.Fatalf("Chtimes: %v", err)
	}

	cfg := &config.Config{}
	cfg.Indexing.Directories = []string{dir}
	cfg.Indexing.Extensions = []string{".txt"}
	cfg.Indexing.PollIntervalSeconds = 60

	cfgMu := &sync.RWMutex{}
	vdb := &fakeIndexerVectorDB{}
	fi := NewFileIndexer(cfg, cfgMu, vdb, stm, logger)

	_, indexed, errs := fi.scanDirectory(dir)
	if indexed != 1 {
		t.Fatalf("initial scan indexed = %d, want 1", indexed)
	}
	if len(errs) != 0 {
		t.Fatalf("initial scan errors = %v", errs)
	}

	if err := os.Remove(path); err != nil {
		t.Fatalf("Remove indexed file: %v", err)
	}

	total, indexed, errs := fi.scanDirectory(dir)
	if total != 0 {
		t.Fatalf("deleted-file scan total = %d, want 0", total)
	}
	if indexed != 0 {
		t.Fatalf("deleted-file scan indexed = %d, want 0", indexed)
	}
	if len(errs) != 0 {
		t.Fatalf("deleted-file scan errors = %v", errs)
	}

	if !reflect.DeepEqual(vdb.deleted, []string{"doc-1"}) {
		t.Fatalf("deleted docs = %v, want [doc-1]", vdb.deleted)
	}

	docIDs, err := stm.GetFileEmbeddingDocIDs(path)
	if err != nil {
		t.Fatalf("GetFileEmbeddingDocIDs after delete: %v", err)
	}
	if len(docIDs) != 0 {
		t.Fatalf("expected tracked IDs removed after file delete, got %v", docIDs)
	}

	lastIndexed, err := stm.GetFileIndex(path)
	if err != nil {
		t.Fatalf("GetFileIndex after delete: %v", err)
	}
	if !lastIndexed.IsZero() {
		t.Fatalf("expected file index removed after file delete, got %v", lastIndexed)
	}
}
