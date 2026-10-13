package tools

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"strings"

	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

const (
	visionMaxDecodedPixels = 4_000_000
	visionMaxEdge          = 1600
	visionJPEGQuality      = 85
	visionMaxPNGBytes      = 1_500_000
)

func init() {
	image.RegisterFormat("gif", "GIF8", gif.Decode, gif.DecodeConfig)
}

// PrepareVisionDataURL decodes one image and returns a JPEG or PNG data URL
// sized for a vision request. declaredMIME is the caller's type hint; the
// encoded bytes decide the type in the URL.
func PrepareVisionDataURL(raw []byte, declaredMIME string) (string, error) {
	_ = declaredMIME
	img, format, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return "", fmt.Errorf("image decode failed")
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= 0 || h <= 0 {
		return "", fmt.Errorf("invalid decoded bounds")
	}
	if w*h > visionMaxDecodedPixels {
		return "", fmt.Errorf("decoded image too large: %dx%d (%d pixels)", w, h, w*h)
	}
	if w > visionMaxEdge || h > visionMaxEdge {
		var targetW, targetH int
		if w >= h {
			targetW = visionMaxEdge
			targetH = int(float64(h) * (float64(visionMaxEdge) / float64(w)))
		} else {
			targetH = visionMaxEdge
			targetW = int(float64(w) * (float64(visionMaxEdge) / float64(h)))
		}
		if targetW < 1 {
			targetW = 1
		}
		if targetH < 1 {
			targetH = 1
		}
		dst := image.NewRGBA(image.Rect(0, 0, targetW, targetH))
		draw.CatmullRom.Scale(dst, dst.Bounds(), img, b, draw.Over, nil)
		img = dst
	}
	hasAlpha := visionImageHasAlpha(img)
	if strings.EqualFold(format, "png") && !hasAlpha {
		var buf bytes.Buffer
		if err := png.Encode(&buf, img); err == nil && buf.Len() <= visionMaxPNGBytes {
			return fmt.Sprintf("data:image/png;base64,%s", base64.StdEncoding.EncodeToString(buf.Bytes())), nil
		}
	}
	if hasAlpha {
		img = visionFlattenAlpha(img, color.White)
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: visionJPEGQuality}); err != nil {
		return "", fmt.Errorf("jpeg encode: %w", err)
	}
	return fmt.Sprintf("data:image/jpeg;base64,%s", base64.StdEncoding.EncodeToString(buf.Bytes())), nil
}

func visionImageHasAlpha(img image.Image) bool {
	switch i := img.(type) {
	case *image.NRGBA:
		return true
	case *image.NRGBA64:
		return true
	case *image.RGBA:
		b := i.Bounds()
		stepX := max(1, b.Dx()/32)
		stepY := max(1, b.Dy()/32)
		for y := b.Min.Y; y < b.Max.Y; y += stepY {
			for x := b.Min.X; x < b.Max.X; x += stepX {
				_, _, _, a := i.At(x, y).RGBA()
				if a != 0xffff {
					return true
				}
			}
		}
		return false
	default:
		return true
	}
}

func visionFlattenAlpha(src image.Image, bg color.Color) image.Image {
	b := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(dst, dst.Bounds(), &image.Uniform{C: bg}, image.Point{}, draw.Src)
	draw.Draw(dst, dst.Bounds(), src, b.Min, draw.Over)
	return dst
}
