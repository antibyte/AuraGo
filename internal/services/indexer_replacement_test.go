package services

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"aurago/internal/config"
	"aurago/internal/memory"
)

func TestAuditFailedFileReplacementKeepsPreviousIndex(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	s, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.txt")
	if err := os.WriteFile(path, []byte("Old searchable audit document."), 0600); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{}
	cfg.Indexing.Extensions = []string{".txt"}
	directory := config.IndexingDirectory{Path: dir}
	vdb := &fakeIndexerVectorDB{}
	fi := NewFileIndexer(cfg, &sync.RWMutex{}, vdb, s, logger)
	old, err := fi.IndexFile(context.Background(), directory, path, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("Changed replacement audit document."), 0600); err != nil {
		t.Fatal(err)
	}
	vdb.storeErr = errors.New("controlled provider failure")
	if _, err := fi.IndexFile(context.Background(), directory, path, nil); err == nil {
		t.Fatal("fixture must fail replacement")
	}
	tracked, err := s.GetFileEmbeddingDocIDs(path, IndexerCollection)
	if err != nil {
		t.Fatal(err)
	}
	if len(vdb.deleted) != 0 || len(tracked) != len(old.DocumentIDs) {
		t.Fatalf("failed replacement discarded old index: deleted=%v tracked=%v original=%v", vdb.deleted, tracked, old.DocumentIDs)
	}
}
