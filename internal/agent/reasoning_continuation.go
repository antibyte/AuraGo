package agent

import (
	"strings"

	"aurago/internal/llm"

	"github.com/sashabaranov/go-openai"
)

// InterruptedReasoningContent is the placeholder content written for a response
// that was cut off with only reasoning.
const InterruptedReasoningContent = "Generation was interrupted. Only an incomplete reasoning fragment was received; no source or tool call from this response was applied."

// IsInterruptedReasoningMessage reports the placeholder written for a response
// that was cut off with only reasoning. Nothing from that response was applied.
func IsInterruptedReasoningMessage(message openai.ChatCompletionMessage) bool {
	return message.Role == openai.ChatMessageRoleAssistant && len(message.ToolCalls) == 0 && message.Content == InterruptedReasoningContent
}

func interruptedReasoningMessage(reasoning string) openai.ChatCompletionMessage {
	return openai.ChatCompletionMessage{
		Role:             openai.ChatMessageRoleAssistant,
		Content:          InterruptedReasoningContent,
		ReasoningContent: reasoning,
	}
}

// sanitizeReasoningForContinuation prevents completed hidden reasoning from
// growing without bound, except for GLM history and opaque Responses state.
// Other providers retain only the newest required tool-call block.
func sanitizeReasoningForContinuation(messages []openai.ChatCompletionMessage, providerType, model string) []openai.ChatCompletionMessage {
	if providerRequiresFullReasoningContinuation(providerType, model) {
		return messages
	}
	return sanitizeReasoningForContinuationRequired(messages, providerRequiresReasoningContinuation(providerType, model))
}

// sanitizeReasoningForRequestRoutes preserves the newest continuation block
// when any eligible primary or failover route requires it. A request may be
// sent through a fallback after final preparation, so primary-only decisions
// are unsafe.
func sanitizeReasoningForRequestRoutes(messages []openai.ChatCompletionMessage, routes []RequestRouteBudget, providerType, model string) []openai.ChatCompletionMessage {
	if providerRequiresFullReasoningContinuation(providerType, model) {
		return messages
	}
	required := providerRequiresReasoningContinuation(providerType, model)
	for _, route := range routes {
		if providerRequiresFullReasoningContinuation(route.Limits.Route.ProviderType, route.Limits.Route.Model) {
			return messages
		}
		if providerRequiresReasoningContinuation(route.Limits.Route.ProviderType, route.Limits.Route.Model) {
			required = true
		}
	}
	return sanitizeReasoningForContinuationRequired(messages, required)
}

func providerRequiresFullReasoningContinuation(providerType, model string) bool {
	provider := strings.ToLower(strings.TrimSpace(providerType))
	model = strings.ToLower(strings.TrimSpace(model))
	model = strings.TrimPrefix(strings.TrimPrefix(model, "z-ai/"), "zai/")
	return provider == "glm" || provider == "zai" || strings.HasPrefix(model, "glm-")
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
		if index != keepIndex && !llm.IsResponsesContinuation(messages[index].ReasoningContent) {
			messages[index].ReasoningContent = ""
		}
	}
	return messages
}

func providerRequiresReasoningContinuation(providerType, model string) bool {
	combined := strings.ToLower(strings.TrimSpace(providerType) + " " + strings.TrimSpace(model))
	return strings.Contains(combined, "minimax") || strings.Contains(combined, "deepseek") || strings.Contains(combined, "anthropic") || strings.Contains(combined, "claude")
}
