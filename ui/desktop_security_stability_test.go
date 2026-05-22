package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesktopGeneratedAppSandboxDisallowsPopups(t *testing.T) {
	t.Parallel()

	mainText := readDesktopAssetText(t, "js/desktop/main.js")
	if strings.Contains(mainText, "allow-popups") {
		t.Fatal("generated desktop iframes must not allow popups")
	}
	for _, want := range []string{
		`const sandboxFlags = ['allow-scripts', 'allow-forms', 'allow-modals'];`,
		`if (options && options.allowSameOrigin) sandboxFlags.push('allow-same-origin');`,
		`if (options && options.allowDownloads) sandboxFlags.push('allow-downloads');`,
		`iframe.setAttribute('sandbox', sandboxFlags.join(' '));`,
		`const cleanup = state.windowCleanups.get(win.id)`,
		`function registerWindowCleanup(windowId, cleanup)`,
		`function renderAppError`,
		`catch (err)`,
		`if (state.z > 100000) normalizeWindowZIndexes();`,
		`document.addEventListener('keydown', closeContextMenuOnEscape)`,
	} {
		if !strings.Contains(mainText, want) {
			t.Fatalf("desktop main script missing stability marker %q", want)
		}
	}
	body := jsFunctionBodyInWindowMenuTest(t, mainText, "function renderGeneratedApp(id, appId)")
	if strings.Contains(body, "allowSameOrigin") || strings.Contains(body, "allow-same-origin") {
		t.Fatal("generated desktop app iframes must keep an opaque sandbox origin so apps cannot fetch desktop APIs directly")
	}

	if strings.Contains(readDesktopAssetText(t, "desktop.html"), `id="radialMenuAnchor"`) {
		t.Fatal("desktop HTML should not keep the unused radial menu anchor")
	}
}

func TestDesktopStoreAppFramesAllowInteractiveWebAppBrowserFeatures(t *testing.T) {
	t.Parallel()

	mainText := readDesktopAssetText(t, "js/desktop/main.js")
	for _, want := range []string{
		`const frameURL = cacheBustURL(storeFrameURL(body.url, storeAppId), 'aurago_store_embed');`,
		`function storeFrameURL(src, storeAppId)`,
		`if (storeAppId === 'uptime-kuma')`,
		`function cacheBustURL(src, paramName)`,
		`const frame = makeSandboxedFrame(frameURL, app.id, '', id, 'vd-generated-frame vd-store-app-frame', appName(app), { allowSameOrigin: true, allowDownloads: true, allowStorageAccess: true, allowTopNavigationByUserActivation: true, allowPointerLock: true, allowFullscreen: true, allowGamepad: true });`,
		`if (options && options.allowStorageAccess) sandboxFlags.push('allow-storage-access-by-user-activation');`,
		`if (options && options.allowTopNavigationByUserActivation) sandboxFlags.push('allow-top-navigation-by-user-activation');`,
		`if (options && options.allowPointerLock) sandboxFlags.push('allow-pointer-lock');`,
		`if (options && options.allowFullscreen) iframe.setAttribute('allowfullscreen', '');`,
		`if (options && options.allowGamepad) allowParts.push('gamepad');`,
	} {
		if !strings.Contains(mainText, want) {
			t.Fatalf("desktop store app iframe missing browser feature marker %q", want)
		}
	}

	body := jsFunctionBodyInWindowMenuTest(t, mainText, "function renderContainerWebApp(id, app)")
	if strings.Contains(body, "allow-popups") {
		t.Fatal("store app frames must not allow popups")
	}
}

func TestDesktopWorkspaceCSPDisallowsPopupsAndTightensBaseAndObjects(t *testing.T) {
	t.Parallel()

	sourceBytes, err := os.ReadFile(filepath.Join("..", "internal", "server", "server_routes_ui.go"))
	if err != nil {
		t.Fatalf("read server_routes_ui.go: %v", err)
	}
	source := string(sourceBytes)
	if strings.Contains(source, "allow-popups") {
		t.Fatal("desktop workspace CSP must not allow popups")
	}
	for _, want := range []string{
		"const desktopAppWorkspaceCSP",
		"sandbox allow-scripts allow-forms allow-modals",
		"const desktopWidgetWorkspaceCSP",
		"sandbox allow-scripts allow-forms allow-modals",
		"object-src 'none'",
		"base-uri 'none'",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("desktop workspace CSP missing %q", want)
		}
	}
}

func TestFileManagerSelectionClickAvoidsFullRerenderAndChecksUploadSize(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/file-manager.js")
	body := jsFunctionBodyInWindowMenuTest(t, source, "function handleItemClick(e)")
	if strings.Contains(body, "renderAll()") {
		t.Fatal("simple file-manager item selection must not full-render the file manager")
	}
	for _, want := range []string{
		"function updateSelectionDOM()",
		"updateSelectionDOM();",
		"maxFileSize()",
		"file.size > limit",
		"desktop.fm.upload_too_large",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("file manager missing selection/upload marker %q", want)
		}
	}
}

func TestDesktopQuickConnectWarnsAndCleansUpSSHResources(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/main.js")
	body := jsFunctionBodyInWindowMenuTest(t, source, "function renderQuickConnect(id)")
	for _, want := range []string{
		"registerWindowCleanup(id, () =>",
		"activeWS.close()",
		"activeTerm.dispose()",
		"activeResizeObserver.disconnect()",
		"msg.type === 'warning'",
		"msg.code === 'insecure_host_key'",
		"desktop.qc_host_key_warning",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("quick connect missing warning/cleanup marker %q", want)
		}
	}
}

func TestDesktopCalculatorRejectsZeroDivisorsBeforeNonFiniteResult(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/main.js")
	for _, want := range []string{
		"function rejectZeroDivisor(operator, right)",
		"rejectZeroDivisor(operator, right);",
		"operator === '/' || operator === '%'",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("calculator missing zero-divisor marker %q", want)
		}
	}
}
