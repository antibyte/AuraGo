package ui

import (
	"strings"
	"testing"
)

func TestDesktopSDKValidatesMessageOrigin(t *testing.T) {
	t.Parallel()

	sdk, err := Content.ReadFile("js/desktop/aura-desktop-sdk.js")
	if err != nil {
		t.Fatalf("SDK asset missing from embedded UI: %v", err)
	}

	listener := string(sdk)
	start := strings.Index(listener, "window.addEventListener('message'")
	if start < 0 {
		t.Fatal("SDK message listener is missing")
	}
	listener = listener[start:]
	end := strings.Index(listener, "\n    function context()")
	if end < 0 {
		t.Fatal("SDK message listener boundary changed")
	}
	listener = listener[:end]

	for _, want := range []string{
		"event.origin !== window.location.origin",
		"event.source !== window.parent",
		"return;",
	} {
		if !strings.Contains(listener, want) {
			t.Fatalf("SDK message listener does not validate message origin/source marker %q", want)
		}
	}
}
