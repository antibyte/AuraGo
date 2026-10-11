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
		"button.title = label;",
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
		"let generation = 0;",
		"loadGeneration !== generation",
		"call('onMarkBroken')",
	} {
		if !strings.Contains(src, marker) {
			t.Errorf("player lacks %q", marker)
		}
	}
}

func TestDesktopRetroGamesAppLifecycle(t *testing.T) {
	src := rawDesktopAssetText(t, "js/desktop/apps/retro-games.js")
	for _, banned := range []string{"innerHTML", "outerHTML", "insertAdjacentHTML", "document.write", "eval(", "alert(", "confirm(", "prompt("} {
		if strings.Contains(src, banned) {
			t.Errorf("app contains %q", banned)
		}
	}
	for _, marker := range []string{
		"window.RetroGamesApp = { render, dispose, open };",
		"function dispose(windowId)",
		"instances.delete(windowId);",
		"st.controller.abort();",
		"stopPlayer(st);",
		"clearTimeout(st.pollTimer);",
		"const BASE = '/api/desktop/retro-games';",
		"'noopener,noreferrer'",
		"seq !== st.listSeq",
		"st.loadingMore",
		"st.librarySeq",
		"seq !== st.librarySeq",
		"st.openSeq",
		"token !== st.openSeq",
		"st.pending",
		"onMarkBroken: () => markBroken(st, game)",
		"function syncButtons(st)",
		"event.ctrlKey || event.metaKey || event.altKey",
		"MAX_STATUS_FAILURES",
		// Refused library changes are explained, not reported as a generic failure.
		"library_full: 'desktop.retro_games_library_full'",
		"desktop_readonly: 'desktop.retro_games_readonly'",
		"ERROR_NOTICES[code] || key",
		"function refreshAfterToggle(st, cfg)",
		// A disabled window keeps one slow passive poll and recovers by itself.
		"const DISABLED_POLL_MS = 30000;",
		"function pollDisabled(st)",
		"request(st, '/status?passive=1')",
		"function reenable(st)",
	} {
		if !strings.Contains(src, marker) {
			t.Errorf("app lacks %q", marker)
		}
	}
	// Only the passive status may be requested while disabled; the plain one starts a sync.
	if strings.Count(src, "'/status?passive=1'") != 1 || strings.Count(src, "'/status'") != 1 {
		t.Error("app must request status once passively (disabled poll) and once plainly (refreshStatus)")
	}
	// The failed-toggle path reloads the library, and a read-only play stays silent.
	sendStart, sendEnd := strings.Index(src, "function sendToggle(st, entry)"), strings.Index(src, "function toggleFavorite(st, game)")
	if sendStart < 0 || sendEnd < sendStart || !strings.Contains(src[sendStart:sendEnd], "if (!st.disabled) refreshAfterToggle(st, cfg);") {
		t.Error("a failed toggle does not reload the library")
	}
	if !strings.Contains(src, "if (errorCode(err) === 'desktop_readonly') return;") {
		t.Error("a read-only play is not kept silent")
	}
	for _, file := range retroGamesAppFiles {
		if lines := strings.Count(rawDesktopAssetText(t, file), "\n"); lines >= 1100 {
			t.Errorf("%s has %d lines", file, lines)
		}
	}
}
