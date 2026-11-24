package ui

import (
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestHomeDashLabColourVisionSheets writes deuteranopia, protanopia and greyscale copies of the HomeDash hero sheets
// (M1 review rubric U4/U5) from a previous control-lab run. No browser. Opt-in: HOMEDASH_LAB_CVD=1.
func TestHomeDashLabColourVisionSheets(t *testing.T) {
	if os.Getenv("HOMEDASH_LAB_CVD") != "1" {
		t.Skip("set HOMEDASH_LAB_CVD=1 after a control-lab run")
	}
	dir := filepath.Join("..", "reports", "homedash-lab")
	all, err := filepath.Glob(filepath.Join(dir, "hero-*.png"))
	if err != nil || len(all) == 0 {
		t.Fatalf("no hero sheets in %s (err %v)", dir, err)
	}
	// Machado, Oliveira and Fernandes (2009), severity 1.0, applied in linear RGB.
	protan := [3][3]float64{{0.152286, 1.052583, -0.204868}, {0.114503, 0.786281, 0.099216}, {-0.003882, -0.048116, 1.051998}}
	deutan := [3][3]float64{{0.367322, 0.860646, -0.227968}, {0.280085, 0.672501, 0.047413}, {-0.011820, 0.042940, 0.968881}}
	var lin [256]float64
	for i := range lin {
		c := float64(i) / 255
		if c <= 0.04045 {
			lin[i] = c / 12.92
		} else {
			lin[i] = math.Pow((c+0.055)/1.055, 2.4)
		}
	}
	enc := func(v float64) uint8 {
		v = math.Max(0, math.Min(1, v))
		if v <= 0.0031308 {
			v *= 12.92
		} else {
			v = 1.055*math.Pow(v, 1/2.4) - 0.055
		}
		return uint8(math.Round(v * 255))
	}
	mul := func(m [3][3]float64, r, g, b float64) (float64, float64, float64) {
		return m[0][0]*r + m[0][1]*g + m[0][2]*b, m[1][0]*r + m[1][1]*g + m[1][2]*b, m[2][0]*r + m[2][1]*g + m[2][2]*b
	}
	transforms := map[string]func(r, g, b float64) (float64, float64, float64){
		"cvd-protan-": func(r, g, b float64) (float64, float64, float64) { return mul(protan, r, g, b) },
		"cvd-deutan-": func(r, g, b float64) (float64, float64, float64) { return mul(deutan, r, g, b) },
		"grey-":       func(r, g, b float64) (float64, float64, float64) { y := 0.2126*r + 0.7152*g + 0.0722*b; return y, y, y },
	}
	count := 0
	for _, path := range all {
		base := filepath.Base(path)
		if strings.HasPrefix(base, "cvd-") || strings.HasPrefix(base, "grey-") {
			continue
		}
		in, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		src, err := png.Decode(in)
		in.Close()
		if err != nil {
			t.Fatalf("decode %s: %v", path, err)
		}
		b := src.Bounds()
		for prefix, fn := range transforms {
			dst := image.NewNRGBA(b)
			for y := b.Min.Y; y < b.Max.Y; y++ {
				for x := b.Min.X; x < b.Max.X; x++ {
					c := color.NRGBAModel.Convert(src.At(x, y)).(color.NRGBA)
					r, g, bl := fn(lin[c.R], lin[c.G], lin[c.B])
					dst.SetNRGBA(x, y, color.NRGBA{R: enc(r), G: enc(g), B: enc(bl), A: c.A})
				}
			}
			out, err := os.Create(filepath.Join(dir, prefix+base))
			if err != nil {
				t.Fatal(err)
			}
			if err := png.Encode(out, dst); err != nil {
				out.Close()
				t.Fatal(err)
			}
			out.Close()
		}
		count++
	}
	t.Logf("wrote colour-vision and greyscale copies of %d sheets", count)
}
