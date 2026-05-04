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
		"function nextWindowPosition",
		"workspaceRect.width",
		"workspaceRect.height",
		"Math.min(maxLeft",
		"Math.min(maxTop",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("window manager placement missing marker %q", marker)
		}
	}
}
