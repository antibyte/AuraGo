package ui

import (
	"strings"
	"testing"
)

func TestDesktopModuleLoaderUsesPrebuiltBundles(t *testing.T) {
	t.Parallel()

	loader := rawDesktopAssetText(t, "js/desktop/core/module-loader.js")
	for _, forbidden := range []string{
		"xhr.open('GET', part, false)",
		"fetchScriptPart(part)",
		"Promise.all(parts.map(fetchScriptPart))",
		"(0, eval)",
		"response.text()",
	} {
		if strings.Contains(loader, forbidden) {
			t.Fatalf("desktop module loader must not use dynamic script assembly marker %q", forbidden)
		}
	}
	for _, marker := range []string{
		"function loadBundle(label, src)",
		"assetLoader().loadScript(src)",
		"aurago:module-loaded",
		"/js/desktop/bundles/main.bundle.js",
		"/js/desktop/bundles/file-manager.bundle.js",
		"/js/desktop/bundles/code-studio.bundle.js",
	} {
		if !strings.Contains(loader, marker) {
			t.Fatalf("desktop module loader missing prebuilt bundle marker %q", marker)
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

	chat := readDesktopAssetText(t, "js/desktop/apps/agent-chat.js")
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
