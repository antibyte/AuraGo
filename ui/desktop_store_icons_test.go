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
