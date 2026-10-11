package ui

import (
	"regexp"
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

// The player must hand the keyboard to the game: the frame gets focus on mount
// and again after every control that would otherwise keep it (Space/Enter would
// re-trigger the button, the arrow keys would go nowhere).
func TestDesktopRetroGamesPlayerGivesTheGameTheKeyboard(t *testing.T) {
	src := rawDesktopAssetText(t, "js/desktop/apps/retro-games-player.js")
	if strings.Contains(src, "back.focus()") {
		t.Error("mount focuses Back, so Space/Enter would leave the game")
	}
	for _, marker := range []string{
		"function focusFrame()",
		"frame.focus();",
		"returnFocus(call('onFavorite'))",
		"returnFocus(call('onBroken'))",
		"returnFocus(() => restartGame())",
		"returnFocus(() => toggleFullscreen())",
		"returnFocus(call('onMarkBroken'))",
		// Interaction with the cross-origin frame is only visible as a window blur.
		"window.addEventListener('blur', onWindowBlur);",
		"document.activeElement === frame",
		// Production waits one minute; only the options (tests) can shorten it.
		"const STALL_MS = 60000;",
		"opts.stallMs",
	} {
		if !strings.Contains(src, marker) {
			t.Errorf("player lacks %q", marker)
		}
	}
	// Opening a new tab leaves the player where it is: no focus juggling for the external link.
	if strings.Contains(src, "returnFocus(call('onExternal'))") {
		t.Error("the external link must not steal the focus back")
	}
	// Mount ends by focusing the frame, after the cabinet is attached.
	attach := strings.Index(src, "stage.appendChild(")
	if attach < 0 {
		t.Fatal("player never attaches the cabinet")
	}
	tail := src[attach:]
	if ret := strings.Index(tail, "return {"); ret < 0 || !strings.Contains(tail[:ret], "focusFrame();") {
		t.Error("mount does not focus the frame after attaching the cabinet")
	}
	// The blur listener lives and dies with the player.
	destroy := strings.Index(src, "destroy() {")
	if destroy < 0 || !strings.Contains(src[destroy:], "window.removeEventListener('blur', onWindowBlur);") {
		t.Error("destroy does not remove the window blur listener")
	}
	if strings.Count(src, "window.addEventListener('blur'") != 1 || strings.Count(src, "window.removeEventListener('blur'") != 1 {
		t.Error("the player must register and remove exactly one window blur listener")
	}
	// Our buttons stay in the tab order.
	if strings.Contains(src, "tabindex") || strings.Contains(src, "tabIndex") {
		t.Error("the player must not take its buttons out of the tab order")
	}
}

// The "taking unusually long" hint is a compact strip: neither the loading bar
// nor the hint may become a layer over the frame that takes its pointer events,
// or the game cannot be clicked while archive.org still loads the splash image.
func TestDesktopRetroGamesStalledHintNeverCoversTheFrame(t *testing.T) {
	css := rawDesktopAssetText(t, "css/desktop-app-retro-games.css")
	css = regexp.MustCompile(`(?s)/\*.*?\*/`).ReplaceAllString(css, "")
	rules := regexp.MustCompile(`(?s)([^{}]+)\{([^{}]*)\}`).FindAllStringSubmatch(css, -1)
	var strip, buttons bool
	for _, rule := range rules {
		selector, body := strings.TrimSpace(rule[1]), rule[2]
		if !strings.Contains(selector, ".rg-overlay") {
			continue
		}
		normalized := strings.Join(strings.Fields(body), " ")
		if strings.Contains(normalized, "pointer-events: auto") {
			for _, part := range strings.Split(selector, ",") {
				if !strings.Contains(part, ".rg-btn") {
					t.Errorf("%q takes pointer events: only the hint buttons may", strings.TrimSpace(part))
				}
			}
		}
		switch selector {
		case ".rg-overlay":
			strip = strings.Contains(normalized, "pointer-events: none") && strings.Contains(normalized, "bottom: 0")
		case ".rg-overlay.is-stalled":
			// The stalled state keeps the bottom strip: no top/inset anchor, no click capture.
			if regexp.MustCompile(`(^|[;\s])(top|inset)\s*:`).MatchString(body) || strings.Contains(normalized, "pointer-events") {
				t.Errorf(".rg-overlay.is-stalled must stay the bottom strip, got %q", normalized)
			}
		case ".rg-overlay-actions .rg-btn":
			buttons = strings.Contains(normalized, "pointer-events: auto")
		}
	}
	if !strip {
		t.Error(".rg-overlay must be a bottom strip with pointer-events: none")
	}
	if !buttons {
		t.Error("the hint buttons must opt back in to pointer events")
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
