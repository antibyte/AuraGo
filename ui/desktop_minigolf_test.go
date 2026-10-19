package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/desktop"
)

func TestDesktopMinigolfRegistrationAndLocalResources(t *testing.T) {
	var found bool
	for _, app := range desktop.BuiltinApps() {
		if app.ID != "minigolf" {
			continue
		}
		found = true
		if app.Entry != "builtin://minigolf" || app.Category != "games" || app.Metadata["open_maximized"] != "true" || len(app.Requires) != 0 {
			t.Fatalf("unexpected minigolf registration: %+v", app)
		}
	}
	if !found {
		t.Fatal("AuraGolf is missing from the Desktop")
	}
	for file, markers := range map[string][]string{
		"js/desktop/core/module-loader.js":      {"'minigolf': ['minigolf']", "'/css/desktop-app-minigolf.css'", "'/js/desktop/apps/minigolf.js'"},
		"js/desktop/core/menus-and-routing.js":  {"appId === 'minigolf'", "window.MinigolfApp.render"},
		"js/desktop/core/desktop-foundation.js": {"'minigolf': 'MinigolfApp'"},
	} {
		data := readDesktopAssetText(t, file)
		for _, marker := range markers {
			if !strings.Contains(data, marker) {
				t.Fatalf("%s missing %s", file, marker)
			}
		}
	}
	loader := readDesktopAssetText(t, "js/desktop/core/module-loader.js")
	entryStart := strings.Index(loader, "'minigolf': {")
	if entryStart < 0 {
		t.Fatal("module-loader.js has no minigolf entry")
	}
	entryLength := strings.Index(loader[entryStart:], "},")
	if entryLength < 0 {
		t.Fatal("module-loader.js minigolf entry is not closed")
	}
	if entry := loader[entryStart : entryStart+entryLength]; strings.Contains(entry, "synth-studio") {
		t.Fatalf("AuraGolf has its own audio engine and must not load Synth Studio: %s", entry)
	}
	for _, file := range []string{"3d/minigolf/manifest.json", "js/vendor/minigolf/game.esm.js", "3d/minigolf/audio/manifest.json", "img/desktop-icons/minigolf.svg"} {
		resource, err := Content.Open(file)
		if err != nil {
			t.Fatalf("missing packaged resource %s: %v", file, err)
		}
		resource.Close()
	}
}

func TestDesktopMinigolfLocales(t *testing.T) {
	var reference map[string]string
	data, err := os.ReadFile(filepath.Join("lang", "desktop", "en.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &reference); err != nil {
		t.Fatal(err)
	}
	for _, locale := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		data, err := os.ReadFile(filepath.Join("lang", "desktop", locale+".json"))
		if err != nil {
			t.Fatal(err)
		}
		var labels map[string]string
		if err := json.Unmarshal(data, &labels); err != nil {
			t.Fatal(err)
		}
		for key := range reference {
			if strings.HasPrefix(key, "minigolf.") && strings.TrimSpace(labels[key]) == "" {
				t.Errorf("%s missing %s", locale, key)
			}
		}
		if labels["desktop.app_minigolf"] != "AuraGolf" {
			t.Errorf("%s missing brand", locale)
		}
	}
}
