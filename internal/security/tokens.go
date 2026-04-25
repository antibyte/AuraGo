package security

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"aurago/internal/uid"
)

// Token represents an API token with scopes and metadata.
// The raw token is only returned once at creation time; only the SHA-256 hash is persisted.
type Token struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	TokenHash  string     `json:"token_hash"`
	Prefix     string     `json:"prefix"`
	Scopes     []string   `json:"scopes"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	Enabled    bool       `json:"enabled"`
}

// TokenMeta is the public view of a token (no hash).
type TokenMeta struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Prefix     string     `json:"prefix"`
	Scopes     []string   `json:"scopes"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	Enabled    bool       `json:"enabled"`
}

// TokenManager provides CRUD and validation for API tokens.
// Token data is stored as an AES-encrypted JSON file via the Vault's master key.
type TokenManager struct {
	mu       sync.RWMutex
	filePath string
	vault    *Vault
	tokens   []Token
}

// NewTokenManager creates a new TokenManager. It loads existing tokens from disk.
func NewTokenManager(vault *Vault, filePath string) (*TokenManager, error) {
	tm := &TokenManager{
		filePath: filePath,
		vault:    vault,
	}
	if err := tm.load(); err != nil {
		// If file doesn't exist, start with empty list
		tm.tokens = []Token{}
	}
	return tm, nil
}

// load reads and decrypts the token file.
func (tm *TokenManager) load() error {
	data, err := os.ReadFile(tm.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			tm.tokens = []Token{}
			return nil
		}
		return fmt.Errorf("failed to read token file: %w", err)
	}
	if len(data) == 0 {
		tm.tokens = []Token{}
		return nil
	}

	// Decrypt using vault's key
	plaintext, err := tm.vault.DecryptBytes(data)
	if err != nil {
		return fmt.Errorf("failed to decrypt token file: %w", err)
	}

	var tokens []Token
	if err := json.Unmarshal(plaintext, &tokens); err != nil {
		return fmt.Errorf("failed to unmarshal tokens: %w", err)
	}
	tm.tokens = tokens
	return nil
}

// save encrypts and writes the token file atomically (write-to-temp then rename).
func (tm *TokenManager) save() error {
	data, err := json.MarshalIndent(tm.tokens, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal tokens: %w", err)
	}

	ciphertext, err := tm.vault.EncryptBytes(data)
	if err != nil {
		return fmt.Errorf("failed to encrypt token file: %w", err)
	}

	if err := writeFileAtomicSynced(tm.filePath, ciphertext, 0o600); err != nil {
		return fmt.Errorf("failed to write token file: %w", err)
	}
	return nil
}

func writeFileAtomicSynced(path string, data []byte, perm os.FileMode) error {
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

	if dirHandle, err := os.Open(dir); err == nil {
		_ = dirHandle.Sync()
		_ = dirHandle.Close()
	}
	return nil
}

// generateToken creates a random token string: aura_ + 32 hex chars = 37 chars total.
func generateToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "aura_" + hex.EncodeToString(b), nil
}

// hashToken returns the SHA-256 hex digest of a raw token.
func hashToken(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}

// Create generates a new token and returns the raw token (shown only once) and its metadata.
func (tm *TokenManager) Create(name string, scopes []string, expiresAt *time.Time) (string, TokenMeta, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	raw, err := generateToken()
	if err != nil {
		return "", TokenMeta{}, fmt.Errorf("failed to generate token: %w", err)
	}

	now := time.Now().UTC()
	t := Token{
		ID:        uid.New(),
		Name:      name,
		TokenHash: hashToken(raw),
		Prefix:    raw[:13] + "...", // "aura_" + 8 hex chars + "..."
		Scopes:    scopes,
		CreatedAt: now,
		Enabled:   true,
	}
	if expiresAt != nil {
		exp := expiresAt.UTC()
		t.ExpiresAt = &exp
	}

	tm.tokens = append(tm.tokens, t)
	if err := tm.save(); err != nil {
		// Roll back
		tm.tokens = tm.tokens[:len(tm.tokens)-1]
		return "", TokenMeta{}, err
	}

	return raw, tm.toMeta(t), nil
}

// List returns metadata for all tokens (without hashes).
func (tm *TokenManager) List() []TokenMeta {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	result := make([]TokenMeta, len(tm.tokens))
	for i, t := range tm.tokens {
		result[i] = tm.toMeta(t)
	}
	return result
}

// Get returns metadata for a single token.
func (tm *TokenManager) Get(id string) (TokenMeta, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	for _, t := range tm.tokens {
		if t.ID == id {
			return tm.toMeta(t), nil
		}
	}
	return TokenMeta{}, fmt.Errorf("token not found")
}

// Update changes token name and/or enabled status.
func (tm *TokenManager) Update(id string, name string, enabled bool) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	for i := range tm.tokens {
		if tm.tokens[i].ID == id {
			if name != "" {
				tm.tokens[i].Name = name
			}
			tm.tokens[i].Enabled = enabled
			return tm.save()
		}
	}
	return fmt.Errorf("token not found")
}

// Delete removes a token by ID.
func (tm *TokenManager) Delete(id string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	for i := range tm.tokens {
		if tm.tokens[i].ID == id {
			tm.tokens = append(tm.tokens[:i], tm.tokens[i+1:]...)
			return tm.save()
		}
	}
	return fmt.Errorf("token not found")
}

// Validate checks a raw token against stored hashes and verifies scope, enabled, and expiry.
// Returns the matching token metadata and true if valid.
func (tm *TokenManager) Validate(rawToken string, requiredScope string) (TokenMeta, bool) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	targetHash := hashToken(rawToken)

	for _, t := range tm.tokens {
		// Constant-time comparison to prevent timing attacks
		if subtle.ConstantTimeCompare([]byte(t.TokenHash), []byte(targetHash)) != 1 {
			continue
		}

		if !t.Enabled {
			return TokenMeta{}, false
		}

		if t.ExpiresAt != nil && time.Now().UTC().After(*t.ExpiresAt) {
			return TokenMeta{}, false
		}

		if requiredScope != "" {
			hasScope := false
			for _, s := range t.Scopes {
				if s == requiredScope {
					hasScope = true
					break
				}
			}
			if !hasScope {
				return TokenMeta{}, false
			}
		}

		return tm.toMeta(t), true
	}

	return TokenMeta{}, false
}

// TouchLastUsed updates the LastUsedAt timestamp for a token.
func (tm *TokenManager) TouchLastUsed(id string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	now := time.Now().UTC()
	for i := range tm.tokens {
		if tm.tokens[i].ID == id {
			tm.tokens[i].LastUsedAt = &now
			_ = tm.save() // best-effort
			return
		}
	}
}

// Count returns the number of stored tokens.
func (tm *TokenManager) Count() int {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return len(tm.tokens)
}

func (tm *TokenManager) toMeta(t Token) TokenMeta {
	return TokenMeta{
		ID:         t.ID,
		Name:       t.Name,
		Prefix:     t.Prefix,
		Scopes:     t.Scopes,
		CreatedAt:  t.CreatedAt,
		LastUsedAt: t.LastUsedAt,
		ExpiresAt:  t.ExpiresAt,
		Enabled:    t.Enabled,
	}
}
