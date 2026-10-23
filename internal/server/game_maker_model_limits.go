package server

import (
	"net/http"
	"strings"

	"aurago/internal/config"
	"aurago/internal/llm"
)

// gameMakerMinContextWindow is the context floor for a new Studio job. Below it
// the phase packet (system prompt, tool schemas, current sources and the output
// reserve) cannot fit reliably; the operator may opt in explicitly.
const gameMakerMinContextWindow = 16384

// Small-model promotion boundary: the same route thresholds gameMakerOutputTokens
// applies to the reasoning allowance. A route below either value gets the compact
// profile and, for a new plan-bound game, the adapt building mode.
const (
	gameMakerSmallContextWindow = 65536
	gameMakerSmallOutputTokens  = 16384
)

// gameMakerSmallModelDecision resolves the recorded small-model decision for a
// job. An explicit opt-in always wins; automatic detection also promotes a route
// that resolves below the boundary, so an explicit false cannot switch it off.
// Unknown models keep the conservative defaults and are never treated as small.
func gameMakerSmallModelDecision(cfg *config.Config, providerID, model string, explicit bool) bool {
	if explicit {
		return true
	}
	if cfg == nil {
		return false
	}
	limits := gameMakerModelLimits(cfg, providerID, model)
	if limits.Unknown {
		return false
	}
	if limits.ContextWindow > 0 && limits.ContextWindow < gameMakerSmallContextWindow {
		return true
	}
	return limits.MaxOutputTokens > 0 && limits.MaxOutputTokens < gameMakerSmallOutputTokens
}

// gameMakerModelLimits resolves the effective limits for a provider/model pair
// without network probes. Unknown models keep the conservative defaults and are
// never treated as verified-small.
func gameMakerModelLimits(cfg *config.Config, providerID, model string) llm.ModelLimits {
	defaults := llm.ModelLimits{Unknown: true}
	if cfg == nil {
		return defaults
	}
	route := llm.ModelRoute{
		ProviderID:   cfg.LLM.Provider,
		ProviderType: cfg.LLM.ProviderType,
		BaseURL:      cfg.LLM.BaseURL,
		Model:        cfg.LLM.Model,
		Primary:      true,
	}
	if providerID = strings.TrimSpace(providerID); providerID != "" {
		if provider := cfg.FindProvider(providerID); provider != nil {
			route.ProviderID = provider.ID
			route.ProviderType = provider.Type
			route.BaseURL = provider.BaseURL
			route.Model = provider.Model
			route.ContextWindowOverride = provider.ContextWindow
			route.MaxOutputTokensOverride = provider.MaxOutputTokens
		}
	}
	if model = strings.TrimSpace(model); model != "" {
		route.Model = model
	}
	if strings.TrimSpace(route.Model) == "" {
		return defaults
	}
	return llm.ResolveModelLimitsCached(route, cfg.Agent.ContextWindow)
}

// gameMakerModelLimitBlocked reports a known model whose usable context is below
// the Studio floor. Unknown models and explicit opt-ins are never blocked.
func gameMakerModelLimitBlocked(cfg *config.Config, providerID, model string, allowSmall bool) (int, bool) {
	if allowSmall {
		return 0, false
	}
	limits := gameMakerModelLimits(cfg, providerID, model)
	if limits.Unknown || limits.ContextWindow <= 0 || limits.ContextWindow >= gameMakerMinContextWindow {
		return limits.ContextWindow, false
	}
	return limits.ContextWindow, true
}

func handleGameMakerModelLimits(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireDesktopPermission(s, w, r, desktopScopeRead) {
			return
		}
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		cfg := s.ConfigSnapshot()
		if cfg == nil {
			jsonError(w, "server not configured", http.StatusServiceUnavailable)
			return
		}
		providerID := strings.TrimSpace(r.URL.Query().Get("provider_id"))
		model := strings.TrimSpace(r.URL.Query().Get("model"))
		if providerID == "" && model == "" {
			jsonError(w, "provider_id or model is required", http.StatusBadRequest)
			return
		}
		limits := gameMakerModelLimits(cfg, providerID, model)
		writeGameMakerJSON(w, http.StatusOK, map[string]any{
			"context_window":         limits.ContextWindow,
			"max_output_tokens":      limits.MaxOutputTokens,
			"known":                  !limits.Unknown,
			"context_source":         limits.ContextSource,
			"minimum_context_window": gameMakerMinContextWindow,
			"below_minimum":          !limits.Unknown && limits.ContextWindow > 0 && limits.ContextWindow < gameMakerMinContextWindow,
			"warning":                limits.Warning,
		})
	}
}
