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
			"'galaxa-deluxe': 'GalaxaDeluxe'",
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
			"window.FileManager = { render, navigateTo, dropDesktopFiles, dispose }",
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

	main := readDesktopAssetText(t, "js/desktop/main.js")
	orderedParts := []string{
		"ui/js/desktop/core/desktop-foundation.js",
		"ui/js/desktop/core/window-shell-runtime.js",
		"ui/js/desktop/core/lifecycle-cleanup.js",
		"ui/js/desktop/core/widget-autosize-runtime.js",
		"ui/js/desktop/core/menus-and-routing.js",
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

func TestDesktopMainBundleKeepsWidgetDrawerOutsideSDKMenuItemSplit(t *testing.T) {
	t.Parallel()

	bundle := readDesktopAssetText(t, "js/desktop/bundles/main.bundle.js")
	widgetMarker := "/* ui/js/desktop/core/widget-drawer-runtime.js */"
	menusMarker := "/* ui/js/desktop/core/menus-and-routing.js */"
	quickConnectMarker := "/* ui/js/desktop/apps/quickconnect-launchpad-chat.js */"
	bootstrapMarker := "/* ui/js/desktop/core/sdk-events-bootstrap.js */"

	widgetIndex := strings.Index(bundle, widgetMarker)
	menusIndex := strings.Index(bundle, menusMarker)
	quickConnectIndex := strings.Index(bundle, quickConnectMarker)
	bootstrapIndex := strings.Index(bundle, bootstrapMarker)
	if widgetIndex < 0 || menusIndex < 0 || quickConnectIndex < 0 || bootstrapIndex < 0 {
		t.Fatalf("desktop main bundle missing expected split markers: widget=%d menus=%d quickconnect=%d bootstrap=%d", widgetIndex, menusIndex, quickConnectIndex, bootstrapIndex)
	}
	if widgetIndex > menusIndex {
		t.Fatal("widget drawer runtime must load before menus-and-routing because that file starts the renderFiles split")
	}

	renderFilesBody := jsFunctionBodyInWindowMenuTest(t, bundle, "async function renderFiles")
	if strings.Contains(renderFilesBody, "function updateTaskbarSystemButtonsForMobile") {
		t.Fatal("widget drawer helpers must not be nested inside renderFiles; wireChrome needs them in the desktop shell scope")
	}
	sdkMenuBody := jsFunctionBodyInWindowMenuTest(t, bundle, "function sdkMenuItems")
	if strings.Contains(sdkMenuBody, "function updateTaskbarSystemButtonsForMobile") {
		t.Fatal("widget drawer helpers must not be nested inside sdkMenuItems; wireChrome needs them in the desktop shell scope")
	}
}

func TestDesktopFoundationKeepsLifecycleHelpersAvailableForEarlyRender(t *testing.T) {
	t.Parallel()

	foundation := rawDesktopAssetText(t, "js/desktop/core/desktop-foundation.js")
	for _, want := range []string{
		"function disposeAppWindow(win)",
		"function clearWidgetRuntime()",
		"function registerWidgetCleanup(cleanup)",
		"function widgetShouldAutoSize(widget)",
		"function scheduleWidgetAutoSize(card, widget)",
		"function applyWidgetAutoSize(card, payload)",
		"function resizeWidgetToContent(widgetId, payload)",
		"function renderAppError(id, appId, err)",
	} {
		if !strings.Contains(foundation, want) {
			t.Fatalf("desktop foundation missing early lifecycle helper %q", want)
		}
	}
	for _, check := range []struct {
		helper string
		user   string
	}{
		{helper: "function clearWidgetRuntime()", user: "function renderWidgets()"},
		{helper: "function widgetShouldAutoSize(widget)", user: "function renderWidgets()"},
		{helper: "function scheduleWidgetAutoSize(card, widget)", user: "function renderWidgets()"},
		{helper: "function disposeAppWindow(win)", user: "function renderDesktop()"},
	} {
		helperAt := strings.Index(foundation, check.helper)
		userAt := strings.Index(foundation, check.user)
		if helperAt < 0 || userAt < 0 {
			t.Fatalf("desktop foundation cannot compare helper %q and user %q", check.helper, check.user)
		}
		if helperAt > userAt {
			t.Fatalf("desktop foundation defines %q after %q; shell startup can miss it when fragments are cached independently", check.helper, check.user)
		}
	}
}

func TestDesktopStandaloneWidgetFilesOpenAsWidgets(t *testing.T) {
	t.Parallel()

	runtime := rawDesktopAssetText(t, "js/desktop/core/window-shell-runtime.js")
	for _, want := range []string{
		"function isStandaloneWidgetPath(path)",
		"function openStandaloneWidget(path, widgetId, options)",
		"function renderStandaloneWidgetContent(id, path, widgetId, title)",
		"context.standaloneWidget === true",
		"desktopEmbedURL(path, { widget_id: widgetId })",
	} {
		if !strings.Contains(runtime, want) {
			t.Fatalf("desktop window runtime missing standalone widget marker %q", want)
		}
	}

	events := rawDesktopAssetText(t, "js/desktop/core/sdk-events-bootstrap.js")
	for _, want := range []string{
		"event.type === 'open_widget'",
		"openStandaloneWidget(event.payload.path, event.payload.widget_id, event.payload)",
		"isStandaloneWidgetPath(event.payload.path) && !appById(event.payload.app_id)",
	} {
		if !strings.Contains(events, want) {
			t.Fatalf("desktop event bootstrap missing standalone widget marker %q", want)
		}
	}
}
