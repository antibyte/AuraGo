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
		`'/js/desktop/apps/terminal.js'`,
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
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("terminal-audio.js missing %q", want)
		}
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
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("terminal.js missing %q", want)
		}
	}
	if strings.Count(source, "new WebSocket") != 1 {
		t.Fatal("style changes must not create extra WebSocket constructors")
	}
	if strings.Contains(source, "onclick=") {
		t.Fatal("terminal.js must not use inline onclick")
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
	"desktop.terminal_retronet_delete_title",
	"desktop.terminal_retronet_delete_confirm",
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
}

func terminalRetroNetI18nKeys(t *testing.T) []string {
	t.Helper()
	keys := append([]string{}, terminalRetroNetFixedKeys...)
	for _, category := range terminalRetroNetCategories {
		keys = append(keys, "desktop.terminal_retronet_cat_"+category)
	}
	for _, state := range []string{"online", "offline", "unknown"} {
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
	german := readTerminalDesktopLocale(t, "de")
	for key, umlaut := range map[string]string{
		"desktop.terminal_retronet_cat_own":                 "ä",
		"desktop.terminal_retronet_telnet_notice":           "ü",
		"desktop.terminal_retronet_press_key":               "ü",
		"desktop.terminal_retronet_result_hostkey_mismatch": "ü",
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
		"window.TerminalText = { cellWidth: cellWidth, fitToCells: fitToCells, printable: printable }",
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
		"['classics', 'bbs', 'muds', 'games', 'own']",
		"online: '[*]'",
		"offline: '[ ]'",
		"unknown: '[?]'",
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
		"function cellWidth", "function fitToCells", "function printable", "ZERO_WIDTH", "WIDE_EMOJI", "activeVersion", `\u001f`} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("terminal-retronet-directory.js must render only into xterm and use the shared TerminalText helpers; found %q", forbidden)
		}
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
	for _, name := range []string{"test-terminal-modem.mjs", "test-terminal-retronet-directory.mjs", "test-terminal-retronet-session.mjs", "test-terminal-retronet-entries.mjs"} {
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
		"window.TerminalRetroNetSession = { open: open, hostKeyAnswer: hostKeyAnswer }",
		"const TerminalText = window.TerminalText",
		"TerminalText.cellWidth(ch.codePointAt(0))",
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
	for _, forbidden := range []string{"&host=", "&port=", "?host=", "innerHTML", "function cellWidth", "function fitToCells", "function printable", "0x1f300", `\u001f`} {
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
		"const MAX_ENTRIES = 64",
		"const BLOCKED_PORTS = [25, 465, 587]",
		`const INVISIBLE = /[\p{Cc}\p{Co}\p{Cs}\p{Zl}\p{Zp}\ufffd]|(?!\u200d)\p{Cf}/u`,
		`const VISIBLE = /[^\p{White_Space}\u200d]/u`,
		"const HEX_LABEL = /^0x[0-9a-f]*$/",
		"const SSH_USER = /^[a-z0-9._-]{1,32}$/",
		"draft.host_key = entry.host_key",
		"api('/api/desktop/retronet/directory')",
		"next[index] = keepHostKey(draft, stored[index])",
		"localizedError('desktop.terminal_retronet_error_gone')",
		"'data-terminal-retronet-dialog'",
		"'data-retronet-error'",
		".textContent = ",
		"tr('desktop.terminal_retronet_error_save', { message: errorText(err, tr) })",
		"tr('desktop.terminal_retronet_error_delete', { message: errorText(err, tr) })",
		"tr('desktop.terminal_retronet_delete_confirm', { name: entry.name })",
		"tr('desktop.terminal_retronet_type_bbs')",
		"tr('desktop.terminal_retronet_type_world')",
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
	} {
		if !strings.Contains(css, want) {
			t.Fatalf("desktop-app-terminal.css missing %q", want)
		}
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
