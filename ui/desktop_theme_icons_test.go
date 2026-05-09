package ui

import (
	"strings"
	"testing"
)

func TestDesktopTrashAndRadioIconsStayThemeConsistent(t *testing.T) {
	t.Parallel()

	for _, theme := range []string{"papirus", "whitesur"} {
		for _, key := range []string{"radio", "trash"} {
			path := "img/" + theme + "/icons/" + key + ".svg"
			svg := rawDesktopAssetText(t, path)
			for _, marker := range []string{`width="24"`, `height="24"`, "currentColor"} {
				if !strings.Contains(svg, marker) {
					t.Fatalf("%s must be a compact theme-consistent icon, missing %q", path, marker)
				}
			}
			for _, forbidden := range []string{"<image", "base64", `width="64"`, `height="64"`, `width="16"`, `height="16"`, `fill="#`} {
				if strings.Contains(svg, forbidden) {
					t.Fatalf("%s must not use raster, full-size, legacy 16px, or hard-coded filled artwork, found %q", path, forbidden)
				}
			}
		}
	}
}
