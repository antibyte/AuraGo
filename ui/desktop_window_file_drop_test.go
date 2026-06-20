package ui

import (
	"strings"
	"testing"
)

func TestDesktopFileOpsExposeWindowDropHelpers(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/main.js")
	for _, marker := range []string{
		"function desktopFilePathInfo(path)",
		"hasDragPayload: hasDesktopFileDragPayload",
		"readDragPayload: desktopFileDragPayload",
		"pathInfo: desktopFilePathInfo",
		"dragType: DESKTOP_FILE_DRAG_TYPE",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("desktop file ops missing shared drop helper marker %q", marker)
		}
	}
}

func TestDesktopFileEntriesKeepNativeDragForWindowDrops(t *testing.T) {
	t.Parallel()

	foundation := rawDesktopAssetText(t, "js/desktop/core/desktop-foundation.js")
	fileDrops := rawDesktopAssetText(t, "js/desktop/core/desktop-file-drops.js")
	for _, marker := range []string{
		"wireDraggableIcon(btn);",
		"wireDesktopFileIconDrag(btn)",
		"if (btn.dataset.desktopEntry !== 'true') event.preventDefault();",
	} {
		if !strings.Contains(foundation, marker) {
			t.Fatalf("desktop icon runtime must preserve native drag for desktop file entries; missing %q", marker)
		}
	}
	for _, marker := range []string{
		"btn.draggable = true;",
		"event.dataTransfer.effectAllowed = 'copyMove';",
		"event.dataTransfer.setData(DESKTOP_FILE_DRAG_TYPE",
	} {
		if !strings.Contains(fileDrops, marker) {
			t.Fatalf("desktop file drag runtime missing native drag payload marker %q", marker)
		}
	}
}

func TestDesktopFileDragSourcesAllowCopyWindowDrops(t *testing.T) {
	t.Parallel()

	for _, asset := range []string{
		"js/desktop/core/desktop-file-drops.js",
		"js/desktop/file-manager/actions-operations.js",
	} {
		source := rawDesktopAssetText(t, asset)
		if !strings.Contains(source, "effectAllowed = 'copyMove'") {
			t.Fatalf("%s must allow copyMove so copy drop targets like zipper do not show a blocked cursor", asset)
		}
	}
}

func TestDesktopWindowDropCapabilityMapIsCentralized(t *testing.T) {
	t.Parallel()

	source := rawDesktopAssetText(t, "js/desktop/core/desktop-window-file-drops.js")
	for _, marker := range []string{
		"const DESKTOP_WINDOW_DROP_CAPABILITIES",
		"files: { multiple: true",
		"pixel: { multiple: false",
		"viewer: { multiple: false",
		"'viewer-3d': { multiple: false",
		"writer: { multiple: false",
		"sheets: { multiple: false",
		"zipper: { multiple: true",
		"accepts: path => !!desktopWindowDropPathInfo(path).name",
		"'code-studio': { multiple: false",
		"editor: { multiple: false",
		"'agent-chat': { multiple: false",
		"if (capability.multiple !== true && paths.length !== 1) return null;",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("desktop window drop capability map missing marker %q", marker)
		}
	}
}

func TestDesktopWindowDropUpdatesTargetWindow(t *testing.T) {
	t.Parallel()

	drops := rawDesktopAssetText(t, "js/desktop/core/desktop-window-file-drops.js")
	routing := rawDesktopAssetText(t, "js/desktop/core/menus-and-routing.js")
	source := drops + "\n" + routing
	for _, marker := range []string{
		"function wireDesktopFileWindowDrop(windowId)",
		"function handleDesktopFileWindowDragOver(event)",
		"function handleDesktopFileWindowDrop(event)",
		"wireDesktopFileWindowDrop(id);",
		"event.dataTransfer.dropEffect = target.effect;",
		"win.element.classList.add(target.accepted ? 'vd-window-file-drop-target' : 'vd-window-file-drop-reject');",
		"if (!DESKTOP_WINDOW_DROP_CAPABILITIES[appId]) return {",
		"if (appId === 'files' && window.FileManager && typeof window.FileManager.dropDesktopFiles === 'function')",
		"renderAppContent(windowId, appId, nextContext);",
		"if (appId === 'agent-chat' && typeof applyChatLaunchContext === 'function')",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("desktop window drop binding missing marker %q", marker)
		}
	}

	body := jsFunctionBodyInWindowMenuTest(t, drops, "function openDesktopFileDropInWindow(windowId, target)")
	if strings.Contains(body, "openApp(appId") || strings.Contains(body, "openApp(target.appId") {
		t.Fatal("desktop window drops must update the target window instead of opening a new app instance")
	}
}

