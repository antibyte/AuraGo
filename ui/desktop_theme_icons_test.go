package ui

import (
	"encoding/json"
	"strings"
	"testing"

	"aurago/internal/desktop"
)

func TestDesktopBuiltInAppsUseDedicatedThemeAppIcons(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/main.js")
	for _, marker := range []string{
		"radio: 'radio'",
		"gallery: 'gallery'",
		"music: 'audio-player'",
		"looper: 'looper'",
		"'agent-chat': 'agent-chat'",
		"'software-store': 'software-store'",
		"zipper: 'zipper'",
		"pixel: 'pixel'",
		"Trash: 'trash-empty'",
		"launchpad: 'launchpad'",
		"appIconKeys['code-studio'] = 'code-studio'",
		"function themeBackedAppIconKey(app)",
		"'galaxa-deluxe': 'galaxa-deluxe'",
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
		"'software-store': 'package'",
		"zipper: 'archive'",
		"pixel: 'image'",
		"Trash: 'trash',",
		"launchpad: 'apps'",
		"appIconKeys['code-studio'] = 'code'",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("desktop app icon mapping must not reuse placeholder/file-type marker %q", forbidden)
		}
	}
	for _, theme := range []string{"papirus", "whitesur"} {
		manifest := rawDesktopAssetText(t, "img/"+theme+"/manifest.json")
		for _, key := range []string{"agent-chat", "code-studio", "commandcode", "galaxa-deluxe", "gallery", "launchpad", "looper", "pixel", "quakejs", "radio", "software-store", "trash", "trash-empty", "trash-full", "zipper"} {
			if !strings.Contains(manifest, `"`+key+`"`) {
				t.Fatalf("%s theme manifest missing %q", theme, key)
			}
		}
	}
}

func TestDesktopBuiltInAppsUseFocusedThemeIconNames(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/main.js")
	for _, marker := range []string{
		"'software-store': 'software-store'",
		"zipper: 'zipper'",
		"pixel: 'pixel'",
		"Trash: 'trash-empty'",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("desktop theme icon resolver missing focused icon marker %q", marker)
		}
	}
	for _, placeholder := range []string{
		"'software-store': 'package'",
		"zipper: 'archive'",
		"pixel: 'image'",
		"Trash: 'trash',",
	} {
		if strings.Contains(source, placeholder) {
			t.Fatalf("desktop app icon mapping still uses placeholder marker %q", placeholder)
		}
	}

	catalog := desktop.DesktopIconCatalog(map[string]string{"appearance.icon_theme": "papirus"})
	for _, key := range []string{"software-store", "zipper", "pixel", "trash-empty", "trash-full"} {
		if !containsString(catalog.Preferred, key) {
			t.Fatalf("backend icon catalog missing focused icon %q", key)
		}
	}
	for alias, want := range map[string]string{
		"store":        "software-store",
		"app-store":    "software-store",
		"file-zip":     "zipper",
		"zip":          "zipper",
		"paint":        "pixel",
		"photo-editor": "pixel",
		"trash":        "trash-empty",
		"user-trash":   "trash-empty",
	} {
		if got := catalog.Aliases[alias]; got != want {
			t.Fatalf("desktop icon alias %q = %q, want %q", alias, got, want)
		}
	}

	for _, theme := range []string{"papirus", "whitesur"} {
		data, err := Content.ReadFile("img/" + theme + "/manifest.json")
		if err != nil {
			t.Fatalf("read %s manifest: %v", theme, err)
		}
		var manifest struct {
			Icons map[string]string `json:"icons"`
		}
		if err := json.Unmarshal(data, &manifest); err != nil {
			t.Fatalf("parse %s manifest: %v", theme, err)
		}
		for _, key := range []string{"software-store", "zipper", "pixel", "agent-chat", "trash-empty", "trash-full"} {
			path, ok := manifest.Icons[key]
			if !ok {
				t.Fatalf("%s theme manifest missing focused icon %q", theme, key)
			}
			svg, err := Content.ReadFile(path)
			if err != nil {
				t.Fatalf("%s focused icon %q not embedded at %s: %v", theme, key, path, err)
			}
			if !strings.Contains(string(svg), "<svg") {
				t.Fatalf("%s focused icon %q is not an SVG asset", theme, key)
			}
		}
		if manifest.Icons["trash-empty"] == manifest.Icons["trash-full"] {
			t.Fatalf("%s theme must expose distinct empty and full trash icons", theme)
		}
		if manifest.Icons["trash"] != manifest.Icons["trash-empty"] {
			t.Fatalf("%s theme trash alias path = %q, want empty trash path %q", theme, manifest.Icons["trash"], manifest.Icons["trash-empty"])
		}
	}
}
