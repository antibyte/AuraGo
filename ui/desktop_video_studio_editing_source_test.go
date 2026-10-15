package ui

import (
	"os"
	"strings"
	"testing"
)

// TestVideoStudioEditingCommandsAreWired checks that the 1b commands exist in the editing part and the shell routes to them.
func TestVideoStudioEditingCommandsAreWired(t *testing.T) {
	editing, err := os.ReadFile("js/desktop/apps/video-studio-editing.js")
	if err != nil {
		t.Fatal(err)
	}
	shell, err := os.ReadFile("js/desktop/apps/video-studio.js")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"toggleLink", "detachAudio", "reattachAudio", "setClipSpeed", "commitSpeedField", "toggleDisabled", "setLabel", "clipCapabilities", "clipCommand", "focusSpeed", "linkedIds"} {
		if !strings.Contains(string(editing), "function "+name+"(") {
			t.Fatalf("video-studio-editing.js does not define %s", name)
		}
	}
	if !strings.Contains(string(editing), "key === 'l'") || !strings.Contains(string(editing), "key === 'e'") {
		t.Fatal("editCommand does not map Ctrl+L and Shift+E")
	}
	if !strings.Contains(string(shell), `action.startsWith('clip-')`) || !strings.Contains(string(shell), "P.linkedIds(s, [id], altKey)") ||
		!strings.Contains(string(shell), "P.linkedIds(s, ids, altKey)") {
		t.Fatal("the shell does not route clip-* actions or select link groups through onSelect and onSelectMany")
	}
}
