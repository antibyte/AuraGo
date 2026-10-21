package ui

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"
)

func TestPixelInspectorTabsRespectHiddenPanels(t *testing.T) {
	css := normalizePixelAsset(readDesktopAssetText(t, "css/pixel.css"))

	if !pixelCSSRuleContains(css, `.pixel-panel-section[hidden]`, "display: none") {
		t.Fatalf("pixel panel sections use flex display, so [hidden] needs an explicit display:none override")
	}
	if !pixelCSSRuleContains(css, `.pixel-empty-state[hidden]`, "display: none") {
		t.Fatalf("pixel empty state uses flex display, so [hidden] needs an explicit display:none override after an image loads")
	}
}

func TestPixelInspectorLayoutFitsLocalizedControls(t *testing.T) {
	css := normalizePixelAsset(readDesktopAssetText(t, "css/pixel.css"))

	panelChecks := []string{
		"clamp(320px",
		"overflow-x: hidden",
		"box-sizing: border-box",
	}
	for _, want := range panelChecks {
		if !pixelCSSRuleContains(css, `.pixel-panel`, want) {
			t.Fatalf("pixel side panel rule should contain %q so localized controls fit", want)
		}
	}

	if !pixelCSSRuleContains(css, `.pixel-btn-group`, "grid-template-columns: repeat(2, minmax(0, 1fr))") {
		t.Fatalf("pixel button groups should use bounded grid columns instead of overflowing rows")
	}
	for _, want := range []string{"white-space: normal", "overflow-wrap: anywhere"} {
		if !pixelCSSRuleContains(css, `.pixel-btn-group .pixel-btn`, want) {
			t.Fatalf("pixel grouped buttons should contain %q to keep long labels inside the side panel", want)
		}
	}
	for _, want := range []string{"max-width: 100%", "text-overflow: ellipsis"} {
		if !pixelCSSRuleContains(css, `.pixel-filter-name`, want) {
			t.Fatalf("pixel filter labels should contain %q to avoid panel overflow", want)
		}
	}
}

func TestPixelImageMenuUsesThemeIcons(t *testing.T) {
	js := readPixelAppScripts(t)

	for _, unavailable := range []string{
		"icon: 'rotate-cw'",
		"icon: 'rotate-ccw'",
		"icon: 'crop'",
		"icon: 'resize'",
		"iconMarkup('rotate-cw'",
		"iconMarkup('rotate-ccw'",
		"iconMarkup('crop'",
		"iconMarkup('resize'",
	} {
		if strings.Contains(js, unavailable) {
			t.Fatalf("pixel image controls still reference unavailable theme icon %s", unavailable)
		}
	}

	expectedMenuIcons := map[string]string{
		"rotate-cw":  "redo",
		"rotate-ccw": "undo",
		"flip-h":     "sort",
		"flip-v":     "sort",
		"crop":       "scissors",
		"resize":     "maximize",
	}
	for id, icon := range expectedMenuIcons {
		pattern := regexp.MustCompile(`\{\s*id:\s*'` + regexp.QuoteMeta(id) + `',\s*labelKey:\s*'[^']+',\s*icon:\s*'` + regexp.QuoteMeta(icon) + `'`)
		if !pattern.MatchString(js) {
			t.Fatalf("pixel image menu item %q should use available icon %q", id, icon)
		}
	}

	for _, manifestPath := range []string{
		"img/papirus/manifest.json",
		"img/whitesur/manifest.json",
	} {
		icons := readThemeIconNames(t, manifestPath)
		for _, icon := range []string{"redo", "undo", "sort", "scissors", "maximize"} {
			if !icons[icon] {
				t.Fatalf("%s does not provide icon %q required by the Pixel image menu", manifestPath, icon)
			}
		}
	}
}

func TestPixelDrawPanelHasGridLayout(t *testing.T) {
	css := normalizePixelAsset(readDesktopAssetText(t, "css/pixel.css"))

	if !pixelCSSRuleContains(css, `.pixel-draw-tools-grid`, "grid-template-columns") {
		t.Fatalf("pixel draw tools should use a grid layout")
	}
}

func TestPixelOverlayCanvasPositionedAbsolute(t *testing.T) {
	css := normalizePixelAsset(readDesktopAssetText(t, "css/pixel.css"))

	if !pixelCSSRuleContains(css, `.pixel-overlay`, "position: absolute") {
		t.Fatalf("pixel overlay canvas must be positioned absolutely over the main canvas")
	}
	if !pixelCSSRuleContains(css, `.pixel-overlay`, "pointer-events: auto") {
		t.Fatalf("pixel overlay canvas needs pointer-events to receive drawing input")
	}
}

func TestPixelCanvasWrapExists(t *testing.T) {
	js := readPixelAppScripts(t)

	if !strings.Contains(js, "pixel-canvas-wrap") {
		t.Fatalf("pixel.js should use a canvas wrapper div for overlay positioning")
	}
	if !strings.Contains(js, "data-overlay") {
		t.Fatalf("pixel.js should have an overlay canvas element")
	}
}

func TestPixelNewMenuItemsUseAvailableIcons(t *testing.T) {
	js := readPixelAppScripts(t)

	newMenuIcons := map[string]string{
		"new-image":  "image",
		"copy":       "image",
		"cut":        "scissors",
		"paste":      "image",
		"select-all": "image",
		"deselect":   "image",
		"shortcuts":  "image",
	}
	for id, icon := range newMenuIcons {
		pattern := regexp.MustCompile(`\{\s*id:\s*'` + regexp.QuoteMeta(id) + `',\s*labelKey:\s*'[^']+',\s*icon:\s*'` + regexp.QuoteMeta(icon) + `'`)
		if !pattern.MatchString(js) {
			t.Fatalf("pixel menu item %q should use available icon %q", id, icon)
		}
	}
}

func TestPixelHasColorPickerCSS(t *testing.T) {
	css := normalizePixelAsset(readDesktopAssetText(t, "css/pixel.css"))

	for _, selector := range []string{
		".pixel-color-swatch",
		".pixel-palette-grid",
		".pixel-hex-input",
		".pixel-recent-colors",
	} {
		if !strings.Contains(css, selector) {
			t.Fatalf("pixel.css should contain styles for %q", selector)
		}
	}
}

