package persondetect

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"math"

	"golang.org/x/image/draw"
)

// Box drawing of alert photos (spec "Alerts"): a 3 px stroke in a high-contrast yellow with
// a 1 px dark outline on both sides, so a box stays visible on bright and dark scenes.
const (
	annotateStrokePixels  = 3
	annotateOutlinePixels = 1
	annotateJPEGQuality   = 85
	// alertPhotoMaxSide bounds the longest side of an alert photo. It keeps photos well
	// inside Telegram's limits (10 MB, width + height at most 10000) and the encode buffer
	// small; frames are scaled down to it, never up.
	alertPhotoMaxSide = 1920
	// alertPhotoMaxBytes is the largest unannotated frame sent as a photo (Telegram's limit).
	alertPhotoMaxBytes = 10 << 20
)

var (
	annotateStrokeColor  = color.RGBA{R: 0xFF, G: 0xD6, B: 0x0A, A: 0xFF}
	annotateOutlineColor = color.RGBA{R: 0x14, G: 0x14, B: 0x14, A: 0xFF}
)

// AnnotateJPEG returns a new JPEG (quality 85) of jpegData with every box drawn on it. A frame
// whose longest side exceeds alertPhotoMaxSide is scaled down to it first (aspect kept), so
// the boxes are drawn at the photo's own scale and the full-size frame is never copied.
// Boxes are normalized to the frame (0..1) and clamped to it; boxes with NaN/Inf coordinates
// or no area are skipped. jpegData itself is never modified, so stored frames stay
// unannotated. Frames beyond the detector's frame limits (validateFrameSize) are refused
// before they are decoded.
func AnnotateJPEG(jpegData []byte, boxes []Box) ([]byte, error) {
	if len(jpegData) == 0 {
		return nil, errors.New("decode jpeg: no data")
	}
	cfg, err := jpeg.DecodeConfig(bytes.NewReader(jpegData))
	if err != nil {
		return nil, fmt.Errorf("decode jpeg: %w", err)
	}
	if err := validateFrameSize(cfg.Width, cfg.Height); err != nil {
		return nil, fmt.Errorf("decode jpeg: %w", err)
	}
	src, err := jpeg.Decode(bytes.NewReader(jpegData))
	if err != nil {
		return nil, fmt.Errorf("decode jpeg: %w", err)
	}
	bounds := src.Bounds()
	w, h := alertPhotoSize(bounds.Dx(), bounds.Dy())
	canvas := image.NewRGBA(image.Rect(0, 0, w, h))
	if w == bounds.Dx() && h == bounds.Dy() {
		draw.Draw(canvas, canvas.Bounds(), src, bounds.Min, draw.Src)
	} else {
		draw.BiLinear.Scale(canvas, canvas.Bounds(), src, bounds, draw.Src, nil)
	}
	annotateBoxes(canvas, boxes)
	var out bytes.Buffer
	if err := jpeg.Encode(&out, canvas, &jpeg.Options{Quality: annotateJPEGQuality}); err != nil {
		return nil, fmt.Errorf("encode jpeg: %w", err)
	}
	return out.Bytes(), nil
}

// alertPhotoSize is the size of the alert photo of a w×h frame: the frame's own size when
// its longest side is at most alertPhotoMaxSide, otherwise scaled down to that side.
func alertPhotoSize(w, h int) (int, int) {
	longest := max(w, h)
	if longest <= alertPhotoMaxSide {
		return w, h
	}
	scale := float64(alertPhotoMaxSide) / float64(longest)
	return max(1, int(math.Round(float64(w)*scale))), max(1, int(math.Round(float64(h)*scale)))
}

// alertPhotoFits reports whether jpegData can be sent unannotated as an alert photo: a JPEG
// whose header reads, at most alertPhotoMaxSide on its longest side and alertPhotoMaxBytes
// in size. The dispatcher sends a frame AnnotateJPEG could not process only when it fits.
func alertPhotoFits(jpegData []byte) bool {
	if len(jpegData) == 0 || len(jpegData) > alertPhotoMaxBytes {
		return false
	}
	cfg, err := jpeg.DecodeConfig(bytes.NewReader(jpegData))
	return err == nil && cfg.Width > 0 && cfg.Height > 0 && max(cfg.Width, cfg.Height) <= alertPhotoMaxSide
}

// annotateBoxes draws each valid box: first the dark band (outline, stroke and outline),
// then the stroke on top of it.
func annotateBoxes(img *image.RGBA, boxes []Box) {
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	for _, box := range boxes {
		r, ok := annotatePixelRect(box, w, h)
		if !ok {
			continue
		}
		annotateStrokeRect(img, r.Inset(-annotateOutlinePixels), annotateStrokePixels+2*annotateOutlinePixels, annotateOutlineColor)
		annotateStrokeRect(img, r, annotateStrokePixels, annotateStrokeColor)
	}
}

// annotatePixelRect maps a normalized box to pixels: coordinates are clamped to 0..1 first
// (so huge values cannot overflow the int conversion), the left/top edge is floored and the
// right/bottom edge ceiled.
func annotatePixelRect(b Box, w, h int) (image.Rectangle, bool) {
	for _, v := range []float64{b.X1, b.Y1, b.X2, b.Y2} {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return image.Rectangle{}, false
		}
	}
	unit := func(v float64) float64 { return math.Max(0, math.Min(1, v)) }
	x1, x2 := unit(math.Min(b.X1, b.X2)), unit(math.Max(b.X1, b.X2))
	y1, y2 := unit(math.Min(b.Y1, b.Y2)), unit(math.Max(b.Y1, b.Y2))
	r := image.Rect(
		int(math.Floor(x1*float64(w))), int(math.Floor(y1*float64(h))),
		int(math.Ceil(x2*float64(w))), int(math.Ceil(y2*float64(h))),
	)
	if r.Dx() < 1 || r.Dy() < 1 {
		return image.Rectangle{}, false
	}
	return r, true
}

// annotateStrokeRect fills a band of the given thickness along the inside of r, clipped to
// the image.
func annotateStrokeRect(img *image.RGBA, r image.Rectangle, thickness int, c color.RGBA) {
	if thickness <= 0 || r.Empty() {
		return
	}
	fill := image.NewUniform(c)
	for _, side := range []image.Rectangle{
		image.Rect(r.Min.X, r.Min.Y, r.Max.X, r.Min.Y+thickness),
		image.Rect(r.Min.X, r.Max.Y-thickness, r.Max.X, r.Max.Y),
		image.Rect(r.Min.X, r.Min.Y, r.Min.X+thickness, r.Max.Y),
		image.Rect(r.Max.X-thickness, r.Min.Y, r.Max.X, r.Max.Y),
	} {
		draw.Draw(img, side.Intersect(img.Bounds()), fill, image.Point{}, draw.Src)
	}
}
