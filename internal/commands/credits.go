package commands

import (
	"strings"

	"aurago/internal/i18n"
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
		return i18n.T(ctx.Lang, "backend.credits_unavailable", ctx.Cfg.LLM.ProviderType), nil
	}

	credits, err := llm.FetchOpenRouterCredits(apiKey, baseURL)
	if err != nil {
		return i18n.T(ctx.Lang, "backend.credits_error", err.Error()), nil
	}

	return credits.FormatCreditsText(), nil
}

func (c *CreditsCommand) Help() string {
	return i18n.T("de", "backend.credits_help")
}

func init() {
	Register("credits", &CreditsCommand{})
}
