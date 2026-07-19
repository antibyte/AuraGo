package catalog

import (
	"embed"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
)

const SourceOhMyPi = "oh-my-pi"

//go:embed ohmypi_models.json ohmypi_providers.json ohmypi_metadata.json
var bundledFS embed.FS

type Cost struct {
	Input      float64 `json:"input,omitempty"`
	Output     float64 `json:"output,omitempty"`
	CacheRead  float64 `json:"cache_read,omitempty"`
	CacheWrite float64 `json:"cache_write,omitempty"`
}

type Model struct {
	ID                string   `json:"id"`
	Name              string   `json:"name,omitempty"`
	Provider          string   `json:"provider"`
	API               string   `json:"api,omitempty"`
	BaseURL           string   `json:"base_url,omitempty"`
	Input             []string `json:"input,omitempty"`
	ContextWindow     int      `json:"context_window,omitempty"`
	MaxTokens         int      `json:"max_tokens,omitempty"`
	SupportsTools     bool     `json:"supports_tools,omitempty"`
	StructuredOutputs bool     `json:"structured_outputs,omitempty"`
	Multimodal        bool     `json:"multimodal,omitempty"`
	Reasoning         bool     `json:"reasoning,omitempty"`
	Cost              Cost     `json:"cost,omitempty"`
	Source            string   `json:"source"`
}

type Provider struct {
	ID                         string      `json:"id"`
	AuraProviderType           string      `json:"aura_provider_type"`
	Name                       string      `json:"name,omitempty"`
	DefaultModel               string      `json:"default_model,omitempty"`
	EnvVars                    []string    `json:"env_vars,omitempty"`
	OAuthProvider              string      `json:"oauth_provider,omitempty"`
	OAuthSetup                 *OAuthSetup `json:"oauth_setup,omitempty"`
	AllowUnauthenticated       bool        `json:"allow_unauthenticated,omitempty"`
	DynamicModelsAuthoritative bool        `json:"dynamic_models_authoritative,omitempty"`
	CatalogOnly                bool        `json:"catalog_only"`
	Source                     string      `json:"source"`
}

type OAuthSetup struct {
	Source            string   `json:"source,omitempty"`
	SourcePackage     string   `json:"source_package,omitempty"`
	SourceProvider    string   `json:"source_provider,omitempty"`
	Flow              string   `json:"flow,omitempty"`
	SetupURL          string   `json:"setup_url,omitempty"`
	DocsURL           string   `json:"docs_url,omitempty"`
	ConsoleLabel      string   `json:"console_label,omitempty"`
	RedirectURIField  string   `json:"redirect_uri_field,omitempty"`
	ClientIDField     string   `json:"client_id_field,omitempty"`
	ClientSecretField string   `json:"client_secret_field,omitempty"`
	ClientID          string   `json:"client_id,omitempty"`
	AuthURL           string   `json:"auth_url,omitempty"`
	TokenURL          string   `json:"token_url,omitempty"`
	Scopes            []string `json:"scopes,omitempty"`
	CallbackPort      int      `json:"callback_port,omitempty"`
	CallbackPath      string   `json:"callback_path,omitempty"`
}

type Metadata struct {
	PackageName   string   `json:"package_name"`
	Version       string   `json:"version"`
	TarballURL    string   `json:"tarball_url"`
	SyncedAt      string   `json:"synced_at"`
	RepositoryURL string   `json:"repository_url,omitempty"`
	License       string   `json:"license"`
	Copyright     string   `json:"copyright,omitempty"`
	SourceFiles   []string `json:"source_files"`
}

type Snapshot struct {
	Metadata  Metadata   `json:"metadata"`
	Providers []Provider `json:"providers"`
	Models    []Model    `json:"models"`

	providersByID    map[string]Provider
	modelsByProvider map[string]Model
	modelsByID       map[string]Model
}

var (
	loadOnce sync.Once
	loaded   *Snapshot
	loadErr  error
)

func Load() (*Snapshot, error) {
	loadOnce.Do(func() {
		modelsData, err := bundledFS.ReadFile("ohmypi_models.json")
		if err != nil {
			loadErr = fmt.Errorf("read bundled models: %w", err)
			return
		}
		providersData, err := bundledFS.ReadFile("ohmypi_providers.json")
		if err != nil {
			loadErr = fmt.Errorf("read bundled providers: %w", err)
			return
		}
		metadataData, err := bundledFS.ReadFile("ohmypi_metadata.json")
		if err != nil {
			loadErr = fmt.Errorf("read bundled metadata: %w", err)
			return
		}
		loaded, loadErr = LoadFromBytes(modelsData, providersData, metadataData)
	})
	return loaded, loadErr
}

