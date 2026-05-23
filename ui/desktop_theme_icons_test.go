package ui

import (
	"strings"
	"testing"
)

func TestDesktopBuiltInAppsUseDedicatedThemeAppIcons(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/main.js")
	for _, marker := range []string{
		"Trash: 'trash'",
		"radio: 'radio'",
		"gallery: 'gallery'",
		"music: 'audio-player'",
		"looper: 'looper'",
		"'agent-chat': 'agent-chat'",
		"launchpad: 'launchpad'",
		"appIconKeys['code-studio'] = 'code-studio'",
		"function shortcutIconForApp(shortcut, app)",
		"return appIconKeys[app.id] || shortcut.icon || app.icon || '';",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("desktop theme icon resolver missing marker %q", marker)
		}
	}
	for _, forbidden := range []string{
		"Trash: 'package'",
		"radio: 'audio'",
		"gallery: 'image'",
		"looper: 'workflow'",
		"'agent-chat': 'mail'",
		"launchpad: 'apps'",
		"appIconKeys['code-studio'] = 'code'",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("desktop app icon mapping must not reuse placeholder/file-type marker %q", forbidden)
		}
	}
	for _, theme := range []string{"papirus", "whitesur"} {
		manifest := rawDesktopAssetText(t, "img/"+theme+"/manifest.json")
		for _, key := range []string{"agent-chat", "code-studio", "gallery", "launchpad", "looper", "radio", "trash"} {
			if !strings.Contains(manifest, `"`+key+`"`) {
				t.Fatalf("%s theme manifest missing %q", theme, key)
			}
		}
	}
}
