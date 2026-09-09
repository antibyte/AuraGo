package ui

import (
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"strings"
	"testing"
)

type galaxaAtlas struct {
	Version int
	Sheets  map[string]struct {
		File          string
		Width, Height int
	}
	Animations map[string]struct {
		Sheet         string
		Frames        [][4]int
		Size, Origin  int
		Pivot         [2]float64
		Ms            int
		Loop          bool
		SourceSize    int
		SourceAnchors [][2]float64
	}
	Scenes map[string]struct {
		Sheet string
		Frame [4]int
	}
}

func TestGalaxaEmbeddedArcadeAtlases(t *testing.T) {
	var atlas galaxaAtlas
	data, err := Content.ReadFile("img/galaxa/atlas.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &atlas); err != nil {
		t.Fatal(err)
	}
	if atlas.Version != 1 || len(atlas.Scenes) != 6 {
		t.Fatal("atlas version or sector backgrounds incomplete")
	}
	images := map[string]image.Image{}
	for id, s := range atlas.Sheets {
		f, err := Content.Open("img/galaxa/" + s.File)
		if err != nil {
			t.Fatal(err)
		}
		im, err := png.Decode(f)
		f.Close()
		if err != nil {
			t.Fatal(err)
		}
		if im.Bounds().Dx() != s.Width || im.Bounds().Dy() != s.Height {
			t.Fatalf("dimensions: %s", id)
		}
		images[id] = im
	}
	for key, a := range atlas.Animations {
		im := images[a.Sheet]
		if im == nil || len(a.Frames) == 0 || a.Ms <= 0 || a.Size <= 0 || a.Pivot != [2]float64{0.5, 0.5} {
			t.Fatalf("invalid animation %s", key)
		}
		for i, f := range a.Frames {
			if a.SourceAnchors != nil {
				if a.SourceSize <= 0 || len(a.SourceAnchors) != len(a.Frames) {
					t.Fatalf("invalid source anchor count %s", key)
				}
				p := a.SourceAnchors[i]
				if p[0] < 0 || p[1] < 0 || p[0] > float64(f[2]) || p[1] > float64(f[3]) {
					t.Fatalf("source anchor outside frame %s/%d", key, i)
				}
			}
			rect := image.Rect(f[0], f[1], f[0]+f[2], f[1]+f[3])
			if !rect.In(im.Bounds()) || rect.Empty() {
				t.Fatalf("invalid crop %s/%d", key, i)
			}
			clear, solid := 0, 0
			for y := rect.Min.Y; y < rect.Max.Y; y += 3 {
				for x := rect.Min.X; x < rect.Max.X; x += 3 {
					_, _, _, alpha := im.At(x, y).RGBA()
					if alpha < 256 {
						clear++
					}
					if alpha > 32000 {
						solid++
					}
				}
			}
			if clear == 0 {
				t.Fatalf("opaque background in %s/%d", key, i)
			}
			if solid == 0 && !strings.HasSuffix(key, ".death") && !strings.HasPrefix(key, "fx.") {
				t.Fatalf("empty sprite %s/%d", key, i)
			}
		}
	}
	require := func(key string, frames, size int) {
		t.Helper()
		a, ok := atlas.Animations[key]
		if !ok || len(a.Frames) < frames || a.Size != size {
			t.Fatalf("%s requires %d frames at %d pixels", key, frames, size)
		}
	}
	for _, ship := range []string{"classic", "interceptor", "heavy", "stealth"} {
		for _, state := range []string{"idle", "left", "right", "fire", "boost", "super"} {
			n := 4
			if state == "idle" || state == "super" {
				n = 8
			}
			require("player."+ship+"."+state, n, 48)
		}
	}
	for _, enemy := range []string{"bee", "butterfly", "stalker", "sniper", "hunter", "spinner", "bomber", "lasher", "weaver", "splitter", "shield_bee", "kamikaze", "carrier", "teleporter", "boss", "miniboss"} {
		size := atlas.Animations[enemy+".idle"].Size
		if size != 32 && size != 48 && size != 64 {
			t.Fatal(enemy + " size")
		}
		require(enemy+".idle", 8, size)
		require(enemy+".attack", 6, size)
		require(enemy+".damage", 2, size)
	}
	for _, sector := range []string{"nebula", "asteroid", "crystal", "storm", "blackhole", "void"} {
		for phase := 1; phase <= 3; phase++ {
			require(fmt.Sprintf("sector.%s.phase%d", sector, phase), 8, 128)
		}
		require("sector."+sector+".death", 16, 128)
	}
	for i, size := range []string{"small", "medium", "large"} {
		require("fx.explosion."+size, 16, []int{48, 80, 128}[i])
	}
}

func TestGalaxaArcadeTranslations(t *testing.T) {
	var english map[string]string
	if err := json.Unmarshal([]byte(readDesktopAssetText(t, "lang/desktop/en.json")), &english); err != nil {
		t.Fatal(err)
	}
	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		var values map[string]string
		if err := json.Unmarshal([]byte(readDesktopAssetText(t, "lang/desktop/"+lang+".json")), &values); err != nil {
			t.Fatal(err)
		}
		for key := range english {
			if strings.HasPrefix(key, "galaxa.") && strings.TrimSpace(values[key]) == "" {
				t.Errorf("%s missing %s", lang, key)
			}
		}
	}
}
