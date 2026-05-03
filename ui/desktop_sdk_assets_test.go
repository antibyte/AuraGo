package ui

import (
	"strings"
	"testing"
)

func TestDesktopSDKAssetsAreEmbedded(t *testing.T) {
	t.Parallel()

	sdk, err := Content.ReadFile("js/desktop/aura-desktop-sdk.js")
	if err != nil {
		t.Fatalf("SDK asset missing from embedded UI: %v", err)
	}
	sdkText := string(sdk)
	for _, want := range []string{
		"window.AuraDesktop",
		"aurago.desktop.request",
		"widgets.register",
		"fs.read",
		"ui.button",
	} {
		if !strings.Contains(sdkText, want) {
			t.Fatalf("SDK asset does not expose %q", want)
		}
	}
	if strings.Contains(sdkText, "fetch(") {
		t.Fatal("SDK must use the iframe bridge instead of direct fetch calls")
	}

	css, err := Content.ReadFile("css/desktop-sdk.css")
	if err != nil {
		t.Fatalf("SDK stylesheet missing from embedded UI: %v", err)
	}
	cssText := string(css)
	for _, want := range []string{".ad-app", ".ad-button", ".ad-toolbar", ".ad-widget"} {
		if !strings.Contains(cssText, want) {
			t.Fatalf("SDK stylesheet does not define %q", want)
		}
	}

	shell, err := Content.ReadFile("js/desktop/main.js")
	if err != nil {
		t.Fatalf("desktop shell missing from embedded UI: %v", err)
	}
	if !strings.Contains(string(shell), "aurago.desktop.request") {
		t.Fatal("desktop shell does not handle SDK bridge requests")
	}
	if !strings.Contains(string(shell), "/api/desktop/embed-token") {
		t.Fatal("desktop shell does not request embed tokens for generated iframes")
	}
	if !strings.Contains(string(shell), "ensureDesktopEmbedHasContent") {
		t.Fatal("desktop shell does not preflight generated iframe content")
	}
	if !strings.Contains(string(shell), "desktop.embed_empty") {
		t.Fatal("desktop shell does not expose an empty iframe file error")
	}
}
