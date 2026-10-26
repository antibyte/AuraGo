package ui

import (
	"regexp"
	"strings"
	"testing"
)

// The generators package is wired through registries only: these tests guard the pieces that connect the library, the
// module, the stylesheet and the loader. The behaviour itself is covered by scripts/test-pixel-gen.mjs.

func TestPixelGeneratorLibraryRegistersRenderOpAndCatalog(t *testing.T) {
	lib := normalizePixelAsset(readDesktopAssetText(t, "js/desktop/apps/pixel-lib-gen.js"))
	for _, want := range []string{
		"Lib.registerOp('gen.render', render)",
		"GENERATORS,",
		"GENERATOR_CATEGORIES: CATEGORIES",
		"generatorDefaults: defaultParams",
		"if (typeof Lib.registerOp !== 'function') return;",
	} {
		if !strings.Contains(lib, want) {
			t.Fatalf("pixel-lib-gen.js should contain %q", want)
		}
	}
	if strings.Contains(lib, "Math.random") {
		t.Fatal("generators must be deterministic per seed: Math.random is not allowed in pixel-lib-gen.js")
	}
	defineRe := regexp.MustCompile(`(?m)^    define\('([a-z0-9-]+)', '(noise|texture|pattern|math)',`)
	found := defineRe.FindAllStringSubmatch(lib, -1)
	if len(found) < 40 {
		t.Fatalf("expected the full generator catalog (>= 40 generators), found %d", len(found))
	}
	for _, category := range []string{"noise", "texture", "pattern", "math"} {
		count := 0
		for _, match := range found {
			if match[2] == category {
				count++
			}
		}
		if count < 9 {
			t.Fatalf("category %s should have at least 9 generators, has %d", category, count)
		}
	}
}

func TestPixelGeneratorsModuleRegistersPanelMenuAndCommands(t *testing.T) {
	source := normalizePixelAsset(readDesktopAssetText(t, "js/desktop/apps/pixel-generators.js"))
	for _, want := range []string{
		"Pixel.registerModule('generators'",
		"Pixel.registerPanel({\n        id: 'generate'",
		"dock: 'inspector'",
		"Pixel.registerMenuItems('generate'",
		"Pixel.registerCommand({",
		"id: 'generate:' + def.id",
		"Pixel.GENERATORS = (lib() && lib().GENERATORS) || [];",
		"r.runJob('gen.render'",
		"key: 'gen-preview:'",
		"signal: controller.signal",
		"commitLayerPixels(layer, canvas, { label: 'generate:' + def.id",
		"r.setLayerPreview(layer, st.liveCanvas)",
		"r.buildParamControls(schema",
		"r.wireParamControls(params, schema",
		"openGenerator: Pixel.bindRuntime(r, function openGenerator(",
		"renderGenerator: Pixel.bindRuntime(r, function renderGenerator(",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("pixel-generators.js should contain %q", want)
		}
	}
	// The preview layer must never reach the undo history.
	if !strings.Contains(source, "stripTransientFromHistory") || !strings.Contains(source, "r.on('history-changed'") {
		t.Fatal("the transient preview layer has to be stripped from history entries")
	}
	// Strings go through t(key) with the English text in en.json; the panel markup escapes dynamic text.
	requirePixelBundleKeys(t, "pixel-generators.js", source, 5)
	if strings.Contains(source, "data-action=") {
		t.Fatal("the generators panel uses data-gen-* hooks, not the shared data-action names")
	}
	for _, forbidden := range []string{"http://", "https://", "eval(", "new Function("} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("pixel-generators.js must not contain %q", forbidden)
		}
	}
}

func TestPixelGeneratorsStylesheetIsScopedAndLoaded(t *testing.T) {
	css := normalizePixelAsset(readDesktopAssetText(t, "css/pixel-generators.css"))
	for _, selector := range []string{".pixel-gen-list", ".pixel-gen-card", ".pixel-gen-thumb", ".pixel-gen-preview", ".pixel-gen-detail", ".pixel-gen-actions", ".pixel-gen-progress"} {
		if !strings.Contains(css, selector) {
			t.Fatalf("pixel-generators.css should style %q", selector)
		}
	}
	if !pixelCSSRuleContains(css, ".pixel-gen-detail[hidden]", "display: none") && !strings.Contains(css, ".pixel-gen [hidden]") {
		t.Fatal("flex containers of the generators panel need an explicit [hidden] override")
	}
	classRe := regexp.MustCompile(`\.([a-zA-Z][a-zA-Z0-9_-]*)`)
	selectors := regexp.MustCompile(`(?s)/\*.*?\*/`).ReplaceAllString(css, "")
	selectors = regexp.MustCompile(`(?s)\{[^}]*\}`).ReplaceAllString(selectors, "{}")
	for _, match := range classRe.FindAllStringSubmatch(selectors, -1) {
		name := match[1]
		if strings.HasPrefix(name, "cc-") {
			t.Fatalf("class %q uses the cookie-blocker prone cc- prefix", name)
		}
		if !strings.HasPrefix(name, "pixel-") {
			t.Fatalf("class %q in pixel-generators.css must carry the pixel- prefix", name)
		}
	}
	if !slicesContains(pixelAppStylePaths, "css/pixel-generators.css") || !slicesContains(pixelAppScriptPaths, "js/desktop/apps/pixel-generators.js") || !slicesContains(pixelAppScriptPaths, "js/desktop/apps/pixel-lib-gen.js") {
		t.Fatal("the generators files must be in the pixel loader lists")
	}
	worker := normalizePixelAsset(readDesktopAssetText(t, "js/desktop/apps/pixel-worker.js"))
	if !strings.Contains(worker, "'pixel-lib-gen.js'") {
		t.Fatal("the worker has to import pixel-lib-gen.js")
	}
}
