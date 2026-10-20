package llm

import (
	"context"
	"net/http"
	"strings"
	"time"

	"aurago/internal/config"
)

// ResolveProviderAIGatewayRoute also covers explicitly selected REST providers,
// which remain independent of the optional rewrite of ordinary providers.
func ResolveProviderAIGatewayRoute(cfg *config.Config, p config.ProviderEntry) AIGatewayRoute {
	if p.Type != "cloudflare-rest" {
		return ResolveAIGatewayRoute(cfg, p.Type, p.AccountID, p.BaseURL)
	}
	view := &config.Config{}
	if cfg != nil {
		*view = *cfg
	}
	config.NormalizeAIGatewayConfig(view)
	route := AIGatewayRoute{Provider: p.Type, Mode: "cloudflare_rest", PrivacyMode: view.AIGateway.LogMode, GatewayID: p.GatewayID, Status: "invalid_config", Message: "Cloudflare REST requires a valid account, gateway and canonical endpoint", RequestTimeoutMS: view.AIGateway.RequestTimeoutMS, EffectiveTimeoutMS: effectiveAIGatewayRequestTimeoutMS(cfg, p.Type)}
	if err := config.ValidateGatewayProvider(p); err != nil {
		return route
	}
	privacyView := *view
	privacyView.AIGateway.Enabled = false
	if config.ValidateAIGatewayConfig(&privacyView) != nil {
		return route
	}
	route.Status = "configured"
	route.Message = "Cloudflare REST route configured"
	route.RouteSupported = true
	route.AuthHeader = "Authorization"
	route.Endpoint = cloudflareRESTBaseURL(p.AccountID)
	return route
}

// AIGatewayMetadataProbe constructs a non-generating probe with the same route,
// authentication, redirect and per-request gateway policy as generation.
// A nil request means no documented safe metadata endpoint is available.
func AIGatewayMetadataProbe(ctx context.Context, cfg *config.Config, p config.ProviderEntry) (*http.Request, *http.Client, error) {
	return providerMetadataProbe(ctx, cfg, p, false)
}

// ProviderMetadataProbe also supports saved direct endpoints, without generation.
func ProviderMetadataProbe(ctx context.Context, cfg *config.Config, p config.ProviderEntry) (*http.Request, *http.Client, error) {
	return providerMetadataProbe(ctx, cfg, p, true)
}

func providerMetadataProbe(ctx context.Context, cfg *config.Config, p config.ProviderEntry, direct bool) (*http.Request, *http.Client, error) {
	route := ResolveProviderAIGatewayRoute(cfg, p)
	if !route.RouteSupported && !direct {
		return nil, nil, nil
	}
	endpoint := strings.TrimRight(route.Endpoint, "/")
	var directClient *http.Client
	if !route.RouteSupported {
		endpoint, directClient = primaryProbeTransport(TaskProviderConfig(cfg, p))
		endpoint = strings.TrimRight(endpoint, "/")
	}
	var target string
	switch p.Type {
	case "workers-ai", "cloudflare-rest":
		target = strings.TrimSuffix(endpoint, "/ai/v1") + "/ai/models/search?per_page=1"
	case "ollama":
		target = strings.TrimSuffix(endpoint, "/v1") + "/api/tags"
	case "anthropic":
		target = strings.TrimSuffix(endpoint, "/v1") + "/v1/models"
	case "openai", "openrouter", "deepseek", "groq", "mistral", "xai", "cerebras", "llamacpp", "lmstudio", "custom", "manifest", "omniroute":
		target = endpoint + "/models"
	default:
		return nil, nil, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, nil, err
	}
	if p.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.APIKey)
	}
	if p.Type == "anthropic" {
		req.Header.Set("x-api-key", p.APIKey)
		req.Header.Set("anthropic-version", anthropicAPIVersion)
	}
	client := newProbeHTTPClient(20 * time.Second)
	if directClient != nil {
		copy := *directClient
		copy.Timeout = 20 * time.Second
		client = &copy
	}
	token := ""
	if cfg != nil && route.AuthHeader == "cf-aig-authorization" {
		token = cfg.AIGateway.Token
	}
	if transport := aiGatewayTransportFromConfig(cfg, p.Type, token, route.Endpoint, client.Transport); transport != nil {
		client.Transport = transport
	}
	if p.Type == "cloudflare-rest" {
		req.Header.Set("cf-aig-gateway-id", p.GatewayID)
	}
	return req, client, nil
}
