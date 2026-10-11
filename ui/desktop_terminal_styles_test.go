package ui

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"aurago/internal/retronet"
)

func TestDesktopTerminalStyleCatalog(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/terminal-styles.js")
	for _, want := range []string{
		"window.TerminalStyles = {",
		"ids:",
		"normalize:",
		"load:",
		"save:",
		"profile:",
		"applyXterm:",
		"'modern'",
		"'amber'",
		"'green'",
		"'apple2'",
		"'commodore64'",
		"'ibm3278'",
		"'vintage'",
		"'mono-green'",
		"'transparent-green'",
		"aurago.desktop.terminal.style",
		"return 'modern'",
		"phosphor:",
		"term.options.theme = profile.theme",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("terminal-styles.js missing %q", want)
		}
	}
	if strings.Contains(source, "crt-shader") || strings.Contains(source, "cool-retro-term") {
		t.Fatal("terminal-styles.js must not reference Chat CRT shaders or cool-retro-term sources")
	}
}

func TestDesktopTerminalStyleI18n(t *testing.T) {
	t.Parallel()

	keys := []string{
		"desktop.terminal_audio",
		"desktop.terminal_audio_off",
		"desktop.terminal_audio_on",
		"desktop.terminal_style",
		"desktop.terminal_style_amber",
		"desktop.terminal_style_apple2",
		"desktop.terminal_style_commodore64",
		"desktop.terminal_style_green",
		"desktop.terminal_style_ibm3278",
		"desktop.terminal_style_modern",
		"desktop.terminal_style_mono_green",
		"desktop.terminal_style_transparent_green",
		"desktop.terminal_style_vintage",
	}
	english := map[string]string{
		"desktop.terminal_audio":                   "Key click",
		"desktop.terminal_audio_off":               "Key click off",
		"desktop.terminal_audio_on":                "Key click on",
		"desktop.terminal_style":                   "Style",
		"desktop.terminal_style_amber":             "Amber",
		"desktop.terminal_style_apple2":            "Apple II",
		"desktop.terminal_style_commodore64":       "Commodore 64",
		"desktop.terminal_style_green":             "Green phosphor",
		"desktop.terminal_style_ibm3278":           "IBM 3278",
		"desktop.terminal_style_modern":            "Modern",
		"desktop.terminal_style_mono_green":        "Monochrome green",
		"desktop.terminal_style_transparent_green": "Transparent green",
		"desktop.terminal_style_vintage":           "Vintage",
	}

	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		path := "lang/desktop/" + lang + ".json"
		var values map[string]string
		if err := json.Unmarshal([]byte(rawDesktopAssetText(t, path)), &values); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, key := range keys {
			got := strings.TrimSpace(values[key])
			if got == "" {
				t.Fatalf("%s missing non-empty %s", path, key)
			}
			if lang != "en" && got == english[key] && (key == "desktop.terminal_audio" || key == "desktop.terminal_style") {
				t.Fatalf("%s must not copy the English %s string", path, key)
			}
		}
		if lang == "de" {
			if strings.Contains(values["desktop.terminal_audio_on"], "ae") || strings.Contains(values["desktop.terminal_style_green"], "ue") {
				t.Fatal("German terminal style strings must use real umlauts")
			}
			if values["desktop.terminal_style"] == "Style" {
				t.Fatal("German desktop.terminal_style must be translated")
			}
		}
	}
}

func TestDesktopTerminalAssetsLoadInDependencyOrder(t *testing.T) {
	t.Parallel()

	loader := readDesktopAssetText(t, "js/desktop/core/module-loader.js")
	start := strings.Index(loader, "        'terminal': {")
	if start < 0 {
		t.Fatal("terminal loader missing 'terminal' entry")
	}
	rest := loader[start+len("        'terminal': {"):]
	endRel := strings.Index(rest, "\n        '")
	if endRel < 0 {
		t.Fatal("terminal loader entry is not followed by another app")
	}
	loader = loader[start : start+len("        'terminal': {")+endRel]
	markers := []string{
		`'/css/xterm.css'`,
		`'/css/desktop-app-terminal.css'`,
		`'/js/vendor/xterm.min.js'`,
		`'/js/vendor/xterm-addon-fit.min.js'`,
		`'/js/vendor/xterm-addon-webgl.min.js'`,
		`'/js/desktop/apps/terminal-styles.js'`,
		`'/js/desktop/apps/terminal-crt.js'`,
		`'/js/desktop/apps/terminal-audio.js'`,
		`'/js/desktop/apps/terminal-text.js'`,
		`'/js/desktop/apps/terminal-geometry.js'`,
		`'/js/desktop/apps/terminal-modem.js'`,
		`'/js/desktop/apps/terminal-retronet-directory.js'`,
		`'/js/desktop/apps/terminal-retronet-session.js'`,
		`'/js/desktop/apps/terminal-retronet-entries.js'`,
		`'/js/desktop/apps/terminal.js'`,
	}
	if got := strings.Count(loader, "'/js/"); got != 13 {
		t.Fatalf("terminal loader lists %d scripts, want exactly the 13 of the C64 Contract C", got)
	}
	prev := -1
	for _, marker := range markers {
		idx := strings.Index(loader, marker)
		if idx < 0 {
			t.Fatalf("terminal loader missing %s", marker)
		}
		if idx < prev {
			t.Fatalf("terminal loader order wrong at %s", marker)
		}
		prev = idx
	}
	if !strings.Contains(readDesktopAssetText(t, "js/vendor/xterm-addon-webgl.min.js"), "WebglAddon") {
		t.Fatal("vendored WebGL addon missing WebglAddon export")
	}
}

func TestDesktopTerminalReceivesBootstrapAccessor(t *testing.T) {
	t.Parallel()

	const marker = "registerWindowCleanup, readonly: desktopReadonly(), getBootstrap: () => state.bootstrap || {} }"
	for _, path := range []string{"js/desktop/core/menus-and-routing.js", "js/desktop/bundles/main.bundle.js"} {
		source := rawDesktopAssetText(t, path)
		start := strings.Index(source, "window.TerminalApp.render(")
		if start < 0 {
			t.Fatalf("%s does not render the Terminal", path)
		}
		line := source[start:]
		if end := strings.Index(line, "\n"); end >= 0 {
			line = line[:end]
		}
		if !strings.Contains(line, marker) {
			t.Fatalf("%s must pass getBootstrap to the Terminal context: %s", path, line)
		}
	}
}

func TestDesktopTerminalAppStylesheet(t *testing.T) {
	t.Parallel()

	css := readDesktopAssetText(t, "css/desktop-app-terminal.css")
	for _, want := range []string{
		".vd-terminal-app",
		".vd-terminal-bezel",
		".vd-terminal-crt-overlay",
		"[data-terminal-style=\"modern\"]",
		"[data-terminal-style=\"amber\"]",
		"[data-terminal-style=\"green\"]",
		"[data-terminal-style=\"apple2\"]",
		"[data-terminal-style=\"commodore64\"]",
		"[data-terminal-style=\"ibm3278\"]",
		"[data-terminal-style=\"vintage\"]",
		"[data-terminal-style=\"mono-green\"]",
		"[data-terminal-style=\"transparent-green\"]",
		"[data-terminal-fallback=\"css\"]",
		"prefers-reduced-motion",
		"[data-terminal-audio]",
		".vd-terminal-app .vd-sr-only",
	} {
		if !strings.Contains(css, want) {
			t.Fatalf("desktop-app-terminal.css missing %q", want)
		}
	}
}

