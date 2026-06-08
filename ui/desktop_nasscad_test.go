package ui

import (
	"strings"
	"testing"
)

func TestNasscadDesktopAppAssets(t *testing.T) {
	t.Parallel()

	for _, path := range []string{
		"js/desktop/apps/nasscad.js",
		"css/desktop-app-nasscad.css",
		"img/papirus/icons/nasscad.svg",
		"img/whitesur/icons/nasscad.svg",
	} {
		rawDesktopAssetText(t, path)
	}

	source := readDesktopAssetText(t, "js/desktop/apps/nasscad.js")
	for _, want := range []string{
		"https://www.nasscad.com/NASSCAD_V4_2_7.htm",
		"window.NasscadApp = { render, dispose }",
		"vd-nasscad-frame",
		"allowfullscreen",
		"desktop.nasscad_load_failed",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("nasscad app missing marker %q", want)
		}
	}

	loader := readEmbeddedText(t, "js/desktop/core/module-loader.js")
	if !strings.Contains(loader, "'nasscad'") || !strings.Contains(loader, "/js/desktop/apps/nasscad.js") {
		t.Fatal("module loader must register nasscad lazy assets")
	}

	routing := readDesktopAssetText(t, "js/desktop/core/menus-and-routing.js")
	if !strings.Contains(routing, "appId === 'nasscad'") || !strings.Contains(routing, "window.NasscadApp") {
		t.Fatal("desktop routing must render NasscadApp")
	}
}