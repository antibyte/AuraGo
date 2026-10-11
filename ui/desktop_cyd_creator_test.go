package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/desktop"
)

// Static contracts of the CYD Creator shell (plan 4a): registration, translations, modules, loader
// order with the insertion markers of plans 4b and 5, lifecycle and stylesheet.

var cydCreatorLocales = []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"}

func TestCYDCreatorManifestAndIcons(t *testing.T) {
	var app *desktop.AppManifest
	for _, candidate := range desktop.BuiltinApps() {
		if candidate.ID == "cyd-creator" {
			found := candidate
			app = &found
		}
	}
	if app == nil {
		t.Fatal("cyd-creator is not a builtin app")
	}
	if app.Name != "CYD Creator" || app.Icon != "cyd-creator" || app.Category != "dev" || app.Entry != "builtin://cyd-creator" ||
		strings.Join(app.Requires, ",") != "cyd_creator" || app.Metadata["open_maximized"] != "true" || !app.StartVisible || !app.DockVisible {
		t.Fatalf("manifest = %+v", *app)
	}
	catalog := desktop.DesktopIconCatalog(map[string]string{"appearance.icon_theme": "papirus"})
	if !containsString(catalog.Preferred, "cyd-creator") {
		t.Fatal("cyd-creator is not a preferred desktop icon")
	}
	for _, theme := range []string{"papirus", "whitesur"} {
		var manifest struct {
			Icons map[string]string `json:"icons"`
		}
		if err := json.Unmarshal([]byte(rawDesktopAssetText(t, "img/"+theme+"/manifest.json")), &manifest); err != nil {
			t.Fatal(err)
		}
		path := manifest.Icons["cyd-creator"]
		if path != "img/"+theme+"/icons/cyd-creator.svg" {
			t.Fatalf("%s manifest icon path = %q", theme, path)
		}
		svg, err := os.ReadFile(filepath.FromSlash(path))
		if err != nil || !strings.Contains(string(svg), "<svg") {
			t.Fatalf("%s cyd-creator icon: %v", theme, err)
		}
	}
	for _, lang := range cydCreatorLocales {
		var words map[string]string
		if err := json.Unmarshal([]byte(rawDesktopAssetText(t, "lang/desktop/"+lang+".json")), &words); err != nil {
			t.Fatal(err)
		}
		if words["desktop.app_cyd_creator"] != "CYD Creator" {
			t.Errorf("lang/desktop/%s.json: desktop.app_cyd_creator = %q", lang, words["desktop.app_cyd_creator"])
		}
	}
}
