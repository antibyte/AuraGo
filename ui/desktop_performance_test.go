package ui

import (
	"strings"
	"testing"
)

func TestDesktopModuleLoaderUsesAsyncOrderedFetch(t *testing.T) {
	t.Parallel()

	loader := rawDesktopAssetText(t, "js/desktop/core/module-loader.js")
	if strings.Contains(loader, "xhr.open('GET', part, false)") {
		t.Fatal("desktop module loader must not use synchronous XHR")
	}
	for _, marker := range []string{
		"function fetchScriptPart(part)",
		"Promise.all(parts.map(fetchScriptPart))",
		"sources.map(source => '\\n;' + source).join('')",
		"auradesktop:module-loaded",
	} {
		if !strings.Contains(loader, marker) {
			t.Fatalf("desktop module loader missing async ordered fetch marker %q", marker)
		}
	}

	for _, path := range []string{
		"js/desktop/main.js",
		"js/desktop/file-manager.js",
		"js/desktop/apps/code-studio.js",
	} {
		wrapper := rawDesktopAssetText(t, path)
		if !strings.Contains(wrapper, ".catch(err => console.error") {
			t.Fatalf("%s should surface async module load errors", path)
		}
	}

	main := readDesktopAssetText(t, "js/desktop/main.js")
	if !strings.Contains(main, "document.readyState === 'loading'") {
		t.Fatal("desktop init must handle async module evaluation after DOMContentLoaded")
	}
}

func TestDesktopPointerHotPathsUseAnimationFrames(t *testing.T) {
	t.Parallel()

	main := readDesktopAssetText(t, "js/desktop/main.js")
	for _, marker := range []string{
		"function scheduleWindowPointerFrame",
		"function cancelWindowPointerFrame",
		"window.requestAnimationFrame",
		"cancelWindowPointerFrame(drag, true)",
		"cancelWindowPointerFrame(resize, true)",
	} {
		if !strings.Contains(main, marker) {
			t.Fatalf("desktop pointer hot path missing RAF marker %q", marker)
		}
	}
}

func TestDesktopPerformanceCachesAppsAndBatchesChat(t *testing.T) {
	t.Parallel()

	main := readDesktopAssetText(t, "js/desktop/main.js")
	for _, marker := range []string{
		"appsCacheBootstrap",
		"state.appsCache",
		"state.appsCache = [...(boot.builtin_apps || []), ...(boot.installed_apps || [])]",
	} {
		if !strings.Contains(main, marker) {
			t.Fatalf("desktop app cache missing marker %q", marker)
		}
	}

	chat := readDesktopAssetText(t, "js/desktop/apps/quickconnect-launchpad-chat.js")
	for _, marker := range []string{
		"let streamTextFrame = 0",
		"function queueStreamingBubbleFlush",
		"window.requestAnimationFrame",
		"flushStreamingBubble",
	} {
		if !strings.Contains(chat, marker) {
			t.Fatalf("desktop chat stream batching missing marker %q", marker)
		}
	}
}

func TestFileManagerIncrementalLargeDirectoryRendering(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/file-manager.js")
	for _, marker := range []string{
		"FILE_RENDER_BATCH_SIZE",
		"FILE_INCREMENTAL_THRESHOLD",
		"function scheduleIncrementalFileRender",
		"insertAdjacentHTML('beforeend'",
		"data-fm-incremental",
		"dataset.fmBound",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("file manager incremental rendering missing marker %q", marker)
		}
	}
}

func rawDesktopAssetText(t *testing.T, path string) string {
	t.Helper()
	data, err := Content.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}
