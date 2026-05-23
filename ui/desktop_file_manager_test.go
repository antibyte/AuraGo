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
	fileManagerSource := readDesktopAssetText(t, "js/desktop/file-manager.js")
	if !strings.Contains(fileManagerSource, "const DESKTOP_FILE_DRAG_TYPE = 'application/x-aurago-desktop-files';") {
		t.Fatal("file manager bundle must define its own desktop file drag payload type")
	}
	for _, marker := range []string{
		"const DESKTOP_FILE_DRAG_TYPE = 'application/x-aurago-desktop-files'",
		"function fileManagerDragPayload(path)",
		"e.dataTransfer.setData(DESKTOP_FILE_DRAG_TYPE, JSON.stringify(fileManagerDragPayload(path)))",
		"function wireDesktopFileIconDrag(btn)",
		"btn.addEventListener('dragstart', event =>",
		"function desktopFileDragPayload(event)",
		"function wireDesktopFileDrops()",
		"function moveDraggedFilesToDesktop(paths, clientX, clientY)",
		"function fileManagerDragPayloadFromEvent(event)",
		"function moveDroppedDesktopFilesToFolder(paths, destPath)",
		"dragType: DESKTOP_FILE_DRAG_TYPE",
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

func TestDesktopFileDragPayloadCanDropOnTrash(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/main.js")
	for _, marker := range []string{
		"function wireDesktopFileTrashDrop(btn)",
		"if (isTrashIcon(btn)) { wireDesktopFileTrashDrop(btn); return; }",
		"btn.addEventListener('dragover', event =>",
		"btn.addEventListener('drop', async event =>",
		"event.stopPropagation()",
		"await movePathToTrash(path)",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("desktop file drag-to-trash integration missing marker %q", marker)
		}
	}
}

func TestDesktopAndFileManagerShareCutCopyPaste(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/main.js") + "\n" + readDesktopAssetText(t, "js/desktop/file-manager.js")
	for _, marker := range []string{
		"window.AuraDesktopFileClipboard",
		"function setDesktopFileClipboard(mode, paths)",
		"function hasDesktopFileClipboard()",
		"async function pasteDesktopFileClipboard(destBase, options)",
		"setDesktopFileClipboard('cut', desktopBatchPaths(btn))",
		"setDesktopFileClipboard('copy', desktopBatchPaths(btn))",
		"{ label: t('desktop.fm.cut', 'Cut'), action: 'cut'",
		"{ label: t('desktop.fm.copy', 'Copy'), action: 'copy'",
		"{ label: t('desktop.fm.paste', 'Paste'), action: 'paste'",
		"pasteDesktopFileClipboard(destBase",
		"desktop.fm.paste",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("desktop shared cut/copy/paste integration missing marker %q", marker)
		}
	}
}

func TestDesktopCoreFileCreationUsesLocalPathJoinHelper(t *testing.T) {
	t.Parallel()

	source := rawDesktopAssetText(t, "js/desktop/core/menus-and-routing.js")
	for _, signature := range []string{
		"async function createFileInPath(basePath)",
		"async function createFolderInPath(basePath)",
		"async function renamePath(path)",
	} {
		body := jsFunctionBodyInWindowMenuTest(t, source, signature)
		if strings.Contains(body, "joinPath(") {
			t.Fatalf("%s must not use the file-manager-local joinPath helper", signature)
		}
		if !strings.Contains(body, "workspaceJoinPath(") {
			t.Fatalf("%s must use workspaceJoinPath from the desktop core runtime", signature)
		}
	}
	if strings.Contains(source, "joinPath(state.filesPath, 'untitled.txt')") {
		t.Fatal("desktop file toolbar must not reference the file-manager-local joinPath helper")
	}
	if !strings.Contains(source, "workspaceJoinPath(state.filesPath, 'untitled.txt')") {
		t.Fatal("desktop file toolbar must use workspaceJoinPath for new file paths")
	}
	settings := rawDesktopAssetText(t, "js/desktop/apps/settings-calculator.js")
	if strings.Contains(settings, "joinPath(path || state.filesPath, 'untitled.txt')") {
		t.Fatal("desktop fallback file menu must not reference the file-manager-local joinPath helper")
	}
	if !strings.Contains(settings, "workspaceJoinPath(path || state.filesPath, 'untitled.txt')") {
		t.Fatal("desktop fallback file menu must use workspaceJoinPath for new file paths")
	}
}

