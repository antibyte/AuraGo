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

func TestDesktopNoisemakerRefreshMenusModule(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/noisemaker-menus.js")
	for _, marker := range []string{
		"window.NoisemakerMenus = { windowMenus, trackContextItems, libraryContextItems, withCheckIcons }",
		"function withCheckIcons(items)",
		"id: 'file'", "id: 'edit'", "id: 'view'", "id: 'playback'",
		"id: 'new-song'", "id: 'favorite-toggle'", "id: 'select-mode'",
		"id: 'view-grid'", "id: 'view-list'", "id: 'filter-favorites'", "id: 'create-panel'", "id: 'now-playing'", "id: 'visualizer'",
		"id: 'play-pause'", "id: 'shuffle'", "'repeat-' + mode", "id: 'clear-queue'",
		"id: 'enqueue'", "id: 'details'", "id: 'toggle-select'",
		"tFull('desktop.menu_file')", "tFull('desktop.menu_edit')", "tFull('desktop.menu_view')",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("noisemaker-menus.js missing marker %q", marker)
		}
	}
	for _, forbidden := range []string{"alert(", "confirm(", "prompt(", "window.NoisemakerApp", "window.NoisemakerLibrary", "window.NoisemakerPlayer"} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("noisemaker-menus.js must not contain %q", forbidden)
		}
	}
}

func TestDesktopNoisemakerRefreshLibraryModule(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/noisemaker-library.js")
	for _, marker := range []string{
		"window.NoisemakerLibrary = { create, formatDuration, formatDate }",
		"function formatDuration(ms)",
		"data-nm-filter=\"favorites\"",
		"data-nm-view=\"list\"",
		"data-nm-select-toggle",
		"nm-selection-bar",
		"nm-grid--list",
		"class=\"nm-card-fav\"",
		"class=\"nm-card-check\"",
		"IntersectionObserver",
		"function setSelectMode(on)",
		"function selectRange(track)",
		"function highlight(id)",
		"is-highlighted",
		"emit('contextmenu', { x: event.clientX, y: event.clientY, track })",
		"no_favorites_title",
		"nm-skeleton",
		"!trackById(",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("noisemaker-library.js missing marker %q", marker)
		}
	}
	for _, forbidden := range []string{"new Audio(", "nm-player", "alert(", "confirm(", "window.NoisemakerApp", "window.NoisemakerPlayer"} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("noisemaker-library.js must not contain %q", forbidden)
		}
	}
}

func TestDesktopNoisemakerRefreshPlayerModule(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/noisemaker-player.js")
	for _, marker := range []string{
		"window.NoisemakerPlayer = { create }",
		"new Audio()",
		"createMediaElementSource(audio)",
		"createAnalyser()",
		"analyser.fftSize = 256",
		"requestAnimationFrame(drawFrame)",
		"prefers-reduced-motion",
		"audioCtx.resume()",
		"getByteFrequencyData",
		"class=\"nm-player\"",
		"nm-now-playing",
		"data-np-lyrics",
		"data-np-queue",
		"data-nm-seek",
		"data-nm-volume",
		"emit('needmore')",
		"emit('visualizer-unavailable')",
		"emit('expand', ",
		"function setRepeat(mode)",
		"function setShuffle(value)",
		"function buildOrder(startIndex)",
		"errorStreak >= 2",
		"document.addEventListener('visibilitychange', onVisibility)",
		"document.removeEventListener('visibilitychange', onVisibility)",
		"audioCtx.close()",
		"'pointercancel'",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("noisemaker-player.js missing marker %q", marker)
		}
	}
	for _, forbidden := range []string{"alert(", "confirm(", "audioCtx.suspend()", "window.NoisemakerApp", "window.NoisemakerLibrary"} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("noisemaker-player.js must not contain %q", forbidden)
		}
	}
}

func TestDesktopNoisemakerRefreshCreateModule(t *testing.T) {
	src := readDesktopAssetText(t, "js/desktop/apps/noisemaker-create.js")
	for _, marker := range []string{
		"window.NoisemakerCreate = { create, PRESETS }",
		"const PRESETS = [",
		"id: 'lofi'", "id: 'synthwave'", "id: 'orchestral'", "id: 'folk'",
		"id: 'techno'", "id: 'boombap'", "id: 'jazz'", "id: 'ambient'",
		"data-nm-mode=\"simple\"",
		"data-nm-mode=\"custom\"",
		"closest('.nm-segment-btn[data-nm-mode]')",
		"data-nm-preset=\"",
		"nm-preset--",
		"data-nm-switch-custom",
		"data-nm-result-play",
		"data-nm-result-library",
		"data-nm-result-new",
		"data-nm-field=\"seed\"",
		"data-nm-local-status",
		"data-nm-elapsed",
		"emit('generate', params)",
		"emit('new-song')",
		"emit('play-result', generation.result)",
		"/api/desktop/noisemaker/enhance",
		"function setGeneration(next)",
		"function setCaps(next, options)",
	} {
		if !strings.Contains(src, marker) {
			t.Fatalf("noisemaker-create.js missing %q", marker)
		}
	}
	for _, forbidden := range []string{"<audio", "alert(", "window.NoisemakerApp", "/api/desktop/noisemaker/generate", "localStorage"} {
		if strings.Contains(src, forbidden) {
			t.Fatalf("noisemaker-create.js must not contain %q", forbidden)
		}
	}
}