func LoadFromBytes(modelsData, providersData, metadataData []byte) (*Snapshot, error) {
	var models []Model
	if err := json.Unmarshal(modelsData, &models); err != nil {
		return nil, fmt.Errorf("parse oh-my-pi models: %w", err)
	}
	var providers []Provider
	if err := json.Unmarshal(providersData, &providers); err != nil {
		return nil, fmt.Errorf("parse oh-my-pi providers: %w", err)
	}
	var metadata Metadata
	if err := json.Unmarshal(metadataData, &metadata); err != nil {
		return nil, fmt.Errorf("parse oh-my-pi metadata: %w", err)
	}
	snapshot := &Snapshot{
		Metadata:  metadata,
		Providers: providers,
		Models:    models,
	}
	if err := snapshot.Validate(); err != nil {
		return nil, err
	}
	snapshot.reindex()
	return snapshot, nil
}

func (s *Snapshot) Validate() error {
	if s == nil {
		return fmt.Errorf("nil catalog snapshot")
	}
	if strings.TrimSpace(s.Metadata.PackageName) == "" {
		return fmt.Errorf("oh-my-pi metadata package_name is required")
	}
	if strings.TrimSpace(s.Metadata.Version) == "" {
		return fmt.Errorf("oh-my-pi metadata version is required")
	}
	if strings.TrimSpace(s.Metadata.License) == "" {
		return fmt.Errorf("oh-my-pi metadata license is required")
	}
	for i := range s.Providers {
		p := &s.Providers[i]
		p.ID = strings.ToLower(strings.TrimSpace(p.ID))
		if p.ID == "" {
			return fmt.Errorf("provider at index %d has no id", i)
		}
		p.AuraProviderType = NormalizeProviderID(firstNonEmpty(p.AuraProviderType, p.ID))
		p.CatalogOnly = p.CatalogOnly || !IsRuntimeProviderType(p.AuraProviderType)
		p.Source = firstNonEmpty(p.Source, SourceOhMyPi)
		normalizeOAuthSetup(p.OAuthSetup)
	}
	inheritRuntimeOAuthSetup(s.Providers)
	for i := range s.Models {
		m := &s.Models[i]
		m.ID = strings.TrimSpace(m.ID)
		m.Provider = NormalizeProviderID(m.Provider)
		if m.ID == "" || m.Provider == "" {
			return fmt.Errorf("model at index %d has incomplete identity", i)
		}
		m.Source = firstNonEmpty(m.Source, SourceOhMyPi)
	}
	return nil
}

func normalizeOAuthSetup(setup *OAuthSetup) {
	if setup == nil {
		return
	}
	setup.Source = firstNonEmpty(setup.Source, SourceOhMyPi)
	setup.SourcePackage = strings.TrimSpace(setup.SourcePackage)
	setup.SourceProvider = strings.TrimSpace(setup.SourceProvider)
	setup.Flow = strings.TrimSpace(setup.Flow)
	setup.SetupURL = strings.TrimSpace(setup.SetupURL)
	setup.DocsURL = strings.TrimSpace(setup.DocsURL)
	setup.ConsoleLabel = strings.TrimSpace(setup.ConsoleLabel)
	setup.RedirectURIField = strings.TrimSpace(setup.RedirectURIField)
	setup.ClientIDField = strings.TrimSpace(setup.ClientIDField)
	setup.ClientSecretField = strings.TrimSpace(setup.ClientSecretField)
	setup.ClientID = strings.TrimSpace(setup.ClientID)
	setup.AuthURL = strings.TrimSpace(setup.AuthURL)
	setup.TokenURL = strings.TrimSpace(setup.TokenURL)
	setup.CallbackPath = strings.TrimSpace(setup.CallbackPath)
	if len(setup.Scopes) > 0 {
		scopes := make([]string, 0, len(setup.Scopes))
		seen := map[string]bool{}
		for _, scope := range setup.Scopes {
			scope = strings.TrimSpace(scope)
			if scope == "" || seen[scope] {
				continue
			}
			seen[scope] = true
			scopes = append(scopes, scope)
		}
		setup.Scopes = scopes
	}
}

