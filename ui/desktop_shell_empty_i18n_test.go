package ui

import (
	"strings"
	"testing"
)

func TestDesktopShellEmptyI18n(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/core/window-shell-runtime.js")
	if strings.Count(source, "t('desktop.load_failed')") < 3 {
		t.Fatal("widget frame, standalone widget, and standalone webamp must localize desktop.load_failed")
	}
	for _, forbidden := range []string{
		"card.innerHTML = `<div class=\"vd-widget-body\">${esc(err.message)}</div>`",
		"host.innerHTML = `<div class=\"vd-empty\">${esc(err.message)}</div>`",
		"message: (err && err.message) || String(err)",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("window shell still dumps err.message via %q", forbidden)
		}
	}
	if !strings.Contains(source, "t('desktop.winamp_unsupported')") {
		t.Fatal("standalone webamp must localize desktop.winamp_unsupported")
	}
	if !strings.Contains(source, "Webamp is not supported in this browser.") {
		t.Fatal("standalone webamp must still map the English unsupported sentinel")
	}
}
