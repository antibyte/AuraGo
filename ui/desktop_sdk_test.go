package ui

import (
	"strings"
	"testing"
)

func TestDesktopSDKValidatesMessageOrigin(t *testing.T) {
	t.Parallel()

	sdk, err := Content.ReadFile("js/desktop/aura-desktop-sdk.js")
	if err != nil {
		t.Fatalf("SDK asset missing from embedded UI: %v", err)
	}

	listener := string(sdk)
	for _, want := range []string{
		"function expectedParentMessageOrigin()",
		"document.referrer",
		"function isTrustedParentMessage(event)",
		"event.origin === expectedParentMessageOrigin()",
		"event.source !== window.parent",
	} {
		if !strings.Contains(listener, want) {
			t.Fatalf("SDK message trust helper missing marker %q", want)
		}
	}
	start := strings.Index(listener, "window.addEventListener('message'")
	if start < 0 {
		t.Fatal("SDK message listener is missing")
	}
	listener = listener[start:]
	end := strings.Index(listener, "\n    function context()")
	if end < 0 {
		t.Fatal("SDK message listener boundary changed")
	}
	listener = listener[:end]

	for _, want := range []string{
		"!isTrustedParentMessage(event)",
		"return;",
	} {
		if !strings.Contains(listener, want) {
			t.Fatalf("SDK message listener does not validate message origin/source marker %q", want)
		}
	}
}

func TestDesktopShellPostsToSandboxedOpaqueFrames(t *testing.T) {
	t.Parallel()

	mainText := readDesktopAssetText(t, "js/desktop/main.js")
	for _, want := range []string{
		"}, '*');",
		"document.addEventListener('keyup', handleDesktopKeyup)",
		"function relayGeneratedFrameKeyboardEvent(event)",
		"type: 'aurago.desktop.key-event'",
		"key: event.key",
		"code: event.code",
		"if (relayGeneratedFrameKeyboardEvent(event)) return;",
		"postSDKMenuAction(windowId, actionId)",
		"postSDKContextMenuAction(client, actionId)",
	} {
		if !strings.Contains(mainText, want) {
			t.Fatalf("desktop shell must post menu events to opaque sandbox frames, missing %q", want)
		}
	}
	if strings.Contains(mainText, "}, window.location.origin);") {
		t.Fatal("desktop shell must not target generated app frames by same-origin because sandboxed frames have opaque origins")
	}
}
