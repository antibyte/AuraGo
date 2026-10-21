package ui

import (
	"regexp"
	"strings"
	"testing"
)

// Source contracts of the Pixel cutout package (WP8a): what the loader, the worker and the server API expect from it.
// The behaviour itself is covered by scripts/test-pixel-cutout.mjs.

func pixelCutoutSources(t *testing.T) (module, lib, css string) {
	t.Helper()
	module = normalizePixelAsset(readDesktopAssetText(t, "js/desktop/apps/pixel-cutout.js"))
	lib = normalizePixelAsset(readDesktopAssetText(t, "js/desktop/apps/pixel-lib-cutout.js"))
	css = normalizePixelAsset(readDesktopAssetText(t, "css/pixel-cutout.css"))
	return module, lib, css
}

func TestPixelCutoutModuleRegistersItsParts(t *testing.T) {
	module, _, _ := pixelCutoutSources(t)
	for _, want := range []string{
		"Pixel.registerModule('cutout'",
		"Pixel.registerPanel({",
		"id: 'cutout'",
		"dock: 'inspector'",
		"Pixel.registerTool({",
		"id: 'cutout-brush'",
		"Pixel.registerMenuItems('ai'",
		"id: 'cutout-remove-bg'",
		"id: 'cutout-to-layer'",
		"computeSubjectMask: Pixel.bindRuntime(r, async function computeSubjectMask(",
		"r.registerOverlay('cutout-view'",
	} {
		if !strings.Contains(module, want) {
			t.Fatalf("pixel-cutout.js should contain %q", want)
		}
	}
	// The brush paints the mask, so a pixel lock must not stop it; it gates itself in onDown.
	tool := module[strings.Index(module, "id: 'cutout-brush'"):]
	tool = tool[:strings.Index(tool, "overlay: (ctx, r)")]
	if !strings.Contains(tool, "raster: false") {
		t.Fatal("the cutout brush works on the mask and must not be a raster tool")
	}
	// "Select subject" in the Select menu belongs to the selection package; the same id here would replace it.
	if strings.Contains(module, "registerMenuItems('select'") {
		t.Fatal("the cutout module must not register Select menu items (it would replace the selection package's Select subject)")
	}
}

func TestPixelCutoutUsesTheServerEndpoints(t *testing.T) {
	module, _, _ := pixelCutoutSources(t)
	for _, want := range []string{
		"'/api/pixel/cutout/status'",
		"'/api/pixel/cutout/prepare'",
		"'/api/pixel/cutout'",
		"source_data",
		"'standard'",
		"'precise'",
	} {
		if !strings.Contains(module, want) {
			t.Fatalf("pixel-cutout.js should contain %q", want)
		}
	}
	// A model download starts on a click, nowhere else.
	if got := strings.Count(module, "'/api/pixel/cutout/prepare'"); got != 1 {
		t.Fatalf("the prepare endpoint should be called from exactly one place, found %d", got)
	}
	body := jsFunctionBodyInWindowMenuTest(t, module, "async function downloadModel(")
	if !strings.Contains(body, "'/api/pixel/cutout/prepare'") {
		t.Fatal("downloadModel should be the caller of the prepare endpoint")
	}
	if got := len(regexp.MustCompile(`\bdownloadModel\(`).FindAllString(module, -1)); got != 2 {
		t.Fatalf("downloadModel should be defined once and called once (from the click dispatcher), found %d mentions", got)
	}
	dispatch := jsFunctionBodyInWindowMenuTest(t, module, "function runAction(")
	if !strings.Contains(dispatch, "case 'download': downloadModel(") {
		t.Fatal("the download must be reachable only through the download button action")
	}
	for _, name := range []string{"async function loadStatus(", "function schedulePoll("} {
		if strings.Contains(jsFunctionBodyInWindowMenuTest(t, module, name), "prepare") {
			t.Fatalf("%s must never start a download", name)
		}
	}
	// the polling of a running download is bounded to the visible panel
	if !strings.Contains(jsFunctionBodyInWindowMenuTest(t, module, "function schedulePoll("), "activePanels.inspector !== 'cutout'") {
		t.Fatal("status polling should only run while the cutout panel is visible")
	}
}

func TestPixelCutoutLibraryOpsAreRegisteredAndUsed(t *testing.T) {
	module, lib, _ := pixelCutoutSources(t)
	for _, op := range []string{"cutout.classic", "cutout.chroma", "cutout.refine", "cutout.decontaminate", "cutout.blur"} {
		if !strings.Contains(lib, "Lib.registerOp('"+op+"'") {
			t.Fatalf("pixel-lib-cutout.js should register %s", op)
		}
		if !strings.Contains(module, "'"+op+"'") {
			t.Fatalf("pixel-cutout.js should run %s", op)
		}
	}
	for _, forbidden := range []string{"document.", "window.", "localStorage", "createElement", "importScripts", "fetch("} {
		if strings.Contains(lib, forbidden) {
			t.Fatalf("pixel-lib-cutout.js runs in a Worker and must stay DOM-free: found %q", forbidden)
		}
	}
	// the GrabCut engine needs a real min-cut, not a threshold in disguise
	for _, want := range []string{"function minCutGrid(", "function grabCut(", "function guidedCoefficients(", "function fitGmm("} {
		if !strings.Contains(lib, want) {
			t.Fatalf("pixel-lib-cutout.js should contain %q", want)
		}
	}
}

