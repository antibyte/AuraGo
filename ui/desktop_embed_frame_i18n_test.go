package ui

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesktopEmbedFrameI18n(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/quickconnect-launchpad-chat.js")
	for _, want := range []string{
		"title || appId || t('desktop.embed_frame_title')",
		"value || t('desktop.embed_bridge_failed')",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("embed frame i18n missing marker %q", want)
		}
	}
	for _, forbidden := range []string{
		"|| 'Aura Desktop app'",
		"|| 'Desktop bridge request failed'",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("embed frame still hardcodes %q", forbidden)
		}
	}

	sdk := readDesktopAssetText(t, "js/desktop/aura-desktop-sdk.js")
	if strings.Contains(sdk, "desktop.embed_") {
		t.Fatal("aura-desktop-sdk must not take desktop embed i18n keys")
	}

	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		path := filepath.ToSlash(filepath.Join("lang", "desktop", lang+".json"))
		var values map[string]string
		if err := json.Unmarshal([]byte(rawDesktopAssetText(t, path)), &values); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, key := range []string{"desktop.embed_frame_title", "desktop.embed_bridge_failed"} {
			if strings.TrimSpace(values[key]) == "" {
				t.Fatalf("%s missing non-empty %s", path, key)
			}
		}
		if lang == "de" && values["desktop.embed_frame_title"] == "Aura Desktop app" {
			t.Fatalf("%s must not copy the English embed frame title", path)
		}
		if lang == "fr" && values["desktop.embed_bridge_failed"] == "Desktop bridge request failed" {
			t.Fatalf("%s must not copy the English embed bridge error", path)
		}
	}
}
