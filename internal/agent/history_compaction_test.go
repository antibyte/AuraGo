package agent

import (
	"strings"
	"testing"

	"github.com/sashabaranov/go-openai"
)

func TestCompactHistoryToolRoundsKeepsLastTwoFullAndSummarizesOlderRound(t *testing.T) {
	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: "sys"},
		{Role: openai.ChatMessageRoleUser, Content: "inspect project"},
		nativeToolCallMessage("call-1", "execute_shell", `{"command":"go test ./..."}`),
		{Role: openai.ChatMessageRoleTool, ToolCallID: "call-1", Content: "Tool Output: {\"status\":\"success\",\"output_ref\":\"toolout_old\"}\ninternal/agent/foo.go:12 ok\nexit_code=0"},
		{Role: openai.ChatMessageRoleUser, Content: "continue"},
		nativeToolCallMessage("call-2", "filesystem", `{"operation":"read_file","path":"internal/agent/bar.go"}`),
		{Role: openai.ChatMessageRoleTool, ToolCallID: "call-2", Content: "Tool Output: bar.go contents"},
		nativeToolCallMessage("call-3", "execute_shell", `{"command":"go test ./internal/agent"}`),
		{Role: openai.ChatMessageRoleTool, ToolCallID: "call-3", Content: "Tool Output: ok"},
	}

	compacted, result := CompactHistoryToolRounds(messages, HistoryCompactionOptions{KeepRecentToolRoundsFull: 2})
	if !result.Compacted {
		t.Fatal("expected compaction")
	}
	if result.RoundsCompacted != 1 {
		t.Fatalf("RoundsCompacted = %d, want 1", result.RoundsCompacted)
	}
	if countNativeToolResult(compacted, "call-1") != 0 {
		t.Fatalf("old tool result should be summarized, got messages: %+v", compacted)
	}
	if countNativeToolResult(compacted, "call-2") != 1 || countNativeToolResult(compacted, "call-3") != 1 {
		t.Fatalf("recent tool rounds should remain full, got messages: %+v", compacted)
	}
	summary := findTaskStateSummary(compacted)
	for _, want := range []string{"TaskStateSummary", "execute_shell", "call-1", "toolout_old", "internal/agent/foo.go:12", "exit_code=0"} {
		if !strings.Contains(summary, want) {
			t.Fatalf("summary missing %q:\n%s", want, summary)
		}
	}
}

func TestCompactHistoryToolRoundsDoesNotSplitIncompleteNativeRound(t *testing.T) {
	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: "sys"},
		{Role: openai.ChatMessageRoleUser, Content: "run two tools"},
		{Role: openai.ChatMessageRoleAssistant, ToolCalls: []openai.ToolCall{
			{
				ID:       "call-a",
				Type:     openai.ToolTypeFunction,
				Function: openai.FunctionCall{Name: "execute_shell", Arguments: `{"command":"pwd"}`},
			},
			{
				ID:       "call-b",
				Type:     openai.ToolTypeFunction,
				Function: openai.FunctionCall{Name: "filesystem", Arguments: `{"operation":"list"}`},
			},
		}},
		{Role: openai.ChatMessageRoleTool, ToolCallID: "call-a", Content: "Tool Output: /tmp"},
	}

	compacted, result := CompactHistoryToolRounds(messages, HistoryCompactionOptions{KeepRecentToolRoundsFull: 0})
	if result.Compacted {
		t.Fatalf("incomplete tool round must not be compacted: %+v", result)
	}
	if len(compacted) != len(messages) {
		t.Fatalf("len(compacted) = %d, want %d", len(compacted), len(messages))
	}
	if countNativeToolResult(compacted, "call-a") != 1 {
		t.Fatalf("existing partial tool result should remain untouched")
	}
}

func nativeToolCallMessage(id, name, args string) openai.ChatCompletionMessage {
	return openai.ChatCompletionMessage{
		Role: openai.ChatMessageRoleAssistant,
		ToolCalls: []openai.ToolCall{{
			ID:       id,
			Type:     openai.ToolTypeFunction,
			Function: openai.FunctionCall{Name: name, Arguments: args},
		}},
	}
}

func countNativeToolResult(messages []openai.ChatCompletionMessage, id string) int {
	count := 0
	for _, msg := range messages {
		if msg.Role == openai.ChatMessageRoleTool && msg.ToolCallID == id {
			count++
		}
	}
	return count
}

func findTaskStateSummary(messages []openai.ChatCompletionMessage) string {
	for _, msg := range messages {
		if strings.Contains(msg.Content, "[TaskStateSummary]") {
			return msg.Content
		}
	}
	return ""
}
