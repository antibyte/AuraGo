package ui

import (
	"regexp"
	"strings"
	"testing"
)

// cydDesignerMarkers are the public entry points and seam usages each designer module must keep.
var cydDesignerMarkers = map[string][]string{
	"cyd-creator-geometry.js":         {"P.geometry = {", "function snapMove(", "function resizeRect(", "function distribute("},
	"cyd-creator-model.js":            {"P.model = {", "function createStore(", "coalesceMs", "RESERVED_NAMES", "function applyOps("},
	"cyd-creator-render.js":           {"P.render = {", "function widget(spec, w, theme, assetUrl, opts)", "'image-rendering': 'pixelated'"},
	"cyd-creator-canvas.js":           {"P.designerCanvas = {", "cc-device-frame", "dataset.ccWidgetId", "dataset.ccScreenId"},
	"cyd-creator-interact.js":         {"P.designerInteract = {", "function startPaletteDrag(", "function handleKey("},
	"cyd-creator-palette.js":          {"P.designerPalette = {", "'data-cc-palette': s.type"},
	"cyd-creator-inspector-fields.js": {"P.designerFields = {", "async function rasterizeSVG("},
	"cyd-creator-inspector.js":        {"P.designerInspector = {", "d.inst.uploadAsset", "d.inst.code.ensureHook"},
	"cyd-creator-screens.js":          {"P.designerScreens = {", "d.inst.confirm(", "d.SYSTEM_WIFI"},
	"cyd-creator-designer.js":         {"P.designer = { create", "mountCanvas(el)", "mountPalette(el)", "mountScreens(el)", "mountInspector(el)", "reveal(target)", "dispose()", "'layout:issues'", "'lock:changed'", "'readonly:changed'", "'designer:reveal'"},
}

// cydDesignerForbidden lists tokens a designer module must not contain: "*" applies to every module
// in cydDesignerScripts (also the ones later tasks add), a file name adds its own. The designer
// reaches the server only through the shell's seam (inst.uploadAsset, the layout store that the shell
// autosaves), never through P.api, inst.api or the layout saver, and builds DOM only from nodes and
// text, never from HTML strings.
var cydDesignerForbidden = map[string][]string{
	"*": {"P.api.", "inst.api.", "layoutSaver", "innerHTML", "insertAdjacentHTML"},
}

// TestCYDCreatorDesignerStaticContracts checks each designer module (one subtest per file) for its
// entry points and the repository UI rules: no native dialogs, none of the forbidden tokens and no
// theme colour literals in the renderer, canvas or interaction code. The css subtest checks the
// designer section of the app stylesheet: the self-hosted preview fonts, the shell's tokens, the
// stage metrics fitZoom relies on and the narrow-window layout of Design mode.
func TestCYDCreatorDesignerStaticContracts(t *testing.T) {
	native := regexp.MustCompile(`\b(alert|confirm|prompt)\s*\(`)
	hex := regexp.MustCompile(`#[0-9a-fA-F]{6}\b`)
	for _, name := range cydDesignerScripts {
		t.Run(name, func(t *testing.T) {
			text := readDesktopAssetText(t, "js/desktop/apps/"+name)
			for _, marker := range cydDesignerMarkers[name] {
				if !strings.Contains(text, marker) {
					t.Errorf("%s misses %q", name, marker)
				}
			}
			for _, token := range append(append([]string{}, cydDesignerForbidden["*"]...), cydDesignerForbidden[name]...) {
				if strings.Contains(text, token) {
					t.Errorf("%s contains the forbidden %q", name, token)
				}
			}
			for _, m := range native.FindAllStringIndex(text, -1) {
				if !strings.HasSuffix(text[:m[0]], "inst.") {
					t.Errorf("%s calls a native dialog: %q", name, text[m[0]:m[1]])
				}
			}
			if name == "cyd-creator-render.js" || name == "cyd-creator-canvas.js" || name == "cyd-creator-interact.js" {
				if found := hex.FindString(text); found != "" {
					t.Errorf("%s hard-codes the colour %s; device colours come from the API themes", name, found)
				}
			}
		})
	}
	t.Run("css", func(t *testing.T) {
		css := readDesktopAssetText(t, "css/desktop-app-cyd-creator.css")
		for _, marker := range []string{"CYD Creator designer (plan 4b) — begin", "CYD Creator designer (plan 4b) — end", "font-family: 'CYD Montserrat'", "font-family: 'CYD JetBrains Mono'", ".cc-device-frame", ".cc-handle-se", "@media (prefers-reduced-motion: reduce)"} {
			if !strings.Contains(css, marker) {
				t.Errorf("desktop-app-cyd-creator.css misses %q", marker)
			}
		}
		begin := strings.Index(css, "/* ==== CYD Creator designer (plan 4b) — begin")
		if begin < 0 {
			t.Fatal("the designer section must start with the shell's delimiter comment")
		}
		section := css[begin:]
		checkCYDDesignerSectionTokens(t, css, section)
		checkCYDDesignerStageMetrics(t, section)
		// Narrow windows: the shell hides .cc-left at ≤ 820 px; Design mode must bring screens,
		// layers and palette back (Code mode keeps the shell layout).
		for _, marker := range []string{"@container cc-app (max-width: 820px)", `.cc-app:not(.no-designer):not([data-cc-mode="code"]) .cc-left { display: grid;`} {
			if !strings.Contains(section, marker) {
				t.Errorf("the designer section misses %q", marker)
			}
		}
	})
}

