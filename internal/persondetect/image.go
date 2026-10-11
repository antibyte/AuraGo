package persondetect

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"math"
	"sort"

	"golang.org/x/image/draw"
)

// Box is a person detection normalized to the frame (0..1).
type Box struct {
	X1    float64 `json:"x1"`
	Y1    float64 `json:"y1"`
	X2    float64 `json:"x2"`
	Y2    float64 `json:"y2"`
	Score float64 `json:"score"`
}

// maxFramePixels bounds JPEG decoding (8K is 33 Mpx).
const maxFramePixels = 40_000_000

// Preprocess decodes a JPEG, resizes it to size×size with x/image/draw's
// antialiased BiLinear kernel (no letterbox, like PIL's BILINEAR resize used
// for the reference numbers) and returns CHW RGB float32 values /255 plus the
// original frame size. Grayscale and CMYK JPEGs are converted to RGB by the
// resize; anything that is not a complete JPEG is an error.
func Preprocess(jpegData []byte, size int) (pixels []float32, width, height int, err error) {
	if size <= 0 {
		return nil, 0, 0, fmt.Errorf("input size must be positive")
	}
	if len(jpegData) == 0 {
		return nil, 0, 0, fmt.Errorf("image is empty")
	}
	config, err := jpeg.DecodeConfig(bytes.NewReader(jpegData))
	if err != nil {
		return nil, 0, 0, fmt.Errorf("decode JPEG header: %w", err)
	}
	if config.Width <= 0 || config.Height <= 0 || int64(config.Width)*int64(config.Height) > maxFramePixels {
		return nil, 0, 0, fmt.Errorf("JPEG size %dx%d is not supported", config.Width, config.Height)
	}
	source, err := jpeg.Decode(bytes.NewReader(jpegData))
	if err != nil {
		return nil, 0, 0, fmt.Errorf("decode JPEG: %w", err)
	}
	bounds := source.Bounds()
	scaled := image.NewRGBA(image.Rect(0, 0, size, size))
	draw.BiLinear.Scale(scaled, scaled.Bounds(), source, bounds, draw.Src, nil)
	plane := size * size
	pixels = make([]float32, 3*plane)
	for y := 0; y < size; y++ {
		row := scaled.Pix[y*scaled.Stride : y*scaled.Stride+4*size]
		for x := 0; x < size; x++ {
			index := y*size + x
			pixels[index] = float32(row[4*x]) / 255
			pixels[plane+index] = float32(row[4*x+1]) / 255
			pixels[2*plane+index] = float32(row[4*x+2]) / 255
		}
	}
	return pixels, bounds.Dx(), bounds.Dy(), nil
}

// DecodePersons turns D-FINE outputs (logits [queries×classes], boxes
// [queries×4] as cx, cy, w, h) into person boxes with sigmoid score ≥
// minScore, clamped to 0..1, degenerate boxes dropped, sorted by score
// (descending). Queries with a non-finite logit or box value never produce a
// box. Invalid sizes, or slices shorter than queries×classes / queries×4,
// return nil; trailing extra values are ignored.
func DecodePersons(logits, boxes []float32, queries, classes, personClass int, minScore float64) []Box {
	if queries <= 0 || classes <= 0 || personClass < 0 || personClass >= classes ||
		queries > len(logits)/classes || queries > len(boxes)/4 {
		return nil
	}
	found := make([]Box, 0, 8)
	for query := 0; query < queries; query++ {
		raw := float64(logits[query*classes+personClass])
		if !finite(raw) {
			continue
		}
		score := sigmoid(raw)
		if !(score >= minScore) {
			continue
		}
		cx, cy := float64(boxes[query*4]), float64(boxes[query*4+1])
		w, h := float64(boxes[query*4+2]), float64(boxes[query*4+3])
		if !finite(cx) || !finite(cy) || !finite(w) || !finite(h) {
			continue
		}
		box := Box{
			X1:    clamp01(cx - w/2),
			Y1:    clamp01(cy - h/2),
			X2:    clamp01(cx + w/2),
			Y2:    clamp01(cy + h/2),
			Score: score,
		}
		if box.X2 <= box.X1 || box.Y2 <= box.Y1 {
			continue
		}
		found = append(found, box)
	}
	sort.SliceStable(found, func(i, j int) bool { return found[i].Score > found[j].Score })
	return found
}

// IoU is the intersection over union of two boxes (0 for empty unions and for
// boxes with non-finite coordinates).
func IoU(a, b Box) float64 {
	width := math.Max(0, math.Min(a.X2, b.X2)-math.Max(a.X1, b.X1))
	height := math.Max(0, math.Min(a.Y2, b.Y2)-math.Max(a.Y1, b.Y1))
	intersection := width * height
	union := boxArea(a) + boxArea(b) - intersection
	if !(union > 0) {
		return 0
	}
	if value := intersection / union; finite(value) {
		return value
	}
	return 0
}

// SameDetections compares two detection sets: same count, then greedy
// best-IoU matching; every pair needs IoU ≥ minIoU and |score difference| ≤
// maxScoreDiff. The reason names the count mismatch or the worst pair values;
// two empty sets are the same (the caller decides whether that is enough).
func SameDetections(a, b []Box, minIoU, maxScoreDiff float64) (bool, string) {
	if len(a) != len(b) {
		return false, fmt.Sprintf("count %d vs %d", len(a), len(b))
	}
	if len(a) == 0 {
		return true, "no detections in either set"
	}
	used := make([]bool, len(b))
	worstIoU, worstDiff := 1.0, 0.0
	for _, box := range a {
		best, bestIoU := -1, -1.0
		for j, other := range b {
			if used[j] {
				continue
			}
			if value := IoU(box, other); value > bestIoU {
				best, bestIoU = j, value
			}
		}
		if best < 0 {
			return false, "unmatched detection"
		}
		used[best] = true
		worstIoU = math.Min(worstIoU, bestIoU)
		worstDiff = math.Max(worstDiff, math.Abs(box.Score-b[best].Score))
	}
	reason := fmt.Sprintf("min IoU %.4f, max score diff %.4f", worstIoU, worstDiff)
	return worstIoU >= minIoU && worstDiff <= maxScoreDiff, reason
}

func sigmoid(x float64) float64 {
	return 1 / (1 + math.Exp(-x))
}

func clamp01(value float64) float64 {
	return math.Min(1, math.Max(0, value))
}

func finite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

func boxArea(box Box) float64 {
	return math.Max(0, box.X2-box.X1) * math.Max(0, box.Y2-box.Y1)
}
