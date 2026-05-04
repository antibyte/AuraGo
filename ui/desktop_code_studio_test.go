package ui

import (
	"strings"
	"testing"
)

func TestCodeStudioUsesPerWindowStateAndClosesTerminal(t *testing.T) {
	t.Parallel()

	jsBytes, err := Content.ReadFile("js/desktop/apps/code-studio.js")
	if err != nil {
		t.Fatalf("Code Studio app missing from embedded UI: %v", err)
	}
	source := string(jsBytes)
	for _, marker := range []string{
		"const instances = new Map()",
		"function createInstance",
		"instances.set(windowId, instance)",
		"CodeStudioApp.dispose",
		"instance.ws.close()",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("Code Studio per-window lifecycle missing marker %q", marker)
		}
	}
	if !strings.Contains(source, "window.CodeStudio = window.CodeStudioApp") {
		t.Fatalf("Code Studio compatibility export missing")
	}
}
