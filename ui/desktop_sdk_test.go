package ui

import (
	"strings"
	"testing"
)

func TestDesktopSDKUsesDocumentBoundMessageChannel(t *testing.T) {
	t.Parallel()

	sdk, err := Content.ReadFile("js/desktop/aura-desktop-sdk.js")
	if err != nil {
		t.Fatalf("SDK asset missing from embedded UI: %v", err)
	}

	listener := string(sdk)
	for _, want := range []string{
		"window.__AURAGO_DESKTOP_SDK_CHANNEL__",
		"channelBridge.onPort(connectParentPort)",
		"parentPort.postMessage({",
		"function receiveParentMessage(event, port)",
		"error.status = Number(msg.status) || 0",
	} {
		if !strings.Contains(listener, want) {
			t.Fatalf("SDK channel bridge missing marker %q", want)
		}
	}
	for _, forbidden := range []string{"expectedParentMessageOrigin", "document.referrer", "window.parent.postMessage({ type: REQUEST_TYPE"} {
		if strings.Contains(listener, forbidden) {
			t.Fatalf("SDK must use the document-bound channel, found legacy parent transport marker %q", forbidden)
		}
	}
	if !strings.Contains(listener, "window.parent.postMessage({ type: ACTIVITY_TYPE }") {
		t.Fatal("SDK v1 activity pings must remain available for screensaver and resize lifecycle behavior")
	}
}

func TestDesktopShellPostsToSandboxedOpaqueFrames(t *testing.T) {
	t.Parallel()

	mainText := readDesktopAssetText(t, "js/desktop/main.js")
	for _, want := range []string{
		"document.addEventListener('keyup', handleDesktopKeyup)",
		"function relayGeneratedFrameKeyboardEvent(event)",
		"type: 'aurago.desktop.key-event'",
		"key: event.key",
		"code: event.code",
		"if (relayGeneratedFrameKeyboardEvent(event)) return;",
	} {
		if !strings.Contains(mainText, want) {
			t.Fatalf("desktop shell must post menu events to opaque sandbox frames, missing %q", want)
		}
	}
	quickconnect, err := Content.ReadFile("js/desktop/apps/quickconnect-launchpad-chat.js")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"type: SDK_CHANNEL_CHALLENGE_TYPE, challenge: client.challenge }, '*'",
		"message.capability !== frame.dataset.sdkChannel",
		"event.origin !== 'null'",
		"event.ports.length === 1",
		"client.port.postMessage({",
	} {
		if !strings.Contains(string(quickconnect), want) {
			t.Fatalf("opaque sandbox channel handling missing %q", want)
		}
	}
}
