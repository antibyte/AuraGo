package llm

import (
	"strings"

	"aurago/internal/config"

	"github.com/sashabaranov/go-openai"
)

// NewClient creates a new OpenAI compatible client based on the routing configuration.
// Handles provider-specific quirks: Ollama doesn't require an API key but the
// go-openai library still sends an Authorization header — we use a dummy value
// so the SDK doesn't choke on an empty string.
func NewClient(cfg *config.Config) *openai.Client {
	apiKey := cfg.LLM.APIKey
	isOllama := strings.EqualFold(cfg.LLM.ProviderType, "ollama")

	// Ollama doesn't require an API key; use a dummy value so the SDK
	// always sends a well-formed Authorization header.
	if apiKey == "" && isOllama {
		apiKey = "ollama"
	}

	clientConfig := openai.DefaultConfig(apiKey)

	// Override the BaseURL if provided in the configuration (crucial for Ollama/OpenRouter)
	if cfg.LLM.BaseURL != "" {
		baseURL := cfg.LLM.BaseURL

		// Ollama's OpenAI-compatible endpoint lives under /v1.  The go-openai
		// library appends "/chat/completions" to BaseURL, so BaseURL must end
		// with "/v1".  Users commonly configure just "http://localhost:11434"
		// which would produce a 404.  Auto-fix this.
		if isOllama {
			baseURL = strings.TrimRight(baseURL, "/")
			if !strings.HasSuffix(baseURL, "/v1") {
				baseURL = baseURL + "/v1"
			}
		}

		clientConfig.BaseURL = baseURL
	}

	return openai.NewClientWithConfig(clientConfig)
}
