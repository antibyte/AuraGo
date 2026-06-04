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

	css := readAllDesktopAppCSS(t)
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

func TestDesktopAgentChatComposerUsesReadableAlignedControls(t *testing.T) {
	t.Parallel()

	css := readAllDesktopAppCSS(t)
	formBody := desktopExactCSSRuleBody(t, css, ".vd-chat-form")
	if !strings.Contains(formBody, "align-items: stretch;") {
		t.Fatal("desktop chat composer must stretch input and buttons to a shared row height")
	}

	inputBody := desktopExactCSSRuleBody(t, css, ".vd-chat-input")
	for _, want := range []string{
		"min-height: 46px;",
		"background: rgba(10, 15, 26, 0.72);",
		"color: #f5f7fb;",
		"border-color: rgba(148, 163, 184, 0.28);",
	} {
		if !strings.Contains(inputBody, want) {
			t.Fatalf("desktop chat input CSS missing readable control marker %q", want)
		}
	}
	if strings.Contains(inputBody, "background: var(--ds-color-control-bg);") {
		t.Fatal("desktop chat input must not use the light generic control background")
	}

	buttonsBody := desktopExactCSSRuleBody(t, css, ".vd-chat-form-buttons")
	if !strings.Contains(buttonsBody, "align-items: stretch;") {
		t.Fatal("desktop chat composer buttons must align with the input height")
	}

	voiceBody := desktopExactCSSRuleBody(t, css, ".vd-chat-voice")
	sendBody := desktopExactCSSRuleBody(t, css, ".vd-chat-send")
	for selector, body := range map[string]string{
		".vd-chat-voice": voiceBody,
		".vd-chat-send":  sendBody,
	} {
		if !strings.Contains(body, "min-height: 46px;") {
			t.Fatalf("%s must share the chat input minimum height", selector)
		}
	}
}

func TestDesktopChatRendererKeepsPersonaAvatarsBesideBothRoles(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/chat-renderer.js")
	appendAvatarBody := jsFunctionBodyInWindowMenuTest(t, source, "appendAvatar(chatLog, role, bubble, isContinuation)")
	for _, want := range []string{
		"const isUser = role === 'user';",
		"row.className = 'vd-chat-message-row ' + (isUser ? 'user' : 'agent');",
		"avatar.className = 'vd-chat-avatar ' + (isUser ? 'user' : 'agent');",
		"window.AuraChatCore.personaAvatarMarkup(isUser ? 'user' : 'agent')",
	} {
		if !strings.Contains(appendAvatarBody, want) {
			t.Fatalf("desktop chat avatar renderer missing marker %q", want)
		}
	}
	if strings.Contains(appendAvatarBody, "role === 'user'") && strings.Contains(appendAvatarBody, "chatLog.appendChild(bubble);\n            return;") {
		t.Fatal("desktop chat renderer must not append user bubbles without an avatar row")
	}

	appendRichBody := jsFunctionBodyInWindowMenuTest(t, source, "appendRichBubble(chatLog, role, text, prevRole)")
	if !strings.Contains(appendRichBody, "this.appendAvatar(chatLog, role, bubble, isGroup);") {
		t.Fatal("desktop chat rich bubble renderer must route both user and agent bubbles through appendAvatar")
	}
	if strings.Contains(appendRichBody, "if (role === 'agent')") {
		t.Fatal("desktop chat rich bubble renderer must not special-case avatars to agent bubbles only")
	}

	chatSource := readDesktopAssetText(t, "js/desktop/apps/agent-chat.js")
	personaBody := jsFunctionBodyInWindowMenuTest(t, chatSource, "function applyDesktopPersonaIconKey(previewKey)")
	for _, want := range []string{
		"window.AuraChatCore.personaImageUrl(key)",
		"window._activePersonaImageUrl = src;",
		".vd-chat-avatar.agent .persona-avatar-img, .vd-chat-welcome-avatar .persona-avatar-img",
	} {
		if !strings.Contains(personaBody, want) {
			t.Fatalf("desktop chat persona image updater missing marker %q", want)
		}
	}
}
