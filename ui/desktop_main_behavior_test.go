package ui

import (
	"strings"
	"testing"
)

func TestDesktopMainRendersDesktopDirectoryEntries(t *testing.T) {
	t.Parallel()

	script := readDesktopAssetText(t, "js/desktop/main.js")
	for _, want := range []string{
		"state.desktopFiles = await loadDesktopFiles()",
		"/api/desktop/files?path=Desktop",
		"desktop-entry-' + file.path",
		"data-desktop-entry",
		"btn.dataset.kind === 'file'",
		"method: 'PUT'",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("desktop main script missing desktop file rendering behavior %q", want)
		}
	}
}

func TestDesktopMainProtectsEditingAndBooleanSettings(t *testing.T) {
	t.Parallel()

	script := readDesktopAssetText(t, "js/desktop/main.js")
	for _, want := range []string{
		"function isEditableTarget",
		"[contenteditable=\"true\"]",
		"if (isEditableTarget(event.target)) return;",
		"function settingBool(key)",
		"value === false || value === 0",
		"String(value).toLowerCase() !== 'false'",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("desktop main script missing editing/settings marker %q", want)
		}
	}
}

func TestDesktopMainDeduplicatesBootstrapReloads(t *testing.T) {
	t.Parallel()

	script := readDesktopAssetText(t, "js/desktop/main.js")
	for _, want := range []string{
		"bootstrapReloadPromise",
		"async function loadBootstrap()",
		"return bootstrapReloadPromise",
		"finally { bootstrapReloadPromise = null; }",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("desktop main script missing bootstrap dedupe marker %q", want)
		}
	}
}
