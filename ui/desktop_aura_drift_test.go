package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/desktop"
)

// Aura Drift is a first-party offline racing app. These checks keep its shell
// registration, module order, assets and sixteen locale bundles aligned.
func TestAuraDriftAppRegistration(t *testing.T) {
	t.Parallel()

	loader := readDesktopAssetText(t, "js/desktop/core/module-loader.js")
	modules := []string{
		"aura-drift-core.js", "aura-drift-physics.js", "aura-drift-track.js",
		"aura-drift-race.js", "aura-drift-render.js", "aura-drift-post.js",
		"aura-drift-hud.js", "aura-drift-audio.js", "aura-drift-ui.js", "aura-drift.js",
	}
	previous := strings.Index(loader, "'aura-drift': {")
	if previous < 0 {
		t.Fatal("module loader has no aura-drift entry")
	}
	for _, name := range modules {
		index := strings.Index(loader, "/js/desktop/apps/"+name)
		if index <= previous {
			t.Fatalf("module %s is missing or out of order in the loader", name)
		}
		previous = index
		if _, err := os.Stat(filepath.Join("js", "desktop", "apps", name)); err != nil {
			t.Fatalf("module %s does not exist: %v", name, err)
		}
	}
	if !strings.Contains(loader, "/css/desktop-app-aura-drift.css") {
		t.Fatal("module loader misses the Aura Drift stylesheet")
	}
	if !strings.Contains(loader, "'aura-drift': ['aura_drift']") {
		t.Fatal("module loader misses the aura_drift i18n section")
	}
	for _, path := range []string{
		filepath.Join("css", "desktop-app-aura-drift.css"),
		filepath.Join("img", "desktop-icons", "aura-drift.svg"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("missing Aura Drift asset %s: %v", path, err)
		}
	}

	css := readDesktopAssetText(t, "css/desktop-app-aura-drift.css")
	if !strings.Contains(css, ":focus-visible") || !strings.Contains(css, ".ad-sr-only") {
		t.Fatal("Aura Drift stylesheet must keep keyboard focus and screen-reader affordances")
	}
	if !strings.Contains(css, ".ad-load-error") {
		t.Fatal("Aura Drift stylesheet must style the visible stale-assets error panel")
	}
	uiSource := readDesktopAssetText(t, "js/desktop/apps/aura-drift-ui.js")
	if !strings.Contains(uiSource, "aria-hidden") {
		t.Fatal("pointer-only touch controls must be hidden from screen readers")
	}
	shell := readDesktopAssetText(t, "js/desktop/apps/aura-drift.js")
	for _, marker := range []string{"missingModules", "showLoadError", "createNoopPost"} {
		if !strings.Contains(shell, marker) {
			t.Fatalf("Aura Drift shell lost its stale-loader guard %s", marker)
		}
	}

	routing := readDesktopAssetText(t, "js/desktop/core/menus-and-routing.js")
	if !strings.Contains(routing, "appId === 'aura-drift'") || !strings.Contains(routing, "window.AuraDriftApp.render") {
		t.Fatal("desktop routing does not mount Aura Drift")
	}
	bundle := readDesktopAssetText(t, "js/desktop/bundles/main.bundle.js")
	if !strings.Contains(bundle, "'aura-drift': 'AuraDriftApp'") {
		t.Fatal("main bundle is stale; run npm run build:ui")
	}

	var found *desktop.AppManifest
	apps := desktop.BuiltinApps()
	for i := range apps {
		if apps[i].ID == "aura-drift" {
			found = &apps[i]
			break
		}
	}
	if found == nil {
		t.Fatal("builtin app catalog has no aura-drift entry")
	}
	if found.Category != "games" {
		t.Fatalf("aura-drift category = %q, want games", found.Category)
	}
	if found.Metadata["logo_path"] != "/img/desktop-icons/aura-drift.svg" {
		t.Fatalf("aura-drift logo_path = %q", found.Metadata["logo_path"])
	}
}

func TestAuraDriftLocaleKeysCoverAllLanguages(t *testing.T) {
	t.Parallel()

	required := []string{
		"desktop.app_aura_drift", "aura_drift.title", "aura_drift.subtitle",
		"aura_drift.quick_race", "aura_drift.time_trial", "aura_drift.cup",
		"aura_drift.daily", "aura_drift.track", "aura_drift.laps",
		"aura_drift.quality", "aura_drift.auto", "aura_drift.low",
		"aura_drift.medium", "aura_drift.high", "aura_drift.ultra",
		"aura_drift.sound", "aura_drift.muted", "aura_drift.weapons_off",
		"aura_drift.reduced_motion", "aura_drift.show_ghost",
		"aura_drift.controls_hint", "aura_drift.best_lap", "aura_drift.record",
		"aura_drift.start", "aura_drift.pause", "aura_drift.resume",
		"aura_drift.restart", "aura_drift.menu", "aura_drift.results",
		"aura_drift.finish", "aura_drift.position", "aura_drift.new_record",
		"aura_drift.next_track", "aura_drift.ready", "aura_drift.lap",
		"aura_drift.weapon", "aura_drift.loading", "aura_drift.catchup",
		"aura_drift.load_error_hint",
	}
	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		values := readMergedLangMap(t, lang)
		for _, key := range required {
			if strings.TrimSpace(values[key]) == "" {
				t.Fatalf("%s missing Aura Drift translation key %s", lang, key)
			}
		}
	}
}

func TestAuraDriftShipsNoExternalResources(t *testing.T) {
	t.Parallel()

	for _, name := range []string{
		"aura-drift-core.js", "aura-drift-physics.js", "aura-drift-track.js",
		"aura-drift-race.js", "aura-drift-render.js", "aura-drift-post.js",
		"aura-drift-hud.js", "aura-drift-audio.js", "aura-drift-ui.js", "aura-drift.js",
	} {
		source := readDesktopAssetText(t, filepath.Join("js", "desktop", "apps", name))
		for _, forbidden := range []string{"https://", "http://", "fetch(", "XMLHttpRequest", "import("} {
			if strings.Contains(source, forbidden) {
				t.Fatalf("%s contains %q; the racer must stay fully offline", name, forbidden)
			}
		}
	}
}
