package server

import (
	"bytes"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/memory"
	"aurago/internal/services"
)

type knowledgeUploadVectorDB struct {
	mu     sync.Mutex
	nextID int
}

func (v *knowledgeUploadVectorDB) StoreDocument(concept, content string) ([]string, error) {
	return v.StoreDocumentInCollection(concept, content, services.IndexerCollection)
}

func (v *knowledgeUploadVectorDB) StoreDocumentInCollection(concept, content, collection string) ([]string, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.nextID++
	return []string{"doc-upload"}, nil
}

func (v *knowledgeUploadVectorDB) StoreDocumentWithEmbedding(concept, content string, embedding []float32) (string, error) {
	return "doc-upload", nil
}
func (v *knowledgeUploadVectorDB) StoreDocumentWithEmbeddingInCollection(concept, content string, embedding []float32, collection string) (string, error) {
	return "doc-upload", nil
}
func (v *knowledgeUploadVectorDB) StoreBatch(items []memory.ArchiveItem) ([]string, error) {
	return nil, nil
}
func (v *knowledgeUploadVectorDB) SearchSimilar(query string, topK int, excludeCollections ...string) ([]string, []string, error) {
	return nil, nil, nil
}
func (v *knowledgeUploadVectorDB) SearchMemoriesOnly(query string, topK int) ([]string, []string, error) {
	return nil, nil, nil
}
func (v *knowledgeUploadVectorDB) GetByID(id string) (string, error) { return "", nil }
func (v *knowledgeUploadVectorDB) GetByIDFromCollection(id, collection string) (string, error) {
	return "", nil
}
func (v *knowledgeUploadVectorDB) DeleteDocument(id string) error { return nil }
func (v *knowledgeUploadVectorDB) DeleteDocumentFromCollection(id, collection string) error {
	return nil
}
func (v *knowledgeUploadVectorDB) Count() int       { return 0 }
func (v *knowledgeUploadVectorDB) IsDisabled() bool { return false }
func (v *knowledgeUploadVectorDB) Close() error     { return nil }
func (v *knowledgeUploadVectorDB) StoreCheatsheet(id, name, content string, attachments ...string) error {
	return nil
}
func (v *knowledgeUploadVectorDB) DeleteCheatsheet(id string) error { return nil }

func TestHandleKnowledgeUploadRejectsDisallowedExtension(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfg := &config.Config{}
	cfg.Indexing.Directories = []config.IndexingDirectory{{Path: dir}}
	s := &Server{Cfg: cfg, Logger: slog.Default()}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "payload.exe")
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	if _, err := io.WriteString(part, "MZ"); err != nil {
		t.Fatalf("WriteString: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/knowledge/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()

	handleKnowledgeUpload(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestHandleKnowledgeUploadRejectsOverwrite(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	target := filepath.Join(dir, "note.md")
	if err := os.WriteFile(target, []byte("existing"), 0o640); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg := &config.Config{}
	cfg.Indexing.Directories = []config.IndexingDirectory{{Path: dir}}
	s := &Server{Cfg: cfg, Logger: slog.Default()}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "note.md")
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	if _, err := io.WriteString(part, "new"); err != nil {
		t.Fatalf("WriteString: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/knowledge/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()

	handleKnowledgeUpload(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusConflict, rec.Body.String())
	}
	content, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if strings.TrimSpace(string(content)) != "existing" {
		t.Fatalf("existing file was modified: %q", string(content))
	}
}

func TestHandleKnowledgeUploadTriggersIndexerRescan(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{}
	cfg.Indexing.Directories = []config.IndexingDirectory{{Path: dir}}
	cfg.Indexing.Extensions = []string{".txt"}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })

	vdb := &knowledgeUploadVectorDB{}
	s := &Server{Cfg: cfg, Logger: logger, ShortTermMem: stm, LongTermMem: vdb}
	s.FileIndexer = services.NewFileIndexer(cfg, &s.CfgMu, vdb, stm, logger)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "krankenkasse.txt")
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	if _, err := io.WriteString(part, "Krankenkasse Beitragserstattung Versicherungsnummer"); err != nil {
		t.Fatalf("WriteString: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/knowledge/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()

	handleKnowledgeUpload(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	indexedPath := filepath.Join(dir, "krankenkasse.txt")
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		docIDs, err := stm.GetFileEmbeddingDocIDs(indexedPath, services.IndexerCollection)
		if err != nil {
			t.Fatalf("GetFileEmbeddingDocIDs: %v", err)
		}
		if len(docIDs) > 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("uploaded knowledge file was not indexed within timeout")
}
