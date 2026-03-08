package commands

import (
	"strings"

	"aurago/internal/llm"
)

// CreditsCommand shows the OpenRouter credit balance.
type CreditsCommand struct{}

func (c *CreditsCommand) Execute(args []string, ctx Context) (string, error) {
	provider := strings.ToLower(ctx.Cfg.LLM.ProviderType)
	if provider != "openrouter" {
		return "ℹ️ /credits ist nur verfügbar wenn OpenRouter als LLM-Provider konfiguriert ist.\nAktueller Provider: " + ctx.Cfg.LLM.ProviderType, nil
	}

	credits, err := llm.FetchOpenRouterCredits(ctx.Cfg.LLM.APIKey, ctx.Cfg.LLM.BaseURL)
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