func TestPixelHasLayerPanelCSS(t *testing.T) {
	css := normalizePixelAsset(readDesktopAssetText(t, "css/pixel.css"))

	for _, selector := range []string{
		".pixel-layer-list",
		".pixel-layer-item",
		".pixel-layer-vis",
		".pixel-layer-actions",
	} {
		if !strings.Contains(css, selector) {
			t.Fatalf("pixel.css should contain styles for %q", selector)
		}
	}
}

func TestPixelHasContextMenuCSS(t *testing.T) {
	css := normalizePixelAsset(readDesktopAssetText(t, "css/pixel.css"))

	if !strings.Contains(css, ".pixel-ctx-menu") {
		t.Fatalf("pixel.css should contain context menu styles")
	}
}

func TestPixelTextInputDoesNotLeakKeyboardShortcuts(t *testing.T) {
	js := normalizePixelAsset(readDesktopAssetText(t, "js/desktop/apps/pixel-events.js"))
	body := jsFunctionBodyInWindowMenuTest(t, js, "showTextInput: Pixel.bindRuntime(runtime, function showTextInput(x, y)")

	for _, want := range []string{
		"ev.stopPropagation();",
		"ev.stopImmediatePropagation();",
		"textarea.addEventListener('keyup', stopTextShortcutPropagation);",
		"textarea.addEventListener('keypress', stopTextShortcutPropagation);",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("Pixel text input should contain %q to keep typing from triggering shortcuts: %s", want, body)
		}
	}
}

func TestPixelOpenDialogStartsInPhotosRoot(t *testing.T) {
	actions := normalizePixelAsset(readDesktopAssetText(t, "js/desktop/apps/pixel-actions.js"))
	dialog := normalizePixelAsset(readDesktopAssetText(t, "js/desktop/core/file-dialog-runtime.js"))
	openBody := jsFunctionBodyInWindowMenuTest(t, actions, "openFile: Pixel.bindRuntime(runtime, async function openFile()")

	if !strings.Contains(openBody, "initialPath: 'Photos'") {
		t.Fatalf("Pixel open dialog should start in the Photos root: %s", openBody)
	}
	if !strings.Contains(dialog, "'Pictures', 'Photos',") {
		t.Fatalf("desktop file dialog should expose Photos next to Pictures in default roots")
	}
}

func TestPixelActionsUseRootAPIEndpoints(t *testing.T) {
	actions := normalizePixelAsset(readDesktopAssetText(t, "js/desktop/apps/pixel-actions.js"))

	if strings.Contains(actions, "/this.api/") {
		t.Fatalf("Pixel actions must call root /api endpoints, not transformed /this.api paths")
	}
	for _, want := range []string{
		"/api/desktop/files?path=Pictures&recursive=true&limit=30",
		"/api/desktop/preview?path=",
		"/api/pixel/save",
		"/api/pixel/generate",
		"/api/pixel/enhance",
		"/api/pixel/remove-bg",
		"/api/pixel/upscale",
		"/api/pixel/config",
	} {
		if !strings.Contains(actions, want) {
			t.Fatalf("Pixel actions missing API endpoint %q", want)
		}
	}
}

func TestPixelLoadsDesktopPreviewAsImageSource(t *testing.T) {
	actions := normalizePixelAsset(readDesktopAssetText(t, "js/desktop/apps/pixel-actions.js"))
	shell := normalizePixelAsset(readDesktopAssetText(t, "js/desktop/apps/pixel.js"))

	if strings.Contains(actions, "this.api('/api/desktop/preview?path='") {
		t.Fatalf("Pixel desktop image loads must not parse /api/desktop/preview as JSON; it serves inline image bytes")
	}
	if strings.Contains(shell, "api('/api/desktop/preview?path='") {
		t.Fatalf("Pixel shell image entry points must not parse /api/desktop/preview as JSON; it serves inline image bytes")
	}
	if !strings.Contains(actions, "loadDesktopImagePath: Pixel.bindRuntime(runtime, async function loadDesktopImagePath(") {
		t.Fatalf("Pixel actions should expose a shared desktop image loader for dialog, recent, photo, drop, and launch paths")
	}
	loadBody := jsFunctionBodyInWindowMenuTest(t, actions, "loadDesktopImagePath: Pixel.bindRuntime(runtime, async function loadDesktopImagePath(")
	for _, want := range []string{"fetch('/api/desktop/preview?path='", "response.blob()", "response.headers.get('ETag')", "URL.revokeObjectURL", "this.state.fileVersion"} {
		if !strings.Contains(loadBody, want) {
			t.Fatalf("Pixel must bind the loaded image to its observed version and release its object URL; missing %s", want)
		}
	}
	if !strings.Contains(shell, "loadDesktopImagePath(filePath).catch(() => {});") {
		t.Fatalf("Pixel initial launch with ctx.path should load that desktop image path")
	}
}

func TestPixelGermanOpenLabelsUseUmlaut(t *testing.T) {
	var values map[string]string
	if err := json.Unmarshal([]byte(readDesktopAssetText(t, "lang/desktop/de.json")), &values); err != nil {
		t.Fatalf("parse German desktop translations: %v", err)
	}

	for _, key := range []string{
		"desktop.file_dialog_open",
		"desktop.gallery_open",
		"desktop.media_open",
		"pixel.open",
	} {
		value := values[key]
		if strings.Contains(value, "oe") || strings.Contains(value, "Oe") {
			t.Fatalf("%s should use an umlaut instead of oe transliteration, got %q", key, value)
		}
		if !strings.Contains(value, "öff") && !strings.Contains(value, "Öff") {
			t.Fatalf("%s should contain öffnen/Öffnen, got %q", key, value)
		}
	}
}