func TestDesktopClipboardPastePreservesFileManagerRootPath(t *testing.T) {
	t.Parallel()

	mainSource := readDesktopAssetText(t, "js/desktop/main.js")
	fileManagerSource := readDesktopAssetText(t, "js/desktop/file-manager.js")
	for _, marker := range []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "desktop paste defaults only when destination is omitted",
			source: mainSource,
			want:   "normalizeDesktopPath(destBase == null ? 'Desktop' : destBase)",
		},
		{
			name:   "file manager paste passes empty root path through",
			source: fileManagerSource,
			want:   "await ops.paste(destBase == null ? fm.currentPath : destBase)",
		},
	} {
		if !strings.Contains(marker.source, marker.want) {
			t.Fatalf("%s missing marker %q", marker.name, marker.want)
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

func TestFileManagerDocumentMountMediaOpensInline(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/main.js") + "\n" + readDesktopAssetText(t, "js/desktop/file-manager.js")
	for _, marker := range []string{
		"function mediaPreviewURL(file)",
		"function mediaDownloadURL(file)",
		"url.searchParams.set('inline', '1')",
		"? `<video controls autoplay src=\"${esc(mediaPreviewURL(file))}\"></video>`",
		"? `<audio controls autoplay src=\"${esc(mediaPreviewURL(file))}\"></audio>`",
		"<a class=\"vd-button\" href=\"${esc(mediaDownloadURL(file))}\" download",
		"if (entry.web_path || entryLooksPlayableMedia(entry)) return openMediaPreview(entry);",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("file manager document-mount media playback missing marker %q", marker)
		}
	}
}

func TestDesktopFilesCanBeAddedOrAskedInAgentChat(t *testing.T) {
	t.Parallel()

	mainSource := readDesktopAssetText(t, "js/desktop/main.js")
	fileManagerSource := readDesktopAssetText(t, "js/desktop/file-manager.js")
	combined := mainSource + "\n" + fileManagerSource
	for _, marker := range []string{
		"function chatFileContextFromEntry(entry)",
		"function addFileContextToChat(file)",
		"function askAgentAboutFile(file)",
		"function chatContextPayload(host)",
		"sendDesktopChatStream(host, message, chatContextPayload(host))",
		"body: JSON.stringify({ message, context })",
		"{ label: t('desktop.fm.add_to_chat', 'Add to chat'), action: 'add-to-chat'",
		"{ label: t('desktop.fm.ask_agent', 'Ask Agent'), action: 'ask-agent'",
		"{ label: t('desktop.fm.add_to_chat', 'Add to chat'), icon: 'chat'",
		"{ label: t('desktop.fm.ask_agent', 'Ask Agent'), icon: 'agent'",
		"openAgentChatForFile(entry)",
		"desktop.chat_ask_file_prompt",
	} {
		if !strings.Contains(combined, marker) {
			t.Fatalf("desktop file agent chat integration missing marker %q", marker)
		}
	}

	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		text := rawDesktopAssetText(t, "lang/desktop/"+lang+".json")
		for _, key := range []string{"desktop.fm.add_to_chat", "desktop.fm.ask_agent", "desktop.chat_file_context", "desktop.chat_ask_file_prompt"} {
			if !strings.Contains(text, `"`+key+`"`) {
				t.Fatalf("%s desktop translations missing %q", lang, key)
			}
		}
	}
}
