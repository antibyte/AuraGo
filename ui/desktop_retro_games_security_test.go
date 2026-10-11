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

func TestDesktopRetroGamesPlayerFrameIsLockedDown(t *testing.T) {
	src := rawDesktopAssetText(t, "js/desktop/apps/retro-games-player.js")
	for _, banned := range []string{"innerHTML", "outerHTML", "insertAdjacentHTML", "document.write", "eval(", "allow-popups", "allow-top-navigation", "allow-forms", "allow-modals", "clipboard", "allow-downloads", "window.open"} {
		if strings.Contains(src, banned) {
			t.Errorf("player contains %q", banned)
		}
	}
	for _, marker := range []string{
		"const EMBED_BASE = 'https://archive.org/embed/';",
		"const SANDBOX = 'allow-scripts allow-same-origin allow-pointer-lock';",
		"const ALLOW = 'fullscreen; gamepad; autoplay';",
		"EMBED_BASE + encodeURIComponent(id)",
		"referrerpolicy: 'no-referrer'",
		"/^[A-Za-z0-9][A-Za-z0-9._-]{0,99}$/",
		"frame.setAttribute('src', 'about:blank');",
		"frame.remove();",
		"window.RetroGamesPlayer = {",
	} {
		if !strings.Contains(src, marker) {
			t.Errorf("player lacks %q", marker)
		}
	}
}
