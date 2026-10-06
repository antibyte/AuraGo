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

// defaultLLMSecretScope serves the single process-wide runtime config.
var defaultLLMSecretScope llmSecretScope

// RegisterLLMSecrets registers the LLM credentials of cfg for log and output
// scrubbing: the main and helper keys, the fallback key and every provider's
// API key and OAuth client secret. It is safe to call on every config reload;
// cmd/aurago calls it once the vault secrets and OAuth tokens are applied, and
// Start, replaceConfigSnapshot and the OAuth authorize path repeat it.
func RegisterLLMSecrets(cfg *config.Config) {
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
// "lm-studio", "not-needed" or "ollama-does-not-need-a-key"; registering those
// would let Scrub's fragmented match redact the ordinary words in chat output,
// tool results and logs. The decision looks at the runs of [A-Za-z0-9] between
// separators: a key qualifies when one run has at least 8 characters mixing
// letters and digits, or at least 16 letters mixing upper and lower case. Real
// keys (sk-ant-api03-…, sk-or-v1-<hex>, AIza…, gsk_…, hex keys, UUIDs) pass;
// words joined by dashes do not. security.RegisterSensitive additionally
// ignores anything under 8 bytes.
func llmSecretWorthMasking(value string) bool {
	runLen := 0
	hasDigit, hasUpper, hasLower := false, false, false
	qualifies := func() bool {
		hasLetter := hasUpper || hasLower
		return (runLen >= 8 && hasLetter && hasDigit) || (runLen >= 16 && hasUpper && hasLower)
	}
	for _, r := range strings.TrimSpace(value) {
		switch {
		case r >= '0' && r <= '9':
			hasDigit = true
		case r >= 'A' && r <= 'Z':
			hasUpper = true
		case r >= 'a' && r <= 'z':
			hasLower = true
		default:
			if qualifies() {
				return true
			}
			runLen, hasDigit, hasUpper, hasLower = 0, false, false, false
			continue
		}
		runLen++
	}
	return qualifies()
}
