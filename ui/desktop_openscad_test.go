package ui

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesktopOpenSCADLazyAssetsRoutingAndWindowRuntime(t *testing.T) {
	t.Parallel()

	loader := readDesktopAssetText(t, "js/desktop/core/module-loader.js")
	for _, want := range []string{
		"'openscad'",
		"'/css/desktop-app-openscad.css'",
		"'/css/stl-viewer.css'",
		"'/js/vendor/three.min.js'",
		"'/js/vendor/STLLoader.min.js'",
		"'/js/vendor/OrbitControls.min.js'",
		"'/js/desktop/apps/openscad.js'",
	} {
		if !strings.Contains(loader, want) {
			t.Fatalf("desktop app asset registry missing OpenSCAD marker %q", want)
		}
	}

	routing := readDesktopAssetText(t, "js/desktop/core/menus-and-routing.js")
	for _, want := range []string{
		"appId === 'openscad'",
		"window.OpenSCADApp",
		"window.OpenSCADApp.render",
		"updateWindowContext",
	} {
		if !strings.Contains(routing, want) {
			t.Fatalf("desktop routing missing OpenSCAD marker %q", want)
		}
	}

	foundation := readDesktopAssetText(t, "js/desktop/core/desktop-foundation.js")
	for _, want := range []string{
		"openscad: 'openscad'",
		"scad: 'openscad'",
		"openscad: 'OS'",
		"OpenSCADApp",
	} {
		if !strings.Contains(foundation, want) {
			t.Fatalf("desktop foundation missing OpenSCAD marker %q", want)
		}
	}

	windows := readDesktopAssetText(t, "js/desktop/core/window-shell-runtime.js")
	for _, want := range []string{
		"openscad: { width: 1120, height: 720 }",
		"openscad: true",
	} {
		if !strings.Contains(windows, want) {
			t.Fatalf("desktop window runtime missing OpenSCAD marker %q", want)
		}
	}
}

func TestDesktopOpenSCADAppMarkers(t *testing.T) {
	t.Parallel()

	app := readDesktopAssetText(t, "js/desktop/apps/openscad.js")
	for _, want := range []string{
		"window.OpenSCADApp = { render, dispose }",
		"/api/openscad/status",
		"/api/openscad/render",
		"/api/desktop/chat/stream",
		"source: 'openscad'",
		"openscad_render",
		"window.AuraSSE.on('virtual_desktop_event'",
		"data.type !== 'openscad_result'",
		"window.THREE.STLLoader || window.STLLoader",
		"requestFullscreen",
		"data-oscad-panel",
		"data-oscad-source",
		"data-oscad-agent",
		"data-oscad-render",
		"desktop.openscad.download_hint",
	} {
		if !strings.Contains(app, want) {
			t.Fatalf("OpenSCAD app missing implementation marker %q", want)
		}
	}

	css := readDesktopAssetText(t, "css/desktop-app-openscad.css")
	for _, want := range []string{
		".openscad-app",
		".oscad-shell",
		"grid-template-columns: minmax(300px, 360px) minmax(0, 1fr);",
		".oscad-source",
		".oscad-panel",
		".oscad-stl",
		".oscad-file-list",
		".oscad-footer",
		"@media (max-width: 860px)",
		"@media (prefers-reduced-motion: reduce)",
	} {
		if !strings.Contains(css, want) {
			t.Fatalf("OpenSCAD CSS missing marker %q", want)
		}
	}
}

func TestDesktopOpenSCADIconsExistInBothThemes(t *testing.T) {
	t.Parallel()

	for _, theme := range []string{"papirus", "whitesur"} {
		manifest := readDesktopAssetText(t, "img/"+theme+"/manifest.json")
		for _, want := range []string{`"openscad"`, `"open_scad"`} {
			if !strings.Contains(manifest, want) {
				t.Fatalf("%s theme manifest missing OpenSCAD icon marker %q", theme, want)
			}
		}
		rawDesktopAssetText(t, "img/"+theme+"/icons/openscad.svg")
	}
}

func TestDesktopOpenSCADTranslations(t *testing.T) {
	t.Parallel()

	keys := []string{
		"desktop.app_openscad",
		"desktop.store.badge_native",
		"desktop.openscad.title",
		"desktop.openscad.subtitle",
		"desktop.openscad.prompt",
		"desktop.openscad.prompt_placeholder",
		"desktop.openscad.ask_agent",
		"desktop.openscad.render",
		"desktop.openscad.exports",
		"desktop.openscad.mode",
		"desktop.openscad.mode_render",
		"desktop.openscad.mode_preview",
		"desktop.openscad.timeout",
		"desktop.openscad.source",
		"desktop.openscad.tab_preview",
		"desktop.openscad.tab_source",
		"desktop.openscad.tab_files",
		"desktop.openscad.tab_log",
		"desktop.openscad.refresh",
		"desktop.openscad.ready",
		"desktop.openscad.download",
		"desktop.openscad.save_desktop",
		"desktop.openscad.fullscreen",
		"desktop.openscad.render_complete",
		"desktop.openscad.status_running",
		"desktop.openscad.rendering",
		"desktop.openscad.agent_working",
		"desktop.openscad.saving",
		"desktop.openscad.saved",
		"desktop.openscad.no_files",
		"desktop.openscad.no_log",
		"desktop.openscad.no_preview",
		"desktop.openscad.download_hint",
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
