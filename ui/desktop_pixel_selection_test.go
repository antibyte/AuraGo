package ui

import (
	"regexp"
	"strings"
	"testing"
)

const pixelSelectionScript = "js/desktop/apps/pixel-selection.js"

func TestPixelSelectionRegistersToolsMenuAndShortcuts(t *testing.T) {
	src := normalizePixelAsset(readDesktopAssetText(t, pixelSelectionScript))

	for _, want := range []string{
		"Pixel.registerModule('selection', function install(r)",
		// the four built-in selection tools are overridden by id, two new ones are added
		"Pixel.registerTool(shapeTool('select-rect'))",
		"Pixel.registerTool(shapeTool('select-ellipse'))",
		"Pixel.registerTool(lassoTool())",
		"Pixel.registerTool(wandTool())",
		"id: 'polygon-lasso'",
		"Pixel.registerTool(quickSelectTool())",
		"id: 'quick-select'",
		// the select menu and the commands that come from its items
		"Pixel.registerMenuItems('select'",
		"id: 'color-range'",
		"id: 'modify'",
		"id: 'select-subject'",
		"id: 'from-layer-alpha'",
		"id: 'to-layer-mask'",
		"id: 'stroke'",
		"shortcut: 'Ctrl+Shift+D'",
		"shortcut: 'Ctrl+Shift+I'",
		"shortcut: 'Shift+F6'",
		"shortcut: 'Ctrl+Shift+J'",
		"shortcutHint: 'Ctrl+A'",
		"shortcutHint: 'Ctrl+D'",
		"shortcut: 'Alt+Backspace'",
		"shortcut: 'Ctrl+Backspace'",
		// the Edit items keep Ctrl+A / Ctrl+D (the shell runs them first) and now leave a history step
		"Pixel.registerMenuItems('edit'",
		"shortcut: 'Ctrl+A'",
		"shortcut: 'Ctrl+D'",
		"id: 'edit.cancel'",
		// the marching ants overlay is replaced, the colour range preview is an overlay too
		"r.registerOverlay('selection'",
		"r.registerOverlay('color-range'",
		// worker ops of the core library are used for feather and smooth
		"'core.blurAlpha'",
		"'core.morph'",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("%s should contain %q", pixelSelectionScript, want)
		}
	}
}

func TestPixelSelectionOptionalPackagesAreFeatureDetected(t *testing.T) {
	src := normalizePixelAsset(readDesktopAssetText(t, pixelSelectionScript))
	for _, want := range []string{
		"typeof r.computeSubjectMask !== 'function'",
		"typeof r.contentAwareFill !== 'function'",
		"r.listCommands().find(c => c.shortcuts.indexOf('ctrl+j')",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("%s must detect optional runtime features (%q)", pixelSelectionScript, want)
		}
	}
	// Ctrl+J belongs to the layers package; the selection menu must not bind it.
	if regexp.MustCompile(`(?i)(shortcut|shortcutHint):\s*'Ctrl\+J'`).MatchString(src) {
		t.Fatalf("%s must not bind Ctrl+J: the layers package owns it", pixelSelectionScript)
	}
}

func TestPixelSelectionPureKitIsDOMFree(t *testing.T) {
	src := normalizePixelAsset(readDesktopAssetText(t, pixelSelectionScript))
	start := strings.Index(src, "const kit = Pixel.selectionKit = {};")
	end := strings.Index(src, "// Window state and shared helpers")
	if start < 0 || end < 0 || end < start {
		t.Fatal("pixel-selection.js should keep the pure kit in front of the window helpers")
	}
	for _, forbidden := range []string{"document.", "window.", "createElement", "getContext", "localStorage", "requestAnimationFrame"} {
		if strings.Contains(src[start:end], forbidden) {
			t.Fatalf("the pure selection kit must stay DOM-free (found %q)", forbidden)
		}
	}
	if !strings.Contains(src, "Object.assign(kit, {") {
		t.Fatal("the kit functions should be exported for scripts/test-pixel-selection.mjs")
	}
}

func TestPixelSelectionStringsUseBundleKeysAndPrefixedClasses(t *testing.T) {
	src := normalizePixelAsset(readDesktopAssetText(t, pixelSelectionScript))
	css := normalizePixelAsset(readDesktopAssetText(t, "css/pixel-selection.css"))

	// Every key goes through t('pixel.key') or t('pixel.key', vars); the English text lives in en.json.
	requirePixelBundleKeys(t, pixelSelectionScript, src, 20)
	for _, text := range []string{src, css} {
		if regexp.MustCompile(`[.\s"'](cc-)[a-z]`).MatchString(text) {
			t.Fatal("cookie-blocker extensions hide cc- classes: use the pixel- prefix")
		}
		for _, forbidden := range []string{"http://", "https://", "eval(", "new Function("} {
			if strings.Contains(text, forbidden) {
				t.Fatalf("pixel selection assets must not load or run anything external (%q)", forbidden)
			}
		}
	}
	for _, selector := range []string{".pixel-sel-group", ".pixel-sel-cr", ".pixel-sel-swatch", ".pixel-sel-seg", ".pixel-sel-dialog"} {
		if !strings.Contains(css, selector) {
			t.Fatalf("pixel-selection.css should style %q", selector)
		}
	}
	// every pixel-sel-* class used by the markup has a rule or is a pure hook
	if !pixelCSSRuleContains(css, ".pixel-workspace", "position: relative") {
		t.Fatal("the floating colour range panel needs the workspace as its positioning context")
	}
	if !pixelCSSRuleContains(css, ".pixel-sel-cr", "position: absolute") {
		t.Fatal("the colour range panel floats over the canvas area")
	}
}
