package llm

import (
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

// DetectContextWindow queries the LLM provider API for the model's context window size.
// Supports OpenRouter and Ollama (via /api/show). Returns the context length in tokens, or 0 if detection fails.
func DetectContextWindow(baseURL, apiKey, model, provider string, logger *slog.Logger) int {
	if strings.EqualFold(provider, "ollama") {
		return detectContextWindowOllama(baseURL, model, logger)
	}
	return detectContextWindowOpenRouter(baseURL, apiKey, model, logger)
}

// detectContextWindowOllama uses Ollama's native /api/show endpoint to get model info.
func detectContextWindowOllama(baseURL, model string, logger *slog.Logger) int {
	// baseURL is typically http://localhost:11434/v1 — strip the /v1 suffix
	ollamaBase := strings.TrimSuffix(strings.TrimSuffix(baseURL, "/"), "/v1")
	showURL := ollamaBase + "/api/show"

	payload := fmt.Sprintf(`{"name":"%s"}`, model)
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("POST", showURL, strings.NewReader(payload))
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
	// OpenRouter exposes model info at /api/v1/models
	modelsURL := strings.TrimSuffix(baseURL, "/v1") + "/api/v1/models"
	// Also try the direct base if it already contains /api
	if strings.Contains(baseURL, "/api/v1") {
		modelsURL = strings.TrimSuffix(baseURL, "/v1") + "/v1/models"
	}

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", modelsURL, nil)
	if err != nil {
		logger.Debug("[ContextDetect] Failed to create request", "error", err)
		return 0
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := client.Do(req)
	if err != nil {
		logger.Debug("[ContextDetect] Failed to query models API", "error", err, "url", modelsURL)
		return 0
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logger.Debug("[ContextDetect] Models API returned non-200", "status", resp.StatusCode)
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
	}
	if err := json.Unmarshal(body, &result); err != nil {
		logger.Debug("[ContextDetect] Failed to parse models response", "error", err)
		return 0
	}

	for _, m := range result.Data {
		if m.ID == model {
			logger.Info("[ContextDetect] Detected model context window", "model", model, "context_length", m.ContextLength)
			return m.ContextLength
		}
	}

	logger.Debug("[ContextDetect] Model not found in API response", "model", model, "total_models", len(result.Data))
	return 0
}

// AutoConfigureBudget sets the system prompt token budget based on the detected context window.
// Budget allocation: 20% system prompt, 50% history, 30% response.
// Only overrides if current budget is the default (1200) and context window was detected.
func AutoConfigureBudget(contextWindow, currentBudget int, logger *slog.Logger) (tokenBudget int, contextWindowOut int) {
	if contextWindow <= 0 {
		return currentBudget, 0
	}

	suggestedBudget := contextWindow * 20 / 100 // 20% for system prompt
	if suggestedBudget < 500 {
		suggestedBudget = 500 // Minimum viable budget
	}
	if suggestedBudget > 8000 {
		suggestedBudget = 8000 // Cap — balances prompt richness vs. history/response space
	}

	logger.Info(fmt.Sprintf("[ContextDetect] Auto-configured: context_window=%d, system_budget=%d (was %d)",
		contextWindow, suggestedBudget, currentBudget))

	return suggestedBudget, contextWindow
}