func inheritRuntimeOAuthSetup(providers []Provider) {
	setupByRuntimeType := map[string]*OAuthSetup{}
	for i := range providers {
		p := &providers[i]
		if p.OAuthSetup == nil {
			continue
		}
		baseType := runtimeTypeFromOAuthVariant(firstNonEmpty(p.OAuthProvider, p.AuraProviderType, p.ID))
		if baseType == "" || !IsRuntimeProviderType(baseType) {
			continue
		}
		if _, exists := setupByRuntimeType[baseType]; !exists {
			setupByRuntimeType[baseType] = p.OAuthSetup
		}
	}
	for i := range providers {
		p := &providers[i]
		if p.OAuthSetup != nil || !IsRuntimeProviderType(p.AuraProviderType) {
			continue
		}
		if setup := setupByRuntimeType[p.AuraProviderType]; setup != nil {
			copied := *setup
			p.OAuthSetup = &copied
		}
	}
}

func runtimeTypeFromOAuthVariant(providerID string) string {
	key := NormalizeProviderID(providerID)
	if strings.HasSuffix(key, "-oauth") {
		return strings.TrimSuffix(key, "-oauth")
	}
	return ""
}

func (s *Snapshot) FindProvider(id string) (Provider, bool) {
	if s == nil {
		return Provider{}, false
	}
	if s.providersByID == nil {
		s.reindex()
	}
	key := strings.ToLower(strings.TrimSpace(id))
	if p, ok := s.providersByID[key]; ok {
		return p, true
	}
	if p, ok := s.providersByID[NormalizeProviderID(key)]; ok {
		return p, true
	}
	return Provider{}, false
}

func (s *Snapshot) FindModel(provider, modelID string) (Model, bool) {
	if s == nil {
		return Model{}, false
	}
	if s.modelsByProvider == nil {
		s.reindex()
	}
	key := modelKey(provider, modelID)
	model, ok := s.modelsByProvider[key]
	return model, ok
}

func (s *Snapshot) FindModelByID(modelID string) (Model, bool) {
	if s == nil {
		return Model{}, false
	}
	if s.modelsByID == nil {
		s.reindex()
	}
	model, ok := s.modelsByID[strings.ToLower(strings.TrimSpace(modelID))]
	return model, ok
}

func (s *Snapshot) ModelsForProvider(provider string) []Model {
	if s == nil {
		return nil
	}
	normalized := NormalizeProviderID(provider)
	var models []Model
	for _, model := range s.Models {
		if model.Provider == normalized {
			models = append(models, model)
		}
	}
	sort.SliceStable(models, func(i, j int) bool {
		return strings.ToLower(models[i].ID) < strings.ToLower(models[j].ID)
	})
	return models
}

func (s *Snapshot) reindex() {
	s.providersByID = make(map[string]Provider, len(s.Providers)*2)
	for _, provider := range s.Providers {
		s.providersByID[strings.ToLower(provider.ID)] = provider
		s.providersByID[strings.ToLower(provider.AuraProviderType)] = provider
	}
	s.modelsByProvider = make(map[string]Model, len(s.Models))
	s.modelsByID = make(map[string]Model, len(s.Models))
	for _, model := range s.Models {
		s.modelsByProvider[modelKey(model.Provider, model.ID)] = model
		idKey := strings.ToLower(strings.TrimSpace(model.ID))
		if _, exists := s.modelsByID[idKey]; !exists {
			s.modelsByID[idKey] = model
		}
	}
}

func NormalizeProviderID(id string) string {
	switch strings.ToLower(strings.TrimSpace(id)) {
	case "lm-studio":
		return "lmstudio"
	case "llama.cpp":
		return "llamacpp"
	case "github-copilot":
		return "copilot"
	default:
		return strings.ToLower(strings.TrimSpace(id))
	}
}

func IsRuntimeProviderType(providerType string) bool {
	return runtimeProviderTypes[NormalizeProviderID(providerType)]
}

func modelKey(provider, modelID string) string {
	return NormalizeProviderID(provider) + "/" + strings.ToLower(strings.TrimSpace(modelID))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

var runtimeProviderTypes = map[string]bool{
	"openai":      true,
	"openrouter":  true,
	"ollama":      true,
	"anthropic":   true,
	"google":      true,
	"workers-ai":  true,
	"custom":      true,
	"stability":   true,
	"ideogram":    true,
	"vision":      true,
	"minimax":     true,
	"agnes":       true,
	"glm":         true,
	"yepapi":      true,
	"huggingface": true,
	"manifest":    true,
	"omniroute":   true,
	"deepseek":    true,
	"groq":        true,
	"mistral":     true,
	"xai":         true,
	"moonshot":    true,
	"qwen":        true,
	"zai":         true,
	"llamacpp":    true,
	"lmstudio":    true,
	"copilot":     true,
	"opencode-go": true,
}
