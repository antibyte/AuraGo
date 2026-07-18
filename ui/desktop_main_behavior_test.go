package ui

import (
	"strings"
	"testing"
)

func TestDesktopMainRendersDesktopDirectoryEntries(t *testing.T) {
	t.Parallel()

	// Boot/render logic lives in the desktop main bundle (foundation + runtime parts).
	script := readDesktopAssetText(t, "js/desktop/bundles/main.bundle.js")
	for _, want := range []string{
		"desktop_files",
		"/api/desktop/files?path=Desktop",
		"desktop-entry-' + file.path",
		"btn.dataset.desktopEntry",
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

	script := readDesktopAssetText(t, "js/desktop/bundles/main.bundle.js")
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

	script := readDesktopAssetText(t, "js/desktop/bundles/main.bundle.js")
	for _, want := range []string{
		"bootstrapReloadPromise",
		"async function loadBootstrap()",
		"async function fetchBootstrapState()",
		"Promise.all([",
		"loadIconManifest()",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("desktop main script missing bootstrap dedupe/parallel boot marker %q", want)
		}
	}
}

func TestDesktopModuleLoaderLoadsAppI18nSections(t *testing.T) {
	t.Parallel()

	script := readDesktopAssetText(t, "js/desktop/core/module-loader.js")
	for _, want := range []string{
		"APP_I18N_SECTIONS",
		"loadAppI18nSections",
		"/api/i18n?lang=",
		"&sections=",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("module-loader missing lazy i18n marker %q", want)
		}
	}
}
