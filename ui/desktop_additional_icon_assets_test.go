package ui

import (
	"bytes"
	"encoding/json"
	"image/png"
	"reflect"
	"strings"
	"testing"

	"aurago/internal/desktop"
)

func TestDesktopAdditionalIconAssets(t *testing.T) {
	catalog := desktop.DesktopIconCatalog(nil)
	for _, theme := range []string{"tela", "candy", "fluent", "buuf_nestort"} {
		t.Run(theme, func(t *testing.T) {
			base := "img/" + strings.ReplaceAll(theme, "_", "-") + "/"
			var manifest struct {
				Icons        map[string]string            `json:"icons"`
				Aliases      map[string]string            `json:"aliases"`
				Actions      map[string]map[string]string `json:"actions"`
				FolderColors []string                     `json:"folder_colors"`
				Folders      map[string]map[string]string `json:"folder_variants"`
			}
			if err := json.Unmarshal([]byte(rawDesktopAssetText(t, base+"manifest.json")), &manifest); err != nil {
				t.Fatal(err)
			}
			if len(manifest.Icons) != len(catalog.Preferred) || !reflect.DeepEqual(manifest.Aliases, catalog.Aliases) {
				t.Fatal("catalog/alias mismatch")
			}
			for _, name := range catalog.Preferred {
				if manifest.Icons[name] == "" {
					t.Errorf("missing %s", name)
				}
			}
			for _, def := range desktop.DesktopSettingDefinitions() {
				if def.Key == "appearance."+theme+"_folder_color" && !reflect.DeepEqual(def.Values, manifest.FolderColors) {
					t.Fatalf("frontend/backend colors differ: %v / %v", manifest.FolderColors, def.Values)
				}
			}
			refs := map[string]bool{}
			for _, group := range []map[string]string{manifest.Icons, manifest.Actions["light"], manifest.Actions["dark"]} {
				for _, path := range group {
					refs[path] = true
				}
			}
			for color, folders := range manifest.Folders {
				for _, path := range folders {
					refs[path] = true
				}
				if color != "standard" {
					plain, _ := Content.ReadFile(manifest.Folders["standard"]["folder"])
					colored, _ := Content.ReadFile(folders["folder"])
					if bytes.Equal(plain, colored) {
						t.Errorf("%s folder did not change color", color)
					}
				}
			}
			for path := range refs {
				if !strings.HasPrefix(path, base) || strings.Contains(path, "..") {
					t.Fatalf("invalid path: %s", path)
				}
				data, err := Content.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				if strings.HasSuffix(path, ".png") {
					img, err := png.Decode(bytes.NewReader(data))
					if err != nil {
						t.Fatal(err)
					}
					transparent := false
					for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y && !transparent; y++ {
						for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
							_, _, _, a := img.At(x, y).RGBA()
							if a < 65535 {
								transparent = true
								break
							}
						}
					}
					if !transparent {
						t.Errorf("opaque PNG: %s", path)
					}
				}
			}
		})
	}
}

func TestDesktopAdditionalIconTranslations(t *testing.T) {
	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		var words map[string]string
		if err := json.Unmarshal([]byte(rawDesktopAssetText(t, "lang/desktop/"+lang+".json")), &words); err != nil {
			t.Fatal(err)
		}
		keys := []string{"settings_icon_theme_tela", "settings_icon_theme_candy", "settings_icon_theme_fluent", "settings_icon_theme_buuf_nestort", "settings_folder_color", "settings_folder_color_desc"}
		for _, def := range desktop.DesktopSettingDefinitions() {
			if strings.HasSuffix(def.Key, "_folder_color") {
				for _, value := range def.Values {
					keys = append(keys, "settings_folder_color_"+value)
				}
			}
		}
		for _, key := range keys {
			if strings.TrimSpace(words["desktop."+key]) == "" {
				t.Errorf("%s missing %s", lang, key)
			}
		}
	}
}
