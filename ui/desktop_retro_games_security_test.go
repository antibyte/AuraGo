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
	for _, banned := range []string{
		"innerHTML", "outerHTML", "insertAdjacentHTML", "document.write", "eval(", "allow-popups", "allow-top-navigation", "allow-forms", "allow-modals", "clipboard", "allow-downloads", "window.open",
		// The frame runs archive.org code: it must stay in an opaque origin and
		// AuraGo must never frame archive.org's own embed page again.
		"allow-same-origin", "EMBED_BASE", "embedURL", "archive.org/embed",
	} {
		if strings.Contains(src, banned) {
			t.Errorf("player contains %q", banned)
		}
	}
	for _, marker := range []string{
		"const PLAYER_BASE = '/api/desktop/retro-games/player?id=';",
		"const SANDBOX = 'allow-scripts allow-pointer-lock';",
		"const ALLOW = 'fullscreen; gamepad; autoplay';",
		"function playerURL(id, muted, soundLabel)",
		"PLAYER_BASE + encodeURIComponent(id)",
		"'&muted=' + (muted ? '1' : '0')",
		"'&sound=' + encodeURIComponent(soundLabel || '')",
		"playerURL(game.identifier, muted, t('desktop.retro_games_click_for_sound'))",
		"referrerpolicy: 'no-referrer'",
		"/^[A-Za-z0-9][A-Za-z0-9._-]{0,99}$/",
		"frame.setAttribute('src', 'about:blank');",
		"frame.remove();",
		"window.RetroGamesPlayer = { mount, playerURL, SANDBOX, ALLOW };",
		"let generation = 0;",
		"pending === generation",
		"call('onMarkBroken')",
	} {
		if !strings.Contains(src, marker) {
			t.Errorf("player lacks %q", marker)
		}
	}
	// The sandbox is exactly script execution and pointer lock, nothing else.
	sandbox := regexp.MustCompile(`const SANDBOX = '([^']*)';`).FindStringSubmatch(src)
	if sandbox == nil {
		t.Fatal("player has no SANDBOX constant")
	}
	if tokens := strings.Fields(sandbox[1]); strings.Join(tokens, ",") != "allow-scripts,allow-pointer-lock" {
		t.Errorf("SANDBOX tokens = %v, want allow-scripts and allow-pointer-lock only", tokens)
	}
	// Neither the player nor the app may point a frame, tab or script at archive.org's embed or an absolute URL.
	for _, file := range retroGamesAppFiles {
		for _, line := range strings.Split(rawDesktopAssetText(t, file), "\n") {
			if strings.Contains(line, "frame.setAttribute('src'") && !strings.Contains(line, "'about:blank'") && !strings.Contains(line, "playerURL(") {
				t.Errorf("%s sets the frame to something other than the player page or about:blank: %s", file, strings.TrimSpace(line))
			}
		}
	}
}