func TestZipperWindowDropCreatesArchiveFromDroppedFiles(t *testing.T) {
	t.Parallel()

	drops := rawDesktopAssetText(t, "js/desktop/core/desktop-window-file-drops.js")
	zipper := readDesktopAssetText(t, "js/desktop/apps/zipper.js")
	for _, marker := range []string{
		"if (appId === 'zipper' && window.ZipperApp && typeof window.ZipperApp.dropDesktopFiles === 'function')",
		"return window.ZipperApp.dropDesktopFiles(windowId, target.paths);",
	} {
		if !strings.Contains(drops, marker) {
			t.Fatalf("desktop window drop runtime must delegate zipper file drops; missing %q", marker)
		}
	}
	for _, marker := range []string{
		"async function createArchiveFromPaths(paths)",
		"body: JSON.stringify({ paths: cleanPaths, dest: dest })",
		"const externalFiles = Array.from((event.dataTransfer && event.dataTransfer.files) || []);",
		"window.ZipperApp.dropDesktopFiles = dropDesktopFiles;",
	} {
		if !strings.Contains(zipper, marker) {
			t.Fatalf("zipper app must create archives from dropped files; missing %q", marker)
		}
	}
}

func TestZipperWindowDropAcceptsHostFilesAndPlainTextPaths(t *testing.T) {
	t.Parallel()

	drops := rawDesktopAssetText(t, "js/desktop/core/desktop-window-file-drops.js")
	zipper := readDesktopAssetText(t, "js/desktop/apps/zipper.js")
	for _, marker := range []string{
		"function desktopWindowHasHostFileDrag(event)",
		"function desktopWindowHasPlainTextDrag(event)",
		"if (appId === 'zipper' && (hostFiles || plainText))",
		"return window.ZipperApp.dropHostFiles(windowId, target.files);",
		"return window.ZipperApp.dropDesktopFiles(windowId, [target.textPath]);",
		"desktopWindowCanHandleDropEvent(event, event.currentTarget.dataset.windowId)",
	} {
		if !strings.Contains(drops, marker) {
			t.Fatalf("desktop window drop runtime must accept zipper host/plain-text drops; missing %q", marker)
		}
	}
	for _, marker := range []string{
		"async function createArchiveFromHostFiles(files)",
		"state.dropHostFiles = createArchiveFromHostFiles;",
		"window.ZipperApp.dropHostFiles = dropHostFiles;",
		"const hasPlainPath = types.includes('text/plain');",
		"hasFileDrag || hasPlainFile || hasPlainPath",
	} {
		if !strings.Contains(zipper, marker) {
			t.Fatalf("zipper app must accept host files and plain-text path drags; missing %q", marker)
		}
	}
}

func TestHostAndPlainTextWindowDropFallbackIsZipperScoped(t *testing.T) {
	t.Parallel()

	source := rawDesktopAssetText(t, "js/desktop/core/desktop-window-file-drops.js")
	for _, marker := range []string{
		"function desktopWindowCanHandleDropEvent(event, windowId)",
		"if (desktopWindowHasDragPayload(event)) return true;",
		"return !!(win && win.appId === 'zipper' && (desktopWindowHasHostFileDrag(event) || desktopWindowHasPlainTextDrag(event)));",
		"desktopWindowCanHandleDropEvent(event, event.currentTarget.dataset.windowId)",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("host/plain-text window drop fallback must be scoped to zipper; missing %q", marker)
		}
	}
}

func TestDesktopWindowDropRunsBeforeAppDropSurfaces(t *testing.T) {
	t.Parallel()

	source := rawDesktopAssetText(t, "js/desktop/core/desktop-window-file-drops.js")
	for _, marker := range []string{
		"const useCapture = win.appId !== 'files';",
		"win.element.addEventListener('dragover', handleDesktopFileWindowDragOver, useCapture);",
		"win.element.addEventListener('dragleave', handleDesktopFileWindowDragLeave, useCapture);",
		"win.element.addEventListener('drop', handleDesktopFileWindowDrop, useCapture);",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("desktop window drops must capture before app drop surfaces while preserving file-manager item drops; missing %q", marker)
		}
	}
}

func TestDesktopChatDropOverlayCleansUpAfterCapturedWindowDrops(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/agent-chat.js")
	for _, marker := range []string{
		"function clearDropOverlay()",
		"host._desktopChatDropCleanup",
		"window.addEventListener('drop', clearDropOverlay, true)",
		"window.addEventListener('dragend', clearDropOverlay, true)",
		"window.removeEventListener('drop', clearDropOverlay, true)",
		"window.removeEventListener('dragend', clearDropOverlay, true)",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("desktop chat drop overlay cleanup missing marker %q", marker)
		}
	}
}

func TestDesktopMainLoadsWindowDropRuntime(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/main.js")
	if !strings.Contains(source, "ui/js/desktop/core/desktop-window-file-drops.js") {
		t.Fatal("desktop main bundle must load the central window file-drop runtime")
	}
}

