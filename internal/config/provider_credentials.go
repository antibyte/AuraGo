package config

import "strings"

// HasMaskedCredentials distinguishes invalid placeholders from keyless providers.
func (p ProviderEntry) HasMaskedCredentials() bool {
	return strings.TrimSpace(p.APIKey) == "••••••••" ||
		strings.TrimSpace(p.OAuthClientSecret) == "••••••••" ||
		p.AuthType == "oauth2" && strings.TrimSpace(p.RuntimeOAuthToken) == "••••••••"
}

// RuntimeAPIKey never substitutes an inactive static key for OAuth credentials.
func (p ProviderEntry) RuntimeAPIKey() string {
	if p.HasMaskedCredentials() {
		return ""
	}
	if p.AuthType == "oauth2" {
		return p.RuntimeOAuthToken
	}
	return p.APIKey
}

func (c *Config) isManagedProviderBinding(binding providerCredentialBinding) bool {
	if !strings.EqualFold(binding.id, LocalLLMProviderID) {
		return false
	}
	if binding.kind == &c.LLM.ProviderType || binding.kind == &c.FallbackLLM.ProviderType {
		return true
	}
	if !strings.EqualFold(c.LLM.Provider, LocalLLMProviderID) {
		return false
	}
	selections := map[*string]string{
		&c.Vision.ProviderType: c.Vision.Provider, &c.Whisper.ProviderType: c.Whisper.Provider,
		&c.Embeddings.ProviderType: c.Embeddings.Provider, &c.CoAgents.LLM.ProviderType: c.CoAgents.LLM.Provider,
		&c.A2A.LLM.ProviderType: c.A2A.LLM.Provider, &c.LLMGuardian.ProviderType: c.LLMGuardian.Provider,
		&c.MissionPreparation.ProviderType: c.MissionPreparation.Provider,
	}
	if selection, ok := selections[binding.kind]; ok {
		return selection == ""
	}
	if c.CoAgents.LLM.Provider != "" {
		return false
	}
	for _, spec := range []*SpecialistConfig{&c.CoAgents.Specialists.Researcher, &c.CoAgents.Specialists.Coder, &c.CoAgents.Specialists.Designer, &c.CoAgents.Specialists.Security, &c.CoAgents.Specialists.Writer} {
		if binding.kind == &spec.LLM.ProviderType {
			return spec.LLM.Provider == ""
		}
	}
	return false
}

// A binding identifies the effective credential owner without comparing URLs.
// It does not change persisted selections: a blank selection inherits its parent.
type providerCredentialBinding struct {
	id             string
	kind, url, key *string
}

func (c *Config) providerCredentialBindings() []providerCredentialBinding {
	parent := func(id, inherited string) string {
		if strings.TrimSpace(id) == "" {
			return inherited
		}
		return id
	}
	main := c.LLM.Provider
	co := parent(c.CoAgents.LLM.Provider, main)
	helperProvider := c.FindProvider(c.LLM.HelperProvider)
	helper := c.LLM.HelperEnabled && helperProvider != nil && strings.TrimSpace(helperProvider.Type) != "" && c.LLM.HelperResolvedModel != ""
	helperID := func(id string) string {
		if helper {
			return c.LLM.HelperProvider
		}
		return id
	}
	bindings := []providerCredentialBinding{
		{main, &c.LLM.ProviderType, &c.LLM.BaseURL, &c.LLM.APIKey},
		{c.LLM.HelperProvider, &c.LLM.HelperProviderType, &c.LLM.HelperBaseURL, &c.LLM.HelperAPIKey},
		{c.FallbackLLM.Provider, &c.FallbackLLM.ProviderType, &c.FallbackLLM.BaseURL, &c.FallbackLLM.APIKey},
		{parent(c.Vision.Provider, main), &c.Vision.ProviderType, &c.Vision.BaseURL, &c.Vision.APIKey},
		{parent(c.Whisper.Provider, main), &c.Whisper.ProviderType, &c.Whisper.BaseURL, &c.Whisper.APIKey},
		{parent(c.Embeddings.Provider, main), &c.Embeddings.ProviderType, &c.Embeddings.BaseURL, &c.Embeddings.APIKey},
		{co, &c.CoAgents.LLM.ProviderType, &c.CoAgents.LLM.BaseURL, &c.CoAgents.LLM.APIKey},
		{parent(c.A2A.LLM.Provider, main), &c.A2A.LLM.ProviderType, &c.A2A.LLM.BaseURL, &c.A2A.LLM.APIKey},
		{parent(c.LLMGuardian.Provider, main), &c.LLMGuardian.ProviderType, &c.LLMGuardian.BaseURL, &c.LLMGuardian.APIKey},
		{parent(c.MissionPreparation.Provider, main), &c.MissionPreparation.ProviderType, &c.MissionPreparation.BaseURL, &c.MissionPreparation.APIKey},
		{helperID(c.Personality.V2Provider), &c.Personality.V2ProviderType, &c.Personality.V2ResolvedURL, &c.Personality.V2ResolvedKey},
		{helperID(c.MemoryAnalysis.Provider), &c.MemoryAnalysis.ProviderType, &c.MemoryAnalysis.BaseURL, &c.MemoryAnalysis.APIKey},
		{helperID(c.Tools.WebScraper.SummaryProvider), nil, &c.Tools.WebScraper.SummaryBaseURL, &c.Tools.WebScraper.SummaryAPIKey},
		{helperID(c.Tools.Wikipedia.SummaryProvider), nil, &c.Tools.Wikipedia.SummaryBaseURL, &c.Tools.Wikipedia.SummaryAPIKey},
		{helperID(c.Tools.DDGSearch.SummaryProvider), nil, &c.Tools.DDGSearch.SummaryBaseURL, &c.Tools.DDGSearch.SummaryAPIKey},
		{helperID(c.Tools.PDFExtractor.SummaryProvider), nil, &c.Tools.PDFExtractor.SummaryBaseURL, &c.Tools.PDFExtractor.SummaryAPIKey},
		{c.ImageGeneration.Provider, &c.ImageGeneration.ProviderType, &c.ImageGeneration.BaseURL, &c.ImageGeneration.APIKey},
		{c.MusicGeneration.Provider, &c.MusicGeneration.ProviderType, &c.MusicGeneration.BaseURL, &c.MusicGeneration.APIKey},
		{c.VideoGeneration.Provider, &c.VideoGeneration.ProviderType, &c.VideoGeneration.BaseURL, &c.VideoGeneration.APIKey},
	}
	for _, spec := range []*SpecialistConfig{&c.CoAgents.Specialists.Researcher, &c.CoAgents.Specialists.Coder, &c.CoAgents.Specialists.Designer, &c.CoAgents.Specialists.Security, &c.CoAgents.Specialists.Writer} {
		bindings = append(bindings, providerCredentialBinding{parent(spec.LLM.Provider, co), &spec.LLM.ProviderType, &spec.LLM.BaseURL, &spec.LLM.APIKey})
	}
	return bindings
}