// The player talks to the app only through the message listener of the page it
// framed: the sender must be that frame's window, the payload must carry the
// {rg: 1} marker, and the state machine must not depend on anything an attacker
// controlling archive.org's page could fake from outside the protocol (the
// frame's load event, window focus changes).
func TestDesktopRetroGamesPlayerTrustsOnlyItsOwnFrame(t *testing.T) {
	src := rawDesktopAssetText(t, "js/desktop/apps/retro-games-player.js")
	if strings.Count(src, "window.addEventListener('message', onMessage);") != 1 || strings.Count(src, "addEventListener('message'") != 1 {
		t.Error("mount must register exactly one message listener")
	}
	if strings.Count(src, "window.removeEventListener('message', onMessage);") != 1 || strings.Count(src, "removeEventListener('message'") != 1 {
		t.Error("destroy must remove exactly the one message listener")
	}
	destroy := strings.Index(src, "destroy() {")
	if destroy < 0 || !strings.Contains(src[destroy:], "window.removeEventListener('message', onMessage);") {
		t.Error("the message listener is not removed in destroy")
	}
	listener := functionBody(t, src, "function onMessage(event)")
	for _, marker := range []string{
		"disposed",
		"event.source !== frame.contentWindow",
		"!data || data.rg !== 1",
		// archive.org's code shares the page's realm and can post any {rg: 1}
		// message: only the known types with well-formed values act.
		"data.type === 'state' && PLAYER_STATES.includes(data.state)",
		"data.type === 'audio' && typeof data.blocked === 'boolean'",
	} {
		if !strings.Contains(listener, marker) {
			t.Errorf("the message listener lacks %q", marker)
		}
	}
	if !strings.Contains(src, "const PLAYER_STATES = ['loading', 'running', 'failed'];") {
		t.Error("the accepted states are not pinned to loading, running and failed")
	}
	// Message content is data: compared and acted on, never rendered, logged or evaluated.
	for _, banned := range []string{"console.", "textContent = data", "textContent = event", "title = data", "setAttribute(data", "event.data."} {
		if strings.Contains(src, banned) {
			t.Errorf("player handles message content with %q", banned)
		}
	}
	if strings.Count(src, "data.state") != 2 || strings.Count(src, "data.blocked") != 1 || strings.Count(src, "event.data") != 1 {
		t.Error("message content may only be read once, in the listener: type and state enum checks, the blocked flag")
	}
	states := functionBody(t, src, "function onPlayerState(state)")
	for _, marker := range []string{
		// failed is final until a restart (and may come before loading: a script that cannot
		// load fails the page first); loading counts once per page; running only follows loading.
		"if (phase === 'failed') return;",
		"if (phase !== 'idle') return;",
		"if (phase !== 'loading') return;",
		"phase = 'failed';",
		"hideOverlay();",
		"showHint('desktop.retro_games_player_failed');",
	} {
		if !strings.Contains(states, marker) {
			t.Errorf("the state handler lacks %q", marker)
		}
	}
	// A new page starts at 'idle': start() and restartGame() reset it.
	if strings.Count(src, "phase = 'idle';") < 2 {
		t.Error("start() and restartGame() must both reset the page phase to idle")
	}
	for _, marker := range []string{
		"showHint('desktop.retro_games_player_stalled');",
		"const STALL_MS = 60000;",
		"opts.stallMs",
	} {
		if !strings.Contains(src, marker) {
			t.Errorf("player lacks %q", marker)
		}
	}
	// The three hint buttons: restart, open on archive.org, mark as not starting.
	for _, marker := range []string{"rg-overlay-restart", "rg-overlay-external", "rg-overlay-broken"} {
		if !strings.Contains(src, marker) {
			t.Errorf("hint strip lacks %q", marker)
		}
	}
	// No state may come from the frame's load event or from window focus changes.
	for _, banned := range []string{
		"addEventListener('load'", "addEventListener('blur'", "addEventListener('focus'", "addEventListener('focusout'", "addEventListener('focusin'",
		".onload", ".onblur", "onWindowBlur", "userFocusedFrame", "frameFocusIsOurs", "loadGeneration",
	} {
		if strings.Contains(src, banned) {
			t.Errorf("player still derives state from %q", banned)
		}
	}
	// Only the sound state travels to the page, always to the frame's window.
	for _, marker := range []string{
		"target.postMessage(message, '*');",
		"frame.contentWindow",
		"{ rg: 1, type: 'mute', muted }",
	} {
		if !strings.Contains(src, marker) {
			t.Errorf("player lacks %q", marker)
		}
	}
	if strings.Count(src, ".postMessage(") != 1 {
		t.Error("the player must post through its single frame-bound helper")
	}
}

// The sound toggle is a normal cabinet button: pressed state, two labels, the
// choice remembered per viewer (storage may be blocked), the current value in
// the player URL and in a message to the running page.
func TestDesktopRetroGamesPlayerHasAMuteToggle(t *testing.T) {
	src := rawDesktopAssetText(t, "js/desktop/apps/retro-games-player.js")
	for _, marker := range []string{
		"const MUTE_KEY = 'aurago.retroGames.muted';",
		"window.localStorage.getItem(MUTE_KEY)",
		"window.localStorage.setItem(MUTE_KEY,",
		"button('rg-mute'",
		"returnFocus(() => toggleMute())",
		"'aria-pressed'",
		"t('desktop.retro_games_sound_off')",
		"t('desktop.retro_games_sound_on')",
		"'🔊'",
		"'🔇'",
		"el('div', { className: 'rg-marquee-actions' }, [fav, broken, mute, restart, full, external])",
		// A click that came before the page listened is repeated once the page says it started.
		"muted !== urlMuted",
		"urlMuted = muted;",
	} {
		if !strings.Contains(src, marker) {
			t.Errorf("player lacks %q", marker)
		}
	}
	for _, accessor := range []string{"function readMuted()", "function writeMuted(on)"} {
		body := functionBody(t, src, accessor)
		if !strings.Contains(body, "try {") || !strings.Contains(body, "catch") {
			t.Errorf("%s does not guard the storage access with try/catch", accessor)
		}
	}
	// Storage keeps nothing but the flag.
	if strings.Count(src, "localStorage") != 2 {
		t.Errorf("localStorage is used %d times, want one read and one write", strings.Count(src, "localStorage"))
	}
}

// functionBody returns the source of the function that starts at signature up
// to its closing brace at the same indentation.
func functionBody(t *testing.T, src, signature string) string {
	t.Helper()
	start := strings.Index(src, signature)
	if start < 0 {
		t.Fatalf("player has no %q", signature)
	}
	lineStart := strings.LastIndex(src[:start], "\n") + 1
	indent := src[lineStart:start]
	end := strings.Index(src[start:], "\n"+indent+"}\n")
	if end < 0 {
		t.Fatalf("%q has no closing brace at its indentation", signature)
	}
	return src[start : start+end]
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
		"returnFocus(() => toggleMute())",
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
	// Focus is moved by us alone: no listener watches the window or the frame for it.
	if strings.Contains(src, "'blur'") || strings.Contains(src, "'focus'") {
		t.Error("the player must not listen to focus or blur events")
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
