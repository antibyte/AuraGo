package ui

import (
	"strings"
	"testing"
)

func TestAgentChatRegistersDisposeLifecycle(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/agent-chat.js")
	for _, marker := range []string{
		"function disposeAgentChatWindow",
		"window.AgentChatApp.dispose = disposeAgentChatWindow",
		"_sidebarResizeObserver.disconnect",
		"_desktopChatDropCleanup",
		"host._desktopChatAbort = null",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("agent-chat.js missing dispose lifecycle marker %q", marker)
		}
	}

	foundation := readDesktopAssetText(t, "js/desktop/core/desktop-foundation.js")
	if !strings.Contains(foundation, "'agent-chat': 'AgentChatApp'") {
		t.Fatal("desktop foundation must map agent-chat to AgentChatApp for disposeAppWindow")
	}
}

func TestMissionControlDisposeRemovesKeydownListener(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/mission-control.js")
	for _, marker := range []string{
		"state.keydownHandler = handleKeydown",
		"document.removeEventListener('keydown', st.keydownHandler)",
		"st.keydownHandler = null",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("mission-control.js missing keydown cleanup marker %q", marker)
		}
	}
}

func TestSystemInfoAppExposesDispose(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/system-info.js")
	for _, marker := range []string{
		"const instances = new Map()",
		"function dispose(windowId)",
		"window.SystemInfoApp = { render, dispose }",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("system-info.js missing marker %q", marker)
		}
	}
}

func TestOfficeAppsDisposeOnRerender(t *testing.T) {
	t.Parallel()

	for path, marker := range map[string]string{
		"js/desktop/apps/writer.js": "dispose(windowId);\n        instances.set",
		"js/desktop/apps/sheets.js": "dispose(windowId);\n        instances.set",
	} {
		source := readDesktopAssetText(t, path)
		if !strings.Contains(source, marker) {
			t.Fatalf("%s missing rerender dispose marker %q", path, marker)
		}
	}
}

func TestViewerPdfUnavailableUsesI18n(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/viewer.js")
	if !strings.Contains(source, "desktop.viewer_pdfjs_unavailable") {
		t.Fatal("viewer.js must use desktop.viewer_pdfjs_unavailable")
	}
	if strings.Contains(source, "pdf.js not loaded") {
		t.Fatal("viewer.js must not hardcode pdf.js not loaded")
	}
}

func TestViewer3DRegisteredInAppGlobalName(t *testing.T) {
	t.Parallel()

	foundation := readDesktopAssetText(t, "js/desktop/core/desktop-foundation.js")
	if !strings.Contains(foundation, "'viewer-3d': 'Viewer3DApp'") {
		t.Fatal("desktop foundation must map viewer-3d to Viewer3DApp")
	}
}