package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"aurago/internal/cydcreator"
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
	// Job steps and system chat lines are translated by their server code (internal/cydcreator texts.go).
	for key := range cydcreator.TextKeys() {
		for _, prefix := range []string{"cyd_creator.step.", "cyd_creator.msg."} {
			if code, ok := strings.CutPrefix(key, prefix); ok {
				families[prefix] = append(families[prefix], code)
			}
		}
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

var cydCreatorPlaceholder = regexp.MustCompile(`\{\{(\w+)\}\}`)

func cydCreatorPlaceholders(text string) string {
	var names []string
	for _, m := range cydCreatorPlaceholder.FindAllStringSubmatch(text, -1) {
		names = append(names, m[1])
	}
	slices.Sort(names)
	return strings.Join(names, ",")
}

// TestCYDCreatorServerTextsAreTranslated: every job step and system line code of the server has its
// key in all sixteen locales; the English bundle is exactly the server's template, every translation
// uses the same placeholders, and no locale keeps a step or message key the server no longer has.
func TestCYDCreatorServerTextsAreTranslated(t *testing.T) {
	keys := cydcreator.TextKeys()
	if len(keys) < 20 {
		t.Fatalf("server text keys = %d", len(keys))
	}
	en := readCYDCreatorShellLang(t, "en")
	for key, template := range keys {
		if en[key] != template {
			t.Errorf("en %s = %q, want the server template %q", key, en[key], template)
		}
	}
	for _, lang := range cydCreatorLocales {
		words := readCYDCreatorShellLang(t, lang)
		for key, template := range keys {
			value := words[key]
			if strings.TrimSpace(value) == "" {
				t.Errorf("%s misses %s", lang, key)
				continue
			}
			if got, want := cydCreatorPlaceholders(value), cydCreatorPlaceholders(template); got != want {
				t.Errorf("%s %s has placeholders %q, want %q", lang, key, got, want)
			}
			if lang != "en" && strings.Contains(value, "(s)") {
				t.Errorf("%s %s uses a plural shortcut: %q", lang, key, value)
			}
		}
		for key := range words {
			if strings.HasPrefix(key, "cyd_creator.step.") || strings.HasPrefix(key, "cyd_creator.msg.") {
				if _, ok := keys[key]; !ok {
					t.Errorf("%s has %s, which is no server text code", lang, key)
				}
			}
		}
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
	// Review fixes (behaviour in scripts/test-cyd-creator-shell.mjs): the collapse survives the
	// restore of the saved tab, pruned builds replayed by the event backlog are ignored, a finished
	// build's stored log replaces the lossy live lines, and the separator states its range.
	for _, marker := range []string{"const startCollapsed = !!prefs.collapsed;", "setCollapsed(startCollapsed);",
		"if (pruned(build, state.builds)) return;", "loadLog(build.id, true);", "'aria-valuemin'", "'aria-valuemax'",
		"downloadLink.download = build.id + '-build.log';"} {
		if !strings.Contains(text, marker) {
			t.Errorf("cyd-creator-dock.js misses %q", marker)
		}
	}
	// A reload (reloadProject after a restored database) replaces stale builds and merges only the
	// builds events reported while it ran.
	if !strings.Contains(text, "const reported = reload ? Array.from(state.seen.values())") {
		t.Error("cyd-creator-dock.js: a reload of the builds must merge only the builds seen meanwhile")
	}
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
	// Fix round: the chip follows every document change, a vanished file keeps unsaved text and can
	// be recreated, dialogs close with the module, the editor has a name, hooks match definitions.
	checkCYDCreatorModule(t, "cyd-creator-code.js",
		"inst.bus.emit('code:state', state.saver ? state.saver.state() : 'saved');", "inst.bus.emit('code:state', 'saved');",
		"return { action: 'overwrite', token: '*' };", "owner: b", "function isEditablePath(rel)",
		"cm.EditorView.contentAttributes.of({ 'aria-label': t('cyd_creator.code.editor_label') })",
		`new RegExp('\\bvoid\\s+' + fn + '\\s*\\(')`)
	// Second fix round: a case-only rename keeps a server copy at every step, a closed document has
	// nothing left to flush, folder spellings clash like file spellings.
	checkCYDCreatorModule(t, "cyd-creator-code.js",
		"const tmp = APP_DIR + P.util.uid('cyd_rename') + '.h';", "else if (kept && !state.openPath) await openFile(kept, 1);",
		"return state.saver.flush().then(ok => ok || !state.saver);", "folderPrefixes(path).some(")
}

func TestCYDCreatorAgentModule(t *testing.T) {
	text := checkCYDCreatorModule(t, "cyd-creator-agent.js",
		"P.agent = { create, tailBytes };", "attachLog(text)", "inst.setPanelBadge('agent', active)", "ccAgentAction: 'confirm'",
		"ccAgentAction: 'resume'", "ccAgentAction: 'discard'", "inst.api.restoreRevision(", "dispose() {")
	checkCYDCreatorCancelNotReadonly(t, "cyd-creator-agent.js", text, "ccAgentAction: 'cancel'")
	// Review fixes: finished jobs stay final, restore flushes first, the dialog has an owner, the
	// publication-review reason is shown, a reloading stream reloads the panel and server lines are
	// translated by their code.
	for _, marker := range []string{"TERMINAL.has(previous.status) && !TERMINAL.has(job.status)) return;", "inst.flushAll()", "owner: b,",
		"reason === 'publication_review'", "inst.bus.on('sse:stream_ready'", "serverText('cyd_creator.step.', step)",
		"serverText('cyd_creator.msg.', message)"} {
		if !strings.Contains(text, marker) {
			t.Errorf("cyd-creator-agent.js misses %q", marker)
		}
	}
}

func TestCYDCreatorHomeModule(t *testing.T) {
	checkCYDCreatorModule(t, "cyd-creator-home.js",
		"P.home = { create, posixTimezone };", "P.transfer && typeof P.transfer.importZip === 'function'", "ccAction: action",
		"'cyd_creator.board.status_' + board.status", "dispose() {")
	// Review fixes: the ICU zone alias, a create dismissed in flight opens nothing, one DELETE per
	// row and only the newest list answer is applied (behaviour: scripts/test-cyd-creator-shell.mjs).
	checkCYDCreatorModule(t, "cyd-creator-home.js",
		"'Asia/Calcutta': 'IST-5:30'", "if (dialog.closed) { if (!state.disposed) refresh(); return; }",
		"if (dialog.closed) { inst.notify('error', P.errors.message(t, err)); return; }", "button.disabled = true;",
		"if (state.disposed || seq !== state.seq) return;")
}

// cydCreatorShellModules are the plan-4a modules in loader order. Plan 4b inserts its ten designer
// modules directly after cyd-creator-events.js (in the loader above the "plan 4b" marker, here after
// "cyd-creator-events.js"), plan 5 its modules below the "plan 5" marker (between home and app);
// both markers stay in place. The tests check relative order only, so inserted modules are fine.
var cydCreatorShellModules = []string{
	"cyd-creator-core.js", "cyd-creator-api.js", "cyd-creator-events.js",
	"cyd-creator-geometry.js", "cyd-creator-model.js", "cyd-creator-render.js", "cyd-creator-canvas.js",
	"cyd-creator-interact.js", "cyd-creator-palette.js", "cyd-creator-inspector-fields.js",
	"cyd-creator-inspector.js", "cyd-creator-screens.js", "cyd-creator-designer.js",
	"cyd-creator-code.js", "cyd-creator-dock.js", "cyd-creator-agent.js", "cyd-creator-home.js",
	"cyd-creator-app.js",
}

const (
	cydCreator4bMarker = "// CYD Creator plan 4b inserts geometry, model, render, canvas, interact, palette, inspector-fields, inspector, screens, designer above this line."
	cydCreator5Marker  = "// CYD Creator plan 5 inserts serial, flasher, monitor, devices, transfer here."
)

func TestCYDCreatorAppModule(t *testing.T) {
	checkCYDCreatorModule(t, "cyd-creator-app.js",
		"window.CYDCreatorApp = { render, dispose, instances };",
		"P.model.createStore(payload.project, { limit: 300, coalesceMs: 1200 })",
		"P.designer.create(inst)", "inst.bus.emit('layout:issues'", "inst.bus.emit('designer:reveal', spec || {})",
		"inst.bus.emit('lock:changed'", "P.flasher && typeof P.flasher.open === 'function'",
		"P.devices && typeof P.devices.open === 'function'", "P.transfer && typeof P.transfer.exportProject === 'function'")
	var all strings.Builder
	for _, name := range cydCreatorShellModules {
		all.WriteString(rawDesktopAssetText(t, "js/desktop/apps/"+name))
	}
	for _, hook := range []string{"new-agent", "new-designer", "import", "open", "build", "flash", "ota", "export", "mode-design", "mode-code"} {
		if !strings.Contains(all.String(), "'"+hook+"'") {
			t.Errorf("stable test hook data-cc-action=%q is missing", hook)
		}
	}
	for _, panel := range []string{"ccPanel: 'properties'", "ccPanel: 'agent'"} {
		if !strings.Contains(all.String(), panel) {
			t.Errorf("right panel tab hook %q is missing", panel)
		}
	}
}

func TestCYDCreatorShellWiring(t *testing.T) {
	foundation := rawDesktopAssetText(t, "js/desktop/core/desktop-foundation.js")
	routing := rawDesktopAssetText(t, "js/desktop/core/menus-and-routing.js")
	bundle := rawDesktopAssetText(t, "js/desktop/bundles/main.bundle.js")
	for _, check := range []struct{ name, source, marker string }{
		{"icon key", foundation, "'cyd-creator': 'cyd-creator'"},
		{"global name", foundation, "'cyd-creator': 'CYDCreatorApp'"},
		{"routing", routing, "if (appId === 'cyd-creator') {"},
		{"routing lazy load", routing, "window.AuraDesktopModules.loadAppScript('cyd-creator')"},
		{"routing render", routing, "window.CYDCreatorApp.render(contentEl(id), id,"},
		{"bundle global name", bundle, "'cyd-creator': 'CYDCreatorApp'"},
		{"bundle routing", bundle, "if (appId === 'cyd-creator') {"},
	} {
		if !strings.Contains(check.source, check.marker) {
			t.Errorf("%s misses %q", check.name, check.marker)
		}
	}
}

func cydCreatorLoaderBlock(t *testing.T) string {
	t.Helper()
	loader := strings.ReplaceAll(rawDesktopAssetText(t, "js/desktop/core/module-loader.js"), "\r\n", "\n")
	start := strings.Index(loader, "'cyd-creator': {")
	if start < 0 {
		t.Fatal("module-loader.js has no cyd-creator entry")
	}
	end := strings.Index(loader[start:], "]\n        }")
	if end < 0 {
		t.Fatal("cyd-creator entry is not closed")
	}
	return loader[start : start+end]
}

func TestCYDCreatorAssetsLoadInOverviewOrder(t *testing.T) {
	block := cydCreatorLoaderBlock(t)
	if !strings.Contains(block, "styles: appStyles('/css/desktop-app-cyd-creator.css')") {
		t.Fatalf("cyd-creator assets miss the stylesheet:\n%s", block)
	}
	position := func(needle string) int {
		idx := strings.Index(block, needle)
		if idx < 0 {
			t.Fatalf("cyd-creator assets miss %s", needle)
		}
		return idx
	}
	last := -1
	for _, name := range cydCreatorShellModules {
		idx := position("'/js/desktop/apps/" + name + "'")
		if idx < last {
			t.Fatalf("%s loads before a module it depends on", name)
		}
		last = idx
	}
	m4b, m5 := position(cydCreator4bMarker), position(cydCreator5Marker)
	if !(position("'/js/desktop/apps/cyd-creator-events.js'") < m4b && m4b < position("'/js/desktop/apps/cyd-creator-code.js'")) {
		t.Fatal("the plan 4b marker must sit between events and code")
	}
	if !(position("'/js/desktop/apps/cyd-creator-home.js'") < m5 && m5 < position("'/js/desktop/apps/cyd-creator-app.js'")) {
		t.Fatal("the plan 5 marker must sit between home and app")
	}
	if !strings.Contains(rawDesktopAssetText(t, "js/desktop/core/module-loader.js"), "'cyd-creator': ['cyd_creator']") {
		t.Fatal("APP_I18N_SECTIONS must load the cyd_creator section")
	}
}

func TestCYDCreatorUsesNoNativeDialogs(t *testing.T) {
	native := regexp.MustCompile(`(^|[^.\w])(alert|confirm|prompt)\s*\(|window\.(alert|confirm|prompt)\s*\(`)
	for _, name := range cydCreatorShellModules {
		for i, line := range strings.Split(rawDesktopAssetText(t, "js/desktop/apps/"+name), "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "//") {
				continue
			}
			if native.MatchString(line) {
				t.Errorf("%s:%d uses a native dialog: %s", name, i+1, strings.TrimSpace(line))
			}
		}
	}
}

func TestCYDCreatorDisposeLifecycle(t *testing.T) {
	app := rawDesktopAssetText(t, "js/desktop/apps/cyd-creator-app.js")
	for _, marker := range []string{
		"function dispose(windowId)", "inst.disposed = true;", "instances.delete(windowId);",
		"if (inst.events) { inst.events.close(); inst.events = null; }",
		"['designer', 'agent', 'code', 'dock'].forEach(name => {", "inst.bag.dispose();", "inst.bus.clear();",
		"if (inst.pullTimer) clearInterval(inst.pullTimer);", "inst.ctx.setWindowBeforeClose(windowId, null)",
	} {
		if !strings.Contains(app, marker) {
			t.Errorf("cyd-creator-app.js misses %q", marker)
		}
	}
	for _, name := range []string{"cyd-creator-dock.js", "cyd-creator-code.js", "cyd-creator-agent.js", "cyd-creator-home.js"} {
		if text := rawDesktopAssetText(t, "js/desktop/apps/"+name); !strings.Contains(text, "dispose() {") || !strings.Contains(text, "b.dispose();") {
			t.Errorf("%s must release its listeners in dispose()", name)
		}
	}
	if !strings.Contains(rawDesktopAssetText(t, "js/desktop/bundles/main.bundle.js"), "'cyd-creator': 'CYDCreatorApp'") {
		t.Error("the shell must find CYDCreatorApp.dispose through appGlobalName")
	}
}

// TestCYDCreatorAppCarriesTheModuleContracts pins what the reviewed modules need from the window
// (behaviour: scripts/test-cyd-creator-shell.mjs): dialogs with an owner, the reload after
// stream_ready, the stream give-up probe, the publication review banner, saving before every agent
// job, the segment buttons and the read-only propagation.
func TestCYDCreatorAppCarriesTheModuleContracts(t *testing.T) {
	checkCYDCreatorModule(t, "cyd-creator-app.js",
		"owner: inst.bag", "owner: inst.projectBag || inst.bag",
		"if (data && data.reload) reloadProject(inst, { keepEdits: true });", "inst.dock ? inst.dock.builds.load() : null",
		"inst.code ? inst.code.refreshFiles() : null", "inst.dock.trackBuild(build);",
		"bus.on('events:failed'", "await P.api.getProject(inst.projectId);", "stopStream(inst, err);",
		"dataset: { ccPublishReview: '' }", "v.reviewBanner.hidden = !project.publish_review;",
		"if (!(await flushAll(inst))) throw unsavedError(inst);", "class: 'cc-btn cc-seg'",
		"'aurago:desktop-policy'", "if (inst.code) inst.code.updateReadOnly();", "if (inst.home) inst.home.refresh();")
	checkCYDCreatorModule(t, "cyd-creator-events.js", "inst.bus.emit('events:failed', attempt);")
}

// TestCYDCreatorWindowKeepsUnsavedEditsAndResumesSafely pins the Task 9 review fixes (behaviour:
// scripts/test-cyd-creator-shell.mjs): an asset upload saves pending edits first and never marks
// later edits saved, a stopped stream resumes where its catch-up started, flush waits for an open
// conflict choice, a failed code save explains a refused job, the review banner is announced, the
// toolchain banner follows read-only and a closed project's probe cannot block the next one's.
func TestCYDCreatorWindowKeepsUnsavedEditsAndResumesSafely(t *testing.T) {
	checkCYDCreatorModule(t, "cyd-creator-app.js",
		"if (!(await inst.layoutSaver.flush())) throw unsavedError(inst);", "if (!isCurrent(inst, seq) || !layoutIdle(inst)) return res.asset;",
		"inst.streamAfter = inst.events.resumeFrom;", "code.saveError()", "role: 'alert', dataset: { ccPublishReview: '' }",
		"renderToolchain(inst, inst.view && inst.view.toolchainSlot);", "if (isCurrent(inst, seq)) inst.probing = false;")
	checkCYDCreatorModule(t, "cyd-creator-events.js", "get resumeFrom() { return catching ? catchupFrom : cursor.after; }")
	checkCYDCreatorModule(t, "cyd-creator-core.js", "if (conflictPending) return conflictPending;")
	checkCYDCreatorModule(t, "cyd-creator-code.js", "saveError: () => (state.saver ? state.saver.error() : null),")
}
