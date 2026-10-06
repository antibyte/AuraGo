package server

import (
	"strings"
	"sync"

	"aurago/internal/config"
	"aurago/internal/security"
)

// llmSecretScope tracks the OAuth access tokens that currently sit in the LLM
// key slots. Access tokens rotate on every refresh, so they use releasable
// scoped registrations instead of the permanent registry that security.Scrub
// walks for every log attribute; otherwise each refresh would add an entry for
// the lifetime of the process.
type llmSecretScope struct {
	mu       sync.Mutex
	releases map[string]func()
}

// defaultLLMSecretScope serves the single server of the process.
var defaultLLMSecretScope llmSecretScope

// registerLLMSecrets registers the LLM credentials of cfg for log and output
// scrubbing: the main and helper keys, the fallback key and every provider's
// API key and OAuth client secret. It is safe to call on every config reload.
func registerLLMSecrets(cfg *config.Config) {
	defaultLLMSecretScope.register(cfg)
}

func (scope *llmSecretScope) register(cfg *config.Config) {
	if cfg == nil {
		return
	}
	oauthProviders := make(map[string]bool, len(cfg.Providers))
	for _, provider := range cfg.Providers {
		if provider.AuthType == "oauth2" {
			oauthProviders[provider.ID] = true
		}
		registerStaticLLMSecret(provider.APIKey)
		registerStaticLLMSecret(provider.OAuthClientSecret)
	}

	// ApplyOAuthTokens copies the current access token of an oauth2 provider into
	// the slot that references it; those values are scoped, everything else is
	// a static key that stays masked even after the user rotates it.
	accessTokens := make(map[string]struct{})
	slot := func(providerID, value string) {
		if !oauthProviders[providerID] {
			registerStaticLLMSecret(value)
			return
		}
		if value = strings.TrimSpace(value); llmSecretWorthMasking(value) {
			accessTokens[value] = struct{}{}
		}
	}
	slot(cfg.LLM.Provider, cfg.LLM.APIKey)
	slot(cfg.LLM.HelperProvider, cfg.LLM.HelperAPIKey)
	slot(cfg.FallbackLLM.Provider, cfg.FallbackLLM.APIKey)

	scope.replaceAccessTokens(accessTokens)
}

func (scope *llmSecretScope) replaceAccessTokens(current map[string]struct{}) {
	scope.mu.Lock()
	defer scope.mu.Unlock()
	if scope.releases == nil {
		scope.releases = make(map[string]func())
	}
	for value := range current {
		if _, registered := scope.releases[value]; !registered {
			scope.releases[value] = security.RegisterScopedSensitiveExact(value)
		}
	}
	for value, release := range scope.releases {
		if _, keep := current[value]; !keep {
			release()
			delete(scope.releases, value)
		}
	}
}

func registerStaticLLMSecret(value string) {
	if value = strings.TrimSpace(value); llmSecretWorthMasking(value) {
		security.RegisterSensitive(value)
	}
}

// llmSecretWorthMasking reports whether a configured key looks like a generated
// credential. Local providers commonly carry placeholder keys such as
// "lm-studio" or "not-needed"; registering those would redact ordinary words in
// chat output, tool results and logs. Real provider keys are long or mix
// letters and digits.
func llmSecretWorthMasking(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) >= 24 {
		return true
	}
	if len(value) < 8 {
		return false
	}
	hasLetter, hasDigit := false, false
	for _, r := range value {
		switch {
		case r >= '0' && r <= '9':
			hasDigit = true
		case (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z'):
			hasLetter = true
		}
	}
	return hasLetter && hasDigit
}
