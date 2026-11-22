package storymaker

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
)

// stripedImage is a w x h picture of the background grey with vertical strokes of the stroke grey, two pixels wide at every
// fourth column, inside the pixel rectangle box. Outside the box the picture is black.
func stripedImage(w, h int, box image.Rectangle, bg, stroke uint8) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			v := uint8(0)
			if image.Pt(x, y).In(box) {
				v = bg
				if x%4 < 2 {
					v = stroke
				}
			}
			img.Set(x, y, color.RGBA{v, v, v, 255})
		}
	}
	return img
}

func encodePNG(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestBoxContrastReadsStrokesAgainstTheGround(t *testing.T) {
	full := image.Rect(0, 0, 200, 100)
	box := LayoutBox{0, 0, 1, 1}
	cases := []struct {
		name       string
		bg, stroke uint8
		min, max   float64
	}{
		{"black strokes on white", 255, 0, 15, 22},
		{"mid grey on slightly lighter grey", 150, 128, 1.15, 3},
		{"uniform grey", 128, 128, 1, 1.0001},
	}
	for _, c := range cases {
		ratio, ok := boxContrast(stripedImage(200, 100, full, c.bg, c.stroke), box)
		if !ok || ratio < c.min || ratio > c.max {
			t.Errorf("%s: ratio %.3f ok %v, want %.2f..%.2f", c.name, ratio, ok, c.min, c.max)
		}
	}
}

func TestBoxContrastClampsTheBoxAndSkipsATinyOne(t *testing.T) {
	img := stripedImage(200, 100, image.Rect(0, 0, 100, 100), 255, 0)
	// The box sticks out of the frame on the right: only the part inside is measured, and it is the striped half.
	if ratio, ok := boxContrast(img, LayoutBox{0.2, 0.2, 2, 0.5}); !ok || ratio < 15 {
		t.Errorf("a clamped box: ratio %.3f ok %v", ratio, ok)
	}
	if _, ok := boxContrast(img, LayoutBox{0.2, 0.2, 0.004, 0.5}); ok {
		t.Error("a box under 2 pixels wide must be skipped")
	}
	if _, ok := boxContrast(img, LayoutBox{1.5, 0.2, 0.2, 0.2}); ok {
		t.Error("a box outside the frame must be skipped")
	}
}

func TestMeasureStillContrastMeasuresTextsAndCaption(t *testing.T) {
	// Left half: black strokes on white (the text "title"); right half uniform grey (the text "note"); the caption band is striped
	// grey on slightly lighter grey.
	img := stripedImage(200, 100, image.Rect(0, 0, 100, 60), 255, 0)
	for y := 0; y < 60; y++ {
		for x := 100; x < 200; x++ {
			img.Set(x, y, color.RGBA{128, 128, 128, 255})
		}
	}
	band := stripedImage(200, 100, image.Rect(0, 70, 200, 100), 150, 128)
	for y := 70; y < 100; y++ {
		for x := 0; x < 200; x++ {
			img.Set(x, y, band.At(x, y))
		}
	}
	scene := layoutScene("s1", 0)
	scene.Texts = []LayoutText{{ID: "title", Box: LayoutBox{0, 0, 0.5, 0.6}}, {ID: "note", Box: LayoutBox{0.5, 0, 0.5, 0.6}}}
	caption := LayoutBox{0, 0.7, 1, 0.3}
	scene.Caption = &caption
	got, err := MeasureStillContrast(encodePNG(t, img), scene)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got[0].ID != "title" || got[1].ID != "note" || got[2].ID != "caption" {
		t.Fatalf("measured %+v, want title, note, caption in order", got)
	}
	if got[0].Ratio < 15 || got[1].Ratio > 1.0001 || got[2].Ratio < 1.15 || got[2].Ratio >= contrastMin {
		t.Errorf("ratios %+v", got)
	}
}

func TestMeasureStillContrastSkipsATinyBoxAndRefusesANonPNG(t *testing.T) {
	scene := layoutScene("s1", 0)
	scene.Texts = []LayoutText{{ID: "dot", Box: LayoutBox{0.5, 0.5, 0.001, 0.001}}}
	got, err := MeasureStillContrast(encodePNG(t, stripedImage(200, 100, image.Rect(0, 0, 200, 100), 255, 0)), scene)
	if err != nil || len(got) != 0 {
		t.Errorf("a tiny box: %+v, %v", got, err)
	}
	if _, err := MeasureStillContrast([]byte("not a png"), scene); err == nil {
		t.Error("a non-PNG must be refused")
	}
}
