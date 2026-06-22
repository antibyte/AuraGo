package server

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"aurago/internal/config"
)

const (
	oauthFlowModeBrowserCallback = "browser_callback"
	oauthFlowModeManualPaste     = "manual_paste"
	oauthSessionTTL              = 10 * time.Minute
)

type oauthSecretStore interface {
	ReadSecret(key string) (string, error)
	WriteSecret(key, value string) error
	DeleteSecret(key string) error
}

type oauthSession struct {
	Version             int       `json:"version,omitempty"`
	ProviderID          string    `json:"provider_id"`
	State               string    `json:"state,omitempty"`
	Mode                string    `json:"mode,omitempty"`
	RedirectURI         string    `json:"redirect_uri,omitempty"`
	CreatedAt           time.Time `json:"created_at,omitempty"`
	ExpiresAt           time.Time `json:"expires_at,omitempty"`
	CodeVerifier        string    `json:"code_verifier,omitempty"`
	CodeChallenge       string    `json:"code_challenge,omitempty"`
	CodeChallengeMethod string    `json:"code_challenge_method,omitempty"`
	FallbackModes       []string  `json:"fallback_modes,omitempty"`
}

func newOAuthPKCE() (verifier, challenge, method string, err error) {
	verifierBytes := make([]byte, 32)
	if _, err := rand.Read(verifierBytes); err != nil {
		return "", "", "", fmt.Errorf("generate PKCE verifier: %w", err)
	}
	verifier = base64.RawURLEncoding.EncodeToString(verifierBytes)
	sum := sha256.Sum256([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(sum[:])
	return verifier, challenge, "S256", nil
}

func newOAuthSession(providerID, mode, redirectURI string, now time.Time) (oauthSession, error) {
	if providerID == "" {
		return oauthSession{}, fmt.Errorf("provider id is required")
	}
	if mode == "" {
		mode = oauthFlowModeBrowserCallback
	}
	stateBytes := make([]byte, 32)
	if _, err := rand.Read(stateBytes); err != nil {
		return oauthSession{}, fmt.Errorf("generate OAuth state: %w", err)
	}
	verifier, challenge, method, err := newOAuthPKCE()
	if err != nil {
		return oauthSession{}, err
	}
	return oauthSession{
		Version:             1,
		ProviderID:          providerID,
		State:               hex.EncodeToString(stateBytes),
		Mode:                mode,
		RedirectURI:         redirectURI,
		CreatedAt:           now.UTC(),
		ExpiresAt:           now.Add(oauthSessionTTL).UTC(),
		CodeVerifier:        verifier,
		CodeChallenge:       challenge,
		CodeChallengeMethod: method,
		FallbackModes:       []string{oauthFlowModeManualPaste},
	}, nil
}

func storeOAuthSession(store oauthSecretStore, session oauthSession) error {
	if store == nil {
		return fmt.Errorf("OAuth session store is not available")
	}
	if session.State == "" {
		return fmt.Errorf("OAuth session state is required")
	}
	data, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("marshal OAuth session: %w", err)
	}
	if err := store.WriteSecret("oauth_state_"+session.State, string(data)); err != nil {
		return fmt.Errorf("store OAuth session: %w", err)
	}
	return nil
}

func consumeOAuthSession(store oauthSecretStore, state string, now time.Time) (oauthSession, error) {
	if store == nil {
		return oauthSession{}, fmt.Errorf("OAuth session store is not available")
	}
	if state == "" {
		return oauthSession{}, fmt.Errorf("OAuth state is required")
	}
	key := "oauth_state_" + state
	raw, err := store.ReadSecret(key)
	if err != nil || raw == "" {
		return oauthSession{}, fmt.Errorf("invalid or expired OAuth state")
	}
	_ = store.DeleteSecret(key)

	var session oauthSession
	if err := json.Unmarshal([]byte(raw), &session); err != nil {
		return oauthSession{}, fmt.Errorf("corrupt OAuth state data")
	}
	if session.ProviderID == "" {
		return oauthSession{}, fmt.Errorf("OAuth state is missing provider")
	}
	if session.State == "" {
		session.State = state
	}
	if session.Mode == "" {
		session.Mode = oauthFlowModeBrowserCallback
	}
	if session.ExpiresAt.IsZero() && !session.CreatedAt.IsZero() {
		session.ExpiresAt = session.CreatedAt.Add(oauthSessionTTL)
	}
	if !session.ExpiresAt.IsZero() && now.UTC().After(session.ExpiresAt) {
		return oauthSession{}, fmt.Errorf("OAuth state expired")
	}
	return session, nil
}

func buildOAuthAuthorizationURL(prov config.ProviderEntry, session oauthSession) (string, error) {
	authURL, err := url.Parse(prov.OAuthAuthURL)
	if err != nil {
		return "", fmt.Errorf("invalid oauth_auth_url: %w", err)
	}
	q := authURL.Query()
	q.Set("response_type", "code")
	q.Set("client_id", prov.OAuthClientID)
	q.Set("redirect_uri", session.RedirectURI)
	q.Set("state", session.State)
	if prov.OAuthScopes != "" {
		q.Set("scope", prov.OAuthScopes)
	}
	if session.CodeChallenge != "" {
		q.Set("code_challenge", session.CodeChallenge)
		q.Set("code_challenge_method", session.CodeChallengeMethod)
	}
	q.Set("access_type", "offline")
	q.Set("prompt", "consent")
	authURL.RawQuery = q.Encode()
	return authURL.String(), nil
}

func oauthTokenFromExchange(tokenResp *tokenExchangeResponse, now time.Time) config.OAuthToken {
	tok := config.OAuthToken{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		TokenType:    tokenResp.TokenType,
	}
	if tokenResp.ExpiresIn > 0 {
		tok.Expiry = now.Add(time.Duration(tokenResp.ExpiresIn) * time.Second).UTC().Format(time.RFC3339)
	}
	return tok
}

func storeOAuthToken(store oauthSecretStore, providerID string, tokenResp *tokenExchangeResponse, now time.Time) error {
	if store == nil {
		return fmt.Errorf("OAuth token store is not available")
	}
	if providerID == "" {
		return fmt.Errorf("provider id is required")
	}
	tok := oauthTokenFromExchange(tokenResp, now)
	tokJSON, err := json.Marshal(tok)
	if err != nil {
		return fmt.Errorf("marshal OAuth token: %w", err)
	}
	if err := store.WriteSecret("oauth_"+providerID, string(tokJSON)); err != nil {
		return fmt.Errorf("store OAuth token: %w", err)
	}
	return nil
}
