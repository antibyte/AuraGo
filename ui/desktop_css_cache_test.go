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
	if !strings.Contains(html, `/css/desktop.css?v={{.BuildVersion}}-desktop-20260509b`) {
		t.Fatalf("desktop.html must bust the desktop.css aggregator cache with the current desktop asset version")
	}
}
