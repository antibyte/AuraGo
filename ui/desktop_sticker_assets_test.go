package ui

import (
	"encoding/json"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestDesktopStickerAssets(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "assets", "stickers", "production.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Items []struct{ ID, Category, Style, Prompt string }
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	if len(manifest.Items) != 100 {
		t.Fatalf("want 100 stickers, got %d", len(manifest.Items))
	}
	seen := map[string]bool{}
	for _, item := range manifest.Items {
		if seen[item.ID] || item.Prompt == "" {
			t.Fatalf("duplicate ID or missing prompt: %s", item.ID)
		}
		seen[item.ID] = true
		for _, size := range []int{1024, 192} {
			name := filepath.Join("img", "stickers", item.ID+".png")
			if size == 192 {
				name = filepath.Join("img", "stickers", "thumbs", item.ID+".png")
			}
			f, err := os.Open(name)
			if err != nil {
				t.Fatal(err)
			}
			img, err := png.Decode(f)
			f.Close()
			if err != nil {
				t.Fatal(err)
			}
			if img.Bounds().Dx() != size || img.Bounds().Dy() != size {
				t.Fatalf("%s: wrong dimensions", name)
			}
			clear, visible, edge := 0, 0, 0
			for y := 0; y < size; y++ {
				for x := 0; x < size; x++ {
					_, _, _, alpha := img.At(x, y).RGBA()
					if alpha == 0 {
						clear++
					}
					if alpha > 32768 {
						visible++
					}
					if (x == 0 || y == 0 || x == size-1 || y == size-1) && alpha > 4096 {
						edge++
					}
				}
			}
			if clear < size*size/50 || visible < size*size/100 {
				t.Fatalf("%s: absent transparency or empty artwork", name)
			}
			if edge != 0 {
				t.Fatalf("%s: artwork touches canvas edge (%d pixels)", name, edge)
			}
		}
	}
}
