package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestChatFrontend_ToolLeakSanitizerPatternsRemainPresent(t *testing.T) {
	t.Parallel()

	messagesPath := filepath.Join("js", "chat", "chat-messages.js")
	streamingPath := filepath.Join("js", "chat", "chat-streaming.js")

	messagesContent, err := os.ReadFile(messagesPath)
	if err != nil {
		t.Fatalf("read %s: %v", messagesPath, err)
	}
	streamingContent, err := os.ReadFile(streamingPath)
	if err != nil {
		t.Fatalf("read %s: %v", streamingPath, err)
	}

	msg := string(messagesContent)
	stream := string(streamingContent)

	requiredMessageMarkers := []string{
		"function stripLeakedToolMarkup",
		"minimax:tool_call",
		"<invoke\\b",
		"<parameter\\b",
		"\\[Suggested next step\\]",
		"containsLeakedToolMarkup(text)",
	}
	for _, marker := range requiredMessageMarkers {
		if !strings.Contains(msg, marker) {
			t.Fatalf("%s is missing expected regression marker %q", messagesPath, marker)
		}
	}

	requiredStreamingMarkers := []string{
		"stripLeakedToolMarkup(payload.content)",
		"stripLeakedToolMarkup(thinkingText)",
	}
	for _, marker := range requiredStreamingMarkers {
		if !strings.Contains(stream, marker) {
			t.Fatalf("%s is missing expected regression marker %q", streamingPath, marker)
		}
	}
}