func TestPixelAdjustApplyCommitsPreviewWithoutRestoringOriginal(t *testing.T) {
	// The Filters module (pixel-filters.js) owns the adjust sliders; the old code in pixel-canvas.js is replaced at install time.
	js := normalizePixelAsset(readDesktopAssetText(t, "js/desktop/apps/pixel-filters.js"))
	applyBody := jsFunctionBodyInWindowMenuTest(t, js, "applyAdjustments: Pixel.bindRuntime(runtime, function applyAdjustments()")

	if strings.Contains(applyBody, "resetAdjustments();") {
		t.Fatalf("Pixel applyAdjustments must not call resetAdjustments because it drops the preview before it is committed: %s", applyBody)
	}
	if !strings.Contains(applyBody, "applyAdjustValues(") {
		t.Fatalf("Pixel applyAdjustments should commit through the shared adjust commit path: %s", applyBody)
	}
	valuesBody := jsFunctionBodyInWindowMenuTest(t, js, "function applyAdjustValues(label)")
	if !strings.Contains(valuesBody, "commit(s, label || 'adjust'") || !strings.Contains(valuesBody, "resetAdjustmentControls()") {
		t.Fatalf("Pixel adjust apply should commit the preview and then reset the slider controls: %s", valuesBody)
	}
	finishBody := jsFunctionBodyInWindowMenuTest(t, js, "function finish(s, img, label)")
	if !strings.Contains(finishBody, "commitLayerPixels(s.layer, canvas, { label })") {
		t.Fatalf("Pixel previews should be committed through commitLayerPixels (selection, locks, history): %s", finishBody)
	}
	if !strings.Contains(js, "captureAdjustmentSource: Pixel.bindRuntime(runtime, function captureAdjustmentSource()") {
		t.Fatal("Pixel keeps captureAdjustmentSource for the modules that call it after a document or layer change")
	}
}

func TestPixelCompareModeClearsWhenLeavingAdjustPanel(t *testing.T) {
	tools := normalizePixelAsset(readDesktopAssetText(t, "js/desktop/apps/pixel-tools.js"))
	canvas := normalizePixelAsset(readDesktopAssetText(t, "js/desktop/apps/pixel-canvas.js"))
	view := normalizePixelAsset(readDesktopAssetText(t, "js/desktop/apps/pixel-view.js"))
	filtersJS := normalizePixelAsset(readDesktopAssetText(t, "js/desktop/apps/pixel-filters.js"))
	showPanelBody := jsFunctionBodyInWindowMenuTest(t, tools, "showPanel: Pixel.bindRuntime(runtime, function showPanel(name)")

	if !strings.Contains(canvas, "exitCompareMode: Pixel.bindRuntime(runtime, function exitCompareMode(") || !strings.Contains(filtersJS, "exitCompareMode: Pixel.bindRuntime(runtime, function exitCompareMode(") {
		t.Fatalf("Pixel needs a shared exitCompareMode helper so panel changes can leave compare mode cleanly")
	}
	if !strings.Contains(showPanelBody, "runHook(old, 'onHide', this)") {
		t.Fatalf("Pixel showPanel should run the onHide hook of the panel that is left: %s", showPanelBody)
	}
	// The Filters module replaces the built-in adjust and filters panels by registering the same ids.
	adjust := pixelPanelRegistration(t, filtersJS, "adjust")
	if !strings.Contains(adjust, "onHide: r => { if (r.compareMode) r.exitCompareMode({ preservePreview: true }); }") {
		t.Fatalf("the adjust panel should disable compare mode when it is left so drawing is not blocked: %s", adjust)
	}
	filters := pixelPanelRegistration(t, filtersJS, "filters")
	if !strings.Contains(filters, "onHide: r => r.resetFilterPreview()") {
		t.Fatalf("the filters panel should drop its preview when it is left: %s", filters)
	}
	transform := pixelPanelRegistration(t, view, "transform")
	if !strings.Contains(transform, "r.cancelCrop()") {
		t.Fatalf("the transform panel should cancel a running crop when it is left: %s", transform)
	}
}

// pixelPanelRegistration returns the source of one Pixel.registerPanel({ id: '<id>' ... }) call.
func pixelPanelRegistration(t *testing.T, source, id string) string {
	t.Helper()
	marker := "Pixel.registerPanel({ id: '" + id + "'"
	start := strings.Index(source, marker)
	if start < 0 {
		t.Fatalf("missing %s", marker)
	}
	end := strings.Index(source[start+len(marker):], "Pixel.registerPanel({")
	if end < 0 {
		end = len(source) - start - len(marker)
	}
	return source[start : start+len(marker)+end]
}

func TestPixelLayersAlwaysOwnCanvases(t *testing.T) {
	var scripts strings.Builder
	for _, scriptPath := range pixelAppScriptPaths {
		if strings.HasPrefix(scriptPath, "js/desktop/apps/pixel") {
			scripts.WriteString(normalizePixelAsset(readDesktopAssetText(t, scriptPath)))
			scripts.WriteByte('\n')
		}
	}
	source := scripts.String()
	for _, forbidden := range []string{"canvas: null", "ensureBackgroundMigrated", "layers[0].canvas"} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("pixel scripts must not keep the shared display/background layer model (%q)", forbidden)
		}
	}
	activeCtx := jsFunctionBodyInWindowMenuTest(t, source, "getActiveCtx: Pixel.bindRuntime(runtime, function getActiveCtx()")
	if strings.Contains(activeCtx, "this.cctx") {
		t.Fatalf("getActiveCtx must return the layer (or mask) context, never the display canvas: %s", activeCtx)
	}
	for _, signature := range []string{
		"loadImageToCanvas: Pixel.bindRuntime(runtime, async function loadImageToCanvas(",
		"newBlankCanvas: Pixel.bindRuntime(runtime, function newBlankCanvas(",
		"addLayer: Pixel.bindRuntime(runtime, function addLayer()",
		"flattenLayers: Pixel.bindRuntime(runtime, function flattenLayers()",
	} {
		if body := jsFunctionBodyInWindowMenuTest(t, source, signature); !strings.Contains(body, "createLayer(") {
			t.Fatalf("%s must create layers through createLayer so every layer owns a document-sized canvas: %s", signature, body)
		}
	}
	paste := jsFunctionBodyInWindowMenuTest(t, source, "pasteClipboard: Pixel.bindRuntime(runtime, async function pasteClipboard()")
	if strings.Contains(paste, "newBlankCanvas(") || !strings.Contains(paste, "createLayer(") {
		t.Fatalf("paste must add a layer instead of replacing the document: %s", paste)
	}
}

