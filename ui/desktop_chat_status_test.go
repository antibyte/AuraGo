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
		"function keepAgentStatusAtEnd()",
		"chatLog.lastElementChild !== statusEl",
		"chatLog.appendChild(statusEl)",
	} {
		if !strings.Contains(chat, marker) {
			t.Fatalf("desktop chat stream missing detailed action handling marker %q", marker)
		}
	}
}

func TestDesktopChatCanStopActiveStreamWithoutOverwritingHistory(t *testing.T) {
	t.Parallel()

	chat := readDesktopAssetText(t, "js/desktop/apps/quickconnect-launchpad-chat.js")
	for _, marker := range []string{
		"setDesktopChatBusy(host, true)",
		"setDesktopChatBusy(host, false)",
		"requestDesktopChatAbort(host)",
		"host._desktopChatAbort = abortChatStream",
		"if (!isDesktopChatAbortError(err)) appendDesktopChatError(host, err);",
		"appendDesktopChatError(host, err)",
		"data-chat-send-label",
		"desktop.chat_stop",
	} {
		if !strings.Contains(chat, marker) {
			t.Fatalf("desktop chat stream missing stop/error handling marker %q", marker)
		}
	}
	for _, forbidden := range []string{
		"bubbles[bubbles.length - 1].textContent = err.message",
		"const bubbles = host.querySelectorAll('.vd-chat-bubble.agent')",
	} {
		if strings.Contains(chat, forbidden) {
			t.Fatalf("desktop chat must not overwrite the last agent bubble on stream errors: %q", forbidden)
		}
	}

	css := readDesktopAssetText(t, "css/desktop-apps.css")
	if !strings.Contains(css, ".vd-chat-send.is-stop") {
		t.Fatal("desktop chat CSS missing stop-state send button styling")
	}
}
