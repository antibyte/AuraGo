package ui

import (
	"strings"
	"testing"
)

func TestDesktopTerminalStyleCatalog(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/terminal-styles.js")
	for _, want := range []string{
		"window.TerminalStyles = {",
		"ids:",
		"normalize:",
		"load:",
		"save:",
		"profile:",
		"applyXterm:",
		"'modern'",
		"'amber'",
		"'green'",
		"'apple2'",
		"'commodore64'",
		"'ibm3278'",
		"'vintage'",
		"'mono-green'",
		"'transparent-green'",
		"aurago.desktop.terminal.style",
		"return 'modern'",
		"phosphor:",
		"term.options.theme = profile.theme",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("terminal-styles.js missing %q", want)
		}
	}
	if strings.Contains(source, "crt-shader") || strings.Contains(source, "cool-retro-term") {
		t.Fatal("terminal-styles.js must not reference Chat CRT shaders or cool-retro-term sources")
	}
}