func TestPixelCoreExposesLayerHistoryAndSelectionContract(t *testing.T) {
	core := normalizePixelAsset(readDesktopAssetText(t, "js/desktop/apps/pixel-core.js"))
	for _, want := range []string{
		"Pixel.installCore = function installCore(runtime)",
		"on: Pixel.bindRuntime(runtime, function on(event, fn)",
		"emit: Pixel.bindRuntime(runtime, function emit(event, data)",
		"createLayer: Pixel.bindRuntime(runtime, function createLayer(",
		"markLayerDirty: Pixel.bindRuntime(runtime, function markLayerDirty(",
		"getActiveReadCtx: Pixel.bindRuntime(runtime, function getActiveReadCtx()",
		"flattenDocument: Pixel.bindRuntime(runtime, function flattenDocument(",
		"setLayerPreview: Pixel.bindRuntime(runtime, function setLayerPreview(",
		"commitLayerPixels: Pixel.bindRuntime(runtime, function commitLayerPixels(",
		"getSelectionMask: Pixel.bindRuntime(runtime, function getSelectionMask()",
		"setSelectionMask: Pixel.bindRuntime(runtime, function setSelectionMask(",
		"invertSelection: Pixel.bindRuntime(runtime, function invertSelection()",
		"'layers-changed'",
		"'history-changed'",
		"'document-changed'",
		"'active-layer-changed'",
	} {
		if !strings.Contains(core, want) {
			t.Fatalf("pixel-core.js is missing %q", want)
		}
	}
	// compositeLayers runs on every mouse move while painting and must not rebuild the sticker DOM.
	composite := jsFunctionBodyInWindowMenuTest(t, core, "compositeLayers: Pixel.bindRuntime(runtime, function compositeLayers()")
	if strings.Contains(composite, "refreshStickerControls") {
		t.Fatalf("compositeLayers must not refresh sticker controls: %s", composite)
	}
	// Mask editing paints alpha: brush reveals with opaque white, the eraser hides; the eraser never paints black.
	events := normalizePixelAsset(readDesktopAssetText(t, "js/desktop/apps/pixel-events.js"))
	spec := jsFunctionBodyInWindowMenuTest(t, events, "brushStrokeSpec: Pixel.bindRuntime(runtime, function brushStrokeSpec()")
	for _, want := range []string{"'destination-out'", "inMask ? '#ffffff'"} {
		if !strings.Contains(spec, want) {
			t.Fatalf("brushStrokeSpec must paint masks as alpha (missing %q): %s", want, spec)
		}
	}
	if strings.Contains(events, "maskErase") {
		t.Fatal("the mask eraser must hide through destination-out, not by painting opaque black")
	}
}

func TestPixelHistorySharesUnchangedLayerSnapshots(t *testing.T) {
	core := normalizePixelAsset(readDesktopAssetText(t, "js/desktop/apps/pixel-core.js"))
	push := jsFunctionBodyInWindowMenuTest(t, core, "pushHistory: Pixel.bindRuntime(runtime, function pushHistory(label)")
	for _, want := range []string{"old.snap.rev === l.rev", "old.maskSnap.maskRev === l.maskRev", "copyCanvas(", "HISTORY_BYTE_BUDGET", "this.MAX_HISTORY"} {
		if !strings.Contains(push, want) {
			t.Fatalf("pushHistory must reuse snapshots of layers whose rev did not change (missing %q): %s", want, push)
		}
	}
	if strings.Contains(push, "acquireTempCanvas") {
		t.Fatalf("history snapshots must be plain canvases; releasing a pooled canvas would clear shared snapshots: %s", push)
	}
	restore := jsFunctionBodyInWindowMenuTest(t, core, "restoreHistory: Pixel.bindRuntime(runtime, function restoreHistory(entry)")
	if !strings.Contains(restore, "old.rev === meta.rev") || !strings.Contains(restore, "entry.selection") {
		t.Fatalf("restoreHistory must reuse canvases with matching revs and restore the selection: %s", restore)
	}
}

func TestPixelModulesHaveValidTransformedSyntaxMarkers(t *testing.T) {
	js := readPixelAppScripts(t)
	for _, forbidden := range []string{
		"this.canvas:",
		"createElement('this.canvas')",
		"data-section=\"this.layers\"",
		"data-action=\"clear-this.selection\"",
		"pixel-crop-this.selection",
		"merge-this.layers",
		"flatten-this.layers",
		"iconMarkup('this.redo'",
		"iconMarkup('this.undo'",
	} {
		if strings.Contains(js, forbidden) {
			t.Fatalf("pixel scripts contain invalid bindRuntime transform artifact %q", forbidden)
		}
	}
}

func TestPixelWiredActionsExistInMarkup(t *testing.T) {
	scripts := readPixelAppScripts(t)
	actionRe := regexp.MustCompile(`\[data-action="([^"]+)"\]`)
	seen := make(map[string]bool)
	for _, match := range actionRe.FindAllStringSubmatch(scripts, -1) {
		action := match[1]
		if seen[action] {
			continue
		}
		seen[action] = true
		// Markup is an attribute that is not the selector form [data-action="x"].
		markup := regexp.MustCompile(`(?:^|[^\[])data-action="` + regexp.QuoteMeta(action) + `"`)
		if !markup.MatchString(scripts) {
			t.Fatalf("pixel markup is missing wired action %q", action)
		}
	}
	if len(seen) < 20 {
		t.Fatalf("expected the panels and toolbar to wire more than %d actions", len(seen))
	}
}

func TestPixelBindRuntimeDoesNotUseEval(t *testing.T) {
	state := readDesktopAssetText(t, "js/desktop/apps/pixel-state.js")
	for _, forbidden := range []string{"eval(", "new Function(", "Function('runtime'"} {
		if strings.Contains(state, forbidden) {
			t.Fatalf("pixel-state.js must not use runtime code evaluation (%s)", forbidden)
		}
	}
	if !strings.Contains(state, "fn.apply(runtime, args)") {
		t.Fatal("pixel bindRuntime should bind methods with fn.apply(runtime, args)")
	}
}

func TestPixelLoaderUsesOrderedSemanticScripts(t *testing.T) {
	loader := readDesktopAssetText(t, "js/desktop/core/module-loader.js")

	last := -1
	for _, scriptPath := range pixelAppScriptPaths {
		needle := "'" + "/" + scriptPath + "'"
		idx := strings.Index(loader, needle)
		if idx < 0 {
			t.Fatalf("pixel loader is missing %s", needle)
		}
		if idx <= last {
			t.Fatalf("pixel loader should load %s after the previous Pixel script", needle)
		}
		last = idx
	}
}

