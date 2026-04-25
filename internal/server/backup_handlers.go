package server

// Backup & Restore — creates/imports .ago archives (ZIP-based, optionally AES-256-GCM encrypted).
//
// .ago file format:
//   Plain (no password):    standard ZIP file, rename to .zip to inspect.
//   Encrypted v2:           4-byte magic "AGOE" + KDF byte + 16-byte salt + nonce + AES-256-GCM(ZIP).
//   Encrypted legacy:       4-byte magic "AGOE" + 16-byte salt + 12-byte nonce + AES-256-GCM(ZIP).
//
// Vault secrets are included as vault_secrets.enc only when a backup password is set.
// They are encrypted independently with AES-256-GCM derived from the same password so they
// can be migrated to a new instance that has a different AURAGO_MASTER_KEY (e.g. systemd).

import (
	"archive/zip"
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/security"
	promptsembed "aurago/prompts"
	"golang.org/x/crypto/argon2"
)

const agoMagic = "AGOE"

const (
	agoMaxExtractSize      int64  = 1 << 30
	agoMaxArchiveEntries          = 100000
	agoMaxCompressionRatio uint64 = 1000
	agoSaltSize                   = 16
	agoKDFArgon2ID         byte   = 1
)

const vaultSecretsMagic = "AGOV"

// currentDBSchemaVersion is incremented whenever any SQLite schema changes.
// Stored in the manifest so imports can warn about version mismatches.
const currentDBSchemaVersion = 1

// agoManifest is written as manifest.json at the ZIP root.
type agoManifest struct {
	Version         string   `json:"version"`
	CreatedAt       string   `json:"created_at"`
	Hostname        string   `json:"hostname"`
	Contents        []string `json:"contents"`
	VaultIncluded   bool     `json:"vault_included,omitempty"`
	DBSchemaVersion int      `json:"db_schema_version,omitempty"`
}

type agoImportEntry struct {
	file    *zip.File
	clean   string
	dest    string
	isDir   bool
	mode    os.FileMode
	stage   string
	zipPath string
}

// vaultSkipKeys contains secrets that are instance-specific and must not be
// migrated between systems. All other vault keys (integration credentials) are
// included in the encrypted vault export.
var vaultSkipKeys = map[string]bool{
	// Session secret is randomly generated per-instance for JWT/cookie signing.
	// Migrating it would NOT break logins, but it's safer to let each instance
	// generate its own — existing sessions on the new instance would be invalidated
	// anyway after a restore+restart.
	"auth_session_secret": true,
}

// ── Vault export / import ────────────────────────────────────────────────────

// exportVaultSecrets reads all transferable secrets from the vault and returns
// them as an AES-256-GCM blob encrypted with the given backup password.
// Returns nil, nil if the vault is nil or has no transferable secrets.
func exportVaultSecrets(vault *security.Vault, password string) ([]byte, error) {
	if vault == nil {
		return nil, nil
	}
	keys, err := vault.ListKeys()
	if err != nil {
		return nil, fmt.Errorf("vault list: %w", err)
	}
	data := make(map[string]string, len(keys))
	for _, k := range keys {
		if vaultSkipKeys[k] {
			continue
		}
		v, err := vault.ReadSecret(k)
		if err == nil && v != "" {
			data[k] = v
		}
	}
	if len(data) == 0 {
		return nil, nil
	}
	plain, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("vault marshal: %w", err)
	}
	// Derive per-blob key with a fresh salt so it's independent of the zip encryption.
	salt := make([]byte, agoSaltSize)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}
	key := deriveBackupKey(password, salt)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	buf.WriteString(vaultSecretsMagic)
	buf.WriteByte(agoKDFArgon2ID)
	buf.Write(salt)
	buf.Write(nonce)
	buf.Write(gcm.Seal(nil, nonce, plain, nil))
	return buf.Bytes(), nil
}

