package server

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"aurago/internal/config"
)

func newIndexingHandlerTestServer(cfg *config.Config) *Server {
	return &Server{
		Cfg:    cfg,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

func serveIndexingDirectoriesRequest(t *testing.T, s *Server, method string, body map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var reqBody bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&reqBody).Encode(body); err != nil {
			t.Fatalf("encode request body: %v", err)
		}
	}
	req := httptest.NewRequest(method, "/api/indexing/directories", &reqBody)
	rec := httptest.NewRecorder()
	handleIndexingDirectories(s).ServeHTTP(rec, req)
	return rec
}

func TestHandleIndexingDirectoriesPostPersistFailureLeavesRuntimeUnchanged(t *testing.T) {
	tmp := t.TempDir()
	existing := filepath.Join(tmp, "existing")
	cfg := &config.Config{ConfigPath: filepath.Join(tmp, "missing", "config.yaml")}
	cfg.Indexing.Enabled = false
	cfg.Indexing.Directories = []config.IndexingDirectory{{Path: existing, Collection: "docs"}}
	original := append([]config.IndexingDirectory(nil), cfg.Indexing.Directories...)
	s := newIndexingHandlerTestServer(cfg)

	rec := serveIndexingDirectoriesRequest(t, s, http.MethodPost, map[string]string{
		"path":       "./new-docs",
		"collection": "new",
	})
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusInternalServerError, rec.Body.String())
	}
	if !reflect.DeepEqual(cfg.Indexing.Directories, original) {
		t.Fatalf("directories mutated after persist failure: got %v want %v", cfg.Indexing.Directories, original)
	}
	if cfg.Indexing.Enabled {
		t.Fatal("indexing.enabled mutated to true after persist failure")
	}
}

func TestHandleIndexingDirectoriesDeletePersistFailureLeavesRuntimeUnchanged(t *testing.T) {
	tmp := t.TempDir()
	existing := filepath.Join(tmp, "existing")
	cfg := &config.Config{ConfigPath: filepath.Join(tmp, "missing", "config.yaml")}
	cfg.Indexing.Enabled = false
	cfg.Indexing.Directories = []config.IndexingDirectory{{Path: existing, Collection: "docs"}}
	original := append([]config.IndexingDirectory(nil), cfg.Indexing.Directories...)
	s := newIndexingHandlerTestServer(cfg)

	rec := serveIndexingDirectoriesRequest(t, s, http.MethodDelete, map[string]string{"path": existing})
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusInternalServerError, rec.Body.String())
	}
	if !reflect.DeepEqual(cfg.Indexing.Directories, original) {
		t.Fatalf("directories mutated after persist failure: got %v want %v", cfg.Indexing.Directories, original)
	}
	if cfg.Indexing.Enabled {
		t.Fatal("indexing.enabled mutated to true after persist failure")
	}
}

func TestHandleIndexingDirectoriesPreservesDisabledConfigOnPost(t *testing.T) {
	tmp := t.TempDir()
	configPath := filepath.Join(tmp, "config.yaml")
	if err := os.WriteFile(configPath, []byte("indexing:\n  enabled: false\n  directories: []\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg := &config.Config{ConfigPath: configPath}
	cfg.Indexing.Enabled = false
	s := newIndexingHandlerTestServer(cfg)

	rec := serveIndexingDirectoriesRequest(t, s, http.MethodPost, map[string]string{
		"path":       "./docs",
		"collection": "docs",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if cfg.Indexing.Enabled {
		t.Fatal("runtime indexing.enabled mutated to true")
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if bytes.Contains(data, []byte("enabled: true")) {
		t.Fatalf("persisted config enabled indexing unexpectedly:\n%s", string(data))
	}
	if !bytes.Contains(data, []byte("enabled: false")) {
		t.Fatalf("persisted config did not preserve enabled: false:\n%s", string(data))
	}
}
