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
	"strings"

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
	case "ollama":
		if !strings.Contains(lower, "localhost") && !strings.Contains(lower, "127.0.0.1") && !strings.Contains(lower, "ollama") {
			return "provider=ollama but URL doesn't reference localhost or ollama — ollama is typically on localhost:11434"
		}
	}
	return ""
}

// NewClient creates a new OpenAI compatible client based on the routing configuration.
// Handles provider-specific quirks: Ollama doesn't require an API key but the
// go-openai library still sends an Authorization header — we use a dummy value
// so the SDK doesn't choke on an empty string.
func NewClient(cfg *config.Config) *openai.Client {
	apiKey := cfg.LLM.APIKey
	providerType := strings.ToLower(cfg.LLM.ProviderType)
	isOllama := providerType == "ollama"
	aiGatewayToken := ""

	// Ollama doesn't require an API key; use a dummy value so the SDK
	// always sends a well-formed Authorization header.
	if apiKey == "" && isOllama {
		apiKey = "ollama"
	}

	clientConfig := openai.DefaultConfig(apiKey)

	// Override the BaseURL if provided in the configuration (crucial for Ollama/OpenRouter)
	if cfg.LLM.BaseURL != "" {
		baseURL := providerutil.NormalizeBaseURL(cfg.LLM.BaseURL)

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

	// Warn if provider type doesn't match known base URL patterns
	if cfg.LLM.BaseURL != "" && cfg.LLM.APIKey != "" {
		if mismatched := detectProviderURLMismatch(providerType, cfg.LLM.BaseURL); mismatched != "" {
			slog.Warn("[LLM] Provider type may not match base URL", "provider", providerType, "base_url", cfg.LLM.BaseURL, "hint", mismatched)
		}
	}

	// Workers AI: auto-build the OpenAI-compatible URL from the account ID.
	// Overrides any manually-set BaseURL since the URL is deterministic.
	if providerType == "workers-ai" && cfg.LLM.AccountID != "" {
		clientConfig.BaseURL = fmt.Sprintf(
			"https://api.cloudflare.com/client/v4/accounts/%s/ai/v1",
			cfg.LLM.AccountID,
		)
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
			aiGatewayToken = cfg.AIGateway.Token
		}
	}

	if isLoopbackHTTPS(cfg.LLM.BaseURL) {
		clientConfig.HTTPClient = &http.Client{Transport: loopbackHTTPSTransport()}
	} else if httpClient := buildLLMHTTPClient(cfg, providerType, aiGatewayToken, clientConfig.BaseURL); httpClient != nil {
		clientConfig.HTTPClient = httpClient
	}

	return openai.NewClientWithConfig(clientConfig)
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
	return NewClientFromProviderDetails(providerType, baseURL, apiKey, "")
}

// NewClientFromProviderDetails creates an OpenAI-compatible client from
// explicit provider details. This variant also supports provider-specific
// metadata such as a Cloudflare Workers AI account ID.
func NewClientFromProviderDetails(providerType, baseURL, apiKey, accountID string) *openai.Client {
	pt := strings.ToLower(providerType)
	isOllama := pt == "ollama"

	if apiKey == "" && isOllama {
		apiKey = "ollama"
	}

	clientConfig := openai.DefaultConfig(apiKey)

	if baseURL != "" {
		u := providerutil.NormalizeBaseURL(baseURL)
		if isOllama {
			u = strings.TrimRight(u, "/")
			if !strings.HasSuffix(u, "/v1") {
				u = u + "/v1"
			}
		}
		clientConfig.BaseURL = u
	}

	if pt == "workers-ai" && strings.TrimSpace(accountID) != "" {
		clientConfig.BaseURL = fmt.Sprintf(
			"https://api.cloudflare.com/client/v4/accounts/%s/ai/v1",
			strings.TrimSpace(accountID),
		)
	}

	if isLoopbackHTTPS(baseURL) {
		clientConfig.HTTPClient = &http.Client{Transport: loopbackHTTPSTransport()}
	} else if httpClient := buildLLMHTTPClient(nil, pt, "", clientConfig.BaseURL); httpClient != nil {
		clientConfig.HTTPClient = httpClient
	}

	return openai.NewClientWithConfig(clientConfig)
}

func buildLLMHTTPClient(cfg *config.Config, providerType, aiGatewayToken, baseURL string) *http.Client {
	transport := http.RoundTripper(http.DefaultTransport)
	hasCustomTransport := false

	if token := strings.TrimSpace(aiGatewayToken); token != "" {
		transport = &aiGatewayAuthTransport{base: transport, token: token}
		hasCustomTransport = true
	}

	if providerType == "minimax" || providerType == "glm" {
		transport = &miniMaxTransport{base: transport}
		hasCustomTransport = true
	}

	if shouldUseOpenAIPromptCacheKey(providerType, baseURL) {
		transport = &openAIPromptCacheTransport{base: transport}
		hasCustomTransport = true
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
		hasCustomTransport = true
	}

	if !hasCustomTransport {
		return nil
	}

	return &http.Client{Transport: transport}
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
	base  http.RoundTripper
	token string
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
	clone.Header.Set("cf-aig-authorization", "Bearer "+t.token)

	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	return base.RoundTrip(clone)
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
	var stream bool
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(body, &payload); err == nil {
		if streamRaw, ok := payload["stream"]; ok {
			_ = json.Unmarshal(streamRaw, &stream)
		}
	}
	body = miniMaxConvertSystemMessages(body)
	body = miniMaxMapReasoningContentToDetails(body)
	body = miniMaxEnableReasoningSplit(body)
	return body, stream
}

func miniMaxMapReasoningContentToDetails(body []byte) []byte {
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
	payload["messages"] = newMsgs
	result, err := json.Marshal(payload)
	if err != nil {
		return body
	}
	return result
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
	for _, m := range filtered {
		if role, _ := m["role"].(string); role == "user" {
			if err := prependToUserContent(m, sysContent); err != nil {
				continue
			}
			break
		}
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