// importVaultSecrets decrypts vault_secrets.enc data and writes every secret
// into the running vault. Returns the number of secrets written.
func importVaultSecrets(vault *security.Vault, encData []byte, password string) (int, error) {
	if vault == nil {
		return 0, nil
	}
	plain, err := decryptVaultSecretsBlob(encData, password)
	if err != nil {
		return 0, err
	}
	var data map[string]string
	if err := json.Unmarshal(plain, &data); err != nil {
		return 0, fmt.Errorf("vault secrets parse: %w", err)
	}
	count := 0
	for k, v := range data {
		if vaultSkipKeys[k] {
			continue
		}
		if err := vault.WriteSecret(k, v); err == nil {
			count++
		}
	}
	return count, nil
}

// ── Key derivation ───────────────────────────────────────────────────────────

// deriveBackupKey derives a 32-byte AES-256 key from a password and random salt
// using Argon2id for newly created encrypted backups.
func deriveBackupKey(password string, salt []byte) []byte {
	return argon2.IDKey([]byte(password), salt, 3, 64*1024, 4, 32)
}

// deriveLegacyBackupKey keeps old SHA-256-loop backups readable.
func deriveLegacyBackupKey(password string, salt []byte) []byte {
	h := sha256.New()
	h.Write([]byte(password))
	h.Write(salt)
	key := h.Sum(nil)
	for i := 0; i < 65535; i++ {
		h.Reset()
		h.Write(key)
		h.Write(salt)
		key = h.Sum(nil)
	}
	return key
}

// ── Encrypt / Decrypt ────────────────────────────────────────────────────────

func encryptAGO(zipData []byte, password string) ([]byte, error) {
	salt := make([]byte, agoSaltSize)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}
	key := deriveBackupKey(password, salt)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize()) // 12 bytes
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	ciphertext := gcm.Seal(nil, nonce, zipData, nil)

	var buf bytes.Buffer
	buf.WriteString(agoMagic)
	buf.WriteByte(agoKDFArgon2ID)
	buf.Write(salt)
	buf.Write(nonce)
	buf.Write(ciphertext)
	return buf.Bytes(), nil
}

func decryptAGO(data []byte, password string) ([]byte, error) {
	if len(data) < 4 || string(data[:4]) != agoMagic {
		return nil, fmt.Errorf("not an encrypted .ago file")
	}
	data = data[4:]
	if len(data) < agoSaltSize+12 { // salt + nonce min
		return nil, fmt.Errorf("invalid encrypted .ago file (too short)")
	}
	if data[0] == agoKDFArgon2ID {
		if len(data) < 1+agoSaltSize+12 {
			return nil, fmt.Errorf("invalid encrypted .ago file (too short)")
		}
		salt := data[1 : 1+agoSaltSize]
		payload := data[1+agoSaltSize:]
		plaintext, err := decryptBackupGCM(payload, deriveBackupKey(password, salt))
		if err == nil {
			return plaintext, nil
		}
		// Very old backups store random salt immediately after the magic. If the first
		// salt byte happens to match the current KDF marker, try the legacy path too.
		if legacy, legacyErr := decryptLegacyAGOBody(data, password); legacyErr == nil {
			return legacy, nil
		}
		return nil, fmt.Errorf("decryption failed (wrong password?): %w", err)
	}
	return decryptLegacyAGOBody(data, password)
}

func decryptLegacyAGOBody(data []byte, password string) ([]byte, error) {
	if len(data) < agoSaltSize+12 {
		return nil, fmt.Errorf("invalid encrypted .ago file (too short)")
	}
	salt := data[:agoSaltSize]
	payload := data[agoSaltSize:]
	plaintext, err := decryptBackupGCM(payload, deriveLegacyBackupKey(password, salt))
	if err != nil {
		return nil, fmt.Errorf("decryption failed (wrong password?): %w", err)
	}
	return plaintext, nil
}

func decryptBackupGCM(payload []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonceSize := gcm.NonceSize()
	if len(payload) < nonceSize {
		return nil, fmt.Errorf("nonce missing")
	}
	return gcm.Open(nil, payload[:nonceSize], payload[nonceSize:], nil)
}

