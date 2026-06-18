package ui

import (
	"strings"
	"testing"
)

func storeTerminalPreviewSource(t *testing.T) string {
	t.Helper()
	return readEmbeddedText(t, "js/desktop/apps/store-terminal-preview.js")
}

func TestDesktopStoreTerminalPreviewSupportsClipboardPaste(t *testing.T) {
	t.Parallel()

	source := storeTerminalPreviewSource(t)
	for _, marker := range []string{
		"data-store-terminal-copy",
		"data-store-terminal-paste",
		"navigator.clipboard.writeText",
		"navigator.clipboard.readText",
		"terminalPasteHandler = event =>",
		"host.addEventListener('paste', terminalPasteHandler, true)",
		"host.removeEventListener('paste', terminalPasteHandler, true)",
		"const writeTerminalInput = (session, text) =>",
		"terminal.getSelection",
		"terminalInputEncoder.encode(text)",
		"String(event.key || '').toLowerCase() !== 'v'",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("store terminal preview missing clipboard paste marker %q", marker)
		}
	}
}

func TestDesktopStoreTerminalPreviewHasPasteToolbarStyles(t *testing.T) {
	t.Parallel()

	css := readEmbeddedText(t, "css/desktop-windows.css")
	for _, marker := range []string{
		".vd-store-terminal-shell",
		".vd-store-terminal-meta",
		".vd-store-terminal-toolbar",
		".vd-store-terminal-action",
		".vd-store-terminal-surface",
		"grid-template-rows: auto minmax(0, 1fr)",
	} {
		if !strings.Contains(css, marker) {
			t.Fatalf("store terminal preview CSS missing paste toolbar marker %q", marker)
		}
	}
}

func TestDesktopStoreTerminalPreviewSupportsMultipleSessions(t *testing.T) {
	t.Parallel()

	source := storeTerminalPreviewSource(t)
	for _, marker := range []string{
		"const terminalSessions = new Map()",
		"let activeTerminalSessionID = ''",
		"data-store-terminal-tabs",
		"data-store-terminal-new",
		"function createTerminalSession(options)",
		"function activateTerminalSession(sessionID)",
		"function closeTerminalSession(sessionID)",
		"if (!session || session.bootstrap) return",
		"createTerminalSession({ bootstrap: true })",
		"createTerminalSession({ bootstrap: false })",
		"?bootstrap=1",
		"desktop.store_terminal_bootstrap_session",
		"terminalSessions.forEach(session => session.cleanup())",
		"session.socket = new WebSocket",
		"session.terminal.open(session.surface)",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("store terminal preview missing multi-session marker %q", marker)
		}
	}
}

func TestDesktopStoreTerminalPreviewSupportsResizableSplit(t *testing.T) {
	t.Parallel()

	source := storeTerminalPreviewSource(t)
	for _, marker := range []string{
		"data-store-terminal-resizer",
		"const terminalPreview = host.querySelector('.vd-store-terminal-preview')",
		"function setTerminalPaneWidthPct(widthPct)",
		"function startTerminalPreviewResize(event)",
		"terminalPreview.getBoundingClientRect()",
		"window.addEventListener('pointermove', resizeMoveHandler)",
		"window.removeEventListener('pointermove', resizeMoveHandler)",
		"terminalPreview.style.setProperty('--store-terminal-width'",
		"resizer.setPointerCapture(event.pointerId)",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("store terminal preview missing resizable split marker %q", marker)
		}
	}

	css := readEmbeddedText(t, "css/desktop-windows.css")
	for _, marker := range []string{
		"--store-terminal-width: 42%",
		"grid-template-columns: minmax(280px, var(--store-terminal-width)) 6px minmax(320px, 1fr)",
		".vd-store-terminal-resizer",
		"cursor: col-resize",
		".vd-store-terminal-preview .vd-store-app-frame",
		"background: #05070a",
	} {
		if !strings.Contains(css, marker) {
			t.Fatalf("store terminal preview CSS missing resizable split marker %q", marker)
		}
	}
}

