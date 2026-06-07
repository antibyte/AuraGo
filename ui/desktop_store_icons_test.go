package ui

import (
	"strings"
	"testing"
)

func TestDesktopStoreAppsPreferCatalogLogos(t *testing.T) {
	t.Parallel()

	mainText := readDesktopAssetText(t, "js/desktop/main.js")
	for _, want := range []string{
		"function appLogoIconKey(app)",
		"metadata.logo_path",
		"'logo:' + path",
		"vd-app-logo-icon",
		"function themeBackedAppIconKey(app)",
		"return themeBackedAppIconKey(app) || appLogoIconKey(app)",
	} {
		if !strings.Contains(mainText, want) {
			t.Fatalf("desktop app logo rendering missing marker %q", want)
		}
	}

	css := readAllDesktopCSS(t)
	for _, want := range []string{
		".vd-app-logo-icon {",
		"object-fit: contain;",
		".vd-app-logo-icon > [hidden] {",
	} {
		if !strings.Contains(css, want) {
			t.Fatalf("desktop app logo CSS missing marker %q", want)
		}
	}
}

func TestDesktopStoreAppLogosNormalizeSizeAndDisableNativeDrag(t *testing.T) {
	t.Parallel()

	foundation := rawDesktopAssetText(t, "js/desktop/core/desktop-foundation.js")
	for _, want := range []string{
		`draggable="false"`,
		`data-vd-logo-img="true"`,
		`ondragstart="return false"`,
		`if (btn.dataset.desktopEntry !== 'true') event.preventDefault();`,
	} {
		if !strings.Contains(foundation, want) {
			t.Fatalf("desktop store app logo drag safety missing marker %q", want)
		}
	}

	css := rawDesktopAssetText(t, "css/desktop-icons.css")
	logoRule := desktopStoreCSSRuleBody(t, css, ".vd-app-logo-icon")
	imageRule := desktopStoreCSSRuleBody(t, css, ".vd-app-logo-icon > img")
	for _, want := range []string{
		"padding: clamp(1px, 10%, 5px);",
		"pointer-events: none;",
		"-webkit-user-drag: none;",
		"user-select: none;",
	} {
		if !desktopStoreCSSRuleHasDeclaration(logoRule, want) {
			t.Fatalf("desktop store app logo normalization CSS missing marker %q", want)
		}
	}
	if desktopStoreCSSRuleHasDeclaration(logoRule, "overflow: hidden;") {
		t.Fatal("desktop store app logo wrapper must not clip oversized logos")
	}
	for _, want := range []string{
		"width: auto;",
		"height: auto;",
		"max-width: 100%;",
		"max-height: 100%;",
		"object-fit: contain;",
	} {
		if !desktopStoreCSSRuleHasDeclaration(imageRule, want) {
			t.Fatalf("desktop store app logo image scaling CSS missing marker %q", want)
		}
	}
	for _, clipped := range []string{
		"width: 100%;",
		"height: 100%;",
	} {
		if desktopStoreCSSRuleHasDeclaration(imageRule, clipped) {
			t.Fatalf("desktop store app logo image must scale by max size, not force %q", clipped)
		}
	}
}

func TestDesktopShortcutAppLogosStayInsideFixedGlyphBox(t *testing.T) {
	t.Parallel()

	css := rawDesktopAssetText(t, "css/desktop-icons.css")
	iconRule := desktopStoreCSSRuleBody(t, css, ".vd-icon")
	for _, want := range []string{
		"grid-template-rows: var(--vd-icon-glyph-size) minmax(0, auto);",
		"align-content: center;",
	} {
		if !desktopStoreCSSRuleHasDeclaration(iconRule, want) {
			t.Fatalf("desktop icon grid CSS missing fixed glyph box marker %q", want)
		}
	}

	logoRule := desktopStoreCSSRuleBody(t, css, ".vd-icon > .vd-app-logo-icon")
	for _, want := range []string{
		"padding: clamp(3px, 12%, 7px);",
		"overflow: visible;",
	} {
		if !desktopStoreCSSRuleHasDeclaration(logoRule, want) {
			t.Fatalf("desktop shortcut logo frame CSS missing marker %q", want)
		}
	}

	imageRule := desktopStoreCSSRuleBody(t, css, ".vd-icon > .vd-app-logo-icon > img")
	for _, want := range []string{
		"width: 100%;",
		"height: 100%;",
		"min-width: 0;",
		"min-height: 0;",
		"object-fit: contain;",
	} {
		if !desktopStoreCSSRuleHasDeclaration(imageRule, want) {
			t.Fatalf("desktop shortcut logo image CSS missing scaling marker %q", want)
		}
	}
}

func desktopStoreCSSRuleBody(t *testing.T, source, selector string) string {
	t.Helper()

	needle := "\n" + selector + " {"
	start := strings.LastIndex(source, needle)
	if start < 0 {
		t.Fatalf("desktop store icon CSS missing selector %q", selector)
	}
	start++
	open := strings.Index(source[start:], "{")
	if open < 0 {
		t.Fatalf("desktop store icon CSS selector %q is missing opening brace", selector)
	}
	bodyStart := start + open + 1
	close := strings.Index(source[bodyStart:], "}")
	if close < 0 {
		t.Fatalf("desktop store icon CSS selector %q is missing closing brace", selector)
	}
	return source[bodyStart : bodyStart+close]
}

func desktopStoreCSSRuleHasDeclaration(body, declaration string) bool {
	for _, line := range strings.Split(body, "\n") {
		if strings.TrimSpace(line) == declaration {
			return true
		}
	}
	return false
}
