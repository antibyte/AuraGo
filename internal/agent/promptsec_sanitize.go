package agent

import (
	"strings"

	"github.com/sashabaranov/go-openai"

	"aurago/internal/security"
)

func applyPromptSecToLatestUserMessage(messages []openai.ChatCompletionMessage, guardian *security.Guardian) ([]openai.ChatCompletionMessage, bool) {
	if guardian == nil {
		return messages, false
	}
	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]
		if msg.Role != openai.ChatMessageRoleUser {
			continue
		}
		if len(msg.MultiContent) > 0 {
			updatedParts := append([]openai.ChatMessagePart(nil), msg.MultiContent...)
			applied := false
			for partIdx, part := range updatedParts {
				if part.Type != openai.ChatMessagePartTypeText {
					continue
				}
				content := strings.TrimSpace(part.Text)
				if content == "" || guardian.HasPromptSecStructuredOutput(part.Text) {
					continue
				}
				scan := guardian.SanitizeForLLM(part.Text, "user")
				if scan.Sanitized == "" || scan.Sanitized == part.Text {
					continue
				}
				updatedParts[partIdx].Text = scan.Sanitized
				applied = true
			}
			if !applied {
				return messages, false
			}
			updated := append([]openai.ChatCompletionMessage(nil), messages...)
			updated[i].MultiContent = updatedParts
			return updated, true
		}
		content := strings.TrimSpace(msg.Content)
		if content == "" {
			return messages, false
		}
		if guardian.HasPromptSecStructuredOutput(msg.Content) {
			return messages, false
		}

		scan := guardian.SanitizeForLLM(msg.Content, "user")
		if scan.Sanitized == "" || scan.Sanitized == msg.Content {
			return messages, false
		}

		updated := append([]openai.ChatCompletionMessage(nil), messages...)
		updated[i].Content = scan.Sanitized
		return updated, true
	}
	return messages, false
}
