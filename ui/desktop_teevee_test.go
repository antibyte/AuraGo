package ui

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesktopTeeVeeLazyAssetsAndRouting(t *testing.T) {
	t.Parallel()

	loader := readDesktopAssetText(t, "js/desktop/core/module-loader.js")
	for _, want := range []string{
		"'teevee'",
		"'/css/teevee.css'",
		"'/js/vendor/hls.min.js'",
		"'/js/desktop/apps/teevee.js'",
	} {
		if !strings.Contains(loader, want) {
			t.Fatalf("desktop app asset registry missing TeeVee marker %q", want)
		}
	}

	routing := readDesktopAssetText(t, "js/desktop/core/menus-and-routing.js")
	for _, want := range []string{
		"appId === 'teevee'",
		"window.TeeVeeApp",
		"window.TeeVeeApp.render",
	} {
		if !strings.Contains(routing, want) {
			t.Fatalf("desktop routing missing TeeVee marker %q", want)
		}
	}

	foundation := readDesktopAssetText(t, "js/desktop/core/desktop-foundation.js")
	for _, want := range []string{
		"teevee: 'teevee'",
		"teevee: 'TV'",
		"win.appId === 'teevee'",
		"window.TeeVeeApp",
	} {
		if !strings.Contains(foundation, want) {
			t.Fatalf("desktop foundation missing TeeVee marker %q", want)
		}
	}

	windows := readDesktopAssetText(t, "js/desktop/core/window-shell-runtime.js")
	for _, want := range []string{
		"teevee: { width: 1120, height: 720 }",
		"teevee: true",
	} {
		if !strings.Contains(windows, want) {
			t.Fatalf("desktop window runtime missing TeeVee marker %q", want)
		}
	}
}

func TestDesktopTeeVeeAppMarkers(t *testing.T) {
	t.Parallel()

	app := readDesktopAssetText(t, "js/desktop/apps/teevee.js")
	for _, want := range []string{
		"const IPTV_API_BASE = 'https://iptv-org.github.io/api'",
		"const CHANNELS_ENDPOINT = IPTV_API_BASE + '/channels.json'",
		"const STREAMS_ENDPOINT = IPTV_API_BASE + '/streams.json'",
		"const CATEGORIES_ENDPOINT = IPTV_API_BASE + '/categories.json'",
		"aurago.teevee.favorites.v1",
		"aurago.teevee.recent.v1",
		"function joinStreamsWithChannels(channels, streams, categories)",
		"stream.channel",
		"channel.id",
		"function isUnsupportedStream(stream)",
		"stream.user_agent",
		"stream.referrer",
		"function playChannel(entry)",
		"window.Hls",
		"canPlayType('application/vnd.apple.mpegurl')",
		"requestFullscreen",
		"mediaSession",
		"Stream unavailable",
		"setWindowMenus",
		"wireContextMenuBoundary",
		"inputmode=\"search\"",
		"enterkeyhint=\"search\"",
		"data-country-filter",
		"data-resolution-filter",
		"function renderFilterControls()",
		"function countryOptions()",
		"function resolutionBucketFromStream(stream)",
		"function resolutionMatches(entry)",
		"quality: clean(stream.quality || stream.label)",
		"resolutionBucket: resolutionBucketFromStream(stream)",
	} {
		if !strings.Contains(app, want) {
			t.Fatalf("TeeVee app missing implementation marker %q", want)
		}
	}

	css := readDesktopAssetText(t, "css/teevee.css")
	for _, want := range []string{
		".teevee-app",
		".teevee-sidebar",
		".teevee-control-grid",
		".teevee-select-field",
		".teevee-player",
		".teevee-channel-list",
		".teevee-channel:hover",
		".teevee-player-bar",
		".teevee-live-dot",
		"@media (max-width: 820px)",
		"@media (prefers-reduced-motion: reduce)",
	} {
		if !strings.Contains(css, want) {
			t.Fatalf("TeeVee CSS missing marker %q", want)
		}
	}
}

func TestDesktopTeeVeeTranslations(t *testing.T) {
	t.Parallel()

	keys := []string{
		"desktop.app_teevee",
		"desktop.teevee_search_placeholder",
		"desktop.teevee_filter_germany",
		"desktop.teevee_filter_global",
		"desktop.teevee_filter_favorites",
		"desktop.teevee_filter_news",
		"desktop.teevee_filter_sports",
		"desktop.teevee_filter_movies",
		"desktop.teevee_filter_music",
		"desktop.teevee_filter_kids",
		"desktop.teevee_filter_documentary",
		"desktop.teevee_country",
		"desktop.teevee_country_all",
		"desktop.teevee_resolution",
		"desktop.teevee_resolution_all",
		"desktop.teevee_resolution_uhd",
		"desktop.teevee_resolution_fhd",
		"desktop.teevee_resolution_hd",
		"desktop.teevee_resolution_sd",
		"desktop.teevee_resolution_unknown",
		"desktop.teevee_loading",
		"desktop.teevee_stream_unavailable",
		"desktop.teevee_unsupported_stream",
		"desktop.teevee_no_channel",
		"desktop.teevee_no_results",
		"desktop.teevee_quality",
		"desktop.teevee_live",
		"desktop.teevee_refresh",
		"desktop.teevee_fullscreen",
		"desktop.teevee_source",
		"desktop.teevee_now_playing",
	}

	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		lang := lang
		t.Run(lang, func(t *testing.T) {
			t.Parallel()

			data, err := Content.ReadFile(filepath.ToSlash(filepath.Join("lang", "desktop", lang+".json")))
			if err != nil {
				t.Fatalf("read %s desktop translations: %v", lang, err)
			}
			var values map[string]string
			if err := json.Unmarshal(data, &values); err != nil {
				t.Fatalf("parse %s desktop translations: %v", lang, err)
			}
			for _, key := range keys {
				if strings.TrimSpace(values[key]) == "" {
					t.Fatalf("%s missing non-empty translation for %s", lang, key)
				}
			}
		})
	}
}
