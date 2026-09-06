package ui

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesktopHostErrorI18n(t *testing.T) {
	t.Parallel()

	store := readDesktopAssetText(t, "js/desktop/apps/quickconnect-launchpad-chat.js")
	for _, want := range []string{
		"t('desktop.store_terminal_load_failed')",
		"script.onerror = () => reject(loadFailed())",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("store terminal load i18n missing marker %q", want)
		}
	}
	if strings.Contains(store, "Failed to load store terminal preview module") {
		t.Fatal("store terminal load still hardcodes Failed to load store terminal preview module")
	}

	host := readDesktopAssetText(t, "js/desktop/core/sdk-events-bootstrap.js")
	for _, want := range []string{
		"t('desktop.clipboard_read_unavailable')",
		"t('desktop.clipboard_write_unavailable')",
	} {
		if !strings.Contains(host, want) {
			t.Fatalf("host clipboard i18n missing marker %q", want)
		}
	}
	for _, forbidden := range []string{
		"Clipboard read is not available.",
		"Clipboard write is not available.",
	} {
		if strings.Contains(host, forbidden) {
			t.Fatalf("host clipboard still hardcodes %q", forbidden)
		}
	}

	sdk := readDesktopAssetText(t, "js/desktop/aura-desktop-sdk.js")
	if strings.Contains(sdk, "desktop.clipboard_") || strings.Contains(sdk, "desktop.store_terminal_load_failed") {
		t.Fatal("aura-desktop-sdk must not take host clipboard or store load i18n keys")
	}

	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		path := filepath.ToSlash(filepath.Join("lang", "desktop", lang+".json"))
		var values map[string]string
		if err := json.Unmarshal([]byte(rawDesktopAssetText(t, path)), &values); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, key := range []string{
			"desktop.store_terminal_load_failed",
			"desktop.clipboard_read_unavailable",
			"desktop.clipboard_write_unavailable",
		} {
			if strings.TrimSpace(values[key]) == "" {
				t.Fatalf("%s missing non-empty %s", path, key)
			}
		}
		if lang == "de" && values["desktop.store_terminal_load_failed"] == "Failed to load the terminal preview module." {
			t.Fatalf("%s must not copy the English store load error", path)
		}
		if lang == "fr" && values["desktop.clipboard_read_unavailable"] == "Clipboard read is not available." {
			t.Fatalf("%s must not copy the English clipboard read error", path)
		}
		if lang == "de" && values["desktop.clipboard_write_unavailable"] == "Clipboard write is not available." {
			t.Fatalf("%s must not copy the English clipboard write error", path)
		}
	}
}
