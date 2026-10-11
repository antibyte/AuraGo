package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"aurago/internal/desktop"
)

// Static contracts of the CYD Creator shell (plan 4a): registration, translations, modules, loader
// order with the insertion markers of plans 4b and 5, lifecycle and stylesheet.

var cydCreatorLocales = []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"}

func TestCYDCreatorManifestAndIcons(t *testing.T) {
	var app *desktop.AppManifest
	for _, candidate := range desktop.BuiltinApps() {
		if candidate.ID == "cyd-creator" {
			found := candidate
			app = &found
		}
	}
	if app == nil {
		t.Fatal("cyd-creator is not a builtin app")
	}
	if app.Name != "CYD Creator" || app.Icon != "cyd-creator" || app.Category != "dev" || app.Entry != "builtin://cyd-creator" ||
		strings.Join(app.Requires, ",") != "cyd_creator" || app.Metadata["open_maximized"] != "true" || !app.StartVisible || !app.DockVisible {
		t.Fatalf("manifest = %+v", *app)
	}
	catalog := desktop.DesktopIconCatalog(map[string]string{"appearance.icon_theme": "papirus"})
	if !containsString(catalog.Preferred, "cyd-creator") {
		t.Fatal("cyd-creator is not a preferred desktop icon")
	}
	for _, theme := range []string{"papirus", "whitesur"} {
		var manifest struct {
			Icons map[string]string `json:"icons"`
		}
		if err := json.Unmarshal([]byte(rawDesktopAssetText(t, "img/"+theme+"/manifest.json")), &manifest); err != nil {
			t.Fatal(err)
		}
		path := manifest.Icons["cyd-creator"]
		if path != "img/"+theme+"/icons/cyd-creator.svg" {
			t.Fatalf("%s manifest icon path = %q", theme, path)
		}
		svg, err := os.ReadFile(filepath.FromSlash(path))
		if err != nil || !strings.Contains(string(svg), "<svg") {
			t.Fatalf("%s cyd-creator icon: %v", theme, err)
		}
	}
	for _, lang := range cydCreatorLocales {
		var words map[string]string
		if err := json.Unmarshal([]byte(rawDesktopAssetText(t, "lang/desktop/"+lang+".json")), &words); err != nil {
			t.Fatal(err)
		}
		if words["desktop.app_cyd_creator"] != "CYD Creator" {
			t.Errorf("lang/desktop/%s.json: desktop.app_cyd_creator = %q", lang, words["desktop.app_cyd_creator"])
		}
	}
}

var cydCreatorKeyLiteral = regexp.MustCompile(`'(cyd_creator\.[A-Za-z0-9_.]*)'(\s*\+)?`)

func readCYDCreatorShellLang(t *testing.T, lang string) map[string]string {
	t.Helper()
	out := map[string]string{}
	if err := json.Unmarshal([]byte(rawDesktopAssetText(t, "lang/cyd_creator/"+lang+".json")), &out); err != nil {
		t.Fatalf("lang/cyd_creator/%s.json: %v", lang, err)
	}
	return out
}