func TestPixelLoaderListsStylesInOrder(t *testing.T) {
	loader := readDesktopAssetText(t, "js/desktop/core/module-loader.js")
	// Only the pixel entry: other apps share some of these files (stickers) earlier in the loader.
	start := strings.Index(loader, "'pixel': {")
	end := strings.Index(loader[start:], "'galaxa-deluxe'")
	if start < 0 || end < 0 {
		t.Fatal("pixel entry not found in the module loader")
	}
	loader = loader[start : start+end]
	last := -1
	for _, stylePath := range pixelAppStylePaths {
		needle := "'/" + stylePath + "'"
		idx := strings.Index(loader, needle)
		if idx < 0 {
			t.Fatalf("pixel loader is missing %s", needle)
		}
		if idx <= last {
			t.Fatalf("pixel loader should load %s after the previous Pixel style", needle)
		}
		last = idx
	}
	if strings.Contains(loader, "pixel-worker.js'") && strings.Contains(loader, "'/js/desktop/apps/pixel-worker.js'") {
		t.Fatal("pixel-worker.js is a Worker script created by runtime.runJob and must not be listed in the module loader")
	}
}

func TestPixelToolRailAndOptionsBarExist(t *testing.T) {
	shell := normalizePixelAsset(readDesktopAssetText(t, "js/desktop/apps/pixel.js"))
	view := normalizePixelAsset(readDesktopAssetText(t, "js/desktop/apps/pixel-view.js"))

	for _, want := range []string{"data-tool-rail", "data-options-bar", "data-panel-container", "buildToolRailHTML()", "buildDocksHTML()"} {
		if !strings.Contains(shell, want) {
			t.Fatalf("pixel shell should contain %q for the tool rail / options bar / dock layout", want)
		}
	}
	for _, want := range []string{"buildToolRailHTML", "pixel-tool-rail", "Pixel.toolGroups()", "pixel-rail-colors", "data-panel=\"${this.esc(def.id)}\"", "data-section=\"${this.esc(def.id)}\"", "data-dock-splitter"} {
		if !strings.Contains(view, want) {
			t.Fatalf("pixel view should contain %q: the rail, tabs and sections are built from the registries", want)
		}
	}
	// The built-in panels: inspector tabs plus the bottom dock with layers and history.
	for _, id := range []string{"adjust", "filters", "colors", "transform", "ai"} {
		if panel := pixelPanelRegistration(t, view, id); !strings.Contains(panel, "dock: 'inspector'") {
			t.Fatalf("panel %q should live in the inspector dock: %s", id, panel)
		}
	}
	for _, id := range []string{"layers", "history"} {
		if panel := pixelPanelRegistration(t, view, id); !strings.Contains(panel, "dock: 'bottom'") {
			t.Fatalf("panel %q should live in the bottom dock: %s", id, panel)
		}
	}
	stickers := normalizePixelAsset(readDesktopAssetText(t, "js/desktop/apps/pixel-stickers.js"))
	if !strings.Contains(stickers, "Pixel.registerPanel({ id: 'stickers'") {
		t.Fatal("stickers should register as an inspector panel")
	}
}

func TestPixelNewToolsRegistered(t *testing.T) {
	state := normalizePixelAsset(readDesktopAssetText(t, "js/desktop/apps/pixel-state.js"))
	for _, tool := range []string{"magic-wand", "airbrush", "dodge-burn", "blur-brush", "gradient", "clone-stamp", "lasso", "move"} {
		if !strings.Contains(state, "'"+tool+"'") {
			t.Fatalf("pixel-state.js should register new tool %q in TOOL_GROUPS/TOOL_SVGS", tool)
		}
	}
	if !strings.Contains(state, "'line': 'Shift+L'") {
		t.Fatal("pixel-state.js should move line shortcut to Shift+L because lasso uses L")
	}
	events := normalizePixelAsset(readDesktopAssetText(t, "js/desktop/apps/pixel-events.js"))
	for _, want := range []string{"magicWandSelect", "applyPointTool", "snapEnd", "drawBrushCursor", "cloneStampAt", "commitLassoPath", "commitMoveLayer", "drawLassoPreview"} {
		if !strings.Contains(events, want) {
			t.Fatalf("pixel-events.js missing new tool wiring %q", want)
		}
	}
	tools := normalizePixelAsset(readDesktopAssetText(t, "js/desktop/apps/pixel-tools.js"))
	for _, want := range []string{"commitGradient", "sprayAt", "dodgeBurnAt", "blurAt", "eraseMaskPixels", "renderOptionsBar", "toggleLayerMask", "MAX_LAYERS", "beginMoveSelection", "commitMoveSelection", "cloneSampleCanvas"} {
		if !strings.Contains(tools, want) {
			t.Fatalf("pixel-tools.js missing new tool implementation %q", want)
		}
	}
	canvas := normalizePixelAsset(readDesktopAssetText(t, "js/desktop/apps/pixel-canvas.js"))
	if !strings.Contains(canvas, "expandCanvasToFit") {
		t.Fatal("pixel-canvas.js should expose expandCanvasToFit for oversized AI layers")
	}
	if strings.Contains(tools, "layers.length >= 10") {
		t.Fatal("pixel-tools.js should use MAX_LAYERS instead of hard-coded 10")
	}
}

