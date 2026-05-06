package ui

import (
	"strings"
	"testing"
)

func TestVirtualDesktopPolishRegressions(t *testing.T) {
	t.Parallel()

	writerBytes, err := Content.ReadFile("js/desktop/apps/writer.js")
	if err != nil {
		t.Fatalf("desktop writer app missing from embedded UI: %v", err)
	}
	sheetsBytes, err := Content.ReadFile("js/desktop/apps/sheets.js")
	if err != nil {
		t.Fatalf("desktop sheets app missing from embedded UI: %v", err)
	}

	source := readDesktopAssetText(t, "js/desktop/main.js") + "\n" + string(writerBytes) + "\n" + string(sheetsBytes)
	for _, marker := range []string{
		"function clampDesktopIconPosition",
		"case 'Delete':",
		"case 'F2':",
		"setTimeout(() => clearSaveError",
		"function setCellFromInput",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("virtual desktop polish regression marker missing %q", marker)
		}
	}
}
