package ui

import (
	"strings"
	"testing"
)

func TestDesktopMobileTaskbarStaysInVisualViewport(t *testing.T) {
	t.Parallel()

	data, err := Content.ReadFile("css/desktop.css")
	if err != nil {
		t.Fatalf("desktop stylesheet missing from embedded UI: %v", err)
	}
	css := string(data)
	for _, want := range []string{
		"@supports (height: 100dvh)",
		"height: 100dvh;",
		"min-height: 0;",
		"grid-template-rows: minmax(0, 1fr) auto;",
		"env(safe-area-inset-bottom)",
	} {
		if !strings.Contains(css, want) {
			t.Fatalf("desktop mobile stylesheet missing visible taskbar rule %q", want)
		}
	}
}
