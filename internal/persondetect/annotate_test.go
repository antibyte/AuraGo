package persondetect

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"math"
	"testing"
)

var annotateTestBackground = color.RGBA{R: 0x80, G: 0x80, B: 0x80, A: 0xFF}

func annotateTestCanvas(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, annotateTestBackground)
		}
	}
	return img
}

func annotateTestJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, annotateTestCanvas(w, h), &jpeg.Options{Quality: 95}); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func annotateTestLuma(c color.Color) int {
	return int(color.GrayModel.Convert(c).(color.Gray).Y)
}

// Box (0.25,0.25)-(0.75,0.75) on 200×100 is the pixel rectangle (50,25)-(150,75). The
// 3 px stroke covers x 50..52, the 1 px outline x 49 and 53 (mirrored on every side).
func TestAnnotateBoxesDrawsStrokeWithDarkOutline(t *testing.T) {
	img := annotateTestCanvas(200, 100)
	annotateBoxes(img, []Box{{X1: 0.25, Y1: 0.25, X2: 0.75, Y2: 0.75, Score: 0.9}})
	cases := []struct {
		x, y int
		want color.RGBA
	}{
		{49, 50, annotateOutlineColor}, {50, 50, annotateStrokeColor}, {52, 50, annotateStrokeColor},
		{53, 50, annotateOutlineColor}, {54, 50, annotateTestBackground},
		{146, 50, annotateOutlineColor}, {147, 50, annotateStrokeColor}, {149, 50, annotateStrokeColor},
		{150, 50, annotateOutlineColor}, {151, 50, annotateTestBackground},
		{100, 24, annotateOutlineColor}, {100, 25, annotateStrokeColor}, {100, 27, annotateStrokeColor},
		{100, 28, annotateOutlineColor}, {100, 29, annotateTestBackground},
		{100, 71, annotateOutlineColor}, {100, 72, annotateStrokeColor}, {100, 74, annotateStrokeColor},
		{100, 75, annotateOutlineColor}, {100, 76, annotateTestBackground},
		{100, 50, annotateTestBackground}, {10, 10, annotateTestBackground},
	}
	for _, c := range cases {
		if got := img.RGBAAt(c.x, c.y); got != c.want {
			t.Errorf("pixel (%d,%d) = %v, want %v", c.x, c.y, got, c.want)
		}
	}
}

func TestAnnotateBoxesClampsAndSkipsInvalidBoxes(t *testing.T) {
	img := annotateTestCanvas(40, 20)
	annotateBoxes(img, []Box{
		{X1: math.NaN(), Y1: 0, X2: 1, Y2: 1},
		{X1: 0.5, Y1: 0.5, X2: 0.5, Y2: 0.9}, // zero width
		{X1: math.Inf(1), Y1: 0, X2: 1, Y2: 1},
	})
	for y := 0; y < 20; y++ {
		for x := 0; x < 40; x++ {
			if got := img.RGBAAt(x, y); got != annotateTestBackground {
				t.Fatalf("invalid boxes drew pixel (%d,%d) = %v", x, y, got)
			}
		}
	}
	annotateBoxes(img, []Box{{X1: -0.5, Y1: -0.5, X2: 1.5, Y2: 1.5, Score: 0.7}})
	if got := img.RGBAAt(0, 10); got != annotateStrokeColor {
		t.Fatalf("clamped stroke at the left edge = %v", got)
	}
	if got := img.RGBAAt(3, 10); got != annotateOutlineColor {
		t.Fatalf("inner outline of a clamped box = %v", got)
	}
	if got := img.RGBAAt(20, 10); got != annotateTestBackground {
		t.Fatalf("center of a clamped box = %v", got)
	}
}