func TestFileManagerExportsMultiFileWindowDrop(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/file-manager.js")
	for _, marker := range []string{
		"async function dropDesktopFiles(windowId, paths, destPath)",
		"await moveDroppedDesktopFilesToFolder(paths, destPath == null ? fm.currentPath : destPath)",
		"window.FileManager = { render, navigateTo, dropDesktopFiles, dispose }",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("file manager window drop export missing marker %q", marker)
		}
	}
}

func TestFileManagerDropsOntoNonDirectoryItemsFallBackToCurrentFolder(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/file-manager.js")
	dragOverBody := jsFunctionBodyInWindowMenuTest(t, source, "function handleDragOverItem(e)")
	for _, marker := range []string{
		"if (payload && type !== 'directory') {",
		"e.dataTransfer.dropEffect = 'move';",
		"return;",
	} {
		if !strings.Contains(dragOverBody, marker) {
			t.Fatalf("file item dragover must keep desktop/file-manager payloads valid over non-directory items; missing %q", marker)
		}
	}

	dropBody := jsFunctionBodyInWindowMenuTest(t, source, "async function handleItemDrop(e)")
	for _, marker := range []string{
		"if (destType !== 'directory') {",
		"if (payload) await moveDroppedDesktopFilesToFolder(payload.paths, fm.currentPath);",
		"return;",
	} {
		if !strings.Contains(dropBody, marker) {
			t.Fatalf("file item drop must fall back to the current folder for non-directory targets; missing %q", marker)
		}
	}
}

func TestPixelAndZipperUseSharedDesktopDropPayloadHelpers(t *testing.T) {
	t.Parallel()

	for _, asset := range []string{
		"js/desktop/apps/pixel.js",
		"js/desktop/apps/zipper.js",
	} {
		source := readDesktopAssetText(t, asset)
		for _, marker := range []string{
			"const fileOps = ctx.fileOps || window.AuraDesktopFileOps || null;",
			"fileOps && typeof fileOps.hasDragPayload === 'function'",
			"fileOps && typeof fileOps.readDragPayload === 'function'",
		} {
			if !strings.Contains(source, marker) {
				t.Fatalf("%s missing shared desktop drop helper marker %q", asset, marker)
			}
		}
		if strings.Contains(source, "JSON.parse(event.dataTransfer.getData('application/x-aurago-desktop-files')") ||
			strings.Contains(source, "JSON.parse(e.dataTransfer.getData('application/x-aurago-desktop-files')") {
			t.Fatalf("%s still parses desktop file drag payloads directly", asset)
		}
	}
}

func TestDesktopWindowDropStylesAreAvailable(t *testing.T) {
	t.Parallel()

	css := readAllDesktopCSS(t)
	for _, marker := range []string{
		".vd-window.vd-window-file-drop-target",
		".vd-window.vd-window-file-drop-reject",
	} {
		if !strings.Contains(css, marker) {
			t.Fatalf("desktop window drop CSS missing marker %q", marker)
		}
	}
}

func TestDesktopBackgroundAcceptsHostFileDrops(t *testing.T) {
	t.Parallel()

	mainText := readDesktopAssetText(t, "js/desktop/bundles/main.bundle.js")
	for _, marker := range []string{
		"function hasDesktopHostFileDrag(event)",
		"dataTransferHasType(event && event.dataTransfer, 'Files')",
		"async function uploadHostFilesToDesktop(files, clientX, clientY)",
		"form.append('path', 'Desktop')",
		"form.append('unique', '1')",
		"await api('/api/desktop/upload', { method: 'POST', body: form })",
		"saveIconPosition('desktop-entry-' + uploadedPath, iconPos.x, iconPos.y)",
		"await refreshAfterDesktopFileDrop()",
	} {
		if !strings.Contains(mainText, marker) {
			t.Fatalf("desktop background host file drop missing marker %q in main bundle", marker)
		}
	}

	drops := rawDesktopAssetText(t, "js/desktop/core/desktop-file-drops.js")
	dragOverBody := jsFunctionBodyInWindowMenuTest(t, drops, "function handleDesktopFileDragOver(event)")
	for _, marker := range []string{
		"hasDesktopHostFileDrag(event)",
		"event.dataTransfer.dropEffect = hasDesktopHostFileDrag(event) ? 'copy' : 'move'",
	} {
		if !strings.Contains(dragOverBody, marker) {
			t.Fatalf("handleDesktopFileDragOver missing marker %q", marker)
		}
	}

	dropBody := jsFunctionBodyInWindowMenuTest(t, drops, "async function handleDesktopFileDrop(event)")
	for _, marker := range []string{
		"await moveDraggedFilesToDesktop(payload.paths, event.clientX, event.clientY)",
		"await uploadHostFilesToDesktop(event.dataTransfer.files, event.clientX, event.clientY)",
	} {
		if !strings.Contains(dropBody, marker) {
			t.Fatalf("handleDesktopFileDrop missing marker %q", marker)
		}
	}
}
