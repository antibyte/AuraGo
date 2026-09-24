package catalogsync

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"aurago/internal/llm/catalog"
)

type PackageMetadata struct {
	Name          string
	Version       string
	TarballURL    string
	License       string
	RepositoryURL string
	Author        string
}

type Snapshot struct {
	Metadata  catalog.Metadata
	Providers []catalog.Provider
	Models    []catalog.Model
}

func (s Snapshot) FindModel(provider, modelID string) (catalog.Model, bool) {
	return s.toCatalogSnapshot().FindModel(provider, modelID)
}

func (s Snapshot) FindProvider(id string) (catalog.Provider, bool) {
	return s.toCatalogSnapshot().FindProvider(id)
}

func (s Snapshot) ToCatalogSnapshot() *catalog.Snapshot {
	return s.toCatalogSnapshot()
}

func (s Snapshot) toCatalogSnapshot() *catalog.Snapshot {
	snapshot := &catalog.Snapshot{
		Metadata:  s.Metadata,
		Providers: append([]catalog.Provider(nil), s.Providers...),
		Models:    append([]catalog.Model(nil), s.Models...),
	}
	_ = snapshot.Validate()
	return snapshot
}

func BuildSnapshot(modelsJSON, rulesJSON []byte, metadata PackageMetadata) (Snapshot, error) {
	if strings.TrimSpace(metadata.Name) == "" {
		metadata.Name = "@oh-my-pi/pi-catalog"
	}
	if strings.TrimSpace(metadata.License) == "" {
		return Snapshot{}, fmt.Errorf("oh-my-pi package license metadata is required")
	}
	if !strings.EqualFold(strings.TrimSpace(metadata.License), "MIT") {
		return Snapshot{}, fmt.Errorf("unexpected oh-my-pi catalog license %q", metadata.License)
	}
	models, err := ParseModelsJSON(modelsJSON)
	if err != nil {
		return Snapshot{}, err
	}
	providers, err := ParseProviderRulesJSON(rulesJSON)
	if err != nil {
		return Snapshot{}, err
	}
	knownProviders := make(map[string]bool, len(providers))
	for _, provider := range providers {
		knownProviders[provider.AuraProviderType] = true
	}
	for _, model := range models {
		if knownProviders[model.Provider] {
			continue
		}
		providers = append(providers, catalog.Provider{
			ID:               model.Provider,
			AuraProviderType: model.Provider,
			Name:             titleFromID(model.Provider),
			CatalogOnly:      true,
			Source:           catalog.SourceOhMyPi,
		})
		knownProviders[model.Provider] = true
	}
	sort.SliceStable(models, func(i, j int) bool {
		left, right := modelSortKey(models[i]), modelSortKey(models[j])
		if left != right {
			return left < right
		}
		return deterministicModelTieKey(models[i]) < deterministicModelTieKey(models[j])
	})
	sort.SliceStable(providers, func(i, j int) bool {
		return providers[i].ID < providers[j].ID
	})
	snapshot := Snapshot{
		Metadata: catalog.Metadata{
			PackageName:   metadata.Name,
			Version:       metadata.Version,
			TarballURL:    metadata.TarballURL,
			SyncedAt:      time.Now().UTC().Format(time.RFC3339),
			RepositoryURL: cleanRepositoryURL(metadata.RepositoryURL),
			License:       metadata.License,
			Copyright:     copyrightNotice(metadata.Author),
			SourceFiles: []string{
				"package/src/models.json",
				"package/src/compat/rules.json",
				"package/package.json",
			},
		},
		Providers: providers,
		Models:    models,
	}
	if err := snapshot.toCatalogSnapshot().Validate(); err != nil {
		return Snapshot{}, err
	}
	return snapshot, nil
}

func ParseModelsJSON(data []byte) ([]catalog.Model, error) {
	var upstream map[string]map[string]upstreamModel
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&upstream); err != nil {
		return nil, fmt.Errorf("parse upstream models.json: %w", err)
	}
	models := make([]catalog.Model, 0)
	for providerID, providerModels := range upstream {
		for key, spec := range providerModels {
			if isNonLLMKind(spec.Kind) {
				continue
			}
			modelID := firstNonEmpty(spec.ID, key)
			provider := catalog.NormalizeProviderID(firstNonEmpty(spec.Provider, providerID))
			inputs := normalizeStringList(spec.Input)
			api := strings.TrimSpace(spec.API)
			supportsTools := apiSupportsTools(api)
			if spec.SupportsTools != nil {
				supportsTools = *spec.SupportsTools
			}
			models = append(models, catalog.Model{
				ID:                modelID,
				Name:              firstNonEmpty(spec.Name, modelID),
				Provider:          provider,
				API:               api,
				BaseURL:           strings.TrimSpace(spec.BaseURL),
				Input:             inputs,
				ContextWindow:     numberValue(spec.ContextWindow),
				MaxTokens:         numberValue(spec.MaxTokens),
				SupportsTools:     supportsTools,
				StructuredOutputs: supportsTools && apiSupportsStructuredOutputs(api, provider),
				Multimodal:        inputsAreMultimodal(inputs),
				Reasoning:         spec.Reasoning,
				Cost: catalog.Cost{
					Input:      spec.Cost.Input,
					Output:     spec.Cost.Output,
					CacheRead:  spec.Cost.CacheRead,
					CacheWrite: spec.Cost.CacheWrite,
				},
				Source: catalog.SourceOhMyPi,
			})
		}
	}
	return models, nil
}