func TestPixelFilterCatalogHasNewFilters(t *testing.T) {
	filters := normalizePixelAsset(readDesktopAssetText(t, "js/desktop/apps/pixel-filters.js"))
	if !strings.Contains(filters, "Pixel.FILTERS = [") || !strings.Contains(filters, "def.op = 'fx.apply'") {
		t.Fatal("pixel filters should keep the Pixel.FILTERS catalog and run every filter through the fx.apply worker op")
	}

	// The v2 filters and every id of the first catalog, which must keep working (favourites and history labels use them).
	for _, id := range []string{"levels", "curves", "hue-saturation", "color-balance", "vibrance", "channel-mixer", "black-white", "photo-filter", "gradient-map", "threshold", "equalize",
		"auto-levels", "auto-contrast", "auto-white-balance", "box-blur", "motion-blur", "radial-blur", "zoom-blur", "lens-blur", "surface-blur", "median", "unsharp-mask", "high-pass",
		"twirl", "pinch", "wave", "ripple", "polar", "spherize", "kaleidoscope", "displace", "glass", "lens-distortion", "oil-paint", "halftone", "dither", "pixel-art", "toon",
		"crystallize", "pointillize", "glowing-edges", "comic", "neon", "add-noise", "dust-scratches", "light-leak", "lens-flare", "vhs", "crt", "glitch", "cross-process"} {
		if !strings.Contains(filters, "id: '"+id+"'") {
			t.Fatalf("pixel filter catalog missing new filter %q", id)
		}
	}
	for _, legacy := range []string{"grayscale", "monochrome", "sepia", "invert", "warm", "cool", "tint", "posterize", "duotone", "cyanotype", "solarize", "vintage", "fade", "high-contrast",
		"vignette", "bloom", "glow", "blur", "sharpen", "edge", "emboss", "denoise", "clarity", "pixelate", "mosaic", "grain", "sketch", "aberration", "tilt-shift"} {
		if !strings.Contains(filters, "id: '"+legacy+"'") {
			t.Fatalf("pixel filter catalog lost legacy filter %q", legacy)
		}
	}
	for _, cat := range []string{"adjust", "blur", "sharpen", "distort", "stylize", "noise", "light", "retro"} {
		if !strings.Contains(filters, "cat: '"+cat+"'") {
			t.Fatalf("pixel filter catalog has no %q category", cat)
		}
	}
	for _, want := range []string{"data-filter-strength", "data-filter-thumb", "data-filter-cat", "data-filter-search", "refreshFilterThumbnails", `data-action="apply-filter"`, `data-action="reset-filter"`,
		"toggleFilterFavorite", "filterCompareMode", "pixel.filter_favorites", "setLayerPreview(", "key: 'fx-preview'", "runJob('fx.apply'", "'filter:' + s.filterId", "repeatLastFilter", "'Ctrl+Alt+F'",
		"shortcut: 'Ctrl+F'", "Pixel.registerMenuItems('filter'", "buildParamControls(", "wireParamControls("} {
		if !strings.Contains(filters, want) {
			t.Fatalf("pixel filters module missing %q", want)
		}
	}
	// The fade (strength + blend mode) applies to the filtered side of the preview.
	if !strings.Contains(filters, "data-filter-blend") || !strings.Contains(filters, "fade: rc.fade") {
		t.Fatal("pixel filter preview should honour the strength and blend mode of the filtered side")
	}
}

func TestPixelFiltersLibraryOpsAndAdjustPanel(t *testing.T) {
	lib := normalizePixelAsset(readDesktopAssetText(t, "js/desktop/apps/pixel-lib-fx.js"))
	for _, want := range []string{"Lib.registerOp('fx.apply'", "Lib.registerOp('fx.render'", "Lib.registerOp('fx.compose'", "Lib.registerOp('fx.histogram'", "Lib.registerOp('fx.autoTone'", "def('adjust'"} {
		if !strings.Contains(lib, want) {
			t.Fatalf("pixel-lib-fx.js missing %q", want)
		}
	}
	filters := normalizePixelAsset(readDesktopAssetText(t, "js/desktop/apps/pixel-filters.js"))
	for _, want := range []string{"Pixel.registerPanel({ id: 'adjust'", "Pixel.registerPanel({ id: 'filters'", "runJob('fx.autoTone'", "runJob('fx.histogram'", "data-fx-hist", `data-action="adjust-auto"`,
		"autoEnhance", "ADJUST_PRESETS", "'Ctrl+Shift+L'", "dehaze", "curvesControl"} {
		if !strings.Contains(filters, want) {
			t.Fatalf("pixel filters module missing %q", want)
		}
	}
	css := normalizePixelAsset(readDesktopAssetText(t, "css/pixel-filters.css"))
	for _, selector := range []string{".pixel-fx-search", ".pixel-fx-detail", ".pixel-fx-hist", ".pixel-fx-curves-canvas", ".pixel-fx-status"} {
		if !strings.Contains(css, selector) {
			t.Fatalf("pixel-filters.css should style %q", selector)
		}
	}
	if strings.Contains(css, ".cc-") || strings.Contains(filters, "cc-") {
		t.Fatal("pixel filters must not use cookie-blocker-prone cc- class names")
	}
}

func TestPixelHistoryPanelExists(t *testing.T) {
	tools := normalizePixelAsset(readDesktopAssetText(t, "js/desktop/apps/pixel-tools.js"))
	canvas := normalizePixelAsset(readDesktopAssetText(t, "js/desktop/apps/pixel-canvas.js"))

	for _, want := range []string{"refreshHistoryPanel", "jumpToHistory", "data-history-idx"} {
		if !strings.Contains(tools, want) {
			t.Fatalf("pixel-tools.js missing history panel feature %q", want)
		}
	}
	if !strings.Contains(canvas, "if (this.refreshHistoryPanel) this.refreshHistoryPanel();") {
		t.Fatal("pixel-canvas.js should refresh the history panel on push/undo/redo")
	}
}

func TestPixelCssHasToolRailAndGalleryStyles(t *testing.T) {
	css := normalizePixelAsset(readDesktopAssetText(t, "css/pixel.css"))

	for _, selector := range []string{
		".pixel-tool-rail",
		".pixel-options-bar",
		".pixel-filter-gallery",
		".pixel-filter-thumb",
		".pixel-chip",
		".pixel-history-list",
		".pixel-history-item",
	} {
		if !strings.Contains(css, selector) {
			t.Fatalf("pixel.css should contain styles for %q", selector)
		}
	}
	if !pixelCSSRuleContains(css, `.pixel-options-bar[hidden]`, "display: none") {
		t.Fatal("pixel options bar uses flex display, so [hidden] needs an explicit display:none override")
	}
}

