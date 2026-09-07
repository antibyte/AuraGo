package ui

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesktopMakerRadioI18n(t *testing.T) {
	t.Parallel()

	maker := readDesktopAssetText(t, "js/desktop/apps/game-maker-studio.js")
	if strings.Count(maker, "state.context.t('game_maker.modules_load_failed')") < 2 {
		t.Fatal("game maker missing-modals path must localize game_maker.modules_load_failed twice")
	}
	if strings.Contains(maker, "Game Maker Studio modules failed to load") {
		t.Fatal("game maker still hardcodes Game Maker Studio modules failed to load")
	}

	radio := readDesktopAssetText(t, "js/desktop/apps/radio.js")
	if !strings.Contains(radio, "throw new Error('Radio Browser HTTP')") {
		t.Fatal("radio fetchJSON must throw the Radio Browser HTTP sentinel without a status")
	}
	if strings.Contains(radio, "'Radio Browser HTTP ' +") {
		t.Fatal("radio still concatenates Radio Browser HTTP with a status")
	}
	if strings.Contains(radio, "Radio Browser HTTP '") {
		t.Fatal("radio still builds Radio Browser HTTP with a trailing status fragment")
	}
	if !strings.Contains(radio, "async function loadCatalog(query)") || !strings.Contains(radio, "t('desktop.radio_catalog_error')") {
		t.Fatal("radio shared catalog loader must localize desktop.radio_catalog_error")
	}
	if strings.Contains(radio, "state.error = err.message") {
		t.Fatal("radio catalog catches still dump err.message")
	}

	englishCatalog := "Could not load stations."
	englishModules := "Game Maker Studio modules failed to load."
	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		path := filepath.ToSlash(filepath.Join("lang", "desktop", lang+".json"))
		var values map[string]string
		if err := json.Unmarshal([]byte(rawDesktopAssetText(t, path)), &values); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, key := range []string{"desktop.radio_catalog_error", "desktop.radio_tune", "desktop.radio_tune_hint", "desktop.radio_empty_favorites", "desktop.radio_popular", "game_maker.modules_load_failed"} {
			got := values[key]
			if strings.TrimSpace(got) == "" {
				t.Fatalf("%s missing non-empty %s", path, key)
			}
		}
		if lang == "de" || lang == "fr" {
			if values["desktop.radio_catalog_error"] == englishCatalog {
				t.Fatalf("%s must not copy the English radio catalog string", path)
			}
			if values["game_maker.modules_load_failed"] == englishModules {
				t.Fatalf("%s must not copy the English game maker modules string", path)
			}
		}
	}
}
