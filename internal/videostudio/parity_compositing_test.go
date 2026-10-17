package videostudio

import (
	"encoding/json"
	"fmt"
	"testing"
)

// Shared compositing fixtures (plan index §4.13, §4.22): topics blend, mask and key, the real-valued forms.
//   - The Go side computes every want with compositing_math.go.
//   - TestVideoStudioEffectsParity (ui) runs the same files through window.VideoStudioPicture.
//   - AURAGO_UPDATE_PARITY=1 regenerates the files from Go first (writeParityFixture). Afterwards, review by hand: the
//     ends of dodge and burn, the static mask centres and the edge points, and the key colour's own alpha 0.
// The exact 8-bit forms are tested in Go only (compositing_math_test.go) and against FFmpeg in the gated render tests.

type blendParityInput struct {
	Mode  string     `json:"mode"`
	Below [3]float64 `json:"below"`
	Layer [3]float64 `json:"layer"`
	Alpha float64    `json:"alpha"`
}

type blendParityWant struct {
	ModeRGB [3]float64 `json:"mode_rgb"` // blendChannel per channel
	RGB     [3]float64 `json:"rgb"`      // compositeChannel per channel
}

type maskParityBox struct {
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

type maskParityInput struct {
	Mask   Mask          `json:"mask"`
	Box    maskParityBox `json:"box"`
	Points [][2]float64  `json:"points"` // box px
}

type maskParityFrame struct {
	CX   float64 `json:"cx"`
	CY   float64 `json:"cy"`
	RX   float64 `json:"rx"`
	RY   float64 `json:"ry"`
	Ramp float64 `json:"ramp"`
}

type maskParityWant struct {
	Frame maskParityFrame `json:"frame"`
	M     []float64       `json:"m"`
}

type keyParityParams struct {
	Color      string  `json:"color"`
	Similarity float64 `json:"similarity"`
	Smoothness float64 `json:"smoothness"`
	Spill      float64 `json:"spill"`
}

type keyParityInput struct {
	Params keyParityParams `json:"params"`
	RGB    [][3]float64    `json:"rgb"`
}

type keyParityWant struct {
	Alpha []float64    `json:"alpha"`
	RGB   [][3]float64 `json:"rgb"`
}

// blendParitySamples covers every registry mode three times: a general triple at alpha 1; the ends (layer 1 over 0,
// layer 0 over 1, both at the 0.5 threshold) at alpha 0.5; and a third triple at alpha 0.25.
func blendParitySamples() []paritySample {
	var samples []paritySample
	for _, mode := range EffectRegistry().BlendModes {
		for _, c := range []struct {
			below, layer [3]float64
			alpha        float64
		}{
			{[3]float64{0.2, 0.55, 0.9}, [3]float64{0.7, 0.3, 0.45}, 1},
			{[3]float64{0, 1, 0.5}, [3]float64{1, 0, 0.5}, 0.5},
			{[3]float64{0.25, 0.75, 0.1}, [3]float64{0.6, 0.9, 0.05}, 0.25},
		} {
			name := fmt.Sprintf("%s below=%v layer=%v alpha=%g", mode.ID, c.below, c.layer, c.alpha)
			samples = append(samples, paritySample{name, blendParityInput{Mode: mode.ID, Below: c.below, Layer: c.layer, Alpha: c.alpha}})
		}
	}
	return samples
}

// maskParitySamples: rectangles and ellipses, static, feathered, rotated, inverted, partly outside the box, a tiny box
// (the 1 px ramp minimum) and a portrait box. Each sample checks the box centre, two corners, two inner points and the
// points 0.75, 0.25 px and 3 px around the edge on both mask axes.
func maskParitySamples() []paritySample {
	sample := func(name string, mask Mask, w, h float64) paritySample {
		frame := maskFrameOf(mask, w, h)
		points := [][2]float64{{w / 2, h / 2}, {0.5, 0.5}, {w - 0.5, h - 0.5}, {w * 0.3, h * 0.7}, {w * 0.8, h * 0.25}}
		for _, delta := range []float64{-0.75, -0.25, 0.25, 0.75, 3} {
			along, across := frame.RX+delta, frame.RY+delta
			points = append(points,
				[2]float64{frame.CX + along*frame.Cos, frame.CY + along*frame.Sin},   // local (rx + δ, 0)
				[2]float64{frame.CX - across*frame.Sin, frame.CY + across*frame.Cos}) // local (0, ry + δ)
		}
		return paritySample{name, maskParityInput{Mask: mask, Box: maskParityBox{Width: w, Height: h}, Points: points}}
	}
	return []paritySample{
		sample("rect static", Mask{Shape: MaskRect, X: 0.25, Y: 0.25, W: 0.5, H: 0.5}, 640, 360),
		sample("rect feathered", Mask{Shape: MaskRect, X: 0.1, Y: 0.2, W: 0.6, H: 0.5, Feather: 0.05}, 640, 360),
		sample("rect rotated 30", Mask{Shape: MaskRect, X: 0.2, Y: 0.1, W: 0.5, H: 0.7, Rotation: 30, Feather: 0.02}, 1280, 720),
		sample("rect inverted", Mask{Shape: MaskRect, X: 0.25, Y: 0.25, W: 0.5, H: 0.5, Feather: 0.1, Invert: true}, 640, 360),
		sample("ellipse static", Mask{Shape: MaskEllipse, X: 0.25, Y: 0.2, W: 0.5, H: 0.6, Feather: 0.04}, 640, 360),
		sample("ellipse rotated -45", Mask{Shape: MaskEllipse, X: 0.3, Y: 0.3, W: 0.4, H: 0.2, Rotation: -45, Feather: 0.1}, 1920, 1080),
		sample("ellipse inverted", Mask{Shape: MaskEllipse, X: 0.25, Y: 0.25, W: 0.5, H: 0.5, Rotation: 15, Feather: 0.2, Invert: true}, 400, 400),
		sample("ellipse wider than the box", Mask{Shape: MaskEllipse, X: -0.5, Y: 0.1, W: 2, H: 0.8, Rotation: 10, Feather: 0.5}, 640, 360),
		sample("tiny box: the ramp stays 1 px", Mask{Shape: MaskRect, X: 0.1, Y: 0.1, W: 0.8, H: 0.8, Feather: 0.01}, 20, 10),
		sample("portrait box", Mask{Shape: MaskEllipse, X: 0.1, Y: 0.4, W: 0.8, H: 0.2, Rotation: 90, Feather: 0.03}, 1080, 1920),
	}
}

// keyParitySamples: green and blue keys, a hard edge, no spill, full spill, a red key (green spill type) and an
// upper-case colour, each over the same colours (the key itself, white, black, skin, spill green, blue, grey and two
// near-key greens).
func keyParitySamples() []paritySample {
	rgb := [][3]float64{{0, 177.0 / 255, 64.0 / 255}, {1, 1, 1}, {0, 0, 0}, {0.85, 0.6, 0.5}, {0.4, 0.7, 0.35}, {0.1, 0.2, 0.8}, {0.5, 0.5, 0.5}, {120.0 / 255, 177.0 / 255, 64.0 / 255}, {0.05, 0.65, 0.3}}
	sample := func(name, color string, similarity, smoothness, spill float64) paritySample {
		return paritySample{name, keyParityInput{Params: keyParityParams{Color: color, Similarity: similarity, Smoothness: smoothness, Spill: spill}, RGB: rgb}}
	}
	return []paritySample{
		sample("green defaults", "#00b140", 0.25, 0.1, 0.5),
		sample("green hard edge", "#00b140", 0.25, 0, 0.5),
		sample("green without spill suppression", "#00b140", 0.25, 0.1, 0),
		sample("pure green, wide and full spill", "#00ff00", 0.4, 0.3, 1),
		sample("blue screen", "#0047bb", 0.2, 0.15, 0.7),
		sample("a red key uses the green spill type", "#ff0000", 0.3, 0.05, 0.5),
		sample("upper-case colour", "#00B140", 0.01, 1, 0.25),
	}
}

func blendParityResult(raw json.RawMessage) (any, error) {
	var input blendParityInput
	if err := json.Unmarshal(raw, &input); err != nil {
		return nil, err
	}
	if _, ok := BlendModeByID(input.Mode); !ok {
		return nil, fmt.Errorf("unknown blend mode %q", input.Mode)
	}
	var want blendParityWant
	for c := range want.RGB {
		want.ModeRGB[c] = blendChannel(input.Mode, input.Below[c], input.Layer[c])
		want.RGB[c] = compositeChannel(input.Mode, input.Below[c], input.Layer[c], input.Alpha)
	}
	return want, nil
}

func maskParityResult(raw json.RawMessage) (any, error) {
	var input maskParityInput
	if err := json.Unmarshal(raw, &input); err != nil {
		return nil, err
	}
	frame := maskFrameOf(input.Mask, input.Box.Width, input.Box.Height)
	want := maskParityWant{Frame: maskParityFrame{CX: frame.CX, CY: frame.CY, RX: frame.RX, RY: frame.RY, Ramp: frame.Ramp}, M: make([]float64, 0, len(input.Points))}
	for _, p := range input.Points {
		want.M = append(want.M, maskAt(frame, p[0], p[1]))
	}
	return want, nil
}

func keyParityResult(raw json.RawMessage) (any, error) {
	var input keyParityInput
	if err := json.Unmarshal(raw, &input); err != nil {
		return nil, err
	}
	if !validHexColor(input.Params.Color) {
		return nil, fmt.Errorf("key colour %q is not #rrggbb", input.Params.Color)
	}
	p := keyParams{Color: keyColor(input.Params.Color), Similarity: input.Params.Similarity, Smoothness: input.Params.Smoothness, Spill: input.Params.Spill}
	want := keyParityWant{Alpha: []float64{}, RGB: [][3]float64{}}
	for _, rgb := range input.RGB {
		want.Alpha = append(want.Alpha, keyAlpha(p, rgb))
		want.RGB = append(want.RGB, despillRGB(p, rgb))
	}
	return want, nil
}

// TestParityBlend runs the blend fixture. JS: VideoStudioPicture.blendChannel and compositeRGB.
func TestParityBlend(t *testing.T) {
	runParityTopic(t, "blend", blendParitySamples(), blendParityResult)
}

// TestParityMask runs the mask fixture. JS: VideoStudioPicture.maskFrame and maskAt.
func TestParityMask(t *testing.T) {
	runParityTopic(t, "mask", maskParitySamples(), maskParityResult)
}

// TestParityKey runs the chroma key fixture. JS: VideoStudioPicture.keyAlpha and despill.
func TestParityKey(t *testing.T) {
	runParityTopic(t, "key", keyParitySamples(), keyParityResult)
}
