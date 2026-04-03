package commands

import (
	"strings"

	"aurago/internal/llm"
)

// CreditsCommand shows the OpenRouter credit balance.
type CreditsCommand struct{}

func (c *CreditsCommand) Execute(args []string, ctx Context) (string, error) {
	var apiKey, baseURL string
	found := false

	// Check primary LLM
	if strings.ToLower(ctx.Cfg.LLM.ProviderType) == "openrouter" && ctx.Cfg.LLM.APIKey != "" {
		apiKey = ctx.Cfg.LLM.APIKey
		baseURL = ctx.Cfg.LLM.BaseURL
		found = true
	} else if strings.ToLower(ctx.Cfg.LLM.HelperProviderType) == "openrouter" && ctx.Cfg.LLM.HelperAPIKey != "" {
		// Check helper LLM
		apiKey = ctx.Cfg.LLM.HelperAPIKey
		baseURL = ctx.Cfg.LLM.HelperBaseURL
		found = true
	} else {
		// Fallback to any provider configured with openrouter
		for _, p := range ctx.Cfg.Providers {
			if strings.ToLower(p.Type) == "openrouter" && p.APIKey != "" {
				apiKey = p.APIKey
				baseURL = p.BaseURL
				found = true
				break
			}
		}
	}

	if !found {
		return "ℹ️ /credits ist nur verfügbar wenn OpenRouter als LLM-Provider konfiguriert ist.\nAktueller Provider: " + ctx.Cfg.LLM.ProviderType, nil
	}

	credits, err := llm.FetchOpenRouterCredits(apiKey, baseURL)
	if err != nil {
		return "❌ Fehler beim Abrufen der OpenRouter Credits: " + err.Error(), nil
	}

	return credits.FormatCreditsText(), nil
}

func (c *CreditsCommand) Help() string {
	return "Zeigt den aktuellen OpenRouter Kontostand und Verbrauch."
}

func init() {
	Register("credits", &CreditsCommand{})
}
