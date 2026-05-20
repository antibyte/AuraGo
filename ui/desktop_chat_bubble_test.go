package ui

import (
	"regexp"
	"strings"
	"testing"
)

func TestDesktopChatAgentBubblesDoNotClipLongResponses(t *testing.T) {
	t.Parallel()

	css := readAllDesktopCSS(t)
	blocks := regexp.MustCompile(`(?s)\.vd-chat-bubble\.agent\s*\{([^}]*)\}`).FindAllStringSubmatch(css, -1)
	if len(blocks) == 0 {
		t.Fatal("desktop chat CSS missing agent bubble rules")
	}
	hasVisible := false
	for _, block := range blocks {
		body := block[1]
		if strings.Contains(body, "overflow: hidden") {
			t.Fatalf("agent chat bubbles must not clip long or formatted responses: %q", block[0])
		}
		if strings.Contains(body, "overflow: visible") {
			hasVisible = true
		}
	}
	if !hasVisible {
		t.Fatal("desktop chat CSS must explicitly keep agent bubbles visible")
	}
}

func TestDesktopChatLogItemsDoNotCollapseAroundLongResponses(t *testing.T) {
	t.Parallel()

	css := readAllDesktopCSS(t)
	for _, marker := range []string{
		"grid-template-rows: auto minmax(0, 1fr) auto auto;",
		".vd-chat-log > *",
		"flex: 0 0 auto;",
		"display: flow-root;",
		"box-sizing: border-box;",
		".vd-chat-bubble.agent > :first-child",
		".vd-chat-bubble.agent > :last-child",
		".vd-chat-status-text",
	} {
		if !strings.Contains(css, marker) {
			t.Fatalf("desktop chat CSS missing anti-collapse marker %q", marker)
		}
	}
}
