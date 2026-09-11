package ui

import (
	"strings"
	"testing"
)

// The gallery grid must keep a stable geometry while thumbnails load so tiles do
// not reflow: square tiles sized by a CSS variable, cover-fit media and a shimmer
// placeholder that occupies the full tile.
func TestDesktopGalleryUsesStableThumbnailDimensions(t *testing.T) {
	t.Parallel()

	css := strings.ReplaceAll(readAllDesktopCSS(t), "\r\n", "\n")
	for _, want := range []string{
		"--vd-gallery-tile: 168px;",
		"grid-template-columns: repeat(auto-fill, minmax(var(--vd-gallery-tile), 1fr));",
		"aspect-ratio: 1 / 1;",
		".vd-gallery-media {",
		"object-fit: cover;",
		".vd-gallery-thumb::before {",
		".vd-gallery-card.is-loaded .vd-gallery-thumb::before,",
		".vd-gallery-skeleton {",
		"@keyframes vd-gallery-shimmer",
	} {
		if !strings.Contains(css, want) {
			t.Fatalf("desktop gallery stylesheet missing stable thumbnail rule %q", want)
		}
	}
	for _, stale := range []string{
		"grid-auto-rows: minmax(192px, auto);",
		"flex: 0 0 126px;",
	} {
		if strings.Contains(css, stale) {
			t.Fatalf("desktop gallery stylesheet still carries legacy fixed-height thumbnail rule %q", stale)
		}
	}
}

// Tile size is user adjustable; the app must expose the full range through the
// data attribute the stylesheet keys on and persist the preference.
func TestDesktopGalleryTileSizesAreWiredBetweenScriptAndStylesheet(t *testing.T) {
	t.Parallel()

	library := readDesktopAssetText(t, "js/desktop/apps/gallery-library.js")
	if !strings.Contains(library, "TILE_SIZES = [104, 136, 168, 208, 256, 320]") {
		t.Fatal("gallery library must define the six tile sizes used by the stylesheet")
	}
	if !strings.Contains(library, "'aurago.desktop.gallery.v1'") {
		t.Fatal("gallery library must persist view preferences under the versioned storage key")
	}

	app := readDesktopAssetText(t, "js/desktop/apps/gallery.js")
	for _, want := range []string{
		"root.dataset.tileIndex = String(clamped);",
		"root.style.setProperty('--vd-gallery-tile'",
		"savePrefs(",
	} {
		if !strings.Contains(app, want) {
			t.Fatalf("gallery app missing tile size wiring marker %q", want)
		}
	}

	css := strings.ReplaceAll(readAllDesktopCSS(t), "\r\n", "\n")
	for _, want := range []string{
		`.vd-gallery[data-tile-index="0"] .vd-gallery-actions`,
		`.vd-gallery[data-tile-index="1"] .vd-gallery-actions`,
		".vd-gallery-zoom-range {",
		".vd-gallery.is-narrow",
		".vd-gallery.is-tiny",
	} {
		if !strings.Contains(css, want) {
			t.Fatalf("desktop gallery stylesheet missing tile size rule %q", want)
		}
	}
}
