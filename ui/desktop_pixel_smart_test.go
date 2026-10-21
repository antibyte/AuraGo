package ui

import (
	"regexp"
	"strings"
	"testing"
)

const (
	pixelSmartScript    = "js/desktop/apps/pixel-smart.js"
	pixelCommandsScript = "js/desktop/apps/pixel-commands.js"
	pixelInpaintLib     = "js/desktop/apps/pixel-lib-inpaint.js"
	pixelSmartStyles    = "css/pixel-smart.css"
)

// The inpainting lib runs in the Pixel worker: it registers the op other packages call by name and stays DOM-free.
func TestPixelSmartInpaintLibRegistersTheFillOp(t *testing.T) {
	lib := normalizePixelAsset(readDesktopAssetText(t, pixelInpaintLib))
	for _, want := range []string{
		"Lib.registerOp('inpaint.fill'",
		"function inpaint(",
		"function inpaintRegion(",
		"function extractPalette(",
		"function colorHarmonies(",
		"function contentBounds(",
		"function estimateBackground(",
		"no-sources",
	} {
		if !strings.Contains(lib, want) {
			t.Fatalf("%s should contain %q", pixelInpaintLib, want)
		}
	}
	for _, forbidden := range []string{"document.", "window.", "localStorage", "createElement", "importScripts"} {
		if strings.Contains(lib, forbidden) {
			t.Fatalf("%s must stay DOM-free, found %q", pixelInpaintLib, forbidden)
		}
	}
}

// Content-aware fill, the remove-object tool, the colour panel and the document actions are all registry calls.
func TestPixelSmartModuleRegistersItsFeatures(t *testing.T) {
	smart := normalizePixelAsset(readDesktopAssetText(t, pixelSmartScript))
	for _, want := range []string{
		"Pixel.registerModule('smart'",
		"Pixel.registerTool({\n        id: 'remove-object'",
		"group: 'retouch'",
		"Pixel.registerPanel({\n        id: 'colors'",
		"contentAwareFill: Pixel.bindRuntime(r, contentAwareFill)",
		"showContentAwareFillDialog",
		"r.runJob('inpaint.fill'",
		"r.commitLayerPixels(layer, out",
		"transfer: [image.data.buffer, mask.buffer]",
		"{ id: 'content-aware-fill'",
		"shortcut: 'Shift+Backspace'",
		"{ id: 'trim'",
		"{ id: 'auto-crop'",
		"{ id: 'canvas-size'",
		"{ id: 'fit-canvas-layers'",
		"r.transformDocument([1, 0, 0, 1,",
		"pixel.swatches",
		"extractPalette(",
		"colorHarmonies(",
		"'colors-changed'",
	} {
		if !strings.Contains(smart, want) {
			t.Fatalf("%s should contain %q", pixelSmartScript, want)
		}
	}
	// The colour panel keeps the hooks of the built-in one: setPrimaryColor writes the hex field, the recent row is rendered by the core.
	panelStart := strings.Index(smart, "Pixel.registerPanel({\n        id: 'colors',")
	if panelStart < 0 {
		t.Fatal("the colors panel override is missing")
	}
	panelEnd := strings.Index(smart[panelStart:], "\n    });")
	if panelEnd < 0 {
		t.Fatal("the colors panel registration does not end")
	}
	panel := smart[panelStart : panelStart+panelEnd]
	for _, forbidden := range []string{"order:", "dock:", "labelKey:"} {
		if strings.Contains(panel, forbidden) {
			t.Fatalf("the colors override should inherit %s from the built-in panel", forbidden)
		}
	}
	for _, hook := range []string{"data-hex-input", "data-recent-colors", "data-palette-color"} {
		if !strings.Contains(smart, hook) {
			t.Fatalf("the colour panel markup should keep %s", hook)
		}
	}
	for _, forbidden := range []string{"icon: 'crop'", "icon: 'resize'", "icon: 'rotate-cw'"} {
		if strings.Contains(smart, forbidden) {
			t.Fatalf("%s uses a theme icon that does not exist: %s", pixelSmartScript, forbidden)
		}
	}
}

func TestPixelCommandPaletteWiring(t *testing.T) {
	commands := normalizePixelAsset(readDesktopAssetText(t, pixelCommandsScript))
	for _, want := range []string{
		"Pixel.registerModule('commands'",
		"shortcut: 'Ctrl+K'",
		"shortcuts: ['Ctrl+Shift+P']",
		"'pixel.recent_commands'",
		"const RECENT_MAX = 20",
		"r.listCommands()",
		"function fuzzyMatch(",
		"function fuzzyScore(",
		"Object.assign(Pixel, { fuzzyMatch, fuzzyScore, searchCommands, synonymsFor, contextSuggestions })",
		"role=\"listbox\"",
		"aria-activedescendant",
		"'ArrowDown'",
		"'Enter'",
		"'Escape'",
		"menu:edit:content-aware-fill",
		"menu:image:trim",
	} {
		if !strings.Contains(commands, want) {
			t.Fatalf("%s should contain %q", pixelCommandsScript, want)
		}
	}
	// German synonyms for the examples of the brief.
	for _, word := range []string{"freistellen", "rauschen", "weichzeichnen", "ebene"} {
		if !strings.Contains(commands, "'"+word+"'") {
			t.Fatalf("the synonym table should know %q", word)
		}
	}
}

