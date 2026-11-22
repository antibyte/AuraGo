package storymaker

import (
	"fmt"
	"image"
	"math"
	"sort"
	"strconv"
)

const (
	// contrastMin is the ratio below which a text is reported: WCAG's 3:1 for large text, which every Storymaker text is.
	contrastMin = 3.0
	// contrastMasked is the ratio below which the box is taken for empty (the text is masked or not drawn) and not reported.
	contrastMasked = 1.15
	// contrastMaxSamples bounds the sample grid inside one box.
	contrastMaxSamples = 20000
)

// linearChannel turns an sRGB channel (0..1) into linear light.
func linearChannel(c float64) float64 {
	if c <= 0.04045 {
		return c / 12.92
	}
	return math.Pow((c+0.055)/1.055, 2.4)
}

// relativeLuminance is the WCAG relative luminance of a 16 bit sRGB colour.
func relativeLuminance(r, g, b uint32) float64 {
	return 0.2126*linearChannel(float64(r)/65535) + 0.7152*linearChannel(float64(g)/65535) + 0.0722*linearChannel(float64(b)/65535)
}

// boxContrast measures the contrast inside a box given as fractions of the picture: the 3rd and 97th percentile of the relative
// luminance over an evenly spaced grid of at most contrastMaxSamples samples give (L97 + 0.05) / (L3 + 0.05). The box is clamped
// to the picture; ok is false when less than 2 x 2 pixels of it lie inside.
func boxContrast(img image.Image, box LayoutBox) (ratio float64, ok bool) {
	b := img.Bounds()
	w, h := float64(b.Dx()), float64(b.Dy())
	x0 := max(b.Min.X, int(math.Floor(box[0]*w))+b.Min.X)
	y0 := max(b.Min.Y, int(math.Floor(box[1]*h))+b.Min.Y)
	x1 := min(b.Max.X, int(math.Ceil((box[0]+box[2])*w))+b.Min.X)
	y1 := min(b.Max.Y, int(math.Ceil((box[1]+box[3])*h))+b.Min.Y)
	if x1-x0 < 2 || y1-y0 < 2 {
		return 0, false
	}
	step := max(1, int(math.Ceil(math.Sqrt(float64(x1-x0)*float64(y1-y0)/contrastMaxSamples))))
	var lum []float64
	for y := y0; y < y1; y += step {
		for x := x0; x < x1; x += step {
			r, g, bl, _ := img.At(x, y).RGBA()
			lum = append(lum, relativeLuminance(r, g, bl))
		}
	}
	sort.Float64s(lum)
	last := len(lum) - 1
	low := lum[int(math.Round(0.03*float64(last)))]
	high := lum[int(math.Round(0.97*float64(last)))]
	return (high + 0.05) / (low + 0.05), true
}

// TextContrast is the measured contrast of one text box on a still; ID is the text id or "caption".
type TextContrast struct {
	ID    string  `json:"id"`
	Ratio float64 `json:"ratio"`
}

// MeasureStillContrast decodes one still (through stillConfig and decodeStill, so the size caps hold) and measures every text
// box and the caption box of its layout scene. Boxes are fractions of the frame, as in LayoutScene. A box too small to measure
// is left out.
func MeasureStillContrast(data []byte, scene LayoutScene) ([]TextContrast, error) {
	img, err := decodeStill(data)
	if err != nil {
		return nil, err
	}
	var out []TextContrast
	for _, text := range scene.Texts {
		if ratio, ok := boxContrast(img, text.Box); ok {
			out = append(out, TextContrast{ID: text.ID, Ratio: ratio})
		}
	}
	if scene.Caption != nil {
		if ratio, ok := boxContrast(img, *scene.Caption); ok {
			out = append(out, TextContrast{ID: "caption", Ratio: ratio})
		}
	}
	return out, nil
}

// ContrastIssues turns the measured ratios into text_contrast warnings: a ratio from contrastMasked up to contrastMin. A lower
// one means the box is empty (the text is masked or not drawn), which is not a contrast problem of the picture.
func ContrastIssues(stills []StillInfo) []Issue {
	var out []Issue
	for _, still := range stills {
		for _, text := range still.Texts {
			if text.Ratio < contrastMasked || text.Ratio >= contrastMin {
				continue
			}
			name := "the text " + strconv.Quote(text.ID)
			if text.ID == "caption" {
				name = "the caption"
			}
			out = append(out, Issue{
				Severity: SeverityWarning, SceneID: still.SceneID, Path: jp("scenes", still.Index), Code: CodeTextContrast,
				Message: fmt.Sprintf("In scene %s %s has a contrast of %.1f:1 against the picture behind it, under the %.0f:1 a viewer needs.", still.SceneID, name, text.Ratio, contrastMin),
				Fix:     "Put the text on a calmer, darker or lighter area of the picture, or change the environment or the actors so the background behind it is calm.",
			})
		}
	}
	return out
}
