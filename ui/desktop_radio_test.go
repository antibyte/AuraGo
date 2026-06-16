package ui

import (
	"strings"
	"testing"
)

func TestDesktopRadioAppMarkers(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/radio.js")
	for _, marker := range []string{
		"window.RadioApp",
		"function dispose(windowId)",
		"aurago.radio.favorites.v1",
		"showStationContextMenu",
		"wireContextMenuBoundary(host)",
		"de1.api.radio-browser.info",
		"/json/stations/bytag/",
		"stopPlayback",
		"disposers.set(windowId",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("radio.js missing marker %q", marker)
		}
	}
}

func TestDesktopRadioDisposeStopsPlayback(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/radio.js")
	if !strings.Contains(source, "disposers.set(windowId, () => {") || !strings.Contains(source, "stopPlayback();") {
		t.Fatal("radio dispose must stop playback and clear timers")
	}
}