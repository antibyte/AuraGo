package ui

import (
	"strings"
	"testing"
)

func TestCodeStudioUsesPerWindowStateAndClosesTerminal(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/code-studio.js")
	for _, marker := range []string{
		"const instances = new Map()",
		"function createInstance",
		"instances.set(windowId, instance)",
		"CodeStudioApp.dispose",
		"instance.ws.close()",
		"function runWithInstance(instance, fn)",
		"finally {",
		"async function runAsyncStep",
		"instances.get(state.windowId) === state",
		"if (!isLiveInstance(instance)) return;",
		"const target = state;",
		"if (!isLiveInstance(target)) return;",
		"if (!isLiveInstance(instance)) return undefined;",
		"await runAsyncStep(target, saveCurrentFile);",
		"renderStatus(tr('codeStudio.running'",
		"function destroyTabView",
		"destroyTabView(tab);",
		"function registerDisposer",
		"state.disposers = state.disposers.filter(item => item !== disposeFn)",
		"removedTabs.forEach(destroyTabView)",
		"registerDisposer(() => cleanup(''))",
		"registerDisposer(() => cleanup(false))",
		"term.dispose()",
		"terminalDisposed",
		"state.disposers.push(() => {",
		"document.removeEventListener('mousedown'",
		"cleanup('')",
		"cleanup(false)",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("Code Studio per-window lifecycle missing marker %q", marker)
		}
	}
	if strings.Contains(source, "result && typeof result.then === 'function'") {
		t.Fatalf("Code Studio runWithInstance must not hold state until promises settle")
	}
	if strings.Contains(source, "return runWithInstance(instance, async () => {") {
		t.Fatalf("Code Studio render must not hold global state across awaited operations")
	}
	if strings.Contains(source, "if (!instances.has(windowId)) return;") {
		t.Fatalf("Code Studio render must ignore stale instances, not just reused window IDs")
	}
	if strings.Contains(source, "if (instances.has(windowId)) runWithInstance(instance") {
		t.Fatalf("Code Studio render catch path must ignore stale instances")
	}
	if !strings.Contains(source, "window.CodeStudio = window.CodeStudioApp") {
		t.Fatalf("Code Studio compatibility export missing")
	}
}
