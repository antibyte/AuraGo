package server

import (
	"archive/zip"
	"bytes"
	"database/sql"
	"encoding/json"
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

func TestManualBackupUsesCoordinatedMemorySnapshotAndRejectsPartialPublication(t *testing.T) {
	root := t.TempDir()
	cfg := &config.Config{ConfigPath: filepath.Join(root, "config.yaml")}
	cfg.Directories.DataDir = filepath.Join(root, "data")
	cfg.Directories.VectorDBDir = filepath.Join(root, "data", "vectordb")
	cfg.Directories.PromptsDir = filepath.Join(root, "prompts")
	cfg.Directories.SkillsDir = filepath.Join(root, "skills")
	cfg.Directories.ToolsDir = filepath.Join(root, "tools")
	cfg.Directories.WorkspaceDir = filepath.Join(root, "workdir")
	cfg.SQLite.ShortTermPath = filepath.Join(root, "data", "stm.sqlite")
	cfg.SQLite.KnowledgeGraphPath = filepath.Join(root, "data", "kg.sqlite")
	if err := os.MkdirAll(cfg.Directories.VectorDBDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfg.ConfigPath, []byte("server:\n  port: 1234\n"), 0600); err != nil {
		t.Fatal(err)
	}
	var stm *sql.DB
	for _, path := range []string{cfg.SQLite.ShortTermPath, cfg.SQLite.KnowledgeGraphPath, config.DocumentCorpusPath(cfg)} {
		db, err := sql.Open("sqlite", path)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { db.Close() })
		if _, err = db.Exec(`PRAGMA journal_mode=WAL; CREATE TABLE fixture(value TEXT); INSERT INTO fixture VALUES('coordinated')`); err != nil {
			t.Fatal(err)
		}
		if path == cfg.SQLite.ShortTermPath {
			stm = db
		}
	}
	if _, err := stm.Exec(`CREATE TABLE memory_maintenance_meta(key TEXT PRIMARY KEY,value TEXT); INSERT INTO memory_maintenance_meta VALUES('snapshot.publication.test','pending')`); err != nil {
		t.Fatal(err)
	}
	s := &Server{Cfg: cfg, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	export := func() *httptest.ResponseRecorder {
		r := httptest.NewRecorder()
		handleBackupCreate(s).ServeHTTP(r, httptest.NewRequest(http.MethodPost, "/api/backup/create", strings.NewReader(`{}`)))
		return r
	}
	if r := export(); r.Code != http.StatusServiceUnavailable || !strings.Contains(r.Body.String(), "unfinished memory write") {
		t.Fatalf("partial publication export status=%d %s", r.Code, r.Body.String())
	}
	if _, err := stm.Exec(`DELETE FROM memory_maintenance_meta`); err != nil {
		t.Fatal(err)
	}
	r := export()
	if r.Code != http.StatusOK {
		t.Fatalf("export status=%d %s", r.Code, r.Body.String())
	}
	zr, err := zip.NewReader(bytes.NewReader(r.Body.Bytes()), int64(r.Body.Len()))
	if err != nil {
		t.Fatal(err)
	}
	files := 0
	certified := false
	for _, file := range zr.File {
		if strings.HasSuffix(file.Name, ".sqlite") {
			files++
		}
		if file.Name == "manifest.json" {
			reader, err := file.Open()
			if err != nil {
				t.Fatal(err)
			}
			var manifest agoManifest
			err = json.NewDecoder(reader).Decode(&manifest)
			reader.Close()
			if err != nil {
				t.Fatal(err)
			}
			certified = manifest.MemoryConsistency == "coordinated-three-db-read-snapshot-v1"
		}
	}
	if files != 3 || !certified {
		t.Fatalf("memory snapshots=%d certified=%v", files, certified)
	}
}
