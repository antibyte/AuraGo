package tools

import (
	"bytes"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"

	"golang.org/x/image/draw"
)

// UpscaleMaxDimension caps output width/height for server-side upscaling.
const UpscaleMaxDimension = 4096

// UpscaleResult holds PNG-encoded upscaled image bytes and dimensions.
type UpscaleResult struct {
	PNGBytes []byte
	Width    int
	Height   int
}

// UpscaleImageBytes decodes image bytes, scales by factor (typically 2), and returns PNG.
func UpscaleImageBytes(src []byte, factor float64) (*UpscaleResult, error) {
	if len(src) == 0 {
		return nil, fmt.Errorf("empty image data")
	}
	if factor <= 1 {
		factor = 2
	}

	img, _, err := image.Decode(bytes.NewReader(src))
	if err != nil {
		return nil, fmt.Errorf("decode image: %w", err)
	}

	bounds := img.Bounds()
	origW, origH := bounds.Dx(), bounds.Dy()
	if origW < 1 || origH < 1 {
	 return nil, fmt.Errorf("invalid image dimensions")
	}

	newW := int(float64(origW) * factor)
	newH := int(float64(origH) * factor)
	if newW > UpscaleMaxDimension || newH > UpscaleMaxDimension {
		return nil, fmt.Errorf("upscaled dimensions %dx%d exceed max %d", newW, newH, UpscaleMaxDimension)
	}

	dst := image.NewRGBA(image.Rect(0, 0, newW, newH))
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, bounds, draw.Over, nil)

	var buf bytes.Buffer
	if err := png.Encode(&buf, dst); err != nil {
		return nil, fmt.Errorf("encode png: %w", err)
	}

	return &UpscaleResult{
		PNGBytes: buf.Bytes(),
		Width:    newW,
		Height:   newH,
	}, nil
}