func ParseProviderRulesJSON(data []byte) ([]catalog.Provider, error) {
	var rules struct {
		Providers map[string]struct {
			ID                         string   `json:"id"`
			DefaultModel               string   `json:"defaultModel"`
			EnvVars                    []string `json:"envVars"`
			AllowUnauthenticated       bool     `json:"allowUnauthenticated"`
			DynamicModelsAuthoritative bool     `json:"dynamicModelsAuthoritative"`
			Discovery                  struct {
				Label                string   `json:"label"`
				EnvVars              []string `json:"envVars"`
				OAuthProvider        string   `json:"oauthProvider"`
				AllowUnauthenticated bool     `json:"allowUnauthenticated"`
			} `json:"discovery"`
		} `json:"providers"`
	}
	if err := json.Unmarshal(data, &rules); err != nil {
		return nil, fmt.Errorf("parse provider rules.json: %w", err)
	}
	if len(rules.Providers) == 0 {
		return nil, fmt.Errorf("no providers parsed from rules.json")
	}
	providers := make([]catalog.Provider, 0, len(rules.Providers))
	for key, rule := range rules.Providers {
		id := strings.ToLower(strings.TrimSpace(firstNonEmpty(rule.ID, key)))
		if id != strings.ToLower(strings.TrimSpace(key)) {
			return nil, fmt.Errorf("provider rule ID %q does not match key %q", rule.ID, key)
		}
		envVars := rule.EnvVars
		if len(envVars) == 0 {
			envVars = rule.Discovery.EnvVars
		}
		auraType := catalog.NormalizeProviderID(id)
		providers = append(providers, catalog.Provider{
			ID:                         id,
			AuraProviderType:           auraType,
			Name:                       firstNonEmpty(rule.Discovery.Label, titleFromID(id)),
			DefaultModel:               rule.DefaultModel,
			EnvVars:                    envVars,
			OAuthProvider:              rule.Discovery.OAuthProvider,
			OAuthSetup:                 providerOAuthSetup(id, rule.Discovery.OAuthProvider),
			AllowUnauthenticated:       rule.AllowUnauthenticated || rule.Discovery.AllowUnauthenticated,
			DynamicModelsAuthoritative: rule.DynamicModelsAuthoritative,
			CatalogOnly:                !catalog.IsRuntimeProviderType(auraType),
			Source:                     catalog.SourceOhMyPi,
		})
	}
	return providers, nil
}

func providerOAuthSetup(id, oauthProvider string) *catalog.OAuthSetup {
	key := strings.ToLower(strings.TrimSpace(firstNonEmpty(oauthProvider, id)))
	switch key {
	case "google", "google-gemini-cli":
		return &catalog.OAuthSetup{
			Source:            catalog.SourceOhMyPi,
			SourcePackage:     "@oh-my-pi/pi-ai",
			SourceProvider:    "google-gemini-cli",
			Flow:              "authorization_code_pkce",
			SetupURL:          "https://console.cloud.google.com/apis/credentials",
			DocsURL:           "https://goo.gle/gemini-cli-auth-docs#workspace-gca",
			ConsoleLabel:      "Google Cloud Credentials",
			RedirectURIField:  "Authorized redirect URIs",
			ClientIDField:     "Client ID",
			ClientSecretField: "Client secret",
			AuthURL:           "https://accounts.google.com/o/oauth2/v2/auth",
			TokenURL:          "https://oauth2.googleapis.com/token",
			Scopes: []string{
				"https://www.googleapis.com/auth/cloud-platform",
				"https://www.googleapis.com/auth/userinfo.email",
				"https://www.googleapis.com/auth/userinfo.profile",
			},
			CallbackPort: 8085,
			CallbackPath: "/oauth2callback",
		}
	case "xai", "xai-oauth":
		return &catalog.OAuthSetup{
			Source:            catalog.SourceOhMyPi,
			SourcePackage:     "@oh-my-pi/pi-ai",
			SourceProvider:    "xai-oauth",
			Flow:              "authorization_code_pkce",
			SetupURL:          "https://hermes-agent.nousresearch.com/docs/guides/xai-grok-oauth",
			DocsURL:           "https://hermes-agent.nousresearch.com/docs/guides/xai-grok-oauth",
			ConsoleLabel:      "xAI Grok OAuth guide",
			RedirectURIField:  "Loopback callback URL",
			ClientIDField:     "Client ID",
			ClientSecretField: "Client secret",
			ClientID:          "b1a00492-073a-47ea-816f-4c329264a828",
			AuthURL:           "https://auth.x.ai/oauth2/authorize",
			TokenURL:          "https://auth.x.ai/oauth2/token",
			Scopes: []string{
				"openid",
				"profile",
				"email",
				"offline_access",
				"grok-cli:access",
				"api:access",
			},
			CallbackPort: 56121,
			CallbackPath: "/callback",
		}
	default:
		return nil
	}
}

