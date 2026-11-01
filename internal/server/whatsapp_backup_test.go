package server

import (
	"archive/zip"
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
)

// vectordb_dir may be data_dir or one of its ancestors. The walk then passes
// data/whatsapp, whose device.db holds the linked device's keys; a restored
// copy would clone the device. The store is skipped, whatever sits around it.
func TestBackupVectorDBWalkSkipsTheWhatsAppStore(t *testing.T) {
	const marker = "WHATSAPP-DEVICE-KEYS"
	root := t.TempDir()
	dataDir := filepath.Join(root, "data")
	cfg := &config.Config{ConfigPath: filepath.Join(root, "config.yaml")}
	cfg.Directories.DataDir = dataDir
	cfg.Directories.VectorDBDir = dataDir
	cfg.Directories.PromptsDir = filepath.Join(root, "prompts")
	cfg.Directories.SkillsDir = filepath.Join(root, "skills")
	cfg.Directories.ToolsDir = filepath.Join(root, "tools")
	cfg.Directories.WorkspaceDir = filepath.Join(root, "workdir")
	if err := os.WriteFile(cfg.ConfigPath, []byte("server:\n  port: 1234\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	store := config.WhatsAppStoreDir(dataDir)
	if err := os.MkdirAll(filepath.Join(store, "nested"), 0o700); err != nil {
		t.Fatal(err)
	}
	for _, path := range append(config.WhatsAppProtectedPaths(cfg), filepath.Join(store, "nested", "key.bin")) {
		if err := os.WriteFile(path, []byte(marker), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dataDir, "index.txt"), []byte("vector index"), 0o600); err != nil {
		t.Fatal(err)
	}
	s := &Server{Cfg: cfg, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	rec := httptest.NewRecorder()
	handleBackupCreate(s).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/backup/create", strings.NewReader(`{"include_vectordb":true}`)))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	zr, err := zip.NewReader(bytes.NewReader(rec.Body.Bytes()), int64(rec.Body.Len()))
	if err != nil {
		t.Fatal(err)
	}
	sawIndex := false
	for _, f := range zr.File {
		if strings.Contains(strings.ToLower(f.Name), "whatsapp") {
			t.Errorf("the backup contains %s", f.Name)
		}
		r, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(r)
		r.Close()
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(body, []byte(marker)) {
			t.Errorf("%s carries WhatsApp key material", f.Name)
		}
		if f.Name == "data/vectordb/index.txt" {
			sawIndex = true
		}
	}
	if !sawIndex {
		t.Error("the walk dropped the ordinary files next to the store")
	}
}
