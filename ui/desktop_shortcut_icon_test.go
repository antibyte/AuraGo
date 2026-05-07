package ui

import (
	"strings"
	"testing"
)

func TestDesktopShortcutsKeepPersistedThemeIcons(t *testing.T) {
	t.Parallel()

	mainText := readDesktopAssetText(t, "js/desktop/main.js")
	renderIconsBody := jsFunctionBodyInWindowMenuTest(t, mainText, "function renderIcons()")
	if !strings.Contains(renderIconsBody, "item.icon || (item.type === 'file'") {
		t.Fatal("desktop icon rendering does not prefer persisted shortcut icon keys")
	}

	shortcutItemsBody := jsFunctionBodyInWindowMenuTest(t, mainText, "function desktopShortcutItems()")
	for _, want := range []string{
		"icon: shortcut.icon || ''",
		"path: shortcut.path || shortcut.target_id || ''",
	} {
		if !strings.Contains(shortcutItemsBody, want) {
			t.Fatalf("desktop shortcut items do not preserve marker %q", want)
		}
	}
}
