package server

import (
	"archive/zip"
	"bytes"
	"encoding/json"
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

func TestHandleBackupImportMasksDecryptionErrors(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	s := &Server{Cfg: cfg, Logger: slog.Default()}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "backup.ago")
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	if _, err := part.Write([]byte("AGOEdefinitely-not-valid")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := writer.WriteField("password", "wrong-password"); err != nil {
		t.Fatalf("WriteField: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/backup/import", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()

	handleBackupImport(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	var payload map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if payload["error"] != "decryption_failed" {
		t.Fatalf("error = %q, want decryption_failed", payload["error"])
	}
	if payload["message"] == "" || bytes.Contains([]byte(payload["message"]), []byte("cipher")) {
		t.Fatalf("expected redacted decryption message, got %q", payload["message"])
	}
}

func TestHandleBackupImportRestoresIntoConfigRoot(t *testing.T) {
	instanceRoot := t.TempDir()
	otherCWD := t.TempDir()
	configPath := filepath.Join(instanceRoot, "config.yaml")
	if err := os.WriteFile(configPath, []byte("server:\n  port: 1234\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(config): %v", err)
	}

	origWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	if err := os.Chdir(otherCWD); err != nil {
		t.Fatalf("Chdir(otherCWD): %v", err)
	}
	defer os.Chdir(origWD)

	cfg := &config.Config{ConfigPath: configPath}
	s := &Server{Cfg: cfg, Logger: slog.Default()}

	zipBuf := &bytes.Buffer{}
	zw := zip.NewWriter(zipBuf)
	w, err := zw.Create("data/state.json")
	if err != nil {
		t.Fatalf("Create zip entry: %v", err)
	}
	if _, err := w.Write([]byte(`{"restored":true}`)); err != nil {
		t.Fatalf("Write zip entry: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("Close zip: %v", err)
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "backup.ago")
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	if _, err := part.Write(zipBuf.Bytes()); err != nil {
		t.Fatalf("Write upload: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close multipart: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/backup/import", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()

	handleBackupImport(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	restoredPath := filepath.Join(instanceRoot, "data", "state.json")
	data, err := os.ReadFile(restoredPath)
	if err != nil {
		t.Fatalf("ReadFile(restored): %v", err)
	}
	if string(data) != `{"restored":true}` {
		t.Fatalf("restored content = %q", string(data))
	}
	if _, err := os.Stat(filepath.Join(otherCWD, "data", "state.json")); !os.IsNotExist(err) {
		t.Fatalf("restore unexpectedly wrote to cwd: err=%v", err)
	}
}

func TestHandleBackupImportRejectsPathTraversal(t *testing.T) {
	t.Parallel()

	instanceRoot := t.TempDir()
	configPath := filepath.Join(instanceRoot, "config.yaml")
	if err := os.WriteFile(configPath, []byte("server:\n  port: 1234\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(config): %v", err)
	}

	cfg := &config.Config{ConfigPath: configPath}
	s := &Server{Cfg: cfg, Logger: slog.Default()}

	zipBuf := &bytes.Buffer{}
	zw := zip.NewWriter(zipBuf)
	w, err := zw.Create("../evil.txt")
	if err != nil {
		t.Fatalf("Create zip entry: %v", err)
	}
	if _, err := w.Write([]byte("nope")); err != nil {
		t.Fatalf("Write zip entry: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("Close zip: %v", err)
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "backup.ago")
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	if _, err := part.Write(zipBuf.Bytes()); err != nil {
		t.Fatalf("Write upload: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close multipart: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/backup/import", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()

	handleBackupImport(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	if _, err := os.Stat(filepath.Join(instanceRoot, "evil.txt")); !os.IsNotExist(err) {
		t.Fatalf("unexpected restored traversal target: err=%v", err)
	}
}

func TestHandleBackupImportRejectsSymlinkEntries(t *testing.T) {
	t.Parallel()

	instanceRoot := t.TempDir()
	configPath := filepath.Join(instanceRoot, "config.yaml")
	if err := os.WriteFile(configPath, []byte("server:\n  port: 1234\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(config): %v", err)
	}

	cfg := &config.Config{ConfigPath: configPath}
	s := &Server{Cfg: cfg, Logger: slog.Default()}

	zipBuf := &bytes.Buffer{}
	zw := zip.NewWriter(zipBuf)
	header := &zip.FileHeader{Name: "link.txt", Method: zip.Deflate}
	header.SetMode(os.ModeSymlink)
	w, err := zw.CreateHeader(header)
	if err != nil {
		t.Fatalf("CreateHeader: %v", err)
	}
	if _, err := w.Write([]byte("ignored")); err != nil {
		t.Fatalf("Write zip entry: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("Close zip: %v", err)
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "backup.ago")
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	if _, err := part.Write(zipBuf.Bytes()); err != nil {
		t.Fatalf("Write upload: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close multipart: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/backup/import", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()

	handleBackupImport(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestHandleBackupCreateIncludesRuntimeFilesAndSQLiteSidecars(t *testing.T) {
	t.Parallel()

	instanceRoot := t.TempDir()
	dataDir := filepath.Join(instanceRoot, "data")
	promptsDir := filepath.Join(instanceRoot, "prompts")
	skillsDir := filepath.Join(instanceRoot, "agent_workspace", "skills")
	toolsDir := filepath.Join(instanceRoot, "agent_workspace", "tools")
	workdir := filepath.Join(instanceRoot, "agent_workspace", "workdir")
	for _, dir := range []string{dataDir, promptsDir, skillsDir, toolsDir, workdir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("MkdirAll(%s): %v", dir, err)
		}
	}

	configPath := filepath.Join(instanceRoot, "config.yaml")
	if err := os.WriteFile(configPath, []byte("server:\n  port: 1234\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(config): %v", err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "tokens.json"), []byte(`{"tokens":[]}`), 0o600); err != nil {
		t.Fatalf("WriteFile(tokens): %v", err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "missions_v2.json"), []byte(`[]`), 0o600); err != nil {
		t.Fatalf("WriteFile(missions): %v", err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "background_tasks.json"), []byte(`[]`), 0o600); err != nil {
		t.Fatalf("WriteFile(background_tasks): %v", err)
	}
	shortTermPath := filepath.Join(dataDir, "short_term.db")
	if err := os.WriteFile(shortTermPath, []byte("db"), 0o600); err != nil {
		t.Fatalf("WriteFile(short_term.db): %v", err)
	}
	if err := os.WriteFile(shortTermPath+"-wal", []byte("wal"), 0o600); err != nil {
		t.Fatalf("WriteFile(short_term.db-wal): %v", err)
	}
	missionHistoryPath := filepath.Join(dataDir, "mission_history.db")
	if err := os.WriteFile(missionHistoryPath, []byte("mh"), 0o600); err != nil {
		t.Fatalf("WriteFile(mission_history.db): %v", err)
	}

	cfg := &config.Config{
		ConfigPath: configPath,
	}
	cfg.Directories.DataDir = dataDir
	cfg.Directories.PromptsDir = promptsDir
	cfg.Directories.SkillsDir = skillsDir
	cfg.Directories.ToolsDir = toolsDir
	cfg.Directories.WorkspaceDir = workdir
	cfg.SQLite.ShortTermPath = shortTermPath
	cfg.SQLite.MissionHistoryPath = missionHistoryPath

	s := &Server{Cfg: cfg, Logger: slog.Default()}

	req := httptest.NewRequest(http.MethodPost, "/api/backup/create", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handleBackupCreate(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	zr, err := zip.NewReader(bytes.NewReader(rec.Body.Bytes()), int64(rec.Body.Len()))
	if err != nil {
		t.Fatalf("zip.NewReader: %v", err)
	}

	entries := map[string]bool{}
	for _, f := range zr.File {
		entries[f.Name] = true
	}

	for _, want := range []string{
		"config.yaml",
		"data/tokens.json",
		"data/missions_v2.json",
		"data/background_tasks.json",
		"data/short_term.db",
		"data/short_term.db-wal",
		"data/mission_history.db",
		"manifest.json",
	} {
		if !entries[want] {
			t.Fatalf("backup missing %s; entries=%v", want, entries)
		}
	}

	manifest, err := zr.Open("manifest.json")
	if err != nil {
		t.Fatalf("Open manifest: %v", err)
	}
	defer manifest.Close()
	manifestData, err := io.ReadAll(manifest)
	if err != nil {
		t.Fatalf("ReadAll manifest: %v", err)
	}
	if !bytes.Contains(manifestData, []byte("sqlite/")) {
		t.Fatalf("manifest does not mention sqlite backup: %s", string(manifestData))
	}
}
