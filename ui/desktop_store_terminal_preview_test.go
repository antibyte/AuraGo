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
		"const writeTerminalInput = text =>",
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
