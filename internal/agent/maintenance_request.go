package agent

import (
	"aurago/internal/config"
	"aurago/internal/llm"
	"aurago/internal/prompts"

	"github.com/sashabaranov/go-openai"
)

// Maintenance completions share the helper/main route and must reserve space
// for reasoning even when the visible answer is only a few sentences.
func maintenanceCompletionBudget(cfg *config.Config, request openai.ChatCompletionRequest) (int, error) {
	route := llm.ModelRoute{
		ProviderID: cfg.LLM.Provider, ProviderType: cfg.LLM.ProviderType,
		BaseURL: cfg.LLM.BaseURL, Model: request.Model, Primary: true,
	}
	if helper := llm.ResolveHelperLLM(cfg); helper.Enabled && helper.Model != "" {
		route.ProviderID, route.ProviderType, route.BaseURL = helper.ProviderID, helper.ProviderType, helper.BaseURL
		route.Primary = false
	}
	if provider := cfg.FindProvider(route.ProviderID); provider != nil {
		route.ContextWindowOverride = provider.ContextWindow
		route.MaxOutputTokensOverride = provider.MaxOutputTokens
	}
	limits := llm.ResolveModelLimitsCached(route, cfg.Agent.ContextWindow)
	inputTokens := 32
	for _, message := range request.Messages {
		inputTokens += prompts.CountTokensForModel(message.Content, request.Model) + 4
	}
	return llm.JSONCompletionOutputBudget(limits, request.MaxTokens, inputTokens)
}
