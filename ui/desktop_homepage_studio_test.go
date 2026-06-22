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
		"function homepageStatusPreviewURL(data, target)",
		"state.previewUrl = homepageStatusPreviewURL(data, state.target);",
		"id=\"hp-url-${windowId}\"",
		"class=\"vd-hp-preview-url\"",
		"previewPanel.insertBefore(iframe, previewLoading);",
		"case 'vercel':",
		"case 'netlify':",
		"case 'remote':",
		"data.vercel_url",
		"data.netlify_url",
		"data.remote_url",
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
		"previewBody.insertBefore(iframe, previewLoading);",
		"state.previewUrl = homepageStatusPreviewURL(data);",
	} {
		if strings.Contains(source, unwanted) {
			t.Fatalf("homepage studio still contains hard-coded local URL marker %q", unwanted)
		}
	}
}

func TestHomepageStudioPreviewSandboxKeepsOpaqueOrigin(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/homepage-studio.js")
	for _, want := range []string{
		"iframe.sandbox = 'allow-scripts allow-forms';",
		"iframe.referrerPolicy = 'no-referrer';",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("homepage studio preview iframe missing sandbox marker %q", want)
		}
	}
	for _, unwanted := range []string{
		"allow-same-origin",
		"allow-popups",
	} {
		if strings.Contains(source, unwanted) {
			t.Fatalf("homepage studio preview iframe must not use sandbox flag %q", unwanted)
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

func TestHomepageStudioHistoryPanelMarkers(t *testing.T) {
	t.Parallel()

	css := readDesktopAssetText(t, "css/desktop-app-homepage-studio.css")
	for _, want := range []string{
		".vd-hp-preview-tabs",
		".vd-hp-history-panel",
		".vd-hp-history-controls",
		".vd-hp-history-entry",
		".vd-hp-history-type-decision",
		".vd-hp-history-delete",
	} {
		if !strings.Contains(css, want) {
			t.Fatalf("homepage studio css missing history marker %q", want)
		}
	}

	source := readDesktopAssetText(t, "js/desktop/apps/homepage-studio.js")
	for _, want := range []string{
		"function switchPanel",
		"function loadHistory",
		"function renderHistory",
		"/api/homepage/history",
		"history_tab",
		"history_search_placeholder",
		"history_delete_confirm",
		"homepage_studio.history_type_",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("homepage studio js missing history marker %q", want)
		}
	}

	var en map[string]string
	if err := json.Unmarshal([]byte(readDesktopAssetText(t, "lang/desktop/en.json")), &en); err != nil {
		t.Fatalf("parse English desktop translations: %v", err)
	}
	for _, key := range []string{
		"homepage_studio.history_tab",
		"homepage_studio.history_search_placeholder",
		"homepage_studio.history_filter_label",
		"homepage_studio.history_filter_all",
		"homepage_studio.history_loading",
		"homepage_studio.history_empty",
		"homepage_studio.history_error",
		"homepage_studio.history_delete",
		"homepage_studio.history_delete_confirm",
		"homepage_studio.history_type_decision",
	} {
		if en[key] == "" {
			t.Fatalf("English desktop translation missing key %q", key)
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
		"homepage_studio.chat_placeholder":    "Beschreibe deine Website-Änderungen...",
		"homepage_studio.preview_unavailable": "Vorschau nicht verfügbar — starte zuerst den Homepage-Container",
		"homepage_studio.preview_empty_title": "Noch keine Live-Vorschau",
		"homepage_studio.welcome":             "Willkommen im Homepage-Studio! Beschreibe die Website, die du erstellen möchtest, und ich erstelle sie für dich.",
	} {
		if values[key] != want {
			t.Fatalf("%s = %q, want %q", key, values[key], want)
		}
	}
}