// Every [data-...] selector the two scripts use needs markup with that attribute somewhere in the Pixel scripts.
func TestPixelSmartSelectorsHaveMarkup(t *testing.T) {
	scripts := readPixelAppScripts(t)
	selector := regexp.MustCompile(`\[(data-(?:sc|pal|cs|caf|ro|sb)-[a-z-]+)\]`)
	for _, name := range []string{pixelSmartScript, pixelCommandsScript} {
		source := normalizePixelAsset(readDesktopAssetText(t, name))
		seen := map[string]bool{}
		for _, match := range selector.FindAllStringSubmatch(source, -1) {
			attr := match[1]
			if seen[attr] {
				continue
			}
			seen[attr] = true
			markup := regexp.MustCompile(`(?:^|[^\[\w-])` + regexp.QuoteMeta(attr) + `(?:=|\s|>|\$|')`)
			if !markup.MatchString(scripts) {
				t.Fatalf("%s selects [%s] but no markup carries it", name, attr)
			}
		}
		if len(seen) == 0 {
			t.Fatalf("%s should select its own markup through data attributes", name)
		}
	}
}

// Strings go through t(key, fallback) / labelKey + fallback, never bare text.
func TestPixelSmartStringsHaveEnglishFallbacks(t *testing.T) {
	for _, name := range []string{pixelSmartScript, pixelCommandsScript} {
		source := normalizePixelAsset(readDesktopAssetText(t, name))
		label := regexp.MustCompile(`labelKey: 'pixel\.[a-z0-9_]+'`)
		for _, loc := range label.FindAllStringIndex(source, -1) {
			end := loc[1] + 90
			if end > len(source) {
				end = len(source)
			}
			if !strings.Contains(source[loc[1]:end], "fallback:") {
				t.Fatalf("%s: %s has no fallback next to it", name, source[loc[0]:loc[1]])
			}
		}
		bare := regexp.MustCompile(`\bt\('pixel\.[a-z0-9_]+'\)`)
		if bare.MatchString(source) {
			t.Fatalf("%s: every t('pixel.…') call needs an English fallback", name)
		}
	}
}

func TestPixelSmartStylesArePrefixedAndComplete(t *testing.T) {
	css := normalizePixelAsset(readDesktopAssetText(t, pixelSmartStyles))
	for _, selector := range []string{
		".pixel-palette-backdrop", ".pixel-palette", ".pixel-palette-input", ".pixel-palette-item.is-active", ".pixel-palette-item.is-disabled",
		".pixel-smart-busy", ".pixel-smart-anchor", ".pixel-sc-sv", ".pixel-sc-hue", ".pixel-sc-harmonies", ".pixel-sc-grid",
	} {
		if !strings.Contains(css, selector) {
			t.Fatalf("%s should style %s", pixelSmartStyles, selector)
		}
	}
	class := regexp.MustCompile(`\.([A-Za-z_][\w-]*)`)
	// Strip declaration blocks, comments and numbers (0.38) so only selectors are left.
	selectors := regexp.MustCompile(`(?s)/\*.*?\*/|\{[^{}]*\}`).ReplaceAllString(css, "{}")
	for _, match := range class.FindAllStringSubmatch(selectors, -1) {
		name := match[1]
		if !strings.HasPrefix(name, "pixel-") && name != "is-active" && name != "is-disabled" && name != "is-denied" && name != "active" {
			t.Fatalf("%s: class .%s must be prefixed with pixel- (cookie blockers hide cc- names)", pixelSmartStyles, name)
		}
	}
	if strings.Contains(css, ".cc-") {
		t.Fatalf("%s must not use cc- class names", pixelSmartStyles)
	}
	// The palette sits inside the window: .pixel-app is the positioning context.
	if !pixelCSSRuleContains(css, ".pixel-app", "position: relative") {
		t.Fatalf("%s should make .pixel-app the positioning context for the palette", pixelSmartStyles)
	}
	if !pixelCSSRuleContains(css, ".pixel-palette-backdrop", "position: absolute") {
		t.Fatalf("the command palette must cover the Pixel window, not the page")
	}
}

func TestPixelSmartMenuIconsExistInThemes(t *testing.T) {
	for _, manifestPath := range []string{"img/papirus/manifest.json", "img/whitesur/manifest.json"} {
		icons := readThemeIconNames(t, manifestPath)
		for _, icon := range []string{"image", "scissors", "maximize", "search"} {
			if !icons[icon] {
				t.Fatalf("%s does not provide icon %q used by the Pixel smart menu items", manifestPath, icon)
			}
		}
	}
	for _, name := range []string{pixelSmartScript, pixelCommandsScript} {
		source := readDesktopAssetText(t, name)
		for _, match := range regexp.MustCompile(`icon: '([a-z-]+)'`).FindAllStringSubmatch(source, -1) {
			switch match[1] {
			case "image", "scissors", "maximize", "search":
			default:
				t.Fatalf("%s uses menu icon %q that is not checked against the theme manifests", name, match[1])
			}
		}
	}
}
