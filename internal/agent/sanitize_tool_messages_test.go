package agent

import (
	"testing"

	"github.com/sashabaranov/go-openai"
)

func TestSanitizeToolMessages_Empty(t *testing.T) {
	msgs := []openai.ChatCompletionMessage{}
	out, dropped := SanitizeToolMessages(msgs)
	if dropped != 0 {
		t.Fatalf("expected 0 dropped, got %d", dropped)
	}
	if len(out) != 0 {
		t.Fatalf("expected 0 messages, got %d", len(out))
	}
}

func TestSanitizeToolMessages_NoToolMessages(t *testing.T) {
	msgs := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: "system"},
		{Role: openai.ChatMessageRoleUser, Content: "hello"},
		{Role: openai.ChatMessageRoleAssistant, Content: "hi"},
	}
	out, dropped := SanitizeToolMessages(msgs)
	if dropped != 0 {
		t.Fatalf("expected 0 dropped, got %d", dropped)
	}
	if len(out) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(out))
	}
}

func TestSanitizeToolMessages_ValidToolRound(t *testing.T) {
	msgs := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: "system"},
		{Role: openai.ChatMessageRoleUser, Content: "do something"},
		{Role: openai.ChatMessageRoleAssistant, ToolCalls: []openai.ToolCall{
			{ID: "call_123", Function: openai.FunctionCall{Name: "test_tool"}},
		}},
		{Role: openai.ChatMessageRoleTool, ToolCallID: "call_123", Content: "result"},
		{Role: openai.ChatMessageRoleAssistant, Content: "done"},
	}
	out, dropped := SanitizeToolMessages(msgs)
	if dropped != 0 {
		t.Fatalf("expected 0 dropped, got %d", dropped)
	}
	if len(out) != 5 {
		t.Fatalf("expected 5 messages, got %d", len(out))
	}
}

func TestSanitizeToolMessages_OrphanedToolResult(t *testing.T) {
	msgs := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: "system"},
		{Role: openai.ChatMessageRoleUser, Content: "hello"},
		{Role: openai.ChatMessageRoleTool, ToolCallID: "call_orphan", Content: "orphan result"},
		{Role: openai.ChatMessageRoleAssistant, Content: "hi"},
	}
	out, dropped := SanitizeToolMessages(msgs)
	if dropped != 1 {
		t.Fatalf("expected 1 dropped, got %d", dropped)
	}
	if len(out) != 3 {
		t.Fatalf("expected 3 messages (system, user, assistant), got %d", len(out))
	}
	for _, m := range out {
		if m.Role == openai.ChatMessageRoleTool {
			t.Fatal("tool message should have been removed")
		}
	}
}

func TestSanitizeToolMessages_EmptyToolCallID(t *testing.T) {
	msgs := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: "system"},
		{Role: openai.ChatMessageRoleUser, Content: "hello"},
		{Role: openai.ChatMessageRoleTool, ToolCallID: "", Content: "empty id result"},
		{Role: openai.ChatMessageRoleAssistant, Content: "hi"},
	}
	out, dropped := SanitizeToolMessages(msgs)
	if dropped != 1 {
		t.Fatalf("expected 1 dropped, got %d", dropped)
	}
	if len(out) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(out))
	}
}

func TestSanitizeToolMessages_MismatchedToolCallID(t *testing.T) {
	msgs := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: "system"},
		{Role: openai.ChatMessageRoleUser, Content: "hello"},
		{Role: openai.ChatMessageRoleAssistant, ToolCalls: []openai.ToolCall{
			{ID: "call_abc", Function: openai.FunctionCall{Name: "tool1"}},
		}},
		{Role: openai.ChatMessageRoleTool, ToolCallID: "call_xyz", Content: "wrong id"},
		{Role: openai.ChatMessageRoleAssistant, Content: "hi"},
	}
	out, dropped := SanitizeToolMessages(msgs)
	// Both the orphaned tool result (call_xyz) and the assistant with
	// unconsumed tool call (call_abc, no matching result) are dropped.
	if dropped != 2 {
		t.Fatalf("expected 2 dropped, got %d", dropped)
	}
	// No tool messages should remain
	for _, m := range out {
		if m.Role == openai.ChatMessageRoleTool {
			t.Fatal("tool message should have been removed")
		}
	}
}

func TestSanitizeToolMessages_UnmatchedToolCallStripped(t *testing.T) {
	// Assistant has content AND orphaned tool calls (no matching results).
	// The ToolCalls are stripped but the message is kept because it has content.
	msgs := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: "system"},
		{Role: openai.ChatMessageRoleUser, Content: "hello"},
		{Role: openai.ChatMessageRoleAssistant, ToolCalls: []openai.ToolCall{
			{ID: "call_abc", Function: openai.FunctionCall{Name: "tool1"}},
		}, Content: "I will call a tool"},
		// No tool result follows — the tool call is orphaned
		{Role: openai.ChatMessageRoleUser, Content: "next message"},
	}
	out, dropped := SanitizeToolMessages(msgs)
	if dropped != 0 {
		t.Fatalf("expected 0 dropped (assistant has content), got %d", dropped)
	}
	// The assistant's ToolCalls should be stripped but content preserved
	found := false
	for _, m := range out {
		if m.Role == openai.ChatMessageRoleAssistant {
			found = true
			if len(m.ToolCalls) != 0 {
				t.Fatal("orphaned tool calls should have been stripped from assistant")
			}
			if m.Content != "I will call a tool" {
				t.Fatalf("assistant content should be preserved, got %q", m.Content)
			}
		}
	}
	if !found {
		t.Fatal("assistant message should have been kept (has content)")
	}
}

