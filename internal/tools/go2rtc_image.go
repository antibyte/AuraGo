package tools

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"math"

	"golang.org/x/image/draw"
)

// transformGo2RTCJPEG gives every input codec the same bounded JPEG semantics.
// The bounds are maximum dimensions; small camera images are never enlarged.
func transformGo2RTCJPEG(data []byte, opts Go2RTCSnapshotOptions) ([]byte, error) {
	if opts.Width == 0 && opts.Height == 0 && opts.Rotate == 0 {
		return data, nil
	}
	info, err := jpeg.DecodeConfig(bytes.NewReader(data))
	if err != nil || info.Width < 1 || info.Height < 1 || info.Width > 8192 || info.Height > 8192 || int64(info.Width)*int64(info.Height) > 40_000_000 {
		return nil, fmt.Errorf("camera image has invalid or excessive dimensions")
	}
	if opts.Rotate == 0 && (opts.Width == 0 || opts.Width >= info.Width) && (opts.Height == 0 || opts.Height >= info.Height) {
		return data, nil
	}
	img, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode camera JPEG: %w", err)
	}
	w, h := info.Width, info.Height
	if opts.Rotate != 0 {
		rw, rh := w, h
		if opts.Rotate == 90 || opts.Rotate == 270 {
			rw, rh = h, w
		}
		rotated := image.NewRGBA(image.Rect(0, 0, rw, rh))
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				switch opts.Rotate {
				case 90:
					rotated.Set(h-1-y, x, img.At(x, y))
				case 180:
					rotated.Set(w-1-x, h-1-y, img.At(x, y))
				case 270:
					rotated.Set(y, w-1-x, img.At(x, y))
				}
			}
		}
		img, w, h = rotated, rw, rh
	}
	ratio := 1.0
	if opts.Width > 0 {
		ratio = math.Min(ratio, float64(opts.Width)/float64(w))
	}
	if opts.Height > 0 {
		ratio = math.Min(ratio, float64(opts.Height)/float64(h))
	}
	if ratio < 1 {
		scaled := image.NewRGBA(image.Rect(0, 0, max(1, int(float64(w)*ratio)), max(1, int(float64(h)*ratio))))
		draw.ApproxBiLinear.Scale(scaled, scaled.Bounds(), img, img.Bounds(), draw.Src, nil)
		img = scaled
	}
	var out bytes.Buffer
	if err := jpeg.Encode(&out, img, &jpeg.Options{Quality: 85}); err != nil {
		return nil, fmt.Errorf("encode camera JPEG: %w", err)
	}
	return out.Bytes(), nil
}
