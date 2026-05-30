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
		"grid-template-rows: minmax(0, 1fr) auto auto;",
		".vd-chat-context[hidden]",
		"display: none;",
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

func TestDesktopChatShellKeepsToolbarAboveScrollableMain(t *testing.T) {
	t.Parallel()

	css := readAllDesktopCSS(t)
	chatBlocks := regexp.MustCompile(`(?s)\.vd-chat\s*\{([^}]*)\}`).FindAllStringSubmatch(css, -1)
	if len(chatBlocks) == 0 {
		t.Fatal("desktop chat CSS missing shell grid rule")
	}
	if !strings.Contains(chatBlocks[0][1], "grid-template-rows: auto minmax(0, 1fr);") {
		t.Fatal("desktop chat shell must reserve a top toolbar row and a flexible main row")
	}

	sidebarBlocks := regexp.MustCompile(`(?s)\.vd-chat-sidebar\s*\{([^}]*)\}`).FindAllStringSubmatch(css, -1)
	if len(sidebarBlocks) == 0 || !strings.Contains(sidebarBlocks[0][1], "grid-row: 1 / -1;") {
		t.Fatal("desktop chat sidebar must span toolbar and main rows")
	}

	toolbarBlocks := regexp.MustCompile(`(?s)\.vd-chat-toolbar\s*\{([^}]*)\}`).FindAllStringSubmatch(css, -1)
	if len(toolbarBlocks) == 0 || !strings.Contains(toolbarBlocks[0][1], "grid-row: 1;") {
		t.Fatal("desktop chat toolbar must stay in the top shell row")
	}

	mainBlocks := regexp.MustCompile(`(?s)\.vd-chat-main\s*\{([^}]*)\}`).FindAllStringSubmatch(css, -1)
	if len(mainBlocks) == 0 {
		t.Fatal("desktop chat CSS missing main grid rule")
	}
	if !strings.Contains(mainBlocks[0][1], "grid-row: 2;") {
		t.Fatal("desktop chat main area must stay below the toolbar")
	}
	if strings.Contains(mainBlocks[0][1], "grid-row: 1 / -1;") {
		t.Fatal("desktop chat main area must not overlap the toolbar row")
	}
}

func TestDesktopChatDropOverlayStartsTrulyHidden(t *testing.T) {
	t.Parallel()

	css := readAllDesktopCSS(t)
	overlayBlocks := regexp.MustCompile(`(?s)\.vd-chat-drop-overlay\s*\{([^}]*)\}`).FindAllStringSubmatch(css, -1)
	if len(overlayBlocks) == 0 {
		t.Fatal("desktop chat CSS missing drop overlay rules")
	}
	if !strings.Contains(overlayBlocks[0][1], "visibility: hidden") {
		t.Fatal("desktop chat drop overlay must be visibility-hidden until a drag is active")
	}
	if !strings.Contains(overlayBlocks[0][1], "pointer-events: none") {
		t.Fatal("desktop chat drop overlay must not intercept normal chat input when inactive")
	}

	activeBlocks := regexp.MustCompile(`(?s)\.vd-chat-drop-overlay\.active\s*\{([^}]*)\}`).FindAllStringSubmatch(css, -1)
	if len(activeBlocks) == 0 {
		t.Fatal("desktop chat CSS missing active drop overlay rules")
	}
	if !strings.Contains(activeBlocks[0][1], "visibility: visible") {
		t.Fatal("desktop chat drop overlay must explicitly become visible only while active")
	}
}

func TestDesktopChatSidebarToggleDoesNotOpenAsCollapsed(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/agent-chat.js")
	body := jsFunctionBodyInWindowMenuTest(t, source, "function initSidebar")
	for _, marker := range []string{
		"chat.dataset.sidebarCompact = isWide ? 'false' : 'true';",
		"const sidebarCollapsed = !sidebarOpen;",
		"chat.dataset.sidebarCollapsed = sidebarCollapsed ? 'true' : 'false';",
		"if (!isWide && sidebarOpen) chat.dataset.sidebarOpen = 'true';",
	} {
		if !strings.Contains(body, marker) {
			t.Fatalf("desktop chat sidebar toggle missing state marker %q", marker)
		}
	}
	if strings.Contains(body, "chat.dataset.sidebarCollapsed = 'true';") {
		t.Fatal("desktop chat sidebar must not mark the sidebar collapsed while opening the mobile overlay")
	}

	css := readDesktopAssetText(t, "css/desktop-app-chat.css")
	for _, marker := range []string{
		`.vd-chat[data-sidebar-compact="true"] {`,
		`.vd-chat[data-sidebar-compact="true"] .vd-chat-sidebar {`,
		`inset: 51px auto 0 0;`,
		`.vd-chat[data-sidebar-compact="true"][data-sidebar-open="true"] .vd-chat-sidebar {`,
		`.vd-chat[data-sidebar-compact="true"] .vd-chat-sidebar-backdrop {`,
		`inset: 51px 0 0 0;`,
	} {
		if !strings.Contains(css, marker) {
			t.Fatalf("desktop chat compact sidebar CSS missing marker %q", marker)
		}
	}
}
