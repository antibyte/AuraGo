package llm

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/providerutil"

	"github.com/sashabaranov/go-openai"
)

// detectProviderURLMismatch checks if the provider type doesn't match known URL patterns
// and returns a hint string if a mismatch is detected. Returns "" if no mismatch.
func detectProviderURLMismatch(providerType, baseURL string) string {
	lower := strings.ToLower(baseURL)
	switch providerType {
	case "openai":
		if strings.Contains(lower, "anthropic") {
			return "provider=openai but URL contains 'anthropic' — did you mean provider=anthropic?"
		}
		if strings.Contains(lower, "minimax") || strings.Contains(lower, "minimaxi") {
			return "provider=openai but URL contains 'minimax/minimaxi' — did you mean provider=minimax?"
		}
		if strings.Contains(lower, "openrouter") {
			return "provider=openai but URL contains 'openrouter' — did you mean provider=openrouter?"
		}
		if strings.Contains(lower, "cloudflare") || strings.Contains(lower, "workers") {
			return "provider=openai but URL contains 'cloudflare/workers' — did you mean provider=workers-ai?"
		}
	case "anthropic":
		if strings.Contains(lower, "openai") {
			return "provider=anthropic but URL contains 'openai' — did you mean provider=openai?"
		}
		if strings.Contains(lower, "openrouter") {
			return "provider=anthropic but URL contains 'openrouter' — did you mean provider=openrouter?"
		}
	case "openrouter":
		if strings.Contains(lower, "api.openai.com") {
			return "provider=openrouter but URL contains 'api.openai.com' — did you mean provider=openai?"
		}
		if strings.Contains(lower, "minimax") || strings.Contains(lower, "minimaxi") {
			return "provider=openrouter but URL contains 'minimax/minimaxi' — did you mean provider=minimax?"
		}
		if strings.Contains(lower, "anthropic") {
			return "provider=openrouter but URL contains 'anthropic' — did you mean provider=anthropic?"
		}
	case "minimax":
		if strings.Contains(lower, "api.openai.com") {
			return "provider=minimax but URL contains 'api.openai.com' — did you mean provider=openai?"
		}
		if strings.Contains(lower, "openrouter") {
			return "provider=minimax but URL contains 'openrouter' — did you mean provider=openrouter?"
		}
		if strings.Contains(lower, "anthropic") {
			return "provider=minimax uses MiniMax's OpenAI-compatible endpoint — use https://api.minimax.io/v1 or provider=anthropic for an Anthropic-compatible endpoint"
		}
	case "yepapi":
		if !strings.Contains(lower, "yepapi") {
			return "provider=yepapi but URL does not contain 'yepapi' — use https://api.yepapi.com/v1/ai for LLM chat completions"
		}
	case "ollama":
		if !strings.Contains(lower, "localhost") && !strings.Contains(lower, "127.0.0.1") && !strings.Contains(lower, "ollama") {
			return "provider=ollama but URL doesn't reference localhost or ollama — ollama is typically on localhost:11434"
		}
	case "deepseek":
		if !strings.Contains(lower, "deepseek") {
			return "provider=deepseek but URL does not contain 'deepseek' — use https://api.deepseek.com/v1"
		}
	case "groq":
		if !strings.Contains(lower, "groq") {
			return "provider=groq but URL does not contain 'groq' — use https://api.groq.com/openai/v1"
		}
	case "mistral":
		if !strings.Contains(lower, "mistral") {
			return "provider=mistral but URL does not contain 'mistral' — use https://api.mistral.ai/v1"
		}
	case "xai":
		if !strings.Contains(lower, "x.ai") {
			return "provider=xai but URL does not contain 'x.ai' — use https://api.x.ai/v1"
		}
	case "moonshot":
		if !strings.Contains(lower, "moonshot") {
			return "provider=moonshot but URL does not contain 'moonshot' — use https://api.moonshot.ai/v1"
		}
	case "qwen":
		if !strings.Contains(lower, "alibaba") && !strings.Contains(lower, "dashscope") {
			return "provider=qwen but URL does not contain 'alibaba' or 'dashscope' — use https://dashscope.aliyuncs.com/compatible-mode/v1"
		}
	case "zai":
		if !strings.Contains(lower, "bigmodel") {
			return "provider=zai but URL does not contain 'bigmodel' — use https://open.bigmodel.cn/api/paas/v4"
		}
	case "llamacpp":
		if !strings.Contains(lower, "localhost") && !strings.Contains(lower, "127.0.0.1") {
			return "provider=llamacpp but URL doesn't reference localhost — llama.cpp is typically on localhost:8080"
		}
	case "lmstudio":
		if !strings.Contains(lower, "localhost") && !strings.Contains(lower, "127.0.0.1") {
			return "provider=lmstudio but URL doesn't reference localhost — LM Studio is typically on localhost:1234"
		}
	case "copilot":
		if !strings.Contains(lower, "githubcopilot") {
			return "provider=copilot but URL does not contain 'githubcopilot' — use https://api.githubcopilot.com"
		}
	case "opencode-go":
		if !strings.Contains(lower, "opencode") {
			return "provider=opencode-go but URL does not contain 'opencode' — use https://opencode.ai/zen/go"
		}
	}
	return ""
}

