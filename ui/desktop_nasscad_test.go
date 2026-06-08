package ui

import (
	"os"
	"path/filepath"
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
		"Apps/nasscad/index.html",
		"window.NasscadApp = { render, dispose }",
		"vd-nasscad-frame vd-generated-frame",
		"desktopEmbedURL",
		"makeSandboxedFrame",
		"allowSameOrigin: true",
		"desktop.nasscad_load_failed",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("nasscad app missing marker %q", want)
		}
	}
	for _, forbidden := range []string{
		"nasscad.com",
		"https://",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("nasscad app must not load remote URL, found %q", forbidden)
		}
	}

	loader := readEmbeddedText(t, "js/desktop/core/module-loader.js")
	if !strings.Contains(loader, "'nasscad'") || !strings.Contains(loader, "/js/desktop/apps/nasscad.js") {
		t.Fatal("module loader must register nasscad lazy assets")
	}

	routing := readDesktopAssetText(t, "js/desktop/core/menus-and-routing.js")
	for _, want := range []string{
		"appId === 'nasscad'",
		"window.NasscadApp",
		"desktopEmbedURL",
		"makeSandboxedFrame",
	} {
		if !strings.Contains(routing, want) {
			t.Fatalf("desktop routing missing nasscad marker %q", want)
		}
	}

	bundledPath := filepath.Join("..", "internal", "desktop", "bundled_apps", "nasscad", "index.html")
	bundled, err := os.ReadFile(bundledPath)
	if err != nil {
		t.Fatalf("read bundled nasscad html: %v", err)
	}
	if len(bundled) < 100000 {
		t.Fatalf("bundled nasscad html looks too small: %d bytes", len(bundled))
	}
}