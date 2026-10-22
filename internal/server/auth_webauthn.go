package server

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"aurago/internal/config"
	"aurago/internal/httporigin"
	"aurago/internal/security"

	"github.com/go-webauthn/webauthn/protocol"
	wa "github.com/go-webauthn/webauthn/webauthn"
)

const (
	webAuthnVaultKey       = "auth_webauthn_state"
	webAuthnCookie         = "aurago_webauthn"
	webAuthnTTL            = 2 * time.Minute
	webAuthnMaxCredentials = 16
)

type webAuthnCredential struct {
	Credential wa.Credential `json:"credential"`
	Label      string        `json:"label"`
	CreatedAt  time.Time     `json:"created_at"`
	LastUsedAt time.Time     `json:"last_used_at,omitempty"`
}

type webAuthnState struct {
	Version     int                  `json:"version"`
	RPID        string               `json:"rp_id"`
	UserID      []byte               `json:"user_id"`
	Credentials []webAuthnCredential `json:"credentials"`
}

func (s *webAuthnState) WebAuthnID() []byte          { return s.UserID }
func (s *webAuthnState) WebAuthnName() string        { return "AuraGo owner" }
func (s *webAuthnState) WebAuthnDisplayName() string { return "AuraGo owner" }
func (s *webAuthnState) WebAuthnCredentials() []wa.Credential {
	credentials := make([]wa.Credential, len(s.Credentials))
	for i := range s.Credentials {
		credentials[i] = s.Credentials[i].Credential
	}
	return credentials
}

// All registry read/modify/write operations hold CfgSaveMu, before webAuthn.mu.
func (s *Server) loadWebAuthnState() (*webAuthnState, error) {
	if s.Vault == nil {
		return nil, errors.New("webauthn vault unavailable")
	}
	raw, err := s.Vault.ReadSecret(webAuthnVaultKey)
	if errors.Is(err, security.ErrSecretNotFound) {
		return &webAuthnState{Version: 1}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read webauthn registry: %w", err)
	}
	if len(raw) > 1<<20 {
		return nil, errors.New("webauthn registry exceeds its size limit")
	}
	var state webAuthnState
	if err := json.Unmarshal([]byte(raw), &state); err != nil {
		return nil, fmt.Errorf("decode webauthn registry: %w", err)
	}
	if state.Version != 1 || len(state.UserID) != 32 || state.RPID == "" || len(state.Credentials) > webAuthnMaxCredentials {
		return nil, errors.New("invalid webauthn registry")
	}
	seen := make(map[string]bool)
	for _, c := range state.Credentials {
		id := base64.RawURLEncoding.EncodeToString(c.Credential.ID)
		if id == "" || len(c.Credential.ID) > 1024 || len(c.Credential.PublicKey) == 0 || !c.Credential.Flags.UserVerified || !validWebAuthnLabel(c.Label) || seen[id] {
			return nil, errors.New("invalid webauthn credential record")
		}
		seen[id] = true
	}
	return &state, nil
}

func (s *Server) saveWebAuthnState(state *webAuthnState) error {
	raw, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("encode webauthn registry: %w", err)
	}
	if len(raw) > 1<<20 {
		return errors.New("webauthn registry exceeds its size limit")
	}
	if err := s.Vault.WriteSecret(webAuthnVaultKey, string(raw)); err != nil {
		return fmt.Errorf("save webauthn registry: %w", err)
	}
	return nil
}

func validWebAuthnLabel(label string) bool {
	return strings.TrimSpace(label) == label && utf8.ValidString(label) && utf8.RuneCountInString(label) > 0 && utf8.RuneCountInString(label) <= 64 && strings.IndexFunc(label, unicode.IsControl) < 0
}

type webAuthnAuth struct {
	Enabled                                   bool
	PasswordHash, TOTPSecret, SessionSecret   string
	TOTPEnabled                               bool
	TimeoutHours, MaxAttempts, LockoutMinutes int
	Config                                    config.WebAuthnConfig
}

