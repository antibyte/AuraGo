package agent

import (
	"strings"

	"github.com/sashabaranov/go-openai"
)

// sanitizeReasoningForContinuation prevents completed hidden reasoning from
// growing without bound. Providers that require reasoning on a tool-call
// continuation retain only the newest assistant tool-call block.
func sanitizeReasoningForContinuation(messages []openai.ChatCompletionMessage, providerType, model string) []openai.ChatCompletionMessage {
	return sanitizeReasoningForContinuationRequired(messages, providerRequiresReasoningContinuation(providerType, model))
}

// sanitizeReasoningForRequestRoutes preserves the newest continuation block
// when any eligible primary or failover route requires it. A request may be
// sent through a fallback after final preparation, so primary-only decisions
// are unsafe.
func sanitizeReasoningForRequestRoutes(messages []openai.ChatCompletionMessage, routes []RequestRouteBudget, providerType, model string) []openai.ChatCompletionMessage {
	required := providerRequiresReasoningContinuation(providerType, model)
	for _, route := range routes {
		if providerRequiresReasoningContinuation(route.Limits.Route.ProviderType, route.Limits.Route.Model) {
			required = true
			break
		}
	}
	return sanitizeReasoningForContinuationRequired(messages, required)
}

func sanitizeReasoningForContinuationRequired(messages []openai.ChatCompletionMessage, required bool) []openai.ChatCompletionMessage {
	if len(messages) == 0 {
		return messages
	}
	keepIndex := -1
	if required {
		for index := len(messages) - 1; index >= 0; index-- {
			if messages[index].Role == openai.ChatMessageRoleAssistant && len(messages[index].ToolCalls) > 0 {
				keepIndex = index
				break
			}
		}
	}
	for index := range messages {
		if index != keepIndex {
			messages[index].ReasoningContent = ""
		}
	}
	return messages
}

func providerRequiresReasoningContinuation(providerType, model string) bool {
	combined := strings.ToLower(strings.TrimSpace(providerType) + " " + strings.TrimSpace(model))
	return strings.Contains(combined, "minimax") || strings.Contains(combined, "deepseek")
}
