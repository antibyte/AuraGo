package catalog

const SourceCloudflare = "cloudflare"

// Curated Chat Completions choices from https://developers.cloudflare.com/ai/models/.
// Reviewed 2026-10-19. This is a starter list, not an availability allowlist.
// Costs intentionally remain unset: Cloudflare billing is separate from upstream billing.
func (s *Snapshot) addCloudflareRESTModels() {
	s.Providers = append(s.Providers, Provider{ID: "cloudflare-rest", AuraProviderType: "cloudflare-rest", Name: "Cloudflare REST", DefaultModel: "google/gemini-2.5-flash", Source: SourceCloudflare})
	for _, m := range []Model{
		{ID: "google/gemini-2.5-flash", Name: "Gemini 2.5 Flash", ContextWindow: 1048576, SupportsTools: true, Multimodal: true, Input: []string{"text", "image"}},
		{ID: "google/gemini-2.5-pro", Name: "Gemini 2.5 Pro", ContextWindow: 1048576, SupportsTools: true, Multimodal: true, Input: []string{"text", "image"}},
		{ID: "openai/gpt-4.1", Name: "GPT-4.1", ContextWindow: 1047576, SupportsTools: true, Multimodal: true, Input: []string{"text", "image"}},
		{ID: "openai/gpt-4.1-mini", Name: "GPT-4.1 mini", ContextWindow: 1047576, SupportsTools: true, Multimodal: true, Input: []string{"text", "image"}},
		{ID: "anthropic/claude-sonnet-4.5", Name: "Claude Sonnet 4.5", ContextWindow: 200000, SupportsTools: true, Multimodal: true, Input: []string{"text", "image"}},
	} {
		m.Provider = "cloudflare-rest"
		m.API = "openai-completions"
		m.Source = SourceCloudflare
		s.Models = append(s.Models, m)
	}
	s.reindex()
}
