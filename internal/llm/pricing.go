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