func (s *Server) webAuthnAuthSnapshot() (webAuthnAuth, *config.Config) {
	s.CfgMu.RLock()
	defer s.CfgMu.RUnlock()
	a := s.Cfg.Auth
	a.WebAuthn.Origins = append([]string(nil), a.WebAuthn.Origins...)
	return webAuthnAuth{a.Enabled, a.PasswordHash, a.TOTPSecret, a.SessionSecret, a.TOTPEnabled, a.SessionTimeoutHours, a.MaxLoginAttempts, a.LockoutMinutes, a.WebAuthn}, s.Cfg
}

func (a webAuthnAuth) fingerprint() [32]byte {
	raw, _ := json.Marshal(a)
	return sha256.Sum256(raw)
}

func webAuthnClient(a webAuthnAuth, origin string) (*wa.WebAuthn, error) {
	if !a.Enabled || a.PasswordHash == "" || a.SessionSecret == "" {
		return nil, errors.New("owner authentication required")
	}
	if err := config.ValidateWebAuthnConfig(a.Config); err != nil {
		return nil, err
	}
	if a.Config.RPID == "" {
		return nil, errors.New("webauthn is not configured")
	}
	requestConfig := a.Config
	requestConfig.Origins = []string{origin}
	if err := config.ValidateWebAuthnConfig(requestConfig); err != nil {
		return nil, errors.New("webauthn origin rejected")
	}
	matched := false
	originURL, _ := url.Parse(origin)
	for _, allowed := range a.Config.Origins {
		allowedURL, _ := url.Parse(allowed)
		if httporigin.SameOrigin(allowedURL, originURL) {
			matched = true
			break
		}
	}
	if !matched {
		return nil, errors.New("webauthn origin rejected")
	}
	return wa.New(&wa.Config{
		RPID: a.Config.RPID, RPDisplayName: "AuraGo", RPOrigins: []string{origin},
		AttestationPreference:  protocol.PreferNoAttestation,
		AuthenticatorSelection: protocol.AuthenticatorSelection{ResidentKey: protocol.ResidentKeyRequirementPreferred, UserVerification: protocol.VerificationRequired},
		Timeouts:               wa.TimeoutsConfig{Login: wa.TimeoutConfig{Enforce: true, Timeout: webAuthnTTL}, Registration: wa.TimeoutConfig{Enforce: true, Timeout: webAuthnTTL}},
	})
}

type webAuthnTransaction struct {
	Session                            wa.SessionData
	Operation, Origin, Label, Redirect string
	Browser, OwnerSession, AuthHash    [32]byte
	Config                             *config.Config
	Vault                              *security.Vault
	RegistryHash                       [32]byte
	FirstFactor                        bool
	Expires                            time.Time
}

type webAuthnRuntime struct {
	mu           sync.Mutex
	transactions map[string]webAuthnTransaction
	requests     map[string]webAuthnRequestBudget
}

type webAuthnRequestBudget struct {
	Starts int
	Until  time.Time
}

func (s *Server) allowWebAuthnStart(ip string) bool {
	s.webAuthn.mu.Lock()
	defer s.webAuthn.mu.Unlock()
	now := time.Now()
	if s.webAuthn.requests == nil {
		s.webAuthn.requests = make(map[string]webAuthnRequestBudget)
	}
	for key, budget := range s.webAuthn.requests {
		if !now.Before(budget.Until) {
			delete(s.webAuthn.requests, key)
		}
	}
	budget := s.webAuthn.requests[ip]
	if budget.Starts == 0 {
		budget.Until = now.Add(time.Minute)
	}
	if budget.Starts >= 20 || len(s.webAuthn.requests) >= 1024 && budget.Starts == 0 {
		return false
	}
	budget.Starts++
	s.webAuthn.requests[ip] = budget
	return true
}