func TestDesktopStoreTerminalPreviewKeepsTerminalFocusWhenPreviewLoads(t *testing.T) {
	t.Parallel()

	source := storeTerminalPreviewSource(t)
	for _, marker := range []string{
		"function refocusActiveTerminalAfterPreviewLoad()",
		"frame.addEventListener('load', refocusActiveTerminalAfterPreviewLoad)",
		"disableAutoFocus: true",
		"session.terminal.focus();",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("store terminal preview missing preview focus retention marker %q", marker)
		}
	}
}

func TestDesktopStoreTerminalPreviewUsesStablePlaceholderAndToggle(t *testing.T) {
	t.Parallel()

	source := storeTerminalPreviewSource(t)
	for _, marker := range []string{
		"data-store-preview-toggle",
		"data-store-preview-open",
		"function renderPreviewPlaceholder()",
		"function openPreviewFrame()",
		"function setPreviewVisible(visible)",
		"previewHost.replaceChildren(renderPreviewPlaceholder())",
		"terminalPreview.classList.toggle('is-preview-hidden', !previewVisible)",
		"previewToggleButton.addEventListener('click', () => setPreviewVisible(!previewVisible))",
		"openButton.addEventListener('click', openPreviewFrame)",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("store terminal preview missing stable placeholder/toggle marker %q", marker)
		}
	}

	css := readEmbeddedText(t, "css/desktop-windows.css")
	for _, marker := range []string{
		".vd-store-preview-placeholder",
		".vd-store-terminal-preview.is-preview-hidden",
		".vd-store-terminal-preview.is-preview-hidden .vd-store-preview-pane",
		"grid-template-columns: minmax(0, 1fr) 0 0",
	} {
		if !strings.Contains(css, marker) {
			t.Fatalf("store terminal preview CSS missing stable placeholder/toggle marker %q", marker)
		}
	}
}

func TestDesktopStoreTerminalPreviewSupportsRestartAndPolling(t *testing.T) {
	t.Parallel()

	source := storeTerminalPreviewSource(t)
	for _, marker := range []string{
		"data-store-terminal-restart",
		"function restartActiveTerminalSession()",
		"async function pollPreviewStatus()",
		"function previewStatusURL(storeAppId, previewPortID)",
		"'/api/desktop/store/apps/' + encodeURIComponent(storeAppId) + '/preview-status?port_id=' + encodeURIComponent(previewPortID)",
		"fetch(previewStatusURL(storeAppId, previewPortID), { credentials: 'same-origin', cache: 'no-store' })",
		"desktop.store_terminal_preview_ready_toast",
		"window.StoreTerminalPreviewApp.render = render",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("store terminal preview missing restart/polling marker %q", marker)
		}
	}
}

func TestDesktopStoreTerminalPreviewDelegatesFromQuickConnect(t *testing.T) {
	t.Parallel()

	source := readEmbeddedText(t, "js/desktop/apps/quickconnect-launchpad-chat.js")
	for _, marker := range []string{
		"function loadStoreTerminalPreviewModule()",
		"/js/desktop/apps/store-terminal-preview.js",
		"window.StoreTerminalPreviewApp.render",
		"function renderStoreTerminalPreviewApp(id, app, storeAppId)",
		"storeTerminalPreviewDeps()",
		"await loadStoreTerminalPreviewModule()",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("quickconnect missing store terminal preview delegation marker %q", marker)
		}
	}
}

func TestDesktopStoreTerminalPreviewLoadsAsStandaloneScript(t *testing.T) {
	t.Parallel()

	bundle := readDesktopAssetText(t, "js/desktop/main.js")
	if strings.Contains(bundle, "/* ui/js/desktop/apps/store-terminal-preview.js */") {
		t.Fatal("store-terminal-preview must not be concatenated into main.bundle.js")
	}
	source := readEmbeddedText(t, "js/desktop/apps/store-terminal-preview.js")
	if !strings.Contains(source, "window.StoreTerminalPreviewApp.render = render") {
		t.Fatal("standalone store-terminal-preview module must register StoreTerminalPreviewApp")
	}
}
