package agent

import (
	"fmt"
	"strings"

	"github.com/sashabaranov/go-openai"
)

func messageText(msg openai.ChatCompletionMessage) string {
	if strings.TrimSpace(msg.Content) != "" {
		return msg.Content
	}
	if len(msg.MultiContent) == 0 {
		return ""
	}

	var textParts []string
	imageCount := 0
	for _, part := range msg.MultiContent {
		switch part.Type {
		case openai.ChatMessagePartTypeText:
			if strings.TrimSpace(part.Text) != "" {
				textParts = append(textParts, part.Text)
			}
		case openai.ChatMessagePartTypeImageURL:
			imageCount++
		}
	}

	out := strings.TrimSpace(strings.Join(textParts, "\n"))
	if out == "" && imageCount > 0 {
		if imageCount == 1 {
			return "[image]"
		}
		return fmt.Sprintf("[image x%d]", imageCount)
	}
	if imageCount > 0 {
		if imageCount == 1 {
			return out + "\n[image]"
		}
		return out + fmt.Sprintf("\n[image x%d]", imageCount)
	}
	return out
}

