package ui

import (
	"strings"
	"testing"
)

func TestDesktopGalleryUsesStableThumbnailDimensions(t *testing.T) {
	t.Parallel()

	css := readAllDesktopCSS(t)
	for _, want := range []string{
		"grid-auto-rows: minmax(208px, auto);",
		"min-height: 208px;",
		"flex: 0 0 126px;",
		"height: 126px;",
		"min-height: 126px;",
	} {
		if !strings.Contains(css, want) {
			t.Fatalf("desktop gallery stylesheet missing stable thumbnail rule %q", want)
		}
	}
}
