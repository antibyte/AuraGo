package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/desktop"
)

func TestGameMakerStudioDesktopRegistrationAndIsolation(t *testing.T) {
	var found bool
	for _, app := range desktop.BuiltinApps() {
		if app.ID != "game-maker-studio" {
			continue
		}
		found = true
		if app.Entry != "builtin://game-maker-studio" || app.Metadata["open_maximized"] != "true" {
			t.Fatalf("unexpected Game Maker manifest: %+v", app)
		}
		if app.Metadata["logo_path"] != "/img/desktop-icons/game-maker-studio.svg" {
			t.Fatalf("unexpected Game Maker icon: %+v", app.Metadata)
		}
	}
	if !found {
		t.Fatal("game-maker-studio is not registered as a built-in desktop app")
	}

	loader := readGameMakerAsset(t, "js", "desktop", "core", "module-loader.js")
	for _, marker := range []string{
		"'game-maker-studio'",
		"/css/desktop-app-game-maker-studio.css",
		"/js/desktop/apps/game-maker-studio-api.js",
		"/js/desktop/apps/game-maker-studio.js",
		"['game_maker']",
	} {
		if !strings.Contains(loader, marker) {
			t.Errorf("desktop lazy loader missing %q", marker)
		}
	}

	app := readGameMakerAsset(t, "js", "desktop", "apps", "game-maker-studio.js")
	for _, marker := range []string{
		`frame.setAttribute('sandbox', 'allow-scripts')`,
		`event.source !== state.frame.contentWindow`,
		`data.channel !== state.channelID`,
		`state.eventSource.close()`,
		`window.removeEventListener('message'`,
		`confirmDialog`,
		`preview_reload`,
		`'polishing', 'cancelling'`,
		`const modal = layer.querySelector('.gm-modal');`,
		`const editable = Boolean(state.project && !active`,
	} {
		if !strings.Contains(app, marker) {
			t.Errorf("Game Maker UI missing lifecycle/security marker %q", marker)
		}
	}
	if strings.Contains(app, "allow-same-origin") ||
		strings.Contains(app, "window.confirm(") ||
		strings.Contains(app, "window.prompt(") ||
		strings.Contains(app, "alert(") {
		t.Fatal("Game Maker UI contains a forbidden iframe or native dialog capability")
	}
}

func TestGameMakerStudioTranslationsCoverEveryDesktopLocale(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("lang", "desktop", "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 16 {
		t.Fatalf("desktop locales = %d, want 16", len(files))
	}
	required := []string{
		"desktop.app_game_maker_studio",
		"game_maker.title",
		"game_maker.new_game",
		"game_maker.dimension",
		"game_maker.image_assets",
		"game_maker.music_assets",
		"game_maker.phase_validating",
		"game_maker.status_interrupted",
		"game_maker.restore_confirm",
		"game_maker.delete_confirm",
	}
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		var values map[string]string
		if err := json.Unmarshal(data, &values); err != nil {
			t.Fatalf("%s: %v", file, err)
		}
		for _, key := range required {
			if strings.TrimSpace(values[key]) == "" {
				t.Errorf("%s missing %s", file, key)
			}
		}
	}
}

func readGameMakerAsset(t *testing.T, parts ...string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(parts...))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
