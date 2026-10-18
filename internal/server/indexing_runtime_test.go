package server

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"aurago/internal/config"
	"aurago/internal/memory"
)

// TestRestartFileIndexerConcurrentWithReads is meant for `go test -race`: it
// replaces the indexer while handlers and helpers read it.
func TestRestartFileIndexerConcurrentWithReads(t *testing.T) {
	dir := t.TempDir()
	dirs := []config.IndexingDirectory{{Path: dir}}
	cfg := &config.Config{}
	cfg.Indexing.Enabled = true
	cfg.Indexing.Directories = dirs
	cfg.Indexing.Extensions = []string{".txt"}
	disabledCfg := &config.Config{}
	disabledCfg.Indexing.Directories = dirs
	disabledCfg.Indexing.Extensions = []string{".txt"}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })

	s := &Server{Cfg: cfg, Logger: logger, ShortTermMem: stm, LongTermMem: &knowledgeUploadVectorDB{}}
	// Registered after the store cleanup, so the last indexer stops before the store closes.
	t.Cleanup(func() {
		if fi := s.fileIndexer(); fi != nil {
			fi.Stop()
		}
	})
	s.restartFileIndexer(cfg)

	stop := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				rec := httptest.NewRecorder()
				handleIndexingStatus(s).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/indexing/status", nil))
				if rec.Code != http.StatusOK {
					t.Errorf("indexing status = %d, want %d", rec.Code, http.StatusOK)
					return
				}
				_ = agodeskKnowledgeArchiveUploadsEnabled(s)
				if fi := s.fileIndexer(); fi != nil {
					_ = fi.Status()
				}
			}
		}()
	}

	for i := 0; i < 25; i++ {
		next := cfg
		if i%5 == 4 {
			next = disabledCfg
		}
		s.restartFileIndexer(next)
	}
	close(stop)
	wg.Wait()

	s.restartFileIndexer(cfg)
	fi := s.fileIndexer()
	if fi == nil {
		t.Fatal("file indexer is nil after restart with indexing enabled")
	}
	if !fi.Status().Running {
		t.Fatal("file indexer is not running after restart with indexing enabled")
	}

	s.restartFileIndexer(disabledCfg)
	if s.fileIndexer() != nil {
		t.Fatal("file indexer is not nil after restart with indexing disabled")
	}
}