func TestSanitizeToolMessages_UnmatchedToolCallDroppedNoContent(t *testing.T) {
	// Assistant has NO content and orphaned tool calls → entire message dropped.
	msgs := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: "system"},
		{Role: openai.ChatMessageRoleUser, Content: "hello"},
		{Role: openai.ChatMessageRoleAssistant, ToolCalls: []openai.ToolCall{
			{ID: "call_abc", Function: openai.FunctionCall{Name: "tool1"}},
		}}, // Content is empty (zero value)
		{Role: openai.ChatMessageRoleUser, Content: "next message"},
	}
	out, dropped := SanitizeToolMessages(msgs)
	if dropped != 1 {
		t.Fatalf("expected 1 dropped (assistant with no content), got %d", dropped)
	}
	if len(out) != 3 {
		t.Fatalf("expected 3 messages (system, user, user), got %d", len(out))
	}
	for _, m := range out {
		if m.Role == openai.ChatMessageRoleAssistant {
			t.Fatal("assistant with no content and orphaned tool calls should have been dropped")
		}
	}
}

func TestSanitizeToolMessages_UnmatchedToolCallKeepsContent(t *testing.T) {
	msgs := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: "system"},
		{Role: openai.ChatMessageRoleUser, Content: "hello"},
		{Role: openai.ChatMessageRoleAssistant, ToolCalls: []openai.ToolCall{
			{ID: "call_abc", Function: openai.FunctionCall{Name: "tool1"}},
		}, Content: "I have content"},
		// No tool result follows — the tool call is orphaned
		{Role: openai.ChatMessageRoleUser, Content: "next message"},
	}
	out, dropped := SanitizeToolMessages(msgs)
	if dropped != 0 {
		// The assistant message has content, so it's kept but ToolCalls are stripped
		t.Fatalf("expected 0 dropped, got %d", dropped)
	}
	// Find the assistant message and verify ToolCalls were stripped
	for _, m := range out {
		if m.Role == openai.ChatMessageRoleAssistant {
			if len(m.ToolCalls) != 0 {
				t.Fatal("orphaned tool calls should have been stripped from assistant with content")
			}
			if m.Content != "I have content" {
				t.Fatal("assistant content should be preserved")
			}
		}
	}
}

func TestSanitizeToolMessages_MultipleOrphans(t *testing.T) {
	msgs := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: "system"},
		{Role: openai.ChatMessageRoleUser, Content: "hello"},
		{Role: openai.ChatMessageRoleTool, ToolCallID: "orphan1", Content: "orphan1"},
		{Role: openai.ChatMessageRoleTool, ToolCallID: "orphan2", Content: "orphan2"},
		{Role: openai.ChatMessageRoleTool, ToolCallID: "", Content: "empty"},
		{Role: openai.ChatMessageRoleAssistant, Content: "hi"},
	}
	out, dropped := SanitizeToolMessages(msgs)
	if dropped != 3 {
		t.Fatalf("expected 3 dropped, got %d", dropped)
	}
	if len(out) != 3 {
		t.Fatalf("expected 3 messages (system, user, assistant), got %d", len(out))
	}
}

func TestSanitizeToolMessages_PartialMatch(t *testing.T) {
	// Assistant calls 2 tools, but only 1 result exists. The consumed tool
	// call (call_abc) is kept, the orphaned one (call_def) is stripped from
	// ToolCalls. No messages are dropped, only a ToolCall is removed.
	msgs := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: "system"},
		{Role: openai.ChatMessageRoleUser, Content: "hello"},
		{Role: openai.ChatMessageRoleAssistant, ToolCalls: []openai.ToolCall{
			{ID: "call_abc", Function: openai.FunctionCall{Name: "tool1"}},
			{ID: "call_def", Function: openai.FunctionCall{Name: "tool2"}},
		}, Content: ""},
		{Role: openai.ChatMessageRoleTool, ToolCallID: "call_abc", Content: "result1"},
		// call_def has no result
		{Role: openai.ChatMessageRoleUser, Content: "next"},
	}
	out, dropped := SanitizeToolMessages(msgs)
	// No messages dropped — only call_def is stripped from ToolCalls
	if dropped != 0 {
		t.Fatalf("expected 0 dropped, got %d", dropped)
	}
	// The assistant should have only 1 ToolCall (call_abc)
	for _, m := range out {
		if m.Role == openai.ChatMessageRoleAssistant {
			if len(m.ToolCalls) != 1 {
				t.Fatalf("expected 1 ToolCall, got %d", len(m.ToolCalls))
			}
			if m.ToolCalls[0].ID != "call_abc" {
				t.Fatalf("expected ToolCall ID call_abc, got %s", m.ToolCalls[0].ID)
			}
		}
	}
}
