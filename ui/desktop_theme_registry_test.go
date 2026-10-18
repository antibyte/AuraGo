package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"aurago/internal/desktop"
)

// The Virtual Desktop theme registry (DESKTOP_THEMES in js/desktop/core/theme-runtime.js) is the single
// description of what each theme provides. These tests parse it as text, so it must keep one descriptor per
// line with single-quoted strings and the keys in desktopThemeKeyOrder.

// desktopThemeKeyOrder is the property order of every descriptor line.
var desktopThemeKeyOrder = []string{
	"id", "labelKey", "descKey", "scheme", "schemeSetting", "shell", "globalMenu", "iconTheme", "iconFamily",
	"accent", "glassPolish", "fx", "wallpaper", "thumb", "motion",
}

// desktopThemeMotionKeys are the eight motion durations every descriptor carries (milliseconds).
var desktopThemeMotionKeys = []string{"open", "close", "minimize", "restore", "menuOpen", "menuClose", "startOpen", "startClose"}

// desktopThemeLocales are the shipped desktop UI languages.
var desktopThemeLocales = []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"}

type desktopThemeDescriptor struct {
	ID            string
	LabelKey      string
	DescKey       string
	Scheme        string
	SchemeSetting string
	Shell         string
	GlobalMenu    bool
	IconTheme     string
	IconFamily    string
	Accent        string
	GlassPolish   bool
	FX            bool
	Wallpaper     string
	Thumb         string
	Motion        map[string]int
}

var (
	desktopThemeMotionBlock = regexp.MustCompile(`motion: Object\.freeze\(\{([^}]*)\}\)`)
	desktopThemeKeyToken    = regexp.MustCompile(`\b(\w+): `)
	desktopThemeStringField = regexp.MustCompile(`\b(\w+): '([^']*)'`)
	desktopThemeBoolField   = regexp.MustCompile(`\b(\w+): (true|false)\b`)
	desktopThemeNumberField = regexp.MustCompile(`\b(\w+): (\d+)\b`)
)

// parseDesktopThemeRegistry reads the DESKTOP_THEMES block of the theme runtime in registry order.
func parseDesktopThemeRegistry(t testing.TB) []desktopThemeDescriptor {
	t.Helper()
	const path = "js/desktop/core/theme-runtime.js"
	source, err := Content.ReadFile(path)
	if err != nil {
		t.Fatalf("read the theme registry: %v", err)
	}
	text := strings.ReplaceAll(string(source), "\r\n", "\n")
	const opening = "const DESKTOP_THEMES = Object.freeze(["
	start := strings.Index(text, opening)
	if start < 0 {
		t.Fatalf("%s has no %q block", path, opening)
	}
	block := text[start+len(opening):]
	end := strings.Index(block, "]);")
	if end < 0 {
		t.Fatalf("the DESKTOP_THEMES block in %s is not closed with ]);", path)
	}
	var themes []desktopThemeDescriptor
	for _, line := range strings.Split(block[:end], "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		themes = append(themes, parseDesktopThemeLine(t, line))
	}
	if len(themes) == 0 {
		t.Fatalf("the DESKTOP_THEMES block in %s lists no theme", path)
	}
	return themes
}

