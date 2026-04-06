package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// ModelInfo contains context window information fetched from the provider API.
type ModelInfo struct {
	ContextLength int `json:"context_length"`
}

// knownContextWindows is a static fallback table for providers whose APIs do not
// expose model info in OpenRouter-compatible format. Keys are lowercase model IDs
// (or partial prefixes that are matched with strings.HasPrefix / Contains).
// Values are context window sizes in tokens.
var knownContextWindows = []struct {
	prefix  string
	context int
}{
	// MiniMax models
	{"minimax-m2.7", 1_000_000},
	{"minimax-text-01", 1_000_000},
	{"abab7", 1_000_000},
	{"abab6.5s", 245_760},
	{"abab6.5t", 8_192},
	{"abab6.5g", 8_192},
	{"abab5.5s", 8_192},
	// Anthropic (direct, not via OpenRouter)
	{"claude-opus-4", 200_000},
	{"claude-sonnet-4", 200_000},
	{"claude-3-7-sonnet", 200_000},
	{"claude-3-5-sonnet", 200_000},
	{"claude-3-5-haiku", 200_000},
	{"claude-3-opus", 200_000},
	{"claude-3-sonnet", 200_000},
	{"claude-3-haiku", 200_000},
	{"claude-2", 200_000},
	// Google Gemini (direct)
	{"gemini-2.5-pro", 1_048_576},
	{"gemini-2.5-flash", 1_048_576},
	{"gemini-2.5", 1_048_576},
	{"gemini-2.0-flash", 1_048_576},
	{"gemini-1.5-pro", 2_097_152},
	{"gemini-1.5-flash", 1_048_576},
	// OpenAI (direct)
	{"o3-pro", 200_000},
	{"o4-mini", 200_000},
	{"o3-mini", 200_000},
	{"o3", 200_000},
	{"o1-pro", 200_000},
	{"o1-mini", 128_000},
	{"o1", 200_000},
	// DeepSeek (direct)
	{"deepseek-v3", 131_072},
	{"deepseek-r1", 131_072},
	{"deepseek-chat", 131_072},
	// Mistral (direct)
	{"mistral-large", 131_072},
	{"mistral-small", 131_072},
	{"mistral-medium", 131_072},
	{"codestral", 256_000},
	// NVIDIA Nemotron (via OpenRouter or direct)
	{"nemotron-super", 131_072},
	{"nemotron-ultra", 131_072},
	{"nemotron-nano", 131_072},
	{"nemotron-51b", 131_072},
	{"nemotron-70b", 131_072},
	{"nemotron", 131_072},
	// ZhipuAI GLM models (direct api.bigmodel.cn / open.bigmodel.cn)
	{"glm-4-long", 1_000_000},
	{"glm-4", 128_000},
	{"glm-z1", 32_000},
	{"glm-", 128_000},
	// Alibaba Qwen (direct / compatible gateway)
	{"qwen-long", 10_000_000},
	{"qwen-turbo", 1_000_000},
	{"qwen-plus", 131_072},
	{"qwen-max", 131_072},
	{"qwen2.5-72b", 131_072},
	{"qwen2.5-32b", 131_072},
	{"qwen2.5-14b", 131_072},
	{"qwen2.5-7b", 131_072},
	{"qwen2.5-coder", 131_072},
	{"qwen2.5-", 131_072},
	{"qwen2-72b", 131_072},
	{"qwen2-", 131_072},
	{"qwq-32b", 131_072},
	{"qwq-", 131_072},
	// Meta Llama (direct / Ollama / HF)
	{"llama-3.3", 131_072},
	{"llama-3.2", 131_072},
	{"llama-3.1", 131_072},
	{"llama-3", 131_072},
	{"llama3", 131_072},
	// Falcon, Phi, Mixtral, Command R (common self-hosted)
	{"phi-4", 16_384},
	{"phi-3.5", 131_072},
	{"phi-3", 131_072},
	{"mixtral-8x22b", 65_536},
	{"mixtral-8x7b", 32_768},
	{"mixtral", 32_768},
	{"command-r-plus", 131_072},
	{"command-r", 131_072},
	{"command-a", 256_000},
	// Grok (xAI)
	{"grok-3", 131_072},
	{"grok-2", 131_072},
	{"grok-", 131_072},
	// Yi (01.ai)
	{"yi-large", 32_768},
	{"yi-medium", 200_000},
	{"yi-", 32_768},
	// Baidu ERNIE
	{"ernie-4.5", 131_072},
	{"ernie-4", 131_072},
	{"ernie-", 8_192},
}