func TestDesktopTerminalCrtContract(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/terminal-crt.js")
	for _, want := range []string{
		"window.TerminalCrt = {",
		"create(",
		"setProfile",
		"setEnabled",
		"resize",
		"dispose",
		"usesFallback",
		"vd-terminal-crt-overlay",
		"data-terminal-fallback",
		"u_phosphor",
		"u_curve",
		"u_bloom",
		"u_burn",
		"u_noise",
		"u_flicker",
		"u_mask",
		"u_alpha",
		"u_motion",
		"1.25",
		"vd-space-hidden",
		"prefers-reduced-motion",
		"dataset.animations",
		"webgl",
		"TEXTURE_2D",
		".xterm-screen canvas",
		"NEAREST",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("terminal-crt.js missing %q", want)
		}
	}
	if strings.Contains(source, "crt-shader.js") || strings.Contains(source, "swordfish90") {
		t.Fatal("CRT engine must be original and must not load Chat CRT shaders")
	}
}

func TestDesktopTerminalAudioContract(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/terminal-audio.js")
	for _, want := range []string{
		"window.TerminalAudio = {",
		"create(",
		"loadMuted",
		"saveMuted",
		"shouldSilence",
		"setProfile",
		"setMuted",
		"playKey",
		"dispose",
		"aurago.desktop.terminal.audioMuted",
		"AudioContext",
		"event.repeat",
		"commodore64",
		"ibm3278",
		"apple2",
		"prefers-reduced-motion",
		"dataset.animations",
		// C64 bell (Contract C): the key-click rules, one short square tone.
		"function beep()",
		"const BELL = { type: 'square'",
		// One tone per BELL_GAP_MS however many BELs arrive, never queued against a suspended clock.
		"const BELL_GAP_MS = 150;",
		"if (clock() - lastBell < BELL_GAP_MS) return;",
		// The gap counts from the tone: a slow first AudioContext must not let BELs queued behind it add a tone.
		"lastBell = clock();",
		"return audio.state === 'running' ? audio : null;",
		"return { setProfile, setMuted, playKey, beep, dispose }",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("terminal-audio.js missing %q", want)
		}
	}
	beep := source[strings.Index(source, "function beep()"):]
	if !strings.Contains(beep[:strings.Index(beep, "}")], "if (!retro || muted || shouldSilence()) return;") {
		t.Fatal("beep() must follow the key-click rules: retro style, not muted, no reduced motion or disabled animations")
	}
}

func TestDesktopTerminalAppWiresStyles(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/terminal.js")
	for _, want := range []string{
		"window.TerminalApp = { render, dispose }",
		"const instances = new Map()",
		"data-terminal-style",
		"data-terminal-audio",
		"data-terminal-bezel",
		"TerminalStyles",
		"TerminalCrt.create",
		"TerminalAudio.create",
		"WebglAddon.WebglAddon",
		"applyXterm",
		"playKey",
		"/api/code-studio/terminal",
		"binaryType = 'arraybuffer'",
		"fonts.load",
		"function openShell()",
		"TerminalModem.create(",
		"TerminalModem.createThrottle(",
		"TerminalRetroNetDirectory.create(",
		"TerminalRetroNetDirectory.LOCAL_SHELL_ID",
		"TerminalRetroNetSession.open(",
		"TerminalRetroNetEntries.open(",
		"TerminalRetroNetEntries.confirmDelete(",
		"'aurago:desktop-policy'",
		"b.retronet_enabled === true",
		// Plan decision 16: explicit readonly/enabled/retronet_enabled switch Retro-Net; missing fields change nothing.
		"const off = detail.readonly === true || detail.enabled === false || detail.retronet_enabled === false",
		"if (!off && detail.retronet_enabled !== true) return",
		// "Open Terminal Here" at the workspace root passes path '' and still means the shell.
		"retroNetOn && typeof ctx.path !== 'string'",
		"term.options.convertEol = false",
		"term.options.convertEol = true",
		"'aurago.desktop.terminal.retronet.last'",
		`const HANGUP_KEY = '\x1d'`,
		"'desktop.terminal_directory'",
		"'desktop.terminal_dialing'",
		"'desktop.terminal_connected'",
		"'desktop.terminal_retronet_result_' + reason",
		"data-terminal-mode",
		`data-terminal-announce aria-live="polite"`,
		"select[data-terminal-baud]",
		"[data-terminal-retronet-action]",
		"window.TerminalText && window.TerminalModem",
		"window.TerminalText.printable(control.fingerprint)",
		// Host-key answers use the session module's rule: localized letters first, then ASCII y/n.
		"window.TerminalRetroNetSession.hostKeyAnswer(data, tr('desktop.terminal_retronet_hostkey_yes'), tr('desktop.terminal_retronet_hostkey_no'))",
		// Ctrl+] never reaches the session: the dispatcher intercepts it in dialing and retro modes.
		"if (data.indexOf(HANGUP_KEY) >= 0) {",
		// Output buffered during the dial is shown when the service hangs up before the dial ends.
		"if (remote && run.connected && !run.live && !run.hostKey) {",
		// Dispose removes own-entry dialogs without close(), which would reopen a dialog with a pending save.
		"root.querySelectorAll('dialog[data-terminal-retronet-dialog]').forEach(function (dialog) { dialog.remove(); })",
		// Screen geometry lives in terminal-geometry.js (Contract C): fit, the 80x24 VT screen, 80x25 BBS, 40x25 C64.
		"window.TerminalGeometry.create({",
		"getProfile: currentProfile,",
		"if (geometry) geometry.set(next);",
		// Every Retro-Net session runs on a pinned grid (Telnet worlds and SSH on 'vt') and opens with it.
		"const pinned = c64 ? 'c64' : entry.protocol === 'telnet' && entry.kind === 'bbs' ? 'bbs' : 'vt';",
		"setGeometry(pinned);",
		"const c64 = entry.kind === 'c64';",
		"cols: grid.cols,",
		"rows: grid.rows,",
		// Only the directory follows the grid; sessions never resize.
		"if (mode === 'directory' && directory) directory.render();",
		// C64 sessions: the commodore64 style without saving it, a locked style select, cursor, screen and border.
		"const next = persist === false ? styles.normalize(id) : styles.save(id);",
		"applyStyle('commodore64', false);",
		"applyStyle(look.style, false);",
		"node.title = ctx.t('desktop.terminal_retronet_style_locked');",
		"if (select) select.disabled = locked;",
		"term.options.cursorStyle = 'block';",
		"term.options.cursorBlink = true;",
		"const C64_THEME = { background: '#000000', cursor: '#706DEB', cursorAccent: '#000000' };",
		"const C64_BORDER = '#706DEB';",
		"crt.setProfile(crtProfile(profile));",
		"term.write(C64_FRESH_SCREEN);",
		`const C64_FRESH_SCREEN = '\x1b[0m\x1b[2J\x1b[25;1H';`,
		// Key hint after CONNECT, colour keys and the bell (C64 calls only).
		"window.TerminalText.wrapHints(tr('desktop.terminal_retronet_c64_keys'), term.cols - 1).forEach(function (line) { writeDim(line); });",
		"term.attachCustomKeyEventHandler(onTermKey);",
		"const sequence = window.TerminalRetroNetSession.c64ColorKey(event);",
		"if (!run || !run.c64 || mode !== 'retro') return true;",
		"if (event.type === 'keydown' && !event.repeat) {",
		// The hint hold: board output waits 2 s after CONNECT unless a key (also a colour key) or a full queue ends it;
		// every way out of the call ends it, a local hang-up drops the held output.
		"const C64_HINT_HOLD_MS = 2000;",
		"if (run.c64) run.hold = window.setTimeout(function () { endHold(run); }, C64_HINT_HOLD_MS);",
		"if ((!run.live || run.hold) && run.buffered + bytes.length > window.TerminalModem.MAX_QUEUE_BYTES) {",
		"if (retro === run && !run.ended) releaseHeld(run);",
		"if (!remote) run.buffer = [];",
		// The CONNECT announcement names Ctrl+] once: the C64 keys include it, other calls get the hang-up hint.
		"[tr('desktop.terminal_retronet_c64_keys'), tr('desktop.terminal_retronet_style_locked')] : [tr('desktop.terminal_retronet_hangup_hint')]",
		"announce(window.TerminalText.sentences(spoken));",
		// Dim text wraps between words inside the screen; the locked style select is described for screen readers.
		"window.TerminalText.wrapWords(text, term.cols - 1)",
		"writeDim(next, true);",
		"select.setAttribute('aria-describedby', styleNote.id);",
		"select.removeAttribute('aria-describedby');",
		// Leaving the C64 look never puts the C64 font back before the geometry leaves 'c64'.
		"const leaving = !c64Look && !!geometry && geometry.mode() === 'c64';",
		"term.onBell(onTermBell);",
		"if (audio && retro && retro.c64 && mode === 'retro') audio.beep();",
		// A result ignores keys for a moment so keys typed as the service hangs up do not dismiss it.
		"if (window.performance.now() - resultAt >= RESULT_GRACE_MS) afterResult();",
		// Placeholders are filled here, never by the shared t(), whose replaceAll treats $ in server text as a pattern.
		"let text = String(ctx.t(key));",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("terminal.js missing %q", want)
		}
	}
	for _, forbidden := range []string{"function printable", "function cellWidth", "function fitToCells", "key === 'y'", "dialog.close(); dialog.remove()", "ctx.t(key, params)",
		"bbsMode", "fitBbsFont", "VGA_FONT", "function wrapHints", "styles.save('commodore64')", "forEach(writeDim)", "function wrapWords", "function sentences",
		"CLASSIC_GRID", "retro.session.resize("} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("terminal.js must use the shared TerminalText/session helpers; found %q", forbidden)
		}
	}
	// Leaving the directory (also on 403 or a policy switch) and dispose remove own-entry dialogs.
	if strings.Count(source, "removeEntryDialogs();") != 2 {
		t.Fatal("terminal.js must remove own-entry dialogs in leaveDirectory() and cleanup()")
	}
	// Every transition resets through resetScreen(): RIS behind queued output, then a visible cursor.
	if strings.Count(source, "term.reset();") != 1 || !strings.Contains(source, `term.write('\x1bc\x1b[?25h')`) {
		t.Fatal("terminal.js must reset the screen only through resetScreen(), which queues RIS and shows the cursor")
	}
	if strings.Count(source, "new WebSocket") != 1 || !strings.Contains(source, "new WebSocket(protocol + '//' + location.host + '/api/code-studio/terminal')") {
		t.Fatal("terminal.js must create exactly one WebSocket: the Code Studio shell socket in openShell()")
	}
	if strings.Contains(source, "/api/desktop/retronet/connect") {
		t.Fatal("the Retro-Net socket belongs to terminal-retronet-session.js")
	}
	session := readDesktopAssetText(t, "js/desktop/apps/terminal-retronet-session.js")
	if strings.Count(session, "new WebSocket") != 1 || !strings.Contains(session, "/api/desktop/retronet/connect") {
		t.Fatal("terminal-retronet-session.js must own exactly one Retro-Net WebSocket")
	}
	for _, module := range []string{"terminal-text.js", "terminal-geometry.js", "terminal-modem.js", "terminal-retronet-directory.js", "terminal-retronet-entries.js", "terminal-styles.js", "terminal-crt.js", "terminal-audio.js"} {
		if strings.Contains(readDesktopAssetText(t, "js/desktop/apps/"+module), "new WebSocket") {
			t.Fatalf("%s must not open sockets", module)
		}
	}
	if strings.Contains(source, "onclick=") {
		t.Fatal("terminal.js must not use inline onclick")
	}
	// The temporary C64 look is entered and left only through setGeometry(), so every exit path restores it.
	if strings.Count(source, "enterC64Look()") != 2 || strings.Count(source, "leaveC64Look()") != 2 {
		t.Fatal("terminal.js must enter and leave the C64 look only in setGeometry()")
	}
}

