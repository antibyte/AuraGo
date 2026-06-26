package ui

import (
	"strings"
	"testing"
)

func TestDesktopSettingsUsesSymbolicSectionIcons(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/settings.js")
	for _, marker := range []string{
		"id: 'appearance', icon: 'settings-symbolic'",
		"id: 'desktop', icon: 'desktop-symbolic'",
		"id: 'windows', icon: 'monitor-symbolic'",
		"id: 'files', icon: 'folder-symbolic'",
		"id: 'agent', icon: 'apps-symbolic'",
		"id: 'system', icon: 'info-symbolic'",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("desktop settings must use compact symbolic section icon marker %q", marker)
		}
	}
}

func TestDesktopSettingsSymbolicIconAssetsStayCompact(t *testing.T) {
	t.Parallel()

	for _, theme := range []string{"papirus", "whitesur"} {
		for _, key := range []string{"apps-symbolic", "desktop-symbolic", "folder-symbolic", "info-symbolic", "monitor-symbolic", "settings-symbolic"} {
			path := "img/" + theme + "/icons/" + key + ".svg"
			svg := rawDesktopAssetText(t, path)
			for _, marker := range []string{`width="24"`, `height="24"`, "currentColor"} {
				if !strings.Contains(svg, marker) {
					t.Fatalf("%s must be a compact symbolic icon, missing %q", path, marker)
				}
			}
			for _, forbidden := range []string{"<image", "base64", `width="64"`, `height="64"`} {
				if strings.Contains(svg, forbidden) {
					t.Fatalf("%s must not use raster or full-size app icon artwork, found %q", path, forbidden)
				}
			}
		}
	}
}
