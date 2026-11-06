package ui

import (
	"os"
	"path/filepath"
	"regexp"
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
		"aura-drift-style.js", "aura-drift-conditions.js", "aura-drift-race.js",
		"aura-drift-cars.js", "aura-drift-scenery.js",
		"aura-drift-render.js", "aura-drift-post.js",
		"aura-drift-hud.js", "aura-drift-audio.js", "aura-drift-music.js",
		"aura-drift-ui.js", "aura-drift.js",
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
	if !strings.Contains(css, ":focus-visible") || !strings.Contains(css, ".agdrift-sr-only") {
		t.Fatal("Aura Drift stylesheet must keep keyboard focus and screen-reader affordances")
	}
	if !strings.Contains(css, ".agdrift-load-error") {
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
		"aura_drift.style", "aura_drift.combo", "aura_drift.sector",
		"aura_drift.best_combo", "aura_drift.conditions",
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
		"aura-drift-style.js", "aura-drift-conditions.js", "aura-drift-race.js",
		"aura-drift-cars.js", "aura-drift-scenery.js",
		"aura-drift-render.js", "aura-drift-post.js",
		"aura-drift-hud.js", "aura-drift-audio.js", "aura-drift-music.js",
		"aura-drift-ui.js", "aura-drift.js",
	} {
		source := readDesktopAssetText(t, filepath.Join("js", "desktop", "apps", name))
		for _, forbidden := range []string{"https://", "http://", "fetch(", "XMLHttpRequest", "import("} {
			if strings.Contains(source, forbidden) {
				t.Fatalf("%s contains %q; the racer must stay fully offline", name, forbidden)
			}
		}
	}
}

// AdGuard/EasyList hide [class^="ad-"] with display:none !important. The racer
// once shipped ad-overlay/ad-panel and was invisible with no console error.
// agdrift- contains no "ad" substring, so even [class*="ad"] rules miss it.
func TestAuraDriftAvoidsAdBlockerClassPrefix(t *testing.T) {
	t.Parallel()
	blockedPrefix := regexp.MustCompile(`(^|[^A-Za-z0-9_-])ad-[a-z]`)
	// Only shipped sources: this test file itself contains the pattern literal.
	paths := []string{
		"css/desktop-app-aura-drift.css",
		"js/desktop/apps/aura-drift.js",
		"js/desktop/apps/aura-drift-ui.js",
	}
	for _, path := range paths {
		content := readDesktopAssetText(t, path)
		if loc := blockedPrefix.FindStringIndex(content); loc != nil {
			start := loc[0]
			if start > 40 {
				start -= 40
			}
			end := loc[1] + 40
			if end > len(content) {
				end = len(content)
			}
			t.Fatalf("%s still uses an ad blocker-vulnerable class: ...%s...", path, content[start:end])
		}
	}
	css := readDesktopAssetText(t, "css/desktop-app-aura-drift.css")
	for _, required := range []string{".agdrift-root", ".agdrift-overlay", ".agdrift-panel", ".agdrift-hud"} {
		if !strings.Contains(css, required) {
			t.Fatalf("Aura Drift stylesheet lost %s", required)
		}
	}
	ui := readDesktopAssetText(t, "js/desktop/apps/aura-drift-ui.js")
	for _, required := range []string{"agdrift-panel agdrift-menu", "agdrift-primary", "agdrift-touch-btn"} {
		if !strings.Contains(ui, required) {
			t.Fatalf("Aura Drift UI lost %s", required)
		}
	}
}