func TestAnnotateJPEGKeepsSizeAndInput(t *testing.T) {
	src := annotateTestJPEG(t, 200, 100)
	original := append([]byte(nil), src...)
	out, err := AnnotateJPEG(src, []Box{{X1: 0.25, Y1: 0.25, X2: 0.75, Y2: 0.75, Score: 0.9}})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(src, original) {
		t.Fatal("AnnotateJPEG modified its input")
	}
	img, err := jpeg.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("annotated output is not a JPEG: %v", err)
	}
	if b := img.Bounds(); b.Dx() != 200 || b.Dy() != 100 {
		t.Fatalf("annotated size = %v, want 200x100", b)
	}
	if luma := annotateTestLuma(img.At(51, 50)); luma < 170 {
		t.Fatalf("stroke luma = %d, want a bright stroke", luma)
	}
	if r, _, b, _ := img.At(51, 50).RGBA(); int(r>>8)-int(b>>8) < 60 {
		t.Fatalf("stroke pixel is not yellow: r=%d b=%d", r>>8, b>>8)
	}
	if luma := annotateTestLuma(img.At(100, 50)); luma < 0x80-12 || luma > 0x80+12 {
		t.Fatalf("box center luma = %d, want the unchanged background", luma)
	}
	if luma := annotateTestLuma(img.At(49, 50)); luma > 100 {
		t.Fatalf("outline luma = %d, want a dark outline", luma)
	}
}

func TestAnnotateJPEGWithoutBoxesAndInvalidInput(t *testing.T) {
	out, err := AnnotateJPEG(annotateTestJPEG(t, 32, 16), nil)
	if err != nil {
		t.Fatal(err)
	}
	if cfg, err := jpeg.DecodeConfig(bytes.NewReader(out)); err != nil || cfg.Width != 32 || cfg.Height != 16 {
		t.Fatalf("output config = %+v, %v", cfg, err)
	}
	if _, err := AnnotateJPEG([]byte("not a jpeg"), nil); err == nil {
		t.Fatal("invalid input must fail")
	}
	if _, err := AnnotateJPEG(nil, nil); err == nil {
		t.Fatal("empty input must fail")
	}
}

// Adaptation: AnnotateJPEG shares the detector's frame limits (validateFrameSize) instead
// of a pixel cap of its own, so a tall frame is refused before it is decoded.
func TestAnnotateJPEGRefusesFramesBeyondTheFrameLimits(t *testing.T) {
	tall := annotateTestJPEG(t, 1, maxFrameSide+1)
	if _, err := AnnotateJPEG(tall, nil); err == nil {
		t.Fatalf("a 1x%d frame must be refused", maxFrameSide+1)
	}
}

// Add-on: a frame larger than alertPhotoMaxSide comes out scaled down (aspect kept), still a
// JPEG, with the box drawn at the photo's own scale.
func TestAnnotateJPEGScalesLargeFramesDown(t *testing.T) {
	out, err := AnnotateJPEG(annotateTestJPEG(t, 8192, 2048), []Box{{X1: 0.25, Y1: 0.25, X2: 0.75, Y2: 0.75, Score: 0.9}})
	if err != nil {
		t.Fatal(err)
	}
	img, err := jpeg.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("scaled output is not a JPEG: %v", err)
	}
	if b := img.Bounds(); b.Dx() != alertPhotoMaxSide || b.Dy() != 480 {
		t.Fatalf("photo size = %v, want %dx480", b, alertPhotoMaxSide)
	}
	// The box is (480,120)-(1440,360) on the 1920×480 photo: stroke from x 480, outline at 479.
	if r, _, b, _ := img.At(481, 240).RGBA(); annotateTestLuma(img.At(481, 240)) < 170 || int(r>>8)-int(b>>8) < 60 {
		t.Fatalf("stroke pixel = %v, want the yellow stroke", img.At(481, 240))
	}
	if luma := annotateTestLuma(img.At(478, 240)); luma < 0x80-12 || luma > 0x80+12 {
		t.Fatalf("pixel left of the outline luma = %d, want the background", luma)
	}
	if luma := annotateTestLuma(img.At(960, 240)); luma < 0x80-12 || luma > 0x80+12 {
		t.Fatalf("box center luma = %d, want the background", luma)
	}
	if w, h := alertPhotoSize(1280, 720); w != 1280 || h != 720 {
		t.Fatalf("a 1280x720 frame must keep its size, got %dx%d", w, h)
	}
}
