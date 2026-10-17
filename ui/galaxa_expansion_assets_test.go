package ui

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"image"
	"image/png"
	"strings"
	"testing"
)

func TestGalaxaExpansionAssets(t *testing.T) {
	var atlas galaxaAtlas
	if err := json.Unmarshal([]byte(readDesktopAssetText(t, "img/galaxa/atlas.json")), &atlas); err != nil {
		t.Fatal(err)
	}
	var provenance struct {
		Files []struct {
			File, SHA256, Source, Prompt string
			Bytes                        int
		}
	}
	if err := json.Unmarshal([]byte(readDesktopAssetText(t, "img/galaxa/provenance.json")), &provenance); err != nil {
		t.Fatal(err)
	}
	verified := map[string]bool{}
	for _, source := range provenance.Files {
		if !strings.HasPrefix(source.File, "exp-") {
			continue
		}
		data, err := Content.ReadFile("img/galaxa/" + source.File)
		if err != nil {
			t.Fatal(err)
		}
		hash := sha256.Sum256(data)
		if hex.EncodeToString(hash[:]) != source.SHA256 || len(data) != source.Bytes || source.Source == "" || source.Prompt == "" {
			t.Fatalf("incomplete provenance: %s", source.File)
		}
		verified[source.File] = true
	}
	if len(verified) != 70 || len(atlas.Expansion.Motifs) != 50 {
		t.Fatal("expected 50 motifs and 70 original sheets")
	}
	images := map[string]image.Image{}
	seen, groups := map[string]bool{}, map[string]int{}
	for i, motif := range atlas.Expansion.Motifs {
		if motif.Number != i+1 || seen[motif.Kind+motif.ID] || motif.Usage == "" {
			t.Fatalf("invalid motif: %+v", motif)
		}
		seen[motif.Kind+motif.ID] = true
		groups[motif.Kind]++
		counts := []int{12}
		switch motif.Kind {
		case "asteroid":
			counts = []int{16}
		case "alien":
			counts = []int{8, 6, 2}
		case "weapon":
			counts = []int{8}
			if strings.Contains(motif.ID, "mine") {
				counts = append(counts, 4)
			}
		}
		if len(motif.Animations) != len(counts) {
			t.Fatalf("missing states: %s", motif.ID)
		}
		for state, key := range motif.Animations {
			a, ok := atlas.Animations[key]
			if !ok || len(a.Frames) < counts[state] || len(a.SourceAnchors) != len(a.Frames) {
				t.Fatalf("incomplete animation: %s", key)
			}
			unique := map[[32]byte]bool{}
			minDiameter, maxDiameter := 100000, 0
			for frame, rect := range a.Frames {
				id := a.Sheet
				if len(a.FrameSheets) > 0 {
					id = a.FrameSheets[frame]
				}
				sheet := atlas.Sheets[id]
				if !verified[sheet.File] || (motif.Kind == "planet") != (sheet.Optional == "planet") {
					t.Fatalf("sheet policy: %s", id)
				}
				im := images[id]
				if im == nil {
					f, err := Content.Open("img/galaxa/" + sheet.File)
					if err != nil {
						t.Fatal(err)
					}
					im, err = png.Decode(f)
					f.Close()
					if err != nil {
						t.Fatal(err)
					}
					images[id] = im
				}
				if motif.Kind == "planet" && (rect[2] < 512 || rect[3] < 512) {
					t.Fatalf("planet resolution: %s", key)
				}
				pixels := make([]byte, 0, rect[2]*rect[3]*4)
				minX, minY, maxX, maxY, edges := rect[2], rect[3], 0, 0, 0
				for y := 0; y < rect[3]; y++ {
					for x := 0; x < rect[2]; x++ {
						r, g, b, alpha := im.At(rect[0]+x, rect[1]+y).RGBA()
						pixels = append(pixels, byte(r>>8), byte(g>>8), byte(b>>8), byte(alpha>>8))
						if alpha < 8192 {
							continue
						}
						minX = min(minX, x)
						maxX = max(maxX, x)
						minY = min(minY, y)
						maxY = max(maxY, y)
						if x == 0 || y == 0 || x == rect[2]-1 || y == rect[3]-1 {
							edges++
						}
					}
				}
				if edges > 12 || maxX <= minX || maxY <= minY {
					t.Fatalf("clipped/empty frame %s/%d: %d edge pixels", key, frame, edges)
				}
				anchor := [2]float64{float64(minX+maxX+1) / 2, float64(minY+maxY+1) / 2}
				if a.SourceAnchors[frame] != anchor {
					t.Fatalf("unstable anchor: %s/%d", key, frame)
				}
				diameter := max(maxX-minX+1, maxY-minY+1)
				minDiameter = min(minDiameter, diameter)
				maxDiameter = max(maxDiameter, diameter)
				hash := sha256.Sum256(pixels)
				if unique[hash] {
					t.Fatalf("duplicated frame: %s/%d", key, frame)
				}
				unique[hash] = true
			}
			if motif.Kind == "planet" && float64(maxDiameter)/float64(minDiameter) > 1.18 {
				t.Fatalf("planet size changes across frames: %s (%d..%d)", key, minDiameter, maxDiameter)
			}
		}
	}
	for kind, count := range map[string]int{"asteroid": 12, "alien": 12, "weapon": 6, "planet": 10, "effect": 10} {
		if groups[kind] != count {
			t.Errorf("%s: expected %d, got %d", kind, count, groups[kind])
		}
	}
}