// Screen geometry (Contract C): fit, the 80x24 VT screen in the style's font, 80x25 BBS with the VGA font, 40x25
// C64 with the C64 font; one fit at a time.
// Behaviour is covered by scripts/test-terminal-geometry.mjs.
func TestDesktopTerminalGeometryContract(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/terminal-geometry.js")
	for _, want := range []string{
		"window.TerminalGeometry = { MODES: MODES, create: create }",
		"fit: null,",
		"vt: { cols: 80, rows: 24, scrollback: FIT_SCROLLBACK },",
		`bbs: { cols: 80, rows: 25, font: '"Aura VGA", "Aura Terminal", ui-monospace, monospace', load: '16px "Aura VGA"', cell: [0.5, 1] }`,
		`c64: { cols: 40, rows: 25, font: '"Aura C64", "Aura Terminal", ui-monospace, monospace', load: '16px "Aura C64"', cell: [1, 1], pixels: 8 }`,
		// C64: whole device pixels (font size pixels * k / dpr) from xterm's alpha glyph atlas, refit on a DPR change.
		"Math.floor(ems * dpr / spec.pixels)",
		"term.options.allowTransparency = !!spec.pixels;",
		"term.options.allowTransparency = false;",
		"window.matchMedia('(resolution: ' + (window.devicePixelRatio || 1) + 'dppx)')",
		// Grids with their own font keep line height 1 (VGA blocks come from the font, a C64 nudge moves glyphs); 'vt'
		// takes the style's font and its fit owns the line height. Sizes ignore the CSS transforms of window animations.
		"if (spec.font) return { family: spec.font, load: spec.load };",
		"return { family: family, load: (profile.fontSize || 16) + 'px ' + first };",
		"if (spec.font) term.options.lineHeight = 1;",
		"if (!spec.font) term.options.lineHeight = fitted.lineHeight;",
		"return { width: screenEl.offsetWidth, height: screenEl.offsetHeight };",
		"set: set,",
		"mode: function () { return current; },",
		"refit: refit,",
		"dispose: dispose",
		"root.setAttribute('data-terminal-geometry', mode);",
		"root.removeAttribute('data-terminal-geometry');",
		"term.resize(spec.cols, spec.rows);",
		"document.fonts.load(face(spec).load)",
		// 'vt' keeps the scrollback and leaves xterm's scrollbar a strip beside the grid; bbs and c64 keep none.
		"term.options.scrollback = spec.scrollback || 0;",
		"term.options.scrollback = FIT_SCROLLBACK;",
		"const bar = spec.scrollback ? (term.options.overviewRuler && term.options.overviewRuler.width) || 14 : 0;",
		"const FIT_SCROLLBACK = 1000;",
		"window.TerminalStyles.applyXterm(term, getProfile());",
		"const step = function (value) { return Math.max(MIN_FONT_SIZE, Math.floor(value * 2) / 2); };",
		// One fit at a time: a request during a fit reruns it once afterwards.
		"if (fitting) {",
		"again = true;",
		"if (disposed || mine !== revision) return;",
		"screenEl.style.background = (term.options.theme && term.options.theme.background) || '';",
		// The 0.5 px search: down from the em cell's size; 'vt' from the current size to the size its measured cells
		// scale to, bracketed, one step tried beyond, and an unchanged glass remembered in `ceiling`.
		"let size = step(scales ? term.options.fontSize : ems);",
		"const expected = scales ? step(size * Math.min(width / box.width, room.height / box.height)) : size;",
		"const next = !scales ? size : Math.min(expected > size ? expected : size + 0.5, bad - 0.5);",
		"const box = await measure(size, 0, scales ? 1 : 0);",
		// 'vt' spaces its rows to fill the glass height: the largest device px cell height that fits, as a line
		// height over the cell at line height 1, at most MAX_LINE_HEIGHT; an unchanged glass keeps its fit.
		"const MAX_LINE_HEIGHT = 2;",
		"const base = Math.round(rect.height * dpr / spec.rows);",
		"for (let cell = Math.floor((room.height + 0.5) * dpr / spec.rows), tries = 0; cell > base && tries < 3 && !spaced; cell -= 1, tries += 1) {",
		"const box = await measure(good.size, 0, Math.min(MAX_LINE_HEIGHT, (cell + 0.5) / base));",
		"if (box.width === ceiling.width && box.height === ceiling.height) rect = box;",
		// The per-axis glass border of 'vt' and 'bbs' (the C64 grid keeps the style padding, a share of the width).
		"const GLASS_X = 0.02;",
		"const GLASS_Y = 0.03;",
		"const GLASS_MIN = 12;",
		"return glass && own > 0 ? Math.min(own, Math.max(GLASS_MIN, share * size)) : own;",
		"const room = space(element, !spec.pixels);",
		"function set(mode) {",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("terminal-geometry.js missing %q", want)
		}
	}
	for _, forbidden := range []string{"new WebSocket", "innerHTML", "localStorage", "ctx.t(", "TerminalRetroNet", "getBoundingClientRect", "lineHeight = withSize",
		"minCols", "shrinkToMinimum", "proposeDimensions"} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("terminal-geometry.js only sizes the grid, by offset sizes, with pinned grids only; found %q", forbidden)
		}
	}
	// A lost WebGL context switches xterm to the DOM renderer, whose cells differ: the pinned grid fits again.
	if !regexp.MustCompile(`(?s)onContextLoss\(\(\) => \{.*?refitScreen\(\);\s*\}\);`).MatchString(readDesktopAssetText(t, "js/desktop/apps/terminal.js")) {
		t.Fatal("terminal.js must refit the screen when the WebGL context is lost")
	}
}

