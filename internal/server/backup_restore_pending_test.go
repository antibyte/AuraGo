package server

import (
	"archive/zip"
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"aurago/internal/config"
	"aurago/internal/restorepending"
)

// applyPendingRestoreForTest simulates the next start of the process.
func applyPendingRestoreForTest(t *testing.T, configPath string) {
	t.Helper()
	root, err := restorepending.ConfigRoot(configPath)
	if err != nil {
		t.Fatal(err)
	}
	res, err := restorepending.Apply(root, restorepending.Options{})
	if err != nil || res == nil || res.Status != restorepending.StatusApplied {
		t.Fatalf("apply pending restore: res=%+v err=%v", res, err)
	}
}

func newPendingTestServer(t *testing.T) (*Server, string) {
	t.Helper()
	root := t.TempDir()
	configPath := filepath.Join(root, "config.yaml")
	if err := os.WriteFile(configPath, []byte("server:\n  port: 1234\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "data"), 0o755); err != nil {
		t.Fatal(err)
	}
	return &Server{Cfg: &config.Config{ConfigPath: configPath}, Logger: slog.Default()}, root
}

func makeUserVersionDB(t *testing.T, version int) []byte {
	t.Helper()
	path := filepath.Join(t.TempDir(), "v.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE t (id INTEGER PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, version)); err != nil {
		t.Fatal(err)
	}
	db.Close()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func importArchiveForTest(t *testing.T, s *Server, files map[string][]byte) *httptest.ResponseRecorder {
	t.Helper()
	var zbuf bytes.Buffer
	zw := zip.NewWriter(&zbuf)
	for name, data := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		w.Write(data)
	}
	zw.Close()
	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	part, _ := mw.CreateFormFile("file", "backup.ago")
	part.Write(zbuf.Bytes())
	mw.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/backup/import", body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	handleBackupImport(s).ServeHTTP(rec, req)
	return rec
}

func TestBackupImportStagesInsteadOfPublishingAndReportsStatus(t *testing.T) {
	s, root := newPendingTestServer(t)
	livePath := filepath.Join(root, "data", "state.json")
	if err := os.WriteFile(livePath, []byte("live"), 0o600); err != nil {
		t.Fatal(err)
	}
	rec := importArchiveForTest(t, s, map[string][]byte{
		"data/state.json": []byte("restored"),
		"manifest.json":   []byte(`{"version":"1"}`),
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["status"] != "ok" || payload["pending_restore"] != true || payload["restart_required"] != true || payload["staged"] != float64(1) {
		t.Fatalf("contract violated: %v", payload)
	}
	for _, k := range []string{"vault_restored", "tokens_restored", "warnings", "message"} {
		if _, ok := payload[k]; !ok {
			t.Fatalf("missing field %q: %v", k, payload)
		}
	}
	if _, ok := payload["skipped"]; ok {
		t.Fatal("skipped must be gone")
	}
	if b, _ := os.ReadFile(livePath); string(b) != "live" {
		t.Fatalf("live file touched by import: %q", b)
	}

	statusRec := httptest.NewRecorder()
	handleBackupRestoreStatus(s).ServeHTTP(statusRec, httptest.NewRequest(http.MethodGet, "/api/backup/restore-status", nil))
	var st map[string]interface{}
	if err := json.Unmarshal(statusRec.Body.Bytes(), &st); err != nil || st["pending"] != true || st["last"] != nil {
		t.Fatalf("status = %v err=%v", st, err)
	}

	applyPendingRestoreForTest(t, s.Cfg.ConfigPath)
	if b, _ := os.ReadFile(livePath); string(b) != "restored" {
		t.Fatalf("not applied: %q", b)
	}
}

func TestBackupImportRejectsNewerDatabaseAndCorruptManifest(t *testing.T) {
	s, root := newPendingTestServer(t)
	if err := os.WriteFile(filepath.Join(root, "data", "tresor.db"), makeUserVersionDB(t, 1), 0o600); err != nil {
		t.Fatal(err)
	}
	rec := importArchiveForTest(t, s, map[string][]byte{"data/tresor.db": makeUserVersionDB(t, 2)})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("newer backup: status %d: %s", rec.Code, rec.Body.String())
	}
	if restorepending.ReadState(root).Pending {
		t.Fatal("rejected import must not leave a pending restore")
	}

	rec = importArchiveForTest(t, s, map[string][]byte{"data/state.json": []byte("x"), "manifest.json": []byte("{not json")})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("corrupt manifest: status %d: %s", rec.Code, rec.Body.String())
	}

	// Older backups and backups without manifest are allowed with warnings.
	if err := os.WriteFile(filepath.Join(root, "data", "tresor.db"), makeUserVersionDB(t, 3), 0o600); err != nil {
		t.Fatal(err)
	}
	rec = importArchiveForTest(t, s, map[string][]byte{"data/tresor.db": makeUserVersionDB(t, 2)})
	if rec.Code != http.StatusOK {
		t.Fatalf("older backup: status %d: %s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Warnings []string `json:"warnings"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil || len(payload.Warnings) != 2 {
		t.Fatalf("warnings = %v err=%v", payload.Warnings, err)
	}
}
