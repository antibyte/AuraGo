package agent

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/sashabaranov/go-openai"
)

// mockChatClient implements llm.ChatClient for testing.
type mockChatClient struct {
	response string
	err      error
	lastReq  openai.ChatCompletionRequest
	calls    int
}

func (m *mockChatClient) CreateChatCompletion(_ context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	m.lastReq = req
	m.calls++
	if m.err != nil {
		return openai.ChatCompletionResponse{}, m.err
	}
	return openai.ChatCompletionResponse{
		Choices: []openai.ChatCompletionChoice{
			{Message: openai.ChatCompletionMessage{Content: m.response}},
		},
	}, nil
}

func TestCompressHistoryBuildsValidUTF8SummaryPrompt(t *testing.T) {
	messages := make([]openai.ChatCompletionMessage, 0, 20)
	messages = append(messages, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleSystem, Content: "System prompt"})
	for i := 0; i < 15; i++ {
		messages = append(messages, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleUser,
			Content: strings.Repeat("界", 300),
		})
	}

	client := &mockChatClient{response: "Compressed summary"}
	_, _, res := CompressHistory(context.Background(), messages, 50, "test", client, 0, testLogger)

	if !res.Compressed {
		t.Fatal("expected compression to occur")
	}
	if len(client.lastReq.Messages) != 1 {
		t.Fatalf("expected one summary request message, got %d", len(client.lastReq.Messages))
	}
	prompt := client.lastReq.Messages[0].Content
	if !utf8.ValidString(prompt) {
		t.Fatalf("expected valid UTF-8 prompt, got %q", prompt)
	}
}

func (m *mockChatClient) CreateChatCompletionStream(_ context.Context, _ openai.ChatCompletionRequest) (*openai.ChatCompletionStream, error) {
	return nil, nil
}

var testLogger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

func TestCompressHistory_BelowThreshold(t *testing.T) {
	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: "You are a helpful assistant."},
		{Role: openai.ChatMessageRoleUser, Content: "Hello"},
		{Role: openai.ChatMessageRoleAssistant, Content: "Hi there!"},
	}

	client := &mockChatClient{response: "Summary"}
	result, lastComp, res := CompressHistory(context.Background(), messages, 100000, "test", client, 0, testLogger)

	if res.Compressed {
		t.Error("Should not compress when below threshold")
	}
	if len(result) != len(messages) {
		t.Error("Messages should be unchanged")
	}
	if lastComp != 0 {
		t.Error("lastCompressionMsg should be unchanged")
	}
}

func TestCompressHistory_TooFewMessages(t *testing.T) {
	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: "System"},
		{Role: openai.ChatMessageRoleUser, Content: "Hello"},
	}

	client := &mockChatClient{response: "Summary"}
	_, _, res := CompressHistory(context.Background(), messages, 10, "test", client, 0, testLogger)

	if res.Compressed {
		t.Error("Should not compress with too few messages")
	}
}

func TestCompressHistory_CooldownPrevention(t *testing.T) {
	// Build enough messages to trigger compression but set lastCompressionMsg close to current count
	messages := make([]openai.ChatCompletionMessage, 0, 15)
	messages = append(messages, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleSystem, Content: "System"})
	for i := 0; i < 14; i++ {
		messages = append(messages, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleUser,
			Content: strings.Repeat("Token filler text for testing. ", 50),
		})
	}

	client := &mockChatClient{response: "Compressed summary"}
	// Set lastCompressionMsg = len(messages)-2 so cooldown prevents compression
	_, _, res := CompressHistory(context.Background(), messages, 100, "test", client, len(messages)-2, testLogger)

	if res.Compressed {
		t.Error("Should not compress within cooldown period")
	}
}

func TestCompressHistory_Succeeds(t *testing.T) {
	// Build a conversation that exceeds the threshold
	messages := make([]openai.ChatCompletionMessage, 0, 20)
	messages = append(messages, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleSystem, Content: "System prompt"})
	for i := 0; i < 15; i++ {
		messages = append(messages, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleUser,
			Content: strings.Repeat("This is a long message that takes up tokens. ", 20),
		})
	}

	client := &mockChatClient{response: "This is a compressed summary of the conversation."}
	// Use a tiny maxHistoryTokens to force compression
	result, lastComp, res := CompressHistory(context.Background(), messages, 50, "test", client, 0, testLogger)

	if !res.Compressed {
		t.Fatal("Expected compression to occur")
	}
	if res.DroppedCount == 0 {
		t.Error("Expected some messages to be dropped")
	}
	if lastComp == 0 {
		t.Error("lastCompressionMsg should be updated")
	}

	// Check that the summary message is present
	foundSummary := false
	for _, m := range result {
		if strings.Contains(m.Content, "[CONVERSATION SUMMARY]") {
			foundSummary = true
			break
		}
	}
	if !foundSummary {
		t.Error("Expected [CONVERSATION SUMMARY] message in result")
	}

	// System prompt should be preserved
	if result[0].Content != "System prompt" {
		t.Error("System prompt should be preserved as first message")
	}

	// Result should be shorter
	if len(result) >= len(messages) {
		t.Errorf("Result should have fewer messages: got %d, had %d", len(result), len(messages))
	}
}

func TestCompressHistory_LLMFailureFallback(t *testing.T) {
	messages := make([]openai.ChatCompletionMessage, 0, 20)
	messages = append(messages, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleSystem, Content: "System"})
	for i := 0; i < 15; i++ {
		messages = append(messages, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleUser,
			Content: strings.Repeat("Filler text for token count. ", 20),
		})
	}

	client := &mockChatClient{err: context.DeadlineExceeded}
	result, _, res := CompressHistory(context.Background(), messages, 50, "test", client, 0, testLogger)

	if res.Compressed {
		t.Error("Should not compress when LLM fails")
	}
	if len(result) != len(messages) {
		t.Error("Messages should be unchanged on LLM failure")
	}
}

func TestIsToolError(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected bool
	}{
		{"Success JSON", `{"status": "success", "result": "ok"}`, false},
		{"Error JSON spaced", `{"status": "error", "message": "not found"}`, true},
		{"Error JSON compact", `{"status":"error","message":"fail"}`, true},
		{"Execution error", `[EXECUTION ERROR] something went wrong`, true},
		{"Exit code 0", `{"exit_code": 0, "output": "ok"}`, false},
		{"Exit code 1", `{"exit_code": 1, "output": "fail"}`, true},
		{"Plain text", `Tool ran successfully`, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isToolError(tt.content)
			if got != tt.expected {
				t.Errorf("isToolError(%q) = %v, want %v", tt.name, got, tt.expected)
			}
		})
	}
}
