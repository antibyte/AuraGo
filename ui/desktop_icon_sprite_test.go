package ui

import (
	"encoding/json"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestDesktopIconSpriteSheetContract(t *testing.T) {
	t.Parallel()

	spritePath := filepath.Join("img", "desktop-icons-sprite.png")
	manifestPath := filepath.Join("img", "desktop-icons-sprite.json")

	f, err := os.Open(spritePath)
	if err != nil {
		t.Fatalf("open %s: %v", spritePath, err)
	}
	defer f.Close()

	img, err := png.Decode(f)
	if err != nil {
		t.Fatalf("decode %s: %v", spritePath, err)
	}
	bounds := img.Bounds()
	if bounds.Dx() != 1536 || bounds.Dy() != 1536 {
		t.Fatalf("%s size = %dx%d, want 1536x1536", spritePath, bounds.Dx(), bounds.Dy())
	}
	for _, point := range [][2]int{{0, 0}, {1535, 0}, {0, 1535}, {1535, 1535}} {
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
			Width  int    `json:"width"`
			Height int    `json:"height"`
		} `json:"icons"`
	}
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read %s: %v", manifestPath, err)
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("parse %s: %v", manifestPath, err)
	}
	if manifest.Columns != 12 || manifest.Rows != 12 || manifest.IconSize != 128 || manifest.Width != 1536 || manifest.Height != 1536 {
		t.Fatalf("manifest grid = %+v, want 12x12 128px 1536x1536", manifest)
	}
	if manifest.Background != "transparent" {
		t.Fatalf("manifest background = %q, want transparent", manifest.Background)
	}
	if len(manifest.Icons) != 144 {
		t.Fatalf("manifest icons = %d, want 144", len(manifest.Icons))
	}
	seen := map[string]bool{}
	for _, icon := range manifest.Icons {
		if icon.Name == "" {
			t.Fatal("manifest contains unnamed icon")
		}
		if seen[icon.Name] {
			t.Fatalf("duplicate icon name %q", icon.Name)
		}
		seen[icon.Name] = true
		if icon.Row < 0 || icon.Row >= 12 || icon.Col < 0 || icon.Col >= 12 {
			t.Fatalf("icon %q has out-of-range cell row=%d col=%d", icon.Name, icon.Row, icon.Col)
		}
		if icon.X != icon.Col*128 || icon.Y != icon.Row*128 || icon.Width != 128 || icon.Height != 128 {
			t.Fatalf("icon %q coordinates = %+v, want 128px grid alignment", icon.Name, icon)
		}
	}
	for _, required := range []string{"desktop", "folder", "javascript", "pdf", "copy", "paste", "trash", "arrow_right", "search", "settings", "agent_chat"} {
		if !seen[required] {
			t.Fatalf("manifest missing required desktop icon %q", required)
		}
	}
}