// TestCYDCreatorKeysExistInAllLocales scans the plan-4a shell modules present for full
// "cyd_creator.…" key literals; dynamic families (a literal ending in "_" or "." or followed by "+")
// need a member list here. All sixteen locales carry every used key and exactly the English key set.
func TestCYDCreatorKeysExistInAllLocales(t *testing.T) {
	errorCodes := []string{"CYD_DISABLED", "CYD_DOCKER_UNAVAILABLE", "CYD_IMAGE_MISSING", "CYD_IMAGE_PULL_FAILED", "CYD_BAD_REQUEST",
		"CYD_NOT_FOUND", "CYD_READONLY", "CYD_BUSY", "CYD_PROJECT_LOCKED", "CYD_PROJECT_CONFLICT", "CYD_INVALID_LAYOUT",
		"CYD_FILE_READONLY", "CYD_FILE_CONFLICT", "CYD_FILE_LIMIT", "CYD_ASSET_TOO_LARGE", "CYD_GENERATOR_ERROR", "CYD_BUILD_FAILED",
		"CYD_BUILD_TIMEOUT", "CYD_BUILD_CANCELLED", "CYD_BUILD_INTERRUPTED", "CYD_FLASH_BUDGET_EXCEEDED", "CYD_JOB_NOT_FOUND",
		"CYD_JOB_STATE", "CYD_DEVICE_NOT_FOUND", "CYD_UNAUTHORIZED", "CYD_RATE_LIMITED", "CYD_OTA_DISABLED", "CYD_IMPORT_INVALID",
		"CYD_TEMPLATE_TOO_NEW", "CYD_FLASH_WRONG_CHIP", "CYD_FLASH_PORT_BUSY", "CYD_FLASH_DISCONNECTED", "CYD_SERIAL_UNSUPPORTED",
		"CYD_INTERNAL", "network", "generic",
		// AuraGo's desktop auth and policy answers (code taken from "error" by cyd-creator-api.js).
		"desktop_scope_required", "unauthorized", "desktop_readonly"}
	families := map[string][]string{
		"cyd_creator.agent.wifi_":       {"off", "optional", "required"},
		"cyd_creator.build.status_":     {"queued", "running", "succeeded", "failed", "cancelled", "interrupted"},
		"cyd_creator.code.group_":       {"app", "generated", "template"},
		"cyd_creator.code.state_":       {"saved", "dirty", "saving", "offline", "failed", "conflict", "locked"},
		"cyd_creator.dock.":             {"log", "problems"},
		"cyd_creator.dock.source_":      {"layout", "build"},
		"cyd_creator.home.board_hint_":  {"verified", "community"},
		"cyd_creator.error.":            errorCodes,
		"cyd_creator.job.phase_":        {"queued", "planning", "layout", "coding", "building", "repairing", "done"},
		"cyd_creator.job.status_":       {"queued", "running", "awaiting_confirmation", "succeeded", "failed", "cancelled", "interrupted"},
		"cyd_creator.orientation.":      {"landscape", "portrait", "landscape_flipped", "portrait_flipped"},
		"cyd_creator.revisions.reason_": {"before_job", "job", "restore", "import", "template_upgrade"},
		"cyd_creator.revisions.status_": {"added", "modified", "deleted"},
	}
	// The key prefix itself and the families plan 4b owns (issue codes, theme names, board states) are
	// not 4a's: the shell reads them with t.has/t.or fallbacks, the designer's own test covers them.
	ignored := map[string]bool{"cyd_creator.": true, "cyd_creator.issue.": true, "cyd_creator.theme.": true, "cyd_creator.board.status_": true}
	// Only the shell modules: the designer modules of plan 4b and the device modules of plan 5 carry
	// their own key tests.
	shell := map[string]bool{"cyd-creator-core.js": true, "cyd-creator-api.js": true, "cyd-creator-events.js": true, "cyd-creator-dock.js": true,
		"cyd-creator-code.js": true, "cyd-creator-agent.js": true, "cyd-creator-home.js": true, "cyd-creator-app.js": true}
	used := map[string]bool{}
	files, err := filepath.Glob(filepath.Join("js", "desktop", "apps", "cyd-creator-*.js"))
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		if !shell[filepath.Base(file)] {
			continue
		}
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range cydCreatorKeyLiteral.FindAllStringSubmatch(string(data), -1) {
			key := m[1]
			if m[2] != "" || strings.HasSuffix(key, "_") || strings.HasSuffix(key, ".") {
				if ignored[key] {
					continue
				}
				if _, ok := families[key]; !ok {
					t.Errorf("dynamic key family %s (in %s) has no member list in this test", key, file)
				}
				continue
			}
			used[key] = true
		}
	}
	for prefix, members := range families {
		for _, member := range members {
			used[prefix+member] = true
		}
	}
	en := readCYDCreatorShellLang(t, "en")
	for _, lang := range cydCreatorLocales {
		words := readCYDCreatorShellLang(t, lang)
		for key := range used {
			if strings.TrimSpace(words[key]) == "" {
				t.Errorf("lang/cyd_creator/%s.json misses %s", lang, key)
			}
		}
		for key := range en {
			if _, ok := words[key]; !ok {
				t.Errorf("lang/cyd_creator/%s.json misses %s (present in en)", lang, key)
			}
		}
		for key := range words {
			if _, ok := en[key]; !ok {
				t.Errorf("lang/cyd_creator/%s.json has %s, which en lacks", lang, key)
			}
		}
	}
	de := readCYDCreatorShellLang(t, "de")
	formal := regexp.MustCompile(`\b(Sie|Ihnen|Ihre[nmrs]?)\b`)
	for key := range used {
		if value := de[key]; formal.MatchString(value) {
			t.Errorf("de %s addresses the user formally: %q", key, value)
		}
	}
	if !strings.Contains(de["cyd_creator.app.close"], "ß") || !strings.Contains(de["cyd_creator.code.conflict_overwrite"], "Ü") {
		t.Error("German strings must use real umlauts and ß")
	}
}

