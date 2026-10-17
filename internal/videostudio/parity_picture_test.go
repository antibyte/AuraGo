package videostudio

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// Shared colour and picture fixtures (plan index §4.13, §4.20): topics color, wheels and picture.
//   - The Go side computes every want with picture_math.go.
//   - TestVideoStudioEffectsParity (ui) runs the same files through window.VideoStudioPicture.
//   - AURAGO_UPDATE_PARITY=1 regenerates the files from Go first. Afterwards, review the identity cases by hand:
//     6500 K and tint 0 give gains of 1, and the defaults give the identity curve.

// paritySample is one fixture input before its want is computed (writeParityFixture).
type paritySample struct {
	name  string
	input any
}

type colorParityInput struct {
	Params colorParams  `json:"params"`
	RGB    [][3]float64 `json:"rgb"`
}

type colorParityWant struct {
	WB         [3]float64   `json:"wb"`
	Mul        [3]float64   `json:"mul"`
	Lift       [3]float64   `json:"lift"`
	Gain       [3]float64   `json:"gain"`
	InvGamma   [3]float64   `json:"invGamma"`
	Saturation [9]float64   `json:"saturation"`
	RGB        [][3]float64 `json:"rgb"`
}

type wheelParityInput struct {
	Wheel  string  `json:"wheel"` // lift, gamma or gain
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Master float64 `json:"master"`
}

type wheelParityWant struct {
	RGB [3]float64 `json:"rgb"` // lift_c, gamma_c or gain_c
}

type pictureParityInput struct {
	Kind   string  `json:"kind"` // crop, mirror, vignette or blur
	Crop   *Crop   `json:"crop,omitempty"`
	FlipH  bool    `json:"flip_h,omitempty"`
	FlipV  bool    `json:"flip_v,omitempty"`
	Amount float64 `json:"amount,omitempty"`
	Size   float64 `json:"size,omitempty"`
	Radius float64 `json:"radius,omitempty"`
	Width  int     `json:"width,omitempty"`
	Height int     `json:"height,omitempty"`
}

func colorParitySamples() []paritySample {
	rgb := [][3]float64{{0, 0, 0}, {1, 1, 1}, {0.5, 0.5, 0.5}, {0.8, 0.3, 0.1}, {0.1, 0.6, 0.9}, {0.25, 0.5, 0.75}}
	sample := func(name string, edit func(*colorParams)) paritySample {
		params := defaultColorParams()
		edit(&params)
		return paritySample{name, colorParityInput{Params: params, RGB: rgb}}
	}
	return []paritySample{
		sample("identity at the defaults", func(*colorParams) {}),
		sample("exposure +1 contrast 1.2", func(p *colorParams) { p.Exposure, p.Contrast = 1, 1.2 }),
		sample("exposure -2 contrast 0.5", func(p *colorParams) { p.Exposure, p.Contrast = -2, 0.5 }),
		sample("warm 9000 K tint +0.3", func(p *colorParams) { p.Temperature, p.Tint = 9000, 0.3 }),
		sample("cool 3200 K tint -0.5", func(p *colorParams) { p.Temperature, p.Tint = 3200, -0.5 }),
		sample("saturation 0", func(p *colorParams) { p.Saturation = 0 }),
		sample("saturation 2", func(p *colorParams) { p.Saturation = 2 }),
		sample("lift wheel toward red", func(p *colorParams) { p.LiftX, p.Lift = 0.8, 0.05 }),
		sample("gamma wheel toward cyan", func(p *colorParams) { p.GammaX, p.GammaY, p.Gamma = -0.6, 0.2, 1.4 }),
		sample("gain wheel toward blue", func(p *colorParams) { p.GainX, p.GainY, p.Gain = -0.5, -0.8, 0.9 }),
		sample("clipping extremes", func(p *colorParams) { p.Exposure, p.Contrast, p.Lift, p.Gain, p.Gamma = 3, 3, -0.5, 4, 0.2 }),
		sample("everything at once", func(p *colorParams) {
			*p = colorParams{Exposure: 0.4, Contrast: 1.15, Saturation: 1.3, Temperature: 7300, Tint: -0.1, LiftX: 0.2, LiftY: -0.3, Lift: 0.02,
				GammaX: 0.1, GammaY: 0.4, Gamma: 0.9, GainX: -0.2, GainY: 0.1, Gain: 1.1}
		}),
	}
}

func wheelParitySamples() []paritySample {
	var samples []paritySample
	for _, wheel := range []struct {
		name   string
		master float64
	}{{"lift", 0.1}, {"gamma", 1.5}, {"gain", 0.8}} {
		for _, at := range [][2]float64{{0, 0}, {1, 0}, {0, 1}, {-0.6, 0.8}, {0.5, -0.5}, {-1, 0}} {
			name := fmt.Sprintf("%s x=%g y=%g master=%g", wheel.name, at[0], at[1], wheel.master)
			samples = append(samples, paritySample{name, wheelParityInput{Wheel: wheel.name, X: at[0], Y: at[1], Master: wheel.master}})
		}
	}
	return samples
}

