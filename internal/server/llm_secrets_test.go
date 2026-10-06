package server

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/security"
)

// The scrub registry is process-wide, so every value below is unique to this
// file and cannot collide with other tests' expectations.

func TestRegisterLLMSecretsMasksKeysInScrub(t *testing.T) {
	t.Parallel()

	const (
		mainKey      = "sk-test-abcdef1234567890"
		helperKey    = "hk-e7-0c4d8e2f6a1b3c5d7e"
		providerKey  = "pk-e7-9f3c2a71e4b8d605aa"
		clientSecret = "ocs-e7-51d0b9c3a8f24e67"
		dummyKey     = "ollama-does-not-need-a-key"
	)
	cfg := &config.Config{}
	cfg.LLM.Provider = "main-e7"
	cfg.LLM.APIKey = mainKey
	cfg.LLM.HelperAPIKey = helperKey
	cfg.FallbackLLM.Provider = "local-e7"
	cfg.FallbackLLM.APIKey = dummyKey
	cfg.Providers = []config.ProviderEntry{
		{ID: "main-e7", Type: "openai", APIKey: providerKey},
		{ID: "oauth-client-e7", Type: "custom", AuthType: "oauth2", OAuthClientSecret: clientSecret},
		{ID: "local-e7", Type: "ollama", APIKey: dummyKey},
	}
	var scope llmSecretScope

	scope.register(cfg)

	if got := security.Scrub("token " + mainKey + " end"); got != "token [redacted] end" {
		t.Fatalf("Scrub(main key) = %q, want %q", got, "token [redacted] end")
	}
	for name, secret := range map[string]string{"helper key": helperKey, "provider key": providerKey, "oauth client secret": clientSecret} {
		got := security.Scrub("value=" + secret + ";")
		if strings.Contains(got, secret) || !strings.Contains(got, "[redacted]") {
			t.Fatalf("Scrub(%s) = %q, want the value masked as [redacted]", name, got)
		}
	}
	// Local providers often carry placeholder keys; masking them would rewrite
	// ordinary words in chat output and logs.
	const sentence = "ollama does not need a key"
	if got := security.Scrub(sentence); got != sentence {
		t.Fatalf("Scrub(placeholder phrase) = %q, want it unchanged", got)
	}
}

func TestRegisterLLMSecretsIgnoresNilAndEmptyConfig(t *testing.T) {
	t.Parallel()

	var scope llmSecretScope
	scope.register(nil)
	scope.register(&config.Config{})
	if len(scope.releases) != 0 {
		t.Fatalf("empty config registered %d scoped values, want 0", len(scope.releases))
	}
}

func TestLLMSecretScopeReleasesRotatedOAuthAccessTokens(t *testing.T) {
	t.Parallel()

	const (
		firstToken  = "oat-e7-first-4b1c9d2e7f80a3b5"
		secondToken = "oat-e7-second-8a2f6c0d4e1b9375"
	)
	configWithToken := func(token string) *config.Config {
		cfg := &config.Config{}
		cfg.LLM.Provider = "oauth-e7"
		cfg.LLM.APIKey = token
		cfg.Providers = []config.ProviderEntry{{ID: "oauth-e7", Type: "custom", AuthType: "oauth2"}}
		return cfg
	}
	var scope llmSecretScope
	t.Cleanup(func() { scope.replaceAccessTokens(nil) })

	scope.register(configWithToken(firstToken))
	if got := security.Scrub("bearer-ish " + firstToken); strings.Contains(got, firstToken) {
		t.Fatalf("Scrub(current OAuth token) = %q, want it masked", got)
	}

	scope.register(configWithToken(secondToken))
	if got := security.Scrub("bearer-ish " + secondToken); strings.Contains(got, secondToken) {
		t.Fatalf("Scrub(refreshed OAuth token) = %q, want it masked", got)
	}
	// Access tokens rotate on every refresh; the replaced one is released so the
	// registry stays bounded over a long uptime.
	if got := security.Scrub("bearer-ish " + firstToken); !strings.Contains(got, firstToken) {
		t.Fatalf("Scrub(rotated-out OAuth token) = %q, want the scoped registration released", got)
	}
}

// Not parallel: applyOAuthTokenToRuntime publishes through replaceConfigSnapshot,
// which updates the process-wide default scope that parallel tests also update.
func TestApplyOAuthTokenToRuntimeRegistersAccessTokenForScrubbing(t *testing.T) {
	server, vault := newOAuthHandlerTestServer(t, "https://accounts.example/token")
	server.Cfg.LLM.Provider = "main"
	server.Cfg.ResolveProviders()
	before := server.Cfg

	token := "oat-a1b2c3d4e5-" + strconv.FormatInt(time.Now().UnixNano(), 36)
	if got := security.Scrub("auth " + token); !strings.Contains(got, token) {
		t.Fatalf("precondition: fresh token already masked: %q", got)
	}
	raw, err := json.Marshal(config.OAuthToken{AccessToken: token, TokenType: "Bearer", RefreshToken: "refresh-e7b"})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	if err := vault.WriteSecret("oauth_main", string(raw)); err != nil {
		t.Fatalf("WriteSecret() error = %v", err)
	}
	t.Cleanup(func() { defaultLLMSecretScope.replaceAccessTokens(nil) })

	applyOAuthTokenToRuntime(server)

	if server.Cfg.LLM.APIKey != token || server.ConfigSnapshot().LLM.APIKey != token {
		t.Fatalf("runtime LLM APIKey = %q / snapshot %q, want the OAuth access token", server.Cfg.LLM.APIKey, server.ConfigSnapshot().LLM.APIKey)
	}
	if before.LLM.APIKey == token {
		t.Fatal("applyOAuthTokenToRuntime mutated the previously published config in place")
	}
	if got := security.Scrub("auth " + token); strings.Contains(got, token) {
		t.Fatalf("Scrub(authorized OAuth token) = %q, want it masked", got)
	}
}

func TestLLMSecretWorthMasking(t *testing.T) {
	t.Parallel()

	for value, want := range map[string]bool{
		"":                                     false,
		"ollama":                               false,
		"lm-studio":                            false,
		"not-needed":                           false,
		"sk-no-key-required":                   false,
		"ollama-does-not-need-a-key":           false,
		"abcdefghijklmnopqrstuvw":              false, // 23 letters, one case: a word, not a key
		"abc1234":                              false,
		"12345678":                             false,
		"1234567890123456789012345":            false,
		"abc12345":                             true,
		"sk-test-abcdef1234567890":             true,
		"550e8400-e29b-41d4-a716-446655440000": true,
		"sk-or-v1-0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef": true,
		"AIzaFakeKey0123456789abcdefGHIJKLMNOPQ":                                    true,
		"  gsk_9d8c7b6a5f4e3d2c  ":                                                  true,
		"QwErTyUiOpAsDfGhJk":                                                        true, // 18 mixed-case letters
	} {
		if got := llmSecretWorthMasking(value); got != want {
			t.Errorf("llmSecretWorthMasking(%q) = %v, want %v", value, got, want)
		}
	}
}
