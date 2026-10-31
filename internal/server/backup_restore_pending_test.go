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
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/restorepending"
)

// applyPendingRestoreForTest simulates the next start of the process.
func applyPendingRestoreForTest(t *testing.T, configPath string, vectorRoot ...string) {
	t.Helper()
	root, err := restorepending.ConfigRoot(configPath)
	if err != nil {
		t.Fatal(err)
	}
	opts := restorepending.Options{}
	if len(vectorRoot) > 0 {
		opts.ResolveVectorRoot = func() (string, error) { return vectorRoot[0], nil }
	}
	res, err := restorepending.Apply(root, opts)
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
	return importArchiveWithPassword(t, s, files, "")
}

func importArchiveWithPassword(t *testing.T, s *Server, files map[string][]byte, password string) *httptest.ResponseRecorder {
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
	if password != "" {
		_ = mw.WriteField("password", password)
	}
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

func TestBackupImportChecksAppVersionEvenWithoutLiveDatabases(t *testing.T) {
	s, root := newPendingTestServer(t)
	versionFile := filepath.Join(root, ".version")
	if err := os.WriteFile(versionFile, []byte("1.4.0\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	files := func(v string) map[string][]byte {
		return map[string][]byte{"data/state.json": []byte("x"), "manifest.json": []byte(`{"version":"1","app_version":"` + v + `"}`)}
	}
	if rec := importArchiveForTest(t, s, files("v1.5.0")); rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "neueren AuraGo-Version") {
		t.Fatalf("newer app version: %d %s", rec.Code, rec.Body.String())
	}
	if rec := importArchiveForTest(t, s, files("1.3.9")); rec.Code != http.StatusOK {
		t.Fatalf("older app version: %d %s", rec.Code, rec.Body.String())
	}
	rec := importArchiveForTest(t, s, files("nightly-abc"))
	var payload struct {
		Warnings []string `json:"warnings"`
	}
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &payload) != nil || len(payload.Warnings) != 1 {
		t.Fatalf("unparseable version: %d %s", rec.Code, rec.Body.String())
	}
	_ = os.Remove(versionFile)
	rec = importArchiveForTest(t, s, files("9.9.9"))
	payload.Warnings = nil
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &payload) != nil || len(payload.Warnings) != 1 {
		t.Fatalf("unknown installed version must warn only: %d %s", rec.Code, rec.Body.String())
	}
}

func TestBackupImportStagesVaultSecretsAndDeleteDiscardsRestore(t *testing.T) {
	s, root := newPendingTestServer(t)
	srcVault := newTestVault(t, filepath.Join(t.TempDir(), "vault.bin"), "05")
	if err := srcVault.WriteSecret("api_key", "from-backup"); err != nil {
		t.Fatal(err)
	}
	blob, err := exportVaultSecrets(srcVault, "pw")
	if err != nil || len(blob) == 0 {
		t.Fatalf("export: %v", err)
	}
	s.Vault = newTestVault(t, filepath.Join(root, "data", "vault.bin"), "06")
	archive := map[string][]byte{"data/state.json": []byte("restored"), "vault_secrets.enc": blob}

	rec := importArchiveWithPassword(t, s, archive, "pw")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	var payload map[string]interface{}
	_ = json.Unmarshal(rec.Body.Bytes(), &payload)
	if payload["vault_restored"] != float64(1) || payload["status"] != "ok" {
		t.Fatalf("payload = %v", payload)
	}
	if _, err := s.Vault.ReadSecret("api_key"); err == nil {
		t.Fatal("secrets must not be live before the restart")
	}

	// Applied: files + secrets arrive at the "next start".
	applyPendingRestoreForTest(t, s.Cfg.ConfigPath)
	restorepending.ApplyStagedVault(root, s.Vault, nil)
	if got, err := s.Vault.ReadSecret("api_key"); err != nil || got != "from-backup" {
		t.Fatalf("secret after start = %q err=%v", got, err)
	}

	// Discard: a second import is dropped via DELETE and nothing is applied.
	if err := s.Vault.DeleteSecret("api_key"); err != nil {
		t.Fatal(err)
	}
	if rec := importArchiveWithPassword(t, s, archive, "pw"); rec.Code != http.StatusOK {
		t.Fatalf("second import: %d", rec.Code)
	}
	del := httptest.NewRecorder()
	handleBackupRestoreStatus(s).ServeHTTP(del, httptest.NewRequest(http.MethodDelete, "/api/backup/restore-status", nil))
	if del.Code != http.StatusOK || !strings.Contains(del.Body.String(), `"discarded":true`) {
		t.Fatalf("delete: %d %s", del.Code, del.Body.String())
	}
	if restorepending.ReadState(root).Pending {
		t.Fatal("pending must be gone")
	}
	restorepending.ApplyStagedVault(root, s.Vault, nil)
	if _, err := s.Vault.ReadSecret("api_key"); err == nil {
		t.Fatal("discarded restore must not bring secrets")
	}
	del = httptest.NewRecorder()
	handleBackupRestoreStatus(s).ServeHTTP(del, httptest.NewRequest(http.MethodDelete, "/api/backup/restore-status", nil))
	if !strings.Contains(del.Body.String(), `"discarded":false`) {
		t.Fatalf("second delete: %s", del.Body.String())
	}
}

