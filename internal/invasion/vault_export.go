package invasion

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"

	"aurago/internal/security"
)

// ExportVaultForEgg reads all secrets from the master vault, serialises them
// to JSON, and encrypts the blob with a freshly generated AES-256 key.
// Returns the encrypted data and the new key (hex-encoded).
// The egg will use this key as its AURAGO_MASTER_KEY to open its own vault.
func ExportVaultForEgg(vault *security.Vault) (encryptedData []byte, newKeyHex string, err error) {
	keys, err := vault.ListKeys()
	if err != nil {
		return nil, "", fmt.Errorf("failed to list vault keys: %w", err)
	}

	secrets := make(map[string]string, len(keys))
	for _, k := range keys {
		val, err := vault.ReadSecret(k)
		if err != nil {
			return nil, "", fmt.Errorf("failed to read vault key %q: %w", k, err)
		}
		secrets[k] = val
	}

	plaintext, err := json.Marshal(secrets)
	if err != nil {
		return nil, "", fmt.Errorf("failed to marshal secrets: %w", err)
	}

	// Generate a new 32-byte key for the egg
	newKey := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, newKey); err != nil {
		return nil, "", fmt.Errorf("failed to generate egg key: %w", err)
	}
	newKeyHex = hex.EncodeToString(newKey)

	// Encrypt the secrets with the new key (same AES-256-GCM format the Vault uses)
	block, err := aes.NewCipher(newKey)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create GCM: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	encryptedData = gcm.Seal(nonce, nonce, plaintext, nil)
	return encryptedData, newKeyHex, nil
}

// GenerateSharedKey creates a random 32-byte hex-encoded key for master↔egg HMAC.
func GenerateSharedKey() (string, error) {
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return "", fmt.Errorf("failed to generate shared key: %w", err)
	}
	return hex.EncodeToString(key), nil
}
