package ui

import (
	"strings"
	"testing"
)

func TestDesktopTrashAndRadioUseDistinctThemeAppIcons(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/main.js")
	for _, marker := range []string{
		"Trash: 'package'",
		"radio: 'audio'",
		"music: 'audio-player'",
		"trash: 'package'",
		"const candidates = [iconAlias(normalized), normalized",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("desktop theme icon resolver missing marker %q", marker)
		}
	}
	for _, theme := range []string{"papirus", "whitesur"} {
		manifest := rawDesktopAssetText(t, "img/"+theme+"/manifest.json")
		for _, key := range []string{"package", "audio", "audio-player"} {
			if !strings.Contains(manifest, `"`+key+`"`) {
				t.Fatalf("%s theme manifest missing %q", theme, key)
			}
		}
	}
}