func TestDesktopTerminalRetroNetSourcesUseKnownKeys(t *testing.T) {
	t.Parallel()

	known := map[string]bool{}
	for _, key := range terminalRetroNetI18nKeys(t) {
		known[key] = true
	}
	pattern := regexp.MustCompile(`desktop\.terminal_(?:retronet_[a-z0-9_]+|directory|dialing|connected)\b`)
	for _, file := range []string{"terminal.js", "terminal-geometry.js", "terminal-modem.js", "terminal-retronet-directory.js", "terminal-retronet-session.js", "terminal-retronet-entries.js"} {
		source := readDesktopAssetText(t, "js/desktop/apps/"+file)
		for _, key := range pattern.FindAllString(source, -1) {
			if strings.HasSuffix(key, "_") {
				continue // dynamic prefix such as result_ + reason; all suffixes are in the key list
			}
			if !known[key] {
				t.Errorf("%s uses %s, which is not in the Retro-Net key list", file, key)
			}
		}
	}
}

func TestDesktopTerminalVGAFontIsVendoredWithAttribution(t *testing.T) {
	t.Parallel()

	font, err := os.ReadFile(filepath.Join("fonts", "Px437_IBM_VGA_8x16.woff"))
	if err != nil {
		t.Fatalf("read VGA font: %v", err)
	}
	if len(font) < 1024 || string(font[:4]) != "wOFF" {
		t.Fatalf("Px437_IBM_VGA_8x16.woff is not a WOFF font (%d bytes)", len(font))
	}
	license, err := os.ReadFile(filepath.Join("fonts", "Px437-LICENSE.txt"))
	if err != nil {
		t.Fatalf("read VGA font license: %v", err)
	}
	sum := sha256.Sum256(font)
	for _, want := range []string{
		"VileR",
		"Copyright (c) 2016-2020 VileR",
		"https://int10h.org/oldschool-pc-fonts/",
		"CC BY-SA 4.0",
		"https://creativecommons.org/licenses/by-sa/4.0/",
		"unmodified",
		"Source archive entry:",
		"SHA-256: " + hex.EncodeToString(sum[:]),
	} {
		if !strings.Contains(string(license), want) {
			t.Fatalf("Px437-LICENSE.txt missing %q", want)
		}
	}
	css := readDesktopAssetText(t, "css/desktop-app-terminal.css")
	for _, want := range []string{
		`font-family: "Aura VGA";`,
		`url('/fonts/Px437_IBM_VGA_8x16.woff') format('woff')`,
		`font-display: block;`,
	} {
		if !strings.Contains(css, want) {
			t.Fatalf("desktop-app-terminal.css missing %q", want)
		}
	}
}

var terminalRetroNetLocales = []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"}

// Server result reasons; TestDesktopTerminalRetroNetReasonsMatchPackage keeps
// this list equal to every Reason* constant declared in internal/retronet.
var terminalRetroNetServerReasons = []string{
	retronet.ReasonRefused,
	retronet.ReasonLimit,
	retronet.ReasonTimeout,
	retronet.ReasonDNS,
	retronet.ReasonBlocked,
	retronet.ReasonRemoteClosed,
	retronet.ReasonIdle,
	retronet.ReasonMaxDuration,
	retronet.ReasonDisabled,
	retronet.ReasonHostKeyMismatch,
	retronet.ReasonHostKeyRejected,
	retronet.ReasonServerShutdown,
}

// Result reasons only the browser produces (Ctrl+] / toolbar hang-up, socket
// closed without a result frame).
var terminalRetroNetBrowserReasons = []string{"local_hangup", "lost"}

var terminalRetroNetCategories = []string{
	retronet.CategoryClassics,
	retronet.CategoryBBS,
	retronet.CategoryMUDs,
	retronet.CategoryGames,
	retronet.CategoryCommodore,
	retronet.CategoryOwn,
}

func terminalRetroNetReasons() []string {
	return append(append([]string{}, terminalRetroNetServerReasons...), terminalRetroNetBrowserReasons...)
}

