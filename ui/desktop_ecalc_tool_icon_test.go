package ui

import (
	"strings"
	"testing"
)

// The chat shows a tool call with the sprite icon of its tool. ecalc has no cell of its own (a test pins the 102
// definitions), so it borrows the gauge of system_metrics through an alias.
func TestECalcToolCallsUseTheMetricsIcon(t *testing.T) {
	t.Parallel()
	icons := rawDesktopAssetText(t, "js/chat/tool-icons.js")
	if got := strings.Count(icons, "slot: "); got != 102 {
		t.Fatalf("tool-icons.js has %d icon definitions, want the 102 that TestChatToolIconPngSpriteCatalogRemainsWired pins", got)
	}
	line := ""
	for _, candidate := range strings.Split(icons, "\n") {
		if strings.Contains(candidate, "key: 'system_metrics'") {
			line = candidate
		}
	}
	if !strings.Contains(line, "aliases: Object.freeze(['metrics', 'ecalc'])") {
		t.Fatalf("the system_metrics icon must list ecalc as an alias, got %q", line)
	}
	bundle := rawDesktopAssetText(t, "js/chat/bundles/chat-runtime.bundle.js")
	if !strings.Contains(bundle, "aliases: Object.freeze(['metrics', 'ecalc'])") {
		t.Fatal("chat-runtime.bundle.js is stale: run npm run build:ui")
	}
}
