package security

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

// ErrTokenStoreUnavailable is returned by every mutation while the token file
// could not be loaded or after the manager was replaced. The store is then
// read-only so a write can never replace tokens.json with an empty or stale set.
var ErrTokenStoreUnavailable = errors.New("token store unavailable")

// ErrTokenNotFound is returned when no token has the requested ID.
var ErrTokenNotFound = errors.New("token not found")

var errTokenManagerRetired = errors.New("token manager was replaced by a newer instance")

// TokenManager provides CRUD and validation for API tokens.
// Token data is stored as an AES-encrypted JSON file via the Vault's master key.
type TokenManager struct {
	mu       sync.RWMutex
	filePath string
	vault    *Vault
	tokens   []Token
	loadErr  error // non-nil: read-only (load failure or retired); save refuses
}

// NewTokenManager loads the token file. A missing or empty file starts an
// empty, writable store. Any other load failure (read, decrypt or parse error)
// returns a non-nil read-only manager together with an error wrapping
// ErrTokenStoreUnavailable: it validates no token and refuses every write, so
// the unreadable file is never overwritten.
func NewTokenManager(vault *Vault, filePath string) (*TokenManager, error) {
	tm := &TokenManager{
		filePath: filePath,
		vault:    vault,
	}
	if err := tm.load(); err != nil {
		tm.tokens = []Token{}
		tm.loadErr = err
		return tm, fmt.Errorf("%w: %w", ErrTokenStoreUnavailable, err)
	}
	return tm, nil
}

// LoadError reports why the store is read-only, or nil when it is writable.
func (tm *TokenManager) LoadError() error {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return tm.loadErr
}

// Retire makes a replaced manager read-only so a request that still holds it
// cannot rewrite the token file with its stale token set.
func (tm *TokenManager) Retire() {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	if tm.loadErr == nil {
		tm.loadErr = errTokenManagerRetired
	}
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
	if tm.loadErr != nil {
		return fmt.Errorf("%w: %w", ErrTokenStoreUnavailable, tm.loadErr)
	}
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

const (
	tokenPrefix      = "aura_"
	CYDTokenBodyLen  = 9
	cydTokenAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789" // no 0/O/1/I/L
)

// generateToken creates a random token string: aura_ + 32 hex chars = 37 chars total.
func generateToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return tokenPrefix + hex.EncodeToString(b), nil
}

// generateCYDToken returns aura_ plus 9 unambiguous chars for on-glass entry.
func generateCYDToken() (string, error) {
	body := make([]byte, CYDTokenBodyLen)
	if _, err := rand.Read(body); err != nil {
		return "", err
	}
	alphabet := []byte(cydTokenAlphabet)
	for i := range body {
		body[i] = alphabet[int(body[i])%len(alphabet)]
	}
	return tokenPrefix + string(body), nil
}

func scopesIncludeCYD(scopes []string) bool {
	for _, s := range scopes {
		if strings.EqualFold(strings.TrimSpace(s), "cyd") {
			return true
		}
	}
	return false
}

// NormalizeAPIToken strips grouping and ensures the aura_ prefix.
// Nine-character CYD codes are uppercased; legacy hex tokens are unchanged.
func NormalizeAPIToken(raw string) string {
	var b strings.Builder
	b.Grow(len(raw) + len(tokenPrefix))
	for _, r := range raw {
		if r == ' ' || r == '-' || r == '\t' || r == '\n' {
			continue
		}
		b.WriteRune(r)
	}
	s := b.String()
	if s == "" {
		return ""
	}
	body := s
	if len(s) >= len(tokenPrefix) && strings.EqualFold(s[:len(tokenPrefix)], tokenPrefix) {
		body = s[len(tokenPrefix):]
	}
	if len(body) == CYDTokenBodyLen {
		return tokenPrefix + strings.ToUpper(body)
	}
	return tokenPrefix + body
}

// FormatCYDTokenDisplay shows the 9-character body as XXX XXX XXX.
func FormatCYDTokenDisplay(raw string) string {
	norm := NormalizeAPIToken(raw)
	body := strings.TrimPrefix(norm, tokenPrefix)
	if len(body) != CYDTokenBodyLen {
		return body
	}
	return body[:3] + " " + body[3:6] + " " + body[6:]
}

func visibleTokenPrefix(raw string) string {
	if len(raw) >= len(tokenPrefix)+3 && strings.HasPrefix(raw, tokenPrefix) {
		return raw[:len(tokenPrefix)+3] + "..."
	}
	if len(raw) >= 13 {
		return raw[:13] + "..."
	}
	return raw
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

	var raw string
	var err error
	if scopesIncludeCYD(scopes) {
		raw, err = generateCYDToken()
	} else {
		raw, err = generateToken()
	}
	if err != nil {
		return "", TokenMeta{}, fmt.Errorf("failed to generate token: %w", err)
	}
	raw = NormalizeAPIToken(raw)

	now := time.Now().UTC()
	t := Token{
		ID:        uid.New(),
		Name:      name,
		TokenHash: hashToken(raw),
		Prefix:    visibleTokenPrefix(raw),
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
	return TokenMeta{}, ErrTokenNotFound
}

// Update changes token name and/or enabled status. The in-memory token is
// restored when the change cannot be persisted.
func (tm *TokenManager) Update(id string, name string, enabled bool) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	for i := range tm.tokens {
		if tm.tokens[i].ID == id {
			previous := tm.tokens[i]
			if name != "" {
				tm.tokens[i].Name = name
			}
			tm.tokens[i].Enabled = enabled
			if err := tm.save(); err != nil {
				tm.tokens[i] = previous
				return err
			}
			return nil
		}
	}
	return ErrTokenNotFound
}

// Delete removes a token by ID. The token stays in memory when the removal
// cannot be persisted.
func (tm *TokenManager) Delete(id string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	for i := range tm.tokens {
		if tm.tokens[i].ID == id {
			previous := append([]Token(nil), tm.tokens...)
			tm.tokens = append(tm.tokens[:i], tm.tokens[i+1:]...)
			if err := tm.save(); err != nil {
				tm.tokens = previous
				return err
			}
			return nil
		}
	}
	return ErrTokenNotFound
}

// Validate checks a raw token against stored hashes and verifies scope, enabled, and expiry.
// Returns the matching token metadata and true if valid.
func (tm *TokenManager) Validate(rawToken string, requiredScope string) (TokenMeta, bool) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	targetHash := hashToken(NormalizeAPIToken(rawToken))

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
