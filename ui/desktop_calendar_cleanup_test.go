package ui

import (
	"strings"
	"testing"
)

func TestDesktopCalendarWindowCleanupRemovesModals(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/calendar.js")
	for _, marker := range []string{
		"registerWindowCleanup(id",
		"vd-modal-backdrop",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("calendar.js missing cleanup marker %q", marker)
		}
	}
}