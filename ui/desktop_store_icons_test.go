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
		"function iconForApp(app) { return app ? (appLogoIconKey(app)",
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
		`btn.addEventListener('dragstart', event => event.preventDefault());`,
	} {
		if !strings.Contains(foundation, want) {
			t.Fatalf("desktop store app logo drag safety missing marker %q", want)
		}
	}

	css := readAllDesktopCSS(t)
	for _, want := range []string{
		"padding: clamp(1px, 10%, 5px);",
		"overflow: hidden;",
		"pointer-events: none;",
		"-webkit-user-drag: none;",
		"user-select: none;",
	} {
		if !strings.Contains(css, want) {
			t.Fatalf("desktop store app logo normalization CSS missing marker %q", want)
		}
	}
}