func TestPixelNewToolI18nKeysExist(t *testing.T) {
	langDir := []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"}
	required := []string{
		"pixel.magic_wand",
		"pixel.airbrush",
		"pixel.dodge_burn",
		"pixel.blur_brush",
		"pixel.gradient",
		"pixel.clone_stamp",
		"pixel.lasso",
		"pixel.move",
		"pixel.filter_strength",
		"pixel.apply_filter",
		"pixel.filter_compare",
		"pixel.remove_bg",
		"pixel.upscale_2x",
		"pixel.export_format",
		"pixel.tab_colors",
		"pixel.tab_history",
		"pixel.history_empty",
	}
	for _, lang := range langDir {
		var values map[string]string
		if err := json.Unmarshal([]byte(readDesktopAssetText(t, "lang/desktop/"+lang+".json")), &values); err != nil {
			t.Fatalf("parse %s desktop translations: %v", lang, err)
		}
		for _, key := range required {
			if strings.TrimSpace(values[key]) == "" {
				t.Fatalf("%s missing translation for %q", lang, key)
			}
		}
	}
}

// Every pixel-lib-*.js file must be imported by the worker (and nothing else may be): a new lib that the worker does not
// import would work in the main-thread fallback and silently break in the worker.
func TestPixelWorkerImportsEveryLib(t *testing.T) {
	entries, err := Content.ReadDir("js/desktop/apps")
	if err != nil {
		t.Fatalf("list pixel apps: %v", err)
	}
	var libs []string
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, "pixel-lib-") && strings.HasSuffix(name, ".js") {
			libs = append(libs, name)
		}
	}
	if len(libs) < 5 {
		t.Fatalf("expected pixel-lib-core.js and the four stub libs, got %v", libs)
	}
	worker := normalizePixelAsset(readDesktopAssetText(t, "js/desktop/apps/pixel-worker.js"))
	if !strings.Contains(worker, "importScripts(file + search)") || !strings.Contains(worker, "self.location.search") {
		t.Fatal("pixel-worker.js must import every lib with the cache-busting query string of its own URL")
	}
	for _, lib := range libs {
		if !strings.Contains(worker, "'"+lib+"'") {
			t.Fatalf("pixel-worker.js does not import %s", lib)
		}
		if !slicesContains(pixelAppScriptPaths, "js/desktop/apps/"+lib) {
			t.Fatalf("%s is missing from pixelAppScriptPaths / the module loader", lib)
		}
		source := readDesktopAssetText(t, "js/desktop/apps/"+lib)
		for _, forbidden := range []string{"document.", "window.", "localStorage", "createElement"} {
			if strings.Contains(source, forbidden) {
				t.Fatalf("%s must stay DOM-free (it runs in a Worker): found %q", lib, forbidden)
			}
		}
	}
	for _, want := range []string{"handleMessage", "{ ready: true }", "transfer"} {
		if !strings.Contains(worker, want) {
			t.Fatalf("pixel-worker.js should contain %q", want)
		}
	}
}

func slicesContains(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}

func TestPixelRegistriesExist(t *testing.T) {
	core := normalizePixelAsset(readDesktopAssetText(t, "js/desktop/apps/pixel-core.js"))
	for _, want := range []string{
		"Pixel.registerModule = function registerModule(",
		"Pixel.registerPanel = function registerPanel(",
		"Pixel.registerTool = function registerTool(",
		"Pixel.registerMenuItems = function registerMenuItems(",
		"Pixel.registerCommand = function registerCommand(",
		"Pixel.composeMenus = function composeMenus(",
		"Pixel.listCommands = function listCommands(",
		"registerOverlay: Pixel.bindRuntime(runtime, function registerOverlay(",
		"unregisterOverlay: Pixel.bindRuntime(runtime, function unregisterOverlay(",
		"requestOverlay: Pixel.bindRuntime(runtime, function requestOverlay(",
		"runJob: Pixel.bindRuntime(runtime, function runJob(",
		"imageFromCanvas: Pixel.bindRuntime(runtime, function imageFromCanvas(",
		"canvasFromImage: Pixel.bindRuntime(runtime, function canvasFromImage(",
		"new Worker(workerURL())",
		"'/js/desktop/apps/pixel-worker.js'",
		"AbortError",
	} {
		if !strings.Contains(core, want) {
			t.Fatalf("pixel-core.js is missing %q", want)
		}
	}
	view := normalizePixelAsset(readDesktopAssetText(t, "js/desktop/apps/pixel-view.js"))
	for _, want := range []string{
		"Pixel.registerPanel({ id: 'colors'",
		"Pixel.registerPanel({ id: 'layers'",
		"Pixel.registerPanel({ id: 'history'",
		"buildParamControls: Pixel.bindRuntime(runtime, function buildParamControls(",
		"wireParamControls: Pixel.bindRuntime(runtime, function wireParamControls(",
		"defaultParams: Pixel.bindRuntime(runtime, function defaultParams(",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("pixel-view.js is missing %q", want)
		}
	}
	// Every shell piece the later packages need is a registry call, so they never edit pixel.js.
	shell := normalizePixelAsset(readDesktopAssetText(t, "js/desktop/apps/pixel.js"))
	for _, want := range []string{"Pixel.listModules().forEach", "runtime.rebuildMenus()", "runtime.mountPanels()", "runtime.initDocks()", "'pointerdown'", "setPointerCapture", "runtime.handleKeydown(e)"} {
		if !strings.Contains(shell, want) {
			t.Fatalf("pixel.js should contain %q", want)
		}
	}
	if strings.Contains(shell, "'mousedown', onCanvasMouseDown") || strings.Contains(shell, "'mousemove', onCanvasMouseMove") {
		t.Fatal("the canvas overlay must use pointer events, not mouse events")
	}
	events := normalizePixelAsset(readDesktopAssetText(t, "js/desktop/apps/pixel-events.js"))
	for _, want := range []string{"handleKeydown: Pixel.bindRuntime(runtime, function handleKeydown(", "e.defaultPrevented", "this.runShortcut(e)", "registerOverlay('cursor'", "pointerInfo: Pixel.bindRuntime("} {
		if !strings.Contains(events, want) {
			t.Fatalf("pixel-events.js should contain %q", want)
		}
	}
	tools := normalizePixelAsset(readDesktopAssetText(t, "js/desktop/apps/pixel-tools.js"))
	for _, want := range []string{"registerOverlay('selection'", "registerOverlay('tool-preview'"} {
		if !strings.Contains(tools, want) {
			t.Fatalf("pixel-tools.js should contain %q", want)
		}
	}
	canvas := normalizePixelAsset(readDesktopAssetText(t, "js/desktop/apps/pixel-canvas.js"))
	if !strings.Contains(canvas, "registerOverlay('compare'") {
		t.Fatal("the compare divider should be an overlay")
	}
	// clearOverlay clears the tool preview only; it must never wipe the shared overlay canvas again.
	clearBody := jsFunctionBodyInWindowMenuTest(t, tools, "clearOverlay: Pixel.bindRuntime(runtime, function clearOverlay()")
	if strings.Contains(clearBody, "clearRect") || !strings.Contains(clearBody, "requestOverlay()") {
		t.Fatalf("clearOverlay should only drop the tool preview and request a repaint: %s", clearBody)
	}
}