func TestBackupCreateAllowsLargeSQLiteAndSkipsOversizedDocuments(t *testing.T) {
	old := agoMaxBackupEntrySize
	agoMaxBackupEntrySize = 4096
	t.Cleanup(func() { agoMaxBackupEntrySize = old })

	s, root := newPendingTestServer(t)
	dataDir := filepath.Join(root, "data")
	s.Cfg.Directories.DataDir = dataDir
	s.Cfg.Directories.PromptsDir = filepath.Join(root, "prompts")
	s.Cfg.Directories.SkillsDir = filepath.Join(root, "agent_workspace", "skills")
	s.Cfg.Directories.ToolsDir = filepath.Join(root, "agent_workspace", "tools")
	s.Cfg.Directories.WorkspaceDir = filepath.Join(root, "agent_workspace", "workdir")
	s.Cfg.SQLite.InventoryPath = filepath.Join(dataDir, "inventory.db")

	db, err := sql.Open("sqlite", s.Cfg.SQLite.InventoryPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE t (v TEXT)`); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 40; i++ {
		if _, err := db.Exec(`INSERT INTO t VALUES (?)`, strings.Repeat("x", 1000)); err != nil {
			t.Fatal(err)
		}
	}
	db.Close()
	if info, _ := os.Stat(s.Cfg.SQLite.InventoryPath); info.Size() <= agoMaxBackupEntrySize {
		t.Fatalf("test database too small: %d", info.Size())
	}
	if err := os.MkdirAll(filepath.Join(dataDir, "documents"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "documents", "big.bin"), make([]byte, 8192), 0o600); err != nil {
		t.Fatal(err)
	}

	create := func() *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		handleBackupCreate(s).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/backup/create", strings.NewReader(`{}`)))
		return rec
	}
	rec := create()
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	zr, err := zip.NewReader(bytes.NewReader(rec.Body.Bytes()), int64(rec.Body.Len()))
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	manifest := ""
	for _, f := range zr.File {
		names[f.Name] = true
		if f.Name == "manifest.json" {
			rc, _ := f.Open()
			b := new(bytes.Buffer)
			b.ReadFrom(rc)
			rc.Close()
			manifest = b.String()
		}
	}
	if !names["data/inventory.db"] || names["data/documents/big.bin"] || !strings.Contains(manifest, "big.bin") {
		t.Fatalf("names=%v manifest=%s", names, manifest)
	}

	// A mandatory runtime file over the limit still fails and names file + size.
	if err := os.WriteFile(filepath.Join(dataDir, "chat_history.json"), make([]byte, 8192), 0o600); err != nil {
		t.Fatal(err)
	}
	rec = create()
	if rec.Code != http.StatusInternalServerError || !strings.Contains(rec.Body.String(), "chat_history.json") || !strings.Contains(rec.Body.String(), "MiB") {
		t.Fatalf("oversized runtime file: %d %s", rec.Code, rec.Body.String())
	}
}