func parseDesktopThemeLine(t testing.TB, line string) desktopThemeDescriptor {
	t.Helper()
	if !strings.HasPrefix(line, "Object.freeze({ id: '") || !strings.HasSuffix(strings.TrimRight(line, ","), "})") {
		t.Fatalf("a DESKTOP_THEMES descriptor must be one `Object.freeze({ id: '...', ... })` per line, got %q", line)
	}
	block := desktopThemeMotionBlock.FindStringSubmatch(line)
	if block == nil {
		t.Fatalf("descriptor has no `motion: Object.freeze({ ... })`: %q", line)
	}
	motion := map[string]int{}
	for _, kv := range desktopThemeNumberField.FindAllStringSubmatch(block[1], -1) {
		if _, duplicate := motion[kv[1]]; duplicate {
			t.Fatalf("descriptor lists the motion key %q twice: %q", kv[1], line)
		}
		milliseconds, err := strconv.Atoi(kv[2])
		if err != nil {
			t.Fatalf("motion.%s is not a number in %q", kv[1], line)
		}
		motion[kv[1]] = milliseconds
	}
	rest := strings.Replace(line, block[0], "motion: MOTION", 1)

	var keys []string
	for _, match := range desktopThemeKeyToken.FindAllStringSubmatch(rest, -1) {
		keys = append(keys, match[1])
	}
	if !reflect.DeepEqual(keys, desktopThemeKeyOrder) {
		t.Fatalf("descriptor has the keys %v, want %v in this order: %q", keys, desktopThemeKeyOrder, line)
	}
	strs := map[string]string{}
	for _, match := range desktopThemeStringField.FindAllStringSubmatch(rest, -1) {
		strs[match[1]] = match[2]
	}
	bools := map[string]bool{}
	for _, match := range desktopThemeBoolField.FindAllStringSubmatch(rest, -1) {
		bools[match[1]] = match[2] == "true"
	}
	needString := func(key string) string {
		value, ok := strs[key]
		if !ok {
			t.Fatalf("descriptor key %q must be a single-quoted string: %q", key, line)
		}
		return value
	}
	needBool := func(key string) bool {
		value, ok := bools[key]
		if !ok {
			t.Fatalf("descriptor key %q must be true or false: %q", key, line)
		}
		return value
	}
	return desktopThemeDescriptor{
		ID:            needString("id"),
		LabelKey:      needString("labelKey"),
		DescKey:       needString("descKey"),
		Scheme:        needString("scheme"),
		SchemeSetting: needString("schemeSetting"),
		Shell:         needString("shell"),
		GlobalMenu:    needBool("globalMenu"),
		IconTheme:     needString("iconTheme"),
		IconFamily:    needString("iconFamily"),
		Accent:        needString("accent"),
		GlassPolish:   needBool("glassPolish"),
		FX:            needBool("fx"),
		Wallpaper:     needString("wallpaper"),
		Thumb:         needString("thumb"),
		Motion:        motion,
	}
}

// desktopSettingValues returns the allowed values of a desktop setting that lists them.
func desktopSettingValues(t testing.TB, key string) []string {
	t.Helper()
	for _, def := range desktop.DesktopSettingDefinitions() {
		if def.Key == key {
			if len(def.Values) == 0 {
				t.Fatalf("desktop setting %s has no fixed value list", key)
			}
			return def.Values
		}
	}
	t.Fatalf("desktop setting %s is not defined", key)
	return nil
}

func desktopThemeContains(list []string, want string) bool {
	for _, item := range list {
		if item == want {
			return true
		}
	}
	return false
}

// readDesktopThemeSource reads a UI source file as is (readDesktopAssetText would follow bundle loaders).
func readDesktopThemeSource(t testing.TB, path string) string {
	t.Helper()
	data, err := Content.ReadFile(filepath.ToSlash(path))
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

// desktopBundleParts returns the part list that scripts/build-ui-bundles.js assembles into the bundle
// written to out (a repository-relative path such as ui/css/desktop-shell.bundle.css). Part lists may be
// inline arrays or names of const arrays, and an array may spread another one.
func desktopBundleParts(t testing.TB, out string) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "scripts", "build-ui-bundles.js"))
	if err != nil {
		t.Fatalf("read scripts/build-ui-bundles.js: %v", err)
	}
	script := strings.ReplaceAll(string(data), "\r\n", "\n")
	marker := "out: '" + out + "'"
	at := strings.Index(script, marker)
	if at < 0 {
		t.Fatalf("scripts/build-ui-bundles.js has no bundle written to %s", out)
	}
	entry := script[at+len(marker):]
	partsAt := strings.Index(entry, "parts:")
	if partsAt < 0 {
		t.Fatalf("the bundle entry for %s has no parts:", out)
	}
	if next := strings.Index(entry, "out: '"); next >= 0 && next < partsAt {
		t.Fatalf("the bundle entry for %s has no parts: before the next entry", out)
	}
	return resolveBundlePartsExpr(t, script, strings.TrimLeft(entry[partsAt+len("parts:"):], " \t\r\n"))
}

var (
	bundlePartsIdentifier  = regexp.MustCompile(`^[A-Za-z_]\w*`)
	bundlePartsLineComment = regexp.MustCompile(`(?m)//.*$`)
)

func resolveBundlePartsExpr(t testing.TB, script, expr string) []string {
	t.Helper()
	if strings.HasPrefix(expr, "[") {
		end := strings.Index(expr, "]")
		if end < 0 {
			t.Fatal("unterminated parts array in scripts/build-ui-bundles.js")
		}
		return parseBundlePartsArray(t, script, expr[1:end])
	}
	name := bundlePartsIdentifier.FindString(expr)
	if name == "" {
		t.Fatalf("scripts/build-ui-bundles.js: cannot read the parts expression %.40q", expr)
	}
	declaration := regexp.MustCompile(`const ` + regexp.QuoteMeta(name) + ` = \[`).FindStringIndex(script)
	if declaration == nil {
		t.Fatalf("scripts/build-ui-bundles.js: no `const %s = [`", name)
	}
	return resolveBundlePartsExpr(t, script, script[declaration[1]-1:])
}