// Each later work package gets its own file that is already wired into the loader, so no shared file changes later.
func TestPixelStubModulesRegisterThemselves(t *testing.T) {
	for _, name := range []string{"layers", "selection", "brush", "tools-pro", "transform", "generators", "cutout", "smart", "commands", "ora"} {
		path := "js/desktop/apps/pixel-" + name + ".js"
		if !slicesContains(pixelAppScriptPaths, path) {
			t.Fatalf("%s is not in the pixel script list", path)
		}
		source := readDesktopAssetText(t, path)
		if !strings.Contains(source, "Pixel.registerModule('"+name+"'") {
			t.Fatalf("%s should register itself with Pixel.registerModule('%s', ...)", path, name)
		}
	}
	for _, stylePath := range pixelAppStylePaths {
		if strings.HasPrefix(stylePath, "css/pixel-") && stylePath != "css/pixel-stickers.css" {
			if strings.TrimSpace(readDesktopAssetText(t, stylePath)) == "" {
				t.Fatalf("%s should at least carry a header comment", stylePath)
			}
		}
	}
}

func TestPixelDocksLayoutCSS(t *testing.T) {
	css := normalizePixelAsset(readDesktopAssetText(t, "css/pixel.css"))
	for _, selector := range []string{".pixel-dock-inspector", ".pixel-dock-bottom", ".pixel-dock-body", ".pixel-splitter", ".pixel-param-list", ".pixel-param-segmented", ".pixel-param-switch", ".pixel-param-gradient-bar", ".pixel-param-dial", ".pixel-param-pad"} {
		if !strings.Contains(css, selector) {
			t.Fatalf("pixel.css should contain styles for %q", selector)
		}
	}
	if !pixelCSSRuleContains(css, ".pixel-tabs", "flex-wrap: wrap") {
		t.Fatal("pixel tabs may wrap to a second row in the inspector")
	}
	if !pixelCSSRuleContains(css, ".pixel-overlay", "touch-action: none") {
		t.Fatal("the pointer-event canvas needs touch-action: none so touch and pen strokes are not scrolled away")
	}
	if strings.Contains(css, ".pixel-toolbar .pixel-tabs") {
		t.Fatal("tabs moved out of the toolbar into the docks")
	}
}

func normalizePixelAsset(text string) string {
	return strings.ReplaceAll(text, "\r\n", "\n")
}

// pixelAppScriptPaths is the module loader's script order for Pixel. pixel-worker.js is not in it: runtime.runJob starts it
// as a Worker. The lib files are listed because the main thread uses them as the fallback when no Worker is available.
var pixelAppScriptPaths = []string{
	"js/desktop/shared/sticker-catalog.js",
	"js/desktop/shared/sticker-library.js",
	"js/desktop/apps/pixel-state.js",
	"js/desktop/apps/pixel-lib-core.js",
	"js/desktop/apps/pixel-lib-fx.js",
	"js/desktop/apps/pixel-lib-gen.js",
	"js/desktop/apps/pixel-lib-cutout.js",
	"js/desktop/apps/pixel-lib-inpaint.js",
	"js/desktop/apps/pixel-core.js",
	"js/desktop/apps/pixel-view.js",
	"js/desktop/apps/pixel-canvas.js",
	"js/desktop/apps/pixel-tools.js",
	"js/desktop/apps/pixel-actions.js",
	"js/desktop/apps/pixel-filters.js",
	"js/desktop/apps/pixel-events.js",
	"js/desktop/apps/pixel-stickers.js",
	"js/desktop/apps/pixel-layers.js",
	"js/desktop/apps/pixel-selection.js",
	"js/desktop/apps/pixel-brush.js",
	"js/desktop/apps/pixel-tools-pro.js",
	"js/desktop/apps/pixel-transform.js",
	"js/desktop/apps/pixel-generators.js",
	"js/desktop/apps/pixel-cutout.js",
	"js/desktop/apps/pixel-smart.js",
	"js/desktop/apps/pixel-commands.js",
	"js/desktop/apps/pixel-ora.js",
	"js/desktop/apps/pixel.js",
}

// pixelAppStylePaths is the loader's style order for Pixel.
var pixelAppStylePaths = []string{
	"css/pixel.css",
	"css/desktop-stickers.css",
	"css/pixel-stickers.css",
	"css/pixel-layers.css",
	"css/pixel-filters.css",
	"css/pixel-generators.css",
	"css/pixel-cutout.css",
	"css/pixel-selection.css",
	"css/pixel-tools-pro.css",
	"css/pixel-smart.css",
}

func readPixelAppScripts(t *testing.T) string {
	t.Helper()
	var b strings.Builder
	for _, scriptPath := range pixelAppScriptPaths {
		b.WriteString(readDesktopAssetText(t, scriptPath))
		b.WriteByte('\n')
	}
	return b.String()
}

func pixelCSSRuleContains(css, selector, needle string) bool {
	re := regexp.MustCompile(regexp.QuoteMeta(selector) + `\s*\{[^}]*` + regexp.QuoteMeta(needle))
	return re.MatchString(css)
}

func readThemeIconNames(t *testing.T, manifestPath string) map[string]bool {
	t.Helper()
	var manifest struct {
		Icons map[string]json.RawMessage `json:"icons"`
	}
	if err := json.Unmarshal([]byte(readDesktopAssetText(t, manifestPath)), &manifest); err != nil {
		t.Fatalf("parse %s: %v", manifestPath, err)
	}
	icons := make(map[string]bool, len(manifest.Icons))
	for name := range manifest.Icons {
		icons[name] = true
	}
	return icons
}
