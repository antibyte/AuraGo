package llm

import (
	"aurago/internal/config"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ModelPricing holds pricing information for a single model.
type ModelPricing struct {
	ModelID          string  `json:"model_id"`
	InputPerMillion  float64 `json:"input_per_million"`
	OutputPerMillion float64 `json:"output_per_million"`
}

// PricingFetcher retrieves model pricing for a provider.
type PricingFetcher interface {
	FetchPricing(apiKey, baseURL string) ([]ModelPricing, error)
}

// pricingCache stores fetched pricing with TTL.
var pricingCache struct {
	sync.RWMutex
	data      []ModelPricing
	fetchedAt time.Time
}

const pricingCacheTTL = 1 * time.Hour

// FetchPricingForProvider returns model pricing for the given provider type.
// Uses OpenRouter's models API as a universal source for cloud providers,
// returns zero-cost entries for ollama, and empty results for custom/unknown.
func FetchPricingForProvider(providerType, apiKey, baseURL string) ([]ModelPricing, error) {
	switch strings.ToLower(providerType) {
	case "openrouter":
		return fetchOpenRouterPricing("")
	case "openai":
		return fetchOpenRouterPricingFiltered("openai/")
	case "anthropic":
		return fetchOpenRouterPricingFiltered("anthropic/")
	case "google":
		return fetchOpenRouterPricingFiltered("google/")
	case "ollama":
		return fetchOllamaPricing(baseURL)
	case "workers-ai":
		return fetchWorkersAIPricing(apiKey, baseURL)
	default:
		return nil, nil
	}
}

// fetchOpenRouterPricing fetches all model pricing from OpenRouter.
func fetchOpenRouterPricing(prefix string) ([]ModelPricing, error) {
	all, err := getCachedOrFetchOpenRouterModels()
	if err != nil {
		return nil, err
	}
	if prefix == "" {
		return all, nil
	}
	var filtered []ModelPricing
	for _, m := range all {
		if strings.HasPrefix(strings.ToLower(m.ModelID), prefix) {
			filtered = append(filtered, m)
		}
	}
	return filtered, nil
}

// fetchOpenRouterPricingFiltered fetches pricing from OpenRouter filtered by prefix,
// and strips the prefix from model IDs (e.g. "openai/gpt-4o" → "gpt-4o").
func fetchOpenRouterPricingFiltered(prefix string) ([]ModelPricing, error) {
	all, err := getCachedOrFetchOpenRouterModels()
	if err != nil {
		return nil, err
	}
	var filtered []ModelPricing
	lowerPrefix := strings.ToLower(prefix)
	for _, m := range all {
		if strings.HasPrefix(strings.ToLower(m.ModelID), lowerPrefix) {
			m.ModelID = m.ModelID[len(prefix):]
			filtered = append(filtered, m)
		}
	}
	return filtered, nil
}

// getCachedOrFetchOpenRouterModels returns cached pricing or fetches fresh data.
func getCachedOrFetchOpenRouterModels() ([]ModelPricing, error) {
	pricingCache.RLock()
	if len(pricingCache.data) > 0 && time.Since(pricingCache.fetchedAt) < pricingCacheTTL {
		result := make([]ModelPricing, len(pricingCache.data))
		copy(result, pricingCache.data)
		pricingCache.RUnlock()
		return result, nil
	}
	pricingCache.RUnlock()

	pricingCache.Lock()
	defer pricingCache.Unlock()

	// Double-check after acquiring write lock
	if len(pricingCache.data) > 0 && time.Since(pricingCache.fetchedAt) < pricingCacheTTL {
		result := make([]ModelPricing, len(pricingCache.data))
		copy(result, pricingCache.data)
		return result, nil
	}

	data, err := doFetchOpenRouterModels()
	if err != nil {
		return nil, err
	}
	pricingCache.data = data
	pricingCache.fetchedAt = time.Now()
	result := make([]ModelPricing, len(data))
	copy(result, data)
	return result, nil
}

// doFetchOpenRouterModels calls the OpenRouter API and parses model pricing.
func doFetchOpenRouterModels() ([]ModelPricing, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get("https://openrouter.ai/api/v1/models")
	if err != nil {
		return nil, fmt.Errorf("failed to reach OpenRouter: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OpenRouter returned HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var apiResp struct {
		Data []struct {
			ID      string `json:"id"`
			Pricing *struct {
				Prompt     string `json:"prompt"`
				Completion string `json:"completion"`
			} `json:"pricing"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse models response: %w", err)
	}

	var result []ModelPricing
	for _, m := range apiResp.Data {
		if m.Pricing == nil {
			continue
		}
		// OpenRouter pricing is per-token; convert to per-million
		promptPerToken, _ := strconv.ParseFloat(m.Pricing.Prompt, 64)
		completionPerToken, _ := strconv.ParseFloat(m.Pricing.Completion, 64)
		result = append(result, ModelPricing{
			ModelID:          m.ID,
			InputPerMillion:  promptPerToken * 1_000_000,
			OutputPerMillion: completionPerToken * 1_000_000,
		})
	}
	return result, nil
}

// fetchOllamaPricing returns zero-cost pricing for all locally available Ollama models.
func fetchOllamaPricing(baseURL string) ([]ModelPricing, error) {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	url := strings.TrimRight(baseURL, "/") + "/api/tags"

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to reach Ollama: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Ollama returned HTTP %d", resp.StatusCode)
	}

	var apiResp struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse Ollama response: %w", err)
	}

	result := make([]ModelPricing, len(apiResp.Models))
	for i, m := range apiResp.Models {
		result[i] = ModelPricing{
			ModelID:          m.Name,
			InputPerMillion:  0,
			OutputPerMillion: 0,
		}
	}
	return result, nil
}

// ToModelCosts converts a slice of ModelPricing to config.ModelCost entries.
func ToModelCosts(pricing []ModelPricing) []config.ModelCost {
	costs := make([]config.ModelCost, len(pricing))
	for i, p := range pricing {
		costs[i] = config.ModelCost{
			Name:             p.ModelID,
			InputPerMillion:  p.InputPerMillion,
			OutputPerMillion: p.OutputPerMillion,
		}
	}
	return costs
}

// fetchWorkersAIPricing returns pricing for Cloudflare Workers AI models.
// Workers AI uses a neuron-based billing model. The values below are converted
// to approximate per-million-token costs for budget tracking compatibility.
// If an API key and base URL are provided, we attempt to list models from the
// Cloudflare API; otherwise we return the hardcoded popular models list.
func fetchWorkersAIPricing(apiKey, baseURL string) ([]ModelPricing, error) {
	// Attempt live model list if credentials are available.
	if apiKey != "" && baseURL != "" {
		models, err := fetchWorkersAIModelsFromAPI(apiKey, baseURL)
		if err == nil && len(models) > 0 {
			return models, nil
		}
		// Fall through to hardcoded list on error.
	}
	return workersAIHardcodedPricing(), nil
}

// fetchWorkersAIModelsFromAPI queries the Cloudflare Workers AI models endpoint.
// baseURL should be the account-scoped API base, e.g.
// "https://api.cloudflare.com/client/v4/accounts/{account_id}/ai/v1".
func fetchWorkersAIModelsFromAPI(apiKey, baseURL string) ([]ModelPricing, error) {
	// The models endpoint is at /models relative to the AI base.
	url := strings.TrimRight(baseURL, "/") + "/models"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to reach Workers AI: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Workers AI returned HTTP %d", resp.StatusCode)
	}

	var apiResp struct {
		Result []struct {
			Name string `json:"name"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse Workers AI models response: %w", err)
	}

	// Build pricing with known costs or zero (free-tier models).
	known := workersAIKnownCosts()
	result := make([]ModelPricing, 0, len(apiResp.Result))
	for _, m := range apiResp.Result {
		p := ModelPricing{ModelID: m.Name}
		if k, ok := known[m.Name]; ok {
			p.InputPerMillion = k.InputPerMillion
			p.OutputPerMillion = k.OutputPerMillion
		}
		result = append(result, p)
	}
	return result, nil
}

// workersAIHardcodedPricing returns a static list of popular Workers AI models
// with approximate per-million-token pricing (USD).
func workersAIHardcodedPricing() []ModelPricing {
	known := workersAIKnownCosts()
	result := make([]ModelPricing, 0, len(known))
	for id, p := range known {
		result = append(result, ModelPricing{
			ModelID:          id,
			InputPerMillion:  p.InputPerMillion,
			OutputPerMillion: p.OutputPerMillion,
		})
	}
	return result
}

// workersAIKnownCosts returns approximate per-million-token costs for known
// Workers AI models.  Workers AI bills by neurons; these are converted to
// per-token equivalents based on published rates.
func workersAIKnownCosts() map[string]ModelPricing {
	return map[string]ModelPricing{
		"@cf/meta/llama-3.1-8b-instruct":               {InputPerMillion: 0.0, OutputPerMillion: 0.0}, // free tier
		"@cf/meta/llama-3.1-70b-instruct":              {InputPerMillion: 0.34, OutputPerMillion: 0.40},
		"@cf/meta/llama-3.2-1b-instruct":               {InputPerMillion: 0.0, OutputPerMillion: 0.0}, // free tier
		"@cf/meta/llama-3.2-3b-instruct":               {InputPerMillion: 0.0, OutputPerMillion: 0.0}, // free tier
		"@cf/meta/llama-3.3-70b-instruct-fp8-fast":     {InputPerMillion: 0.34, OutputPerMillion: 0.40},
		"@cf/mistral/mistral-7b-instruct-v0.2":         {InputPerMillion: 0.0, OutputPerMillion: 0.0}, // free tier
		"@cf/google/gemma-7b-it":                       {InputPerMillion: 0.0, OutputPerMillion: 0.0}, // free tier
		"@cf/qwen/qwen1.5-14b-chat-awq":                {InputPerMillion: 0.0, OutputPerMillion: 0.0}, // free tier
		"@cf/deepseek-ai/deepseek-r1-distill-qwen-32b": {InputPerMillion: 0.15, OutputPerMillion: 0.15},
		"@hf/thebloke/codellama-7b-instruct-awq":       {InputPerMillion: 0.0, OutputPerMillion: 0.0}, // free tier
	}
}
