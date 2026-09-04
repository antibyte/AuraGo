package ui

import (
	"strings"
	"testing"
)

func TestDesktopVirtualComputersVolumeTtlI18n(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/virtual-computers.js")
	for _, want := range []string{
		"formatDuration(86400, c)",
		"formatDuration(604800, c)",
		"formatDuration(2592000, c)",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("virtual computers volume TTL i18n missing marker %q", want)
		}
	}
	for _, forbidden := range []string{
		">1 d<",
		">7 d<",
		">30 d<",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("virtual computers still hardcodes %q", forbidden)
		}
	}
}
