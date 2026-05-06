package ui

import (
	"strings"
	"testing"
)

func TestVirtualDesktopCreatesRadialMenuAnchorDynamically(t *testing.T) {
	mainText := readDesktopAssetText(t, "js/desktop/main.js")
	for _, want := range []string{
		"function ensureDesktopRadialMenuAnchor()",
		"anchor.id = 'radialMenuAnchor'",
		"anchor.className = 'vd-radial-anchor'",
		"insertBefore(anchor, agentButton)",
		"if (typeof injectRadialMenu === 'function') injectRadialMenu();",
		"ensureDesktopRadialMenuAnchor();",
	} {
		if !strings.Contains(mainText, want) {
			t.Fatalf("desktop shell missing dynamic radial menu anchor marker %q", want)
		}
	}
	if strings.Contains(readDesktopAssetText(t, "desktop.html"), `id="radialMenuAnchor"`) {
		t.Fatal("desktop HTML should keep the radial menu anchor dynamic, not static")
	}
}
