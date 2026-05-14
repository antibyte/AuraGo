package agent

import (
	"strings"
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
		{name: "space agent bridge source", runCfg: RunConfig{MessageSource: "space_agent_bridge"}, sessionID: "default", want: true},
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

func TestBuildLoopbackSessionConversationMessagesIncludesVirtualDesktopContext(t *testing.T) {
	sessionMessages := []memory.HistoryMessage{
		{ChatCompletionMessage: openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: "MUSIK nicht sound"}},
		{ChatCompletionMessage: openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, Content: "Sag mir kurz noch den Style."}},
		{ChatCompletionMessage: openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: "retro arcade"}},
	}

	messages := buildLoopbackSessionConversationMessages(
		[]openai.ChatCompletionMessage{{Role: openai.ChatMessageRoleSystem, Content: "system"}},
		sessionMessages,
		"retro arcade",
	)

	if len(messages) != 4 {
		t.Fatalf("message count = %d, want 4: %#v", len(messages), messages)
	}
	if messages[1].Content != "MUSIK nicht sound" || messages[2].Content != "Sag mir kurz noch den Style." {
		t.Fatalf("prior desktop context was not preserved: %#v", messages)
	}
	countCurrent := 0
	for _, msg := range messages {
		if msg.Role == openai.ChatMessageRoleUser && msg.Content == "retro arcade" {
			countCurrent++
		}
	}
	if countCurrent != 1 {
		t.Fatalf("current desktop prompt count = %d, want 1: %#v", countCurrent, messages)
	}
}

func TestBuildLoopbackSessionConversationMessagesLimitsLongDesktopContext(t *testing.T) {
	sessionMessages := make([]memory.HistoryMessage, 0, 80)
	for i := 0; i < 79; i++ {
		sessionMessages = append(sessionMessages, memory.HistoryMessage{
			ChatCompletionMessage: openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, Content: strings.Repeat("old desktop output ", 200)},
		})
	}
	sessionMessages = append(sessionMessages, memory.HistoryMessage{
		ChatCompletionMessage: openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: "retro arcade"},
	})

	messages := buildLoopbackSessionConversationMessages(
		[]openai.ChatCompletionMessage{{Role: openai.ChatMessageRoleSystem, Content: "system"}},
		sessionMessages,
		"retro arcade",
	)

	if len(messages) > 1+loopbackSessionMaxMessages {
		t.Fatalf("message count = %d, want at most %d", len(messages), 1+loopbackSessionMaxMessages)
	}
	if got := messages[len(messages)-1].Content; got != "retro arcade" {
		t.Fatalf("latest desktop prompt = %q, want current request", got)
	}
	for _, msg := range messages[1:] {
		if msg.Content == strings.Repeat("old desktop output ", 200) && len(messages) == len(sessionMessages)+1 {
			t.Fatal("expected old desktop transcript to be trimmed")
		}
	}
}
