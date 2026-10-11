package ui

import (
	"strings"
	"testing"
)

var retroGamesAppFiles = []string{
	"js/desktop/apps/retro-games-views.js",
	"js/desktop/apps/retro-games-player.js",
	"js/desktop/apps/retro-games.js",
}

func TestDesktopRetroGamesViewsNeverRenderHTML(t *testing.T) {
	src := rawDesktopAssetText(t, "js/desktop/apps/retro-games-views.js")
	for _, banned := range []string{"innerHTML", "outerHTML", "insertAdjacentHTML", "document.write", "eval("} {
		if strings.Contains(src, banned) {
			t.Errorf("views uses %s", banned)
		}
	}
	for _, marker := range []string{
		"window.RetroGamesViews = {",
		"'https://archive.org/services/img/'",
		"encodeURIComponent(game.identifier)",
		"referrerpolicy: 'no-referrer'",
		"node.textContent = String(value)",
	} {
		if !strings.Contains(src, marker) {
			t.Errorf("views lacks %q", marker)
		}
	}
}
