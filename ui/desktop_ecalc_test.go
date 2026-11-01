package ui

import (
	"io/fs"
	"regexp"
	"strings"
	"testing"
)

// The ECalc app is a set of classic scripts in js/desktop/apps/ecalc/ plus css/desktop-app-ecalc.css. The tests here
// are source markers in the style of desktop_calculator_test.go; the behaviour is covered by scripts/test-ecalc-core.mjs
// (the core) and desktop_ecalc_browser_test.go (the page).

const ecalcDir = "js/desktop/apps/ecalc"

func ecalcScripts(t *testing.T) map[string]string {
	t.Helper()
	entries, err := fs.ReadDir(Content, ecalcDir)
	if err != nil {
		t.Fatalf("read %s: %v", ecalcDir, err)
	}
	scripts := map[string]string{}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".js") {
			scripts[entry.Name()] = readDesktopAssetText(t, ecalcDir+"/"+entry.Name())
		}
	}
	if len(scripts) != 11 {
		t.Fatalf("%s holds %d scripts, want the 11 of the plan", ecalcDir, len(scripts))
	}
	return scripts
}

func assertECalcMarkersInOrder(t *testing.T, source string, markers []string) {
	t.Helper()
	offset := 0
	for _, marker := range markers {
		index := strings.Index(source[offset:], marker)
		if index < 0 {
			t.Fatalf("marker %q missing after offset %d", marker, offset)
		}
		offset += index + len(marker)
	}
}

func TestECalcDoesNotUseDynamicEvaluation(t *testing.T) {
	t.Parallel()
	forbidden := map[string]string{
		"Function call":           `\bFunction\s*\(`,
		"new Function":            `\bnew\s+Function\b`,
		"eval":                    `\beval\s*\(`,
		"globalThis.Function":     `\bglobalThis\s*\.\s*Function\b`,
		"window.Function":         `\bwindow\s*\.\s*Function\b`,
		"window bracket Function": `\bwindow\s*\[\s*['"]Function['"]\s*\]`,
		"constructor constructor": `\bconstructor\s*\.\s*constructor\b`,
		"string timer":            `\bset(?:Timeout|Interval)\s*\(\s*['"]`,
	}
	for name, script := range ecalcScripts(t) {
		for label, pattern := range forbidden {
			if regexp.MustCompile(pattern).MatchString(script) {
				t.Errorf("%s must not use %s", name, label)
			}
		}
	}
	expr := ecalcScripts(t)["ecalc-expr.js"]
	for _, marker := range []string{"function tokenize(", "function parseTokens(", "function evaluate(", "case 'sqrt': return Math.sqrt(x[0]);", "const ARITY = {", "throw fail('unknown function '"} {
		if !strings.Contains(expr, marker) {
			t.Errorf("ecalc-expr.js is missing the safe parser marker %q", marker)
		}
	}
}

// Every storage access sits in a try block, so a browser that blocks storage still gets a working app.
func TestECalcStorageIsGuarded(t *testing.T) {
	t.Parallel()
	for name, script := range ecalcScripts(t) {
		if name != "ecalc-store.js" && strings.Contains(script, "localStorage") {
			t.Errorf("%s touches localStorage; only ecalc-store.js may", name)
		}
	}
	store := ecalcScripts(t)["ecalc-store.js"]
	if !strings.Contains(store, "const KEY = 'aurago.ecalc.v1';") {
		t.Error("ecalc-store.js must keep its data under aurago.ecalc.v1")
	}
	lines := strings.Split(store, "\n")
	guarded := 0
	for i, line := range lines {
		if !strings.Contains(line, "backend()") || strings.Contains(line, "const backend") {
			continue
		}
		guarded++
		window := strings.Join(lines[max(0, i-2):i+1], "\n")
		if !strings.Contains(window, "try {") {
			t.Errorf("ecalc-store.js line %d reads or writes storage outside a try block: %s", i+1, strings.TrimSpace(line))
		}
	}
	if guarded != 2 {
		t.Errorf("ecalc-store.js has %d storage accesses, want the read and the write", guarded)
	}
	if strings.Count(store, "catch (error)") < 2 {
		t.Error("ecalc-store.js must catch the storage errors")
	}
}

