package goldenframe

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
)

// ArtifactsEnv names the environment variable that holds the directory for want/got/diff PNGs of failed cases.
const ArtifactsEnv = "AURAGO_GOLDEN_OUT"

// DecodeRGBA wraps top-down 8-bit RGBA bytes (a painter read-back) as an image.
func DecodeRGBA(width, height int, data []byte) (*image.RGBA, error) {
	if width <= 0 || height <= 0 || width > 16384 || height > 16384 {
		return nil, fmt.Errorf("RGBA size %dx%d is not supported", width, height)
	}
	if len(data) != width*height*4 {
		return nil, fmt.Errorf("RGBA data has %d bytes, want %d for %dx%d", len(data), width*height*4, width, height)
	}
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	copy(img.Pix, data)
	return img, nil
}

// WriteArtifacts writes <name>_want.png, <name>_got.png and, when the sizes match, <name>_diff.png (absolute
// difference per channel, amplified four times) into dir. The runner calls it for failed cases when ArtifactsEnv is set.
func WriteArtifacts(dir, name string, want, got image.Image) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	base := artifactName(name)
	if err := writePNG(filepath.Join(dir, base+"_want.png"), want); err != nil {
		return err
	}
	if err := writePNG(filepath.Join(dir, base+"_got.png"), got); err != nil {
		return err
	}
	if want.Bounds().Size() != got.Bounds().Size() {
		return nil
	}
	return writePNG(filepath.Join(dir, base+"_diff.png"), diffImage(want, got))
}

func artifactName(name string) string {
	clean := strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-' {
			return r
		}
		return '_'
	}, name)
	if clean == "" {
		return "case"
	}
	return clean
}

func diffImage(want, got image.Image) *image.NRGBA {
	wb, gb := want.Bounds(), got.Bounds()
	out := image.NewNRGBA(image.Rect(0, 0, wb.Dx(), wb.Dy()))
	for y := 0; y < wb.Dy(); y++ {
		for x := 0; x < wb.Dx(); x++ {
			a := color.NRGBAModel.Convert(want.At(wb.Min.X+x, wb.Min.Y+y)).(color.NRGBA)
			b := color.NRGBAModel.Convert(got.At(gb.Min.X+x, gb.Min.Y+y)).(color.NRGBA)
			out.SetNRGBA(x, y, color.NRGBA{R: amplified(a.R, b.R), G: amplified(a.G, b.G), B: amplified(a.B, b.B), A: 255})
		}
	}
	return out
}

func amplified(a, b uint8) uint8 {
	d := int(a) - int(b)
	if d < 0 {
		d = -d
	}
	if d*4 > 255 {
		return 255
	}
	return uint8(d * 4)
}

func writePNG(path string, img image.Image) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := png.Encode(file, img); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}
