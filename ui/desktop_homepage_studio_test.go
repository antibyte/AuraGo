package ui

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDesktopHomepageStudioRouting(t *testing.T) {
	t.Parallel()

	routing := readDesktopAssetText(t, "js/desktop/core/menus-and-routing.js")
	for _, want := range []string{
		"appId === 'homepage-studio'",
		"loadAppScript('homepage-studio')",
		"window.HomepageStudioApp.render",
	} {
		if !strings.Contains(routing, want) {
			t.Fatalf("menus-and-routing missing homepage studio marker %q", want)
		}
	}

	foundation := readDesktopAssetText(t, "js/desktop/core/desktop-foundation.js")
	if !strings.Contains(foundation, "'homepage-studio': 'HomepageStudioApp'") {
		t.Fatalf("desktop-foundation missing homepage-studio dispose mapping")
	}
}

func TestHomepageStudioRedesignMarkers(t *testing.T) {
	t.Parallel()

	css := readDesktopAssetText(t, "css/desktop-app-homepage-studio.css")
	for _, want := range []string{
		"--hp-accent:",
		"focus-visible",
		"vd-hp-preview-skeleton",
		"text-wrap: balance",
		"prefers-reduced-motion",
	} {
		if !strings.Contains(css, want) {
			t.Fatalf("homepage studio css missing redesign marker %q", want)
		}
	}

	source := readDesktopAssetText(t, "js/desktop/apps/homepage-studio.js")
	for _, want := range []string{
		"<aside class=\"vd-hp-chat\"",
		"<main class=\"vd-hp-preview\"",
		"vd-hp-preview-skeleton",
		"preview_empty_title",
		"externalBtn.disabled",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("homepage studio js missing redesign marker %q", want)
		}
	}
}

func TestHomepageStudioUsesStatusPreviewURL(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/homepage-studio.js")

	for _, want := range []string{
		"function homepageStatusPreviewURL(data)",
		"state.previewUrl = homepageStatusPreviewURL(data);",
		"data.preview_url",
		"data.web_container.browser_url",
		"data.python_server.browser_url",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("homepage studio missing preview URL marker %q", want)
		}
	}

	for _, unwanted := range []string{
		"state.previewUrl = 'http://localhost:' + port;",
		"const port = 8080;",
	} {
		if strings.Contains(source, unwanted) {
			t.Fatalf("homepage studio still contains hard-coded local URL marker %q", unwanted)
		}
	}
}

func TestHomepageStudioChatPayloadCarriesHomepageScope(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/homepage-studio.js")
	for _, want := range []string{
		"function homepageWindowContext()",
		"window_context: homepageWindowContext()",
		"homepage_mode: true",
		"Use homepage_project, homepage_file, homepage_quality, homepage_deploy, and homepage_git.",
		"Do not use virtual_desktop apps, widgets, or files for Homepage Studio site changes.",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("homepage studio chat payload missing scope marker %q", want)
		}
	}
}

func TestHomepageStudioGermanUsesInformalAddress(t *testing.T) {
	t.Parallel()

	var values map[string]string
	if err := json.Unmarshal([]byte(readDesktopAssetText(t, "lang/desktop/de.json")), &values); err != nil {
		t.Fatalf("parse German desktop translations: %v", err)
	}
	for key, want := range map[string]string{
		"homepage_studio.chat_placeholder":     "Beschreibe deine Website-Änderungen...",
		"homepage_studio.preview_unavailable":  "Vorschau nicht verfügbar — starte zuerst den Homepage-Container",
		"homepage_studio.preview_empty_title":  "Noch keine Live-Vorschau",
		"homepage_studio.welcome":              "Willkommen im Homepage-Studio! Beschreibe die Website, die du erstellen möchtest, und ich erstelle sie für dich.",
	} {
		if values[key] != want {
			t.Fatalf("%s = %q, want %q", key, values[key], want)
		}
	}
}