// Ctrl/Cmd+K focuses the in-app search and must not reach the desktop's Spotlight shortcut on the document.
func TestECalcSearchShortcutStopsPropagation(t *testing.T) {
	t.Parallel()
	root := ecalcScripts(t)["ecalc.js"]
	assertECalcMarkersInOrder(t, root, []string{
		"root.addEventListener('keydown', event => {",
		"(event.ctrlKey || event.metaKey) && !event.altKey && !event.shiftKey && event.key.toLowerCase() === 'k'",
		"event.preventDefault();",
		"event.stopPropagation();",
		"shell.focusSearch()",
	})
	for _, marker := range []string{"function dispose(id)", "window.ECalcApp = { render, dispose };", "ctx.api(catalogURL(lang))", "desktop.ecalc_load_failed", "desktop.ecalc_retry"} {
		if !strings.Contains(root, marker) {
			t.Errorf("ecalc.js is missing %q", marker)
		}
	}
	shell := ecalcScripts(t)["ecalc-shell.js"]
	for _, marker := range []string{"const COMPACT_BELOW = 720;", "ui.compact", "data-action=\"back\""} {
		if !strings.Contains(shell, marker) {
			t.Errorf("ecalc-shell.js is missing %q", marker)
		}
	}
}

// The app only speaks through desktop.ecalc_* and desktop.app_ecalc keys, and the dimension names that are looked up
// dynamically exist in every language.
func TestECalcUsesOnlyItsOwnTranslationKeys(t *testing.T) {
	t.Parallel()
	allowed := map[string]bool{}
	for _, key := range strings.Fields(`app_ecalc ecalc_search_placeholder ecalc_favorites ecalc_recent ecalc_empty_favorites ecalc_empty_recent
		ecalc_no_results ecalc_favorite_add ecalc_favorite_remove ecalc_copy_results ecalc_copied ecalc_reset ecalc_explanation ecalc_source
		ecalc_derived ecalc_eseries_nearest ecalc_eseries_series ecalc_lock ecalc_unlock ecalc_unit_auto ecalc_error_undetermined
		ecalc_error_nonfinite ecalc_error_range ecalc_error_parse ecalc_error_locked ecalc_settings ecalc_settings_digits ecalc_settings_symbols ecalc_symbols_iec
		ecalc_symbols_ansi ecalc_back ecalc_load_failed ecalc_retry ecalc_warnings ecalc_inputs ecalc_computed ecalc_dim_voltage ecalc_dim_current
		ecalc_dim_resistance ecalc_dim_power ecalc_dim_charge ecalc_dim_energy ecalc_dim_number ecalc_dim_gain`) {
		allowed["desktop."+key] = true
	}
	keyPattern := regexp.MustCompile(`['"](desktop\.[A-Za-z0-9_.]+)['"]`)
	used := 0
	for name, script := range ecalcScripts(t) {
		for _, match := range keyPattern.FindAllStringSubmatch(script, -1) {
			used++
			key := match[1]
			if key == "desktop.ecalc_dim_" {
				continue // the prefix of the dimension names
			}
			if !allowed[key] {
				t.Errorf("%s uses the translation key %s, which is not one of the ECalc keys", name, key)
			}
		}
	}
	if used < 30 {
		t.Fatalf("found only %d translation keys in the ECalc scripts, the pattern no longer matches", used)
	}
	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		words := readMergedLangMap(t, lang)
		for _, dim := range []string{"voltage", "current", "resistance", "power", "charge", "energy", "number", "gain"} {
			if strings.TrimSpace(words["desktop.ecalc_dim_"+dim]) == "" {
				t.Errorf("%s misses desktop.ecalc_dim_%s, which the unit dropdown looks up dynamically", lang, dim)
			}
		}
	}
}

