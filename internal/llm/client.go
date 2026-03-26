package llm

import (
	"fmt"
	"net/http"
	"strings"

	"aurago/internal/config"

	"github.com/sashabaranov/go-openai"
)

// miniMaxTransport is a custom RoundTripper for MiniMax's API.
// MiniMax does not accept the standard "Authorization: Bearer <key>" header —
// it authenticates via the "api_password" query parameter instead.
type miniMaxTransport struct {
	base   http.RoundTripper
	apiKey string
}

func (t *miniMaxTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req2 := req.Clone(req.Context())
	req2.Header.Del("Authorization")
	q := req2.URL.Query()
	q.Set("api_password", t.apiKey)
	req2.URL.RawQuery = q.Encode()
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	return base.RoundTrip(req2)
}

// applyMiniMaxConfig sets up a MiniMax-specific HTTP client on the given client config.
// It strips the Authorization header and injects the API key as a query param.
func applyMiniMaxConfig(cfg *openai.ClientConfig, apiKey, baseURL string) {
	// Ensure base URL ends with /v1
	if baseURL != "" {
		baseURL = strings.TrimRight(baseURL, "/")
		if !strings.HasSuffix(baseURL, "/v1") {
			baseURL = baseURL + "/v1"
		}
		cfg.BaseURL = baseURL
	} else if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.minimaxi.com/v1"
	}

	cfg.HTTPClient = &http.Client{
		Transport: &miniMaxTransport{
			base:   http.DefaultTransport,
			apiKey: apiKey,
		},
	}
}

// NewClient creates a new OpenAI compatible client based on the routing configuration.
// Handles provider-specific quirks: Ollama doesn't require an API key but the
// go-openai library still sends an Authorization header — we use a dummy value
// so the SDK doesn't choke on an empty string.
func NewClient(cfg *config.Config) *openai.Client {
	apiKey := cfg.LLM.APIKey
	providerType := strings.ToLower(cfg.LLM.ProviderType)
	isOllama := providerType == "ollama"
	isMiniMax := providerType == "minimax"

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

	// Workers AI: auto-build the OpenAI-compatible URL from the account ID.
	// Overrides any manually-set BaseURL since the URL is deterministic.
	if providerType == "workers-ai" && cfg.LLM.AccountID != "" {
		clientConfig.BaseURL = fmt.Sprintf(
			"https://api.cloudflare.com/client/v4/accounts/%s/ai/v1",
			cfg.LLM.AccountID,
		)
	}

	// MiniMax: uses query-parameter auth, not Authorization header.
	// Apply before the AI Gateway block so it doesn't conflict.
	if isMiniMax {
		applyMiniMaxConfig(&clientConfig, apiKey, cfg.LLM.BaseURL)
		return openai.NewClientWithConfig(clientConfig)
	}

	// AI Gateway: rewrite BaseURL to route through Cloudflare AI Gateway.
	// Provides caching, rate-limiting, logging and fallback for any provider.
	// Does not apply to local providers (Ollama) — no point proxying localhost.
	if cfg.AIGateway.Enabled && cfg.AIGateway.AccountID != "" && cfg.AIGateway.GatewayID != "" && !isOllama {
		segment := aiGatewaySegment(providerType)
		if segment != "" {
			clientConfig.BaseURL = fmt.Sprintf(
				"https://gateway.ai.cloudflare.com/v1/%s/%s/%s",
				cfg.AIGateway.AccountID,
				cfg.AIGateway.GatewayID,
				segment,
			)
		}
	}

	return openai.NewClientWithConfig(clientConfig)
}

// NewClientFromProvider creates an OpenAI-compatible client from explicit provider
// details (type, base URL, API key). Used by subsystems that resolve their own
// provider (memory analysis, personality engine, etc.) instead of using the main LLM.
func NewClientFromProvider(providerType, baseURL, apiKey string) *openai.Client {
	pt := strings.ToLower(providerType)
	isOllama := pt == "ollama"
	isMiniMax := pt == "minimax"

	if apiKey == "" && isOllama {
		apiKey = "ollama"
	}

	clientConfig := openai.DefaultConfig(apiKey)

	if isMiniMax {
		applyMiniMaxConfig(&clientConfig, apiKey, baseURL)
		return openai.NewClientWithConfig(clientConfig)
	}

	if baseURL != "" {
		u := baseURL
		if isOllama {
			u = strings.TrimRight(u, "/")
			if !strings.HasSuffix(u, "/v1") {
				u = u + "/v1"
			}
		}
		clientConfig.BaseURL = u
	}

	return openai.NewClientWithConfig(clientConfig)
}

// aiGatewaySegment maps a provider type to the Cloudflare AI Gateway URL segment.
func aiGatewaySegment(providerType string) string {
	switch providerType {
	case "openai":
		return "openai"
	case "anthropic":
		return "anthropic"
	case "google":
		return "google-ai-studio"
	case "workers-ai":
		return "workers-ai"
	case "openrouter", "custom":
		// OpenRouter and custom providers are OpenAI-compatible
		return "openai"
	default:
		return ""
	}
}
