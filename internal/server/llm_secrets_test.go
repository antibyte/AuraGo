package server

import (
	"strings"
	"testing"

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
		dummyKey     = "local-dummy-key"
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
		{ID: "local-e7", Type: "openai", APIKey: dummyKey},
	}

	registerLLMSecrets(cfg)

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
	if got := security.Scrub("use local-dummy-key here"); got != "use local-dummy-key here" {
		t.Fatalf("Scrub(placeholder key) = %q, want it unchanged", got)
	}
}

func TestRegisterLLMSecretsIgnoresNilAndEmptyConfig(t *testing.T) {
	t.Parallel()

	registerLLMSecrets(nil)
	registerLLMSecrets(&config.Config{})
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

func TestLLMSecretWorthMasking(t *testing.T) {
	t.Parallel()

	for value, want := range map[string]bool{
		"":                         false,
		"ollama":                   false,
		"lm-studio":                false,
		"not-needed":               false,
		"sk-no-key-required":       false,
		"12345678":                 false,
		"abc12345":                 true,
		"sk-test-abcdef1234567890": true,
		"abcdefghijklmnopqrstuvwx": true,
		"  gsk_9d8c7b6a5f4e3d2c  ": true,
	} {
		if got := llmSecretWorthMasking(value); got != want {
			t.Errorf("llmSecretWorthMasking(%q) = %v, want %v", value, got, want)
		}
	}
}