func TestDesktopNoisemakerRefreshAssets(t *testing.T) {
	loader := readDesktopAssetText(t, "js/desktop/core/module-loader.js")
	wantOrder := "scripts: ['/js/desktop/apps/noisemaker-menus.js', '/js/desktop/apps/noisemaker-library.js', '/js/desktop/apps/noisemaker-player.js', '/js/desktop/apps/noisemaker-create.js', '/js/desktop/apps/noisemaker.js']"
	if !strings.Contains(loader, wantOrder) {
		t.Fatalf("module-loader must load the noisemaker modules in order: %s", wantOrder)
	}

	routing := readDesktopAssetText(t, "js/desktop/core/menus-and-routing.js")
	wantCtx := "window.NoisemakerApp.render(contentEl(id), id, Object.assign({}, context || {}, { esc, api, t, iconMarkup, notify: showDesktopNotification, openApp, confirmDialog, setWindowMenus, clearWindowMenus, showContextMenu, wireContextMenuBoundary, readonly: desktopReadonly() }))"
	if !strings.Contains(routing, wantCtx) {
		t.Fatalf("noisemaker window context must expose context menus and readonly: %s", wantCtx)
	}

	shell := readDesktopAssetText(t, "js/desktop/apps/noisemaker.js")
	for _, marker := range []string{
		"window.NoisemakerApp = { render, dispose }",
		"const PREF_KEY = 'aurago.desktop.noisemaker.prefs'",
		"const CREATE_MIN = 320",
		"const CREATE_MAX = 560",
		"const COMPACT_WIDTH = 860",
		"const TRACKS_PAGE_SIZE = 60",
		"window.NoisemakerMenus.windowMenus(menuModel(S))",
		"window.NoisemakerMenus.trackContextItems(",
		"window.NoisemakerMenus.libraryContextItems(",
		"window.NoisemakerLibrary.create(",
		"window.NoisemakerPlayer.create(",
		"window.NoisemakerCreate.create(",
		"data-nm-splitter",
		"role=\"separator\"",
		"data-nm-pane=\"create\"",
		"data-nm-pane=\"library\"",
		"data-nm-player-slot",
		"data-nm-pane-switch",
		"data-nm-toggle-create",
		"data-nm-refresh",
		"params.set('favorites', '1')",
		"method: 'PATCH'",
		"method: 'DELETE'",
		"'/api/desktop/noisemaker/generate'",
		"'/api/desktop/noisemaker/state'",
		"ctx.clearWindowMenus",
		"new ResizeObserver(",
		"ctx.wireContextMenuBoundary(S.root)",
		"S.ctx.showContextMenu(payload.x, payload.y, items)",
		"tracks_deleted_partial",
		"favorite_failed",
		"playback_failed",
		"seq !== S.loadSeq || !S.library",
	} {
		if !strings.Contains(shell, marker) {
			t.Fatalf("noisemaker.js missing %q", marker)
		}
	}
	for _, forbidden := range []string{"alert(", "new Audio(", "/api/music-generation", "desktop.rel_time_", "nm-tab\" role=\"tab\" data-view"} {
		if strings.Contains(shell, forbidden) {
			t.Fatalf("noisemaker.js must not contain %q", forbidden)
		}
	}
}

