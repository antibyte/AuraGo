package server

import (
	"archive/zip"
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"database/sql"
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
	"aurago/internal/security"

	_ "modernc.org/sqlite"
)

func writeTestSQLiteDB(t *testing.T, path string) {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("Open sqlite %s: %v", path, err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS test_items (id INTEGER PRIMARY KEY, name TEXT)`); err != nil {
		t.Fatalf("Create sqlite table %s: %v", path, err)
	}
	if _, err := db.Exec(`INSERT INTO test_items (name) VALUES ('backup-test')`); err != nil {
		t.Fatalf("Insert sqlite row %s: %v", path, err)
	}
}

func newTestVault(t *testing.T, path, keyByte string) *security.Vault {
	t.Helper()
	vault, err := security.NewVault(strings.Repeat(keyByte, 32), path)
	if err != nil {
		t.Fatalf("NewVault: %v", err)
	}
	return vault
}

func TestEncryptAGOUsesVersionedArgon2idFormat(t *testing.T) {
	t.Parallel()

	plain := []byte("zip data")
	encrypted, err := encryptAGO(plain, "correct horse battery staple")
	if err != nil {
		t.Fatalf("encryptAGO: %v", err)
	}
	if len(encrypted) < len(agoMagic)+1 {
		t.Fatalf("encrypted data too short: %d", len(encrypted))
	}
	if string(encrypted[:len(agoMagic)]) != agoMagic {
		t.Fatalf("missing magic prefix")
	}
	if encrypted[len(agoMagic)] != agoKDFArgon2ID {
		t.Fatalf("kdf marker = %d, want %d", encrypted[len(agoMagic)], agoKDFArgon2ID)
	}

	decrypted, err := decryptAGO(encrypted, "correct horse battery staple")
	if err != nil {
		t.Fatalf("decryptAGO: %v", err)
	}
	if !bytes.Equal(decrypted, plain) {
		t.Fatalf("decrypted = %q, want %q", decrypted, plain)
	}
}

func TestDecryptAGOLegacySHA256FormatStillWorks(t *testing.T) {
	t.Parallel()

	plain := []byte("legacy zip data")
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		t.Fatalf("rand.Read(salt): %v", err)
	}
	key := deriveLegacyBackupKey("old password", salt)
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatalf("NewCipher: %v", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatalf("NewGCM: %v", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		t.Fatalf("rand.Read(nonce): %v", err)
	}

	var legacy bytes.Buffer
	legacy.WriteString(agoMagic)
	legacy.Write(salt)
	legacy.Write(nonce)
	legacy.Write(gcm.Seal(nil, nonce, plain, nil))

	decrypted, err := decryptAGO(legacy.Bytes(), "old password")
	if err != nil {
		t.Fatalf("decryptAGO legacy: %v", err)
	}
	if !bytes.Equal(decrypted, plain) {
		t.Fatalf("decrypted = %q, want %q", decrypted, plain)
	}
}

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

func TestHandleBackupImportRejectsUnsupportedRestorePath(t *testing.T) {
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
	w, err := zw.Create("start.bat")
	if err != nil {
		t.Fatalf("Create zip entry: %v", err)
	}
	if _, err := w.Write([]byte("echo owned")); err != nil {
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
	if _, err := os.Stat(filepath.Join(instanceRoot, "start.bat")); !os.IsNotExist(err) {
		t.Fatalf("unsupported restore path was written: err=%v", err)
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

func TestHandleBackupImportRejectsCorruptSQLiteBeforeOverwrite(t *testing.T) {
	t.Parallel()

	instanceRoot := t.TempDir()
	dataDir := filepath.Join(instanceRoot, "data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(data): %v", err)
	}
	configPath := filepath.Join(instanceRoot, "config.yaml")
	if err := os.WriteFile(configPath, []byte("server:\n  port: 1234\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(config): %v", err)
	}
	dbPath := filepath.Join(dataDir, "short_term.db")
	if err := os.WriteFile(dbPath, []byte("original-db"), 0o600); err != nil {
		t.Fatalf("WriteFile(existing db): %v", err)
	}

	cfg := &config.Config{ConfigPath: configPath}
	s := &Server{Cfg: cfg, Logger: slog.Default()}

	zipBuf := &bytes.Buffer{}
	zw := zip.NewWriter(zipBuf)
	w, err := zw.Create("data/short_term.db")
	if err != nil {
		t.Fatalf("Create zip entry: %v", err)
	}
	if _, err := w.Write([]byte("not a sqlite database")); err != nil {
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
	data, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatalf("ReadFile(existing db): %v", err)
	}
	if string(data) != "original-db" {
		t.Fatalf("corrupt restore overwrote existing db: %q", string(data))
	}
}

func TestHandleBackupImportSQLiteRestoreRequiresRestart(t *testing.T) {
	t.Parallel()

	instanceRoot := t.TempDir()
	dataDir := filepath.Join(instanceRoot, "data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(data): %v", err)
	}
	configPath := filepath.Join(instanceRoot, "config.yaml")
	if err := os.WriteFile(configPath, []byte("server:\n  port: 1234\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(config): %v", err)
	}
	srcDB := filepath.Join(t.TempDir(), "valid.db")
	db, err := sql.Open("sqlite", srcDB)
	if err != nil {
		t.Fatalf("Open sqlite: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE test_items (id INTEGER PRIMARY KEY, name TEXT)`); err != nil {
		db.Close()
		t.Fatalf("Create table: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("Close sqlite: %v", err)
	}
	dbBytes, err := os.ReadFile(srcDB)
	if err != nil {
		t.Fatalf("ReadFile(valid db): %v", err)
	}

	cfg := &config.Config{ConfigPath: configPath}
	s := &Server{Cfg: cfg, Logger: slog.Default()}

	zipBuf := &bytes.Buffer{}
	zw := zip.NewWriter(zipBuf)
	w, err := zw.Create("data/short_term.db")
	if err != nil {
		t.Fatalf("Create zip entry: %v", err)
	}
	if _, err := w.Write(dbBytes); err != nil {
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
	var payload map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if payload["restart_required"] != true {
		t.Fatalf("restart_required = %#v, want true; payload=%v", payload["restart_required"], payload)
	}
}

func TestHandleBackupCreateIncludesRuntimeFilesAndConsistentSQLiteSnapshots(t *testing.T) {
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
	if err := os.WriteFile(filepath.Join(dataDir, "missions_v2_queue.json"), []byte(`[]`), 0o600); err != nil {
		t.Fatalf("WriteFile(missions queue): %v", err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "background_tasks.json"), []byte(`[]`), 0o600); err != nil {
		t.Fatalf("WriteFile(background_tasks): %v", err)
	}
	shortTermPath := filepath.Join(dataDir, "short_term.db")
	writeTestSQLiteDB(t, shortTermPath)
	if err := os.WriteFile(shortTermPath+"-wal", []byte("wal"), 0o600); err != nil {
		t.Fatalf("WriteFile(short_term.db-wal): %v", err)
	}
	missionHistoryPath := filepath.Join(dataDir, "mission_history.db")
	writeTestSQLiteDB(t, missionHistoryPath)

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
		"data/missions_v2_queue.json",
		"data/background_tasks.json",
		"data/short_term.db",
		"data/mission_history.db",
		"manifest.json",
	} {
		if !entries[want] {
			t.Fatalf("backup missing %s; entries=%v", want, entries)
		}
	}
	if entries["data/short_term.db-wal"] {
		t.Fatal("backup should contain a consistent SQLite snapshot, not live WAL sidecars")
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
	if !bytes.Contains(manifestData, []byte("consistent database snapshot")) {
		t.Fatalf("manifest does not mention sqlite backup: %s", string(manifestData))
	}
}

func TestHandleBackupCreateRequiresPasswordForPlaintextConfigSecrets(t *testing.T) {
	t.Parallel()

	instanceRoot := t.TempDir()
	dataDir := filepath.Join(instanceRoot, "data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(data): %v", err)
	}
	configPath := filepath.Join(instanceRoot, "config.yaml")
	configData := []byte("providers:\n  - id: main\n    api_key: plain-config-secret\n")
	if err := os.WriteFile(configPath, configData, 0o600); err != nil {
		t.Fatalf("WriteFile(config): %v", err)
	}

	cfg := &config.Config{ConfigPath: configPath}
	cfg.Directories.DataDir = dataDir
	s := &Server{Cfg: cfg, Logger: slog.Default()}

	req := httptest.NewRequest(http.MethodPost, "/api/backup/create", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handleBackupCreate(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestEncryptedBackupRestoresTokenStoreWithNewVaultKey(t *testing.T) {
	t.Parallel()

	sourceRoot := t.TempDir()
	sourceData := filepath.Join(sourceRoot, "data")
	if err := os.MkdirAll(sourceData, 0o755); err != nil {
		t.Fatalf("MkdirAll(sourceData): %v", err)
	}
	sourceConfig := filepath.Join(sourceRoot, "config.yaml")
	if err := os.WriteFile(sourceConfig, []byte("server:\n  port: 1234\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(source config): %v", err)
	}
	sourceVault := newTestVault(t, filepath.Join(sourceData, "vault.bin"), "01")
	sourceTM, err := security.NewTokenManager(sourceVault, filepath.Join(sourceData, "tokens.json"))
	if err != nil {
		t.Fatalf("NewTokenManager(source): %v", err)
	}
	if _, _, err := sourceTM.Create("admin token", []string{"admin"}, nil); err != nil {
		t.Fatalf("Create token: %v", err)
	}

	sourceCfg := &config.Config{ConfigPath: sourceConfig}
	sourceCfg.Directories.DataDir = sourceData
	sourceCfg.Directories.PromptsDir = filepath.Join(sourceRoot, "prompts")
	sourceCfg.Directories.SkillsDir = filepath.Join(sourceRoot, "agent_workspace", "skills")
	sourceCfg.Directories.ToolsDir = filepath.Join(sourceRoot, "agent_workspace", "tools")
	sourceCfg.Directories.WorkspaceDir = filepath.Join(sourceRoot, "agent_workspace", "workdir")
	for _, dir := range []string{sourceCfg.Directories.PromptsDir, sourceCfg.Directories.SkillsDir, sourceCfg.Directories.ToolsDir, sourceCfg.Directories.WorkspaceDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("MkdirAll(%s): %v", dir, err)
		}
	}
	sourceServer := &Server{Cfg: sourceCfg, Vault: sourceVault, Logger: slog.Default()}

	createReq := httptest.NewRequest(http.MethodPost, "/api/backup/create", strings.NewReader(`{"password":"portable"}`))
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	handleBackupCreate(sourceServer).ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusOK {
		t.Fatalf("create status = %d, want %d; body=%s", createRec.Code, http.StatusOK, createRec.Body.String())
	}

	destRoot := t.TempDir()
	destData := filepath.Join(destRoot, "data")
	if err := os.MkdirAll(destData, 0o755); err != nil {
		t.Fatalf("MkdirAll(destData): %v", err)
	}
	destConfig := filepath.Join(destRoot, "config.yaml")
	if err := os.WriteFile(destConfig, []byte("server:\n  port: 4321\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(dest config): %v", err)
	}
	destVault := newTestVault(t, filepath.Join(destData, "vault.bin"), "02")
	destCfg := &config.Config{ConfigPath: destConfig}
	destCfg.Directories.DataDir = destData
	destServer := &Server{Cfg: destCfg, Vault: destVault, Logger: slog.Default()}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "backup.ago")
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	if _, err := part.Write(createRec.Body.Bytes()); err != nil {
		t.Fatalf("Write upload: %v", err)
	}
	if err := writer.WriteField("password", "portable"); err != nil {
		t.Fatalf("WriteField(password): %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close multipart: %v", err)
	}

	importReq := httptest.NewRequest(http.MethodPost, "/api/backup/import", body)
	importReq.Header.Set("Content-Type", writer.FormDataContentType())
	importRec := httptest.NewRecorder()
	handleBackupImport(destServer).ServeHTTP(importRec, importReq)
	if importRec.Code != http.StatusOK {
		t.Fatalf("import status = %d, want %d; body=%s", importRec.Code, http.StatusOK, importRec.Body.String())
	}

	destTM, err := security.NewTokenManager(destVault, filepath.Join(destData, "tokens.json"))
	if err != nil {
		t.Fatalf("NewTokenManager(dest): %v", err)
	}
	if got := len(destTM.List()); got != 1 {
		t.Fatalf("restored token count = %d, want 1", got)
	}
}