// checkCYDCreatorModule asserts the common module shape (purpose comment, strict mode, the shared
// namespace, the 1100-line budget) and the given markers.
func checkCYDCreatorModule(t *testing.T, name string, markers ...string) string {
	t.Helper()
	text := rawDesktopAssetText(t, "js/desktop/apps/"+name)
	if lines := strings.Count(text, "\n"); lines >= 1100 {
		t.Errorf("%s has %d lines, the desktop budget is 1100", name, lines)
	}
	if !strings.HasPrefix(strings.TrimSpace(text), "//") || !strings.Contains(text, "'use strict';") || !strings.Contains(text, "window.CYDCreatorParts") {
		t.Errorf("%s must start with a purpose comment, use strict mode and the CYDCreatorParts namespace", name)
	}
	for _, marker := range markers {
		if !strings.Contains(text, marker) {
			t.Errorf("%s misses %q", name, marker)
		}
	}
	return text
}

func TestCYDCreatorFoundationModules(t *testing.T) {
	checkCYDCreatorModule(t, "cyd-creator-core.js",
		"P.util = { el, bag, debounce, clamp, uid, escapeText, deepClone, deepFreeze };",
		"P.i18n = function i18n(inst)", "P.errors = {", "P.createBus = function createBus()",
		"P.autosave = { create: createAutosave, classify: classifySaveError };", "P.dialog = { open: openDialog };")
	checkCYDCreatorModule(t, "cyd-creator-api.js",
		"P.api = {", "class ApiError extends Error", "'/capabilities'", "'/toolchain/pull'", "'/projects'", "'/layout'", "'/files/'",
		"'/assets'", "'/builds'", "'/cancel'", "'/log'", "'/artifacts/'", "'/decode-backtrace'", "'/events'", "'/jobs'",
		"'/confirm'", "'/draft/discard'", "'/messages'", "'/revisions'", "'/diff'", "'/restore'", "'If-Match'", "'X-CYD-SHA256'")
	checkCYDCreatorModule(t, "cyd-creator-events.js",
		"P.events = { TYPES, connect, createCursor, createCatchup, backoff };",
		"'project_changed'", "'lock_changed'", "'build_status'", "'build_log'", "'job_status'", "'job_step'", "'job_message'",
		"'device_changed'", "'stream_ready'")
}

