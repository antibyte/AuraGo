package server

// Backup & Restore — creates/imports .ago archives (ZIP-based, optionally AES-256-GCM encrypted).
//
// .ago file format:
//   Plain (no password):    standard ZIP file, rename to .zip to inspect.
//   Encrypted (password):   4-byte magic "AGOE" + 16-byte salt + 12-byte nonce + AES-256-GCM(ZIP).
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
)

const agoMagic = "AGOE"

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
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}
	key := deriveKey(password, salt)
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
	if len(encData) < 28 { // 16 salt + 12 nonce min
		return 0, fmt.Errorf("vault_secrets.enc too short")
	}
	salt := encData[:16]
	rest := encData[16:]
	key := deriveKey(password, salt)
	block, err := aes.NewCipher(key)
	if err != nil {
		return 0, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return 0, err
	}
	nonceSize := gcm.NonceSize()
	if len(rest) < nonceSize {
		return 0, fmt.Errorf("vault_secrets.enc nonce missing")
	}
	plain, err := gcm.Open(nil, rest[:nonceSize], rest[nonceSize:], nil)
	if err != nil {
		return 0, fmt.Errorf("vault secrets decryption failed (wrong password?): %w", err)
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

// deriveKey derives a 32-byte AES-256 key from a password and random salt
// using 65536 SHA-256 iterations (fast but meaningful KDF for a backup tool).
func deriveKey(password string, salt []byte) []byte {
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
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}
	key := deriveKey(password, salt)
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
	if len(data) < 28 { // 16 salt + 12 nonce min
		return nil, fmt.Errorf("invalid encrypted .ago file (too short)")
	}
	salt := data[:16]
	data = data[16:]
	nonce := data[:12]
	ciphertext := data[12:]

	key := deriveKey(password, salt)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decryption failed (wrong password?): %w", err)
	}
	return plaintext, nil
}

// ── Create backup ────────────────────────────────────────────────────────────

// handleBackupCreate builds a .ago archive from the running instance's files
// and streams it to the client as a download.
func handleBackupCreate(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			Password        string `json:"password"`
			IncludeVectorDB bool   `json:"include_vectordb"`
			IncludeWorkdir  bool   `json:"include_workdir"`
		}
		body, _ := io.ReadAll(io.LimitReader(r.Body, 1024))
		json.Unmarshal(body, &req)

		// Build ZIP in memory
		var zipBuf bytes.Buffer
		zw := zip.NewWriter(&zipBuf)
		contents := []string{}

		// addFile adds a single file to the ZIP at the given zip-internal path.
		addFile := func(zipPath, filePath string) {
			info, err := os.Stat(filePath)
			if err != nil || info.IsDir() {
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
			fh := &zip.FileHeader{
				Name:   zipPath,
				Method: zip.Deflate,
			}
			fh.SetModTime(info.ModTime())
			fw, err := zw.CreateHeader(fh)
			if err != nil {
				return
			}
			io.Copy(fw, f)
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

		// 1. config.yaml
		absConfig, _ := filepath.Abs(s.Cfg.ConfigPath)
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

		// 3. Curated data files (no sqlite WAL files, just final DBs + JSON/MD)
		absData, _ := filepath.Abs(s.Cfg.Directories.DataDir)
		dataFiles := []string{
			"chat_history.json", "state.json", "graph.json",
			"crontab.json", "current_plan.md", "character_journal.md",
			"budget.json", "long_term.db", "short_term.db", "inventory.db",
		}
		for _, fname := range dataFiles {
			addFile("data/"+fname, filepath.Join(absData, fname))
		}
		contents = append(contents, "data/ (curated)")

		// 4. VectorDB (optional — can be large)
		if req.IncludeVectorDB {
			absVDB, _ := filepath.Abs(s.Cfg.Directories.VectorDBDir)
			addDir("data/vectordb/", absVDB)
			contents = append(contents, "data/vectordb/")
		}

		// 5. Skills (exclude OAuth credentials)
		absSkills, _ := filepath.Abs(s.Cfg.Directories.SkillsDir)
		addDir("agent_workspace/skills/", absSkills,
			"client_secret.json", "client_secrets.json", "token.json")
		contents = append(contents, "agent_workspace/skills/")

		// 6. Tools
		absTools, _ := filepath.Abs(s.Cfg.Directories.ToolsDir)
		addDir("agent_workspace/tools/", absTools)
		contents = append(contents, "agent_workspace/tools/")

		// 7. Workdir (optional — excludes images sub-dir to keep size manageable)
		if req.IncludeWorkdir {
			absWorkdir, _ := filepath.Abs(s.Cfg.Directories.WorkspaceDir)
			addDir("agent_workspace/workdir/", absWorkdir, "/images/", "/attachments/")
			contents = append(contents, "agent_workspace/workdir/")
		}

		// 8. Vault secrets (only when a password is set — never in plain-text backups)
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

		// 9. Write manifest
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
				http.Error(w, "Encryption failed", http.StatusInternalServerError)
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
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
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

		// Restore relative to CWD (same as where the binary runs)
		cwd, err := os.Getwd()
		if err != nil {
			http.Error(w, "Cannot determine working directory", http.StatusInternalServerError)
			return
		}

		restored, skipped := 0, 0
		var vaultEncData []byte // vault_secrets.enc bytes, if present
		var backupManifest *agoManifest

		for _, f := range zr.File {
			// Security: reject path-traversal attempts
			clean := filepath.Clean(filepath.FromSlash(f.Name))
			if strings.HasPrefix(clean, "..") || filepath.IsAbs(clean) {
				s.Logger.Warn("[Backup] Skipping unsafe path in archive", "path", f.Name)
				skipped++
				continue
			}

			// vault_secrets.enc is never written to disk — handled separately below.
			if clean == "vault_secrets.enc" {
				if rc, err := f.Open(); err == nil {
					vaultEncData, _ = io.ReadAll(rc)
					rc.Close()
				}
				continue
			}

			// manifest.json — read for schema version check, skip file-system restore.
			if clean == "manifest.json" {
				if rc, err := f.Open(); err == nil {
					var m agoManifest
					if jsonErr := json.NewDecoder(rc).Decode(&m); jsonErr == nil {
						backupManifest = &m
					}
					rc.Close()
				}
				continue
			}

			destPath := filepath.Join(cwd, clean)

			if f.FileInfo().IsDir() {
				os.MkdirAll(destPath, 0755)
				continue
			}

			if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
				skipped++
				continue
			}

			rc, err := f.Open()
			if err != nil {
				skipped++
				continue
			}

			out, err := os.Create(destPath)
			if err != nil {
				rc.Close()
				skipped++
				continue
			}

			_, copyErr := io.Copy(out, rc)
			out.Close()
			rc.Close()

			if copyErr != nil {
				skipped++
			} else {
				restored++
			}
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
