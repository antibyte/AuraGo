package ui

import (
	"encoding/json"
	"strings"
	"testing"

	"aurago/internal/desktop"
)

func TestDesktopWebampAssetsAreEmbedded(t *testing.T) {
	t.Parallel()

	bundle, err := Content.ReadFile("js/vendor/webamp/webamp.bundle.min.mjs")
	if err != nil {
		t.Fatalf("Webamp ESM bundle missing from embedded UI: %v", err)
	}
	if len(bundle) < 900000 {
		t.Fatalf("Webamp ESM bundle is unexpectedly small: %d bytes", len(bundle))
	}
	license, err := Content.ReadFile("js/vendor/webamp/LICENSE.txt")
	if err != nil {
		t.Fatalf("Webamp license missing from embedded UI: %v", err)
	}
	if !strings.Contains(string(license), "MIT License") {
		t.Fatal("Webamp license does not look like MIT")
	}
	manifestBytes, err := Content.ReadFile("js/vendor/webamp/manifest.json")
	if err != nil {
		t.Fatalf("Webamp manifest missing from embedded UI: %v", err)
	}
	var manifest struct {
		Name         string `json:"name"`
		Version      string `json:"version"`
		Source       string `json:"source"`
		License      string `json:"license"`
		Bundle       string `json:"bundle"`
		BundleType   string `json:"bundle_type"`
		NPMIntegrity string `json:"npm_integrity"`
	}
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatalf("parse Webamp manifest: %v", err)
	}
	if manifest.Name != "webamp" || manifest.Version != "2.2.0" || manifest.License != "MIT" {
		t.Fatalf("unexpected Webamp manifest identity: %+v", manifest)
	}
	if manifest.Source != "https://github.com/captbaritone/webamp" {
		t.Fatalf("unexpected Webamp source %q", manifest.Source)
	}
	if manifest.Bundle != "webamp.bundle.min.mjs" || manifest.BundleType != "core-esm" {
		t.Fatalf("unexpected Webamp bundle metadata: %+v", manifest)
	}
	if !strings.HasPrefix(manifest.NPMIntegrity, "sha512-") {
		t.Fatalf("unexpected Webamp integrity %q", manifest.NPMIntegrity)
	}
}

func TestDesktopMusicPlayerUsesLocalWebamp(t *testing.T) {
	t.Parallel()

	shell, err := Content.ReadFile("js/desktop/main.js")
	if err != nil {
		t.Fatalf("desktop shell missing from embedded UI: %v", err)
	}
	text := string(shell)
	for _, want := range []string{
		"const WEBAMP_MODULE_PATH = '/js/vendor/webamp/webamp.bundle.min.mjs'",
		"import(WEBAMP_MODULE_PATH)",
		"new Webamp({ initialTracks: tracks })",
		"renderWhenReady(webampHostNode())",
		"setTracksToPlay(tracks)",
		"loadMusicLibrary('Music')",
		"WEBAMP_TRACK_SCAN_LIMIT",
		"recursive: 'true'",
		"url: file.web_path || await desktopEmbedURL(file.path)",
		"/\\.(mp3|wav|flac|ogg|m4a|opus)$/i",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("desktop shell missing Webamp marker %q", want)
		}
	}
	for _, forbidden := range []string{"cdn.jsdelivr", "unpkg.com", "https://docs.webamp.org", "https://github.com/captbaritone/webamp", "const WEBAMP_AUDIO_PATTERN = /\\\\."} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("desktop shell must not reference remote Webamp asset %q", forbidden)
		}
	}

	var found bool
	for _, app := range desktop.BuiltinApps() {
		if app.ID == "music-player" && app.Entry == "builtin://music-player" {
			found = true
		}
	}
	if !found {
		t.Fatalf("music-player builtin app missing or changed: %+v", desktop.BuiltinApps())
	}
}
