package llm

import (
	"net/url"
	"strings"
)

// A custom endpoint is an explicit operator routing choice. Only recognized
// provider origins and paths may be replaced by a Cloudflare gateway route.
func canonicalAIGatewayProviderEndpoint(provider, accountID, raw string) bool {
	if strings.TrimSpace(raw) == "" {
		return true
	}
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme != "https" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.RawPath != "" || (u.Port() != "" && u.Port() != "443") {
		return false
	}
	expected := map[string]string{
		"openai": "api.openai.com/v1", "anthropic": "api.anthropic.com/v1",
		"openrouter": "openrouter.ai/api/v1", "deepseek": "api.deepseek.com/v1",
		"groq": "api.groq.com/openai/v1", "mistral": "api.mistral.ai/v1", "xai": "api.x.ai/v1",
	}
	if provider == "workers-ai" && accountID != "" {
		expected[provider] = "api.cloudflare.com/client/v4/accounts/" + accountID + "/ai/v1"
	}
	actual := strings.ToLower(u.Hostname()) + strings.TrimRight(u.Path, "/")
	if provider == "deepseek" && actual == "api.deepseek.com" {
		return true
	}
	return expected[provider] != "" && actual == expected[provider]
}
