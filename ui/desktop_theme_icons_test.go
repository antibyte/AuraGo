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
		"music: 'audio-player'",
		"looper: 'looper'",
		"appIconKeys['code-studio'] = 'code-studio'",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("desktop theme icon resolver missing marker %q", marker)
		}
	}
	for _, forbidden := range []string{
		"Trash: 'package'",
		"radio: 'audio'",
		"looper: 'workflow'",
		"appIconKeys['code-studio'] = 'code'",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("desktop app icon mapping must not reuse placeholder/file-type marker %q", forbidden)
		}
	}
	for _, theme := range []string{"papirus", "whitesur"} {
		manifest := rawDesktopAssetText(t, "img/"+theme+"/manifest.json")
		for _, key := range []string{"code-studio", "looper", "radio", "trash"} {
			if !strings.Contains(manifest, `"`+key+`"`) {
				t.Fatalf("%s theme manifest missing %q", theme, key)
			}
		}
	}
}
