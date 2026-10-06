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

// requireContainersTranslations checks every key in all 16 locales and refuses
// English copies outside en.json (AGENTS.md: translate, never fill with English).
func requireContainersTranslations(t *testing.T, required []string) {
	t.Helper()
	langs := []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"}
	bundles := make(map[string]map[string]string, len(langs))
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
		bundles[lang] = values
	}
	for _, lang := range langs {
		for _, key := range required {
			value := strings.TrimSpace(bundles[lang][key])
			if value == "" {
				t.Fatalf("lang/containers/%s.json missing non-empty translation for %s", lang, key)
			}
			if lang != "en" && value == strings.TrimSpace(bundles["en"][key]) {
				t.Fatalf("lang/containers/%s.json copies the English text for %s", lang, key)
			}
		}
	}
}

func TestContainersProtectedActionTranslationsExist(t *testing.T) {
	t.Parallel()

	requireContainersTranslations(t, []string{
		"containers.protected_badge",
		"containers.protected_endpoint_warning",
		"containers.protected_network_warning",
		"containers.protected_self_warning",
		"containers.protected_terminal_confirm",
		"containers.protected_terminal_confirm_btn",
		"containers.protected_terminal_title",
		"containers.protected_unverified_warning",
		"containers.protected_warning",
		"containers.self_update_unsupported",
	})
}

func TestContainersScriptConfirmsProtectedContainersBeforeSendingTheFlag(t *testing.T) {
	t.Parallel()

	source := rawDesktopAssetText(t, "js/containers/main.js")
	for _, marker := range []string{
		"const CONFIRM_PROTECTED_QUERY = 'confirm=protected';",
		"const protectionById = new Map();",
		"function rememberProtection(containers)",
		"function containerProtection(c)",
		"function showProtectedTerminalModal(id, name, protection)",
		"function confirmProtectedTerminal()",
		"if (target) openTerminal(target.id, target.name, true);",
		"function openTerminal(id, name, confirmed)",
		"const query = confirmed ? `?${CONFIRM_PROTECTED_QUERY}` : '';",
		"const query = updateProtection ? `?${CONFIRM_PROTECTED_QUERY}` : '';",
		"const confirmQuery = deleteProtection ? `&${CONFIRM_PROTECTED_QUERY}` : '';",
		"if (!updateTarget || updateInFlight || updateBlocked()) return;",
		"data.code === 'container_protected_confirmation_required'",
		"data.code === 'container_self_update_unsupported'",
		"containers.protected_badge",
		"if (c.shared_network) return 'shared-network';",
		"if (kind === 'shared-network') return 'containers.protected_network_warning';",
		"if (kind === 'unverified') return 'containers.protected_unverified_warning';",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("containers script missing protected-container marker %q", marker)
		}
	}
	if strings.Count(source, "confirm=protected") != 1 {
		t.Fatal("the confirmation query must come from CONFIRM_PROTECTED_QUERY only")
	}
	html := rawDesktopAssetText(t, "containers.html")
	for _, marker := range []string{
		`id="protected-terminal-modal"`,
		`aria-labelledby="protected-terminal-modal-title"`,
		`onclick="confirmProtectedTerminal()"`,
		`data-i18n="containers.protected_terminal_confirm"`,
		`id="update-protected-warning"`,
		`id="delete-protected-warning"`,
	} {
		if !strings.Contains(html, marker) {
			t.Fatalf("containers page missing protected-container marker %q", marker)
		}
	}
}

func TestContainersListFailureShowsDockerMessageNotDisabledState(t *testing.T) {
	t.Parallel()

	source := rawDesktopAssetText(t, "js/containers/main.js")
	for _, marker := range []string{
		"if (resp.status === 503) {",
		"showListErrorState(dockerErrMsg(data.message));",
		"function showListErrorState(message)",
		"document.getElementById('ct-list-error-message').textContent = message;",
		"document.getElementById('ct-list-error').classList.add('is-hidden');",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("containers script missing list-error marker %q", marker)
		}
	}
	html := rawDesktopAssetText(t, "containers.html")
	for _, marker := range []string{`id="ct-list-error"`, `id="ct-list-error-message"`, `data-i18n="containers.list_error_title"`} {
		if !strings.Contains(html, marker) {
			t.Fatalf("containers page missing list-error marker %q", marker)
		}
	}
	requireContainersTranslations(t, []string{"containers.list_error_title"})
}
