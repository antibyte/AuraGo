package agent

import (
	openai "github.com/sashabaranov/go-openai"
	"testing"
)

func TestGamePreviewImagesRetainUserTaskAnchor(t *testing.T) {
	messages := []openai.ChatCompletionMessage{
		{Role: "user", Content: "Implement my game"},
		{Role: "assistant", ToolCalls: []openai.ToolCall{{ID: "capture", Type: "function", Function: openai.FunctionCall{Name: "game_maker_preview", Arguments: `{"operation":"screenshot"}`}}}},
		{Role: "tool", ToolCallID: "capture", Content: `{"capture_id":"image"}`},
		{Role: "user", Name: "game_maker_preview", MultiContent: []openai.ChatMessagePart{{Type: openai.ChatMessagePartTypeImageURL, ImageURL: &openai.ChatMessageImageURL{URL: "data:image/jpeg;base64,fixture"}}}},
	}
	if got := latestGenuineUserIndex(messages); got != 0 {
		t.Fatalf("image replaced task anchor: %d", got)
	}
	clean, dropped := SanitizeToolMessages(messages)
	if dropped != 0 || len(clean) != len(messages) {
		t.Fatal("image broke native tool round")
	}
}