func parseBundlePartsArray(t testing.TB, script, body string) []string {
	t.Helper()
	body = bundlePartsLineComment.ReplaceAllString(body, "")
	var parts []string
	for _, item := range strings.Split(body, ",") {
		item = strings.TrimSpace(item)
		switch {
		case item == "":
		case strings.HasPrefix(item, "..."):
			parts = append(parts, resolveBundlePartsExpr(t, script, strings.TrimPrefix(item, "..."))...)
		case len(item) >= 2 && (item[0] == '\'' || item[0] == '"') && item[len(item)-1] == item[0]:
			parts = append(parts, item[1:len(item)-1])
		default:
			t.Fatalf("scripts/build-ui-bundles.js: unexpected parts item %q", item)
		}
	}
	return parts
}

func TestDesktopThemeRegistryMatchesGoAndAssets(t *testing.T) {
	t.Parallel()
	themes := parseDesktopThemeRegistry(t)
	var ids []string
	for _, theme := range themes {
		ids = append(ids, theme.ID)
	}
	if want := desktop.DesktopThemeIDs(); !reflect.DeepEqual(ids, want) {
		t.Fatalf("DESKTOP_THEMES ids %v differ from desktop.DesktopThemeIDs() %v (ids and order must match)", ids, want)
	}
	if ids[0] != desktop.DefaultDesktopTheme {
		t.Errorf("the first registry theme is %q, want the default %q", ids[0], desktop.DefaultDesktopTheme)
	}

	idPattern := regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	iconThemes := desktopSettingValues(t, "appearance.icon_theme")
	wallpapers := desktopSettingValues(t, "appearance.wallpaper")
	fxParts := desktopBundleParts(t, "ui/js/desktop/bundles/desktop-theme-fx.bundle.js")
	loader := readDesktopThemeSource(t, "js/desktop/core/module-loader.js")

	for _, theme := range themes {
		theme := theme
		t.Run(theme.ID, func(t *testing.T) {
			if !idPattern.MatchString(theme.ID) {
				t.Errorf("id %q does not match %s", theme.ID, idPattern)
			}
			if theme.ID == "light" || theme.ID == "dark" {
				t.Errorf("id %q is reserved for the colour scheme and cannot name a theme", theme.ID)
			}
			if theme.Scheme != "light" && theme.Scheme != "dark" {
				t.Errorf("scheme %q must be light or dark", theme.Scheme)
			}
			if theme.Shell != "taskbar" && theme.Shell != "dock" {
				t.Errorf("shell %q must be taskbar or dock", theme.Shell)
			}
			if theme.Shell == "dock" && !theme.GlobalMenu {
				t.Errorf("a dock shell needs the global menu bar (globalMenu: true)")
			}
			if theme.SchemeSetting != "" && theme.SchemeSetting != "appearance.fruity_mode" {
				t.Errorf("schemeSetting %q must be empty or appearance.fruity_mode", theme.SchemeSetting)
			}
			if !desktopThemeContains(iconThemes, theme.IconTheme) {
				t.Errorf("iconTheme %q is not an appearance.icon_theme value %v", theme.IconTheme, iconThemes)
			}
			if theme.IconFamily != "standard" && theme.IconFamily != "fruity" {
				t.Errorf("iconFamily %q must be standard or fruity", theme.IconFamily)
			}
			if theme.Accent != "user" && theme.Accent != "fixed" {
				t.Errorf("accent %q must be user or fixed", theme.Accent)
			}
			if !desktopThemeContains(wallpapers, theme.Wallpaper) {
				t.Errorf("wallpaper %q is not an appearance.wallpaper value", theme.Wallpaper)
			}
			if want := "/img/themes/" + theme.ID + ".svg"; theme.Thumb != want {
				t.Errorf("thumb %q, want %q", theme.Thumb, want)
			} else if _, err := Content.ReadFile("img/themes/" + theme.ID + ".svg"); err != nil {
				t.Errorf("thumbnail img/themes/%s.svg is missing: %v", theme.ID, err)
			}

			if len(theme.Motion) != len(desktopThemeMotionKeys) {
				t.Errorf("motion has %d keys, want exactly %v", len(theme.Motion), desktopThemeMotionKeys)
			}
			for _, key := range desktopThemeMotionKeys {
				ms, ok := theme.Motion[key]
				if !ok {
					t.Errorf("motion.%s is missing", key)
				} else if ms < 20 || ms > 800 {
					t.Errorf("motion.%s is %d ms, want 20..800", key, ms)
				}
			}

			if !theme.FX {
				return
			}
			rel := "js/desktop/theme-fx/" + theme.ID + ".js"
			source, err := Content.ReadFile(rel)
			if err != nil {
				t.Errorf("fx theme %s has no %s: %v", theme.ID, rel, err)
			} else if !strings.Contains(string(source), "register('"+theme.ID+"'") {
				t.Errorf("%s must register itself with register('%s', ...)", rel, theme.ID)
			}
			effect, core := indexOfBundlePart(fxParts, "ui/"+rel), indexOfBundlePart(fxParts, "ui/js/desktop/theme-fx/fx-core.js")
			switch {
			case effect < 0:
				t.Errorf("scripts/build-ui-bundles.js: desktop-theme-fx.bundle.js must list ui/%s", rel)
			case core < 0 || core > effect:
				t.Errorf("scripts/build-ui-bundles.js: ui/js/desktop/theme-fx/fx-core.js must come before ui/%s in desktop-theme-fx.bundle.js", rel)
			}
			if !strings.Contains(loader, "'desktop-theme-fx'") {
				t.Errorf("js/desktop/core/module-loader.js does not register the 'desktop-theme-fx' bundle")
			}
		})
	}
}

