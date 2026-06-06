package ui

import (
	"encoding/json"
	"strings"
	"testing"
)

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

func TestHomepageStudioGermanUsesInformalAddress(t *testing.T) {
	t.Parallel()

	var values map[string]string
	if err := json.Unmarshal([]byte(readDesktopAssetText(t, "lang/desktop/de.json")), &values); err != nil {
		t.Fatalf("parse German desktop translations: %v", err)
	}
	for key, want := range map[string]string{
		"homepage_studio.chat_placeholder":    "Beschreibe deine Website-Änderungen...",
		"homepage_studio.preview_unavailable": "Vorschau nicht verfügbar — starte zuerst den Homepage-Container",
		"homepage_studio.welcome":             "Willkommen im Homepage-Studio! Beschreibe die Website, die du erstellen möchtest, und ich erstelle sie für dich.",
	} {
		if values[key] != want {
			t.Fatalf("%s = %q, want %q", key, values[key], want)
		}
	}
}
