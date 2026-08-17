package ui

import (
	"strings"
	"testing"

	"aurago/internal/desktop"
)

func TestDesktopLogViewerRegistrationAndLifecycle(t *testing.T) {
	t.Parallel()

	var found bool
	for _, app := range desktop.BuiltinApps() {
		if app.ID != "log-viewer" {
			continue
		}
		found = true
		if app.Entry != "builtin://log-viewer" || app.Icon != "monitor" {
			t.Fatalf("unexpected log-viewer manifest: %+v", app)
		}
		if !app.DockVisible || !app.StartVisible {
			t.Fatalf("log-viewer should be dock and start visible: %+v", app)
		}
	}
	if !found {
		t.Fatal("log-viewer is not registered as a built-in desktop app")
	}

	loader := readDesktopAssetText(t, "js/desktop/core/module-loader.js")
	for _, marker := range []string{
		"'log-viewer'",
		"/css/desktop-app-log-viewer.css",
		"/js/desktop/apps/log-viewer-filters.js",
		"/js/desktop/apps/log-viewer.js",
	} {
		if !strings.Contains(loader, marker) {
			t.Errorf("desktop lazy loader missing %q", marker)
		}
	}

	app := readDesktopAssetText(t, "js/desktop/apps/log-viewer.js")
	for _, marker := range []string{
		"window.LogViewerApp = { render, dispose }",
		"const instances = new Map()",
		"function render(host, windowId, context)",
		"function dispose(windowId)",
		"new EventSource(url)",
		"state.source.close()",
		"document.removeEventListener('keydown'",
		"desktop.log_viewer_download",
		"textContent",
	} {
		if !strings.Contains(app, marker) {
			t.Errorf("log-viewer.js missing lifecycle marker %q", marker)
		}
	}
	if strings.Contains(app, "innerHTML = record") || strings.Contains(app, "innerHTML = payload") {
		t.Fatal("log-viewer must not inject raw log text as HTML")
	}

	filters := readDesktopAssetText(t, "js/desktop/apps/log-viewer-filters.js")
	for _, marker := range []string{
		"window.LogViewerFilters = { create",
		"function apply(records)",
		"function toggleLevel(level)",
		"function setQuery(query)",
		"function setRegex(enabled)",
		"function serialize()",
		"function deserialize(data)",
	} {
		if !strings.Contains(filters, marker) {
			t.Errorf("log-viewer-filters.js missing marker %q", marker)
		}
	}

	foundation := readDesktopAssetText(t, "js/desktop/core/desktop-foundation.js")
	for _, marker := range []string{
		"'log-viewer': 'monitor'",
		"'log-viewer': 'LogViewerApp'",
		"callAppDispose(window.LogViewerApp, win.id)",
	} {
		if !strings.Contains(foundation, marker) {
			t.Errorf("desktop foundation missing %q", marker)
		}
	}

	routing := readDesktopAssetText(t, "js/desktop/core/menus-and-routing.js")
	if !strings.Contains(routing, "readonly: desktopReadonly()") || !strings.Contains(routing, "window.LogViewerApp.render") {
		t.Fatal("menus-and-routing must dispatch log-viewer with readonly context")
	}
}
