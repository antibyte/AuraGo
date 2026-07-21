package ui

import (
	"strings"
	"testing"
)

func TestDesktopNetworkCamerasInteractionContracts(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/network-cameras.js")
	for _, marker := range []string{
		"const iconAliases",
		"vd-sprite-icon nc-glyph",
		"nc-discovery-progress",
		`role="status" aria-live="polite"`,
		`data-delete="`,
		"async function deleteStream",
		"if (ids.length >= 4) break",
		"const liveGrid = state.mode === 'live'",
		"(liveGrid ? '' : detailMarkup(state))",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("network-cameras.js missing interaction contract %q", marker)
		}
	}
	if strings.Contains(source, "liveIDs.has(stream.id) && !selected") {
		t.Fatal("selected camera must remain live when the dedicated live grid is active")
	}
}

func TestDesktopNetworkCamerasStylesExposeProgressAndLiveGrid(t *testing.T) {
	t.Parallel()

	styles := readDesktopAssetText(t, "css/desktop-app-network-cameras.css")
	for _, marker := range []string{
		".nc-layout.is-live-grid",
		".nc-card-admin-actions",
		".nc-discovery-progress",
		".nc-spinner.is-small",
	} {
		if !strings.Contains(styles, marker) {
			t.Fatalf("network camera styles missing %q", marker)
		}
	}
}