func TestPixelCutoutHistoryLabelsAndMaskContract(t *testing.T) {
	module, _, _ := pixelCutoutSources(t)
	for _, label := range []string{"'cutout:remove-bg'", "'cutout:refine'", "'cutout:brush'", "'cutout:to-layer'", "'cutout:apply-mask'", "'cutout:blur-bg'"} {
		if !strings.Contains(module, label) {
			t.Fatalf("pixel-cutout.js should push history with %s", label)
		}
	}
	// pixel writes go through the core, the mask is written next to the layer canvas
	if !strings.Contains(module, "r.commitLayerPixels(layer, baked.canvas") {
		t.Fatal("baking the mask into the pixels should go through commitLayerPixels")
	}
	if !strings.Contains(module, "r.requireRasterLayer(") {
		t.Fatal("cutout actions should respect sticker layers through requireRasterLayer")
	}
	if !strings.Contains(module, "r.markLayerDirty(layer, { mask: true })") {
		t.Fatal("mask changes should bump the mask revision")
	}
}

func TestPixelCutoutStringsCarryFallbacksAndSnakeCaseKeys(t *testing.T) {
	module, _, _ := pixelCutoutSources(t)
	keys := regexp.MustCompile(`pixel\.cutout_[A-Za-z0-9_]*`).FindAllString(module, -1)
	if len(keys) < 40 {
		t.Fatalf("expected the module to use many pixel.cutout_ keys, found %d", len(keys))
	}
	snake := regexp.MustCompile(`^pixel\.cutout_[a-z0-9]+(_[a-z0-9]+)*$`)
	for _, key := range keys {
		if !snake.MatchString(key) {
			t.Fatalf("i18n key %q is not lowercase snake_case", key)
		}
	}
	// every tx(r, key, ...) call and every schema label brings its English text
	for _, call := range regexp.MustCompile(`tx\((?:r|this), '(pixel\.cutout_[a-z0-9_]+)'[^,)]*([,)])`).FindAllStringSubmatch(module, -1) {
		if call[2] != "," {
			t.Fatalf("tx call for %s has no English fallback", call[1])
		}
	}
	for _, label := range regexp.MustCompile(`labelKey: '(pixel\.cutout_[a-z0-9_]+)'(.{0,24})`).FindAllStringSubmatch(module, -1) {
		if !strings.Contains(label[2], "fallback:") {
			t.Fatalf("schema label %s has no fallback", label[1])
		}
	}
	// nothing is localised by hand: no inline English UI strings outside tx()/fallbacks is checked by the key count above,
	// but the module must not fall back to alert/confirm dialogs
	for _, forbidden := range []string{"alert(", "confirm(", "prompt("} {
		if strings.Contains(module, forbidden) {
			t.Fatalf("pixel-cutout.js should use notifications, found %q", forbidden)
		}
	}
}

func TestPixelCutoutStylesArePrefixedAndComplete(t *testing.T) {
	module, _, css := pixelCutoutSources(t)
	for _, selector := range []string{".pixel-cutout {", ".pixel-cutout-group", ".pixel-cutout-model", ".pixel-cutout-bar", ".pixel-cutout-actions", ".pixel-cutout-busy", ".pixel-cutout-refine-body"} {
		if !strings.Contains(css, selector) {
			t.Fatalf("pixel-cutout.css should style %s", selector)
		}
	}
	if strings.Contains(css, ".cc-") || strings.Contains(module, "\"cc-") || strings.Contains(module, " cc-") {
		t.Fatal("class names with the cc- prefix are hidden by cookie-blocker extensions")
	}
	// every class the module puts into markup is a pixel- class
	for _, match := range regexp.MustCompile(`class="([^"$]+)"`).FindAllStringSubmatch(module, -1) {
		for _, name := range strings.Fields(match[1]) {
			if !strings.HasPrefix(name, "pixel-") && name != "active" && !strings.HasPrefix(name, "is-") {
				t.Fatalf("class %q in the cutout markup lacks the pixel- prefix", name)
			}
		}
	}
	if !strings.Contains(css, "prefers-reduced-motion") {
		t.Fatal("the indeterminate progress animation should respect reduced motion")
	}
	for _, external := range []string{"http://", "https://", "@import", "url("} {
		if strings.Contains(css, external) {
			t.Fatalf("pixel-cutout.css must not load anything external (%q)", external)
		}
	}
}
