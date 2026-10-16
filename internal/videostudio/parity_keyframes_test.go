package videostudio

import (
	"encoding/json"
	"math"
	"testing"
)

// TestParityEasing runs the shared easing fixture (plan index §4.13); TestVideoStudioEffectsParity (ui) runs the same
// file through VideoStudioEffects.ease.
func TestParityEasing(t *testing.T) {
	for _, c := range readParityCases(t, "easing") {
		var input struct {
			Easing Easing  `json:"easing"`
			T      float64 `json:"t"`
		}
		var want struct {
			V float64 `json:"v"`
		}
		if err := json.Unmarshal(c.Input, &input); err != nil {
			t.Fatalf("%s input: %v", c.Name, err)
		}
		if err := json.Unmarshal(c.Want, &want); err != nil {
			t.Fatalf("%s want: %v", c.Name, err)
		}
		if got := Ease(input.Easing, input.T); math.Abs(got-want.V) > 1e-6 {
			t.Errorf("%s: Ease(%s, %v) = %v, want %v", c.Name, input.Easing, input.T, got, want.V)
		}
	}
}

// TestParityKeyframes runs the shared keyframe fixture in one of two forms: a keyframe list sampled at a source
// position (sampleKeyframes; JS sample), or a clip's path at a timeline frame (ValueAt; JS valueAt).
func TestParityKeyframes(t *testing.T) {
	for _, c := range readParityCases(t, "keyframes") {
		var input struct {
			Keyframes []Keyframe `json:"keyframes"`
			F         float64    `json:"f"`
			Clip      *Clip      `json:"clip"`
			Path      string     `json:"path"`
			Frame     int        `json:"frame"`
		}
		var want struct {
			V float64 `json:"v"`
		}
		if err := json.Unmarshal(c.Input, &input); err != nil {
			t.Fatalf("%s input: %v", c.Name, err)
		}
		if err := json.Unmarshal(c.Want, &want); err != nil {
			t.Fatalf("%s want: %v", c.Name, err)
		}
		var got float64
		switch {
		case input.Clip != nil:
			value, ok := ValueAt(*input.Clip, input.Path, input.Frame)
			if !ok {
				t.Errorf("%s: ValueAt does not know path %q", c.Name, input.Path)
				continue
			}
			got = value
		case len(input.Keyframes) > 0:
			got = sampleKeyframes(input.Keyframes, input.F)
		default:
			t.Errorf("%s: the input has neither a clip nor keyframes", c.Name)
			continue
		}
		if math.Abs(got-want.V) > 1e-6 {
			t.Errorf("%s: got %v, want %v", c.Name, got, want.V)
		}
	}
}
