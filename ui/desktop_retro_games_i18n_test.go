package ui

import (
	"strings"
	"testing"
)

// Keys that are proper names or universal symbols and may equal English.
var retroGamesSameAsEnglish = map[string]bool{
	"desktop.app_retro_games":              true,
	"desktop.retro_games_nav_hall_of_fame": true,
	"desktop.retro_games_platform_msdos":   true,
	"desktop.retro_games_platform_arcade":  true,
	"desktop.retro_games_platform_c64":     true,
	"desktop.retro_games_platform_amiga":   true,
	"desktop.retro_games_platform_apple2":  true,
	"desktop.retro_games_platform_zx":      true,
	"desktop.retro_games_platform_win3":    true,
	"desktop.retro_games_sort_title":       true,
}

func TestDesktopRetroGamesTranslations(t *testing.T) {
	english := readMergedLangMap(t, "en")
	var keys []string
	for key := range english {
		if strings.HasPrefix(key, "desktop.retro_games_") {
			keys = append(keys, key)
		}
	}
	if len(keys) != 81 {
		t.Fatalf("desktop.retro_games_* keys = %d, want 81", len(keys))
	}
	// The own player page: the failure hint, the sound toggle and the page's
	// "click for sound" pill (its label travels in the player URL).
	for key, text := range map[string]string{
		"desktop.retro_games_player_failed":   "The game could not be started.",
		"desktop.retro_games_sound_off":       "Sound off",
		"desktop.retro_games_sound_on":        "Sound on",
		"desktop.retro_games_click_for_sound": "Click for sound",
	} {
		if english[key] != text {
			t.Errorf("en: %s = %q, want %q", key, english[key], text)
		}
	}
	german := readMergedLangMap(t, "de")
	for key, text := range map[string]string{
		"desktop.retro_games_player_failed":   "Das Spiel konnte nicht gestartet werden.",
		"desktop.retro_games_sound_off":       "Ton aus",
		"desktop.retro_games_sound_on":        "Ton an",
		"desktop.retro_games_click_for_sound": "Klick für Ton",
	} {
		if german[key] != text {
			t.Errorf("de: %s = %q, want %q", key, german[key], text)
		}
	}
	// The game starts by itself now: no word about clicking an on-screen start button.
	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		if loading := readMergedLangMap(t, lang)["desktop.retro_games_player_loading"]; strings.ContainsRune(loading, '⏻') {
			t.Errorf("%s: the loading text still tells the user to click ⏻: %q", lang, loading)
		}
	}
	// The specific refusals of a library change (409 library_full, 403 desktop_readonly).
	for _, key := range []string{"desktop.retro_games_library_full", "desktop.retro_games_readonly"} {
		if _, ok := english[key]; !ok {
			t.Errorf("en: %s missing", key)
		}
	}
	keys = append(keys, "desktop.app_retro_games")
	placeholders := []string{"{count}", "{date}", "{title}", "{decade}", "{platform}", "{done}", "{total}"}
	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		values := readMergedLangMap(t, lang)
		for _, key := range keys {
			value := strings.TrimSpace(values[key])
			if value == "" || strings.ContainsRune(value, '�') {
				t.Errorf("%s: %s missing or broken", lang, key)
				continue
			}
			for _, ph := range placeholders {
				if strings.Contains(english[key], ph) != strings.Contains(value, ph) {
					t.Errorf("%s: %s placeholder %s mismatch: %q", lang, key, ph, value)
				}
			}
			if lang != "en" && !retroGamesSameAsEnglish[key] && value == english[key] {
				t.Errorf("%s: %s is untranslated", lang, key)
			}
			if lang == "de" {
				for _, formal := range []string{" Sie ", " Ihre ", " Ihnen ", " Ihr "} {
					if strings.Contains(" "+value+" ", formal) {
						t.Errorf("de: %s uses formal address: %q", key, value)
					}
				}
			}
		}
	}
}
