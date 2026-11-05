package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/desktop"
)

// The native music player (builtin://audio-player) owns the window that opens
// when an audio file is opened from the file manager or the desktop.

func TestAudioPlayerManifest(t *testing.T) {
	t.Parallel()

	var app *desktop.AppManifest
	for _, candidate := range desktop.BuiltinApps() {
		if candidate.ID == "audio-player" {
			found := candidate
			app = &found
		}
	}
	if app == nil {
		t.Fatal("audio-player builtin app missing")
	}
	if app.Entry != "builtin://audio-player" {
		t.Fatalf("audio-player entry = %q", app.Entry)
	}
	if app.Icon != "audio-player" {
		t.Fatalf("audio-player icon = %q", app.Icon)
	}
	if app.Category != "media" {
		t.Fatalf("audio-player category = %q", app.Category)
	}
	if !strings.Contains(strings.ToLower(app.Description), "visualization") {
		t.Fatalf("audio-player description does not mention the visualization: %q", app.Description)
	}
	if len(app.Permissions) == 0 {
		t.Fatal("audio-player needs files:read to build the playlist")
	}
}

func TestAudioPlayerAssets(t *testing.T) {
	t.Parallel()

	loader := readDesktopAssetText(t, "js/desktop/core/module-loader.js")
	for _, want := range []string{
		"'audio-player'",
		"/js/desktop/apps/audio-player.js",
		"/js/desktop/apps/audio-player-visualizer.js",
		"/js/desktop/apps/audio-player-tags.js",
		"/css/desktop-app-audio-player.css",
	} {
		if !strings.Contains(loader, want) {
			t.Fatalf("module loader missing audio player marker %q", want)
		}
	}

	routing := readDesktopAssetText(t, "js/desktop/core/menus-and-routing.js")
	for _, want := range []string{
		"appId === 'audio-player'",
		"window.AudioPlayerApp.render",
		"desktop.app_audio_player",
	} {
		if !strings.Contains(routing, want) {
			t.Fatalf("desktop routing missing audio player marker %q", want)
		}
	}

	shell := readDesktopAssetText(t, "js/desktop/core/window-shell-runtime.js")
	for _, want := range []string{
		"'audio-player': { width:",
		"window.AudioPlayerApp?.open?.(existing.id, context)",
	} {
		if !strings.Contains(shell, want) {
			t.Fatalf("window shell missing audio player marker %q", want)
		}
	}

	filemenu := readDesktopAssetText(t, "js/desktop/apps/editor-filemenu.js")
	if !strings.Contains(filemenu, "openApp('audio-player', { path: file.path })") {
		t.Fatal("audio files must open in the player window instead of the gallery lightbox")
	}

	player := readDesktopAssetText(t, "js/desktop/apps/audio-player.js")
	for _, want := range []string{
		"window.AudioPlayerApp",
		"createMediaElementSource",
		"createAnalyser",
		"window.AuraDesktopMediaSession",
		"/api/desktop/files?",
		"desktop.audio_player_queue_count",
	} {
		if !strings.Contains(player, want) {
			t.Fatalf("audio player implementation missing marker %q", want)
		}
	}

	visualizer := readDesktopAssetText(t, "js/desktop/apps/audio-player-visualizer.js")
	for _, want := range []string{"'bars'", "'radial'", "'wave'", "getByteFrequencyData", "getByteTimeDomainData"} {
		if !strings.Contains(visualizer, want) {
			t.Fatalf("audio player visualizer missing marker %q", want)
		}
	}

	tags := readDesktopAssetText(t, "js/desktop/apps/audio-player-tags.js")
	for _, want := range []string{"parseID3v2", "TIT2", "TPE1", "TALB", "APIC"} {
		if !strings.Contains(tags, want) {
			t.Fatalf("audio player tag reader missing marker %q", want)
		}
	}

	css := readDesktopAssetText(t, "css/desktop-app-audio-player.css")
	for _, want := range []string{".vd-player-canvas", ".vd-player-queue", ".vd-player-seek-fill"} {
		if !strings.Contains(css, want) {
			t.Fatalf("audio player CSS missing marker %q", want)
		}
	}
}

func TestAudioPlayerTranslations(t *testing.T) {
	t.Parallel()

	keys := []string{
		"desktop.app_audio_player",
		"desktop.audio_player_no_track",
		"desktop.audio_player_play",
		"desktop.audio_player_pause",
		"desktop.audio_player_next",
		"desktop.audio_player_previous",
		"desktop.audio_player_shuffle",
		"desktop.audio_player_repeat_off",
		"desktop.audio_player_repeat_all",
		"desktop.audio_player_repeat_one",
		"desktop.audio_player_mute",
		"desktop.audio_player_unmute",
		"desktop.audio_player_volume",
		"desktop.audio_player_seek",
		"desktop.audio_player_queue",
		"desktop.audio_player_queue_count",
		"desktop.audio_player_queue_empty",
		"desktop.audio_player_reload",
		"desktop.audio_player_mode_bars",
		"desktop.audio_player_mode_radial",
		"desktop.audio_player_mode_wave",
		"desktop.audio_player_unknown_artist",
	}
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
		for _, key := range keys {
			if strings.TrimSpace(values[key]) == "" {
				t.Fatalf("%s missing non-empty %s", path, key)
			}
		}
		if !strings.Contains(values["desktop.audio_player_queue_count"], "{{count}}") {
			t.Fatalf("%s queue count must keep the {{count}} placeholder", path)
		}
	}
}
