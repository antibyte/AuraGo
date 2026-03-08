package server

import (
	"aurago/internal/config"
	"encoding/json"
	"net/http"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// providerJSON is the API representation of a provider entry.
type providerJSON struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	Type              string `json:"type"`
	BaseURL           string `json:"base_url"`
	APIKey            string `json:"api_key"`
	Model             string `json:"model"`
	AuthType          string `json:"auth_type"`
	OAuthAuthURL      string `json:"oauth_auth_url"`
	OAuthTokenURL     string `json:"oauth_token_url"`
	OAuthClientID     string `json:"oauth_client_id"`
	OAuthClientSecret string `json:"oauth_client_secret"`
	OAuthScopes       string `json:"oauth_scopes"`
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
			AuthType:          authType,
			OAuthAuthURL:      p.OAuthAuthURL,
			OAuthTokenURL:     p.OAuthTokenURL,
			OAuthClientID:     p.OAuthClientID,
			OAuthClientSecret: clientSecret,
			OAuthScopes:       p.OAuthScopes,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

// handlePutProviders saves a new provider array to config.yaml and hot-reloads.
func handlePutProviders(s *Server, w http.ResponseWriter, r *http.Request) {
	var incoming []providerJSON
	if err := json.NewDecoder(r.Body).Decode(&incoming); err != nil {
		http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
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
			AuthType:          authType,
			OAuthAuthURL:      p.OAuthAuthURL,
			OAuthTokenURL:     p.OAuthTokenURL,
			OAuthClientID:     p.OAuthClientID,
			OAuthClientSecret: clientSecret,
			OAuthScopes:       p.OAuthScopes,
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
	s.CfgMu.Unlock()

	s.Logger.Info("[Providers] Updated", "count", len(entries))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "ok",
		"count":  len(entries),
	})
}
