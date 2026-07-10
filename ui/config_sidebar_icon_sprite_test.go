package ui

import (
	"encoding/json"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigSidebarIconSpriteContract(t *testing.T) {
	t.Parallel()

	spritePath := filepath.Join("img", "config-sidebar-icons-sprite.png")
	manifestPath := filepath.Join("img", "config-sidebar-icons-sprite.json")

	f, err := os.Open(spritePath)
	if err != nil {
		t.Fatalf("open %s: %v", spritePath, err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatalf("decode %s: %v", spritePath, err)
	}
	if got := img.Bounds(); got.Dx() != 1056 || got.Dy() != 960 {
		t.Fatalf("%s size = %dx%d, want 1056x960", spritePath, got.Dx(), got.Dy())
	}
	for _, point := range [][2]int{{0, 0}, {1055, 0}, {0, 959}, {1055, 959}} {
		_, _, _, a := img.At(point[0], point[1]).RGBA()
		if a != 0 {
			t.Fatalf("%s corner %v alpha = %d, want transparent", spritePath, point, a)
		}
	}

	var manifest struct {
		Columns    int    `json:"columns"`
		Rows       int    `json:"rows"`
		IconSize   int    `json:"icon_size"`
		Width      int    `json:"width"`
		Height     int    `json:"height"`
		Background string `json:"background"`
		Icons      []struct {
			Name   string `json:"name"`
			Row    int    `json:"row"`
			Col    int    `json:"col"`
			X      int    `json:"x"`
			Y      int    `json:"y"`
			Source string `json:"source"`
		} `json:"icons"`
	}
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read %s: %v", manifestPath, err)
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("parse %s: %v", manifestPath, err)
	}
	if manifest.Columns != 11 || manifest.Rows != 10 || manifest.IconSize != 96 || manifest.Width != 1056 || manifest.Height != 960 || manifest.Background != "transparent" {
		t.Fatalf("unexpected config sprite grid: %+v", manifest)
	}
	if len(manifest.Icons) != 101 {
		t.Fatalf("config sprite icons = %d, want 101", len(manifest.Icons))
	}
	seenNames := make(map[string]bool, len(manifest.Icons))
	seenCells := make(map[[2]int]bool, len(manifest.Icons))
	for _, icon := range manifest.Icons {
		if icon.Name == "" || icon.Source == "" {
			t.Fatalf("icon must include a name and source: %+v", icon)
		}
		if seenNames[icon.Name] || seenCells[[2]int{icon.Row, icon.Col}] {
			t.Fatalf("config sprite has duplicate icon or cell for %q", icon.Name)
		}
		seenNames[icon.Name] = true
		seenCells[[2]int{icon.Row, icon.Col}] = true
		if icon.Row < 0 || icon.Row >= 10 || icon.Col < 0 || icon.Col >= 11 || icon.X != icon.Col*96 || icon.Y != icon.Row*96 {
			t.Fatalf("invalid grid coordinates for %q: %+v", icon.Name, icon)
		}
	}
	for _, required := range []string{"overview", "docker", "github", "cloudflare_tunnel", "huggingface", "telegram", "discord", "truenas", "grafana", "google_workspace"} {
		if !seenNames[required] {
			t.Fatalf("config sprite missing required icon %q", required)
		}
	}

	mainJS := normalizeAssetText(mustReadUIFile(t, "js/config/main.js"))
	if strings.Contains(mainJS, "function configSectionIcon(") {
		t.Fatal("config sidebar must not retain the generic configSectionIcon renderer")
	}
	for _, marker := range []string{"const CONFIG_SIDEBAR_ICON_KEYS", "function configSidebarSpriteIcon(", "configSidebarSpriteIcon(s.key)", "configSidebarSpriteIcon('overview')"} {
		if !strings.Contains(mainJS, marker) {
			t.Fatalf("config main.js missing sprite marker %q", marker)
		}
	}

	css := normalizeAssetText(mustReadUIFile(t, "css/config-workspace.css"))
	for _, marker := range []string{"config-sidebar-icons-sprite.png", ".cfg-sprite-icon", "background-position", "background-size"} {
		if !strings.Contains(css, marker) {
			t.Fatalf("config workspace CSS missing sprite marker %q", marker)
		}
	}
}