// resolvedProvider holds explicit provider connection details shared by the main
// LLM, helper LLM, and subsystem clients.
type resolvedProvider struct {
	ProviderType string
	BaseURL      string
	APIKey       string
	AccountID    string
}

func normalizeProviderBaseURL(providerType, baseURL string) string {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return ""
	}
	u := providerutil.NormalizeBaseURL(baseURL)
	pt := strings.ToLower(strings.TrimSpace(providerType))
	if pt == "ollama" || pt == "llamacpp" || pt == "lmstudio" {
		u = strings.TrimRight(u, "/")
		if !strings.HasSuffix(u, "/v1") {
			u = u + "/v1"
		}
	}
	return u
}

func isLocalProviderType(providerType string) bool {
	pt := strings.ToLower(strings.TrimSpace(providerType))
	return pt == "ollama" || pt == "llamacpp" || pt == "lmstudio"
}

// buildOpenAIClientConfig assembles a go-openai client config from resolved
// provider details. When cfg is non-nil, global options such as AI Gateway and
// Anthropic thinking are applied consistently for main and helper providers.
func buildOpenAIClientConfig(cfg *config.Config, p resolvedProvider) openai.ClientConfig {
	providerType := strings.ToLower(strings.TrimSpace(p.ProviderType))
	apiKey := strings.TrimSpace(p.APIKey)
	baseURLRaw := strings.TrimSpace(p.BaseURL)
	accountID := strings.TrimSpace(p.AccountID)
	isLocal := isLocalProviderType(providerType)
	aiGatewayToken := ""

	if apiKey == "" && isLocal {
		apiKey = providerType
	}

	clientConfig := openai.DefaultConfig(apiKey)

	if baseURLRaw != "" {
		clientConfig.BaseURL = normalizeProviderBaseURL(providerType, baseURLRaw)
	}

	if baseURLRaw != "" && apiKey != "" {
		if mismatched := detectProviderURLMismatch(providerType, baseURLRaw); mismatched != "" {
			slog.Warn("[LLM] Provider type may not match base URL", "provider", providerType, "base_url", baseURLRaw, "hint", mismatched)
		}
	}

	if providerType == "workers-ai" && accountID != "" {
		clientConfig.BaseURL = fmt.Sprintf(
			"https://api.cloudflare.com/client/v4/accounts/%s/ai/v1",
			accountID,
		)
	}

	if cfg != nil && cfg.AIGateway.Enabled && cfg.AIGateway.AccountID != "" && cfg.AIGateway.GatewayID != "" && !isLocal {
		route := ResolveAIGatewayRoute(cfg, providerType, accountID)
		if route.RouteSupported {
			clientConfig.BaseURL = route.Endpoint
			if route.AuthHeader == "cf-aig-authorization" {
				aiGatewayToken = cfg.AIGateway.Token
			}
		}
	}

	if isLoopbackHTTPS(baseURLRaw) {
		transport := http.RoundTripper(loopbackHTTPSTransport())
		if providerType == "manifest" && cfg != nil {
			transport = &manifestRoutingTransport{base: transport, routing: cfg.Manifest.Routing}
		}
		clientConfig.HTTPClient = &http.Client{Transport: transport}
	} else if httpClient := buildLLMHTTPClient(cfg, providerType, aiGatewayToken, clientConfig.BaseURL); httpClient != nil {
		clientConfig.HTTPClient = httpClient
	}

	return clientConfig
}

// NewClient creates a new OpenAI compatible client based on the routing configuration.
// Handles provider-specific quirks: Ollama doesn't require an API key but the
// go-openai library still sends an Authorization header — we use a dummy value
// so the SDK doesn't choke on an empty string.
func NewClient(cfg *config.Config) *openai.Client {
	if cfg == nil {
		return openai.NewClientWithConfig(openai.DefaultConfig(""))
	}
	p := resolvedProvider{
		ProviderType: cfg.LLM.ProviderType,
		BaseURL:      cfg.LLM.BaseURL,
		APIKey:       cfg.LLM.APIKey,
		AccountID:    cfg.LLM.AccountID,
	}
	return openai.NewClientWithConfig(buildOpenAIClientConfig(cfg, p))
}

// isLoopbackHTTPS returns true when the URL targets https://127.0.0.1 or https://localhost.
// These addresses use a self-signed certificate and require a TLS-lenient transport.
func isLoopbackHTTPS(rawURL string) bool {
	lower := strings.ToLower(strings.TrimSpace(rawURL))
	if !strings.HasPrefix(lower, "https://") {
		return false
	}
	// Strip scheme and extract host
	hostpart := lower[len("https://"):]
	host := hostpart
	if idx := strings.IndexByte(hostpart, '/'); idx != -1 {
		host = hostpart[:idx]
	}
	// Remove port
	h, _, err := net.SplitHostPort(host)
	if err == nil {
		host = h
	}
	return host == "127.0.0.1" || host == "::1" || host == "localhost"
}

