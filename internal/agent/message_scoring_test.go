package agent

import (
	"testing"

	"github.com/sashabaranov/go-openai"
)

func TestScoreMessage_System(t *testing.T) {
	msg := openai.ChatCompletionMessage{Role: openai.ChatMessageRoleSystem, Content: "You are helpful"}
	score, reason := ScoreMessage(msg, nil)
	if score != ImportanceCritical {
		t.Errorf("system message score = %d, want %d", score, ImportanceCritical)
	}
	if reason != "system" {
		t.Errorf("reason = %q, want system", reason)
	}
}

func TestScoreMessage_UserAck(t *testing.T) {
	msg := openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: "ok"}
	score, reason := ScoreMessage(msg, nil)
	if score != ImportanceLow {
		t.Errorf("short ack score = %d, want %d", score, ImportanceLow)
	}
	if reason != "short_ack" {
		t.Errorf("reason = %q, want short_ack", reason)
	}
}

func TestScoreMessage_UserQuestion(t *testing.T) {
	msg := openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: "How do I restart the docker container?"}
	score, reason := ScoreMessage(msg, nil)
	if score != ImportanceHigh {
		t.Errorf("user question score = %d, want %d", score, ImportanceHigh)
	}
	if reason != "user_intent" {
		t.Errorf("reason = %q, want user_intent", reason)
	}
}

func TestScoreMessage_AssistantToolCalls(t *testing.T) {
	msg := openai.ChatCompletionMessage{
		Role: openai.ChatMessageRoleAssistant,
		ToolCalls: []openai.ToolCall{
			{ID: "call_1", Function: openai.FunctionCall{Name: "docker"}},
		},
	}
	score, reason := ScoreMessage(msg, nil)
	if score != ImportanceMedium {
		t.Errorf("assistant tool_calls score = %d, want %d", score, ImportanceMedium)
	}
	if reason != "tool_calls" {
		t.Errorf("reason = %q, want tool_calls", reason)
	}
}

func TestScoreMessage_AssistantPlan(t *testing.T) {
	msg := openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, Content: "Plan: first check docker ps, then restart the container."}
	score, reason := ScoreMessage(msg, nil)
	if score != ImportanceHigh {
		t.Errorf("assistant plan score = %d, want %d", score, ImportanceHigh)
	}
	if reason != "plan" {
		t.Errorf("reason = %q, want plan", reason)
	}
}

func TestScoreMessage_ToolError(t *testing.T) {
	msg := openai.ChatCompletionMessage{Role: openai.ChatMessageRoleTool, Content: "Error: container not found"}
	prev := &openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, ToolCalls: []openai.ToolCall{{Function: openai.FunctionCall{Name: "docker"}}}}
	score, reason := ScoreMessage(msg, prev)
	if score != ImportanceHigh {
		t.Errorf("tool error score = %d, want %d", score, ImportanceHigh)
	}
	if reason != "tool_error" {
		t.Errorf("reason = %q, want tool_error", reason)
	}
}

func TestScoreMessage_ToolUtility(t *testing.T) {
	msg := openai.ChatCompletionMessage{Role: openai.ChatMessageRoleTool, Content: "/home/user"}
	prev := &openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, ToolCalls: []openai.ToolCall{{Function: openai.FunctionCall{Name: "execute_shell"}}}}
	score, reason := ScoreMessage(msg, prev)
	if score != ImportanceLow {
		t.Errorf("utility output score = %d, want %d", score, ImportanceLow)
	}
	if reason != "utility_output" {
		t.Errorf("reason = %q, want utility_output", reason)
	}
}

func TestBuildToolCallGroups(t *testing.T) {
	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: "sys"},
		{Role: openai.ChatMessageRoleAssistant, Content: "call tools", ToolCalls: []openai.ToolCall{
			{ID: "call_1", Function: openai.FunctionCall{Name: "docker"}},
			{ID: "call_2", Function: openai.FunctionCall{Name: "shell"}},
		}},
		{Role: openai.ChatMessageRoleTool, Content: "result1", ToolCallID: "call_1"},
		{Role: openai.ChatMessageRoleTool, Content: "result2", ToolCallID: "call_2"},
		{Role: openai.ChatMessageRoleUser, Content: "ok"},
	}
	groups := buildToolCallGroups(messages)
	if len(groups) != 3 {
		t.Fatalf("expected 3 grouped messages, got %d", len(groups))
	}
	if groups[1] != 1 {
		t.Errorf("assistant message leader = %d, want 1", groups[1])
	}
	if groups[2] != 1 {
		t.Errorf("tool result 1 leader = %d, want 1", groups[2])
	}
	if groups[3] != 1 {
		t.Errorf("tool result 2 leader = %d, want 1", groups[3])
	}
}

