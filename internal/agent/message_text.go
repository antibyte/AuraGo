package agent

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/sashabaranov/go-openai"
)

const multimodalImageTokenReserve = 4096

var multimodalImageAccountingReserve = strings.Repeat(" image", multimodalImageTokenReserve)

func messageText(msg openai.ChatCompletionMessage) string {
	content := strings.TrimSpace(msg.Content)

	if content != "" {
		return content
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

func messageTextWithReasoningForAccounting(msg openai.ChatCompletionMessage) string {
	parts := make([]string, 0, 8)
	if content := messageText(msg); content != "" {
		parts = append(parts, content)
	}
	if reasoning := strings.TrimSpace(msg.ReasoningContent); reasoning != "" {
		parts = append(parts, reasoning)
	}
	if refusal := strings.TrimSpace(msg.Refusal); refusal != "" {
		parts = append(parts, refusal)
	}
	if name := strings.TrimSpace(msg.Name); name != "" {
		parts = append(parts, "name="+name)
	}
	if msg.FunctionCall != nil {
		if encoded, err := json.Marshal(msg.FunctionCall); err == nil {
			parts = append(parts, string(encoded))
		}
	}
	if len(msg.ToolCalls) > 0 {
		if encoded, err := json.Marshal(msg.ToolCalls); err == nil {
			parts = append(parts, string(encoded))
		}
	}
	if toolCallID := strings.TrimSpace(msg.ToolCallID); toolCallID != "" {
		parts = append(parts, "tool_call_id="+toolCallID)
	}
	for _, part := range msg.MultiContent {
		if part.Type != openai.ChatMessagePartTypeImageURL || part.ImageURL == nil {
			continue
		}
		// Image payloads consume model context even though their binary bytes are
		// not text-tokenized. Reserve a conservative per-image envelope while
		// retaining only non-sensitive metadata in the accounting string.
		parts = append(parts, "image_detail="+string(part.ImageURL.Detail)+multimodalImageAccountingReserve)
	}
	return strings.Join(parts, "\n")
}