// loopbackHTTPSTransport returns an http.Transport suitable for loopback HTTPS:
// TLS certificate verification is skipped (self-signed cert) and HTTP/2 is
// disabled (avoids "tls: bad record MAC" caused by h2 ALPN + self-signed TLS).
func loopbackHTTPSTransport() *http.Transport {
	return &http.Transport{
		TLSClientConfig:   &tls.Config{InsecureSkipVerify: true}, // #nosec G402 — loopback only
		ForceAttemptHTTP2: false,
	}
}

// NewClientFromProvider creates an OpenAI-compatible client from explicit provider
// details (type, base URL, API key). Used by subsystems that resolve their own
// provider (memory analysis, personality engine, etc.) instead of using the main LLM.
func NewClientFromProvider(providerType, baseURL, apiKey string) *openai.Client {
	return NewClientFromProviderWithConfig(nil, providerType, baseURL, apiKey, "")
}

// NewClientFromProviderDetails creates an OpenAI-compatible client from
// explicit provider details. This variant also supports provider-specific
// metadata such as a Cloudflare Workers AI account ID.
func NewClientFromProviderDetails(providerType, baseURL, apiKey, accountID string) *openai.Client {
	return NewClientFromProviderWithConfig(nil, providerType, baseURL, apiKey, accountID)
}

// NewClientFromProviderWithConfig creates a client from explicit provider details
// and applies global cfg options (AI Gateway, Anthropic thinking) when cfg is set.
func NewClientFromProviderWithConfig(cfg *config.Config, providerType, baseURL, apiKey, accountID string) *openai.Client {
	p := resolvedProvider{
		ProviderType: providerType,
		BaseURL:      baseURL,
		APIKey:       apiKey,
		AccountID:    accountID,
	}
	return openai.NewClientWithConfig(buildOpenAIClientConfig(cfg, p))
}

func buildLLMHTTPClient(cfg *config.Config, providerType, aiGatewayToken, baseURL string) *http.Client {
	headerTimeout := responseHeaderTimeoutForProvider(cfg, providerType)
	transport := http.RoundTripper(defaultLLMHTTPTransport(headerTimeout))

	// Log the configured transport timeout so operators can verify that
	// large-prompt scenarios (Virtual Desktop) get a sufficiently long
	// ResponseHeaderTimeout instead of the 30s default.
	if headerTimeout >= 60*time.Second {
		slog.Debug("[LLM HTTP] Using extended ResponseHeaderTimeout for provider",
			"provider", providerType,
			"response_header_timeout", headerTimeout,
			"per_attempt_timeout", perAttemptTimeout(),
			"base_url", baseURL,
		)
	}

	if gatewayTransport := aiGatewayTransportFromConfig(cfg, providerType, aiGatewayToken, baseURL, transport); gatewayTransport != nil {
		transport = gatewayTransport
	}

	if providerType == "minimax" || providerType == "glm" {
		transport = &miniMaxTransport{base: transport}
	}

	if shouldUseOpenAIPromptCacheKey(providerType, baseURL) {
		transport = &openAIPromptCacheTransport{base: transport}
	}

	if providerType == "anthropic" {
		at := &anthropicTransport{base: transport}
		if cfg != nil {
			at.thinkingCfg = anthropicThinkingConfig{
				Enabled:        cfg.LLM.AnthropicThinking.Enabled,
				BudgetTokens:   cfg.LLM.AnthropicThinking.BudgetTokens,
				ModelAllowlist: cfg.LLM.AnthropicThinking.ModelAllowlist,
			}
		}
		transport = at
	}

	if providerType == "copilot" {
		transport = &copilotTransport{base: transport, auth: copilotAuthInstance}
	}

	if providerType == "opencode-go" {
		transport = &opencodeGoTransport{base: transport}
	}

	if providerType == "manifest" && cfg != nil {
		transport = &manifestRoutingTransport{base: transport, routing: cfg.Manifest.Routing}
	}

	// Always return a custom HTTP client so every provider gets proper
	// ResponseHeaderTimeout and transport settings.  Using nil here caused
	// generic providers (crof.ai, openrouter, etc.) to fall back to the
	// bare http.Client with no timeout, leading to invisible hangs until
	// the context deadline killed the request.
	client := &http.Client{Transport: transport, Timeout: 3 * time.Minute}

	client.Transport = &rateLimitAwareTransport{base: client.Transport}

	// Wrap the transport with instrumentation so we can pinpoint stalls
	// (body write, TLS handshake, first byte) when Virtual Desktop hangs.
	client.Transport = newLoggingTransport(client.Transport, slog.Default())

	return client
}

