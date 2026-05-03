package ui

import (
	"bytes"
	"encoding/json"
	"image"
	_ "image/jpeg"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/desktop"
)

var desktopWallpaperOptions = []string{
	"alpine_dawn",
	"city_rain",
	"ocean_cliff",
	"aurora_glass",
	"nebula_flow",
	"paper_waves",
}

func TestDesktopWallpaperAssetsAreEmbeddedAndSelectable(t *testing.T) {
	t.Parallel()

	shell, err := Content.ReadFile("js/desktop/main.js")
	if err != nil {
		t.Fatalf("desktop shell missing from embedded UI: %v", err)
	}
	shellText := string(shell)
	css, err := Content.ReadFile("css/desktop.css")
	if err != nil {
		t.Fatalf("desktop stylesheet missing from embedded UI: %v", err)
	}
	cssText := string(css)

	defs := desktop.DesktopSettingDefinitions()
	wallpaperValues := map[string]bool{}
	for _, def := range defs {
		if def.Key == "appearance.wallpaper" {
			for _, value := range def.Values {
				wallpaperValues[value] = true
			}
		}
	}

	for _, name := range desktopWallpaperOptions {
		translationKey := "desktop.settings_wallpaper_" + name
		if !strings.Contains(shellText, "'"+name+"'") || !strings.Contains(shellText, translationKey) {
			t.Fatalf("desktop shell is missing wallpaper option %q", name)
		}
		if !strings.Contains(cssText, `data-wallpaper="`+name+`"`) || !strings.Contains(cssText, "wallpapers/"+name+".jpg") {
			t.Fatalf("desktop stylesheet is missing wallpaper background %q", name)
		}
		if !wallpaperValues[name] {
			t.Fatalf("desktop setting definitions missing wallpaper value %q", name)
		}
		data, err := Content.ReadFile("img/wallpapers/" + name + ".jpg")
		if err != nil {
			t.Fatalf("wallpaper asset %q is not embedded: %v", name, err)
		}
		if len(data) < 500000 {
			t.Fatalf("wallpaper asset %q is unexpectedly small: %d bytes", name, len(data))
		}
		cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
		if err != nil {
			t.Fatalf("decode wallpaper %q dimensions: %v", name, err)
		}
		if cfg.Width != 3840 || cfg.Height != 2160 {
			t.Fatalf("wallpaper %q dimensions = %dx%d, want 3840x2160", name, cfg.Width, cfg.Height)
		}
	}

	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		path := filepath.Join("lang", "desktop", lang+".json")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		var values map[string]string
		if err := json.Unmarshal(data, &values); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, name := range desktopWallpaperOptions {
			key := "desktop.settings_wallpaper_" + name
			if strings.TrimSpace(values[key]) == "" {
				t.Fatalf("%s missing non-empty translation for %s", path, key)
			}
		}
	}
}
