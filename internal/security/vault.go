package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/gofrs/flock"
)

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

func (v *Vault) ReadSecret(key string) (string, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

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
		return "", fmt.Errorf("secret not found")
	}

	return val, nil
}

func (v *Vault) WriteSecret(key, value string) error {
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

	secrets[key] = value
	return v.encryptAndSave(secrets)
}

// DeleteSecret removes a secret by key. Returns nil if the key didn't exist.
func (v *Vault) DeleteSecret(key string) error {
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

	delete(secrets, key)
	return v.encryptAndSave(secrets)
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
		keys = append(keys, k)
	}
	return keys, nil
}
