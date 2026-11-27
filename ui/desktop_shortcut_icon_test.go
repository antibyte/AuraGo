package ui

import (
	"strings"
	"testing"
)

func TestDesktopShortcutsPreferCurrentAppLogosAndKeepPersistedThemeIcons(t *testing.T) {
	t.Parallel()

	foundation := rawDesktopAssetText(t, "js/desktop/core/desktop-foundation.js")
	updateIconBody := jsFunctionBodyInWindowMenuTest(t, foundation, "function updateDesktopIconButton")
	if !strings.Contains(updateIconBody, "item.icon || (item.type === 'file'") {
		t.Fatal("desktop icon rendering does not prefer persisted shortcut icon keys")
	}

	for _, want := range []string{
		"function shortcutIconForApp(shortcut, app)",
		"const appLogo = appLogoIconKey(app);",
		"if (appLogo) return appLogo;",
		"return appIconKeys[app.id] || shortcut.icon || app.icon || '';",
	} {
		if !strings.Contains(foundation, want) {
			t.Fatalf("desktop shortcut icon resolver missing marker %q", want)
		}
	}

	shortcutItemsBody := jsFunctionBodyInWindowMenuTest(t, foundation, "function desktopShortcutItems()")
	for _, want := range []string{
		"icon: shortcutIconForApp(shortcut, app),",
		"path: shortcut.path || shortcut.target_id || ''",
	} {
		if !strings.Contains(shortcutItemsBody, want) {
			t.Fatalf("desktop shortcut items do not preserve marker %q", want)
		}
	}
	for _, want := range []string{
		"shortcut.target_type === 'dashboard'",
		"appById('homedash')",
		"/^d_[a-z2-7]{12}$/",
		"type: 'dashboard'",
	} {
		if !strings.Contains(shortcutItemsBody, want) {
			t.Fatalf("desktop shortcut items miss the dashboard marker %q", want)
		}
	}
	for _, want := range []string{"btn.dataset.dashboardId = item.dashboardId || ''", "item.type === 'dashboard' ? ''"} {
		if !strings.Contains(updateIconBody, want) {
			t.Fatalf("desktop icon button misses the dashboard marker %q", want)
		}
	}
}