func pictureParitySamples() []paritySample {
	crop := func(name string, c Crop, w, h int) paritySample {
		return paritySample{"crop " + name, pictureParityInput{Kind: "crop", Crop: &c, Width: w, Height: h}}
	}
	mirror := func(flipH, flipV bool) paritySample {
		c := Crop{L: 0.1, T: 0.2, R: 0.3, B: 0.05}
		return paritySample{fmt.Sprintf("mirror flip_h=%t flip_v=%t", flipH, flipV), pictureParityInput{Kind: "mirror", Crop: &c, FlipH: flipH, FlipV: flipV}}
	}
	vignette := func(amount, size float64, w, h int) paritySample {
		return paritySample{fmt.Sprintf("vignette amount=%g size=%g %dx%d", amount, size, w, h), pictureParityInput{Kind: "vignette", Amount: amount, Size: size, Width: w, Height: h}}
	}
	blur := func(radius float64, h int) paritySample {
		return paritySample{fmt.Sprintf("blur radius=%g height=%d", radius, h), pictureParityInput{Kind: "blur", Radius: radius, Height: h}}
	}
	return []paritySample{
		crop("symmetric 640x360", Crop{L: 0.1, T: 0.05, R: 0.1, B: 0.05}, 640, 360),
		crop("none 1920x1080", Crop{}, 1920, 1080),
		crop("r limited to 0.95 - l", Crop{L: 0.3, R: 0.7}, 1920, 1080),
		crop("half pixel rounds away from zero", Crop{L: 0.01, T: 0.01}, 100, 100),
		crop("odd size", Crop{L: 0.123, T: 0.077, R: 0.211, B: 0.3}, 641, 361),
		crop("near the limit", Crop{L: 0.95}, 64, 36),
		crop("t + b at 0.95", Crop{T: 0.5, B: 0.45}, 1280, 720),
		crop("portrait", Crop{L: 0.25, T: 0.25, R: 0.25, B: 0.25}, 1080, 1920),
		crop("1x1 picture", Crop{L: 0.3, T: 0.3, R: 0.3, B: 0.3}, 1, 1),
		crop("3x3 picture", Crop{L: 0.5, T: 0.1, R: 0.3, B: 0.6}, 3, 3),
		mirror(false, false), mirror(true, false), mirror(false, true), mirror(true, true),
		vignette(0.5, 1, 1280, 720),
		vignette(1, 0.25, 1280, 720),
		vignette(0.3, 2, 720, 1280),
		vignette(0, 1, 640, 360),
		vignette(0.75, 0.6, 1920, 1080),
		blur(0.01, 720), blur(0.05, 1080), blur(0, 360), blur(0.0005, 2160),
	}
}

func colorParityResult(raw json.RawMessage) (any, error) {
	var input colorParityInput
	if err := json.Unmarshal(raw, &input); err != nil {
		return nil, err
	}
	want := colorParityWant{WB: whiteBalance(input.Params.Temperature, input.Params.Tint), Saturation: saturationMatrix(input.Params.Saturation), RGB: [][3]float64{}}
	for c, curve := range colorCurves(input.Params) {
		want.Mul[c], want.Lift[c], want.Gain[c], want.InvGamma[c] = curve.Mul, curve.Lift, curve.Gain, curve.InvGamma
	}
	for _, rgb := range input.RGB {
		want.RGB = append(want.RGB, applyColor(input.Params, rgb))
	}
	return want, nil
}

func wheelParityResult(raw json.RawMessage) (any, error) {
	var input wheelParityInput
	if err := json.Unmarshal(raw, &input); err != nil {
		return nil, err
	}
	p := defaultColorParams()
	switch input.Wheel {
	case "lift":
		p.LiftX, p.LiftY, p.Lift = input.X, input.Y, input.Master
	case "gamma":
		p.GammaX, p.GammaY, p.Gamma = input.X, input.Y, input.Master
	case "gain":
		p.GainX, p.GainY, p.Gain = input.X, input.Y, input.Master
	default:
		return nil, fmt.Errorf("unknown wheel %q", input.Wheel)
	}
	var want wheelParityWant
	for c, curve := range colorCurves(p) {
		switch input.Wheel {
		case "lift":
			want.RGB[c] = curve.Lift
		case "gamma":
			want.RGB[c] = 1 / curve.InvGamma
		default:
			want.RGB[c] = curve.Gain
		}
	}
	return want, nil
}

