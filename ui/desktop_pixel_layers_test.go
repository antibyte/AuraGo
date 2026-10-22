package ui

import (
	"regexp"
	"strings"
	"testing"
)

// Source-contract tests for the Pixel layers dock (WP1). Behaviour is covered by scripts/test-pixel-layers.mjs.

func pixelLayersSource(t *testing.T) string {
	t.Helper()
	return normalizePixelAsset(readDesktopAssetText(t, "js/desktop/apps/pixel-layers.js"))
}

func TestPixelLayersModuleRegistersDockPanelAndMenu(t *testing.T) {
	source := pixelLayersSource(t)
	for _, want := range []string{
		"Pixel.registerModule('layers'",
		"id: 'layers'",
		"dock: 'bottom'",
		"order: 10",
		"Pixel.registerPanel({",
		"Pixel.registerMenuItems('layer'",
		// markup hooks of the dock; data-layer-list is also asserted by the core registry test
		"data-layer-list",
		"data-layer-rows",
		"data-layer-drop",
		"data-layer-blend",
		"data-layer-opacity-range",
		"data-layer-lock=",
		"data-layer-act=",
		"data-layer-eye",
		"data-layer-mask-thumb",
		"data-layer-thumb",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("pixel-layers.js is missing %q", want)
		}
	}
	// DOM work belongs in the panel mount; the module installer only extends the runtime.
	install := jsFunctionBodyInWindowMenuTest(t, source, "Pixel.registerModule('layers', function install(r)")
	if strings.Contains(install, "document.") || strings.Contains(install, "querySelector") {
		t.Fatalf("the layers module installer must not touch the DOM: %s", install)
	}
	if !strings.Contains(source, "mount: (r, section) => { r.layersPanel = createPanel(r, section); }") {
		t.Fatal("the layers dock must be built in the panel mount")
	}
	// Everything that refreshed the built-in list calls refreshLayerPanel; the module takes that over.
	if !strings.Contains(install, "r.refreshLayerPanel = Pixel.bindRuntime(r, function refreshLayerPanel()") {
		t.Fatal("the module must replace runtime.refreshLayerPanel so history restore and sticker code refresh the new dock")
	}
}

func TestPixelLayersShortcutsAreRegistered(t *testing.T) {
	source := pixelLayersSource(t)
	for _, shortcut := range []string{
		"shortcut: 'Ctrl+J'",
		"shortcut: 'Ctrl+E'",
		"shortcut: 'Ctrl+Shift+E'",
		"shortcut: 'Ctrl+Alt+G'",
		"shortcut: 'Ctrl+]'",
		"shortcut: 'Ctrl+['",
		"shortcut: 'Alt+]'",
		"shortcut: 'Alt+['",
		"shortcut: 'F2'",
		// layouts without bracket keys get arrow alternatives
		"'Ctrl+Shift+ArrowUp'",
		"'Alt+ArrowDown'",
	} {
		if !strings.Contains(source, shortcut) {
			t.Fatalf("pixel-layers.js should register %s", shortcut)
		}
	}
}

func TestPixelLayersMenuCoversTheBrief(t *testing.T) {
	source := pixelLayersSource(t)
	for _, id := range []string{
		"new-layer", "new-fill-layer", "via-copy", "duplicate", "delete", "rename", "properties",
		"merge-down", "merge-visible", "flatten", "rasterize",
		"mask-reveal", "mask-from-selection", "mask-invert", "mask-apply", "mask-delete", "mask-disable",
		"select-from-layer", "clip", "fill-primary", "fill-secondary", "clear",
		"place-image", "export-layer",
	} {
		if !strings.Contains(source, "item('"+id+"',") {
			t.Fatalf("the Layer menu is missing the item %q", id)
		}
	}
	for _, mode := range []string{"left", "hcenter", "right", "top", "vcenter", "bottom"} {
		if !strings.Contains(source, "alignItem('"+mode+"',") {
			t.Fatalf("the Layer menu is missing align %q", mode)
		}
	}
	for _, mode := range []string{"Pixel.BLEND_MODES", "r.BLEND_MODES"} {
		if strings.Contains(source, mode) {
			return
		}
	}
	t.Fatal("the blend mode list must come from Pixel.BLEND_MODES, not a second copy")
}