func TestDesktopNoisemakerRefreshStyles(t *testing.T) {
	css := readDesktopAssetText(t, "css/desktop-app-noisemaker.css")
	for _, marker := range []string{
		".noisemaker-app {",
		"--nm-bg: var(--vd-theme-app-bg)",
		"--nm-create-width",
		".noisemaker-app [hidden] { display: none !important; }",
		".nm-workbench {",
		".nm-splitter {",
		".nm-splitter:focus-visible",
		".noisemaker-app.is-create-collapsed .nm-pane-create",
		// explicit grid placement: hiding the create pane must not shift the library into the 6px splitter track
		".nm-pane-library { grid-column: 3; }",
		".noisemaker-app.is-compact .nm-workbench",
		".noisemaker-app.is-compact[data-nm-pane=\"create\"] .nm-pane-library",
		".noisemaker-app.is-compact[data-nm-pane=\"library\"] .nm-pane-create",
		".nm-presets {",
		".nm-preset.is-active",
		".nm-mode-switch",
		".nm-grid {",
		// cards use overflow:hidden, so auto rows would be squeezed into the definite grid height
		"grid-auto-rows: max-content;",
		".nm-grid--list",
		".nm-create-action {",
		"position: sticky;",
		".nm-card.is-playing",
		".nm-card.is-selected",
		".nm-card.is-highlighted",
		".nm-row {",
		".nm-selection-bar {",
		".nm-skeleton {",
		".nm-player {",
		".nm-player.is-visible",
		"visibility: visible; border-top-color: var(--nm-border);",
		".nm-player-viz",
		".nm-now-playing {",
		".nm-now-playing.is-open",
		".nm-np-hero",
		".nm-queue-item.is-current",
		"@media (max-width: 720px)",
		"@media (prefers-reduced-motion: reduce)",
		"@keyframes nm-eq",
	} {
		if !strings.Contains(css, marker) {
			t.Fatalf("desktop-app-noisemaker.css missing %q", marker)
		}
	}
	for _, forbidden := range []string{"var(--vd-surface, #111827)", ".nm-tab {", ".nm-view {", ".nm-player-btn"} {
		if strings.Contains(css, forbidden) {
			t.Fatalf("desktop-app-noisemaker.css must not contain %q", forbidden)
		}
	}
}

func TestDesktopNoisemakerReviewFixes(t *testing.T) {
	t.Parallel()

	shell := readDesktopAssetText(t, "js/desktop/apps/noisemaker.js")
	player := readDesktopAssetText(t, "js/desktop/apps/noisemaker-player.js")
	css := readDesktopAssetText(t, "css/desktop-app-noisemaker.css")

	for _, marker := range []string{
		"data.status === 'error'",
		"throw new Error(data.message || '')",
		"err.name === 'AbortError'",
		"desktop.noisemaker_error_unknown",
		"S.tracks.filter(track => !S.player.queueHas(track.id))",
		"S.player.cancelPendingAutoplay()",
		// end of queue waits for a library load that is already running instead of giving up
		"function continueQueue(S, attempt)",
		"S.tracksInFlight.then(() => continueQueue(S, attempt + 1))",
		"function loadMoreTracks(S) { return S.tracksLoading ? Promise.resolve([]) : watchLoad(S, loadMoreTracksNow(S)); }",
		"S.visualizerAvailable = S.player.visualizerAvailable()",
		// desktop toasts take a payload object; a bare string renders an empty toast
		"title: S.t('desktop.app_noisemaker'), message: String(message || ''), appId: 'noisemaker'",
		"S.prefs.shuffle !== prevShuffle || S.prefs.repeat !== prevRepeat || S.prefs.visualizer !== prevVisualizer || S.prefs.muted !== prevMuted",
	} {
		if !strings.Contains(shell, marker) {
			t.Fatalf("noisemaker.js missing review-fix marker %q", marker)
		}
	}

	for _, marker := range []string{
		"function cancelPendingAutoplay()",
		"function queueHas(id)",
		"pendingAutoplay = false",
		"volumeInput.addEventListener('change', () => { emitChange(); })",
		"audio.volume = value;",
		"np.classList.toggle('is-viz-off', vizOff)",
	} {
		if !strings.Contains(player, marker) {
			t.Fatalf("noisemaker-player.js missing review-fix marker %q", marker)
		}
	}

	for _, marker := range []string{
		".noisemaker-app.is-compact .nm-player-volume",
		".noisemaker-app.is-compact .nm-player-info",
		".noisemaker-app.is-compact .nm-row {",
		".noisemaker-app.is-compact .nm-np-body",
		"color-scheme: light",
		".desktop-body[data-theme=\"fruity\"][data-fruity-mode=\"dark\"] .noisemaker-app",
		".nm-now-playing.is-viz-off .nm-np-viz",
		".nm-player-viz { border-radius: 6px; flex: 0 0 auto; height: 40px; width: 80px; color: var(--nm-accent); }",
		".nm-np-viz { border-radius: 8px; height: 72px; width: min(420px, 100%); color: var(--nm-accent); }",
	} {
		if !strings.Contains(css, marker) {
			t.Fatalf("desktop-app-noisemaker.css missing review-fix marker %q", marker)
		}
	}
}
