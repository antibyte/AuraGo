package ui

import (
	"strings"
	"testing"
)

func TestDesktopGalleryUsesStableThumbnailDimensions(t *testing.T) {
	t.Parallel()

	data, err := Content.ReadFile("css/desktop.css")
	if err != nil {
		t.Fatalf("desktop stylesheet missing from embedded UI: %v", err)
	}
	css := string(data)
	for _, want := range []string{
		"grid-auto-rows: minmax(176px, auto);",
		"min-height: 176px;",
		"flex: 0 0 126px;",
		"height: 126px;",
		"min-height: 126px;",
	} {
		if !strings.Contains(css, want) {
			t.Fatalf("desktop gallery stylesheet missing stable thumbnail rule %q", want)
		}
	}
}