func TestPixelLayersOperationsGoThroughTheCore(t *testing.T) {
	source := pixelLayersSource(t)
	start := strings.Index(source, "function createOps(r)")
	end := strings.Index(source, "// --- Layer menu")
	if start < 0 || end < start {
		t.Fatal("pixel-layers.js should keep its operations in createOps(r)")
	}
	ops := source[start:end]
	for _, want := range []string{
		"r.createLayer(", "r.markLayerDirty(", "r.pushHistory(", "r.commitLayerPixels(", "r.setSelectionMask(", "r.getSelectionMask(",
		"r.layersStructureChanged()", "r.renderComposition(", "r.flattenDocument()", "r.MAX_LAYERS", "r.canPaintActiveLayer(",
	} {
		if !strings.Contains(ops, want) {
			t.Fatalf("layer operations should use %s", want)
		}
	}
	// Layers always own plain canvases (see TestPixelLayersAlwaysOwnCanvases); the temp pool is only for scratch work.
	for _, forbidden := range []string{"canvas: null", "layers[0].canvas"} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("pixel-layers.js must not use %q", forbidden)
		}
	}
	// An opacity drag records one history entry when it ends, not one per input event.
	if !strings.Contains(source, "history: false") {
		t.Fatal("live opacity changes must not push history")
	}
}

func TestPixelLayersStringsHaveEnglishFallbacks(t *testing.T) {
	source := pixelLayersSource(t)
	keyed := regexp.MustCompile(`tr\('(pixel\.[a-z0-9_]+)'(.)`)
	count := 0
	for _, match := range keyed.FindAllStringSubmatch(source, -1) {
		count++
		if match[2] != "," {
			t.Fatalf("tr(%q) needs an English fallback as second argument", match[1])
		}
	}
	if count < 40 {
		t.Fatalf("expected the layers module to translate its strings through tr(), found %d calls", count)
	}
	bare := regexp.MustCompile(`\.t\('pixel\.[a-z0-9_]+'\)`)
	if m := bare.FindString(source); m != "" {
		t.Fatalf("use tr(key, fallback) for new strings, found %s", m)
	}
}

func TestPixelLayersDynamicTextIsEscaped(t *testing.T) {
	source := pixelLayersSource(t)
	// layer names are user text and end up in markup
	if !strings.Contains(source, `${esc(layer.name)}`) {
		t.Fatal("layer names must be escaped before they are put into the row markup")
	}
	if strings.Contains(source, "${layer.name}") {
		t.Fatal("layer names must never be interpolated unescaped")
	}
}

func TestPixelLayersCSS(t *testing.T) {
	css := normalizePixelAsset(readDesktopAssetText(t, "css/pixel-layers.css"))
	for _, selector := range []string{
		".pixel-layers-header", ".pixel-layers-toggle", ".pixel-layers-footer", ".pixel-layers-btn",
		".pixel-layer-rows", ".pixel-layer-drop", ".pixel-layer-thumb", ".pixel-layer-maskthumb", ".pixel-layer-maskthumb.disabled",
		".pixel-layer-item.pixel-layer-clipped", ".pixel-layer-item.dragging", ".pixel-layer-badge", ".pixel-layer-rename", ".pixel-layer-dialog",
	} {
		if !strings.Contains(css, selector) {
			t.Fatalf("pixel-layers.css should style %s", selector)
		}
	}
	if !pixelCSSRuleContains(css, ".pixel-layer-drop[hidden]", "display: none") {
		t.Fatal("the drop indicator uses [hidden] and needs an explicit display:none")
	}
	// Class names stay in the pixel- namespace (cookie-blocker extensions hide cc- classes); vd- is the shell's own and a few
	// state modifiers are only ever combined with a pixel- class.
	modifiers := map[string]bool{"active": true, "disabled": true, "dragging": true, "is-dragging": true, "is-hidden": true, "editing-mask": true, "off": true}
	classes := regexp.MustCompile(`\.([A-Za-z_][\w-]*)`)
	for _, match := range classes.FindAllStringSubmatch(regexp.MustCompile(`/\*[\s\S]*?\*/`).ReplaceAllString(css, ""), -1) {
		name := match[1]
		if strings.HasPrefix(name, "cc-") || !(strings.HasPrefix(name, "pixel-") || strings.HasPrefix(name, "vd-") || modifiers[name]) {
			t.Fatalf("pixel-layers.css uses the class .%s outside the pixel- namespace", name)
		}
	}
	for _, forbidden := range []string{"@import", "url(http", "url('http", "url(\"http"} {
		if strings.Contains(css, forbidden) {
			t.Fatalf("pixel-layers.css must not load external assets (%s)", forbidden)
		}
	}
}

func TestPixelLayersFilesAreInTheLoader(t *testing.T) {
	loader := readDesktopAssetText(t, "js/desktop/core/module-loader.js")
	for _, want := range []string{"/js/desktop/apps/pixel-layers.js", "/css/pixel-layers.css"} {
		if !strings.Contains(loader, want) {
			t.Fatalf("module-loader.js should load %s for the pixel app", want)
		}
	}
	// pixel-layers.js registers over the built-in panel, so it must load after pixel-view.js.
	view := strings.Index(loader, "/js/desktop/apps/pixel-view.js")
	layers := strings.Index(loader, "/js/desktop/apps/pixel-layers.js")
	if view < 0 || layers < view {
		t.Fatal("pixel-layers.js must be loaded after pixel-view.js")
	}
}
