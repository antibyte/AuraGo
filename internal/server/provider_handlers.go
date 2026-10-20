package server

import (
	"aurago/internal/agent"
	"aurago/internal/config"
	"aurago/internal/llm"
	"aurago/internal/security"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"reflect"
	"strings"
)

// providerJSON is the API representation of a provider entry.
type providerJSON struct {
	ID                        string                     `json:"id"`
	Name                      string                     `json:"name"`
	Usage                     string                     `json:"usage,omitempty"`
	Type                      string                     `json:"type"`
	BaseURL                   string                     `json:"base_url"`
	APIKey                    string                     `json:"api_key"`
	Model                     string                     `json:"model"`
	ContextWindow             int                        `json:"context_window"`
	MaxOutputTokens           int                        `json:"max_output_tokens"`
	EffectiveContextWindow    int                        `json:"effective_context_window"`
	EffectiveMaxOutputTokens  int                        `json:"effective_max_output_tokens"`
	ContextWindowSource       string                     `json:"context_window_source"`
	MaxOutputTokensSource     string                     `json:"max_output_tokens_source"`
	ModelLimitsWarning        string                     `json:"model_limits_warning,omitempty"`
	ModelLimitsProbeStatus    string                     `json:"model_limits_probe_status,omitempty"`
	ModelLimitsMetadataSource string                     `json:"model_limits_metadata_source,omitempty"`
	ContextCapApplied         bool                       `json:"context_cap_applied,omitempty"`
	AccountID                 string                     `json:"account_id"`
	GatewayID                 string                     `json:"gateway_id,omitempty"`
	AuthType                  string                     `json:"auth_type"`
	OAuthAuthURL              string                     `json:"oauth_auth_url"`
	OAuthTokenURL             string                     `json:"oauth_token_url"`
	OAuthClientID             string                     `json:"oauth_client_id"`
	OAuthClientSecret         string                     `json:"oauth_client_secret"`
	OAuthScopes               string                     `json:"oauth_scopes"`
	Models                    []config.ModelCost         `json:"models,omitempty"`
	Capabilities              *providerCapabilitiesJSON  `json:"capabilities,omitempty"`
	EffectiveCapabilities     providerCapabilitiesJSON   `json:"effective_capabilities,omitempty"`
	References                []providerReferencePayload `json:"references,omitempty"`
	RuntimeChat               speechLabRuntimeChatStatus `json:"runtime_chat"`
}

type providerCapabilitiesJSON struct {
	Auto              bool   `json:"auto"`
	ToolCalling       bool   `json:"tool_calling"`
	StructuredOutputs bool   `json:"structured_outputs"`
	Multimodal        bool   `json:"multimodal"`
	DetectedModel     string `json:"detected_model,omitempty"`
	Source            string `json:"source,omitempty"`
	Known             bool   `json:"known,omitempty"`
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
	if normalized == "oauth" {
		return "oauth2"
	}
	return normalized
}

func boolPtr(value bool) *bool {
	return &value
}

func providerCapabilitiesToJSON(c config.ProviderCapabilities) providerCapabilitiesJSON {
	return providerCapabilitiesJSON{
		Auto:              c.AutoEnabled(),
		ToolCalling:       c.ToolCalling,
		StructuredOutputs: c.StructuredOutputs,
		Multimodal:        c.Multimodal,
		DetectedModel:     c.DetectedModel,
		Source:            c.Source,
	}
}

func providerCapabilitiesFromJSON(c *providerCapabilitiesJSON) config.ProviderCapabilities {
	if c == nil {
		return config.ProviderCapabilities{}
	}
	return config.ProviderCapabilities{
		Auto:              boolPtr(c.Auto),
		ToolCalling:       c.ToolCalling,
		StructuredOutputs: c.StructuredOutputs,
		Multimodal:        c.Multimodal,
		DetectedModel:     c.DetectedModel,
		Source:            c.Source,
	}
}

func providerCapabilitiesResultToJSON(c llm.ProviderCapabilityResult) providerCapabilitiesJSON {
	return providerCapabilitiesJSON{
		Auto:              c.Source != llm.CapabilitySourceManual,
		ToolCalling:       c.ToolCalling,
		StructuredOutputs: c.StructuredOutputs,
		Multimodal:        c.Multimodal,
		DetectedModel:     c.DetectedModel,
		Source:            c.Source,
		Known:             c.Known,
	}
}

