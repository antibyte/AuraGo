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
		"root.addEventListener('pointerdown'",
		"fm.activeKeyboardWindow = fm.windowId",
		"fm.activeKeyboardWindow !== fm.windowId",
		"root.contains(document.activeElement)",
		"function focusFileItem(path)",
		"focusFileItem(path);",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("file manager keyboard shortcut scoping missing marker %q", marker)
		}
	}
}

func TestFileManagerToolbarAndContextMenuCleanup(t *testing.T) {
	t.Parallel()

	jsBytes, err := Content.ReadFile("js/desktop/file-manager.js")
	if err != nil {
		t.Fatalf("file manager script missing from embedded UI: %v", err)
	}
	source := string(jsBytes)
	if count := strings.Count(source, "function updateToolbarState()"); count != 1 {
		t.Fatalf("file manager toolbar updater count = %d, want 1", count)
	}
	for _, marker := range []string{
		"Math.max(8,",
		"menuRect.left < 8",
		"menuRect.top < 8",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("file manager context menu clamp missing marker %q", marker)
		}
	}
}

func TestFileManagerMobileInteractionMarkers(t *testing.T) {
	t.Parallel()

	jsBytes, err := Content.ReadFile("js/desktop/file-manager.js")
	if err != nil {
		t.Fatalf("file manager script missing from embedded UI: %v", err)
	}
	source := string(jsBytes)
	for _, marker := range []string{
		"function isTouchLikePointer(event)",
		"function wireLongPress(element, callback, options)",
		"function openFileItem(path, type)",
		"function handleSidebarToggle()",
		"fm.sidebarOpen",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("file manager mobile interaction missing marker %q", marker)
		}
	}
}
