package security

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gofrs/flock"
)

const vaultAgentReadableMetadataKey = "__aurago_agent_readable_secrets_v1"

var (
	ErrSecretNotFound          = errors.New("secret not found")
	ErrSecretExists            = errors.New("secret already exists")
	ErrSecretAgentAccessDenied = errors.New("secret is not agent-readable")
	ErrReservedVaultKey        = errors.New("reserved vault key")
)

type vaultAgentReadableMetadata struct {
	Version int      `json:"version"`
	Keys    []string `json:"keys"`
	MAC     string   `json:"mac"`
}

type vaultBackupProvenance struct {
	Version int      `json:"version"`
	Keys    []string `json:"keys"`
}

type Vault struct {
	mu       sync.Mutex
	key      []byte
	filePath string
	fileLock *flock.Flock
}

func NewVault(masterKeyHex string, filePath string) (*Vault, error) {
	key, err := hex.DecodeString(masterKeyHex)
	if err != nil {
		return nil, fmt.Errorf("invalid master key format, expected hex: %w", err)
	}

	if len(key) != 32 {
		return nil, fmt.Errorf("invalid master key length, expected 32 bytes (64 hex characters)")
	}

	return &Vault{
		key:      key,
		filePath: filePath,
		fileLock: flock.New(filePath + ".lock"),
	}, nil
}

func (v *Vault) loadAndDecrypt() (map[string]string, error) {
	secrets := make(map[string]string)

	data, err := os.ReadFile(v.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return secrets, nil // Return empty map if file doesn't exist
		}
		return nil, fmt.Errorf("failed to read vault file: %w", err)
	}

	if len(data) == 0 {
		return secrets, nil
	}

	block, err := aes.NewCipher(v.key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt vault: %w", err)
	}
	defer clear(plaintext)

	if err := json.Unmarshal(plaintext, &secrets); err != nil {
		return nil, fmt.Errorf("failed to unmarshal secrets: %w", err)
	}

	return secrets, nil
}

func (v *Vault) encryptAndSave(secrets map[string]string) error {
	plaintext, err := json.Marshal(secrets)
	if err != nil {
		return fmt.Errorf("failed to marshal secrets: %w", err)
	}
	defer clear(plaintext)

	block, err := aes.NewCipher(v.key)
	if err != nil {
		return fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return fmt.Errorf("failed to generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)

	if err := writeVaultFileAtomic(v.filePath, ciphertext, 0o600); err != nil {
		return fmt.Errorf("failed to write vault file: %w", err)
	}

	return nil
}

func writeVaultFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	success := false
	defer func() {
		_ = tmp.Close()
		if !success {
			_ = os.Remove(tmpPath)
		}
	}()

	if err := tmp.Chmod(perm); err != nil {
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	success = true
	return nil
}

func writeVaultFileAtomicContext(ctx context.Context, path string, data []byte, perm os.FileMode) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	success := false
	defer func() {
		_ = tmp.Close()
		if !success {
			_ = os.Remove(tmpPath)
		}
	}()

	if err := tmp.Chmod(perm); err != nil {
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	success = true
	return nil
}

func (v *Vault) ReadSecret(key string) (string, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	if isReservedVaultMetadataKey(key) {
		return "", ErrSecretNotFound
	}

	// Double-check: mutex guards this goroutine; flock guards other processes.
	// On Windows, flock uses the native locking API (LockFileEx).
	// If flock fails (e.g., filesystem doesn't support locking), we fail safe.
	if err := v.fileLock.Lock(); err != nil {
		return "", fmt.Errorf("failed to acquire vault file lock: %w", err)
	}
	defer v.fileLock.Unlock()

	secrets, err := v.loadAndDecrypt()
	if err != nil {
		return "", err
	}

	val, ok := secrets[key]
	if !ok {
		return "", ErrSecretNotFound
	}

	return val, nil
}

func (v *Vault) WriteSecret(key, value string) error {
	return v.writeSecret(key, value, true, false, false)
}

// WriteUserSecret stores a user-supplied secret. User-supplied secrets are
// deliberately not readable by the agent, even when the agent chose the key.
func (v *Vault) WriteUserSecret(key, value string, replace bool) error {
	return v.writeSecret(key, value, replace, false, false)
}

// WriteUserSecretContext stores a user-supplied secret while respecting
// cancellation while waiting for either the in-process or cross-process Vault
// lock. Cancellation is checked again before the atomic publish.
func (v *Vault) WriteUserSecretContext(ctx context.Context, key, value string, replace bool) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := lockMutexContext(ctx, &v.mu); err != nil {
		return err
	}
	defer v.mu.Unlock()

	locked, err := v.fileLock.TryLockContext(ctx, 25*time.Millisecond)
	if err != nil {
		return fmt.Errorf("failed to acquire vault file lock: %w", err)
	}
	if !locked {
		if err := ctx.Err(); err != nil {
			return err
		}
		return fmt.Errorf("failed to acquire vault file lock")
	}
	defer v.fileLock.Unlock()

	secrets, err := v.loadAndDecrypt()
	if err != nil {
		return err
	}
	if isReservedVaultMetadataKey(key) {
		return ErrReservedVaultKey
	}
	readable, err := v.loadAgentReadableKeys(secrets)
	if err != nil {
		return err
	}
	if _, exists := secrets[key]; exists && !replace {
		return ErrSecretExists
	}
	secrets[key] = value
	delete(readable, key)
	if err := v.storeAgentReadableKeys(secrets, readable); err != nil {
		return err
	}
	return v.encryptAndSaveContext(ctx, secrets)
}

func lockMutexContext(ctx context.Context, mu *sync.Mutex) error {
	for {
		if mu.TryLock() {
			return nil
		}
		timer := time.NewTimer(10 * time.Millisecond)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return ctx.Err()
		case <-timer.C:
		}
	}
}