func MarshalSnapshotFiles(snapshot Snapshot) (modelsJSON, providersJSON, metadataJSON []byte, err error) {
	modelsJSON, err = json.MarshalIndent(snapshot.Models, "", "  ")
	if err != nil {
		return nil, nil, nil, fmt.Errorf("marshal normalized models: %w", err)
	}
	providersJSON, err = json.MarshalIndent(snapshot.Providers, "", "  ")
	if err != nil {
		return nil, nil, nil, fmt.Errorf("marshal normalized providers: %w", err)
	}
	metadataJSON, err = json.MarshalIndent(snapshot.Metadata, "", "  ")
	if err != nil {
		return nil, nil, nil, fmt.Errorf("marshal normalized metadata: %w", err)
	}
	return append(modelsJSON, '\n'), append(providersJSON, '\n'), append(metadataJSON, '\n'), nil
}

type upstreamModel struct {
	ID            string       `json:"id"`
	Name          string       `json:"name"`
	Kind          string       `json:"kind"`
	API           string       `json:"api"`
	Provider      string       `json:"provider"`
	BaseURL       string       `json:"baseUrl"`
	Reasoning     bool         `json:"reasoning"`
	SupportsTools *bool        `json:"supportsTools"`
	Input         []string     `json:"input"`
	Cost          upstreamCost `json:"cost"`
	ContextWindow any          `json:"contextWindow"`
	MaxTokens     any          `json:"maxTokens"`
}

type upstreamCost struct {
	Input      float64 `json:"input"`
	Output     float64 `json:"output"`
	CacheRead  float64 `json:"cacheRead"`
	CacheWrite float64 `json:"cacheWrite"`
}

func isNonLLMKind(kind string) bool {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "embedding", "image", "video", "stt", "tts", "rerank", "search":
		return true
	default:
		return false
	}
}

func numberValue(value any) int {
	switch v := value.(type) {
	case nil:
		return 0
	case float64:
		return int(v)
	case json.Number:
		n, _ := v.Int64()
		return int(n)
	default:
		return 0
	}
}

func normalizeStringList(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		normalized := strings.ToLower(strings.TrimSpace(value))
		if normalized == "" || seen[normalized] {
			continue
		}
		seen[normalized] = true
		out = append(out, normalized)
	}
	return out
}

func inputsAreMultimodal(inputs []string) bool {
	for _, input := range inputs {
		switch strings.ToLower(strings.TrimSpace(input)) {
		case "image", "images", "audio", "video", "file", "files", "pdf":
			return true
		}
	}
	return false
}

func apiSupportsTools(api string) bool {
	api = strings.ToLower(strings.TrimSpace(api))
	return strings.Contains(api, "openai") ||
		strings.Contains(api, "openrouter") ||
		strings.Contains(api, "anthropic") ||
		strings.Contains(api, "google") ||
		strings.Contains(api, "gemini")
}

func modelSortKey(model catalog.Model) string {
	return strings.Join([]string{
		model.Provider,
		strings.ToLower(model.ID),
		strings.ToLower(model.Name),
		strings.ToLower(model.API),
		strings.ToLower(model.BaseURL),
		fmt.Sprintf("%020d", model.ContextWindow),
		fmt.Sprintf("%020d", model.MaxTokens),
	}, "\x00")
}

func deterministicModelTieKey(model catalog.Model) string {
	payload, _ := json.Marshal(model)
	return string(payload)
}

func apiSupportsStructuredOutputs(api, provider string) bool {
	if catalog.NormalizeProviderID(provider) == "ollama" {
		return false
	}
	return apiSupportsTools(api)
}

func titleFromID(id string) string {
	parts := strings.FieldsFunc(id, func(r rune) bool {
		return r == '-' || r == '_' || r == '.'
	})
	for i, part := range parts {
		if len(part) == 0 {
			continue
		}
		parts[i] = strings.ToUpper(part[:1]) + part[1:]
	}
	return strings.Join(parts, " ")
}

func cleanRepositoryURL(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "git+")
	raw = strings.TrimSuffix(raw, ".git")
	return raw
}

func copyrightNotice(author string) string {
	if strings.TrimSpace(author) != "" {
		return strings.TrimSpace(author) + " and oh-my-pi contributors"
	}
	return "Can Boluk and oh-my-pi contributors"
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
