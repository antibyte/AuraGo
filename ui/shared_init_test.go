package ui

import (
	"os"
	"strings"
	"testing"
)

func TestSharedComponentsDoNotWarnForExpectedDesktopInitializationStates(t *testing.T) {
	t.Parallel()

	sourceBytes, err := os.ReadFile("shared.js")
	if err != nil {
		t.Fatalf("read shared.js: %v", err)
	}
	source := string(sourceBytes)
	for _, noisyMarker := range []string{
		"Radial menu already initialized",
		"Theme toggle button not found",
	} {
		if strings.Contains(source, noisyMarker) {
			t.Fatalf("shared.js should not log expected desktop initialization state %q", noisyMarker)
		}
	}
}