func TestCYDCreatorStylesheetUsesThemeTokens(t *testing.T) {
	css := rawDesktopAssetText(t, "css/desktop-app-cyd-creator.css")
	split := strings.Index(css, ".cc-app [hidden] { display: none !important; }")
	if split < 0 {
		t.Fatal("the [hidden] rule must follow the token block")
	}
	// Plan 4b appends a delimited designer section with its own rules; only the shell part is checked.
	shell := css[split:]
	if end := strings.Index(shell, "/* ==== CYD Creator designer (plan 4b) — begin"); end >= 0 {
		shell = shell[:end]
	}
	if hex := regexp.MustCompile(`#[0-9a-fA-F]{3,8}\b`).FindString(shell); hex != "" {
		t.Fatalf("literal colour %s outside the .cc-app token block", hex)
	}
	if fn := regexp.MustCompile(`\b(?:rgba?|hsla?)\(`).FindString(shell); fn != "" {
		t.Fatalf("literal colour function %s…) outside the .cc-app token block", fn)
	}
	// Shell rules only read --cc-* tokens the stylesheet defines (plan 4b's inline --cc-grid/--cc-depth
	// live in its own section).
	defined := map[string]bool{}
	for _, m := range regexp.MustCompile(`(--cc-[a-z0-9-]+)\s*:`).FindAllStringSubmatch(css, -1) {
		defined[m[1]] = true
	}
	for _, m := range regexp.MustCompile(`var\((--cc-[a-z0-9-]+)`).FindAllStringSubmatch(shell, -1) {
		if !defined[m[1]] {
			t.Errorf("shell rules use %s but desktop-app-cyd-creator.css never defines it", m[1])
		}
	}
	for _, marker := range []string{
		"--cc-bg: var(--vd-theme-app-bg", "--cc-panel: var(--vd-theme-panel-bg", "--cc-border: var(--vd-theme-border",
		"--cc-text: var(--vd-text", "container: cc-app / inline-size;", "@container cc-app (max-width: 820px)",
		"@media (prefers-reduced-motion: reduce)", ".cc-app .cc-modal-backdrop",
	} {
		if !strings.Contains(css, marker) {
			t.Errorf("desktop-app-cyd-creator.css misses %q", marker)
		}
	}
}

func TestCYDCreatorDockModule(t *testing.T) {
	text := checkCYDCreatorModule(t, "cyd-creator-dock.js",
		"P.dock = { create };", "registerTab,", "ccDockTab: id", "problems: { set: setIssues, setDiagnostics,",
		"inst.bus.on('sse:build_log'", "inst.bus.on('layout:issues'", "inst.reveal(", "inst.openFile(", "trackBuild(build) {", "dispose() {")
	checkCYDCreatorCancelNotReadonly(t, "cyd-creator-dock.js", text, "cancelBtn.hidden =")
}

// checkCYDCreatorCancelNotReadonly: plan 2 allows the job/build cancel routes while the desktop is
// read-only (cydRoute.Stop), so no line that shows or enables a Cancel button may depend on
// inst.readonly.
func checkCYDCreatorCancelNotReadonly(t *testing.T, name, text, marker string) {
	t.Helper()
	found := 0
	for _, line := range strings.Split(text, "\n") {
		if strings.Contains(line, marker) {
			found++
			if strings.Contains(line, "readonly") || strings.Contains(line, "disabled,") {
				t.Errorf("%s gates a Cancel button on read-only: %s", name, strings.TrimSpace(line))
			}
		}
	}
	if found == 0 {
		t.Errorf("%s has no line with %q", name, marker)
	}
}

func TestCYDCreatorCodeModule(t *testing.T) {
	checkCYDCreatorModule(t, "cyd-creator-code.js",
		"P.code = { create, HOOK_FILE };", "const HOOK_FILE = 'src/app/app_events.cpp';", "import('/js/vendor/codemirror-bundle.esm.js')",
		"cm.cpp()", "ensureHook,", "P.autosave.create({", "cyd_creator.code.generated_hint", "view.lineBlockAt(", "dispose() {")
}

func TestCYDCreatorAgentModule(t *testing.T) {
	text := checkCYDCreatorModule(t, "cyd-creator-agent.js",
		"P.agent = { create, tailBytes };", "attachLog(text)", "inst.setPanelBadge('agent', active)", "ccAgentAction: 'confirm'",
		"ccAgentAction: 'resume'", "ccAgentAction: 'discard'", "inst.api.restoreRevision(", "dispose() {")
	checkCYDCreatorCancelNotReadonly(t, "cyd-creator-agent.js", text, "ccAgentAction: 'cancel'")
}