var terminalRetroNetFixedKeys = []string{
	"desktop.terminal_directory",
	"desktop.terminal_dialing",
	"desktop.terminal_connected",
	"desktop.terminal_retronet_directory",
	"desktop.terminal_retronet_hangup",
	"desktop.terminal_retronet_baud",
	"desktop.terminal_retronet_baud_off",
	"desktop.terminal_retronet_baud_rate",
	"desktop.terminal_retronet_title",
	"desktop.terminal_retronet_local_shell",
	"desktop.terminal_retronet_local_shell_desc",
	"desktop.terminal_retronet_loading",
	"desktop.terminal_retronet_load_failed",
	"desktop.terminal_retronet_checking",
	"desktop.terminal_retronet_status_failed",
	"desktop.terminal_retronet_not_own",
	"desktop.terminal_retronet_help",
	"desktop.terminal_retronet_help_admin",
	"desktop.terminal_retronet_announce",
	"desktop.terminal_retronet_hangup_hint",
	"desktop.terminal_retronet_last_seen",
	"desktop.terminal_retronet_telnet_notice",
	"desktop.terminal_retronet_press_key",
	"desktop.terminal_retronet_press_key_shell",
	"desktop.terminal_retronet_hostkey_title",
	"desktop.terminal_retronet_hostkey_fingerprint",
	"desktop.terminal_retronet_hostkey_question",
	"desktop.terminal_retronet_hostkey_yes",
	"desktop.terminal_retronet_hostkey_no",
	"desktop.terminal_retronet_new_entry",
	"desktop.terminal_retronet_edit_entry",
	"desktop.terminal_retronet_form_hint",
	"desktop.terminal_retronet_field_name",
	"desktop.terminal_retronet_field_description",
	"desktop.terminal_retronet_field_protocol",
	"desktop.terminal_retronet_field_type",
	"desktop.terminal_retronet_type_bbs",
	"desktop.terminal_retronet_type_world",
	"desktop.terminal_retronet_type_c64",
	"desktop.terminal_retronet_field_charset",
	"desktop.terminal_retronet_field_host",
	"desktop.terminal_retronet_field_port",
	"desktop.terminal_retronet_field_user",
	"desktop.terminal_retronet_error_name",
	"desktop.terminal_retronet_error_description",
	"desktop.terminal_retronet_error_host",
	"desktop.terminal_retronet_error_private_host",
	"desktop.terminal_retronet_error_port",
	"desktop.terminal_retronet_error_mail_port",
	"desktop.terminal_retronet_error_user",
	"desktop.terminal_retronet_error_limit",
	"desktop.terminal_retronet_error_save",
	"desktop.terminal_retronet_error_delete",
	"desktop.terminal_retronet_error_gone",
	"desktop.terminal_retronet_discard_question",
	"desktop.terminal_retronet_discard",
	"desktop.terminal_retronet_keep_editing",
	"desktop.terminal_retronet_delete_title",
	"desktop.terminal_retronet_delete_confirm",
	"desktop.terminal_retronet_status_c64_help",
	"desktop.terminal_retronet_c64_keys",
	"desktop.terminal_retronet_style_locked",
}

// Full sentences that must be translated in every non-English locale.
var terminalRetroNetSentenceKeys = []string{
	"desktop.terminal_retronet_load_failed",
	"desktop.terminal_retronet_not_own",
	"desktop.terminal_retronet_help",
	"desktop.terminal_retronet_help_admin",
	"desktop.terminal_retronet_telnet_notice",
	"desktop.terminal_retronet_press_key",
	"desktop.terminal_retronet_press_key_shell",
	"desktop.terminal_retronet_hostkey_title",
	"desktop.terminal_retronet_form_hint",
	"desktop.terminal_retronet_error_name",
	"desktop.terminal_retronet_error_host",
	"desktop.terminal_retronet_error_private_host",
	"desktop.terminal_retronet_error_port",
	"desktop.terminal_retronet_error_mail_port",
	"desktop.terminal_retronet_error_user",
	"desktop.terminal_retronet_error_limit",
	"desktop.terminal_retronet_error_gone",
	"desktop.terminal_retronet_discard_question",
	"desktop.terminal_retronet_status_c64_help",
	"desktop.terminal_retronet_c64_keys",
	"desktop.terminal_retronet_style_locked",
}

func terminalRetroNetI18nKeys(t *testing.T) []string {
	t.Helper()
	keys := append([]string{}, terminalRetroNetFixedKeys...)
	for _, category := range terminalRetroNetCategories {
		keys = append(keys, "desktop.terminal_retronet_cat_"+category)
	}
	for _, state := range []string{"online", "offline", "unknown", "c64"} {
		keys = append(keys, "desktop.terminal_retronet_status_"+state)
	}
	for _, reason := range terminalRetroNetReasons() {
		keys = append(keys, "desktop.terminal_retronet_result_"+reason)
	}
	catalog := retronet.DefaultCatalog()
	if len(catalog) == 0 {
		t.Fatal("retronet.DefaultCatalog() returned no entries")
	}
	for _, entry := range catalog {
		if !strings.HasPrefix(entry.DescriptionKey, "desktop.terminal_retronet_entry_") {
			t.Fatalf("catalog entry %q has description key %q", entry.ID, entry.DescriptionKey)
		}
		keys = append(keys, entry.DescriptionKey)
	}
	seen := make(map[string]bool, len(keys))
	for _, key := range keys {
		if seen[key] {
			t.Fatalf("Retro-Net i18n key %s is listed twice", key)
		}
		seen[key] = true
	}
	return keys
}

func readTerminalDesktopLocale(t *testing.T, lang string) map[string]string {
	t.Helper()
	path := filepath.Join("lang", "desktop", lang+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var values map[string]string
	if err := json.Unmarshal(data, &values); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return values
}

func TestDesktopTerminalRetroNetReasonsMatchPackage(t *testing.T) {
	t.Parallel()

	files, err := filepath.Glob(filepath.Join("..", "internal", "retronet", "*.go"))
	if err != nil || len(files) == 0 {
		t.Fatalf("list internal/retronet sources: %v (%d files)", err, len(files))
	}
	fset := token.NewFileSet()
	declared := map[string]bool{}
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		parsed, err := parser.ParseFile(fset, file, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", file, err)
		}
		for _, decl := range parsed.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.CONST {
				continue
			}
			for _, spec := range gen.Specs {
				value, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for i, name := range value.Names {
					if !strings.HasPrefix(name.Name, "Reason") || i >= len(value.Values) {
						continue
					}
					lit, ok := value.Values[i].(*ast.BasicLit)
					if !ok || lit.Kind != token.STRING {
						continue
					}
					reason, err := strconv.Unquote(lit.Value)
					if err != nil {
						t.Fatalf("unquote %s in %s: %v", name.Name, file, err)
					}
					declared[reason] = true
				}
			}
		}
	}
	listed := map[string]bool{}
	for _, reason := range terminalRetroNetServerReasons {
		listed[reason] = true
		if !declared[reason] {
			t.Errorf("terminalRetroNetServerReasons lists %q, which internal/retronet does not declare", reason)
		}
	}
	for reason := range declared {
		if !listed[reason] {
			t.Errorf("internal/retronet declares reason %q; add it to terminalRetroNetServerReasons and translate desktop.terminal_retronet_result_%s", reason, reason)
		}
	}
	for _, reason := range terminalRetroNetBrowserReasons {
		if declared[reason] {
			t.Errorf("browser-only reason %q is also declared by internal/retronet", reason)
		}
	}
}

