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
		"'/js/desktop/core/media-helpers.js'",
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
		"return { entries, countries }",
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
		"data-favorites-section",
		"data-favorites-list",
		"const MAX_RECENT_SHORTCUTS = 2",
		"const MAX_FAVORITE_SHORTCUTS = 3",
		"const Media = window.AuraDesktopMediaHelpers",
		"function renderFilterControls()",
		"function countryOptions()",
		"function renderFavorites()",
		"function renderShortcutList(",
		"function resolutionBucketFromStream(stream)",
		"function resolutionMatches(entry)",
		"function formatPlaybackError(err)",
		"function destroyHls()",
		"state.hlsErrorCount = (state.hlsErrorCount || 0) + 1",
		"video.addEventListener('stalled'",
		"root.addEventListener('keydown'",
		"case 'f':",
		"case 'm':",
		"const VISIBLE_BATCH = 40",
		"state.visibleLimit = VISIBLE_BATCH",
		"state.totalVisible = entries.length",
		"'teevee-sentinel'",
		"new IntersectionObserver",
		"function fetchJSON(url, cacheMode)",
		"cache: cacheMode || 'force-cache'",
		"new AbortController",
		"setTimeout(() => controller.abort(), 20000)",
		"quality: clean(stream.quality || stream.label)",
		"resolutionBucket: resolutionBucketFromStream(stream)",
		"id: stableID(stream.channel, stream.url)",
		"favoriteKey: stableID(stream.channel, stream.url)",
		"function stableID(channelID, url)",
		"function migrateFavorites(entries)",
		"aurago.teevee.favorites.v2",
		"aurago.teevee.favorites.v1",
		"function isLegacyFavoriteKey(key)",
		"data-action=\"fullscreen-video\"",
		"playerShell.addEventListener('dblclick'",
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
		".teevee-shortcuts-panel",
		"grid-template-columns: repeat(2, minmax(0, 1fr));",
		".teevee-filter:nth-child(-n+2)",
		".teevee-favorites",
		".teevee-shortcut-list",
		".teevee-video-fullscreen",
		".teevee-now strong",
		".teevee-player",
		"container-type: inline-size;",
		".teevee-channel-list",
		".teevee-channel:hover",
		".teevee-player-bar",
		"@container (max-width: 620px)",
		".teevee-live-dot",
		"@media (max-width: 820px)",
		"@media (prefers-reduced-motion: reduce)",
	} {
		if !strings.Contains(css, want) {
			t.Fatalf("TeeVee CSS missing marker %q", want)
		}
	}
}

func TestDesktopTeeVeeFavoritesInListAndContextMenu(t *testing.T) {
	t.Parallel()

	app := readDesktopAssetText(t, "js/desktop/apps/teevee.js")
	recentMarker := `<section class="teevee-recent" data-recent-section hidden>`
	favoritesMarker := `<section class="teevee-favorites" data-favorites-section hidden>`
	if !strings.Contains(app, recentMarker) || !strings.Contains(app, favoritesMarker) {
		t.Fatalf("TeeVee app missing recent/favorites shortcut sections")
	}
	if strings.Index(app, recentMarker) > strings.Index(app, favoritesMarker) {
		t.Fatalf("TeeVee recent shortcuts should render before favorites")
	}
	for _, want := range []string{
		`data-action="favorite" data-channel-id`,
		`event.stopPropagation()`,
		"action: () => toggleFavorite(entry)",
		"checked: isFavorite(entry)",
	} {
		if !strings.Contains(app, want) {
			t.Fatalf("TeeVee app missing list/context favorite marker %q", want)
		}
	}
}

func TestDesktopTeeVeeMediaHelpers(t *testing.T) {
	t.Parallel()

	helper := readDesktopAssetText(t, "js/desktop/core/media-helpers.js")
	for _, want := range []string{
		"window.AuraDesktopMediaHelpers",
		"function clean(value)",
		"function cleanID(value)",
		"function escapeHTML(value)",
		"function normalizeSearch(value)",
		"function countryFlag(code)",
		"function countryDisplayName(code)",
		"function debounce(fn, delay)",
		"function createToast(container)",
		"function updateMediaSession(entry, album)",
		"'use strict'",
	} {
		if !strings.Contains(helper, want) {
			t.Fatalf("desktop media helpers missing marker %q", want)
		}
	}
}