func decryptVaultSecretsBlob(encData []byte, password string) ([]byte, error) {
	if bytes.HasPrefix(encData, []byte(vaultSecretsMagic)) {
		body := encData[len(vaultSecretsMagic):]
		if len(body) < 1+agoSaltSize+12 {
			return nil, fmt.Errorf("vault_secrets.enc too short")
		}
		if body[0] != agoKDFArgon2ID {
			return nil, fmt.Errorf("vault_secrets.enc has unsupported KDF version")
		}
		salt := body[1 : 1+agoSaltSize]
		plain, err := decryptBackupGCM(body[1+agoSaltSize:], deriveBackupKey(password, salt))
		if err != nil {
			return nil, fmt.Errorf("vault secrets decryption failed (wrong password?): %w", err)
		}
		return plain, nil
	}

	if len(encData) < agoSaltSize+12 {
		return nil, fmt.Errorf("vault_secrets.enc too short")
	}
	salt := encData[:agoSaltSize]
	plain, err := decryptBackupGCM(encData[agoSaltSize:], deriveLegacyBackupKey(password, salt))
	if err != nil {
		return nil, fmt.Errorf("vault secrets decryption failed (wrong password?): %w", err)
	}
	return plain, nil
}

func agoArchiveBaseDir(targetDir string) (string, error) {
	absBase, err := filepath.Abs(targetDir)
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(absBase); err == nil {
		absBase = resolved
	}
	return filepath.Clean(absBase), nil
}

func agoSafeDestination(baseDir, name string) (string, error) {
	dest := filepath.Clean(filepath.Join(baseDir, name))
	rel, err := filepath.Rel(baseDir, dest)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("path traversal")
	}
	return dest, nil
}

func agoEnsureSafePath(baseDir, dest string, dirTarget bool) error {
	rel, err := filepath.Rel(baseDir, dest)
	if err != nil {
		return err
	}
	if rel == "." && dirTarget {
		return nil
	}
	parts := strings.Split(filepath.Clean(rel), string(os.PathSeparator))
	current := baseDir
	limit := len(parts)
	if !dirTarget {
		limit--
	}
	for i := 0; i < limit; i++ {
		current = filepath.Join(current, parts[i])
		info, err := os.Lstat(current)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing to restore through symlink path: %s", current)
		}
		if !info.IsDir() {
			return fmt.Errorf("refusing to restore through non-directory path: %s", current)
		}
	}
	if !dirTarget {
		if info, err := os.Lstat(dest); err == nil && info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing to overwrite symlink target: %s", dest)
		}
	}
	return nil
}

func agoRelZipPath(baseDir, absPath string) (string, bool) {
	rel, err := filepath.Rel(baseDir, absPath)
	if err != nil {
		return "", false
	}
	clean := filepath.Clean(rel)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
		return "", false
	}
	return filepath.ToSlash(clean), true
}

func agoAtomicCopyFile(src, dest string, perm os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(dest), ".ago-restore-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if err := tmp.Chmod(perm); err != nil {
		tmp.Close()
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		tmp.Close()
		return err
	}
	if _, err := io.Copy(tmp, in); err != nil {
		in.Close()
		tmp.Close()
		return err
	}
	if err := in.Close(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, dest)
}

// ── Create backup ────────────────────────────────────────────────────────────