func TestDesktopTerminalRetroNetI18n(t *testing.T) {
	t.Parallel()

	keys := terminalRetroNetI18nKeys(t)
	copyChecked := append([]string{}, terminalRetroNetSentenceKeys...)
	for _, reason := range terminalRetroNetReasons() {
		copyChecked = append(copyChecked, "desktop.terminal_retronet_result_"+reason)
	}
	for _, entry := range retronet.DefaultCatalog() {
		if entry.Kind == retronet.KindC64 {
			copyChecked = append(copyChecked, entry.DescriptionKey) // C64 descriptions are translated, never copied
		}
	}
	placeholder := regexp.MustCompile(`\{\{[a-z_]+\}\}`)
	english := readTerminalDesktopLocale(t, "en")
	for _, lang := range terminalRetroNetLocales {
		values := readTerminalDesktopLocale(t, lang)
		for _, key := range keys {
			got := strings.TrimSpace(values[key])
			if got == "" {
				t.Errorf("lang/desktop/%s.json missing non-empty %s", lang, key)
				continue
			}
			want := placeholder.FindAllString(english[key], -1)
			have := placeholder.FindAllString(got, -1)
			sort.Strings(want)
			sort.Strings(have)
			if strings.Join(want, ",") != strings.Join(have, ",") {
				t.Errorf("lang/desktop/%s.json %s placeholders %v, want %v", lang, key, have, want)
			}
		}
		if lang != "en" {
			for _, key := range copyChecked {
				if values[key] != "" && values[key] == english[key] {
					t.Errorf("lang/desktop/%s.json copies the English %s", lang, key)
				}
			}
		}
		yes := values["desktop.terminal_retronet_hostkey_yes"]
		no := values["desktop.terminal_retronet_hostkey_no"]
		if utf8.RuneCountInString(yes) != 1 || utf8.RuneCountInString(no) != 1 || strings.EqualFold(yes, no) {
			t.Errorf("lang/desktop/%s.json host-key answers must be two different single letters, got %q/%q", lang, yes, no)
		}
	}
	if english["desktop.terminal_retronet_telnet_notice"] != "Unencrypted Telnet connection – do not use real passwords" {
		t.Errorf("English Telnet notice changed: %q", english["desktop.terminal_retronet_telnet_notice"])
	}
	for _, lang := range terminalRetroNetLocales {
		values := readTerminalDesktopLocale(t, lang)
		if !strings.HasPrefix(values["desktop.terminal_retronet_status_c64_help"], "[C]") {
			t.Errorf("lang/desktop/%s.json desktop.terminal_retronet_status_c64_help must start with the [C] marker", lang)
		}
		keys := values["desktop.terminal_retronet_c64_keys"]
		for _, key := range []string{"DEL", "RUN/STOP", "HOME", "CLR", "INST", "F1", "F8", "RVS", "1–8", "9–16", "9/0", "+]"} {
			if !strings.Contains(keys, key) {
				t.Errorf("lang/desktop/%s.json desktop.terminal_retronet_c64_keys must name %q", lang, key)
			}
		}
		if strings.Count(keys, " · ") < 9 {
			t.Errorf("lang/desktop/%s.json desktop.terminal_retronet_c64_keys must keep its 10 parts separated by \" · \"", lang)
		}
	}
	german := readTerminalDesktopLocale(t, "de")
	for key, umlaut := range map[string]string{
		"desktop.terminal_retronet_cat_own":                 "ä",
		"desktop.terminal_retronet_telnet_notice":           "ü",
		"desktop.terminal_retronet_press_key":               "ü",
		"desktop.terminal_retronet_result_hostkey_mismatch": "ü",
		"desktop.terminal_retronet_c64_keys":                "ü",
		"desktop.terminal_retronet_style_locked":            "ä",
		"desktop.terminal_retronet_status_c64":              "ü",
	} {
		if !strings.Contains(german[key], umlaut) {
			t.Errorf("German %s must use real umlauts, got %q", key, german[key])
		}
	}
}

func TestDesktopTerminalModemContract(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/terminal-modem.js")
	for _, want := range []string{
		"window.TerminalModem = {",
		"BAUD_RATES: BAUD_RATES.slice()",
		"const BAUD_RATES = [0, 300, 1200, 2400, 9600, 14400]",
		"'aurago.desktop.terminal.baud'",
		"loadBaud: loadBaud",
		"saveBaud: saveBaud",
		"create: create",
		"createThrottle: createThrottle",
		"return { dial: dial, skip: skip, connectLine: connectLine, dispose: dispose }",
		"return { setBaud: setBaud, push: push, flush: flush, dispose: dispose }",
		`'ATZ\r\nOK\r\nATDT '`,
		"'CONNECT '",
		"|| 14400",
		"createOscillator",
		"createBiquadFilter",
		"createBufferSource",
		"getChannelData",
		"freq: 2100",
		"'1': [697, 1209]",
		"'0': [941, 1336]",
		"HANDSHAKE_MS = 2500",
		"Math.imul(hash, 0x01000193)",
		"requestAnimationFrame(release)",
		"cancelAnimationFrame(frame)",
		"(baud / 10)",
		"shouldSilence",
		"prefers-reduced-motion: reduce",
		"dataset.animations === 'false'",
		"profile.retro",
		"isMuted()",
		"window.TerminalText.printable(host)",
		"if (audible()) context();",
		"ctx.state === 'running'",
		// Throttle units (Contract C): code points, whole escape sequences and whole synchronized-output blocks.
		"const SEQUENCE_LIMIT = 1024;",
		"const SYNC_BEGIN = '?2026h';",
		"const SYNC_END = '?2026l';",
		"const found = allowance > 0 ? scan(allowance) : { units: 0, bytes: 0 };",
		"if (queue.length && found.units === allowance) schedule();",
		// Unterminated blocks leave at 64 KiB or 1 s after a stall; scanning goes on from where it stopped.
		"const SYNC_LIMIT = 64 * 1024;",
		"if (b === 0x18 || b === 0x1a) return true;",
		"const SYNC_WAIT_MS = 1000;",
		"function scan(limit)",
		"stall = window.setTimeout(expire, SYNC_WAIT_MS);",
		"if (stall) window.clearTimeout(stall);",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("terminal-modem.js missing %q", want)
		}
	}
	for _, forbidden := range []string{"new WebSocket", ".mp3", ".wav", ".ogg", "new Audio(", "function printable", `\u001f`} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("terminal-modem.js must synthesize audio and never open sockets; found %q", forbidden)
		}
	}
}

// Shared display-width and control-character helpers (Contract E, terminal-text.js).
// Behaviour against the vendored xterm is covered by scripts/test-terminal-retronet-directory.mjs.
func TestDesktopTerminalTextContract(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/terminal-text.js")
	for _, want := range []string{
		"window.TerminalText = { cellWidth: cellWidth, cells: cells, fitToCells: fitToCells, printable: printable, sentences: sentences, wrapHints: wrapHints, wrapWords: wrapWords }",
		"function cells(value)",
		"function wrapHints(value, width)",
		// Text to read wraps at spaces and splits only over-long words, at cluster boundaries; nothing is cut.
		"function wrapWords(value, width)",
		"else clusters(word).forEach(function (part, index) { put(part.text, part.width, index === 0); });",
		// Announcements: a full stop only after a part without sentence punctuation (ASCII, ellipsis, danda, CJK).
		"function sentences(parts)",
		`/[.!?\u2026\u0964\u3002\uff01\uff0e\uff1f]$/.test(text) ? text : text + '.'`,
		`const HINT_SEPARATOR = ' \xb7 ';`,
		"function cellWidth(code)",
		"function fitToCells(value, width, pad)",
		"function printable(value)",
		`/[\u0000-\u001f\u007f-\u009f]/g`,
		"Unicode 6",
		"0x200b, 0x200f",
		"0x1100, 0x115f",
		"0x1f3fb",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("terminal-text.js missing %q", want)
		}
	}
	for _, forbidden := range []string{"WIDE_EMOJI", "activeVersion", "._core", "unicodeService", "new WebSocket", ".length > width", ".slice(0, width"} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("terminal-text.js must follow the vendored xterm's Unicode 6 widths without private APIs; found %q", forbidden)
		}
	}
}

