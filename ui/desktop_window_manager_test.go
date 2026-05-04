package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesktopWindowPlacementIsClamped(t *testing.T) {
	sourceBytes, err := os.ReadFile(filepath.Join("js", "desktop", "main.js"))
	if err != nil {
		t.Fatalf("read desktop window manager: %v", err)
	}
	source := string(sourceBytes)
	for _, marker := range []string{
		"function clampWindowSize",
		"function nextWindowPosition",
		"workspaceRect.width",
		"workspaceRect.height",
		"workspaceRect.width - margin * 2",
		"workspaceRect.height - margin * 2",
		"Math.min(maxLeft",
		"Math.min(maxTop",
		"const requestedSize = appWindowSize(appId)",
		"const size = clampWindowSize(requestedSize)",
		"win.style.width = size.width + 'px'",
		"win.style.height = size.height + 'px'",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("window manager placement missing marker %q", marker)
		}
	}
}
