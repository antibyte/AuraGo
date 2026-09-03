package ui

import (
	"strings"
	"testing"
)

func TestDesktopZipperPreviewMarkers(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/zipper.js")
	for _, marker := range []string{
		"/api/desktop/archive/entry",
		"function openArchiveMember",
		"function showInlinePreview",
		"openApp('viewer'",
		"openApp('viewer-3d'",
		"archiveEntry: entry.name",
		"forceNew: true",
		"zipper.preview_unsupported",
		"zipper.open_entry",
		"data-preview",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("zipper.js missing marker %q", marker)
		}
	}
}
