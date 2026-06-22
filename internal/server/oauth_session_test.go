package server

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/security"
)

func newOAuthSessionTestVault(t *testing.T) *security.Vault {
	t.Helper()
	vault, err := security.NewVault(strings.Repeat("b", 64), filepath.Join(t.TempDir(), "vault.bin"))
	if err != nil {
		t.Fatalf("security.NewVault() error = %v", err)
	}
	return vault
}

func TestOAuthPKCEChallengeUsesS256URLSafeNoPadding(t *testing.T) {
	t.Parallel()

	verifier, challenge, method, err := newOAuthPKCE()
	if err != nil {
		t.Fatalf("newOAuthPKCE() error = %v", err)
	}
	if len(verifier) < 43 || len(verifier) > 128 {
		t.Fatalf("verifier length = %d, want RFC 7636 range 43..128", len(verifier))
	}
	if strings.ContainsAny(challenge, "+/=") {
		t.Fatalf("challenge %q is not raw URL-safe base64", challenge)
	}
	sum := sha256.Sum256([]byte(verifier))
	want := base64.RawURLEncoding.EncodeToString(sum[:])
	if challenge != want {
		t.Fatalf("challenge = %q, want S256 challenge %q", challenge, want)
	}
	if method != "S256" {
		t.Fatalf("method = %q, want S256", method)
	}
}

func TestOAuthSessionRoundTripConsumesVaultState(t *testing.T) {
	t.Parallel()

	vault := newOAuthSessionTestVault(t)
	now := time.Date(2026, 6, 21, 8, 0, 0, 0, time.UTC)
	session, err := newOAuthSession("main", oauthFlowModeBrowserCallback, "https://aurago.example/api/oauth/callback", now)
	if err != nil {
		t.Fatalf("newOAuthSession() error = %v", err)
	}
	if session.ProviderID != "main" || session.Mode != oauthFlowModeBrowserCallback {
		t.Fatalf("unexpected session identity: %+v", session)
	}
	if session.State == "" || session.CodeVerifier == "" || session.CodeChallenge == "" {
		t.Fatalf("session missing state or PKCE fields: %+v", session)
	}
	if session.RedirectURI != "https://aurago.example/api/oauth/callback" {
		t.Fatalf("redirect_uri = %q", session.RedirectURI)
	}
	if !session.ExpiresAt.After(now) {
		t.Fatalf("expires_at = %s, want after %s", session.ExpiresAt, now)
	}

	if err := storeOAuthSession(vault, session); err != nil {
		t.Fatalf("storeOAuthSession() error = %v", err)
	}
	raw, err := vault.ReadSecret("oauth_state_" + session.State)
	if err != nil {
		t.Fatalf("stored session missing from vault: %v", err)
	}
	var stored oauthSession
	if err := json.Unmarshal([]byte(raw), &stored); err != nil {
		t.Fatalf("stored session JSON invalid: %v", err)
	}
	if stored.CodeVerifier != session.CodeVerifier {
		t.Fatal("stored session must keep the PKCE verifier server-side")
	}

	got, err := consumeOAuthSession(vault, session.State, now.Add(time.Minute))
	if err != nil {
		t.Fatalf("consumeOAuthSession() error = %v", err)
	}
	if got.ProviderID != "main" || got.CodeVerifier != session.CodeVerifier {
		t.Fatalf("consumed session mismatch: %+v", got)
	}
	if _, err := vault.ReadSecret("oauth_state_" + session.State); err == nil {
		t.Fatal("consumeOAuthSession() must delete one-time state")
	}
}

func TestOAuthSessionRejectsExpiredState(t *testing.T) {
	t.Parallel()

	vault := newOAuthSessionTestVault(t)
	now := time.Date(2026, 6, 21, 8, 0, 0, 0, time.UTC)
	session, err := newOAuthSession("main", oauthFlowModeBrowserCallback, "https://aurago.example/api/oauth/callback", now)
	if err != nil {
		t.Fatalf("newOAuthSession() error = %v", err)
	}
	session.ExpiresAt = now.Add(-time.Minute)
	if err := storeOAuthSession(vault, session); err != nil {
		t.Fatalf("storeOAuthSession() error = %v", err)
	}

	if _, err := consumeOAuthSession(vault, session.State, now); err == nil {
		t.Fatal("consumeOAuthSession() succeeded for expired state")
	}
	if _, err := vault.ReadSecret("oauth_state_" + session.State); err == nil {
		t.Fatal("expired state should be removed during consumption")
	}
}

func TestOAuthSessionAcceptsLegacyProviderState(t *testing.T) {
	t.Parallel()

	vault := newOAuthSessionTestVault(t)
	const state = "legacy-state"
	if err := vault.WriteSecret("oauth_state_"+state, `{"provider_id":"legacy","created_at":"2026-06-21T08:00:00Z"}`); err != nil {
		t.Fatalf("WriteSecret() error = %v", err)
	}

	got, err := consumeOAuthSession(vault, state, time.Date(2026, 6, 21, 8, 1, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("consumeOAuthSession() error = %v", err)
	}
	if got.ProviderID != "legacy" || got.State != state {
		t.Fatalf("legacy session = %+v, want provider legacy and state %q", got, state)
	}
	if got.CodeVerifier != "" {
		t.Fatalf("legacy session code_verifier = %q, want empty", got.CodeVerifier)
	}
}