func pictureParityResult(raw json.RawMessage) (any, error) {
	var input pictureParityInput
	if err := json.Unmarshal(raw, &input); err != nil {
		return nil, err
	}
	crop := Crop{}
	if input.Crop != nil {
		crop = *input.Crop
	}
	switch input.Kind {
	case "crop":
		x, y, w, h := cropRect(crop, input.Width, input.Height)
		return map[string]int{"x": x, "y": y, "w": w, "h": h}, nil
	case "mirror":
		return mirroredCrop(crop, input.FlipH, input.FlipV), nil
	case "vignette":
		return map[string]float64{"angle": vignetteAngle(input.Amount, input.Size, input.Width, input.Height)}, nil
	case "blur":
		return map[string]float64{"sigma": blurSigma(input.Radius, input.Height)}, nil
	}
	return nil, fmt.Errorf("unknown picture input kind %q", input.Kind)
}

// TestParityColor runs the colour fixture. JS: VideoStudioPicture.colorCoefficients, whiteBalance, saturationMatrix
// and applyColor.
func TestParityColor(t *testing.T) {
	runParityTopic(t, "color", colorParitySamples(), colorParityResult)
}

// TestParityWheels runs the colour-wheel fixture. JS: VideoStudioPicture.colorCoefficients with one wheel set.
func TestParityWheels(t *testing.T) {
	runParityTopic(t, "wheels", wheelParitySamples(), wheelParityResult)
}

// TestParityPicture runs the crop, mirror, vignette and blur fixture. JS: VideoStudioPicture.cropRect, mirroredCrop,
// vignetteAngle and blurSigma.
func TestParityPicture(t *testing.T) {
	runParityTopic(t, "picture", pictureParitySamples(), pictureParityResult)
}

// runParityTopic optionally regenerates a topic (AURAGO_UPDATE_PARITY=1). It then checks every case of the file
// against compute, number by number within 1e-6, like the JS runner's match.
func runParityTopic(t *testing.T, topic string, samples []paritySample, compute func(json.RawMessage) (any, error)) {
	t.Helper()
	if os.Getenv("AURAGO_UPDATE_PARITY") == "1" {
		writeParityFixture(t, topic, samples, compute)
	}
	for _, c := range readParityCases(t, topic) {
		got, err := compute(c.Input)
		if err != nil {
			t.Errorf("%s/%s: %v", topic, c.Name, err)
			continue
		}
		data, err := json.Marshal(got)
		if err != nil {
			t.Fatal(err)
		}
		var gotValue, wantValue any
		if err := json.Unmarshal(data, &gotValue); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(c.Want, &wantValue); err != nil {
			t.Fatalf("%s/%s want: %v", topic, c.Name, err)
		}
		if !parityNear(gotValue, wantValue) {
			t.Errorf("%s/%s: got %s, want %s", topic, c.Name, data, c.Want)
		}
	}
}

// parityNear compares two decoded JSON values: numbers within 1e-6, lists element by element, objects on the keys
// of want.
func parityNear(got, want any) bool {
	switch w := want.(type) {
	case float64:
		g, ok := got.(float64)
		return ok && math.Abs(g-w) <= 1e-6
	case []any:
		g, ok := got.([]any)
		if !ok || len(g) != len(w) {
			return false
		}
		for i := range w {
			if !parityNear(g[i], w[i]) {
				return false
			}
		}
		return true
	case map[string]any:
		g, ok := got.(map[string]any)
		if !ok {
			return false
		}
		for key, value := range w {
			if !parityNear(g[key], value) {
				return false
			}
		}
		return true
	}
	return reflect.DeepEqual(got, want)
}

// writeParityFixture writes testdata/parity/<topic>.json from Go: every sample's input with the want that compute
// returns, one case per line (the layout of transitions.json).
func writeParityFixture(t *testing.T, topic string, samples []paritySample, compute func(json.RawMessage) (any, error)) {
	t.Helper()
	var buf bytes.Buffer
	buf.WriteString("{\n  \"cases\": [\n")
	for i, sample := range samples {
		input, err := json.Marshal(sample.input)
		if err != nil {
			t.Fatal(err)
		}
		want, err := compute(input)
		if err != nil {
			t.Fatalf("%s/%s: %v", topic, sample.name, err)
		}
		wantJSON, err := json.Marshal(want)
		if err != nil {
			t.Fatal(err)
		}
		line, err := json.Marshal(parityCase{Name: sample.name, Input: input, Want: wantJSON})
		if err != nil {
			t.Fatal(err)
		}
		buf.WriteString("    ")
		buf.Write(line)
		if i < len(samples)-1 {
			buf.WriteString(",")
		}
		buf.WriteString("\n")
	}
	buf.WriteString("  ]\n}\n")
	if err := os.WriteFile(filepath.Join("testdata", "parity", topic+".json"), buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}