// lookupKnownContextWindow returns the known context window for a model based on the
// static table above. Returns (size, true) if found, (0, false) otherwise.
func lookupKnownContextWindow(model string) (int, bool) {
	lower := strings.ToLower(model)
	for _, entry := range knownContextWindows {
		if strings.HasPrefix(lower, entry.prefix) || strings.Contains(lower, entry.prefix) {
			return entry.context, true
		}
	}
	return 0, false
}

// DetectContextWindow queries the LLM provider API for the model's context window size.
// Supports OpenRouter and Ollama (via /api/show). Returns the context length in tokens, or 0 if detection fails.
func DetectContextWindow(baseURL, apiKey, model, provider string, logger *slog.Logger) int {
	// Check static known-models table first — covers providers that don't expose
	// an OpenRouter-compatible /api/v1/models endpoint (MiniMax, direct Anthropic, etc.)
	if ctxLen, ok := lookupKnownContextWindow(model); ok {
		logger.Info("[ContextDetect] Using known context window from static table", "model", model, "context_length", ctxLen)
		return ctxLen
	}
	lowerProvider := strings.ToLower(provider)
	if lowerProvider == "ollama" {
		return detectContextWindowOllama(baseURL, model, logger)
	}
	// Anthropic's /v1/models endpoint exists but does NOT return context_length.
	// All Claude models should already be covered by the static table above.
	// Skip the API call to avoid a useless HTTP request that always returns 0.
	if lowerProvider == "anthropic" {
		logger.Debug("[ContextDetect] Anthropic provider: static table is authoritative, skipping API query", "model", model)
		return 0
	}
	return detectContextWindowOpenRouter(baseURL, apiKey, model, logger)
}

// detectContextWindowOllama uses Ollama's native /api/show endpoint to get model info.
func detectContextWindowOllama(baseURL, model string, logger *slog.Logger) int {
	// baseURL is typically http://localhost:11434/v1 — strip the /v1 suffix
	ollamaBase := strings.TrimSuffix(strings.TrimSuffix(baseURL, "/"), "/v1")
	showURL := ollamaBase + "/api/show"

	payloadBytes, err := json.Marshal(map[string]string{"name": model})
	if err != nil {
		logger.Debug("[ContextDetect/Ollama] Failed to marshal request payload", "error", err)
		return 0
	}
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("POST", showURL, bytes.NewReader(payloadBytes))
	if err != nil {
		logger.Debug("[ContextDetect/Ollama] Failed to create request", "error", err)
		return 0
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		logger.Debug("[ContextDetect/Ollama] Failed to query /api/show", "error", err, "url", showURL)
		return 0
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logger.Debug("[ContextDetect/Ollama] /api/show returned non-200", "status", resp.StatusCode)
		return 0
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		logger.Debug("[ContextDetect/Ollama] Failed to read response", "error", err)
		return 0
	}

	// Ollama /api/show returns model_info with a key like
	// "<arch>.context_length" (e.g. "llama.context_length": 131072)
	// or the older "context_length" at the top level of model_info.
	var showResp struct {
		ModelInfo map[string]json.RawMessage `json:"model_info"`
	}
	if err := json.Unmarshal(body, &showResp); err != nil {
		logger.Debug("[ContextDetect/Ollama] Failed to parse /api/show response", "error", err)
		return 0
	}

	// Look for any key ending in "context_length" inside model_info
	for key, raw := range showResp.ModelInfo {
		if strings.HasSuffix(key, "context_length") || key == "context_length" {
			var ctxLen int
			if err := json.Unmarshal(raw, &ctxLen); err == nil && ctxLen > 0 {
				logger.Info("[ContextDetect/Ollama] Detected model context window", "model", model, "context_length", ctxLen, "key", key)
				return ctxLen
			}
		}
	}

	logger.Debug("[ContextDetect/Ollama] No context_length found in model_info", "model", model)
	return 0
}

