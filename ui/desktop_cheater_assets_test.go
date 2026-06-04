package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesktopCheaterAppRegistration(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/cheater.js")
	for _, marker := range []string{
		"window.CheaterApp",
		"window.CheaterApp.render = render",
		"window.CheaterApp.dispose = dispose",
		"data-cheater",
		"cheater-empty",
		"data-empty",
		"cheater-empty-title",
		"data-action=\"create\"",
		"Cmd/Ctrl + K",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("cheater app missing JS marker %q", marker)
		}
	}
}

func TestDesktopCheaterModuleLoaderRegistration(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/core/module-loader.js")
	if !strings.Contains(source, "'cheater':") {
		t.Fatalf("module-loader missing cheater registration")
	}
	if !strings.Contains(source, "/css/desktop-app-cheater.css") {
		t.Fatalf("module-loader missing cheater styles")
	}
	if !strings.Contains(source, "/js/desktop/apps/cheater.js") {
		t.Fatalf("module-loader missing cheater script")
	}
}

func TestDesktopCheaterRouting(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/core/menus-and-routing.js")
	if !strings.Contains(source, "appId === 'cheater'") {
		t.Fatalf("menus-and-routing missing cheater dispatch")
	}
}

func TestDesktopCheaterIcon(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/core/desktop-foundation.js")
	if !strings.Contains(source, "cheater: 'cheater'") {
		t.Fatalf("desktop-foundation missing cheater icon mapping")
	}
}

func TestDesktopCheaterIconAsset(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile(filepath.Join("img", "chat-ui-icons", "cheater.svg"))
	if err != nil {
		t.Fatalf("read cheater icon: %v", err)
	}
	if !strings.Contains(string(data), "<svg") {
		t.Fatalf("cheater icon is not a valid svg")
	}
}

func TestDesktopCheaterEmptyStateStyles(t *testing.T) {
	t.Parallel()

	css := readAllDesktopAppCSS(t)
	for _, marker := range []string{
		".cheater-app",
		".cheater-empty",
		".cheater-empty-icon",
		".cheater-empty-title",
		".cheater-primary",
		"prefers-reduced-motion",
	} {
		if !strings.Contains(css, marker) {
			t.Fatalf("cheater CSS missing marker %q", marker)
		}
	}
}
