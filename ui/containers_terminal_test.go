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
		"onclick=\"showTerminal(${safeID}, ${terminalName})\"",
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

func TestContainersScriptEscapesInlineActionArguments(t *testing.T) {
	t.Parallel()

	source := rawDesktopAssetText(t, "js/containers/main.js")
	for _, marker := range []string{
		"function jsArg(value)",
		"const safeID = jsArg(c.id || '')",
		"onclick=\"containerAction(${safeID},'stop')\"",
		"onclick=\"showTerminal(${safeID}, ${terminalName})\"",
		"onclick=\"showUpdateModal(${safeID}, ${updateName})\"",
		"onclick=\"showLogs(${safeID})\"",
		"onclick=\"showInspect(${safeID})\"",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("containers script missing safe inline argument marker %q", marker)
		}
	}
	for _, forbidden := range []string{
		"onclick=\"containerAction('${c.id}'",
		"onclick=\"showTerminal('${c.id}'",
		"onclick=\"showUpdateModal('${c.id}'",
		"onclick=\"showLogs('${c.id}'",
		"onclick=\"showInspect('${c.id}'",
		"data-id=\"${c.id}\"",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("containers script still embeds raw container data marker %q", forbidden)
		}
	}
}

func TestContainersTerminalModalHasBoundedLayout(t *testing.T) {
	t.Parallel()

	css := rawDesktopAssetText(t, "css/containers.css")
	for _, marker := range []string{
		".ct-terminal-modal",
		"max-height:",
		"display: flex",
		"flex-direction: column",
		".ct-terminal-body",
		"min-height: 0",
		"overflow: hidden",
		".ct-terminal-output",
		"height: clamp(",
		"min-height: 0",
	} {
		if !strings.Contains(css, marker) {
			t.Fatalf("containers terminal CSS missing bounded layout marker %q", marker)
		}
	}
}

func TestContainersTerminalWritesVisibleSessionText(t *testing.T) {
	t.Parallel()

	source := rawDesktopAssetText(t, "js/containers/main.js")
	for _, marker := range []string{
		"function writeTerminalNotice",
		"terminal.writeln",
		"containers.terminal_opening",
		"containers.terminal_unavailable",
		"output.textContent",
		"requestAnimationFrame",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("containers terminal script missing visible text marker %q", marker)
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
		"containers.terminal_opening",
		"containers.terminal_unavailable",
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

func TestContainersScriptIncludesUpdateActionWithConfirmation(t *testing.T) {
	t.Parallel()

	source := rawDesktopAssetText(t, "js/containers/main.js")
	for _, marker := range []string{
		"onclick=\"showUpdateModal(${safeID}, ${updateName})\"",
		"data-i18n=\"containers.btn_update\"",
		"function showUpdateModal(id, name)",
		"function confirmUpdate()",
		"/update",
		"containers.update_success",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("containers script missing update marker %q", marker)
		}
	}
	html := rawDesktopAssetText(t, "containers.html")
	for _, marker := range []string{
		`id="update-modal"`,
		`data-i18n="containers.update_title"`,
		`data-i18n="containers.update_confirm"`,
		`data-i18n="containers.update_note"`,
		`onclick="confirmUpdate()"`,
	} {
		if !strings.Contains(html, marker) {
			t.Fatalf("containers page missing update modal marker %q", marker)
		}
	}
	if strings.Contains(source, "alert(") {
		t.Fatal("containers script must not use alert() for update confirmation")
	}
}

func TestContainersUpdateTranslationsExist(t *testing.T) {
	t.Parallel()

	required := []string{
		"containers.btn_update",
		"containers.update_title",
		"containers.update_confirm",
		"containers.update_note",
		"containers.update_cancel",
		"containers.update_confirm_btn",
		"containers.update_success",
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
