package ui

import (
	"strings"
	"testing"
)

func TestDesktopViewerUsesDOMPurifyForDocumentHTML(t *testing.T) {
	t.Parallel()
	source := rawDesktopAssetText(t, "js/desktop/apps/viewer.js")
	body := jsFunctionBodyInWindowMenuTest(t, source, "function sanitizeViewerHTML(html)")
	for _, want := range []string{"window.DOMPurify.sanitize", "FORBID_TAGS", "FORBID_ATTR: ['srcdoc']", "return esc(html)"} {
		if !strings.Contains(body, want) {
			t.Fatalf("viewer document sanitizer missing %q", want)
		}
	}
	if strings.Contains(body, "div.innerHTML = html") {
		t.Fatal("viewer must not fall back to its former regex/DOM sanitizer")
	}
}

func TestDesktopBatchRenameBoundsRegularExpressions(t *testing.T) {
	t.Parallel()
	source := rawDesktopAssetText(t, "js/desktop/file-manager/advanced-actions.js")
	worker := rawDesktopAssetText(t, "js/desktop/workers/batch-rename-regex-worker.js")
	for _, want := range []string{"new Worker('/js/desktop/workers/batch-rename-regex-worker.js?v='", "findText.length > 256", "}, 200)", "worker.terminate()", "generation !== previewGeneration", "replaceAll(findText, replaceText)"} {
		if !strings.Contains(source, want) {
			t.Fatalf("batch rename regex guard missing %q", want)
		}
	}
	if !strings.Contains(worker, "new RegExp") || !strings.Contains(worker, "self.postMessage") {
		t.Fatal("batch rename regex worker is incomplete")
	}
	if strings.Contains(source, "new RegExp(findText") {
		t.Fatal("batch rename must not evaluate user regexes on the UI thread")
	}
}

func TestDesktopAppsDoNotFallBackToNativeDialogs(t *testing.T) {
	t.Parallel()
	for _, asset := range []string{
		"js/desktop/apps/looper.js",
		"js/desktop/apps/quickconnect-launchpad-chat.js",
		"js/desktop/core/file-dialog-runtime.js",
	} {
		source := rawDesktopAssetText(t, asset)
		for _, forbidden := range []string{"window.prompt(", "window.confirm(", "const newName = prompt(", "const dirName = prompt(", "const dstPath = prompt("} {
			if strings.Contains(source, forbidden) {
				t.Fatalf("%s still uses native dialog %q", asset, forbidden)
			}
		}
	}
}

func TestVirtualComputersPreservesTextErrorBodies(t *testing.T) {
	t.Parallel()
	source := rawDesktopAssetText(t, "js/desktop/apps/virtual-computers.js")
	body := jsFunctionBodyInWindowMenuTest(t, source, "async function request(path, options)")
	for _, want := range []string{"const raw = await resp.text()", "JSON.parse(raw)", "textBody", ".slice(0, 4096)", "resp.statusText", "'HTTP ' + resp.status"} {
		if !strings.Contains(body, want) {
			t.Fatalf("virtual computer request error handling missing %q", want)
		}
	}
}

func TestCodeStudioSanitizesPersistedStateAndSocketAliases(t *testing.T) {
	t.Parallel()
	source := rawDesktopAssetText(t, "js/desktop/apps/code-studio/core.js")
	for _, want := range []string{
		"const MIN_SIDEBAR_WIDTH = 180", "const MAX_SIDEBAR_WIDTH = 500",
		"const MIN_TERMINAL_HEIGHT = 80", "const MAX_TERMINAL_HEIGHT = 600",
		"function normalizePersistedPaths(values)", "normalizeCodeStudioPath(rawPath)",
		"if (path === WORKSPACE_ROOT || seen.has(path)) continue", "const sockets = new Set()",
		"if (instance.ws) sockets.add(instance.ws)", "session.ws = null",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("Code Studio persisted-state hardening missing %q", want)
		}
	}
}

func TestLiveSpeechSeparatesDeploymentAndRealtimeInitialization(t *testing.T) {
	t.Parallel()
	source := rawDesktopAssetText(t, "js/desktop/apps/live-speech.js")
	for _, want := range []string{"Promise.allSettled", "statusResult.status !== 'fulfilled'", "realtimeResult.status === 'rejected'", "desktop.live_speech_config_unavailable", "activate.disabled = configurationUnavailable"} {
		if !strings.Contains(source, want) {
			t.Fatalf("Live Speech initialization handling missing %q", want)
		}
	}
	if strings.Contains(source, "initialize().catch(() => null)") {
		t.Fatal("Live Speech must not swallow realtime initialization errors")
	}
}

func TestVirtualWorkspaceToolsHaveDedicatedManuals(t *testing.T) {
	t.Parallel()
	workspace := string(mustReadUIFile(t, "../prompts/tools_manuals/virtual_workspace.md"))
	browser := string(mustReadUIFile(t, "../prompts/tools_manuals/virtual_browser.md"))
	if !strings.Contains(workspace, "workspace_agent_upgrade_required") || !strings.Contains(workspace, "request_credential_grant") {
		t.Fatal("virtual_workspace manual is missing upgrade or credential boundaries")
	}
	if !strings.Contains(browser, "untrusted external data") || !strings.Contains(browser, "credential_fill") {
		t.Fatal("virtual_browser manual is missing page-trust or credential boundaries")
	}
}
