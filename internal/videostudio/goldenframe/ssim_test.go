package goldenframe

import (
	"image"
	"image/color"
	"math"
	"os"
	"path/filepath"
	"testing"
)

// smoothImage is a low-frequency colour field; shift moves it right by whole pixels.
func smoothImage(width, height, shift int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			fx, fy := float64(x-shift), float64(y)
			img.SetRGBA(x, y, color.RGBA{
				R: uint8(math.Round(128 + 60*math.Sin(2*math.Pi*fx/64) + 30*math.Cos(2*math.Pi*fy/48))),
				G: uint8(math.Round(128 + 70*math.Sin(2*math.Pi*(fy/40+fx/90)))),
				B: uint8(math.Round(128 + 80*math.Cos(2*math.Pi*(fx+fy)/120))),
				A: 255,
			})
		}
	}
	return img
}

func TestSSIMScoresIdenticalShiftedAndHueShiftedImages(t *testing.T) {
	want := smoothImage(64, 64, 0)
	same, err := SSIM(want, smoothImage(64, 64, 0))
	if err != nil || math.Abs(same.R-1) > 1e-9 || math.Abs(same.G-1) > 1e-9 || math.Abs(same.B-1) > 1e-9 || math.Abs(same.Min-1) > 1e-9 {
		t.Fatalf("identical images: %+v, %v", same, err)
	}
	shifted, err := SSIM(want, smoothImage(64, 64, 1))
	if err != nil || shifted.Min < 0.98 || shifted.Min >= 1 {
		t.Fatalf("smooth one-pixel shift: %+v, %v (want 0.98 <= Min < 1)", shifted, err)
	}
	rotated := image.NewRGBA(want.Bounds())
	for i := 0; i < len(want.Pix); i += 4 {
		rotated.Pix[i], rotated.Pix[i+1], rotated.Pix[i+2], rotated.Pix[i+3] = want.Pix[i+1], want.Pix[i+2], want.Pix[i], 255
	}
	hue, err := SSIM(want, rotated)
	if err != nil || hue.Min >= ThresholdColour || hue.Min != math.Min(hue.R, math.Min(hue.G, hue.B)) {
		t.Fatalf("hue shift: %+v, %v (want Min below %.2f and equal to the smallest channel)", hue, err, ThresholdColour)
	}
	if _, err := SSIM(want, smoothImage(64, 63, 0)); err == nil {
		t.Fatal("images of different size were compared")
	}
	if _, err := SSIM(smoothImage(8, 8, 0), smoothImage(8, 8, 0)); err == nil {
		t.Fatal("images smaller than the SSIM window were compared")
	}
}

func TestDecodeRGBAAndWriteArtifacts(t *testing.T) {
	data := make([]byte, 3*2*4)
	data[4], data[5], data[6], data[7] = 10, 20, 30, 255 // pixel (1, 0)
	img, err := DecodeRGBA(3, 2, data)
	if err != nil || img.RGBAAt(1, 0) != (color.RGBA{R: 10, G: 20, B: 30, A: 255}) {
		t.Fatalf("DecodeRGBA = %v, %v", img, err)
	}
	if _, err := DecodeRGBA(3, 2, data[:20]); err == nil {
		t.Fatal("short RGBA data was accepted")
	}
	dir := filepath.Join(t.TempDir(), "out")
	if err := WriteArtifacts(dir, "geometry_full/gl", smoothImage(16, 16, 0), smoothImage(16, 16, 1)); err != nil {
		t.Fatal(err)
	}
	for _, suffix := range []string{"_want.png", "_got.png", "_diff.png"} {
		if _, err := os.Stat(filepath.Join(dir, "geometry_full_gl"+suffix)); err != nil {
			t.Errorf("artifact %s: %v", suffix, err)
		}
	}
}