func defaultLLMHTTPTransport(responseHeaderTimeout time.Duration) *http.Transport {
	base, _ := http.DefaultTransport.(*http.Transport)
	if base == nil {
		base = &http.Transport{}
	}
	transport := base.Clone()
	if responseHeaderTimeout <= 0 {
		responseHeaderTimeout = 30 * time.Second
	}
	// Floor at 60s so Virtual Desktop and other large-prompt scenarios never
	// hit the old 30s default that caused http2 response-header timeouts.
	const floor = 60 * time.Second
	if responseHeaderTimeout < floor {
		responseHeaderTimeout = floor
	}
	transport.ResponseHeaderTimeout = responseHeaderTimeout
	return transport
}

func responseHeaderTimeoutForProvider(cfg *config.Config, providerType string) time.Duration {
	// Use the per-attempt timeout as the ceiling so the transport layer never
	// aborts before the retry context does.  This prevents "http2: timeout
	// awaiting response headers" on providers that need >30s just to start
	// streaming a response for large prompt payloads.
	ceiling := perAttemptTimeout()
	if ceiling <= 0 {
		ceiling = 120 * time.Second
	}

	var base time.Duration
	switch strings.ToLower(strings.TrimSpace(providerType)) {
	case "minimax":
		base = 90 * time.Second
	case "glm":
		base = 60 * time.Second
	default:
		base = 30 * time.Second
	}

	if base > ceiling {
		return base
	}
	return ceiling
}

func shouldUseOpenAIPromptCacheKey(providerType, baseURL string) bool {
	if providerType != "openai" {
		return false
	}
	return isOfficialOpenAIBaseURL(baseURL)
}

func isOfficialOpenAIBaseURL(rawURL string) bool {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return true
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	return strings.EqualFold(parsed.Hostname(), "api.openai.com")
}

// aiGatewaySegment maps a provider type to the Cloudflare AI Gateway URL segment.
func aiGatewaySegment(providerType string) string {
	switch strings.ToLower(strings.TrimSpace(providerType)) {
	case "openai":
		return "openai"
	case "anthropic":
		return "anthropic"
	case "workers-ai":
		return "workers-ai"
	case "openrouter":
		return "openrouter"
	case "deepseek":
		return "deepseek"
	case "groq":
		return "groq"
	case "mistral":
		return "mistral"
	case "xai":
		return "xai"
	default:
		return ""
	}
}

// AIGatewayRoute describes how a provider would be routed through Cloudflare AI Gateway.
type AIGatewayRoute struct {
	Status         string
	Message        string
	Provider       string
	Mode           string
	Endpoint       string
	Segment        string
	RouteSupported bool
	PrivacyMode    string
	AuthHeader     string
	GatewayID      string
	Warnings       []string
}

// ResolveAIGatewayRoute returns the runtime route AuraGo will use for an LLM provider.
func ResolveAIGatewayRoute(cfg *config.Config, providerType, providerAccountID string) AIGatewayRoute {
	provider := strings.ToLower(strings.TrimSpace(providerType))
	route := AIGatewayRoute{
		Status:      "disabled",
		Message:     "AI Gateway is not enabled",
		Provider:    provider,
		Mode:        "auto",
		PrivacyMode: "metadata_only",
	}
	if cfg == nil {
		return route
	}
	cfgCopy := *cfg
	config.NormalizeAIGatewayConfig(&cfgCopy)
	gw := cfgCopy.AIGateway
	route.Mode = gw.Mode
	route.PrivacyMode = gw.LogMode
	route.GatewayID = strings.TrimSpace(gw.GatewayID)
	if !gw.Enabled {
		return route
	}
	route.Status = "no_credentials"
	route.Message = "AI Gateway account ID or gateway ID is not configured"
	accountID := strings.TrimSpace(gw.AccountID)
	if accountID == "" || route.GatewayID == "" {
		return route
	}
	if isLocalProviderType(provider) {
		route.Status = "local_provider"
		route.Message = "Local providers are not routed through Cloudflare AI Gateway"
		route.Warnings = append(route.Warnings, "Local providers are excluded from AI Gateway routing.")
		return route
	}
	if provider == "workers-ai" {
		workersAccountID := strings.TrimSpace(providerAccountID)
		if workersAccountID == "" {
			workersAccountID = accountID
		}
		route.Status = "configured"
		route.Message = "AI Gateway route configured"
		route.RouteSupported = true
		route.AuthHeader = "Authorization"
		route.Endpoint = fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/ai/v1", workersAccountID)
		return route
	}
	segment := aiGatewaySegment(provider)
	if segment == "" {
		route.Status = "unsupported_provider"
		route.Message = "AI Gateway does not have a safe route for this provider in the selected mode"
		route.Warnings = append(route.Warnings, fmt.Sprintf("Provider %q is not routed through AI Gateway in mode %q.", provider, gw.Mode))
		return route
	}
	route.Status = "configured"
	route.Message = "AI Gateway route configured"
	route.RouteSupported = true
	route.AuthHeader = "cf-aig-authorization"
	route.Segment = segment
	route.Endpoint = fmt.Sprintf("https://gateway.ai.cloudflare.com/v1/%s/%s/%s", accountID, route.GatewayID, segment)
	return route
}

