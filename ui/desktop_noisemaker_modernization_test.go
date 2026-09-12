package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var noisemakerRefreshTranslationKeys = []string{
	"desktop.noisemaker_mode_simple",
	"desktop.noisemaker_mode_custom",
	"desktop.noisemaker_mode_switch_custom",
	"desktop.noisemaker_presets_label",
	"desktop.noisemaker_preset_lofi",
	"desktop.noisemaker_preset_lofi_hint",
	"desktop.noisemaker_preset_synthwave",
	"desktop.noisemaker_preset_synthwave_hint",
	"desktop.noisemaker_preset_orchestral",
	"desktop.noisemaker_preset_orchestral_hint",
	"desktop.noisemaker_preset_folk",
	"desktop.noisemaker_preset_folk_hint",
	"desktop.noisemaker_preset_techno",
	"desktop.noisemaker_preset_techno_hint",
	"desktop.noisemaker_preset_boombap",
	"desktop.noisemaker_preset_boombap_hint",
	"desktop.noisemaker_preset_jazz",
	"desktop.noisemaker_preset_jazz_hint",
	"desktop.noisemaker_preset_ambient",
	"desktop.noisemaker_preset_ambient_hint",
	"desktop.noisemaker_create_panel",
	"desktop.noisemaker_create_panel_show",
	"desktop.noisemaker_create_panel_hide",
	"desktop.noisemaker_refresh",
	"desktop.noisemaker_back_to_library",
	"desktop.noisemaker_filter_all",
	"desktop.noisemaker_filter_favorites",
	"desktop.noisemaker_view_grid",
	"desktop.noisemaker_view_list",
	"desktop.noisemaker_select",
	"desktop.noisemaker_select_all",
	"desktop.noisemaker_select_none",
	"desktop.noisemaker_selected_count",
	"desktop.noisemaker_selected_count_one",
	"desktop.noisemaker_favorite_add",
	"desktop.noisemaker_favorite_remove",
	"desktop.noisemaker_favorite_failed",
	"desktop.noisemaker_no_favorites_title",
	"desktop.noisemaker_no_favorites_hint",
	"desktop.noisemaker_more_actions",
	"desktop.noisemaker_enqueue",
	"desktop.noisemaker_details",
	"desktop.noisemaker_tracks_delete_title",
	"desktop.noisemaker_tracks_delete_confirm",
	"desktop.noisemaker_tracks_deleted",
	"desktop.noisemaker_tracks_deleted_partial",
	"desktop.noisemaker_track_delete_failed",
	"desktop.noisemaker_player_shuffle",
	"desktop.noisemaker_player_repeat_off",
	"desktop.noisemaker_player_repeat_all",
	"desktop.noisemaker_player_repeat_one",
	"desktop.noisemaker_player_mute",
	"desktop.noisemaker_player_unmute",
	"desktop.noisemaker_player_expand",
	"desktop.noisemaker_player_collapse",
	"desktop.noisemaker_now_playing",
	"desktop.noisemaker_queue_title",
	"desktop.noisemaker_queue_empty",
	"desktop.noisemaker_queue_clear",
	"desktop.noisemaker_queue_remove",
	"desktop.noisemaker_queue_added",
	"desktop.noisemaker_queue_added_one",
	"desktop.noisemaker_lyrics_empty",
	"desktop.noisemaker_visualizer",
	"desktop.noisemaker_visualizer_unavailable",
	"desktop.noisemaker_playback_failed",
	"desktop.noisemaker_menu_playback",
	"desktop.noisemaker_menu_new_song",
	"desktop.noisemaker_menu_play_pause",
	"desktop.noisemaker_menu_next",
	"desktop.noisemaker_menu_previous",
	"desktop.noisemaker_result_play",
}

func TestDesktopNoisemakerRefreshTranslations(t *testing.T) {
	t.Parallel()

	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		path := filepath.Join("lang", "desktop", lang+".json")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		var values map[string]string
		if err := json.Unmarshal(data, &values); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, key := range noisemakerRefreshTranslationKeys {
			if strings.TrimSpace(values[key]) == "" {
				t.Fatalf("%s missing non-empty translation for %s", path, key)
			}
		}
		for _, key := range []string{"desktop.noisemaker_selected_count", "desktop.noisemaker_tracks_delete_title", "desktop.noisemaker_tracks_deleted", "desktop.noisemaker_queue_added"} {
			if !strings.Contains(values[key], "{{count}}") {
				t.Fatalf("%s: %s must keep the {{count}} placeholder", path, key)
			}
		}
		if !strings.Contains(values["desktop.noisemaker_tracks_deleted_partial"], "{{done}}") || !strings.Contains(values["desktop.noisemaker_tracks_deleted_partial"], "{{total}}") {
			t.Fatalf("%s: tracks_deleted_partial must keep {{done}} and {{total}}", path)
		}
		if !strings.Contains(values["desktop.noisemaker_onboarding_hint"], "ACE-Step") {
			t.Fatalf("%s: onboarding hint must mention ACE-Step", path)
		}
		if lang == "de" {
			for key, value := range values {
				if !strings.HasPrefix(key, "desktop.noisemaker_") {
					continue
				}
				for _, forbidden := range []string{"Sie k", "Loeschen", "Auswaehlen", "hinzufuegen"} {
					if strings.Contains(value, forbidden) {
						t.Fatalf("%s: %s must use Du form and real umlauts, found %q", path, key, forbidden)
					}
				}
			}
		}
	}
}
