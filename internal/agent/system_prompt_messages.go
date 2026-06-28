package agent

import (
	"strings"

	"github.com/sashabaranov/go-openai"
)

func ensureGeneratedSystemPromptMessage(messages []openai.ChatCompletionMessage, sysPrompt, previousGenerated string) []openai.ChatCompletionMessage {
	generated := openai.ChatCompletionMessage{Role: openai.ChatMessageRoleSystem, Content: sysPrompt}
	if len(messages) == 0 {
		return []openai.ChatCompletionMessage{generated}
	}

	start := 0
	if messages[0].Role == openai.ChatMessageRoleSystem {
		firstContent := strings.TrimSpace(messages[0].Content)
		if firstContent == "" || (previousGenerated != "" && messages[0].Content == previousGenerated) {
			start = 1
		}
	}

	updated := make([]openai.ChatCompletionMessage, 0, len(messages)+1)
	updated = append(updated, generated)
	for i := start; i < len(messages); i++ {
		msg := messages[i]
		if msg.Role == openai.ChatMessageRoleSystem && strings.TrimSpace(msg.Content) == "" {
			continue
		}
		updated = append(updated, msg)
	}
	return updated
}
