package ui

import (
	"strings"
	"testing"
)

func TestDesktopMainRendersDesktopDirectoryEntries(t *testing.T) {
	t.Parallel()

	data, err := Content.ReadFile("js/desktop/main.js")
	if err != nil {
		t.Fatalf("desktop main script missing from embedded UI: %v", err)
	}
	script := string(data)
	for _, want := range []string{
		"state.desktopFiles = await loadDesktopFiles()",
		"/api/desktop/files?path=Desktop",
		"desktop-entry-' + file.path",
		"data-desktop-entry",
		"btn.dataset.kind === 'file'",
		"method: 'PUT'",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("desktop main script missing desktop file rendering behavior %q", want)
		}
	}
}