func (v *Vault) encryptAndSaveContext(ctx context.Context, secrets map[string]string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	plaintext, err := json.Marshal(secrets)
	if err != nil {
		return fmt.Errorf("failed to marshal secrets: %w", err)
	}
	defer clear(plaintext)
	if err := ctx.Err(); err != nil {
		return err
	}

	block, err := aes.NewCipher(v.key)
	if err != nil {
		return fmt.Errorf("failed to create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("failed to create GCM: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return fmt.Errorf("failed to generate nonce: %w", err)
	}
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	if err := writeVaultFileAtomicContext(ctx, v.filePath, ciphertext, 0o600); err != nil {
		return fmt.Errorf("failed to write vault file: %w", err)
	}
	return nil
}

// WriteAgentSecret stores a value that the model itself supplied. It may only
// create a new key or replace another agent-created value; a user/system value
// can never be reclassified or overwritten through the agent path.
func (v *Vault) WriteAgentSecret(key, value string) error {
	return v.writeSecret(key, value, true, true, true)
}

func (v *Vault) writeSecret(key, value string, replace, agentReadable, protectHidden bool) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	// Double-check: mutex guards this goroutine; flock guards other processes.
	// On Windows, flock uses the native locking API (LockFileEx).
	// If flock fails (e.g., filesystem doesn't support locking), we fail safe.
	if err := v.fileLock.Lock(); err != nil {
		return fmt.Errorf("failed to acquire vault file lock: %w", err)
	}
	defer v.fileLock.Unlock()

	secrets, err := v.loadAndDecrypt()
	if err != nil {
		return err
	}

	if isReservedVaultMetadataKey(key) {
		return ErrReservedVaultKey
	}
	readable, err := v.loadAgentReadableKeys(secrets)
	if err != nil {
		return err
	}
	_, exists := secrets[key]
	if exists && !replace {
		return ErrSecretExists
	}
	if exists && protectHidden {
		if _, ok := readable[key]; !ok {
			return ErrSecretAgentAccessDenied
		}
	}
	secrets[key] = value
	if agentReadable {
		readable[key] = struct{}{}
	} else {
		delete(readable, key)
	}
	if err := v.storeAgentReadableKeys(secrets, readable); err != nil {
		return err
	}
	return v.encryptAndSave(secrets)
}

// DeleteSecret removes a secret by key. Returns nil if the key didn't exist.
func (v *Vault) DeleteSecret(key string) error {
	return v.deleteSecret(key, false)
}

// DeleteAgentSecret removes only values previously created by the agent.
func (v *Vault) DeleteAgentSecret(key string) error {
	return v.deleteSecret(key, true)
}

func (v *Vault) deleteSecret(key string, agentOnly bool) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	// Double-check: mutex guards this goroutine; flock guards other processes.
	// On Windows, flock uses the native locking API (LockFileEx).
	// If flock fails (e.g., filesystem doesn't support locking), we fail safe.
	if err := v.fileLock.Lock(); err != nil {
		return fmt.Errorf("failed to acquire vault file lock: %w", err)
	}
	defer v.fileLock.Unlock()

	secrets, err := v.loadAndDecrypt()
	if err != nil {
		return err
	}

	if isReservedVaultMetadataKey(key) {
		return ErrReservedVaultKey
	}
	readable, err := v.loadAgentReadableKeys(secrets)
	if err != nil {
		return err
	}
	if _, exists := secrets[key]; !exists {
		return nil
	}
	if agentOnly {
		if _, ok := readable[key]; !ok {
			return ErrSecretAgentAccessDenied
		}
	}
	delete(secrets, key)
	delete(readable, key)
	if err := v.storeAgentReadableKeys(secrets, readable); err != nil {
		return err
	}
	return v.encryptAndSave(secrets)
}