// detectContextWindowOpenRouter queries the OpenRouter models API.
func detectContextWindowOpenRouter(baseURL, apiKey, model string, logger *slog.Logger) int {
	// Normalise: strip any trailing / so we can always append the path cleanly.
	base := strings.TrimRight(baseURL, "/")

	// Build a prioritised list of candidate URLs to try.
	// Different provider APIs host their model list at different paths:
	//   - OpenRouter / LiteLLM: <base>/api/v1/models  (base already ends with /api or not)
	//   - Standard OpenAI-compatible: <base>/v1/models  or  <base>/models
	//   - Self-hosted (base has /v1): strip /v1 then try /v1/models
	var candidates []string
	stripped := base
	if strings.HasSuffix(stripped, "/v1") {
		stripped = strings.TrimSuffix(stripped, "/v1")
	}
	if strings.HasSuffix(stripped, "/api") {
		// e.g. "https://openrouter.ai/api"  →  /v1/models
		candidates = append(candidates, stripped+"/v1/models")
	} else {
		// Typical case: try OpenRouter-style first, then bare /v1/models
		candidates = append(candidates, stripped+"/api/v1/models")
		candidates = append(candidates, stripped+"/v1/models")
		candidates = append(candidates, base+"/models")
	}

	client := &http.Client{Timeout: 10 * time.Second}

	for _, modelsURL := range candidates {
		ctxLen := queryModelsEndpoint(client, modelsURL, apiKey, model, logger)
		if ctxLen > 0 {
			return ctxLen
		}
	}

	logger.Debug("[ContextDetect] All candidate URLs exhausted, model not found", "model", model, "tried", candidates)
	return 0
}

// queryModelsEndpoint performs GET <url> and looks for the model's context_length.
func queryModelsEndpoint(client *http.Client, modelsURL, apiKey, model string, logger *slog.Logger) int {
	req, err := http.NewRequest("GET", modelsURL, nil)
	if err != nil {
		logger.Debug("[ContextDetect] Failed to create request", "error", err, "url", modelsURL)
		return 0
	}
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := client.Do(req)
	if err != nil {
		logger.Debug("[ContextDetect] Failed to query models API", "error", err, "url", modelsURL)
		return 0
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logger.Debug("[ContextDetect] Models API returned non-200", "status", resp.StatusCode, "url", modelsURL)
		return 0
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024)) // 2MB limit
	if err != nil {
		logger.Debug("[ContextDetect] Failed to read models response", "error", err)
		return 0
	}

	// Parse response — OpenRouter returns { "data": [ { "id": "...", "context_length": N, ... } ] }
	var result struct {
		Data []struct {
			ID            string `json:"id"`
			ContextLength int    `json:"context_length"`
		} `json:"data"`
		// Some providers wrap data at the top level as an array.
	}
	if err := json.Unmarshal(body, &result); err != nil {
		logger.Debug("[ContextDetect] Failed to parse models response", "error", err, "url", modelsURL)
		return 0
	}

	for _, m := range result.Data {
		if m.ID == model {
			if m.ContextLength > 0 {
				logger.Info("[ContextDetect] Detected model context window", "model", model, "context_length", m.ContextLength, "url", modelsURL)
				return m.ContextLength
			}
		}
	}

	logger.Debug("[ContextDetect] Model not found or context_length=0 in API response", "model", model, "total_models", len(result.Data), "url", modelsURL)
	return 0
}

// AutoConfigureBudget sets the system prompt token budget based on the detected context window.
// Budget allocation based on detected context window size.
// Only overrides if current budget is the default (0/unlimited) and context window was detected.
// If window is unknown, it defaults to 8192.
func AutoConfigureBudget(contextWindow, currentBudget int, logger *slog.Logger) (tokenBudget int, contextWindowOut int) {
	var suggestedBudget int

	if contextWindow <= 0 {
		suggestedBudget = 8192
		contextWindow = 0
	} else if contextWindow > 100000 {
		suggestedBudget = contextWindow * 50 / 100
	} else {
		suggestedBudget = contextWindow * 25 / 100
	}

	if suggestedBudget < 500 {
		suggestedBudget = 500
	}

	logger.Info(fmt.Sprintf("[ContextDetect] Auto-configured: context_window=%d, system_budget=%d (was %d)",
		contextWindow, suggestedBudget, currentBudget))

	return suggestedBudget, contextWindow
}
