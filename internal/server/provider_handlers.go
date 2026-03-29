package server

import (
	"aurago/internal/config"
	"aurago/internal/llm"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// providerJSON is the API representation of a provider entry.
type providerJSON struct {
	ID                string             `json:"id"`
	Name              string             `json:"name"`
	Type              string             `json:"type"`
	BaseURL           string             `json:"base_url"`
	APIKey            string             `json:"api_key"`
	Model             string             `json:"model"`
	AccountID         string             `json:"account_id"`
	AuthType          string             `json:"auth_type"`
	OAuthAuthURL      string             `json:"oauth_auth_url"`
	OAuthTokenURL     string             `json:"oauth_token_url"`
	OAuthClientID     string             `json:"oauth_client_id"`
	OAuthClientSecret string             `json:"oauth_client_secret"`
	OAuthScopes       string             `json:"oauth_scopes"`
	Models            []config.ModelCost `json:"models,omitempty"`
}

const maskedKey = "••••••••"

// handleProviders dispatches GET / PUT for /api/providers.
func handleProviders(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handleGetProviders(s, w, r)
		case http.MethodPut:
			handlePutProviders(s, w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

// handleGetProviders returns the provider list with API keys masked.
func handleGetProviders(s *Server, w http.ResponseWriter, _ *http.Request) {
	s.CfgMu.RLock()
	providers := s.Cfg.Providers
	s.CfgMu.RUnlock()

	out := make([]providerJSON, len(providers))
	for i, p := range providers {
		apiKey := p.APIKey
		if apiKey != "" {
			apiKey = maskedKey
		}
		clientSecret := p.OAuthClientSecret
		if clientSecret != "" {
			clientSecret = maskedKey
		}
		authType := p.AuthType
		if authType == "" {
			authType = "api_key"
		}
		out[i] = providerJSON{
			ID:                p.ID,
			Name:              p.Name,
			Type:              p.Type,
			BaseURL:           p.BaseURL,
			APIKey:            apiKey,
			Model:             p.Model,
			AccountID:         p.AccountID,
			AuthType:          authType,
			OAuthAuthURL:      p.OAuthAuthURL,
			OAuthTokenURL:     p.OAuthTokenURL,
			OAuthClientID:     p.OAuthClientID,
			OAuthClientSecret: clientSecret,
			OAuthScopes:       p.OAuthScopes,
			Models:            p.Models,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

// handlePutProviders saves a new provider array to config.yaml and hot-reloads.
func handlePutProviders(s *Server, w http.ResponseWriter, r *http.Request) {
	var incoming []providerJSON
	if err := json.NewDecoder(r.Body).Decode(&incoming); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Build id → old secret maps so masked keys are preserved
	s.CfgMu.RLock()
	oldKeyMap := make(map[string]string, len(s.Cfg.Providers))
	oldSecretMap := make(map[string]string, len(s.Cfg.Providers))
	for _, p := range s.Cfg.Providers {
		oldKeyMap[p.ID] = p.APIKey
		oldSecretMap[p.ID] = p.OAuthClientSecret
	}
	configPath := s.Cfg.ConfigPath
	s.CfgMu.RUnlock()

	if configPath == "" {
		http.Error(w, "Config path not set", http.StatusInternalServerError)
		return
	}

	// Convert to ProviderEntry slice; write secrets to vault, not YAML
	entries := make([]config.ProviderEntry, len(incoming))
	for i, p := range incoming {
		p.ID = strings.TrimSpace(p.ID)
		if p.ID == "" {
			http.Error(w, "Provider ID must not be empty", http.StatusBadRequest)
			return
		}

		// ── API Key → vault ──
		apiKey := p.APIKey
		if apiKey == maskedKey {
			// Unchanged — keep existing vault value
			if old, ok := oldKeyMap[p.ID]; ok {
				apiKey = old
			}
		}
		if apiKey != "" && apiKey != maskedKey && s.Vault != nil {
			if err := s.Vault.WriteSecret("provider_"+p.ID+"_api_key", apiKey); err != nil {
				s.Logger.Error("[Providers] Failed to write API key to vault", "id", p.ID, "error", err)
			}
		}

		// ── OAuth Client Secret → vault ──
		clientSecret := p.OAuthClientSecret
		if clientSecret == maskedKey {
			if old, ok := oldSecretMap[p.ID]; ok {
				clientSecret = old
			}
		}
		if clientSecret != "" && clientSecret != maskedKey && s.Vault != nil {
			if err := s.Vault.WriteSecret("provider_"+p.ID+"_oauth_client_secret", clientSecret); err != nil {
				s.Logger.Error("[Providers] Failed to write OAuth secret to vault", "id", p.ID, "error", err)
			}
		}

		authType := p.AuthType
		if authType == "" {
			authType = "api_key"
		}
		entries[i] = config.ProviderEntry{
			ID:                p.ID,
			Name:              p.Name,
			Type:              p.Type,
			BaseURL:           p.BaseURL,
			APIKey:            apiKey,
			Model:             p.Model,
			AccountID:         p.AccountID,
			AuthType:          authType,
			OAuthAuthURL:      p.OAuthAuthURL,
			OAuthTokenURL:     p.OAuthTokenURL,
			OAuthClientID:     p.OAuthClientID,
			OAuthClientSecret: clientSecret,
			OAuthScopes:       p.OAuthScopes,
			Models:            p.Models,
		}
	}

	// Read raw YAML, update providers key, write back
	data, err := os.ReadFile(configPath)
	if err != nil {
		s.Logger.Error("Failed to read config for provider update", "error", err)
		http.Error(w, "Failed to read config", http.StatusInternalServerError)
		return
	}

	var rawCfg map[string]interface{}
	if err := yaml.Unmarshal(data, &rawCfg); err != nil {
		s.Logger.Error("Failed to parse config for provider update", "error", err)
		http.Error(w, "Failed to parse config", http.StatusInternalServerError)
		return
	}

	// Build providers as []interface{} for YAML marshal (secrets excluded)
	provList := make([]interface{}, len(entries))
	for i, e := range entries {
		m := map[string]interface{}{
			"id":       e.ID,
			"name":     e.Name,
			"type":     e.Type,
			"base_url": e.BaseURL,
			"model":    e.Model,
		}
		if e.AccountID != "" {
			m["account_id"] = e.AccountID
		}
		if len(e.Models) > 0 {
			m["models"] = e.Models
		}
		// Secrets are NOT written to YAML — they live in the vault.
		if e.AuthType != "" && e.AuthType != "api_key" {
			m["auth_type"] = e.AuthType
			m["oauth_auth_url"] = e.OAuthAuthURL
			m["oauth_token_url"] = e.OAuthTokenURL
			m["oauth_client_id"] = e.OAuthClientID
			m["oauth_scopes"] = e.OAuthScopes
		}
		provList[i] = m
	}
	rawCfg["providers"] = provList

	out, err := yaml.Marshal(rawCfg)
	if err != nil {
		s.Logger.Error("Failed to marshal config after provider update", "error", err)
		http.Error(w, "Failed to save config", http.StatusInternalServerError)
		return
	}

	if err := os.WriteFile(configPath, out, 0644); err != nil {
		s.Logger.Error("Failed to write config after provider update", "error", err)
		http.Error(w, "Failed to write config", http.StatusInternalServerError)
		return
	}

	// Hot-reload
	s.CfgMu.Lock()
	newCfg, loadErr := config.Load(configPath)
	if loadErr != nil {
		s.CfgMu.Unlock()
		s.Logger.Error("[Providers] Hot-reload failed", "error", loadErr)
		http.Error(w, "Saved but reload failed: "+loadErr.Error(), http.StatusInternalServerError)
		return
	}
	savedPath := s.Cfg.ConfigPath
	*s.Cfg = *newCfg
	s.Cfg.ConfigPath = savedPath
	// Apply vault secrets and re-resolve providers after hot-reload
	s.Cfg.ApplyVaultSecrets(s.Vault)
	s.Cfg.ResolveProviders()
	s.Cfg.ApplyOAuthTokens(s.Vault)

	// Reconfigure LLM client so model/key/URL changes take effect immediately.
	if fm, ok := s.LLMClient.(*llm.FailoverManager); ok {
		fm.Reconfigure(s.Cfg)
		s.Logger.Info("[Providers] LLM client reconfigured",
			"model", s.Cfg.LLM.Model,
			"provider", s.Cfg.LLM.ProviderType)
	}

	// Capture updated agent info before releasing the lock.
	activeLLMModel := s.Cfg.LLM.Model
	activeLLMProvider := s.Cfg.LLM.ProviderType
	s.CfgMu.Unlock()

	s.Logger.Info("[Providers] Updated", "count", len(entries))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":           "ok",
		"count":            len(entries),
		"active_llm_model": activeLLMModel,
		"active_llm_type":  activeLLMProvider,
	})
}

// handleProviderPricing dispatches GET/POST for /api/providers/pricing?id=<providerID>.
// GET  — fetch available pricing for the provider type (from OpenRouter or local)
// POST — apply the given pricing to the provider's Models list and save config
func handleProviderPricing(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		providerID := r.URL.Query().Get("id")
		if providerID == "" {
			http.Error(w, "Missing 'id' query parameter", http.StatusBadRequest)
			return
		}

		switch r.Method {
		case http.MethodGet:
			handleFetchPricing(s, w, providerID)
		case http.MethodPost:
			handleApplyPricing(s, w, r, providerID)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

// handleFetchPricing returns available model pricing for the given provider.
func handleFetchPricing(s *Server, w http.ResponseWriter, providerID string) {
	s.CfgMu.RLock()
	p := s.Cfg.FindProvider(providerID)
	var providerType, apiKey, baseURL string
	if p != nil {
		providerType = p.Type
		apiKey = p.APIKey
		baseURL = p.BaseURL
		// Workers AI: auto-build URL from account ID if BaseURL is empty.
		if providerType == "workers-ai" && baseURL == "" && p.AccountID != "" {
			baseURL = fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/ai/v1", p.AccountID)
		}
	}
	s.CfgMu.RUnlock()

	if providerType == "" {
		http.Error(w, "Provider not found", http.StatusNotFound)
		return
	}

	pricing, err := llm.FetchPricingForProvider(providerType, apiKey, baseURL)
	if err != nil {
		s.Logger.Error("[Pricing] Failed to fetch pricing", "provider", providerID, "error", err)
		http.Error(w, "Failed to fetch pricing", http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(pricing)
}

// handleApplyPricing writes the given model pricing to the provider's Models list.
func handleApplyPricing(s *Server, w http.ResponseWriter, r *http.Request, providerID string) {
	var incoming []llm.ModelPricing
	if err := json.NewDecoder(r.Body).Decode(&incoming); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	models := llm.ToModelCosts(incoming)

	s.CfgMu.Lock()
	p := s.Cfg.FindProvider(providerID)
	if p == nil {
		s.CfgMu.Unlock()
		http.Error(w, "Provider not found", http.StatusNotFound)
		return
	}
	p.Models = models
	configPath := s.Cfg.ConfigPath
	s.CfgMu.Unlock()

	// Persist to YAML
	if err := persistProviders(s, configPath); err != nil {
		http.Error(w, "Failed to save providers", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "ok",
		"count":  len(models),
	})
}

// persistProviders writes the current in-memory providers to config.yaml.
func persistProviders(s *Server, configPath string) error {
	s.CfgMu.RLock()
	providers := s.Cfg.Providers
	s.CfgMu.RUnlock()

	data, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}
	var rawCfg map[string]interface{}
	if err := yaml.Unmarshal(data, &rawCfg); err != nil {
		return err
	}

	provList := make([]interface{}, len(providers))
	for i, e := range providers {
		m := map[string]interface{}{
			"id":       e.ID,
			"name":     e.Name,
			"type":     e.Type,
			"base_url": e.BaseURL,
			"model":    e.Model,
		}
		if len(e.Models) > 0 {
			// Convert to generic []interface{} for clean YAML output
			ml := make([]interface{}, len(e.Models))
			for j, mc := range e.Models {
				ml[j] = map[string]interface{}{
					"name":               mc.Name,
					"input_per_million":  mc.InputPerMillion,
					"output_per_million": mc.OutputPerMillion,
				}
			}
			m["models"] = ml
		}
		if e.AuthType != "" && e.AuthType != "api_key" {
			m["auth_type"] = e.AuthType
			m["oauth_auth_url"] = e.OAuthAuthURL
			m["oauth_token_url"] = e.OAuthTokenURL
			m["oauth_client_id"] = e.OAuthClientID
			m["oauth_scopes"] = e.OAuthScopes
		}
		provList[i] = m
	}
	rawCfg["providers"] = provList

	out, err := yaml.Marshal(rawCfg)
	if err != nil {
		return err
	}
	return os.WriteFile(configPath, out, 0644)
}
