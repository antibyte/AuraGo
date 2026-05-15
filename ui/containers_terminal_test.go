package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestContainersPageIncludesXtermTerminalModal(t *testing.T) {
	t.Parallel()

	html := rawDesktopAssetText(t, "containers.html")
	for _, marker := range []string{
		`<link rel="stylesheet" href="/css/xterm.css">`,
		`<script defer src="/js/vendor/xterm.min.js"></script>`,
		`<script defer src="/js/vendor/xterm-addon-fit.min.js"></script>`,
		`id="terminal-modal"`,
		`id="terminal-output"`,
		`data-i18n="containers.terminal_title"`,
	} {
		if !strings.Contains(html, marker) {
			t.Fatalf("containers page missing terminal marker %q", marker)
		}
	}
}

func TestContainersScriptRendersRunningShellButtonAndCleansUpTerminal(t *testing.T) {
	t.Parallel()

	source := rawDesktopAssetText(t, "js/containers/main.js")
	for _, marker := range []string{
		"onclick=\"showTerminal('${c.id}'",
		"data-i18n=\"containers.btn_shell\"",
		"if (isRunning) {",
		"function showTerminal(id, name)",
		"new window.Terminal",
		"new window.FitAddon.FitAddon",
		"new WebSocket",
		"/terminal",
		"new TextEncoder().encode(data)",
		"terminalSocket.close()",
		"terminal.dispose()",
		"function closeTerminalModal()",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("containers script missing terminal marker %q", marker)
		}
	}
}

func TestContainersTerminalTranslationsExist(t *testing.T) {
	t.Parallel()

	required := []string{
		"containers.btn_shell",
		"containers.terminal_title",
		"containers.terminal_connecting",
		"containers.terminal_connected",
		"containers.terminal_closed",
		"containers.terminal_error",
	}
	langs := []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"}
	for _, lang := range langs {
		path := filepath.Join("lang", "containers", lang+".json")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		var values map[string]string
		if err := json.Unmarshal(data, &values); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, key := range required {
			if strings.TrimSpace(values[key]) == "" {
				t.Fatalf("%s missing non-empty translation for %s", path, key)
			}
		}
	}
}