func isOpenAICompatibleAIGatewayProvider(providerType string) bool {
	switch strings.ToLower(strings.TrimSpace(providerType)) {
	case "openai", "openrouter", "custom", "manifest", "yepapi", "deepseek", "groq", "mistral", "xai", "moonshot", "qwen", "zai", "copilot", "opencode-go":
		return true
	default:
		return false
	}
}

// miniMaxTransport is an http.RoundTripper that makes requests compatible with
// MiniMax's OpenAI-compatible endpoint. MiniMax does not accept the "system"
// message role; this transport converts system messages by prepending their
// content to the first "user" message. It also maps MiniMax's reasoning_details
// extension to go-openai's reasoning_content field so the agent can preserve the
// interleaved-thinking chain across tool calls.
type miniMaxTransport struct {
	base http.RoundTripper
}

type openAIPromptCacheTransport struct {
	base http.RoundTripper
}

type aiGatewayAuthTransport struct {
	base             http.RoundTripper
	token            string
	gatewayID        string
	collectLog       bool
	collectPayload   bool
	metadata         map[string]string
	requestTimeoutMS int
	maxAttempts      int
	retryDelayMS     int
	backoff          string
}

func (t *openAIPromptCacheTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	isChatCompletion := req.Method == http.MethodPost && strings.HasSuffix(req.URL.Path, "/chat/completions")
	if req.Body == nil || !isChatCompletion {
		return t.baseTransport().RoundTrip(req)
	}

	body, err := io.ReadAll(req.Body)
	req.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("openai prompt cache transport: read body: %w", err)
	}

	body = openAIPromptCacheRequestBody(body)

	clone := req.Clone(req.Context())
	clone.Header = req.Header.Clone()
	clone.Body = io.NopCloser(bytes.NewReader(body))
	clone.ContentLength = int64(len(body))

	return t.baseTransport().RoundTrip(clone)
}

func (t *openAIPromptCacheTransport) baseTransport() http.RoundTripper {
	if t.base != nil {
		return t.base
	}
	return http.DefaultTransport
}

func openAIPromptCacheRequestBody(body []byte) []byte {
	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		return body
	}
	if _, exists := payload["prompt_cache_key"]; exists {
		return body
	}
	key := buildOpenAIPromptCacheKey(payload)
	if key == "" {
		return body
	}
	payload["prompt_cache_key"] = key
	result, err := json.Marshal(payload)
	if err != nil {
		return body
	}
	return result
}

func buildOpenAIPromptCacheKey(payload map[string]interface{}) string {
	fingerprint := struct {
		Model          string        `json:"model"`
		SystemPrefixes []string      `json:"system_prefixes,omitempty"`
		Tools          []interface{} `json:"tools,omitempty"`
	}{}
	if model, ok := payload["model"].(string); ok {
		fingerprint.Model = strings.TrimSpace(model)
	}

	if messages, ok := payload["messages"].([]interface{}); ok {
		for _, rawMsg := range messages {
			msg, ok := rawMsg.(map[string]interface{})
			if !ok || msg["role"] != "system" {
				continue
			}
			text := openAIRequestContentText(msg["content"])
			if text == "" {
				continue
			}
			fingerprint.SystemPrefixes = append(fingerprint.SystemPrefixes, openAICacheStablePrefix(text))
		}
	}
	if tools, ok := payload["tools"].([]interface{}); ok && len(tools) > 0 {
		fingerprint.Tools = tools
	}
	if fingerprint.Model == "" && len(fingerprint.SystemPrefixes) == 0 && len(fingerprint.Tools) == 0 {
		return ""
	}

	raw, err := json.Marshal(fingerprint)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(raw)
	return fmt.Sprintf("aurago-%x", sum[:16])
}

func openAIRequestContentText(content interface{}) string {
	switch v := content.(type) {
	case string:
		return v
	case []interface{}:
		var b strings.Builder
		for _, partRaw := range v {
			part, ok := partRaw.(map[string]interface{})
			if !ok {
				continue
			}
			if text, ok := part["text"].(string); ok {
				b.WriteString(text)
			}
		}
		return b.String()
	default:
		return ""
	}
}

func openAICacheStablePrefix(system string) string {
	if idx := strings.Index(system, "# TURN CONTEXT"); idx >= 0 {
		return strings.TrimSpace(system[:idx])
	}
	return strings.TrimSpace(system)
}

