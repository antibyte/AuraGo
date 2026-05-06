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
			"'code-studio': 'CodeStudioApp'",
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
