package server

import (
	"archive/zip"
	"bytes"
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

// newBackupTestServer returns a server whose data_dir holds valid short_term
// and galaxa databases.
func newBackupTestServer(t *testing.T) (*Server, string) {
	t.Helper()
	instanceRoot := t.TempDir()
	dataDir := filepath.Join(instanceRoot, "data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(instanceRoot, "config.yaml")
	if err := os.WriteFile(configPath, []byte("server:\n  port: 1234\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	shortTermPath := filepath.Join(dataDir, "short_term.db")
	writeTestSQLiteDB(t, shortTermPath)
	writeTestSQLiteDB(t, filepath.Join(dataDir, config.GalaxaDBFilename))
	cfg := &config.Config{ConfigPath: configPath}
	cfg.Directories.DataDir = dataDir
	cfg.Directories.PromptsDir = filepath.Join(instanceRoot, "prompts")
	cfg.Directories.SkillsDir = filepath.Join(instanceRoot, "agent_workspace", "skills")
	cfg.Directories.ToolsDir = filepath.Join(instanceRoot, "agent_workspace", "tools")
	cfg.Directories.WorkspaceDir = filepath.Join(instanceRoot, "agent_workspace", "workdir")
	cfg.SQLite.ShortTermPath = shortTermPath
	return &Server{Cfg: cfg, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}, dataDir
}

func createTestBackup(t *testing.T, s *Server) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/backup/create", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handleBackupCreate(s).ServeHTTP(rec, req)
	return rec
}

func TestHandleBackupCreateSkipsAnUnreadableRetroGamesIndex(t *testing.T) {
	t.Parallel()
	cases := map[string]func(t *testing.T, path string){
		"garbage": func(t *testing.T, path string) {
			garbage := bytes.Repeat([]byte("this is not an sqlite database. "), 128)
			if err := os.WriteFile(path, garbage, 0o600); err != nil {
				t.Fatal(err)
			}
		},
		"directory": func(t *testing.T, path string) {
			if err := os.Mkdir(path, 0o700); err != nil {
				t.Fatal(err)
			}
		},
	}
	for name, breakIt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			s, dataDir := newBackupTestServer(t)
			index := filepath.Join(dataDir, config.RetroGamesDirName, config.RetroGamesDBFilename)
			if err := os.MkdirAll(filepath.Dir(index), 0o700); err != nil {
				t.Fatal(err)
			}
			breakIt(t, index)

			rec := createTestBackup(t, s)
			if rec.Code != http.StatusOK {
				t.Fatalf("an unreadable game index failed the backup: %d %s", rec.Code, rec.Body.String())
			}
			zr, err := zip.NewReader(bytes.NewReader(rec.Body.Bytes()), int64(rec.Body.Len()))
			if err != nil {
				t.Fatal(err)
			}
			entries := map[string]bool{}
			for _, f := range zr.File {
				entries[f.Name] = true
			}
			for _, want := range []string{"config.yaml", "data/short_term.db", "data/galaxa.db", "manifest.json"} {
				if !entries[want] {
					t.Fatalf("backup misses %s: %v", want, entries)
				}
			}
			if entries["data/retrogames/retrogames.db"] {
				t.Fatal("the unreadable index was put into the backup")
			}
			mf, err := zr.Open("manifest.json")
			if err != nil {
				t.Fatal(err)
			}
			defer mf.Close()
			var manifest agoManifest
			if err := json.NewDecoder(mf).Decode(&manifest); err != nil {
				t.Fatal(err)
			}
			if !containsString(manifest.Contents, "sqlite/retrogames.db skipped (snapshot failed)") {
				t.Fatalf("manifest does not report the skipped index: %v", manifest.Contents)
			}
			if !containsString(manifest.Contents, "sqlite/ (2 consistent database snapshot(s))") {
				t.Fatalf("the other databases were not snapshotted: %v", manifest.Contents)
			}
		})
	}
}

func TestHandleBackupCreateIncludesAReadableRetroGamesIndex(t *testing.T) {
	t.Parallel()
	s, dataDir := newBackupTestServer(t)
	index := filepath.Join(dataDir, config.RetroGamesDirName, config.RetroGamesDBFilename)
	if err := os.MkdirAll(filepath.Dir(index), 0o700); err != nil {
		t.Fatal(err)
	}
	writeTestSQLiteDB(t, index)
	rec := createTestBackup(t, s)
	if rec.Code != http.StatusOK {
		t.Fatalf("backup: %d %s", rec.Code, rec.Body.String())
	}
	zr, err := zip.NewReader(bytes.NewReader(rec.Body.Bytes()), int64(rec.Body.Len()))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range zr.File {
		if f.Name == "data/retrogames/retrogames.db" {
			return
		}
	}
	t.Fatal("a readable game index is missing from the backup")
}

// Every other database keeps the policy that a snapshot failure fails the
// whole backup.
func TestHandleBackupCreateFailsOnAnUnreadableDatabase(t *testing.T) {
	t.Parallel()
	s, dataDir := newBackupTestServer(t)
	garbage := bytes.Repeat([]byte("this is not an sqlite database. "), 128)
	if err := os.WriteFile(filepath.Join(dataDir, config.GalaxaDBFilename), garbage, 0o600); err != nil {
		t.Fatal(err)
	}
	rec := createTestBackup(t, s)
	if rec.Code != http.StatusInternalServerError || !strings.Contains(rec.Body.String(), "consistent SQLite backup") {
		t.Fatalf("an unreadable galaxa.db must fail the backup: %d %s", rec.Code, rec.Body.String())
	}
}