// The stylesheet takes every colour from tokens and gates its motion.
func TestECalcStylesheetUsesOnlyTokens(t *testing.T) {
	t.Parallel()
	css := readDesktopAssetText(t, "css/desktop-app-ecalc.css")
	body := regexp.MustCompile(`(?s)/\*.*?\*/`).ReplaceAllString(css, "")
	for label, pattern := range map[string]string{
		"hex colour":          `#[0-9a-fA-F]{3,8}\b`,
		"colour function":     `\b(?:rgba?|hsla?|hwb|lab|lch|oklab|oklch)\(\s*[\d.]`,
		"named colour":        `(?i)[:\s(,](?:white|black|red|green|blue|gray|grey|orange|yellow|purple)\b\s*[;,)]`,
		"external url":        `url\(`,
		"import":              `@import`,
		"theme id":            `data-theme=`,
		"cookie banner class": `\.cc-`,
	} {
		if match := regexp.MustCompile(pattern).FindString(body); match != "" {
			t.Errorf("css/desktop-app-ecalc.css has a %s: %q", label, match)
		}
	}
	if strings.Count(body, "!important") != 1 || !strings.Contains(body, ".ec-app [hidden] { display: none !important; }") {
		t.Error("the only !important allowed is the [hidden] rule")
	}
	for _, marker := range []string{"var(--vd-mat-canvas", "var(--vd-mat-bar", "var(--vd-mat-side", "var(--vd-mat-card", "var(--vd-mat-well", "var(--vd-mat-field", "var(--vd-mat-btn", "var(--vd-mat-selected", "var(--ds-color-success-fg)", "var(--ds-color-danger-fg)", "var(--ds-color-border-focus)", `.ec-app[data-compact="true"]`, `.ec-field[data-state="computed"]`, `.ec-field[data-state="error"]`} {
		if !strings.Contains(body, marker) {
			t.Errorf("css/desktop-app-ecalc.css is missing %q", marker)
		}
	}
	gate := strings.Index(body, "@media (prefers-reduced-motion: no-preference)")
	if gate < 0 {
		t.Fatal("css/desktop-app-ecalc.css has no reduced-motion gate")
	}
	for _, motion := range regexp.MustCompile(`\b(?:transition|animation)\s*:`).FindAllStringIndex(body, -1) {
		if motion[0] < gate {
			t.Errorf("a transition or animation at offset %d sits outside @media (prefers-reduced-motion: no-preference)", motion[0])
		}
	}
}

// ECalc is registered in the loader, the router, the global-name table and the window presets, and the bundle holds them.
func TestECalcIsWiredIntoTheDesktopCore(t *testing.T) {
	t.Parallel()
	loader := readDesktopAssetText(t, "js/desktop/core/module-loader.js")
	start := strings.Index(loader, "        ecalc: {")
	if start < 0 {
		t.Fatal("module-loader.js has no ecalc entry in DESKTOP_APP_ASSETS")
	}
	entry := loader[start : start+strings.Index(loader[start:], "\n        },")]
	if !strings.Contains(entry, "appStyles('/css/desktop-app-ecalc.css')") {
		t.Error("the ecalc loader entry must load css/desktop-app-ecalc.css")
	}
	order := []string{"ecalc-expr.js", "ecalc-units.js", "ecalc-eseries.js", "ecalc-solve.js", "ecalc-typeset.js", "ecalc-symbols.js", "ecalc-schematics.js", "ecalc-store.js", "ecalc-calc-page.js", "ecalc-shell.js", "/ecalc.js'"}
	offset := 0
	for _, script := range order {
		index := strings.Index(entry[offset:], script)
		if index < 0 {
			t.Fatalf("the ecalc loader entry lists %s out of order or not at all (the core first, the composition root last)", script)
		}
		offset += index + len(script)
	}
	for _, check := range []struct{ file, marker string }{
		{"js/desktop/core/menus-and-routing.js", "if (appId === 'ecalc') {"},
		{"js/desktop/core/menus-and-routing.js", "window.ECalcApp.render(contentEl(id), id,"},
		{"js/desktop/core/desktop-foundation.js", "ecalc: 'ECalcApp',"},
		{"js/desktop/core/window-shell-runtime.js", "ecalc: { width: 1100, height: 760 },"},
		{"js/desktop/core/window-shell-runtime.js", "if (appId === 'ecalc') return { width: 640, height: 480 };"},
		{"js/desktop/bundles/main.bundle.js", "ecalc: 'ECalcApp',"},
		{"js/desktop/bundles/main.bundle.js", "window.ECalcApp.render(contentEl(id), id,"},
		{"js/desktop/bundles/main.bundle.js", "if (appId === 'ecalc') return { width: 640, height: 480 };"},
	} {
		if !strings.Contains(readDesktopAssetText(t, check.file), check.marker) {
			t.Errorf("%s is missing %q", check.file, check.marker)
		}
	}
}
