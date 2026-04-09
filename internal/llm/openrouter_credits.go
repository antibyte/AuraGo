package llm

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// OpenRouterCredits holds the credit balance information from OpenRouter.
type OpenRouterCredits struct {
	Balance     float64 `json:"balance"`      // remaining balance in USD
	Usage       float64 `json:"usage"`        // total usage in USD
	Limit       float64 `json:"limit"`        // credit limit (if any)
	IsFreeTier  bool    `json:"is_free_tier"` // whether using free tier
	RateLimited bool    `json:"rate_limited"` // whether currently rate-limited
}

// openRouterHTTPClient for credit queries.
var openRouterHTTPClient = &http.Client{Timeout: 10 * time.Second}

// FetchOpenRouterCredits queries the OpenRouter API for the current credit balance.
// The apiKey should be the OpenRouter API key.
// baseURL can be empty (defaults to "https://openrouter.ai/api/v1").
func FetchOpenRouterCredits(apiKey, baseURL string) (*OpenRouterCredits, error) {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return nil, fmt.Errorf("no API key provided")
	}

	url := "https://openrouter.ai/api/v1/auth/key"
	if baseURL != "" && strings.Contains(baseURL, "openrouter.ai") {
		url = strings.TrimRight(baseURL, "/") + "/auth/key"
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := openRouterHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(data))
	}

	// OpenRouter returns: {"data": {"label": "...", "usage": 0.123, "limit": null, "is_free_tier": false, "rate_limit": {...}}}
	var apiResp struct {
		Data struct {
			Label      string   `json:"label"`
			Usage      float64  `json:"usage"`
			Limit      *float64 `json:"limit"`
			IsFreeTier bool     `json:"is_free_tier"`
			RateLimit  *struct {
				Requests int    `json:"requests"`
				Interval string `json:"interval"`
			} `json:"rate_limit"`
		} `json:"data"`
	}

	if err := json.Unmarshal(data, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	credits := &OpenRouterCredits{
		Usage:      apiResp.Data.Usage,
		IsFreeTier: apiResp.Data.IsFreeTier,
	}

	if apiResp.Data.Limit != nil {
		credits.Limit = *apiResp.Data.Limit
		credits.Balance = *apiResp.Data.Limit - apiResp.Data.Usage
		if credits.Balance < 0 {
			credits.Balance = 0
		}
	}

	return credits, nil
}

// FormatCreditsText formats the credit information into a readable text string.
func (c *OpenRouterCredits) FormatCreditsText() string {
	if c == nil {
		return "❌ Keine Kreditdaten verfügbar."
	}

	lines := []string{"💳 **OpenRouter Credits**"}

	if c.Limit > 0 {
		lines = append(lines, fmt.Sprintf("• Guthaben: **$%.4f**", c.Balance))
		lines = append(lines, fmt.Sprintf("• Verbrauch: $%.4f / $%.2f", c.Usage, c.Limit))
		pct := (c.Usage / c.Limit) * 100
		lines = append(lines, fmt.Sprintf("• Nutzung: %.1f%%", pct))
	} else {
		lines = append(lines, fmt.Sprintf("• Verbrauch: **$%.4f**", c.Usage))
		lines = append(lines, "• Limit: keins (Pay-as-you-go)")
	}

	if c.IsFreeTier {
		lines = append(lines, "• Tier: Free")
	}

	return strings.Join(lines, "\n")
}
