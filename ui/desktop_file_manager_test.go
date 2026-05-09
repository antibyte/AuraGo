package ui

import (
	"strings"
	"testing"
)

func TestFileManagerInlineRenameMarkers(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/file-manager.js")
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

	cssText := readAllDesktopCSS(t)
	if !strings.Contains(cssText, ".fm-rename-input") {
		t.Fatalf("desktop stylesheet missing file manager rename input rule")
	}
}

func TestFileManagerKeyboardShortcutsAreInstanceScoped(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/file-manager.js")
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

	source := readDesktopAssetText(t, "js/desktop/file-manager.js")
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

func TestFileManagerContextMenuPreservesThemeIconKeys(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/file-manager.js")
	for _, marker := range []string{
		"icon: item.icon || 'tools'",
		"fallback: contextIconGlyph(item.icon)",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("file manager context menu theme icon conversion missing marker %q", marker)
		}
	}
	if strings.Contains(source, "icon: contextIconGlyph(item.icon)") {
		t.Fatal("file manager context menu must not pass legacy glyphs as icon keys")
	}
}

func TestFileManagerItemsCanDropOntoDesktop(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/main.js") + "\n" + readDesktopAssetText(t, "js/desktop/file-manager.js")
	for _, marker := range []string{
		"const DESKTOP_FILE_DRAG_TYPE = 'application/x-aurago-desktop-files'",
		"function fileManagerDragPayload(path)",
		"e.dataTransfer.setData(DESKTOP_FILE_DRAG_TYPE, JSON.stringify(fileManagerDragPayload(path)))",
		"function desktopFileDragPayload(event)",
		"function wireDesktopFileDrops()",
		"function moveDraggedFilesToDesktop(paths, clientX, clientY)",
		"await api('/api/desktop/file',",
		"body: JSON.stringify({ old_path: src, new_path: newPath })",
		"saveIconPosition('desktop-entry-' + newPath",
		"workspace.addEventListener('drop', handleDesktopFileDrop)",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("desktop file drag-to-desktop integration missing marker %q", marker)
		}
	}
}

func TestFileManagerMobileInteractionMarkers(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/file-manager.js")
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
