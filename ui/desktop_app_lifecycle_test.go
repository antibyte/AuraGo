package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesktopAppsExposeDisposeLifecycle(t *testing.T) {
	t.Parallel()

	markers := map[string][]string{
		"ui/js/desktop/main.js": {
			"function appGlobalName",
			"files: 'FileManager'",
			"'code-studio': 'CodeStudioApp'",
			"looper: 'LooperApp'",
			"camera: 'CameraApp'",
			"function appGlobalFallbackName",
			"'code-studio': 'CodeStudio'",
			"function callAppDispose",
			"try {",
			"console.warn('Desktop app dispose failed'",
			"function disposeAppWindow",
			"window[disposeName]",
			"window[fallbackName]",
			"const disposed = callAppDispose",
			"!disposed && fallbackName",
			"closeWindow(id)",
		},
		"ui/js/desktop/file-manager.js": {
			"const instances = new Map()",
			"function createInstance",
			"instances.set(windowId, instance)",
			"function dispose(windowId)",
			"instances.delete(windowId)",
			"window.FileManager = { render, navigateTo, dispose }",
		},
		"ui/js/desktop/apps/sheets.js": {
			"SheetsApp.dispose",
			"closeContextMenu: () => closeSheetContextMenu()",
			"instance.closeContextMenu()",
			"instances.delete(windowId)",
		},
		"ui/js/desktop/apps/writer.js": {
			"WriterApp.dispose",
			"instances.delete(windowId)",
		},
	}

	for path, wants := range markers {
		var source string
		if strings.HasPrefix(path, "ui/js/desktop/") {
			source = readDesktopAssetText(t, strings.TrimPrefix(path, "ui/"))
		} else {
			sourcePath := filepath.FromSlash(path)
			sourceBytes, err := os.ReadFile(sourcePath)
			if err != nil && strings.HasPrefix(path, "ui/") {
				sourcePath = filepath.FromSlash(strings.TrimPrefix(path, "ui/"))
				sourceBytes, err = os.ReadFile(sourcePath)
			}
			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}
			source = string(sourceBytes)
		}
		for _, want := range wants {
			if !strings.Contains(source, want) {
				t.Fatalf("%s missing desktop app lifecycle marker %q", path, want)
			}
		}
	}
}

func TestDesktopMainBundleOrdersSplitShellFragmentsBeforeLifecycleHelpers(t *testing.T) {
	t.Parallel()

	main := rawDesktopAssetText(t, "js/desktop/main.js")
	orderedParts := []string{
		"/js/desktop/core/desktop-foundation.js",
		"/js/desktop/core/window-shell-runtime.js",
		"/js/desktop/core/lifecycle-cleanup.js",
		"/js/desktop/core/widget-autosize-runtime.js",
		"/js/desktop/core/menus-and-routing.js",
	}
	last := -1
	for _, part := range orderedParts {
		index := strings.Index(main, part)
		if index < 0 {
			t.Fatalf("desktop main bundle missing script part %s", part)
		}
		if index <= last {
			t.Fatalf("desktop main bundle loads %s before the preceding split-shell dependency", part)
		}
		last = index
	}
}
