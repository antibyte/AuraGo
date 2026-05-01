package agent

import (
	"testing"

	"aurago/internal/memory"

	"github.com/sashabaranov/go-openai"
)

func TestShouldPersistLoopbackHistory(t *testing.T) {
	tests := []struct {
		name      string
		sessionID string
		want      bool
	}{
		{name: "default session", sessionID: "default", want: true},
		{name: "heartbeat session", sessionID: "heartbeat", want: false},
		{name: "custom chat session", sessionID: "sess-123", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldPersistLoopbackHistory(tt.sessionID); got != tt.want {
				t.Fatalf("shouldPersistLoopbackHistory(%q) = %v, want %v", tt.sessionID, got, tt.want)
			}
		})
	}
}

func TestIsAutonomousLoopback(t *testing.T) {
	tests := []struct {
		name      string
		runCfg    RunConfig
		sessionID string
		want      bool
	}{
		{name: "heartbeat source", runCfg: RunConfig{MessageSource: "heartbeat"}, sessionID: "default", want: true},
		{name: "heartbeat session", runCfg: RunConfig{}, sessionID: "heartbeat", want: true},
		{name: "planner notification source", runCfg: RunConfig{MessageSource: "planner_notification"}, sessionID: "default", want: true},
		{name: "uptime kuma source", runCfg: RunConfig{MessageSource: "uptime_kuma"}, sessionID: "default", want: true},
		{name: "sms loopback remains visible", runCfg: RunConfig{MessageSource: "sms"}, sessionID: "default", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isAutonomousLoopback(tt.runCfg, tt.sessionID); got != tt.want {
				t.Fatalf("isAutonomousLoopback(%+v, %q) = %v, want %v", tt.runCfg, tt.sessionID, got, tt.want)
			}
		})
	}
}

func TestBuildLoopbackConversationMessagesHidesGlobalHistoryForHeartbeat(t *testing.T) {
	hm := memory.NewEphemeralHistoryManager()
	t.Cleanup(hm.Close)
	if err := hm.Add(openai.ChatMessageRoleUser, "old KI-News task from normal chat", 1, false, false); err != nil {
		t.Fatalf("add history: %v", err)
	}

	messages := buildLoopbackConversationMessages(
		[]openai.ChatCompletionMessage{{Role: openai.ChatMessageRoleSystem, Content: "system"}},
		hm,
		"[SYSTEM HEARTBEAT] check now",
		false,
	)

	if len(messages) != 2 {
		t.Fatalf("message count = %d, want 2: %#v", len(messages), messages)
	}
	if messages[1].Role != openai.ChatMessageRoleUser || messages[1].Content != "[SYSTEM HEARTBEAT] check now" {
		t.Fatalf("last message = (%q, %q), want heartbeat user prompt", messages[1].Role, messages[1].Content)
	}
	for _, msg := range messages {
		if msg.Content == "old KI-News task from normal chat" {
			t.Fatalf("heartbeat prompt included global chat history: %#v", messages)
		}
	}
}

func TestBuildLoopbackConversationMessagesDoesNotDuplicateCurrentDefaultMessage(t *testing.T) {
	hm := memory.NewEphemeralHistoryManager()
	t.Cleanup(hm.Close)
	if err := hm.Add(openai.ChatMessageRoleUser, "current SMS prompt", 1, false, false); err != nil {
		t.Fatalf("add history: %v", err)
	}

	messages := buildLoopbackConversationMessages(
		[]openai.ChatCompletionMessage{{Role: openai.ChatMessageRoleSystem, Content: "system"}},
		hm,
		"current SMS prompt",
		true,
	)

	count := 0
	for _, msg := range messages {
		if msg.Role == openai.ChatMessageRoleUser && msg.Content == "current SMS prompt" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("current prompt count = %d, want 1: %#v", count, messages)
	}
}
