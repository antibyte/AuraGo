package server

import (
	"encoding/json"
	"net/http"
	"strings"

	"aurago/internal/config"
	"aurago/internal/llm/catalog"
)

type modelCatalogResponse struct {
	Enabled   bool                           `json:"enabled"`
	Metadata  catalog.Metadata               `json:"metadata"`
	Providers []modelCatalogProviderResponse `json:"providers"`
	Models    []modelCatalogModelResponse    `json:"models"`
}

type modelCatalogProviderResponse struct {
	ID                         string              `json:"id"`
	AuraProviderType           string              `json:"aura_provider_type"`
	Name                       string              `json:"name,omitempty"`
	DefaultModel               string              `json:"default_model,omitempty"`
	EnvVars                    []string            `json:"env_vars,omitempty"`
	OAuthProvider              string              `json:"oauth_provider,omitempty"`
	OAuthSetup                 *catalog.OAuthSetup `json:"oauth_setup,omitempty"`
	AllowUnauthenticated       bool                `json:"allow_unauthenticated"`
	DynamicModelsAuthoritative bool                `json:"dynamic_models_authoritative"`
	CatalogOnly                bool                `json:"catalog_only"`
	Available                  bool                `json:"available"`
	Availability               string              `json:"availability"`
	ModelsCount                int                 `json:"models_count"`
}

type modelCatalogModelResponse struct {
	ID            string       `json:"id"`
	Provider      string       `json:"provider"`
	Name          string       `json:"name,omitempty"`
	API           string       `json:"api,omitempty"`
	BaseURL       string       `json:"base_url,omitempty"`
	ContextWindow int          `json:"context_window,omitempty"`
	MaxTokens     int          `json:"max_tokens,omitempty"`
	Capabilities  modelCaps    `json:"capabilities"`
	Cost          catalog.Cost `json:"cost"`
	CatalogOnly   bool         `json:"catalog_only"`
}

type modelCaps struct {
	ToolCalling       bool `json:"tool_calling"`
	StructuredOutputs bool `json:"structured_outputs"`
	Multimodal        bool `json:"multimodal"`
	Reasoning         bool `json:"reasoning"`
}

func handleModelCatalog(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		snapshot, err := catalog.Load()
		if err != nil {
			jsonError(w, "Model catalog unavailable", http.StatusInternalServerError)
			return
		}
		cfg := s.Cfg
		enabled := cfg == nil || cfg.ModelCatalog.Enabled
		response := modelCatalogResponse{
			Enabled:  enabled,
			Metadata: snapshot.Metadata,
		}
		if !enabled {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(response)
			return
		}

		disabled := disabledCatalogProviders(cfg)
		modelCounts := map[string]int{}
		for _, model := range snapshot.Models {
			modelCounts[model.Provider]++
		}
		for _, provider := range snapshot.Providers {
			if provider.CatalogOnly && cfg != nil && !cfg.ModelCatalog.CatalogOnlyVisible {
				continue
			}
			available, availability := catalogProviderAvailability(cfg, provider, disabled)
			response.Providers = append(response.Providers, modelCatalogProviderResponse{
				ID:                         provider.ID,
				AuraProviderType:           provider.AuraProviderType,
				Name:                       provider.Name,
				DefaultModel:               provider.DefaultModel,
				EnvVars:                    append([]string(nil), provider.EnvVars...),
				OAuthProvider:              provider.OAuthProvider,
				OAuthSetup:                 provider.OAuthSetup,
				AllowUnauthenticated:       provider.AllowUnauthenticated,
				DynamicModelsAuthoritative: provider.DynamicModelsAuthoritative,
				CatalogOnly:                provider.CatalogOnly,
				Available:                  available,
				Availability:               availability,
				ModelsCount:                modelCounts[provider.AuraProviderType],
			})
		}
		for _, model := range snapshot.Models {
			provider, _ := snapshot.FindProvider(model.Provider)
			if provider.CatalogOnly && cfg != nil && !cfg.ModelCatalog.CatalogOnlyVisible {
				continue
			}
			response.Models = append(response.Models, modelCatalogModelResponse{
				ID:            model.ID,
				Provider:      model.Provider,
				Name:          model.Name,
				API:           model.API,
				BaseURL:       model.BaseURL,
				ContextWindow: model.ContextWindow,
				MaxTokens:     model.MaxTokens,
				Capabilities: modelCaps{
					ToolCalling:       model.SupportsTools,
					StructuredOutputs: model.StructuredOutputs,
					Multimodal:        model.Multimodal,
					Reasoning:         model.Reasoning,
				},
				Cost:        model.Cost,
				CatalogOnly: provider.CatalogOnly,
			})
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}
}

func disabledCatalogProviders(cfg *config.Config) map[string]bool {
	disabled := map[string]bool{}
	if cfg == nil {
		return disabled
	}
	for _, value := range cfg.ModelCatalog.DisabledProviders {
		key := strings.ToLower(strings.TrimSpace(value))
		if key == "" {
			continue
		}
		disabled[key] = true
		disabled[catalog.NormalizeProviderID(key)] = true
	}
	return disabled
}

func catalogProviderAvailability(cfg *config.Config, provider catalog.Provider, disabled map[string]bool) (bool, string) {
	if disabled[strings.ToLower(provider.ID)] || disabled[provider.AuraProviderType] {
		return false, "disabled"
	}
	if provider.CatalogOnly {
		return false, "catalog_only"
	}
	if provider.AllowUnauthenticated || providerTypeWorksWithoutKey(provider.AuraProviderType) {
		return true, "available"
	}
	if cfg == nil {
		return false, "missing_credentials"
	}
	for _, entry := range cfg.Providers {
		if catalog.NormalizeProviderID(entry.Type) != provider.AuraProviderType && strings.ToLower(entry.ID) != strings.ToLower(provider.ID) {
			continue
		}
		if strings.EqualFold(entry.AuthType, "oauth2") {
			if strings.TrimSpace(entry.APIKey) != "" {
				return true, "available"
			}
			return false, "missing_credentials"
		}
		if strings.TrimSpace(entry.APIKey) != "" {
			return true, "available"
		}
	}
	return false, "missing_credentials"
}

func providerTypeWorksWithoutKey(providerType string) bool {
	switch catalog.NormalizeProviderID(providerType) {
	case "ollama", "llamacpp", "lmstudio", "manifest", "copilot":
		return true
	default:
		return false
	}
}
