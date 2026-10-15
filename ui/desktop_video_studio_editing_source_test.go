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
	// Reattach is offered exactly when reattachAudio acts (owned audio clip or an orphaned detached clip).
	if !strings.Contains(string(editing), "E().canReattach(s.project, clip.id)") {
		t.Fatal("clipCapabilities does not offer Reattach through VideoStudioEdit.canReattach")
	}
	if !strings.Contains(string(shell), `action.startsWith('clip-')`) || !strings.Contains(string(shell), "P.linkedIds(s, [id], altKey)") ||
		!strings.Contains(string(shell), "P.linkedIds(s, ids, altKey)") {
		t.Fatal("the shell does not route clip-* actions or select link groups through onSelect and onSelectMany")
	}
}

// TestVideoStudioDeriveWiring checks the derived-media part, the job hook and the load order.
func TestVideoStudioDeriveWiring(t *testing.T) {
	derive, err := os.ReadFile("js/desktop/apps/video-studio-derive.js")
	if err != nil {
		t.Fatalf("the derive part is missing: %v", err)
	}
	if !strings.Contains(string(derive), "Object.assign(P, { reverseClip, freezeFrame, exportFrame, reverseFits })") {
		t.Fatal("video-studio-derive.js does not export its commands")
	}
	jobs, err := os.ReadFile("js/desktop/apps/video-studio-jobs.js")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(jobs), "onSucceeded") {
		t.Fatal("createJob and pollJob do not take the onSucceeded hook")
	}
	loader, err := os.ReadFile("js/desktop/core/module-loader.js")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(loader), "'/js/desktop/apps/video-studio-derive.js', '/js/desktop/apps/video-studio-projects.js'") {
		t.Fatal("module-loader.js does not load video-studio-derive.js directly before video-studio-projects.js")
	}
	// Both browser fixtures load the same scripts, so derive must come before projects there too.
	for _, path := range []string{"desktop_video_studio_browser_test.go", "../internal/server/video_studio_browser_test.go"} {
		fixture, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		deriveAt, projectsAt := strings.Index(string(fixture), "video-studio-derive.js"), strings.Index(string(fixture), "video-studio-projects.js")
		if deriveAt < 0 || projectsAt < 0 || deriveAt > projectsAt {
			t.Fatalf("%s does not load video-studio-derive.js before video-studio-projects.js", path)
		}
	}
}
