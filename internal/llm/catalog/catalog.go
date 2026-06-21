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
	ID                         string   `json:"id"`
	AuraProviderType           string   `json:"aura_provider_type"`
	Name                       string   `json:"name,omitempty"`
	DefaultModel               string   `json:"default_model,omitempty"`
	EnvVars                    []string `json:"env_vars,omitempty"`
	OAuthProvider              string   `json:"oauth_provider,omitempty"`
	AllowUnauthenticated       bool     `json:"allow_unauthenticated,omitempty"`
	DynamicModelsAuthoritative bool     `json:"dynamic_models_authoritative,omitempty"`
	CatalogOnly                bool     `json:"catalog_only"`
	Source                     string   `json:"source"`
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
	}
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
	if model, ok := s.modelsByProvider[key]; ok {
		return model, true
	}
	return s.FindModelByID(modelID)
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
	"glm":         true,
	"yepapi":      true,
	"manifest":    true,
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
