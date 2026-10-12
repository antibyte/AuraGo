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

// TestCYDCreatorDesignerStaticContracts checks each designer module (one subtest per file) for its
// entry points and the repository UI rules: no native dialogs and no theme colour literals in the
// renderer, canvas or interaction code. The css subtest checks the designer section of the app
// stylesheet with the self-hosted preview fonts.
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
	})
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