// handleBackupCreate builds a .ago archive from the running instance's files
// and streams it to the client as a download.
func handleBackupCreate(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			Password        string `json:"password"`
			IncludeVectorDB bool   `json:"include_vectordb"`
			IncludeWorkdir  bool   `json:"include_workdir"`
		}
		body, _ := io.ReadAll(io.LimitReader(r.Body, 1024))
		json.Unmarshal(body, &req)

		absConfig, _ := filepath.Abs(s.Cfg.ConfigPath)
		configRoot := filepath.Dir(absConfig)

		// Build ZIP in memory
		var zipBuf bytes.Buffer
		zw := zip.NewWriter(&zipBuf)
		contents := []string{}
		seenZipPaths := map[string]struct{}{}

		// addFile adds a single file to the ZIP at the given zip-internal path.
		addFile := func(zipPath, filePath string) {
			info, err := os.Lstat(filePath)
			if err != nil || info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
				return
			}
			zipPath = filepath.ToSlash(filepath.Clean(zipPath))
			if zipPath == "." {
				return
			}
			if _, exists := seenZipPaths[zipPath]; exists {
				return
			}
			// Skip excessively large files (>100 MB) to prevent OOM
			if info.Size() > 100<<20 {
				s.Logger.Warn("[Backup] Skipping oversized file", "path", filePath, "size", info.Size())
				return
			}
			f, err := os.Open(filePath)
			if err != nil {
				return
			}
			defer f.Close()
			fh, err := zip.FileInfoHeader(info)
			if err != nil {
				return
			}
			fh.Name = zipPath
			fh.Method = zip.Deflate
			fw, err := zw.CreateHeader(fh)
			if err != nil {
				return
			}
			if _, err := io.Copy(fw, f); err != nil {
				return
			}
			seenZipPaths[zipPath] = struct{}{}
		}

		// addDir recursively adds all files under dirPath into zipPrefix, skipping
		// any path components matching the excludes substrings.
		addDir := func(zipPrefix, dirPath string, excludes ...string) {
			filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
				if err != nil || info.IsDir() {
					return nil
				}
				fwdPath := filepath.ToSlash(path)
				for _, ex := range excludes {
					if strings.Contains(fwdPath, ex) {
						return nil
					}
				}
				rel, _ := filepath.Rel(dirPath, path)
				addFile(zipPrefix+filepath.ToSlash(rel), path)
				return nil
			})
		}

		addFile("config.yaml", absConfig)
		contents = append(contents, "config.yaml")

		// 2. prompts/ — only custom files (user-created or user-modified).
		// Files that exist in the embedded defaults and are byte-identical are
		// skipped; they come bundled with the binary so there is no need to back
		// them up.  Only new personalities or edited prompt files are included.
		absPrompts, _ := filepath.Abs(s.Cfg.Directories.PromptsDir)
		customPromptCount := 0
		filepath.Walk(absPrompts, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			rel, _ := filepath.Rel(absPrompts, path)
			embedPath := filepath.ToSlash(rel)

			// Read the on-disk file
			diskBytes, err := os.ReadFile(path)
			if err != nil {
				return nil
			}

			// Check if this path exists in the embedded FS
			embedBytes, embedErr := fs.ReadFile(promptsembed.FS, embedPath)
			if embedErr == nil && bytes.Equal(diskBytes, embedBytes) {
				// Identical to the embedded default — skip
				return nil
			}

			// Include: either not in embed (user-created) or content differs (user-modified)
			addFile("prompts/"+embedPath, path)
			customPromptCount++
			return nil
		})
		if customPromptCount > 0 {
			contents = append(contents, fmt.Sprintf("prompts/ (custom only, %d file(s))", customPromptCount))
		}

		// 3. Curated runtime files
		absData, _ := filepath.Abs(s.Cfg.Directories.DataDir)
		dataFiles := []string{
			"chat_history.json", "state.json", "graph.json",
			"crontab.json", "current_plan.md", "character_journal.md",
			"budget.json", "tokens.json", "webhooks.json", "webhook_log.json",
			"background_tasks.json", "missions_v2.json",
		}
		for _, fname := range dataFiles {
			addFile("data/"+fname, filepath.Join(absData, fname))
		}
		contents = append(contents, "data/ (runtime files)")

		// 4. SQLite databases + sidecars
		sqlitePaths := []string{
			s.Cfg.SQLite.ShortTermPath,
			s.Cfg.SQLite.LongTermPath,
			s.Cfg.SQLite.InventoryPath,
			s.Cfg.SQLite.InvasionPath,
			s.Cfg.SQLite.CheatsheetPath,
			s.Cfg.SQLite.ImageGalleryPath,
			s.Cfg.SQLite.RemoteControlPath,
			s.Cfg.SQLite.MediaRegistryPath,
			s.Cfg.SQLite.HomepageRegistryPath,
			s.Cfg.SQLite.ContactsPath,
			s.Cfg.SQLite.PlannerPath,
			s.Cfg.SQLite.SiteMonitorPath,
			s.Cfg.SQLite.SQLConnectionsPath,
			s.Cfg.SQLite.SkillsPath,
			s.Cfg.SQLite.KnowledgeGraphPath,
			s.Cfg.SQLite.OptimizationPath,
			s.Cfg.SQLite.PreparedMissionsPath,
			s.Cfg.SQLite.MissionHistoryPath,
			s.Cfg.SQLite.PushPath,
		}
		sqliteAdded := 0
		for _, dbPath := range sqlitePaths {
			if dbPath == "" {
				continue
			}
			absDB, err := filepath.Abs(dbPath)
			if err != nil {
				continue
			}
			zipPath, ok := agoRelZipPath(configRoot, absDB)
			if !ok {
				s.Logger.Warn("[Backup] Skipping SQLite path outside config root", "path", absDB)
				continue
			}
			before := len(seenZipPaths)
			addFile(zipPath, absDB)
			addFile(zipPath+"-wal", absDB+"-wal")
			addFile(zipPath+"-shm", absDB+"-shm")
			if len(seenZipPaths) > before {
				sqliteAdded++
			}
		}
		if sqliteAdded > 0 {
			contents = append(contents, fmt.Sprintf("sqlite/ (%d database roots incl. WAL/SHM)", sqliteAdded))
		}

		// 5. VectorDB (optional — can be large)
		if req.IncludeVectorDB {
			absVDB, _ := filepath.Abs(s.Cfg.Directories.VectorDBDir)
			addDir("data/vectordb/", absVDB)
			contents = append(contents, "data/vectordb/")
		}

		// 6. Skills (exclude OAuth credentials)
		absSkills, _ := filepath.Abs(s.Cfg.Directories.SkillsDir)
		addDir("agent_workspace/skills/", absSkills,
			"client_secret.json", "client_secrets.json", "token.json")
		contents = append(contents, "agent_workspace/skills/")

		// 7. Tools
		absTools, _ := filepath.Abs(s.Cfg.Directories.ToolsDir)
		addDir("agent_workspace/tools/", absTools)
		contents = append(contents, "agent_workspace/tools/")

		// 8. Workdir (optional — excludes images sub-dir to keep size manageable)
		if req.IncludeWorkdir {
			absWorkdir, _ := filepath.Abs(s.Cfg.Directories.WorkspaceDir)
			addDir("agent_workspace/workdir/", absWorkdir, "/images/", "/attachments/")
			contents = append(contents, "agent_workspace/workdir/")
		}

		// 9. Vault secrets (only when a password is set — never in plain-text backups)
		vaultIncluded := false
		if req.Password != "" && s.Vault != nil {
			if vaultBlob, err := exportVaultSecrets(s.Vault, req.Password); err != nil {
				s.Logger.Warn("[Backup] Could not export vault secrets", "error", err)
			} else if len(vaultBlob) > 0 {
				if vw, err := zw.Create("vault_secrets.enc"); err == nil {
					vw.Write(vaultBlob)
					vaultIncluded = true
					contents = append(contents, "vault_secrets.enc (secrets, encrypted)")
				}
			}
		}

		// 10. Write manifest
		hostname, _ := os.Hostname()
		manifest := agoManifest{
			Version:         "1",
			CreatedAt:       time.Now().UTC().Format(time.RFC3339),
			Hostname:        hostname,
			Contents:        contents,
			VaultIncluded:   vaultIncluded,
			DBSchemaVersion: currentDBSchemaVersion,
		}
		if mw, err := zw.Create("manifest.json"); err == nil {
			json.NewEncoder(mw).Encode(manifest)
		}

		zw.Close()

		zipData := zipBuf.Bytes()
		filename := fmt.Sprintf("aurago_backup_%s.ago", time.Now().Format("20060102_150405"))

		var outData []byte
		if req.Password != "" {
			var err error
			outData, err = encryptAGO(zipData, req.Password)
			if err != nil {
				s.Logger.Error("[Backup] Encryption failed", "error", err)
				jsonError(w, "Encryption failed", http.StatusInternalServerError)
				return
			}
		} else {
			outData = zipData
		}

		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(outData)))
		w.Write(outData)

		s.Logger.Info("[Backup] Backup created",
			"filename", filename,
			"size_bytes", len(outData),
			"encrypted", req.Password != "", "vault_included", vaultIncluded, "vectordb", req.IncludeVectorDB,
			"workdir", req.IncludeWorkdir)
	}
}