func TestDesktopTeeVeePerformanceMarkers(t *testing.T) {
	t.Parallel()

	app := readDesktopAssetText(t, "js/desktop/apps/teevee.js")
	for _, forbidden := range []string{
		"state.entries.forEach(entry => {\n                if (/^[A-Z]{2}$/.test(entry.country)) seen.add(entry.country);",
		"fetch(url, { cache: 'force-cache' })",
	} {
		if strings.Contains(app, forbidden) {
			t.Fatalf("TeeVee app still uses unoptimized pattern %q", forbidden)
		}
	}
	for _, want := range []string{
		"state.countries = data.countries || new Set()",
		"const seen = state.countries || new Set()",
		"state.totalVisible = entries.length",
		"state.visibleLimit = VISIBLE_BATCH",
		"fetchJSON(CHANNELS_ENDPOINT, force ? 'no-store' : 'force-cache')",
	} {
		if !strings.Contains(app, want) {
			t.Fatalf("TeeVee app missing performance marker %q", want)
		}
	}
}

func TestDesktopTeeVeeStableIdentity(t *testing.T) {
	t.Parallel()

	app := readDesktopAssetText(t, "js/desktop/apps/teevee.js")
	for _, forbidden := range []string{
		"clean(stream.channel || stream.title || 'stream') + ':' + index",
		"favoriteKey: clean(stream.url || stream.channel || stream.title)",
	} {
		if strings.Contains(app, forbidden) {
			t.Fatalf("TeeVee app still uses index or url based identity marker %q", forbidden)
		}
	}
	for _, want := range []string{
		"function stableID(channelID, url)",
		"hashString(streamURL || id || 'stream')",
		"id: stableID(stream.channel, stream.url)",
		"favoriteKey: stableID(stream.channel, stream.url)",
	} {
		if !strings.Contains(app, want) {
			t.Fatalf("TeeVee app missing stable identity marker %q", want)
		}
	}
}

func TestDesktopTeeVeeFavoriteMigration(t *testing.T) {
	t.Parallel()

	app := readDesktopAssetText(t, "js/desktop/apps/teevee.js")
	for _, want := range []string{
		"aurago.teevee.favorites.v2",
		"aurago.teevee.favorites.v1",
		"function isLegacyFavoriteKey(key)",
		"/^https?:\\/\\//.test(clean(key))",
		"function migrateFavorites(entries)",
		"localStorage.removeItem(LEGACY_FAVORITES_KEY)",
	} {
		if !strings.Contains(app, want) {
			t.Fatalf("TeeVee app missing favorite migration marker %q", want)
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
		"desktop.teevee_unsupported_hint",
		"desktop.teevee_timeout",
		"desktop.teevee_network_error",
		"desktop.teevee_cors_error",
		"desktop.teevee_format_error",
		"desktop.teevee_stream_stalled",
		"desktop.teevee_no_channel",
		"desktop.teevee_no_results",
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

func TestDesktopTeeVeeMigrateFavoritesInRenderScope(t *testing.T) {
	t.Parallel()

	app := readDesktopAssetText(t, "js/desktop/apps/teevee.js")
	renderStart := strings.Index(app, "function render(host, windowId, context)")
	if renderStart < 0 {
		t.Fatal("render() not found")
	}
	migrateIdx := strings.Index(app, "function migrateFavorites(entries)")
	if migrateIdx < 0 {
		t.Fatal("migrateFavorites not found")
	}
	if migrateIdx < renderStart {
		t.Fatal("migrateFavorites must be defined inside render(), not at module scope")
	}
	disposeIdx := strings.Index(app, "function dispose(windowId)")
	if disposeIdx > 0 && migrateIdx > disposeIdx {
		t.Fatal("migrateFavorites must not be defined after dispose() at module scope")
	}
}

func TestDesktopTeeVeeDisposeCleanupChain(t *testing.T) {
	t.Parallel()

	app := readDesktopAssetText(t, "js/desktop/apps/teevee.js")
	for _, want := range []string{
		"disposers.set(windowId",
		"resetPlayback()",
		"clearWindowMenus",
		"disposers.delete(windowId)",
		"case ' ':",
	} {
		if !strings.Contains(app, want) {
			t.Fatalf("teevee.js missing dispose/shortcut marker %q", want)
		}
	}
}

func TestDesktopTeeVeeUsesSameOriginStreamProxy(t *testing.T) {
	t.Parallel()

	app := readDesktopAssetText(t, "js/desktop/apps/teevee.js")
	for _, want := range []string{
		"/api/desktop/teevee/stream",
		"function streamPlaybackURL(url)",
		"state.entries = data.entries",
	} {
		if !strings.Contains(app, want) {
			t.Fatalf("teevee.js missing stream proxy marker %q", want)
		}
	}
	if strings.Contains(app, "filterEntriesForPage") {
		t.Fatal("teevee must not hide HTTP channels; use stream proxy instead")
	}
}
