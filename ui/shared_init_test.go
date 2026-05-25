package ui

import (
	"strings"
	"testing"
)

func TestSharedComponentsDoNotWarnForExpectedDesktopInitializationStates(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "shared.js")
	for _, noisyMarker := range []string{
		"Radial menu already initialized",
		"Theme toggle button not found",
	} {
		if strings.Contains(source, noisyMarker) {
			t.Fatalf("shared.js should not log expected desktop initialization state %q", noisyMarker)
		}
	}
}
