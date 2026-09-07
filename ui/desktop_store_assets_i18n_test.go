package ui

import (
	"strings"
	"testing"
)

func TestDesktopStoreAssetsI18n(t *testing.T) {
	t.Parallel()

	preview := readDesktopAssetText(t, "js/desktop/apps/store-terminal-preview.js")
	if !strings.Contains(preview, "t('desktop.store_terminal_load_failed')") {
		t.Fatal("store terminal-preview asset loads must localize desktop.store_terminal_load_failed")
	}
	if !strings.Contains(preview, "loadStyle('/css/xterm.css', t)") {
		t.Fatal("store terminal-preview must pass t into loadStyle")
	}
	if !strings.Contains(preview, "loadScript('/js/vendor/xterm.min.js', t)") {
		t.Fatal("store terminal-preview must pass t into loadScript")
	}
	if strings.Count(preview, "throw loadFailed(t);") < 2 {
		t.Fatal("store terminal-preview must wrap AuraLazyAssets style and script loads")
	}
	if !strings.Contains(preview, "link.onerror = () => reject(loadFailed(t))") {
		t.Fatal("store terminal-preview stylesheet fallback must reject with store_terminal_load_failed")
	}
	if !strings.Contains(preview, "script.onerror = () => reject(loadFailed(t))") {
		t.Fatal("store terminal-preview script fallback must reject with store_terminal_load_failed")
	}
	for _, forbidden := range []string{
		"Failed to load stylesheet: ",
		"Failed to load script: ",
		"'Failed to load stylesheet: ' + href",
		"'Failed to load script: ' + src",
	} {
		if strings.Contains(preview, forbidden) {
			t.Fatalf("store terminal-preview still hardcodes %q", forbidden)
		}
	}
}