func (t *aiGatewayAuthTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	clone.Header = req.Header.Clone()
	if strings.TrimSpace(t.token) != "" {
		clone.Header.Set("cf-aig-authorization", "Bearer "+strings.TrimSpace(t.token))
	}
	if strings.TrimSpace(t.gatewayID) != "" {
		clone.Header.Set("cf-aig-gateway-id", strings.TrimSpace(t.gatewayID))
	}
	clone.Header.Set("cf-aig-collect-log", strconv.FormatBool(t.collectLog))
	clone.Header.Set("cf-aig-collect-log-payload", strconv.FormatBool(t.collectPayload))
	if len(t.metadata) > 0 {
		if raw, err := json.Marshal(t.metadata); err == nil {
			clone.Header.Set("cf-aig-metadata", string(raw))
		}
	}
	if t.requestTimeoutMS > 0 {
		clone.Header.Set("cf-aig-request-timeout", strconv.Itoa(t.requestTimeoutMS))
	}
	if t.maxAttempts > 0 {
		clone.Header.Set("cf-aig-max-attempts", strconv.Itoa(t.maxAttempts))
	}
	if t.retryDelayMS > 0 {
		clone.Header.Set("cf-aig-retry-delay", strconv.Itoa(t.retryDelayMS))
	}
	if strings.TrimSpace(t.backoff) != "" {
		clone.Header.Set("cf-aig-backoff", strings.TrimSpace(t.backoff))
	}

	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	return base.RoundTrip(clone)
}

func aiGatewayTransportFromConfig(cfg *config.Config, providerType, aiGatewayToken, baseURL string, base http.RoundTripper) http.RoundTripper {
	if cfg == nil || !cfg.AIGateway.Enabled || !isAIGatewayRoutedBaseURL(providerType, baseURL) {
		return nil
	}
	cfgCopy := *cfg
	config.NormalizeAIGatewayConfig(&cfgCopy)
	gw := cfgCopy.AIGateway
	transport := &aiGatewayAuthTransport{
		base:             base,
		metadata:         gw.Metadata,
		requestTimeoutMS: gw.RequestTimeoutMS,
		maxAttempts:      gw.MaxAttempts,
		retryDelayMS:     gw.RetryDelayMS,
		backoff:          gw.Backoff,
	}
	switch gw.LogMode {
	case "off":
		transport.collectLog = false
		transport.collectPayload = false
	case "full":
		transport.collectLog = true
		transport.collectPayload = true
	default:
		transport.collectLog = true
		transport.collectPayload = false
	}
	if strings.ToLower(strings.TrimSpace(providerType)) == "workers-ai" {
		transport.gatewayID = gw.GatewayID
	} else {
		transport.token = aiGatewayToken
	}
	return transport
}

func isAIGatewayRoutedBaseURL(providerType, baseURL string) bool {
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil || parsed == nil {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	if host == "gateway.ai.cloudflare.com" {
		return true
	}
	return strings.ToLower(strings.TrimSpace(providerType)) == "workers-ai" &&
		host == "api.cloudflare.com" &&
		strings.Contains(parsed.Path, "/client/v4/accounts/") &&
		strings.Contains(parsed.Path, "/ai/v1")
}

func (t *miniMaxTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	isChatCompletion := req.Method == http.MethodPost && strings.HasSuffix(req.URL.Path, "/chat/completions")
	isStream := false
	if req.Body != nil && isChatCompletion {
		body, err := io.ReadAll(req.Body)
		req.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("minimax transport: read body: %w", err)
		}
		body, isStream = miniMaxPrepareRequestBody(body)
		req.Body = io.NopCloser(bytes.NewReader(body))
		req.ContentLength = int64(len(body))
	}
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	resp, err := base.RoundTrip(req)
	if err != nil || resp == nil || !isChatCompletion || resp.Body == nil || resp.StatusCode >= http.StatusBadRequest {
		return resp, err
	}
	if isStream {
		resp.Body = newMiniMaxReasoningStreamBody(resp.Body)
		return resp, nil
	}
	body, readErr := io.ReadAll(resp.Body)
	resp.Body.Close()
	if readErr != nil {
		return nil, fmt.Errorf("minimax transport: read response body: %w", readErr)
	}
	body = miniMaxNormalizeResponseBody(body)
	resp.Body = io.NopCloser(bytes.NewReader(body))
	resp.ContentLength = int64(len(body))
	return resp, nil
}

func miniMaxPrepareRequestBody(body []byte) ([]byte, bool) {
	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		return body, false
	}

	stream, _ := payload["stream"].(bool)
	miniMaxMapReasoningContentToDetailsPayload(payload)
	miniMaxConvertSystemMessagesPayload(payload)
	payload["reasoning_split"] = true

	result, err := json.Marshal(payload)
	if err != nil {
		return body, stream
	}
	return result, stream
}

func miniMaxMapReasoningContentToDetailsPayload(payload map[string]interface{}) {
	messages, ok := payload["messages"].([]interface{})
	if !ok {
		return
	}
	for _, rawMessage := range messages {
		message, ok := rawMessage.(map[string]interface{})
		if !ok {
			continue
		}
		reasoning, _ := message["reasoning_content"].(string)
		if strings.TrimSpace(reasoning) == "" {
			continue
		}
		if _, exists := message["reasoning_details"]; !exists {
			message["reasoning_details"] = []map[string]interface{}{{
				"type":   "reasoning.text",
				"id":     "reasoning-text-1",
				"format": "MiniMax-response-v1",
				"index":  0,
				"text":   reasoning,
			}}
		}
		delete(message, "reasoning_content")
	}
}