func (c *Config) applyProviderCredentialBindings(tokens map[string]string) {
	inherit := func(id string, kind, url, key, parentKind, parentURL, parentKey *string) {
		if id != "" {
			return
		}
		*kind = *parentKind
		*url = *parentURL
		*key = *parentKey
	}
	inherit(c.CoAgents.LLM.Provider, &c.CoAgents.LLM.ProviderType, &c.CoAgents.LLM.BaseURL, &c.CoAgents.LLM.APIKey, &c.LLM.ProviderType, &c.LLM.BaseURL, &c.LLM.APIKey)
	inherit(c.Vision.Provider, &c.Vision.ProviderType, &c.Vision.BaseURL, &c.Vision.APIKey, &c.LLM.ProviderType, &c.LLM.BaseURL, &c.LLM.APIKey)
	inherit(c.Whisper.Provider, &c.Whisper.ProviderType, &c.Whisper.BaseURL, &c.Whisper.APIKey, &c.LLM.ProviderType, &c.LLM.BaseURL, &c.LLM.APIKey)
	inherit(c.Embeddings.Provider, &c.Embeddings.ProviderType, &c.Embeddings.BaseURL, &c.Embeddings.APIKey, &c.LLM.ProviderType, &c.LLM.BaseURL, &c.LLM.APIKey)
	inherit(c.A2A.LLM.Provider, &c.A2A.LLM.ProviderType, &c.A2A.LLM.BaseURL, &c.A2A.LLM.APIKey, &c.LLM.ProviderType, &c.LLM.BaseURL, &c.LLM.APIKey)
	inherit(c.LLMGuardian.Provider, &c.LLMGuardian.ProviderType, &c.LLMGuardian.BaseURL, &c.LLMGuardian.APIKey, &c.LLM.ProviderType, &c.LLM.BaseURL, &c.LLM.APIKey)
	inherit(c.MissionPreparation.Provider, &c.MissionPreparation.ProviderType, &c.MissionPreparation.BaseURL, &c.MissionPreparation.APIKey, &c.LLM.ProviderType, &c.LLM.BaseURL, &c.LLM.APIKey)
	for _, spec := range []*SpecialistConfig{&c.CoAgents.Specialists.Researcher, &c.CoAgents.Specialists.Coder, &c.CoAgents.Specialists.Designer, &c.CoAgents.Specialists.Security, &c.CoAgents.Specialists.Writer} {
		inherit(spec.LLM.Provider, &spec.LLM.ProviderType, &spec.LLM.BaseURL, &spec.LLM.APIKey, &c.CoAgents.LLM.ProviderType, &c.CoAgents.LLM.BaseURL, &c.CoAgents.LLM.APIKey)
	}
	for _, binding := range c.providerCredentialBindings() {
		if binding.id == LocalMusicProviderID && binding.kind == &c.MusicGeneration.ProviderType {
			continue
		}
		if strings.TrimSpace(binding.id) == "" {
			continue
		}
		if c.isManagedProviderBinding(binding) || binding.kind == &c.Embeddings.ProviderType && isBuiltinEmbeddingProvider(binding.id) {
			continue
		}
		provider := c.FindProvider(binding.id)
		if provider == nil || provider.ID == "google_workspace" && provider.Type == "" {
			*binding.key = ""
			*binding.url = "invalid://provider-reference"
			if binding.kind != nil {
				*binding.kind = "invalid-provider"
			}
			continue
		}
		if provider.HasMaskedCredentials() {
			*binding.key = "••••••••"
			*binding.url = "invalid://provider-credential"
			if binding.kind != nil {
				*binding.kind = "invalid-provider"
			}
			continue
		}
		*binding.key = provider.APIKey
		*binding.url = provider.BaseURL
		if binding.kind != nil {
			*binding.kind = provider.Type
		}
		if provider.AuthType == "oauth2" {
			*binding.key = tokens[provider.ID]
			if *binding.key == "" {
				*binding.url = "invalid://provider-oauth"
				if binding.kind != nil {
					*binding.kind = "invalid-provider"
				}
			}
		}
	}
}