// AgentSecretInfo reports whether a key exists and whether its value may be
// returned to the model. Unclassified legacy values fail closed as unreadable.
func (v *Vault) AgentSecretInfo(key string) (present, readable bool, err error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	if err := v.fileLock.Lock(); err != nil {
		return false, false, fmt.Errorf("failed to acquire vault file lock: %w", err)
	}
	defer v.fileLock.Unlock()

	secrets, err := v.loadAndDecrypt()
	if err != nil {
		return false, false, err
	}
	if isReservedVaultMetadataKey(key) {
		return false, false, nil
	}
	if _, present = secrets[key]; !present {
		return false, false, nil
	}
	readableKeys, err := v.loadAgentReadableKeys(secrets)
	if err != nil {
		return true, false, err
	}
	_, readable = readableKeys[key]
	return present, readable, nil
}

// AgentCanReadSecret is used by Python, sandbox and skill secret injection.
func (v *Vault) AgentCanReadSecret(key string) (bool, error) {
	present, readable, err := v.AgentSecretInfo(key)
	return present && readable, err
}

// ReadSecretForAgent returns a value only when it was explicitly created by
// the model. Server-side integrations should continue to use ReadSecret.
func (v *Vault) ReadSecretForAgent(key string) (string, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	if err := v.fileLock.Lock(); err != nil {
		return "", fmt.Errorf("failed to acquire vault file lock: %w", err)
	}
	defer v.fileLock.Unlock()

	secrets, err := v.loadAndDecrypt()
	if err != nil {
		return "", err
	}
	value, ok := secrets[key]
	if !ok || isReservedVaultMetadataKey(key) {
		return "", ErrSecretNotFound
	}
	readable, err := v.loadAgentReadableKeys(secrets)
	if err != nil {
		return "", err
	}
	if _, ok := readable[key]; !ok {
		return "", ErrSecretAgentAccessDenied
	}
	return value, nil
}

func isReservedVaultMetadataKey(key string) bool {
	return strings.EqualFold(strings.TrimSpace(key), vaultAgentReadableMetadataKey)
}

func (v *Vault) loadAgentReadableKeys(secrets map[string]string) (map[string]struct{}, error) {
	readable := make(map[string]struct{})
	raw := strings.TrimSpace(secrets[vaultAgentReadableMetadataKey])
	if raw == "" {
		return readable, nil
	}
	var metadata vaultAgentReadableMetadata
	if err := json.Unmarshal([]byte(raw), &metadata); err != nil {
		return readable, nil
	}
	keys := canonicalAgentReadableKeys(secrets, metadata.Keys)
	if metadata.Version != 1 || metadata.MAC == "" ||
		!hmac.Equal([]byte(strings.ToLower(metadata.MAC)), []byte(v.agentReadableMetadataMAC(keys))) {
		return readable, nil
	}
	for _, key := range keys {
		readable[key] = struct{}{}
	}
	return readable, nil
}

