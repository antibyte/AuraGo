package ui

import (
	"strings"
	"testing"
)

func TestDesktopStoreTerminalPreviewSupportsClipboardPaste(t *testing.T) {
	t.Parallel()

	source := readEmbeddedText(t, "js/desktop/apps/quickconnect-launchpad-chat.js")
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

	source := readEmbeddedText(t, "js/desktop/apps/quickconnect-launchpad-chat.js")
	for _, marker := range []string{
		"const terminalSessions = new Map()",
		"let activeTerminalSessionID = ''",
		"data-store-terminal-tabs",
		"data-store-terminal-new",
		"function createTerminalSession()",
		"function activateTerminalSession(sessionID)",
		"function closeTerminalSession(sessionID)",
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

	source := readEmbeddedText(t, "js/desktop/apps/quickconnect-launchpad-chat.js")
	for _, marker := range []string{
		"data-store-terminal-resizer",
		"function setTerminalPaneWidthPct(widthPct)",
		"function startTerminalPreviewResize(event)",
		"window.addEventListener('pointermove', resizeMoveHandler)",
		"window.removeEventListener('pointermove', resizeMoveHandler)",
		"host.style.setProperty('--store-terminal-width'",
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
	} {
		if !strings.Contains(css, marker) {
			t.Fatalf("store terminal preview CSS missing resizable split marker %q", marker)
		}
	}
}