func miniMaxConvertSystemMessagesPayload(payload map[string]interface{}) {
	rawMessages, ok := payload["messages"].([]interface{})
	if !ok {
		return
	}
	messages := make([]map[string]interface{}, 0, len(rawMessages))
	for _, rawMessage := range rawMessages {
		message, ok := rawMessage.(map[string]interface{})
		if !ok {
			return
		}
		messages = append(messages, message)
	}

	var system strings.Builder
	filtered := make([]map[string]interface{}, 0, len(messages))
	for _, message := range messages {
		if role, _ := message["role"].(string); role == "system" {
			text := extractTextContent(message["content"])
			if text != "" {
				if system.Len() > 0 {
					system.WriteString("\n\n")
				}
				system.WriteString(text)
			}
			continue
		}
		filtered = append(filtered, message)
	}
	if system.Len() == 0 {
		return
	}

	prepended := false
	for _, message := range filtered {
		if role, _ := message["role"].(string); role == "user" {
			if err := prependToUserContent(message, system.String()); err == nil {
				prepended = true
				break
			}
		}
	}
	if !prepended {
		filtered = append([]map[string]interface{}{{"role": "user", "content": system.String()}}, filtered...)
	}
	payload["messages"] = filtered
}

// miniMaxMapReasoningContentToDetailsRaw works on an already-parsed payload so
// the transport does not pay for a second JSON parse.
func miniMaxMapReasoningContentToDetailsRaw(payload map[string]json.RawMessage, body []byte) []byte {
	msgsRaw, ok := payload["messages"]
	if !ok {
		return body
	}
	var msgs []map[string]interface{}
	if err := json.Unmarshal(msgsRaw, &msgs); err != nil {
		return body
	}
	changed := false
	for _, msg := range msgs {
		reasoning, _ := msg["reasoning_content"].(string)
		if strings.TrimSpace(reasoning) == "" {
			continue
		}
		if _, exists := msg["reasoning_details"]; !exists {
			msg["reasoning_details"] = []map[string]interface{}{
				{
					"type":   "reasoning.text",
					"id":     "reasoning-text-1",
					"format": "MiniMax-response-v1",
					"index":  0,
					"text":   reasoning,
				},
			}
		}
		delete(msg, "reasoning_content")
		changed = true
	}
	if !changed {
		return body
	}
	newMsgs, err := json.Marshal(msgs)
	if err != nil {
		return body
	}
	payload["messages"] = json.RawMessage(newMsgs)
	result, err := json.Marshal(payload)
	if err != nil {
		return body
	}
	return result
}

func miniMaxMapReasoningContentToDetails(body []byte) []byte {
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(body, &payload); err != nil {
		return body
	}
	return miniMaxMapReasoningContentToDetailsRaw(payload, body)
}

func miniMaxEnableReasoningSplit(body []byte) []byte {
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(body, &payload); err != nil {
		return body
	}
	payload["reasoning_split"] = json.RawMessage("true")
	result, err := json.Marshal(payload)
	if err != nil {
		return body
	}
	return result
}

func miniMaxNormalizeResponseBody(body []byte) []byte {
	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		return body
	}
	choices, ok := payload["choices"].([]interface{})
	if !ok {
		return body
	}
	changed := false
	for _, choiceRaw := range choices {
		choice, ok := choiceRaw.(map[string]interface{})
		if !ok {
			continue
		}
		if msg, ok := choice["message"].(map[string]interface{}); ok {
			changed = miniMaxNormalizeReasoningContainer(msg) || changed
		}
		if delta, ok := choice["delta"].(map[string]interface{}); ok {
			changed = miniMaxNormalizeReasoningContainer(delta) || changed
		}
	}
	if !changed {
		return body
	}
	result, err := json.Marshal(payload)
	if err != nil {
		return body
	}
	return result
}

func miniMaxNormalizeReasoningContainer(container map[string]interface{}) bool {
	if existing, _ := container["reasoning_content"].(string); strings.TrimSpace(existing) != "" {
		return false
	}
	reasoning := miniMaxReasoningDetailsText(container["reasoning_details"])
	if reasoning == "" {
		return false
	}
	container["reasoning_content"] = reasoning
	return true
}