// checkCYDDesignerSectionTokens holds the designer section to the shell's token rules: literal
// colours only in its own token block (the first rule), chrome colours only from --cc-* tokens the
// stylesheet defines (plus the inline --cc-grid and --cc-depth the canvas and layers set), and none
// of the plan's former --ccd-* re-derivations of the desktop theme.
func checkCYDDesignerSectionTokens(t *testing.T, css, section string) {
	t.Helper()
	if strings.Contains(css, "--ccd-") {
		t.Error("desktop-app-cyd-creator.css uses --ccd-* tokens; the designer reads the shell's --cc-* tokens")
	}
	open := strings.Index(section, ".cc-app, .cc-drag-ghost {")
	if open < 0 {
		t.Fatal("the designer section has no token block for .cc-app and the drag ghost")
	}
	head := section[:open]
	rules := section[open:]
	rules = rules[strings.Index(rules, "}")+1:]
	for name, text := range map[string]string{"before its token block": head, "after its token block": rules} {
		if hex := regexp.MustCompile(`#[0-9a-fA-F]{3,8}\b`).FindString(text); hex != "" {
			t.Errorf("literal colour %s in the designer section %s", hex, name)
		}
		if fn := regexp.MustCompile(`\b(?:rgba?|hsla?)\(`).FindString(text); fn != "" {
			t.Errorf("literal colour function %s…) in the designer section %s", fn, name)
		}
	}
	defined := map[string]bool{"--cc-grid": true, "--cc-depth": true}
	for _, m := range regexp.MustCompile(`(--cc-[a-z0-9-]+)\s*:`).FindAllStringSubmatch(css, -1) {
		defined[m[1]] = true
	}
	for _, m := range regexp.MustCompile(`var\((--[a-z0-9-]+)`).FindAllStringSubmatch(section, -1) {
		if !defined[m[1]] {
			t.Errorf("the designer section reads %s, which is no --cc-* token of desktop-app-cyd-creator.css", m[1])
		}
	}
}

// checkCYDDesignerStageMetrics pins the canvas geometry that fitZoom in cyd-creator-canvas.js
// subtracts: the stage padding is PAD and the device frame's padding is BEZEL.
func checkCYDDesignerStageMetrics(t *testing.T, section string) {
	t.Helper()
	canvas := readDesktopAssetText(t, "js/desktop/apps/cyd-creator-canvas.js")
	for _, pin := range []struct{ constant, rule string }{{"PAD", ".cc-app .cc-canvas-stage {"}, {"BEZEL", ".cc-app .cc-device-frame {"}} {
		m := regexp.MustCompile(`const ` + pin.constant + ` = (\d+);`).FindStringSubmatch(canvas)
		if m == nil {
			t.Errorf("cyd-creator-canvas.js has no const %s", pin.constant)
			continue
		}
		at := strings.Index(section, pin.rule)
		if at < 0 {
			t.Errorf("the designer section has no rule %q", pin.rule)
			continue
		}
		body := section[at:]
		body = body[:strings.Index(body, "}")]
		if !regexp.MustCompile(`(?:^|[;{]\s*)padding: ` + m[1] + `px;`).MatchString(body) {
			t.Errorf("%s must have padding: %spx (cyd-creator-canvas.js %s, used by fitZoom): %s", pin.rule, m[1], pin.constant, body)
		}
	}
}

// TestCYDCreatorDesignerModulesLoadInOrder pins the designer scripts in DESKTOP_APP_ASSETS between
// cyd-creator-events.js and cyd-creator-code.js.
func TestCYDCreatorDesignerModulesLoadInOrder(t *testing.T) {
	loader := readDesktopAssetText(t, "js/desktop/core/module-loader.js")
	entry := strings.Index(loader, "'cyd-creator': {")
	if entry < 0 {
		t.Fatal("module-loader.js has no cyd-creator entry")
	}
	end := strings.Index(loader[entry:], "]")
	block := loader[entry : entry+end]
	order := append([]string{"cyd-creator-events.js"}, cydDesignerScripts...)
	order = append(order, "cyd-creator-code.js")
	last := -1
	for _, name := range order {
		at := strings.Index(block, "'/js/desktop/apps/"+name+"'")
		if at < 0 {
			t.Fatalf("cyd-creator assets miss %s", name)
		}
		if at < last {
			t.Fatalf("cyd-creator assets load %s out of order", name)
		}
		last = at
	}
}
