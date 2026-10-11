package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/desktop"
)

func TestDesktopRetroGamesRegistration(t *testing.T) {
	var app *desktop.AppManifest
	for _, candidate := range desktop.BuiltinApps() {
		if candidate.ID == "retro-games" {
			found := candidate
			app = &found
		}
	}
	if app == nil || app.Icon != "arcade" || app.Category != "games" || app.Entry != "builtin://retro-games" || strings.Join(app.Requires, ",") != "retro_games" {
		t.Fatalf("manifest = %+v", app)
	}
	for _, theme := range []string{"papirus", "whitesur"} {
		data, err := os.ReadFile(filepath.Join("img", theme, "manifest.json"))
		if err != nil {
			t.Fatal(err)
		}
		var manifest struct {
			Icons   map[string]string `json:"icons"`
			Aliases map[string]string `json:"aliases"`
		}
		if err := json.Unmarshal(data, &manifest); err != nil {
			t.Fatal(err)
		}
		target := manifest.Aliases["arcade"]
		iconPath := manifest.Icons[target]
		if target == "" || iconPath == "" {
			t.Fatalf("%s: arcade icon does not resolve (alias %q)", theme, target)
		}
		if _, err := os.Stat(filepath.FromSlash(iconPath)); err != nil {
			t.Fatalf("%s: %v", theme, err)
		}
	}

	loader := rawDesktopAssetText(t, "js/desktop/core/module-loader.js")
	viewsAt := strings.Index(loader, "'/js/desktop/apps/retro-games-views.js'")
	playerAt := strings.Index(loader, "'/js/desktop/apps/retro-games-player.js'")
	appAt := strings.Index(loader, "'/js/desktop/apps/retro-games.js'")
	if !strings.Contains(loader, "'retro-games': {") || !strings.Contains(loader, "appStyles('/css/desktop-app-retro-games.css')") || viewsAt < 0 || playerAt < viewsAt || appAt < playerAt {
		t.Fatal("module loader does not register the Retro Games assets in order")
	}

	foundation := rawDesktopAssetText(t, "js/desktop/core/desktop-foundation.js")
	routing := rawDesktopAssetText(t, "js/desktop/core/menus-and-routing.js")
	shell := rawDesktopAssetText(t, "js/desktop/core/window-shell-runtime.js")
	events := rawDesktopAssetText(t, "js/desktop/core/sdk-events-bootstrap.js")
	bundle := rawDesktopAssetText(t, "js/desktop/bundles/main.bundle.js")
	markers := []struct{ name, source, marker string }{
		{"foundation", foundation, "'retro-games': 'RetroGamesApp'"},
		{"routing", routing, "if (appId === 'retro-games') {"},
		{"routing", routing, "window.RetroGamesApp.render(contentEl(id), id, Object.assign({}, context || {}, { esc, api, t, iconMarkup, openApp, showContextMenu }))"},
		{"size", shell, "'retro-games': { width: 1100, height: 720 }"},
		{"min size", shell, "'retro-games': { width: 640, height: 480 }"},
		{"mobile", shell, "'retro-games': true"},
		{"existing window", shell, "window.RetroGamesApp.open(existing.id, context)"},
		{"events", events, "launch.game = event.payload.game"},
	}
	// Every marker lives in a core file that main.bundle.js includes.
	for _, m := range markers {
		if !strings.Contains(m.source, m.marker) {
			t.Errorf("%s lacks %q", m.name, m.marker)
		}
		if !strings.Contains(bundle, m.marker) {
			t.Errorf("bundle lacks %s marker %q (run npm run build:ui)", m.name, m.marker)
		}
	}
}
