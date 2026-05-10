package ui

import (
	"strings"
	"testing"
)

func TestDesktopChatShowsDetailedAgentActionStatus(t *testing.T) {
	t.Parallel()

	renderer := readDesktopAssetText(t, "js/desktop/chat-renderer.js")
	for _, marker := range []string{
		"formatAgentActionStatus(data)",
		"extractToolCallNarration(text)",
		"chat.sse_tool_start",
		"chat.sse_tool_end",
		"chat.sse_coding",
		"chat.sse_error_recovery",
		"chat.sse_co_agent_spawn",
		"chat.sse_workflow_plan",
	} {
		if !strings.Contains(renderer, marker) {
			t.Fatalf("desktop chat renderer missing detailed action status marker %q", marker)
		}
	}

	chat := readDesktopAssetText(t, "js/desktop/apps/quickconnect-launchpad-chat.js")
	for _, marker := range []string{
		"renderer.formatAgentActionStatus(data)",
		"renderer.extractToolCallNarration(data.detail || data.message || '')",
		"event === 'tool_call'",
		"event === 'co_agent_spawn'",
		"event === 'workflow_plan'",
		"event === 'coding'",
		"event === 'error_recovery'",
	} {
		if !strings.Contains(chat, marker) {
			t.Fatalf("desktop chat stream missing detailed action handling marker %q", marker)
		}
	}
}
