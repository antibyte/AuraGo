package server

import (
	"testing"

	openai "github.com/sashabaranov/go-openai"

	"aurago/internal/agent"
	"aurago/internal/memory"
)

func TestRecentMessagesForMissionUseOnlyCurrentRequest(t *testing.T) {
	current := []openai.ChatCompletionMessage{{
		Role:    openai.ChatMessageRoleUser,
		Content: "current clean mission prompt",
	}}
	stored := []memory.HistoryMessage{
		{
			ChatCompletionMessage: openai.ChatCompletionMessage{
				Role:    openai.ChatMessageRoleUser,
				Content: "old mission prompt\n\n---\n## Mission Execution Plan (Advisory)\nstale",
			},
		},
		{
			ChatCompletionMessage: openai.ChatCompletionMessage{
				Role:    openai.ChatMessageRoleAssistant,
				Content: "old mission answer",
			},
		},
	}

	got := recentMessagesForRequest("mission-m1", "m1", current, nil, stored)

	if len(got) != 1 {
		t.Fatalf("mission execution should not reuse stored session history, got %d messages", len(got))
	}
	if got[0].Content != "current clean mission prompt" {
		t.Fatalf("mission execution used wrong prompt: %q", got[0].Content)
	}
}

func TestRecentMessagesForChatSessionStillUsesStoredContext(t *testing.T) {
	current := []openai.ChatCompletionMessage{{
		Role:    openai.ChatMessageRoleUser,
		Content: "new user prompt",
	}}
	stored := []memory.HistoryMessage{
		{
			ChatCompletionMessage: openai.ChatCompletionMessage{
				Role:    openai.ChatMessageRoleUser,
				Content: "previous visible prompt",
			},
		},
		{
			IsInternal: true,
			ChatCompletionMessage: openai.ChatCompletionMessage{
				Role:    openai.ChatMessageRoleTool,
				Content: "internal tool output",
			},
		},
		{
			ChatCompletionMessage: openai.ChatCompletionMessage{
				Role:    openai.ChatMessageRoleUser,
				Content: "new user prompt",
			},
		},
	}

	got := recentMessagesForRequest("sess-1", "", current, nil, stored)

	if len(got) != 2 {
		t.Fatalf("chat session should keep visible stored context, got %d messages", len(got))
	}
	if got[0].Content != "previous visible prompt" || got[1].Content != "new user prompt" {
		t.Fatalf("unexpected chat context: %#v", got)
	}
}

func TestInternalLoopbackRequestMessagesAreHidden(t *testing.T) {
	tests := []struct {
		name       string
		isFollowUp bool
		missionID  string
		want       bool
	}{
		{name: "visible user chat", isFollowUp: false, missionID: "", want: false},
		{name: "generic internal loopback", isFollowUp: true, missionID: "", want: true},
		{name: "mission request", isFollowUp: true, missionID: "mission-1", want: true},
		{name: "mission request without followup header remains hidden", isFollowUp: false, missionID: "mission-1", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := requestMessageIsInternal(tt.isFollowUp, tt.missionID); got != tt.want {
				t.Fatalf("requestMessageIsInternal(%v, %q) = %v, want %v", tt.isFollowUp, tt.missionID, got, tt.want)
			}
		})
	}
}

func TestMissionRequestsUseSilentFeedbackBroker(t *testing.T) {
	regular := feedbackBrokerForRequest(NewSSEBroadcaster(), "default", "")
	if _, ok := regular.(*SSEBrokerAdapter); !ok {
		t.Fatalf("regular chat broker = %T, want *SSEBrokerAdapter", regular)
	}

	mission := feedbackBrokerForRequest(NewSSEBroadcaster(), "mission-m1", "m1")
	if _, ok := mission.(agent.NoopBroker); !ok {
		t.Fatalf("mission broker = %T, want agent.NoopBroker", mission)
	}
}
