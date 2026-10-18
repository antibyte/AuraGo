package server

import (
	"archive/zip"
	"bytes"
	"database/sql"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestBackupCorpusAlwaysSnapshotsAndRestoresCustomDirectory(t *testing.T) {
	for _, include := range []string{`false`, `true`} {
		t.Run(include, func(t *testing.T) {
			root, vectorDir := t.TempDir(), t.TempDir()
			cfg := &config.Config{ConfigPath: filepath.Join(root, "config.yaml")}
			cfg.Directories.VectorDBDir = vectorDir
			cfg.Directories.DataDir = filepath.Join(root, "data")
			cfg.Directories.PromptsDir = filepath.Join(root, "prompts")
			cfg.Directories.SkillsDir = filepath.Join(root, "skills")
			cfg.Directories.ToolsDir = filepath.Join(root, "tools")
			cfg.Directories.WorkspaceDir = filepath.Join(root, "workdir")
			if err := os.WriteFile(cfg.ConfigPath, []byte("server:\n  port: 1234\n"), 0600); err != nil {
				t.Fatal(err)
			}
			corpus := config.DocumentCorpusPath(cfg)
			db, err := sql.Open("sqlite", corpus)
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			if _, err := db.Exec(`PRAGMA journal_mode=WAL; PRAGMA wal_autocheckpoint=0; CREATE TABLE documents(content TEXT); INSERT INTO documents VALUES ('durable WAL content')`); err != nil {
				t.Fatal(err)
			}
			s := &Server{Cfg: cfg, Logger: slog.Default()}
			rec := httptest.NewRecorder()
			handleBackupCreate(s).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/backup/create", strings.NewReader(`{"include_vectordb":`+include+`}`)))
			if rec.Code != 200 {
				t.Fatal(rec.Code, rec.Body.String())
			}
			zr, err := zip.NewReader(bytes.NewReader(rec.Body.Bytes()), int64(rec.Body.Len()))
			if err != nil {
				t.Fatal(err)
			}
			var snapshot []byte
			count := 0
			for _, f := range zr.File {
				if strings.HasPrefix(f.Name, "data/vectordb/documents.sqlite-") {
					t.Fatalf("raw sidecar: %s", f.Name)
				}
				if f.Name == "data/vectordb/documents.sqlite" {
					count++
					r, err := f.Open()
					if err != nil {
						t.Fatal(err)
					}
					snapshot, err = io.ReadAll(r)
					r.Close()
					if err != nil {
						t.Fatal(err)
					}
				}
			}
			if count != 1 {
				t.Fatalf("corpus count = %d", count)
			}
			destRoot, destVector := t.TempDir(), filepath.Join(t.TempDir(), "custom")
			destCfg := &config.Config{ConfigPath: filepath.Join(destRoot, "config.yaml")}
			destCfg.Directories.VectorDBDir = destVector
			var archive bytes.Buffer
			zw := zip.NewWriter(&archive)
			entry, _ := zw.Create("data/vectordb/documents.sqlite")
			entry.Write(snapshot)
			zw.Close()
			var body bytes.Buffer
			mw := multipart.NewWriter(&body)
			part, _ := mw.CreateFormFile("file", "backup.ago")
			part.Write(archive.Bytes())
			mw.Close()
			req := httptest.NewRequest(http.MethodPost, "/api/backup/import", &body)
			req.Header.Set("Content-Type", mw.FormDataContentType())
			rr := httptest.NewRecorder()
			handleBackupImport(&Server{Cfg: destCfg, Logger: slog.Default()}).ServeHTTP(rr, req)
			if rr.Code != 200 {
				t.Fatal(rr.Code, rr.Body.String())
			}
			restored, err := sql.Open("sqlite", config.DocumentCorpusPath(destCfg))
			if err != nil {
				t.Fatal(err)
			}
			defer restored.Close()
			var content string
			if err := restored.QueryRow(`SELECT content FROM documents`).Scan(&content); err != nil || content != "durable WAL content" {
				t.Fatalf("restore: %q %v", content, err)
			}
			if _, err := os.Stat(filepath.Join(destRoot, "data", "vectordb", "documents.sqlite")); !os.IsNotExist(err) {
				t.Fatalf("wrong restore destination: %v", err)
			}
		})
	}
}
