package ui

import (
	"bytes"
	"encoding/json"
	"image/png"
	"testing"
)

func TestDesktopLeafyAtlasAndLocales(t *testing.T) {
	data, err := Content.ReadFile("img/leafy/fallback.png")
	if err != nil {
		t.Fatal(err)
	}
	atlas, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if atlas.Bounds().Dx() != 1024 || atlas.Bounds().Dy() != 512 {
		t.Fatal("invalid Leafy atlas dimensions")
	}
	for cell := 0; cell < 8; cell++ {
		opaque, soft := 0, 0
		for y := 0; y < 256; y++ {
			for x := 0; x < 256; x++ {
				_, _, _, a := atlas.At((cell%4)*256+x, (cell/4)*256+y).RGBA()
				if a > 0 {
					opaque++
				}
				if a > 0 && a < 65535 {
					soft++
				}
				if (x < 2 || y < 2 || x > 253 || y > 253) && a != 0 {
					t.Fatalf("cell %d clips or bleeds at %d,%d", cell, x, y)
				}
			}
		}
		if opaque < 200 || soft < 20 {
			t.Fatalf("cell %d lacks artwork or soft alpha", cell)
		}
	}
	english, _ := Content.ReadFile("lang/desktop/en.json")
	var words map[string]string
	json.Unmarshal(english, &words)
	for _, lang := range []string{"cs", "da", "de", "el", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		raw, err := Content.ReadFile("lang/desktop/" + lang + ".json")
		if err != nil {
			t.Fatal(err)
		}
		var translated map[string]string
		if err := json.Unmarshal(raw, &translated); err != nil {
			t.Fatal(err)
		}
		for key := range words {
			if len(key) > 14 && key[:14] == "desktop.leafy_" && translated[key] == "" {
				t.Errorf("%s missing %s", lang, key)
			}
		}
	}
}

// Protect the alpha-derived blade bounds and attachment lookup from atlas regressions.
func TestDesktopLeafyNaturalTexture(t *testing.T) {
	data, err := Content.ReadFile("img/leafy/leaves-natural.png")
	if err != nil {
		t.Fatal(err)
	}
	atlas, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	width, height := atlas.Bounds().Dx(), atlas.Bounds().Dy()
	if width < 512 || width > 2048 || width != height || width%2 != 0 {
		t.Fatal("expected a bounded square 2x2 botanical atlas")
	}
	cellSize := width / 2
	for cell := 0; cell < 4; cell++ {
		opaque, soft := 0, 0
		for y := 0; y < cellSize; y++ {
			for x := 0; x < cellSize; x++ {
				_, _, _, a := atlas.At(cell%2*cellSize+x, cell/2*cellSize+y).RGBA()
				if a > 60000 {
					opaque++
				}
				if a > 0 && a < 60000 {
					soft++
				}
				if ((cell%2 == 0 && x < 2) || (cell%2 == 1 && x >= cellSize-2) || (cell/2 == 0 && y < 2) || (cell/2 == 1 && y >= cellSize-2)) && a > 16000 {
					t.Fatalf("botanical cell %d clips the sheet edge at %d,%d", cell, x, y)
				}
			}
		}
		if opaque < 10000 || soft < 20 {
			t.Fatalf("botanical cell %d lacks blade detail or soft edges", cell)
		}
	}
}
