package videostudio

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
)

// The shared float32 transition parameters (plan 2e D1, index §4.13 topic xfade).
//   - The Go side computes every want with transitionFrameAt.
//   - TestVideoStudioEffectsParity (ui) runs the same file through VideoStudioPicture.transitionParams, and both sides
//     compare exactly.
//   - Float32 values are written as the float64 that holds them, so JSON carries them bit for bit.
//   - AURAGO_UPDATE_PARITY=1 regenerates the file from Go first.

type xfadeParityInput struct {
	Type     string `json:"type"`
	Index    int    `json:"index"`
	Duration int    `json:"duration"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
}

// xfadeParitySamples cover the 24 registry types with 30 rows each:
//   - every overlap frame of D = 7;
//   - i ∈ {0, 1, ⌊D/4⌋, ⌊D/2⌋, ⌊3D/4⌋, D−1} for D = 3, 20, 25 and 90 (each i once);
//   - the snap-audit frames at 320×180 and 180×320.
//
// The sizes take turns among 64×36, 160×90, 201×139 and 1920×1080.
func xfadeParitySamples() []paritySample {
	sizes := [][2]int{{64, 36}, {160, 90}, {201, 139}, {1920, 1080}}
	var samples []paritySample
	turn := 0
	add := func(kind TransitionType, i, D int, size [2]int) {
		samples = append(samples, paritySample{fmt.Sprintf("%s i=%d D=%d %dx%d", kind, i, D, size[0], size[1]),
			xfadeParityInput{Type: string(kind), Index: i, Duration: D, Width: size[0], Height: size[1]}})
	}
	for _, spec := range registryTransitions {
		for i := 0; i < 7; i++ {
			add(spec.ID, i, 7, sizes[turn%len(sizes)])
			turn++
		}
		for _, D := range []int{3, 20, 25, 90} {
			seen := map[int]bool{}
			for _, i := range []int{0, 1, D / 4, D / 2, 3 * D / 4, D - 1} {
				if seen[i] {
					continue
				}
				seen[i] = true
				add(spec.ID, i, D, sizes[turn%len(sizes)])
				turn++
			}
		}
		add(spec.ID, 1, 5, [2]int{320, 180})
		add(spec.ID, 3, 5, [2]int{180, 320})
	}
	return samples
}

func xfadeParityResult(raw json.RawMessage) (any, error) {
	var input xfadeParityInput
	if err := json.Unmarshal(raw, &input); err != nil {
		return nil, err
	}
	kind := TransitionType(input.Type)
	if _, ok := TransitionByID(kind); !ok {
		return nil, fmt.Errorf("unknown transition %q", input.Type)
	}
	return xfadeParityWant(kind, transitionFrameAt(kind, input.Index, input.Duration, input.Width, input.Height)), nil
}

// xfadeParityWant is the JS shape of transitionParams for kind: P and Q, plus the parameters of §4.24 that the type
// uses, under the names z, s, cx, cy, R, c, r, zw, zh, k, u, v, sq, zf and w.
func xfadeParityWant(kind TransitionType, frame transitionFrame) map[string]any {
	spec, _ := TransitionByID(kind)
	want := map[string]any{"P": float64(frame.P), "Q": float64(frame.Q)}
	switch spec.XFade {
	case "fadeblack", "fadewhite", "fadegrays":
		want["u"], want["v"] = float64(frame.U), float64(frame.V)
	case "wipeleft", "wiperight", "wipeup", "wipedown":
		want["z"] = frame.Edge
	case "slideleft", "slideright", "slideup", "slidedown":
		want["s"] = frame.Shift
	case "circleopen", "circleclose":
		want["cx"], want["cy"], want["R"], want["c"] = frame.CX, frame.CY, float64(frame.R), float64(frame.C)
	case "circlecrop":
		want["cx"], want["cy"], want["R"], want["r"] = frame.CX, frame.CY, float64(frame.R), float64(frame.Radius)
	case "rectcrop":
		want["cx"], want["cy"], want["zw"], want["zh"] = frame.CX, frame.CY, frame.ZW, frame.ZH
	case "radial":
		want["cx"], want["cy"], want["R"], want["k"] = frame.CX, frame.CY, float64(frame.R), frame.K
	case "smoothleft", "smoothright", "diagtl", "diagbr":
		want["c"] = float64(frame.C)
	case "pixelize":
		want["sq"] = float64(frame.Sq)
	case "zoomin":
		want["zf"], want["w"] = float64(frame.Zf), float64(frame.W)
	}
	return want
}

// TestParityXFade runs the xfade fixture.
//   - runParityTopic regenerates it under AURAGO_UPDATE_PARITY=1 and compares within 1e-6.
//   - Because the topic is exact, every decoded value must also be equal bit for bit, and every registry type must
//     have rows.
func TestParityXFade(t *testing.T) {
	runParityTopic(t, "xfade", xfadeParitySamples(), xfadeParityResult)
	seen := map[string]bool{}
	for _, c := range readParityCases(t, "xfade") {
		var input xfadeParityInput
		if err := json.Unmarshal(c.Input, &input); err != nil {
			t.Fatalf("xfade/%s input: %v", c.Name, err)
		}
		seen[input.Type] = true
		got, err := xfadeParityResult(c.Input)
		if err != nil {
			t.Fatalf("xfade/%s: %v", c.Name, err)
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
			t.Fatalf("xfade/%s want: %v", c.Name, err)
		}
		if !reflect.DeepEqual(gotValue, wantValue) {
			t.Errorf("xfade/%s: got %s, want %s (exact)", c.Name, data, c.Want)
		}
	}
	for _, spec := range registryTransitions {
		if !seen[string(spec.ID)] {
			t.Errorf("the xfade fixture has no rows for %q", spec.ID)
		}
	}
}