func providerCapabilitiesConfigured(c config.ProviderCapabilities) bool {
	return c.Auto != nil ||
		c.ToolCalling ||
		c.StructuredOutputs ||
		c.Multimodal ||
		strings.TrimSpace(c.DetectedModel) != "" ||
		strings.TrimSpace(c.Source) != ""
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

type providerVaultStore interface {
	ReadSecret(string) (string, error)
	WriteSecret(string, string) error
	DeleteSecret(string) error
}

func providerVaultMissing(vault providerVaultStore) bool {
	return vault == nil || reflect.ValueOf(vault).Kind() == reflect.Pointer && reflect.ValueOf(vault).IsNil()
}

func snapshotVaultSecrets(vault providerVaultStore, keys []string) map[string]vaultSnapshot {
	snapshots := make(map[string]vaultSnapshot, len(keys))
	if providerVaultMissing(vault) {
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

func restoreVaultSecrets(vault providerVaultStore, snapshots map[string]vaultSnapshot) error {
	if providerVaultMissing(vault) {
		return nil
	}
	var failures []error
	for key, snapshot := range snapshots {
		var err error
		if snapshot.exists {
			err = vault.WriteSecret(key, snapshot.value)
		} else {
			err = vault.DeleteSecret(key)
		}
		if err != nil {
			failures = append(failures, fmt.Errorf("restore vault secret %s: %w", key, err))
		}
	}
	return errors.Join(failures...)
}

func applyVaultMutations(vault providerVaultStore, mutations []vaultMutation) (map[string]vaultSnapshot, error) {
	if providerVaultMissing(vault) || len(mutations) == 0 {
		return nil, nil
	}

	keys := make([]string, 0, len(mutations))
	for _, mutation := range mutations {
		keys = append(keys, mutation.key)
	}
	snapshots := make(map[string]vaultSnapshot, len(keys))
	for _, key := range keys {
		if _, exists := snapshots[key]; exists {
			continue
		}
		value, err := vault.ReadSecret(key)
		if err != nil && !errors.Is(err, security.ErrSecretNotFound) {
			return nil, fmt.Errorf("snapshot provider credential: %w", err)
		}
		snapshots[key] = vaultSnapshot{exists: err == nil, value: value}
	}
	applied := make(map[string]vaultSnapshot)
	for _, mutation := range mutations {
		before := snapshots[mutation.key]
		if mutation.delete && !before.exists || !mutation.delete && before.exists && before.value == mutation.value {
			continue
		}
		applied[mutation.key] = before
		var err error
		if mutation.delete {
			err = vault.DeleteSecret(mutation.key)
		} else {
			err = vault.WriteSecret(mutation.key, mutation.value)
		}
		if err != nil {
			rollback := restoreVaultSecrets(vault, applied)
			if rollback != nil {
				return applied, errors.Join(err, rollback, &providerWriteError{status: 500, code: "provider_rollback_failed", message: "Provider credential rollback was incomplete; inspect the server log"})
			}
			return applied, err
		}
	}
	return applied, nil
}

type providerSaveResult struct {
	Count             int
	ActiveLLMModel    string
	ActiveLLMProvider string
	Revision          string
	config            *config.Config
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
func handleGetProviders(s *Server, w http.ResponseWriter, r *http.Request) {
	s.CfgMu.RLock()
	cfgSnapshot := *s.Cfg
	revision := s.providerETag()
	providers := append([]config.ProviderEntry(nil), s.Cfg.Providers...)
	fallback := llm.CapabilityFallback{
		ToolCalling:       s.Cfg.LLM.UseNativeFunctions,
		StructuredOutputs: s.Cfg.LLM.StructuredOutputs,
		Multimodal:        s.Cfg.LLM.Multimodal,
	}
	s.CfgMu.RUnlock()

	// The config read path is cache-only. Missing provider metadata is refreshed
	// asynchronously so an unavailable endpoint cannot delay the config UI.
	limitResults := make([]llm.ModelLimits, len(providers))
	for i, provider := range providers {
		route := llm.ModelRoute{
			ProviderID:              provider.ID,
			ProviderType:            provider.Type,
			BaseURL:                 provider.BaseURL,
			APIKey:                  provider.RuntimeAPIKey(),
			Model:                   provider.Model,
			ContextWindowOverride:   provider.ContextWindow,
			MaxOutputTokensOverride: provider.MaxOutputTokens,
			Primary:                 provider.ID == cfgSnapshot.LLM.Provider,
		}
		limitResults[i] = llm.ResolveModelLimitsCached(route, cfgSnapshot.Agent.ContextWindow)
		if limitResults[i].ProbeStatus == "pending" {
			llm.ScheduleModelLimitRefresh(route, s.Logger)
		}
	}

	out := make([]providerJSON, len(providers))
	for i, p := range providers {
		limits := limitResults[i]
		authType := normalizeProviderAuthType(p.AuthType)
		apiKey := ""
		if p.APIKey != "" {
			apiKey = maskedKey
		}
		clientSecret := ""
		if p.OAuthClientSecret != "" {
			clientSecret = maskedKey
		}
		runtimeChat, _ := speechLabRuntimeChatProvider(&p, s.Vault)
		out[i] = providerJSON{
			ID:                        p.ID,
			Name:                      p.Name,
			Usage:                     p.Usage,
			Type:                      p.Type,
			BaseURL:                   p.BaseURL,
			APIKey:                    apiKey,
			Model:                     p.Model,
			ContextWindow:             p.ContextWindow,
			MaxOutputTokens:           p.MaxOutputTokens,
			EffectiveContextWindow:    limits.ContextWindow,
			EffectiveMaxOutputTokens:  limits.MaxOutputTokens,
			ContextWindowSource:       limits.ContextSource,
			MaxOutputTokensSource:     limits.OutputSource,
			ModelLimitsProbeStatus:    limits.ProbeStatus,
			ModelLimitsMetadataSource: limits.MetadataSource,
			ContextCapApplied:         limits.ContextCapApplied,
			AccountID:                 p.AccountID,
			GatewayID:                 p.GatewayID,
			AuthType:                  authType,
			OAuthAuthURL:              p.OAuthAuthURL,
			OAuthTokenURL:             p.OAuthTokenURL,
			OAuthClientID:             p.OAuthClientID,
			OAuthClientSecret:         clientSecret,
			OAuthScopes:               p.OAuthScopes,
			Models:                    p.Models,
			Capabilities:              &providerCapabilitiesJSON{},
			EffectiveCapabilities:     providerCapabilitiesResultToJSON(llm.ResolveProviderCapabilities(p, fallback)),
			References:                providerReferences(&cfgSnapshot, p.ID),
			RuntimeChat:               runtimeChat,
		}
		if limits.Unknown {
			out[i].ModelLimitsWarning = "unknown_model_conservative_limits"
		} else if limits.ContextCapApplied {
			out[i].ModelLimitsWarning = "context_cap_applied"
		}
		caps := providerCapabilitiesToJSON(p.Capabilities)
		out[i].Capabilities = &caps
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("ETag", revision)
	json.NewEncoder(w).Encode(out)
}

// handlePutProviders saves a new provider array to config.yaml and hot-reloads.
func handlePutProviders(s *Server, w http.ResponseWriter, r *http.Request) {
	var incoming []providerJSON
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 10<<20))
	if err := decoder.Decode(&incoming); err != nil || decoder.Decode(&struct{}{}) != io.EOF {
		jsonError(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	result, err := persistProviderEntries(s, r.Header.Get("If-Match"), func(current *config.Config) ([]config.ProviderEntry, []vaultMutation, error) {
		return providerEntriesFromJSON(s, current, incoming)
	})
	if err != nil {
		if s.Logger != nil {
			s.Logger.Warn("[Providers] Save rejected", "error", err)
		}
		writeProviderError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("ETag", result.Revision)
	_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "count": result.Count, "active_llm_model": result.ActiveLLMModel, "active_llm_type": result.ActiveLLMProvider, "revision": result.Revision})
}

func persistProviderEntries(s *Server, expectedRevision string, mutate providerMutation) (providerSaveResult, error) {
	if s == nil {
		return providerSaveResult{}, fmt.Errorf("server config is not available")
	}
	s.CfgSaveMu.Lock()
	defer s.CfgSaveMu.Unlock()
	s.CfgMu.Lock()
	defer s.CfgMu.Unlock()
	if err := s.checkProviderRevision(expectedRevision); err != nil {
		return providerSaveResult{}, err
	}
	if s.Cfg == nil {
		return providerSaveResult{}, fmt.Errorf("server config is not available")
	}
	current := s.Cfg.Clone()
	candidate := current.Clone()
	entries, mutations, err := mutate(candidate)
	if err != nil {
		return providerSaveResult{}, err
	}
	candidate.Providers = entries
	if err = validateProviderEntries(current, candidate); err != nil {
		return providerSaveResult{}, err
	}
	keep := map[string]bool{}
	for _, p := range entries {
		keep[p.ID] = true
	}
	for _, old := range current.Providers {
		if !keep[old.ID] {
			for _, key := range []string{"provider_" + old.ID + "_api_key", "provider_" + old.ID + "_oauth_client_secret", "oauth_" + old.ID} {
				mutations = append(mutations, vaultMutation{key: key, delete: true})
			}
		}
	}
	configPath := current.ConfigPath
	if strings.TrimSpace(configPath) == "" {
		return providerSaveResult{}, fmt.Errorf("config path not set")
	}
	before, err := os.ReadFile(configPath)
	if err != nil {
		return providerSaveResult{}, fmt.Errorf("read provider config: %w", err)
	}
	out, err := marshalConfigWithProviderEntries(before, entries)
	if err != nil {
		return providerSaveResult{}, err
	}
	newCfg, err := saveProviderConfig(s, configPath, before, out, mutations)
	if err != nil {
		return providerSaveResult{}, err
	}
	newCfg.Runtime = current.Runtime
	s.replaceConfigSnapshot(newCfg)
	if s.BudgetTracker != nil {
		s.BudgetTracker.UpdateConfig(newCfg)
	}
	llm.InvalidateModelLimitCache()
	transportChanged := !reflect.DeepEqual(current.LLM, newCfg.LLM) || !reflect.DeepEqual(current.FallbackLLM, newCfg.FallbackLLM) || !reflect.DeepEqual(current.AIGateway, newCfg.AIGateway) || !reflect.DeepEqual(current.Manifest, newCfg.Manifest) || !reflect.DeepEqual(current.CircuitBreaker, newCfg.CircuitBreaker)
	if fm, ok := s.LLMClient.(*llm.FailoverManager); ok && transportChanged {
		fm.Reconfigure(newCfg)
	}
	s.LLMGuardian = security.NewLLMGuardian(newCfg, s.Logger)
	agent.ResetGlobalHelperLLMManager()
	if s.Logger != nil {
		s.Logger.Info("[Providers] Settings saved", "count", len(entries))
	}
	return providerSaveResult{Count: len(entries), ActiveLLMModel: newCfg.LLM.Model, ActiveLLMProvider: newCfg.LLM.ProviderType, Revision: s.providerETag(), config: newCfg}, nil
}

// handleProviderCapabilities detects capability flags for a provider/model pair.
func handleProviderCapabilities(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		providerID := strings.TrimSpace(r.URL.Query().Get("id"))
		providerType := strings.TrimSpace(r.URL.Query().Get("type"))
		model := strings.TrimSpace(r.URL.Query().Get("model"))

		s.CfgMu.RLock()
		fallback := llm.CapabilityFallback{
			ToolCalling:       s.Cfg.LLM.UseNativeFunctions,
			StructuredOutputs: s.Cfg.LLM.StructuredOutputs,
			Multimodal:        s.Cfg.LLM.Multimodal,
		}
		var provider config.ProviderEntry
		if providerID != "" {
			if p := s.Cfg.FindProvider(providerID); p != nil {
				provider = *p
			}
		}
		s.CfgMu.RUnlock()

		if providerType != "" {
			provider.Type = providerType
		}
		if model != "" {
			provider.Model = model
		}
		if strings.TrimSpace(provider.Type) == "" && strings.TrimSpace(provider.Model) == "" {
			jsonError(w, "Missing provider type or model", http.StatusBadRequest)
			return
		}

		var caps llm.ProviderCapabilityResult
		var ok bool
		if strings.EqualFold(provider.Type, "openrouter") {
			var err error
			caps, ok, err = llm.FetchOpenRouterModelCapabilities(provider.Model)
			if err != nil {
				s.Logger.Warn("[Providers] OpenRouter capability lookup failed", "model", provider.Model, "error", err)
			}
		}
		if !ok {
			provider.Capabilities = config.ProviderCapabilities{}
			caps = llm.ResolveProviderCapabilities(provider, fallback)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(providerCapabilitiesResultToJSON(caps))
	}
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
		apiKey = p.RuntimeAPIKey()
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
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 10<<20))
	if err := decoder.Decode(&incoming); err != nil || decoder.Decode(&struct{}{}) != io.EOF {
		jsonError(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	models := llm.ToModelCosts(incoming)
	result, err := persistProviderEntries(s, r.Header.Get("If-Match"), func(current *config.Config) ([]config.ProviderEntry, []vaultMutation, error) {
		p := current.FindProvider(providerID)
		if p == nil {
			return nil, nil, &providerWriteError{status: http.StatusNotFound, code: "provider_not_found", message: "Provider not found"}
		}
		p.Models = models
		return current.Providers, nil, nil
	})
	if err != nil {
		writeProviderError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("ETag", result.Revision)
	_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "count": len(models), "revision": result.Revision})
}

// buildProviderYAMLEntry converts a ProviderEntry to a map suitable for YAML marshalling.
// Secrets (API keys, OAuth tokens) are intentionally excluded — they live in the vault.
func buildProviderYAMLEntry(e config.ProviderEntry) map[string]interface{} {
	m := map[string]interface{}{
		"id":       e.ID,
		"name":     e.Name,
		"usage":    e.Usage,
		"type":     e.Type,
		"base_url": e.BaseURL,
		"model":    e.Model,
	}
	if e.ContextWindow > 0 {
		m["context_window"] = e.ContextWindow
	}
	if e.MaxOutputTokens > 0 {
		m["max_output_tokens"] = e.MaxOutputTokens
	}
	if e.AccountID != "" {
		m["account_id"] = e.AccountID
	}
	if e.GatewayID != "" {
		m["gateway_id"] = e.GatewayID
	}
	if len(e.Models) > 0 {
		ml := make([]interface{}, len(e.Models))
		for j, mc := range e.Models {
			ml[j] = map[string]interface{}{
				"name":               mc.Name,
				"input_per_million":  mc.InputPerMillion,
				"output_per_million": mc.OutputPerMillion,
			}
			if mc.Source != "" {
				ml[j].(map[string]interface{})["source"] = mc.Source
			}
			if mc.Estimated {
				ml[j].(map[string]interface{})["estimated"] = true
			}
		}
		m["models"] = ml
	}
	if providerCapabilitiesConfigured(e.Capabilities) {
		caps := map[string]interface{}{
			"auto":               e.Capabilities.AutoEnabled(),
			"tool_calling":       e.Capabilities.ToolCalling,
			"structured_outputs": e.Capabilities.StructuredOutputs,
			"multimodal":         e.Capabilities.Multimodal,
		}
		if e.Capabilities.DetectedModel != "" {
			caps["detected_model"] = e.Capabilities.DetectedModel
		}
		if e.Capabilities.Source != "" {
			caps["source"] = e.Capabilities.Source
		}
		m["capabilities"] = caps
	}
	if e.AuthType != "" && e.AuthType != "api_key" {
		m["auth_type"] = e.AuthType
	}
	if e.AuthType == "oauth2" || e.OAuthAuthURL != "" || e.OAuthTokenURL != "" || e.OAuthClientID != "" || e.OAuthScopes != "" {
		m["oauth_auth_url"] = e.OAuthAuthURL
		m["oauth_token_url"] = e.OAuthTokenURL
		m["oauth_client_id"] = e.OAuthClientID
		m["oauth_scopes"] = e.OAuthScopes
	}
	return m
}