func indexOfBundlePart(parts []string, want string) int {
	for index, part := range parts {
		if part == want {
			return index
		}
	}
	return -1
}

func TestDesktopThemeRegistryTranslations(t *testing.T) {
	t.Parallel()
	themes := parseDesktopThemeRegistry(t)
	keys := []string{"desktop.settings_theme", "desktop.settings_theme_desc", "desktop.settings_theme_picker_label"}
	for _, theme := range themes {
		keys = append(keys, theme.LabelKey, theme.DescKey)
	}

	entries, err := os.ReadDir(filepath.Join("lang", "desktop"))
	if err != nil {
		t.Fatalf("list the desktop locales: %v", err)
	}
	var shipped []string
	for _, entry := range entries {
		if name := entry.Name(); strings.HasSuffix(name, ".json") {
			shipped = append(shipped, strings.TrimSuffix(name, ".json"))
		}
	}
	if !reflect.DeepEqual(shipped, desktopThemeLocales) {
		t.Fatalf("lang/desktop holds the locales %v, but this test covers %v; update desktopThemeLocales", shipped, desktopThemeLocales)
	}

	for _, locale := range desktopThemeLocales {
		locale := locale
		t.Run(locale, func(t *testing.T) {
			t.Parallel()
			data, err := Content.ReadFile("lang/desktop/" + locale + ".json")
			if err != nil {
				t.Fatalf("read lang/desktop/%s.json: %v", locale, err)
			}
			var words map[string]interface{}
			if err := json.Unmarshal(data, &words); err != nil {
				t.Fatalf("parse lang/desktop/%s.json: %v", locale, err)
			}
			for _, key := range keys {
				if value, ok := words[key].(string); !ok || strings.TrimSpace(value) == "" {
					t.Errorf("lang/desktop/%s.json: %s is missing or empty", locale, key)
				}
			}
		})
	}
}

func TestDesktopThemeRegistryShellUsesCapabilities(t *testing.T) {
	t.Parallel()
	// The shell asks the registry what a theme provides; isFruityTheme survives only as its definition.
	shell := readDesktopAssetText(t, "js/desktop/main.js")
	if count := strings.Count(shell, "isFruityTheme("); count != 1 {
		t.Errorf("the desktop shell mentions isFruityTheme( %d times, want exactly 1 (the definition)", count)
	}

	// Apps read the published colour scheme of the body, never the theme id. Layerling and Leafy alias
	// document.body.dataset to a local name first, hence the second spelling.
	schemeRead := regexp.MustCompile(`\b(?:dataset|bodyData)\.scheme\b`)
	for _, path := range []string{
		"js/desktop/apps/code-studio/editor.js",
		"js/desktop/apps/cyd-creator-code.js",
		"js/desktop/apps/layerling.js",
		"js/desktop/leafy/runtime.js",
	} {
		if !schemeRead.MatchString(readDesktopThemeSource(t, path)) {
			t.Errorf("%s does not read the body data-scheme (dataset.scheme)", path)
		}
	}
}