func TestDesktopTerminalRetroNetDirectoryContract(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/terminal-retronet-directory.js")
	for _, want := range []string{
		"window.TerminalRetroNetDirectory = { LOCAL_SHELL_ID: LOCAL_SHELL_ID, create: create }",
		"const TerminalText = window.TerminalText",
		"const LOCAL_SHELL_ID = 'local-shell'",
		"'aurago.desktop.terminal.retronet.last'",
		"['classics', 'bbs', 'muds', 'games', 'commodore', 'own']",
		"online: '[*]'",
		"offline: '[ ]'",
		"unknown: '[?]'",
		"c64: '[C]'",
		"if (entry.kind === 'c64') return { state: 'c64', lastOnline: '' };",
		"TerminalText.wrapHints(tr('desktop.terminal_retronet_status_c64_help'), cols - 1)",
		"api('/api/desktop/retronet/directory')",
		"api('/api/desktop/retronet/status', { method: 'POST' })",
		"payload.can_edit === true",
		"entry.description_key",
		"last_online_at",
		"DOUBLE_TAP_MS = 500",
		"DIGIT_WINDOW_MS = 1000",
		`'\x1b[5~'`,
		`'\x1b[6~'`,
		`'\x1b[3~'`,
		`'\x1b[?7l\x1b[?25l'`,
		`'\x1b[?1049h'`,
		"active.viewportY - active.baseY",
		"'dblclick'",
		"'.xterm-screen'",
		"Intl.RelativeTimeFormat",
		"announce(TerminalText.printable(tr('desktop.terminal_retronet_announce'",
		"tr('desktop.terminal_retronet_help_admin')",
		"TerminalText.fitToCells(entry.name, nameWidth, true)",
		"toLocaleUpperCase(window.SYSTEM_LANG",
		"dialing = false;",
		"load: load",
		"render: render",
		"handleData: handleData",
		"handleMouse: handleMouse",
		"refreshStatus: refreshStatus",
		"selected: function",
		"entries: function",
		"dispose: dispose",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("terminal-retronet-directory.js missing %q", want)
		}
	}
	for _, forbidden := range []string{"new WebSocket", "innerHTML", "alert(", "window.confirm", "._core", "unicodeService", ".length > width", ".slice(0, width",
		"function cellWidth", "function cells", "function fitToCells", "function printable", "ZERO_WIDTH", "WIDE_EMOJI", "activeVersion", `\u001f`} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("terminal-retronet-directory.js must render only into xterm and use the shared TerminalText helpers; found %q", forbidden)
		}
	}
	if strings.Contains(source, "function wrapHints") || strings.Contains(source, "HINT_SEPARATOR") {
		t.Fatal("terminal-retronet-directory.js must pack its key help through TerminalText.wrapHints")
	}
}

// The directory numbers entries with two digits; 00 is the local shell.
func TestDesktopTerminalRetroNetDirectoryNumbersFitTwoDigits(t *testing.T) {
	t.Parallel()

	if total := len(retronet.DefaultCatalog()) + retronet.MaxOwnEntries; total > 99 {
		t.Fatalf("%d catalog and own entries do not fit the two-digit directory numbers 01-99", total)
	}
}

// Runs the Node behaviour scripts (vendored xterm in node:vm, no npm packages; also `npm run test:terminal-retronet`).
func TestDesktopTerminalBehaviourScripts(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not installed")
	}
	for _, name := range []string{"test-terminal-modem.mjs", "test-terminal-geometry.mjs", "test-terminal-retronet-directory.mjs", "test-terminal-retronet-session.mjs", "test-terminal-retronet-entries.mjs"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
			defer cancel()
			output, err := exec.CommandContext(ctx, node, filepath.Join("..", "scripts", name)).CombinedOutput()
			if err != nil {
				t.Fatalf("%s failed: %v\n%s", name, err, output)
			}
		})
	}
}

// Retro-Net session client (Contract D/E): one socket keyed by entry ID, echo modes, shared TerminalText widths.
// Behaviour is covered by scripts/test-terminal-retronet-session.mjs.
func TestDesktopTerminalRetroNetSessionContract(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/terminal-retronet-session.js")
	for _, want := range []string{
		"window.TerminalRetroNetSession = { open: open, hostKeyAnswer: hostKeyAnswer, c64ColorKey: c64ColorKey }",
		// C64 colour keys (Contract C): event.code, exact modifiers, the private sequence.
		"Digit1: 0x90, Digit2: 0x05, Digit3: 0x1c, Digit4: 0x9f, Digit5: 0x9c,",
		"Digit6: 0x1e, Digit7: 0x1f, Digit8: 0x9e, Digit9: 0x12, Digit0: 0x92",
		"Digit1: 0x81, Digit2: 0x95, Digit3: 0x96, Digit4: 0x97, Digit5: 0x98, Digit6: 0x99, Digit7: 0x9a, Digit8: 0x9b",
		"if (!event || !event.shiftKey || event.metaKey || event.ctrlKey === event.altKey) return '';",
		`'\x1b[64;' + keys[event.code] + '~'`,
		"const TerminalText = window.TerminalText",
		"const width = TerminalText.cells(String(text || '').split('\\t').join(' '));",
		"line = lineText(text)",
		`split('\t').map(TerminalText.printable).join('\t')`,
		"const start = lastCharacterStart(chars)",
		"KEYS_IGNORED.indexOf(input) >= 0",
		"input.replace(ESCAPES, '')",
		"if (!isOpen() || typeof input !== 'string' || !input) return",
		"'/api/desktop/retronet/connect?entry='",
		"encodeURIComponent(String(entry.id || ''))",
		"'&cols=' + cols + '&rows=' + rows",
		"ws.binaryType = 'arraybuffer'",
		"new TextEncoder()",
		"event.data instanceof ArrayBuffer",
		"type: 'resize'",
		"type: 'hostkey_decision'",
		"control.type === 'echo'",
		"setEchoMode(control.remote === true, control.hidden === true)",
		`if (term && text && !hiddenEcho) term.write(TerminalText.printable(text.split('\t').join(' ')))`,
		"if (!secret) remember(text)",
		"if (!hiddenEcho) recall(-1)",
		"if (lineCapable && !remoteEcho)",
		"entry.protocol === 'telnet' && entry.kind === 'world'",
		"HISTORY_LIMIT = 50",
		"FRAME_BYTES = 16 * 1024",
		`'\x1b[A'`,
		`sendBytes(text + '\r')`,
		"function hostKeyAnswer(input, yes, no)",
		"if (key === 'y') return true",
		"if (key === 'n') return false",
		"return { send: send, resize: resize, hostKeyDecision: hostKeyDecision, hangup: shutdown, dispose: shutdown }",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("terminal-retronet-session.js missing %q", want)
		}
	}
	if strings.Count(source, "new WebSocket") != 1 {
		t.Fatal("terminal-retronet-session.js must create exactly one WebSocket")
	}
	// Typed text and history (a hidden password buffer included) are dropped on hang-up, dispose and remote close.
	if !strings.Contains(source, "function forget()") || strings.Count(strings.ReplaceAll(source, "\r\n", "\n"), "detach();\n            forget();") != 2 {
		t.Fatal("terminal-retronet-session.js must forget the line buffer and history in shutdown() and on remote close")
	}
	for _, forbidden := range []string{"&host=", "&port=", "?host=", "innerHTML", "function cellWidth", "function cells", "function fitToCells", "function printable", "0x1f300", `\u001f`} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("terminal-retronet-session.js dials by entry ID only and uses the shared TerminalText helpers; found %q", forbidden)
		}
	}
}

