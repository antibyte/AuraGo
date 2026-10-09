package ui

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
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