// ── Import backup ────────────────────────────────────────────────────────────

// handleBackupImport restores files from a .ago archive into the running instance.
func handleBackupImport(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Multipart form — allow up to 512 MB for vectordb-inclusive backups
		r.Body = http.MaxBytesReader(w, r.Body, 512<<20)
		if err := r.ParseMultipartForm(512 << 20); err != nil {
			s.Logger.Warn("[Backup] Invalid import form", "error", err)
			jsonError(w, "Invalid backup upload", http.StatusBadRequest)
			return
		}

		file, header, err := r.FormFile("file")
		if err != nil {
			jsonError(w, "No file uploaded", http.StatusBadRequest)
			return
		}
		defer file.Close()

		if !strings.HasSuffix(strings.ToLower(header.Filename), ".ago") {
			jsonError(w, "File must have .ago extension", http.StatusBadRequest)
			return
		}

		password := r.FormValue("password")

		rawData, err := io.ReadAll(file)
		if err != nil {
			s.Logger.Warn("[Backup] Failed to read import upload", "filename", header.Filename, "error", err)
			jsonError(w, "Failed to read uploaded backup", http.StatusBadRequest)
			return
		}

		// Detect and handle encryption
		var zipData []byte
		if len(rawData) >= 4 && string(rawData[:4]) == agoMagic {
			if password == "" {
				// Special case: tell UI that a password is required
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]string{
					"error":   "password_required",
					"message": "Diese Backup-Datei ist verschlüsselt. Bitte Passwort eingeben.",
				})
				return
			}
			zipData, err = decryptAGO(rawData, password)
			if err != nil {
				s.Logger.Warn("[Backup] Backup decryption failed", "filename", header.Filename, "error", err)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]string{
					"error":   "decryption_failed",
					"message": "Invalid backup password or corrupted encrypted backup file.",
				})
				return
			}
		} else {
			zipData = rawData
		}

		// Open as ZIP
		zr, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
		if err != nil {
			s.Logger.Warn("[Backup] Invalid backup archive", "filename", header.Filename, "error", err)
			jsonError(w, "Invalid backup archive", http.StatusBadRequest)
			return
		}

		// Restore relative to the configured instance root, not the process CWD.
		configRoot, err := agoArchiveBaseDir(filepath.Dir(s.Cfg.ConfigPath))
		if err != nil {
			jsonError(w, "Cannot determine restore root", http.StatusInternalServerError)
			return
		}

		if len(zr.File) > agoMaxArchiveEntries {
			jsonError(w, "Backup archive contains too many entries", http.StatusBadRequest)
			return
		}

		restored, skipped := 0, 0
		var vaultEncData []byte // vault_secrets.enc bytes, if present
		var backupManifest *agoManifest
		var importEntries []agoImportEntry
		var totalBytes int64

		for _, f := range zr.File {
			clean := filepath.Clean(filepath.FromSlash(f.Name))
			if clean == "." {
				continue
			}
			if strings.HasPrefix(clean, "..") || filepath.IsAbs(clean) {
				s.Logger.Warn("[Backup] Unsafe path in archive", "path", f.Name)
				jsonError(w, "Backup archive contains unsafe paths", http.StatusBadRequest)
				return
			}
			if f.Mode()&os.ModeSymlink != 0 {
				s.Logger.Warn("[Backup] Symlink entry rejected", "path", f.Name)
				jsonError(w, "Backup archive contains unsupported symlink entries", http.StatusBadRequest)
				return
			}
			if f.CompressedSize64 > 0 && f.UncompressedSize64 > 0 {
				ratio := f.UncompressedSize64 / f.CompressedSize64
				if ratio > agoMaxCompressionRatio {
					s.Logger.Warn("[Backup] Suspicious compression ratio", "path", f.Name, "ratio", ratio)
					jsonError(w, "Backup archive looks corrupted or malicious", http.StatusBadRequest)
					return
				}
			}
			if f.UncompressedSize64 > uint64(agoMaxExtractSize) {
				jsonError(w, "Backup archive exceeds maximum size", http.StatusBadRequest)
				return
			}
			totalBytes += int64(f.UncompressedSize64)
			if totalBytes > agoMaxExtractSize {
				jsonError(w, "Backup archive exceeds maximum size", http.StatusBadRequest)
				return
			}

			// vault_secrets.enc is never written to disk — handled separately below.
			if clean == "vault_secrets.enc" {
				rc, err := f.Open()
				if err != nil {
					jsonError(w, "Failed to read vault secrets from backup", http.StatusBadRequest)
					return
				}
				vaultEncData, err = io.ReadAll(io.LimitReader(rc, 8<<20))
				rc.Close()
				if err != nil {
					jsonError(w, "Failed to read vault secrets from backup", http.StatusBadRequest)
					return
				}
				continue
			}

			// manifest.json — read for schema version check, skip file-system restore.
			if clean == "manifest.json" {
				rc, err := f.Open()
				if err != nil {
					jsonError(w, "Failed to read backup manifest", http.StatusBadRequest)
					return
				}
				var m agoManifest
				if jsonErr := json.NewDecoder(io.LimitReader(rc, 1<<20)).Decode(&m); jsonErr == nil {
					backupManifest = &m
				}
				rc.Close()
				continue
			}

			destPath, err := agoSafeDestination(configRoot, clean)
			if err != nil {
				s.Logger.Warn("[Backup] Illegal path in archive", "path", f.Name, "error", err)
				jsonError(w, "Backup archive contains unsafe paths", http.StatusBadRequest)
				return
			}
			if err := agoEnsureSafePath(configRoot, destPath, f.FileInfo().IsDir()); err != nil {
				s.Logger.Warn("[Backup] Unsafe restore destination", "path", f.Name, "error", err)
				jsonError(w, "Restore target path is unsafe", http.StatusBadRequest)
				return
			}
			importEntries = append(importEntries, agoImportEntry{
				file:    f,
				clean:   clean,
				dest:    destPath,
				isDir:   f.FileInfo().IsDir(),
				mode:    f.Mode().Perm(),
				zipPath: f.Name,
			})
		}

		stagingRoot, err := os.MkdirTemp(configRoot, ".ago-import-*")
		if err != nil {
			jsonError(w, "Failed to prepare restore staging area", http.StatusInternalServerError)
			return
		}
		defer os.RemoveAll(stagingRoot)

		for i := range importEntries {
			entry := &importEntries[i]
			entry.stage = filepath.Join(stagingRoot, entry.clean)
			if entry.isDir {
				if err := os.MkdirAll(entry.stage, 0o755); err != nil {
					s.Logger.Warn("[Backup] Failed to stage directory", "path", entry.zipPath, "error", err)
					jsonError(w, "Failed to stage restore files", http.StatusInternalServerError)
					return
				}
				continue
			}
			if err := os.MkdirAll(filepath.Dir(entry.stage), 0o755); err != nil {
				s.Logger.Warn("[Backup] Failed to stage file parent", "path", entry.zipPath, "error", err)
				jsonError(w, "Failed to stage restore files", http.StatusInternalServerError)
				return
			}
			rc, err := entry.file.Open()
			if err != nil {
				s.Logger.Warn("[Backup] Failed to open archive entry", "path", entry.zipPath, "error", err)
				jsonError(w, "Failed to read backup archive", http.StatusBadRequest)
				return
			}
			data, readErr := io.ReadAll(io.LimitReader(rc, int64(entry.file.UncompressedSize64)+1))
			rc.Close()
			if readErr != nil {
				s.Logger.Warn("[Backup] Failed to read archive entry", "path", entry.zipPath, "error", readErr)
				jsonError(w, "Failed to read backup archive", http.StatusBadRequest)
				return
			}
			if uint64(len(data)) > entry.file.UncompressedSize64 {
				s.Logger.Warn("[Backup] Entry exceeded declared size", "path", entry.zipPath)
				jsonError(w, "Backup archive is inconsistent", http.StatusBadRequest)
				return
			}
			perm := entry.mode
			if perm == 0 {
				perm = 0o644
			}
			if err := config.WriteFileAtomic(entry.stage, data, perm); err != nil {
				s.Logger.Warn("[Backup] Failed to stage file", "path", entry.zipPath, "error", err)
				jsonError(w, "Failed to stage restore files", http.StatusInternalServerError)
				return
			}
		}

		for _, entry := range importEntries {
			if entry.isDir {
				if err := os.MkdirAll(entry.dest, 0o755); err != nil {
					s.Logger.Warn("[Backup] Failed to restore directory", "path", entry.zipPath, "error", err)
					jsonError(w, "Restore failed while writing files", http.StatusInternalServerError)
					return
				}
				continue
			}
			perm := entry.mode
			if perm == 0 {
				perm = 0o644
			}
			if err := agoAtomicCopyFile(entry.stage, entry.dest, perm); err != nil {
				s.Logger.Warn("[Backup] Failed to restore file", "path", entry.zipPath, "error", err)
				jsonError(w, "Restore failed while writing files", http.StatusInternalServerError)
				return
			}
			restored++
		}

		// Vault secrets import — re-encrypt with the local AURAGO_MASTER_KEY so
		// the secrets are immediately available without restarting.
		vaultRestored := 0
		vaultErr := ""
		if len(vaultEncData) > 0 && password != "" && s.Vault != nil {
			n, err := importVaultSecrets(s.Vault, vaultEncData, password)
			if err != nil {
				s.Logger.Warn("[Backup] Vault secrets import failed", "error", err)
				vaultErr = err.Error()
			} else {
				vaultRestored = n
				s.Logger.Info("[Backup] Vault secrets imported", "count", n)
				// Hot-reload config so that vault-backed fields like Auth.PasswordHash
				// are reflected in the in-memory config immediately — without this the
				// login handler would still see the old (empty) hash and reject logins.
				s.CfgMu.Lock()
				if newCfg, loadErr := config.Load(s.Cfg.ConfigPath); loadErr == nil {
					newCfg.ApplyVaultSecrets(s.Vault)
					savedPath := s.Cfg.ConfigPath
					*s.Cfg = *newCfg
					s.Cfg.ConfigPath = savedPath
				}
				s.CfgMu.Unlock()
			}
		}

		s.Logger.Info("[Backup] Import completed", "restored", restored, "skipped", skipped, "vault_secrets", vaultRestored)

		schemaWarning := ""
		if backupManifest != nil && backupManifest.DBSchemaVersion != 0 && backupManifest.DBSchemaVersion != currentDBSchemaVersion {
			schemaWarning = fmt.Sprintf(
				"DB-Schema-Version im Backup (%d) unterscheidet sich von aktueller Version (%d). "+
					"AuraGo führt beim Neustart automatische Migrationen aus.",
				backupManifest.DBSchemaVersion, currentDBSchemaVersion)
			s.Logger.Warn("[Backup] DB schema version mismatch",
				"backup_version", backupManifest.DBSchemaVersion,
				"current_version", currentDBSchemaVersion)
		}

		msg := fmt.Sprintf("%d Dateien wiederhergestellt, %d übersprungen.", restored, skipped)
		if vaultRestored > 0 {
			msg += fmt.Sprintf(" %d Vault-Secrets wiederhergestellt.", vaultRestored)
		}
		if vaultErr != "" {
			msg += " Vault-Import fehlgeschlagen."
		}
		if schemaWarning != "" {
			msg += " " + schemaWarning
		}
		msg += " Neustart empfohlen um alle Änderungen zu übernehmen."

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":         "ok",
			"restored":       restored,
			"skipped":        skipped,
			"vault_restored": vaultRestored,
			"schema_warning": schemaWarning,
			"message":        msg,
		})
	}
}