func miniMaxReasoningDetailsText(raw interface{}) string {
	switch v := raw.(type) {
	case []interface{}:
		var b strings.Builder
		for _, item := range v {
			if text := miniMaxReasoningDetailsText(item); text != "" {
				if b.Len() > 0 {
					b.WriteString("\n")
				}
				b.WriteString(text)
			}
		}
		return b.String()
	case map[string]interface{}:
		if text, _ := v["text"].(string); strings.TrimSpace(text) != "" {
			return text
		}
		if text, _ := v["thinking"].(string); strings.TrimSpace(text) != "" {
			return text
		}
		if text, _ := v["content"].(string); strings.TrimSpace(text) != "" {
			return text
		}
	case string:
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func newMiniMaxReasoningStreamBody(body io.ReadCloser) io.ReadCloser {
	reader, writer := io.Pipe()
	go func() {
		defer body.Close()
		buf := bufio.NewReader(body)
		for {
			line, err := buf.ReadString('\n')
			if line != "" {
				if _, writeErr := writer.Write([]byte(miniMaxNormalizeSSELine(line))); writeErr != nil {
					_ = writer.CloseWithError(writeErr)
					return
				}
			}
			if err != nil {
				if err == io.EOF {
					_ = writer.Close()
				} else {
					_ = writer.CloseWithError(err)
				}
				return
			}
		}
	}()
	return reader
}

func miniMaxNormalizeSSELine(line string) string {
	const prefix = "data: "
	if !strings.HasPrefix(line, prefix) {
		return line
	}
	body := strings.TrimPrefix(line, prefix)
	lineEnding := ""
	if strings.HasSuffix(body, "\r\n") {
		lineEnding = "\r\n"
		body = strings.TrimSuffix(body, "\r\n")
	} else if strings.HasSuffix(body, "\n") {
		lineEnding = "\n"
		body = strings.TrimSuffix(body, "\n")
	}
	if strings.TrimSpace(body) == "[DONE]" {
		return line
	}
	normalized := miniMaxNormalizeResponseBody([]byte(body))
	return prefix + string(normalized) + lineEnding
}

// miniMaxConvertSystemMessages rewrites a chat completions JSON request body,
// collapsing all "system" role messages and prepending them to the first
// "user" message. If no user message exists, system content is discarded.
func miniMaxConvertSystemMessages(body []byte) []byte {
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(body, &payload); err != nil {
		return body
	}
	msgsRaw, ok := payload["messages"]
	if !ok {
		return body
	}

	var msgs []map[string]interface{}
	if err := json.Unmarshal(msgsRaw, &msgs); err != nil {
		return body
	}

	// Collect and remove system messages, handling both string and structured content.
	var sysBuilder strings.Builder
	var filtered []map[string]interface{}
	for _, m := range msgs {
		if role, _ := m["role"].(string); role == "system" {
			sysText := extractTextContent(m["content"])
			if sysText != "" {
				if sysBuilder.Len() > 0 {
					sysBuilder.WriteString("\n\n")
				}
				sysBuilder.WriteString(sysText)
			}
		} else {
			filtered = append(filtered, m)
		}
	}

	if sysBuilder.Len() == 0 {
		return body
	}
	sysContent := sysBuilder.String()

	// Prepend system content to the first user message, handling both string and structured content.
	prepended := false
	for _, m := range filtered {
		if role, _ := m["role"].(string); role == "user" {
			if err := prependToUserContent(m, sysContent); err != nil {
				continue
			}
			prepended = true
			break
		}
	}
	if !prepended {
		filtered = append([]map[string]interface{}{{
			"role":    "user",
			"content": sysContent,
		}}, filtered...)
	}

	newMsgs, err := json.Marshal(filtered)
	if err != nil {
		return body
	}
	payload["messages"] = newMsgs

	result, err := json.Marshal(payload)
	if err != nil {
		return body
	}
	return result
}

// prependToUserContent prepends sysContent to the content of a user message,
// handling both string and structured (array of parts) content.
func prependToUserContent(msg map[string]interface{}, sysContent string) error {
	content := msg["content"]
	switch v := content.(type) {
	case string:
		msg["content"] = sysContent + "\n\n" + v
	case []interface{}:
		// Try to inject as first text part; if no text part found, prepend as string.
		parts, ok := toContentParts(v)
		if !ok {
			return fmt.Errorf("cannot convert content parts")
		}
		foundTextPart := false
		for idx, part := range parts {
			if part["type"] == "text" {
				if text, ok := part["text"].(string); ok {
					parts[idx]["text"] = sysContent + "\n\n" + text
					foundTextPart = true
					break
				}
			}
		}
		if !foundTextPart {
			// No text part found: prepend a text part with the system content.
			newParts := make([]map[string]interface{}, 0, len(parts)+1)
			newParts = append(newParts, map[string]interface{}{"type": "text", "text": sysContent})
			newParts = append(newParts, parts...)
			msg["content"] = newParts
			return nil
		}
		msg["content"] = parts
	default:
		return fmt.Errorf("unsupported content type %T", content)
	}
	return nil
}

// toContentParts converts []interface{} to []map[string]interface{} for content parts.
func toContentParts(parts []interface{}) ([]map[string]interface{}, bool) {
	result := make([]map[string]interface{}, 0, len(parts))
	for _, p := range parts {
		if m, ok := p.(map[string]interface{}); ok {
			result = append(result, m)
		} else {
			return nil, false
		}
	}
	return result, true
}
