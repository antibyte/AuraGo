package ui

import (
	"regexp"
	"strings"
	"testing"
)

func TestDesktopCSSImportsBustComponentCache(t *testing.T) {
	t.Parallel()

	css := readDesktopAssetText(t, "css/desktop.css")
	importRE := regexp.MustCompile(`@import\s+url\('([^']+)'\);`)
	matches := importRE.FindAllStringSubmatch(css, -1)
	if len(matches) == 0 {
		t.Fatal("desktop.css must import split desktop component stylesheets")
	}
	for _, match := range matches {
		path := match[1]
		if strings.HasPrefix(path, "desktop-") && !strings.Contains(path, "?v=") {
			t.Fatalf("desktop.css imports %q without component cache busting", path)
		}
	}
}

func TestDesktopHTMLBustsDesktopCSSAggregatorCache(t *testing.T) {
	t.Parallel()

	html := readDesktopAssetText(t, "desktop.html")
	if !strings.Contains(html, `/css/desktop.css?v={{.BuildVersion}}-desktop-20260509m`) {
		t.Fatalf("desktop.html must bust the desktop.css aggregator cache with the current desktop asset version")
	}
}

func TestDesktopCSSCarriesFinalFruityWindowControlOverride(t *testing.T) {
	t.Parallel()

	css := readDesktopAssetText(t, "css/desktop.css")
	for _, want := range []string{
		".desktop-body[data-theme=\"fruity\"] .vd-window > .vd-window-titlebar > .vd-window-actions",
		".desktop-body[data-theme=\"fruity\"] .vd-window.has-window-menu > .vd-window-titlebar",
		"grid-template-columns: 78px minmax(0, 1fr) 78px !important;",
		"padding: 0 14px !important;",
		".desktop-body[data-theme=\"fruity\"] .vd-window.has-window-menu > .vd-window-titlebar > .vd-window-title-group",
		"grid-column: 2 !important;",
		"justify-content: center !important;",
		".desktop-body[data-theme=\"fruity\"] .vd-window.has-window-menu > .vd-window-titlebar > .vd-window-actions",
		"left: 14px !important;",
		"right: auto !important;",
		"justify-self: start !important;",
		"grid-column: 1 !important;",
		"position: static !important;",
		"transform: none !important;",
	} {
		if !strings.Contains(css, want) {
			t.Fatalf("desktop.css missing final Fruity window control override marker %q", want)
		}
	}
}