// Own-entry editor (Contract E): native dialogs, text only through textContent/value, the stored document keys
// and the server's field rules mirrored for UX. Behaviour is covered by scripts/test-terminal-retronet-entries.mjs.
func TestDesktopTerminalRetroNetEntriesContract(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/terminal-retronet-entries.js")
	for _, want := range []string{
		"window.TerminalRetroNetEntries = { open: open, confirmDelete: confirmDelete }",
		"document.createElement('dialog')",
		"dialog.showModal()",
		"const SETTING_KEY = 'retronet.entries'",
		"api('/api/desktop/settings', {",
		"method: 'PUT'",
		"key: SETTING_KEY",
		"JSON.stringify({ version: 1, entries: entries })",
		"window.crypto.getRandomValues",
		"'own-' + chars.join('')",
		"const ID_LENGTH = 12",
		"const MAX_ENTRIES = 50",
		"const BLOCKED_PORTS = [25, 465, 587]",
		`const INVISIBLE = /[\p{Cc}\p{Co}\p{Cs}\p{Zl}\p{Zp}\ufffd]|(?!\u200d)\p{Cf}/u`,
		`const VISIBLE = /[^\p{White_Space}\u200d]/u`,
		"const HEX_LABEL = /^0x[0-9a-f]*$/",
		"const SSH_USER = /^[a-z0-9._-]{1,32}$/",
		"draft.host_key = entry.host_key",
		"api('/api/desktop/retronet/directory')",
		"next[index] = keepHostKey(draft, stored[index])",
		"localizedError('desktop.terminal_retronet_error_gone')",
		"dialog.addEventListener('keydown'",
		"if (state.busy && dialog.isConnected)",
		"'data-retronet-confirm'",
		"tr('desktop.terminal_retronet_discard_question')",
		"tr('desktop.terminal_retronet_keep_editing')",
		"tr('desktop.terminal_retronet_discard')",
		"'data-terminal-retronet-dialog'",
		"'data-retronet-error'",
		".textContent = ",
		"tr('desktop.terminal_retronet_error_save', { message: errorText(err, tr) })",
		"tr('desktop.terminal_retronet_error_delete', { message: errorText(err, tr) })",
		"tr('desktop.terminal_retronet_delete_confirm', { name: entry.name })",
		"tr('desktop.terminal_retronet_type_bbs')",
		"tr('desktop.terminal_retronet_type_world')",
		// C64 entries (Contract C): type c64, charset fixed to petscii (UX mirror of kind c64 <=> charset petscii).
		"const KINDS = ['bbs', 'world', 'c64']",
		"tr('desktop.terminal_retronet_type_c64')",
		"if (kind === 'c64') return 'petscii';",
		"(c64 && row.getAttribute('data-retronet-field') === 'charset')",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("terminal-retronet-entries.js missing %q", want)
		}
	}
	for _, forbidden := range []string{"alert(", "window.confirm", "prompt(", "new WebSocket", "innerHTML", "outerHTML", "insertAdjacentHTML", "document.write"} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("terminal-retronet-entries.js must use desktop dialogs built from DOM nodes only; found %q", forbidden)
		}
	}
}

// Retro-Net styles: theme-following toolbar controls and dialogs, LED states, the directory cursor and the 80x25
// letterbox. The dialing blink runs only without reduced motion and with desktop animations on.
func TestDesktopTerminalRetroNetStylesheet(t *testing.T) {
	t.Parallel()

	css := strings.ReplaceAll(readDesktopAssetText(t, "css/desktop-app-terminal.css"), "\r\n", "\n")
	for _, want := range []string{
		".vd-terminal-toolbar select[data-terminal-baud]",
		`.vd-terminal-toolbar [data-terminal-retronet-action="hangup"]`,
		`.vd-terminal-app[data-terminal-mode="directory"] .xterm`,
		`[data-terminal-state="desktop.terminal_connected"] .vd-terminal-led`,
		`[data-terminal-state="desktop.terminal_directory"] .vd-terminal-led`,
		`[data-terminal-state="desktop.terminal_dialing"] .vd-terminal-led`,
		"@keyframes vd-terminal-led-dial",
		"@media (prefers-reduced-motion: no-preference)",
		`body:not([data-animations="false"])`,
		`.vd-terminal-app[data-terminal-geometry="bbs"]:not([data-terminal-style="modern"]) .vd-terminal-screen`,
		`.vd-terminal-app[data-terminal-geometry="bbs"] .xterm .xterm-viewport`,
		`.vd-terminal-app[data-terminal-geometry="c64"] .xterm .xterm-viewport`,
		".vd-terminal-app[data-terminal-geometry=\"c64\"] .vd-terminal-screen {\n    background: #706DEB;\n}",
		".vd-terminal-retronet-dialog {",
		".vd-terminal-retronet-dialog::backdrop",
		".vd-terminal-retronet-fields label[hidden]",
		".vd-terminal-retronet-error",
		".vd-terminal-retronet-primary",
		".vd-terminal-retronet-danger",
		"var(--vd-theme-panel-bg",
		"var(--vd-theme-control-bg",
		"var(--vd-theme-border",
		"var(--vd-text",
		"var(--vd-font-ui",
		".vd-terminal-retronet-confirm",
		".vd-terminal-retronet-dialog footer[hidden]",
	} {
		if !strings.Contains(css, want) {
			t.Fatalf("desktop-app-terminal.css missing %q", want)
		}
	}
	// Accents differ per theme and several are light, so the Save button keeps the theme's text color on an
	// accent wash (the Local Wikipedia pattern) instead of white text on the raw accent.
	primary := css[strings.Index(css, ".vd-terminal-retronet-dialog .vd-terminal-retronet-primary"):]
	primary = primary[:strings.Index(primary, "}")]
	if strings.Contains(primary, "#fff") || !strings.Contains(primary, "var(--vd-theme-accent-soft") {
		t.Fatalf("the Save button must use theme text on --vd-theme-accent-soft, got %q", primary)
	}
	const blink = "animation: vd-terminal-led-dial"
	if strings.Count(css, blink) != 1 {
		t.Fatalf("desktop-app-terminal.css must start the dialing blink exactly once; found %d", strings.Count(css, blink))
	}
	at := strings.Index(css, blink)
	gate := strings.LastIndex(css[:at], "@media (prefers-reduced-motion: no-preference)")
	if gate < 0 || strings.Contains(css[gate:at], "}\n}") || !strings.Contains(css[gate:at], `body:not([data-animations="false"])`) {
		t.Fatal("the dialing blink must sit inside @media (prefers-reduced-motion: no-preference) behind body:not([data-animations=\"false\"])")
	}
}
