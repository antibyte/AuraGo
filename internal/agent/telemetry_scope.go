package agent

import (
	"aurago/internal/llm"

	"github.com/sashabaranov/go-openai"
)

type telemetryScopeAwareClient interface {
	ActiveProviderAndModel() (string, string)
}

func refreshTelemetryScope(scope AgentTelemetryScope, client llm.ChatClient, resp *openai.ChatCompletionResponse) AgentTelemetryScope {
	refreshed := scope
	if scopedClient, ok := client.(telemetryScopeAwareClient); ok {
		providerType, model := scopedClient.ActiveProviderAndModel()
		if providerType != "" {
			refreshed.ProviderType = providerType
		}
		if model != "" {
			refreshed.Model = model
		}
	}
	if resp != nil && resp.Model != "" {
		refreshed.Model = resp.Model
	}
	return refreshed
}
