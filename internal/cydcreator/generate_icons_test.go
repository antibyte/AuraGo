package cydcreator

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// iconFixture covers both encodings, all motion kinds, every export size and buttons.
func iconFixture(t *testing.T, board string) Project {
	t.Helper()
	p, err := NewProject("Icon library", board, "night", "en", "UTC0", &SeqIDs{})
	if err != nil {
		t.Fatal(err)
	}
	library, err := LoadIconLibrary(os.DirFS("../../ui"))
	if err != nil {
		t.Fatal(err)
	}
	requests := []IconRequest{
		{Style: "modern", Tone: "color", Motif: "fan", Size: 32, Animated: true},
		{Style: "playful", Tone: "mono", Motif: "wifi", Size: 32, Animated: true},
		{Style: "modern", Tone: "mono", Motif: "settings", Size: 24, Animated: true},
		{Style: "comic", Tone: "color", Motif: "back", Size: 48, Animated: true},
		{Style: "sober", Tone: "mono", Motif: "home", Size: 64, Animated: true},
		{Style: "modern", Tone: "mono", Motif: "check", Size: 32},
	}
	for i, r := range requests {
		m, err := r.motif()
		if err != nil {
			t.Fatal(err)
		}
		n := 1
		if r.Animated && m.Motion == "frames" {
			n = 6
		}
		var frames []string
		for f := 0; f < n; f++ {
			_, a, err := library.read(r, f)
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, old := range p.Assets {
				if old.ID == a.ID {
					found = true
					break
				}
			}
			if !found {
				p.Assets = append(p.Assets, a)
			}
			frames = append(frames, a.ID)
		}
		w := Widget{ID: fmt.Sprintf("w_icon_%d", i), Type: "image", X: 8 + i*40, Y: 30, W: r.Size, H: r.Size,
			Props: map[string]any{"asset": frames[0], "icon_animate": r.Animated, "icon_motion": m.Motion}}
		if n == 6 {
			w.Props["icon_frames"] = frames
		}
		if i == len(requests)-1 {
			w.Type = "button"
			w.X = 16
			w.Y = 120
			w.W = 140
			w.H = 44
			delete(w.Props, "asset")
			w.Props["icon_asset"] = frames[0]
			w.Props["text"] = "Ready"
			w.Props["icon_tint"] = "#20D070"
		}
		p.Screens[0].Widgets = append(p.Screens[0].Widgets, w)
	}
	return p
}

func iconFixtureAssets(t *testing.T, p Project) map[string][]byte {
	t.Helper()
	assets := goldenAssets(t)
	for _, a := range p.Assets {
		if a.Icon != "" {
			raw, err := os.ReadFile(filepath.Join("../../ui/img/cyd-icons", filepath.FromSlash(strings.TrimPrefix(a.Icon, "1/"))))
			if err != nil {
				t.Fatal(err)
			}
			assets[a.ID] = raw
		}
	}
	return assets
}

func TestGenerateIconGolden(t *testing.T) {
	boards, err := Boards()
	if err != nil {
		t.Fatal(err)
	}
	for _, board := range boards {
		t.Run(board.ID, func(t *testing.T) {
			p := iconFixture(t, board.ID)
			out, err := Generate(GenerateInput{Project: p, ProjectID: testProjectID, Assets: iconFixtureAssets(t, p), AppFiles: ScaffoldAppFiles(p)})
			if err != nil {
				t.Fatalf("%v: %s", err, fmtIssues(out.Issues))
			}
			if board.ID != "esp32-2432s028r" {
				return
			}
			for _, name := range []string{"ui.cpp", "ui_assets.cpp"} {
				path := filepath.Join("testdata/icon-golden", name)
				raw := out.Files["src/generated/"+name]
				if *updateGolden {
					if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(path, raw, 0644); err != nil {
						t.Fatal(err)
					}
				}
				want, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(raw, want) {
					t.Fatalf("%s differs; review and update the icon golden", name)
				}
			}
		})
	}
}
