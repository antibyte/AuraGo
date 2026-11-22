package storymaker

import (
	"context"
	"fmt"
	"image"
	"math"
	"strconv"
)

const (
	// contrastMin is the ratio below which a text is reported: WCAG's 3:1 for large text, which every Storymaker text is.
	contrastMin = 3.0
	// contrastMasked is the ratio below which the box is taken for empty (the text is masked or not drawn) and not reported.
	contrastMasked = 1.15
	// contrastMaxSamples bounds the sample grid inside one box.
	contrastMaxSamples = 20000
	// maxMeasuredBoxes bounds the boxes measured on one still, so a crafted project cannot burn CPU with 65 boxes on each of 200
	// scenes.
	maxMeasuredBoxes = 16
	// lumBins is the resolution of the luminance histogram the percentiles are read from.
	lumBins = 4096
)

// linearLUT is the sRGB-to-linear step of every 8 bit channel value.
var linearLUT = func() (t [256]float64) {
	for i := range t {
		c := float64(i) / 255
		if c <= 0.04045 {
			t[i] = c / 12.92
		} else {
			t[i] = math.Pow((c+0.055)/1.055, 2.4)
		}
	}
	return t
}()

// lumBin is the histogram bin of the relative luminance of an 8 bit sRGB colour.
func lumBin(r, g, b uint8) int {
	l := 0.2126*linearLUT[r] + 0.7152*linearLUT[g] + 0.0722*linearLUT[b]
	return int(math.Round(l * (lumBins - 1)))
}

// percentileBin returns the bin of the element at rank round(p*(total-1)) of the sorted samples.
func percentileBin(hist *[lumBins]int, total int, p float64) int {
	rank := int(math.Round(p * float64(total-1)))
	seen := 0
	for i, n := range hist {
		seen += n
		if rank < seen {
			return i
		}
	}
	return lumBins - 1
}

// boxContrast measures the contrast inside a box given as fractions of the picture: the 3rd and 97th percentile of the relative
// luminance over an evenly spaced grid of at most contrastMaxSamples samples give (L97 + 0.05) / (L3 + 0.05). The percentiles come
// from a histogram, not a sort. The box is clamped to the picture; ok is false when less than 2 x 2 pixels of it lie inside.
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
	var hist [lumBins]int
	total := 0
	nrgba, isNRGBA := img.(*image.NRGBA)
	rgba, isRGBA := img.(*image.RGBA)
	for y := y0; y < y1; y += step {
		for x := x0; x < x1; x += step {
			var bin int
			switch {
			case isRGBA:
				// An RGBA pixel is premultiplied; only an opaque one is the colour itself.
				if p := rgba.Pix[rgba.PixOffset(x, y):]; p[3] == 255 {
					bin = lumBin(p[0], p[1], p[2])
					break
				}
				r, g, bl, _ := img.At(x, y).RGBA()
				bin = lumBin(uint8(r>>8), uint8(g>>8), uint8(bl>>8))
			case isNRGBA:
				if p := nrgba.Pix[nrgba.PixOffset(x, y):]; p[3] == 255 {
					bin = lumBin(p[0], p[1], p[2])
					break
				}
				r, g, bl, _ := img.At(x, y).RGBA()
				bin = lumBin(uint8(r>>8), uint8(g>>8), uint8(bl>>8))
			default:
				r, g, bl, _ := img.At(x, y).RGBA()
				bin = lumBin(uint8(r>>8), uint8(g>>8), uint8(bl>>8))
			}
			hist[bin]++
			total++
		}
	}
	low := float64(percentileBin(&hist, total, 0.03)) / (lumBins - 1)
	high := float64(percentileBin(&hist, total, 0.97)) / (lumBins - 1)
	return (high + 0.05) / (low + 0.05), true
}

// TextContrast is the measured contrast of one text box on a still; ID is the text id or "caption".
type TextContrast struct {
	ID    string  `json:"id"`
	Ratio float64 `json:"ratio"`
}

// MeasureStillContrast decodes one still (through stillConfig and decodeStill, so the size caps hold) and measures the text
// boxes and the caption box of its layout scene. Boxes are fractions of the frame, as in LayoutScene. A box too small to measure
// is left out. At most maxMeasuredBoxes boxes are measured: the caption always counts, so when there are more boxes the texts
// are cut (the first ones, in order, are kept) and the caption keeps its place at the end. ctx is checked between boxes; a
// cancelled context returns ctx.Err() and no ratios.
func MeasureStillContrast(ctx context.Context, data []byte, scene LayoutScene) ([]TextContrast, error) {
	img, err := decodeStill(data)
	if err != nil {
		return nil, err
	}
	texts := scene.Texts
	budget := maxMeasuredBoxes
	if scene.Caption != nil {
		budget--
	}
	if len(texts) > budget {
		texts = texts[:budget]
	}
	var out []TextContrast
	for _, text := range texts {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if ratio, ok := boxContrast(img, text.Box); ok {
			out = append(out, TextContrast{ID: text.ID, Ratio: ratio})
		}
	}
	if scene.Caption != nil {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
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
