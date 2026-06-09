package agent

import (
	"testing"

	"github.com/sashabaranov/go-openai"
)

func TestShouldAppendHistoryMessage(t *testing.T) {
	tests := []struct {
		name string
		id   int64
		err  error
		want bool
	}{
		{name: "success", id: 42, err: nil, want: true},
		{name: "insert error", id: -1, err: errHistorySyncTest, want: false},
		{name: "invalid id", id: 0, err: nil, want: false},
		{name: "negative id", id: -1, err: nil, want: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := ShouldAppendHistoryMessage(tc.id, tc.err); got != tc.want {
				t.Fatalf("ShouldAppendHistoryMessage(%d, err=%v) = %v, want %v", tc.id, tc.err, got, tc.want)
			}
		})
	}
}

func TestNativeToolCallHistoryMessage_WithNativeCallID(t *testing.T) {
	msg := NativeToolCallHistoryMessage(ToolCall{
		Action:       "query_memory",
		NativeCallID: "call_abc",
		Params:       map[string]interface{}{"query": "nas backup"},
	}, `{"action": "query_memory"}`)

	if msg.Role != openai.ChatMessageRoleAssistant {
		t.Fatalf("role = %q, want assistant", msg.Role)
	}
	if len(msg.ToolCalls) != 1 {
		t.Fatalf("tool calls = %d, want 1", len(msg.ToolCalls))
	}
	if msg.ToolCalls[0].ID != "call_abc" {
		t.Fatalf("tool call id = %q, want call_abc", msg.ToolCalls[0].ID)
	}
	if msg.ToolCalls[0].Function.Name != "query_memory" {
		t.Fatalf("function name = %q, want query_memory", msg.ToolCalls[0].Function.Name)
	}
	if msg.ToolCalls[0].Function.Arguments == "" {
		t.Fatal("expected function arguments")
	}
}

func TestNativeToolCallHistoryMessage_WithoutNativeCallID(t *testing.T) {
	msg := NativeToolCallHistoryMessage(ToolCall{Action: "shell"}, `{"action":"shell"}`)
	if msg.Content != `{"action":"shell"}` {
		t.Fatalf("content = %q", msg.Content)
	}
	if len(msg.ToolCalls) != 0 {
		t.Fatalf("expected no tool calls, got %d", len(msg.ToolCalls))
	}
}

var errHistorySyncTest = &historySyncTestError{}

type historySyncTestError struct{}

func (historySyncTestError) Error() string { return "insert failed" }