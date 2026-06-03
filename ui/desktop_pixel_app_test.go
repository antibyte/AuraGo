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
		"new-image": "image",
		"copy":      "image",
		"cut":       "scissors",
		"paste":     "image",
		"select-all": "image",
		"deselect":  "image",
		"shortcuts": "image",
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
	js := normalizePixelAsset(readDesktopAssetText(t, "js/desktop/apps/pixel-canvas.js"))
	applyBody := jsFunctionBodyInWindowMenuTest(t, js, "applyAdjustments: Pixel.bindRuntime(runtime, function applyAdjustments()")

	if strings.Contains(applyBody, "resetAdjustments();") {
		t.Fatalf("Pixel applyAdjustments must not call resetAdjustments because it redraws the old original image: %s", applyBody)
	}
	if !strings.Contains(applyBody, "resetAdjustmentControls();") {
		t.Fatalf("Pixel applyAdjustments should reset slider controls without repainting the canvas: %s", applyBody)
	}
	if !strings.Contains(applyBody, "originalImage = img;") {
		t.Fatalf("Pixel applyAdjustments should commit the adjusted canvas as the new original image: %s", applyBody)
	}
}

func TestPixelCompareModeClearsWhenLeavingAdjustPanel(t *testing.T) {
	tools := normalizePixelAsset(readDesktopAssetText(t, "js/desktop/apps/pixel-tools.js"))
	canvas := normalizePixelAsset(readDesktopAssetText(t, "js/desktop/apps/pixel-canvas.js"))
	showPanelBody := jsFunctionBodyInWindowMenuTest(t, tools, "showPanel: Pixel.bindRuntime(runtime, function showPanel(name)")

	if !strings.Contains(canvas, "exitCompareMode: Pixel.bindRuntime(runtime, function exitCompareMode(") {
		t.Fatalf("Pixel needs a shared exitCompareMode helper so panel changes can leave compare mode cleanly")
	}
	if !strings.Contains(showPanelBody, "if (name !== 'adjust' && compareMode) exitCompareMode({ preservePreview: true });") {
		t.Fatalf("Pixel showPanel should disable compare mode when leaving adjustments so drawing is not blocked: %s", showPanelBody)
	}
}

func TestPixelAddLayerPreservesCurrentCanvasAsBackground(t *testing.T) {
	canvas := normalizePixelAsset(readDesktopAssetText(t, "js/desktop/apps/pixel-canvas.js"))
	tools := normalizePixelAsset(readDesktopAssetText(t, "js/desktop/apps/pixel-tools.js"))
	migrateBody := jsFunctionBodyInWindowMenuTest(t, canvas, "ensureBackgroundMigrated: Pixel.bindRuntime(runtime, function ensureBackgroundMigrated()")
	addLayerBody := jsFunctionBodyInWindowMenuTest(t, tools, "addLayer: Pixel.bindRuntime(runtime, function addLayer()")

	if strings.Contains(migrateBody, "layers.length <= 1") {
		t.Fatalf("ensureBackgroundMigrated must also copy the single-layer canvas before a new layer is added: %s", migrateBody)
	}
	if !strings.Contains(migrateBody, "bgCanvas.getContext('2d').drawImage(canvas, 0, 0);") {
		t.Fatalf("ensureBackgroundMigrated should preserve the current visible canvas as the background layer: %s", migrateBody)
	}
	if strings.Index(addLayerBody, "ensureBackgroundMigrated();") < 0 || strings.Index(addLayerBody, "ensureBackgroundMigrated();") > strings.Index(addLayerBody, "layers.push(") {
		t.Fatalf("addLayer should migrate the background before inserting a blank layer: %s", addLayerBody)
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

func normalizePixelAsset(text string) string {
	return strings.ReplaceAll(text, "\r\n", "\n")
}

var pixelAppScriptPaths = []string{
	"js/desktop/apps/pixel-state.js",
	"js/desktop/apps/pixel-view.js",
	"js/desktop/apps/pixel-canvas.js",
	"js/desktop/apps/pixel-tools.js",
	"js/desktop/apps/pixel-actions.js",
	"js/desktop/apps/pixel-events.js",
	"js/desktop/apps/pixel.js",
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