func webAuthnBrowser(r *http.Request) string {
	if c, err := r.Cookie(webAuthnCookie); err == nil && len(c.Value) == 43 {
		if value, err := base64.RawURLEncoding.DecodeString(c.Value); err == nil && len(value) == 32 {
			return c.Value
		}
	}
	return ""
}

func (s *Server) putWebAuthnTransaction(w http.ResponseWriter, r *http.Request, transaction webAuthnTransaction) (string, error) {
	browser := webAuthnBrowser(r)
	if browser == "" {
		browser = base64.RawURLEncoding.EncodeToString(randomWebAuthnBytes())
		http.SetCookie(w, &http.Cookie{Name: webAuthnCookie, Value: browser, Path: "/", MaxAge: 600, Secure: IsSecureRequest(r), HttpOnly: true, SameSite: http.SameSiteStrictMode})
	}
	transaction.Browser = sha256.Sum256([]byte(browser))
	transaction.Expires = time.Now().Add(webAuthnTTL)
	id := base64.RawURLEncoding.EncodeToString(randomWebAuthnBytes())
	s.webAuthn.mu.Lock()
	defer s.webAuthn.mu.Unlock()
	if s.webAuthn.transactions == nil {
		s.webAuthn.transactions = make(map[string]webAuthnTransaction)
	}
	count := 0
	for key, pending := range s.webAuthn.transactions {
		if !time.Now().Before(pending.Expires) {
			delete(s.webAuthn.transactions, key)
			continue
		}
		if pending.Browser == transaction.Browser {
			count++
		}
	}
	if count >= 4 || len(s.webAuthn.transactions) >= 256 {
		return "", errors.New("webauthn ceremony limit reached")
	}
	s.webAuthn.transactions[id] = transaction
	return id, nil
}

func (s *Server) takeWebAuthnTransaction(r *http.Request, id, operation string, a webAuthnAuth, cfg *config.Config) (webAuthnTransaction, bool) {
	s.webAuthn.mu.Lock()
	tx, ok := s.webAuthn.transactions[id]
	delete(s.webAuthn.transactions, id)
	s.webAuthn.mu.Unlock()
	browser := webAuthnBrowser(r)
	if !ok || browser == "" || !time.Now().Before(tx.Expires) || tx.Operation != operation || tx.Origin != r.Header.Get("Origin") || tx.Browser != sha256.Sum256([]byte(browser)) || tx.AuthHash != a.fingerprint() || tx.Config != cfg || tx.Vault != s.Vault {
		return webAuthnTransaction{}, false
	}
	if operation == "register" {
		cookie, err := r.Cookie(sessionCookieName)
		if err != nil || tx.OwnerSession != sha256.Sum256([]byte(cookie.Value)) || !IsAuthenticated(r, a.SessionSecret) {
			return webAuthnTransaction{}, false
		}
	}
	return tx, true
}

func (s *Server) clearWebAuthnTransactions() {
	s.webAuthn.mu.Lock()
	defer s.webAuthn.mu.Unlock()
	clear(s.webAuthn.transactions)
}

func randomWebAuthnBytes() []byte {
	value := make([]byte, 32)
	_, _ = rand.Read(value) // crypto/rand.Read terminates the process if randomness is unavailable.
	return value
}

func webAuthnRegistryHash(state *webAuthnState) [32]byte {
	raw, _ := json.Marshal(state)
	return sha256.Sum256(raw)
}

func webAuthnCredentialIndex(state *webAuthnState, id string) int {
	for i, c := range state.Credentials {
		if webAuthnCredentialRef(c.Credential.ID) == id {
			return i
		}
	}
	return -1
}

// Administrative URLs carry a digest, so access logs never contain raw WebAuthn credential IDs.
func webAuthnCredentialRef(id []byte) string {
	digest := sha256.Sum256(id)
	return hex.EncodeToString(digest[:])
}
