package agent

import (
	"strings"

	"github.com/sashabaranov/go-openai"
)

// sanitizeReasoningForContinuation prevents completed hidden reasoning from
// growing without bound. Providers that require reasoning on a tool-call
// continuation retain only the newest assistant tool-call block.
func sanitizeReasoningForContinuation(messages []openai.ChatCompletionMessage, providerType, model string) []openai.ChatCompletionMessage {
	if len(messages) == 0 {
		return messages
	}
	keepIndex := -1
	if providerRequiresReasoningContinuation(providerType, model) {
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
