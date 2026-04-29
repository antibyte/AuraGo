package server

import (
	"aurago/internal/agent"
	"aurago/internal/config"
	"aurago/internal/llm"
	"aurago/internal/security"
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

// copyFromPrefix is a sentinel prefix sent by the UI when the user selects an
// existing provider's key to copy.  Format: "__copy_from__<sourceProviderID>".
// The prefix is resolved in the PUT handler and never persisted.
const copyFromPrefix = "__copy_from__"

func normalizeProviderAuthType(authType string) string {
	normalized := strings.ToLower(strings.TrimSpace(authType))
	if normalized == "" {
		return "api_key"
	}
	return normalized
}

type vaultMutation struct {
	key    string
	value  string
	delete bool
}

type vaultSnapshot struct {
	exists bool
	value  string
}

func snapshotVaultSecrets(vault *security.Vault, keys []string) map[string]vaultSnapshot {
	snapshots := make(map[string]vaultSnapshot, len(keys))
	if vault == nil {
		return snapshots
	}
	for _, key := range keys {
		if _, seen := snapshots[key]; seen || strings.TrimSpace(key) == "" {
			continue
		}
		value, err := vault.ReadSecret(key)
		if err != nil {
			snapshots[key] = vaultSnapshot{}
			continue
		}
		snapshots[key] = vaultSnapshot{exists: true, value: value}
	}
	return snapshots
}

func restoreVaultSecrets(vault *security.Vault, snapshots map[string]vaultSnapshot) error {
	if vault == nil {
		return nil
	}
	for key, snapshot := range snapshots {
		if snapshot.exists {
			if err := vault.WriteSecret(key, snapshot.value); err != nil {
				return fmt.Errorf("restore vault secret %s: %w", key, err)
			}
			continue
		}
		if err := vault.DeleteSecret(key); err != nil {
			return fmt.Errorf("delete restored vault secret %s: %w", key, err)
		}
	}
	return nil
}

func applyVaultMutations(vault *security.Vault, mutations []vaultMutation) (map[string]vaultSnapshot, error) {
	if vault == nil || len(mutations) == 0 {
		return nil, nil
	}

	keys := make([]string, 0, len(mutations))
	for _, mutation := range mutations {
		keys = append(keys, mutation.key)
	}
	snapshots := snapshotVaultSecrets(vault, keys)
	for _, mutation := range mutations {
		var err error
		if mutation.delete {
			err = vault.DeleteSecret(mutation.key)
		} else {
			err = vault.WriteSecret(mutation.key, mutation.value)
		}
		if err != nil {
			_ = restoreVaultSecrets(vault, snapshots)
			return nil, err
		}
	}
	return snapshots, nil
}

// handleProviders dispatches GET / PUT for /api/providers.
func handleProviders(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handleGetProviders(s, w, r)
		case http.MethodPut:
			handlePutProviders(s, w, r)
		default:
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
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
		authType := normalizeProviderAuthType(p.AuthType)
		apiKey := ""
		if authType != "oauth2" && p.APIKey != "" {
			apiKey = maskedKey
		}
		clientSecret := ""
		if authType == "oauth2" && p.OAuthClientSecret != "" {
			clientSecret = maskedKey
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
		jsonError(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Build id → old secret maps so masked keys are preserved
	s.CfgMu.RLock()
	oldKeyMap := make(map[string]string, len(s.Cfg.Providers))
	oldSecretMap := make(map[string]string, len(s.Cfg.Providers))
	oldAuthTypeMap := make(map[string]string, len(s.Cfg.Providers))
	oldProviderIDs := make([]string, len(s.Cfg.Providers))
	for i, p := range s.Cfg.Providers {
		oldAuthTypeMap[p.ID] = normalizeProviderAuthType(p.AuthType)
		if normalizeProviderAuthType(p.AuthType) != "oauth2" {
			oldKeyMap[p.ID] = p.APIKey
		}
		oldSecretMap[p.ID] = p.OAuthClientSecret
		oldProviderIDs[i] = p.ID
	}
	configPath := s.Cfg.ConfigPath
	s.CfgMu.RUnlock()
	if s.Vault != nil {
		for _, id := range oldProviderIDs {
			if secret, err := s.Vault.ReadSecret("provider_" + id + "_api_key"); err == nil {
				oldKeyMap[id] = secret
			}
			if secret, err := s.Vault.ReadSecret("provider_" + id + "_oauth_client_secret"); err == nil {
				oldSecretMap[id] = secret
			}
		}
	}

	if configPath == "" {
		jsonError(w, "Config path not set", http.StatusInternalServerError)
		return
	}

	// Convert to ProviderEntry slice and stage vault mutations; secrets stay in the vault, not YAML.
	entries := make([]config.ProviderEntry, len(incoming))
	seenIDs := make(map[string]bool, len(incoming))
	vaultMutations := make([]vaultMutation, 0, len(incoming)*3)
	for i, p := range incoming {
		p.ID = strings.TrimSpace(p.ID)
		if p.ID == "" {
			jsonError(w, "Provider ID must not be empty", http.StatusBadRequest)
			return
		}
		if seenIDs[p.ID] {
			jsonError(w, "Duplicate provider ID: "+p.ID, http.StatusBadRequest)
			return
		}
		seenIDs[p.ID] = true

		authType := normalizeProviderAuthType(p.AuthType)

		// ── API Key → vault ──
		apiKey := p.APIKey
		if authType == "oauth2" {
			apiKey = ""
		} else {
			if apiKey == maskedKey {
				// Unchanged — keep existing vault value
				if old, ok := oldKeyMap[p.ID]; ok {
					apiKey = old
				}
			} else if strings.HasPrefix(apiKey, copyFromPrefix) {
				// User selected "copy from existing provider" in the UI.
				sourceID := strings.TrimPrefix(apiKey, copyFromPrefix)
				if s.Vault != nil {
					copied, err := s.Vault.ReadSecret("provider_" + sourceID + "_api_key")
					if err != nil || copied == "" {
						s.Logger.Warn("[Providers] Copy-from source key not found in vault",
							"source_id", sourceID, "target_id", p.ID)
						apiKey = ""
					} else {
						s.Logger.Info("[Providers] Copied API key from source provider",
							"source_id", sourceID, "target_id", p.ID)
						apiKey = copied
					}
				} else {
					apiKey = ""
				}
			}
		}

		// ── OAuth Client Secret → vault ──
		clientSecret := p.OAuthClientSecret
		if authType == "oauth2" {
			if clientSecret == maskedKey {
				if old, ok := oldSecretMap[p.ID]; ok {
					clientSecret = old
				}
			}
		} else {
			clientSecret = ""
		}

		if s.Vault != nil {
			apiKeyVaultID := "provider_" + p.ID + "_api_key"
			clientSecretVaultID := "provider_" + p.ID + "_oauth_client_secret"
			oauthTokenVaultID := "oauth_" + p.ID

			if authType == "oauth2" {
				if oldAuthTypeMap[p.ID] != "oauth2" {
					vaultMutations = append(vaultMutations, vaultMutation{key: apiKeyVaultID, delete: true})
				}
				if strings.TrimSpace(clientSecret) == "" {
					vaultMutations = append(vaultMutations, vaultMutation{key: clientSecretVaultID, delete: true})
				} else {
					vaultMutations = append(vaultMutations, vaultMutation{key: clientSecretVaultID, value: clientSecret})
				}
			} else {
				vaultMutations = append(vaultMutations,
					vaultMutation{key: clientSecretVaultID, delete: true},
					vaultMutation{key: oauthTokenVaultID, delete: true},
				)
				if strings.TrimSpace(apiKey) == "" {
					vaultMutations = append(vaultMutations, vaultMutation{key: apiKeyVaultID, delete: true})
				} else {
					vaultMutations = append(vaultMutations, vaultMutation{key: apiKeyVaultID, value: apiKey})
				}
			}
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

	// Clean up vault secrets for providers that were removed.
	if s.Vault != nil {
		newIDSet := make(map[string]bool, len(entries))
		for _, e := range entries {
			newIDSet[e.ID] = true
		}
		for _, oldID := range oldProviderIDs {
			if !newIDSet[oldID] {
				vaultMutations = append(vaultMutations,
					vaultMutation{key: "provider_" + oldID + "_api_key", delete: true},
					vaultMutation{key: "provider_" + oldID + "_oauth_client_secret", delete: true},
					vaultMutation{key: "oauth_" + oldID, delete: true},
				)
				s.Logger.Info("[Providers] Cleaned up vault secrets for removed provider", "id", oldID)
			}
		}
	}

	// Read raw YAML, update providers key, write back
	data, err := os.ReadFile(configPath)
	if err != nil {
		s.Logger.Error("Failed to read config for provider update", "error", err)
		jsonError(w, "Failed to read config", http.StatusInternalServerError)
		return
	}

	var rawCfg map[string]interface{}
	if err := yaml.Unmarshal(data, &rawCfg); err != nil {
		s.Logger.Error("Failed to parse config for provider update", "error", err)
		jsonError(w, "Failed to parse config", http.StatusInternalServerError)
		return
	}

	// Build providers as []interface{} for YAML marshal (secrets excluded)
	provList := make([]interface{}, len(entries))
	for i, e := range entries {
		provList[i] = buildProviderYAMLEntry(e)
	}
	rawCfg["providers"] = provList

	out, err := yaml.Marshal(rawCfg)
	if err != nil {
		s.Logger.Error("Failed to marshal config after provider update", "error", err)
		jsonError(w, "Failed to save config", http.StatusInternalServerError)
		return
	}

	if err := config.WriteFileAtomic(configPath, out, 0o600); err != nil {
		s.Logger.Error("Failed to write config after provider update", "error", err)
		jsonError(w, "Failed to write config", http.StatusInternalServerError)
		return
	}

	vaultSnapshots, err := applyVaultMutations(s.Vault, vaultMutations)
	if err != nil {
		_ = config.WriteFileAtomic(configPath, data, 0o600)
		s.Logger.Error("[Providers] Failed to update vault after config write", "error", err)
		jsonError(w, "Failed to update provider secrets", http.StatusInternalServerError)
		return
	}

	// Hot-reload
	s.CfgMu.Lock()
	newCfg, loadErr := config.Load(configPath)
	if loadErr != nil {
		s.CfgMu.Unlock()
		_ = config.WriteFileAtomic(configPath, data, 0o600)
		if restoreErr := restoreVaultSecrets(s.Vault, vaultSnapshots); restoreErr != nil {
			s.Logger.Error("[Providers] Failed to restore vault after hot-reload failure", "error", restoreErr)
		}
		s.Logger.Error("[Providers] Hot-reload failed", "error", loadErr)
		jsonError(w, "Saved but reload failed: "+loadErr.Error(), http.StatusInternalServerError)
		return
	}
	// Apply vault secrets and re-resolve providers after hot-reload
	newCfg.ConfigPath = configPath
	newCfg.ApplyVaultSecrets(s.Vault)
	newCfg.ResolveProviders()
	newCfg.ApplyOAuthTokens(s.Vault)
	s.replaceConfigSnapshot(newCfg)

	// Reconfigure LLM client so model/key/URL changes take effect immediately.
	if fm, ok := s.LLMClient.(*llm.FailoverManager); ok {
		fm.Reconfigure(newCfg)
		s.Logger.Info("[Providers] LLM client reconfigured",
			"model", newCfg.LLM.Model,
			"provider", newCfg.LLM.ProviderType)
	}
	// Recreate LLMGuardian so its client uses the updated API keys.
	s.LLMGuardian = security.NewLLMGuardian(newCfg, s.Logger)

	// Capture updated agent info before releasing the lock.
	activeLLMModel := newCfg.LLM.Model
	activeLLMProvider := newCfg.LLM.ProviderType
	s.CfgMu.Unlock()

	// Reset the global Helper-LLM singleton so its next request picks up the
	// new configuration (updated API keys, model, etc.).
	agent.ResetGlobalHelperLLMManager()

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
			jsonError(w, "Missing 'id' query parameter", http.StatusBadRequest)
			return
		}

		switch r.Method {
		case http.MethodGet:
			handleFetchPricing(s, w, providerID)
		case http.MethodPost:
			handleApplyPricing(s, w, r, providerID)
		default:
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
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
		jsonError(w, "Provider not found", http.StatusNotFound)
		return
	}

	pricing, err := llm.FetchPricingForProvider(providerType, apiKey, baseURL)
	if err != nil {
		s.Logger.Error("[Pricing] Failed to fetch pricing", "provider", providerID, "error", err)
		jsonError(w, "Failed to fetch pricing", http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(pricing)
}

// handleApplyPricing writes the given model pricing to the provider's Models list.
func handleApplyPricing(s *Server, w http.ResponseWriter, r *http.Request, providerID string) {
	var incoming []llm.ModelPricing
	if err := json.NewDecoder(r.Body).Decode(&incoming); err != nil {
		jsonError(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	models := llm.ToModelCosts(incoming)

	s.CfgMu.Lock()
	p := s.Cfg.FindProvider(providerID)
	if p == nil {
		s.CfgMu.Unlock()
		jsonError(w, "Provider not found", http.StatusNotFound)
		return
	}
	p.Models = models
	configPath := s.Cfg.ConfigPath
	s.CfgMu.Unlock()

	// Persist to YAML
	if err := persistProviders(s, configPath); err != nil {
		jsonError(w, "Failed to save providers", http.StatusInternalServerError)
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
		provList[i] = buildProviderYAMLEntry(e)
	}
	rawCfg["providers"] = provList

	out, err := yaml.Marshal(rawCfg)
	if err != nil {
		return err
	}
	return config.WriteFileAtomic(configPath, out, 0o600)
}

// buildProviderYAMLEntry converts a ProviderEntry to a map suitable for YAML marshalling.
// Secrets (API keys, OAuth tokens) are intentionally excluded — they live in the vault.
func buildProviderYAMLEntry(e config.ProviderEntry) map[string]interface{} {
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
	return m
}