func (v *Vault) storeAgentReadableKeys(secrets map[string]string, readable map[string]struct{}) error {
	if len(readable) == 0 {
		delete(secrets, vaultAgentReadableMetadataKey)
		return nil
	}
	keys := make([]string, 0, len(readable))
	for key := range readable {
		if _, exists := secrets[key]; exists && !isReservedVaultMetadataKey(key) {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	if len(keys) == 0 {
		delete(secrets, vaultAgentReadableMetadataKey)
		return nil
	}
	encoded, err := json.Marshal(vaultAgentReadableMetadata{
		Version: 1,
		Keys:    keys,
		MAC:     v.agentReadableMetadataMAC(keys),
	})
	if err != nil {
		return fmt.Errorf("encode vault access metadata: %w", err)
	}
	secrets[vaultAgentReadableMetadataKey] = string(encoded)
	return nil
}

func canonicalAgentReadableKeys(secrets map[string]string, candidates []string) []string {
	seen := make(map[string]struct{}, len(candidates))
	keys := make([]string, 0, len(candidates))
	for _, key := range candidates {
		key = strings.TrimSpace(key)
		if key == "" || isReservedVaultMetadataKey(key) {
			continue
		}
		if _, exists := secrets[key]; !exists {
			continue
		}
		if _, duplicate := seen[key]; duplicate {
			continue
		}
		seen[key] = struct{}{}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func (v *Vault) agentReadableMetadataMAC(keys []string) string {
	mac := hmac.New(sha256.New, v.key)
	_, _ = mac.Write([]byte(vaultAgentReadableMetadataKey))
	_, _ = mac.Write([]byte{0})
	for _, key := range keys {
		_, _ = mac.Write([]byte(key))
		_, _ = mac.Write([]byte{0})
	}
	return hex.EncodeToString(mac.Sum(nil))
}

// EncryptBytes encrypts arbitrary data using the Vault's AES-256-GCM key.
func (v *Vault) EncryptBytes(plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(v.key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

// DecryptBytes decrypts data that was encrypted with EncryptBytes.
func (v *Vault) DecryptBytes(data []byte) ([]byte, error) {
	block, err := aes.NewCipher(v.key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}

// ListKeys returns all stored secret keys (without values).
func (v *Vault) ListKeys() ([]string, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	if err := v.fileLock.Lock(); err != nil {
		return nil, fmt.Errorf("failed to acquire vault file lock: %w", err)
	}
	defer v.fileLock.Unlock()

	secrets, err := v.loadAndDecrypt()
	if err != nil {
		return nil, err
	}

	keys := make([]string, 0, len(secrets))
	for k := range secrets {
		if isReservedVaultMetadataKey(k) {
			continue
		}
		keys = append(keys, k)
	}
	return keys, nil
}

// BackupSnapshot returns a consistent copy of all Vault values plus a portable
// provenance marker. The marker contains key names only; it is protected by the
// outer encrypted backup and never exposes secret values. The live HMAC is not
// portable because it is bound to the current master key.
func (v *Vault) BackupSnapshot() (map[string]string, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	if err := v.fileLock.Lock(); err != nil {
		return nil, fmt.Errorf("failed to acquire vault file lock: %w", err)
	}
	defer v.fileLock.Unlock()

	secrets, err := v.loadAndDecrypt()
	if err != nil {
		return nil, err
	}
	readable, err := v.loadAgentReadableKeys(secrets)
	if err != nil {
		return nil, err
	}
	snapshot := make(map[string]string, len(secrets))
	for key, value := range secrets {
		if !isReservedVaultMetadataKey(key) {
			snapshot[key] = value
		}
	}
	keys := make([]string, 0, len(readable))
	for key := range readable {
		if _, exists := snapshot[key]; exists {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	if len(keys) > 0 {
		encoded, err := json.Marshal(vaultBackupProvenance{Version: 1, Keys: keys})
		if err != nil {
			return nil, fmt.Errorf("encode vault backup provenance: %w", err)
		}
		snapshot[vaultAgentReadableMetadataKey] = string(encoded)
	}
	return snapshot, nil
}

// RestoreBackupSnapshot atomically merges backup values into the Vault and
// restores agent-readable provenance under the current master key. Backups
// without valid provenance fail closed: every imported value is classified as
// user-provided and remains hidden from the agent.
func (v *Vault) RestoreBackupSnapshot(snapshot map[string]string) (int, error) {
	incoming := make(map[string]string, len(snapshot))
	for key, value := range snapshot {
		incoming[key] = value
	}

	var portable vaultBackupProvenance
	provenanceValid := false
	if raw, ok := incoming[vaultAgentReadableMetadataKey]; ok {
		delete(incoming, vaultAgentReadableMetadataKey)
		if err := json.Unmarshal([]byte(raw), &portable); err == nil && portable.Version == 1 {
			provenanceValid = true
		}
	}
	for key := range incoming {
		if isReservedVaultMetadataKey(key) {
			delete(incoming, key)
		}
	}

	v.mu.Lock()
	defer v.mu.Unlock()
	if err := v.fileLock.Lock(); err != nil {
		return 0, fmt.Errorf("failed to acquire vault file lock: %w", err)
	}
	defer v.fileLock.Unlock()

	secrets, err := v.loadAndDecrypt()
	if err != nil {
		return 0, err
	}
	readable, err := v.loadAgentReadableKeys(secrets)
	if err != nil {
		return 0, err
	}
	for key, value := range incoming {
		secrets[key] = value
		delete(readable, key)
	}
	if provenanceValid {
		for _, key := range canonicalAgentReadableKeys(incoming, portable.Keys) {
			readable[key] = struct{}{}
		}
	}
	if err := v.storeAgentReadableKeys(secrets, readable); err != nil {
		return 0, err
	}
	if err := v.encryptAndSave(secrets); err != nil {
		return 0, err
	}
	return len(incoming), nil
}