func TestScoreBasedTrimming(t *testing.T) {
	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: "sys"},
		{Role: openai.ChatMessageRoleUser, Content: "question"},          // score High, idx=1
		{Role: openai.ChatMessageRoleAssistant, Content: "ok"},           // score Low, idx=2
		{Role: openai.ChatMessageRoleAssistant, Content: "ack"},          // score Low, idx=3
		{Role: openai.ChatMessageRoleUser, Content: "another question"},  // score High, idx=4
		{Role: openai.ChatMessageRoleAssistant, Content: "response"},     // score Medium, idx=5
		{Role: openai.ChatMessageRoleUser, Content: "q3"},                // score High, idx=6
		{Role: openai.ChatMessageRoleAssistant, Content: "a3"},           // score Medium, idx=7
		{Role: openai.ChatMessageRoleUser, Content: "latest"},            // score High, idx=8
	}

	tokenFn := func(s string) int { return len(s) }

	// Simulate being over budget by setting maxHistoryTokens lower than current tokens
	totalTokens := 0
	for _, m := range messages {
		totalTokens += tokenFn(messageText(m)) + 4
	}
	maxHistoryTokens := totalTokens - 12 // Force trimming of only low-score messages

	result, dropped, newTokens := scoreBasedTrimming(messages, maxHistoryTokens, totalTokens, tokenFn, nil)

	if len(dropped) == 0 {
		t.Fatal("expected some messages to be dropped")
	}

	// The lowest-score messages should be dropped first.
	// Index 2 (assistant "ok", Low) and index 3 (assistant "ack", Low) should be dropped.
	droppedMap := make(map[int]bool)
	for _, d := range dropped {
		droppedMap[d] = true
	}
	if !droppedMap[2] {
		t.Error("expected assistant ack (idx=2, Low) to be dropped")
	}
	if !droppedMap[3] {
		t.Error("expected assistant ack (idx=3, Low) to be dropped")
	}
	if droppedMap[1] {
		t.Error("user question (idx=1, High) should NOT be dropped")
	}
	if droppedMap[8] {
		t.Error("latest user message (idx=8, High) should NOT be dropped")
	}
	if newTokens >= totalTokens {
		t.Error("expected token count to decrease after trimming")
	}
	_ = result
}

func TestScoreBasedTrimming_PreservesToolCallGroups(t *testing.T) {
	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: "sys"},
		{Role: openai.ChatMessageRoleUser, Content: "q1"},
		{Role: openai.ChatMessageRoleAssistant, Content: "a1"},
		{Role: openai.ChatMessageRoleUser, Content: "ok"}, // Low score, candidate for drop
		{Role: openai.ChatMessageRoleUser, Content: "q2"},
		{Role: openai.ChatMessageRoleAssistant, Content: "call", ToolCalls: []openai.ToolCall{
			{ID: "call_1", Function: openai.FunctionCall{Name: "docker"}},
		}},
		{Role: openai.ChatMessageRoleTool, Content: "error!", ToolCallID: "call_1"},
		{Role: openai.ChatMessageRoleUser, Content: "q3"},
		{Role: openai.ChatMessageRoleAssistant, Content: "response"},
		{Role: openai.ChatMessageRoleUser, Content: "latest"},
	}

	tokenFn := func(s string) int { return 10 }
	totalTokens := len(messages) * 14
	maxHistoryTokens := totalTokens - 25

	result, dropped, _ := scoreBasedTrimming(messages, maxHistoryTokens, totalTokens, tokenFn, nil)

	// The tool-call group (indices 5-6) has max score High because of the error.
	// The user "ok" (index 3) is Low and is in the candidate region.
	// So index 3 should be dropped, not the group.
	droppedMap := make(map[int]bool)
	for _, d := range dropped {
		droppedMap[d] = true
	}
	if droppedMap[5] || droppedMap[6] {
		t.Error("tool-call group with error should be preserved")
	}
	if !droppedMap[3] {
		t.Error("low-score user ack (idx=3) should be dropped before high-score tool group")
	}
	_ = result
}

func TestScoreBasedTrimming_NothingToDrop(t *testing.T) {
	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: "sys"},
		{Role: openai.ChatMessageRoleUser, Content: "q1"},
		{Role: openai.ChatMessageRoleAssistant, Content: "a1"},
	}
	tokenFn := func(s string) int { return 10 }
	result, dropped, _ := scoreBasedTrimming(messages, 1000, 50, tokenFn, nil)
	if len(dropped) != 0 {
		t.Error("expected no messages to be dropped when under budget")
	}
	if len(result) != len(messages) {
		t.Error("expected unchanged message list")
	}
}

func TestContainsPlanningMarker(t *testing.T) {
	cases := []struct {
		input string
		want  bool
	}{
		{"plan: do x then y", true},
		{"i will restart the service", true},
		{"decision: use postgres", true},
		{"let me check that", true},
		{"approach: incremental", true},
		{"strategy: blue-green", true},
		{"next steps are clear", true},
		{"i'll handle it", true},
		{"i shall investigate", true},
		{"normal response", false},
	}
	for _, c := range cases {
		got := containsPlanningMarker(c.input)
		if got != c.want {
			t.Errorf("containsPlanningMarker(%q) = %v, want %v", c.input, got, c.want)
		}
	}
}
