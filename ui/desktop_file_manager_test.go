package ui

import (
	"strings"
	"testing"
)

func TestFileManagerInlineRenameMarkers(t *testing.T) {
	t.Parallel()

	jsBytes, err := Content.ReadFile("js/desktop/file-manager.js")
	if err != nil {
		t.Fatalf("file manager script missing from embedded UI: %v", err)
	}
	source := string(jsBytes)
	for _, marker := range []string{
		"data-rename-input",
		"finishRename",
		"cancelRename",
		"fm.renamePath === file.path",
		"event.key === 'Enter'",
		"event.key === 'Escape'",
		"renameInput.addEventListener('blur'",
		"method: 'PATCH'",
		"old_path: path",
		"new_path: nextPath",
		"/api/desktop/file",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("file manager rename missing marker %q", marker)
		}
	}

	cssBytes, err := Content.ReadFile("css/desktop.css")
	if err != nil {
		t.Fatalf("desktop stylesheet missing from embedded UI: %v", err)
	}
	if !strings.Contains(string(cssBytes), ".fm-rename-input") {
		t.Fatalf("desktop stylesheet missing file manager rename input rule")
	}
}

func TestFileManagerKeyboardShortcutsAreInstanceScoped(t *testing.T) {
	t.Parallel()

	jsBytes, err := Content.ReadFile("js/desktop/file-manager.js")
	if err != nil {
		t.Fatalf("file manager script missing from embedded UI: %v", err)
	}
	source := string(jsBytes)
	if strings.Contains(source, "document.activeElement === document.body") {
		t.Fatalf("file manager keyboard shortcuts must not run from body focus")
	}
	for _, marker := range []string{
		"fm.activeKeyboardWindow",
		"root.addEventListener('focusin'",
		"root.contains(document.activeElement)",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("file manager keyboard shortcut scoping missing marker %q", marker)
		}
	}
}
