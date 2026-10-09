package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/desktop"
)

func TestDesktopLocalWikipediaRegistration(t *testing.T) {
	t.Parallel()

	var app *desktop.AppManifest
	for _, candidate := range desktop.BuiltinApps() {
		if candidate.ID == "local-wikipedia" {
			found := candidate
			app = &found
		}
	}
	if app == nil {
		t.Fatal("local-wikipedia is not a builtin app")
	}
	if app.Name != "Wikipedia" || app.Icon != "book" || app.Category != "office" || app.Entry != "builtin://local-wikipedia" || strings.Join(app.Requires, ",") != "local_wikipedia" || !app.StartVisible {
		t.Fatalf("manifest = %+v", *app)
	}
	for _, theme := range []string{"papirus", "whitesur"} {
		var manifest struct {
			Icons map[string]string `json:"icons"`
		}
		if err := json.Unmarshal([]byte(rawDesktopAssetText(t, "img/"+theme+"/manifest.json")), &manifest); err != nil {
			t.Fatal(err)
		}
		path := manifest.Icons["book"]
		if path == "" {
			t.Fatalf("%s manifest has no book icon", theme)
		}
		if _, err := os.Stat(filepath.FromSlash(path)); err != nil {
			t.Fatalf("%s book icon: %v", theme, err)
		}
	}

	loader := rawDesktopAssetText(t, "js/desktop/core/module-loader.js")
	viewsAt := strings.Index(loader, "'/js/desktop/apps/local-wikipedia-views.js'")
	appAt := strings.Index(loader, "'/js/desktop/apps/local-wikipedia.js'")
	if !strings.Contains(loader, "'local-wikipedia': {") || !strings.Contains(loader, "appStyles('/css/desktop-app-local-wikipedia.css')") || viewsAt < 0 || appAt < viewsAt {
		t.Fatal("module loader must load the stylesheet, then the views before the app")
	}
	foundation := rawDesktopAssetText(t, "js/desktop/core/desktop-foundation.js")
	routing := rawDesktopAssetText(t, "js/desktop/core/menus-and-routing.js")
	shell := rawDesktopAssetText(t, "js/desktop/core/window-shell-runtime.js")
	bundle := rawDesktopAssetText(t, "js/desktop/bundles/main.bundle.js")
	for _, check := range []struct{ name, source, marker string }{
		{"foundation", foundation, "'local-wikipedia': 'LocalWikipediaApp'"},
		{"routing", routing, "if (appId === 'local-wikipedia') {"},
		{"routing", routing, "window.LocalWikipediaApp.render(contentEl(id), id,"},
		{"window size", shell, "'local-wikipedia': { width: 1080, height: 760 }"},
		{"bundle", bundle, "'local-wikipedia': 'LocalWikipediaApp'"},
		{"bundle", bundle, "window.LocalWikipediaApp.render(contentEl(id), id,"},
		{"bundle", bundle, "'local-wikipedia': { width: 1080, height: 760 }"},
	} {
		if !strings.Contains(check.source, check.marker) {
			t.Errorf("%s missing %q", check.name, check.marker)
		}
	}
}
