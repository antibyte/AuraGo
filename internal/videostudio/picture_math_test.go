package videostudio

import (
	"fmt"
	"math"
	"testing"
)

// The colour constants are mirrored in JS VideoStudioPicture. TestVideoStudioPictureConstantsMatchGo (ui) checks the
// exported ones against Go, and the unexported ones against the same literals as here.
func TestColorConstants(t *testing.T) {
	if ColorWheelK != 0.2 || ColorNeutralKelvin != 6500 || colorTemperatureGain != 0.5 || colorTintGain != 0.3 {
		t.Fatalf("colour constants = %v %v %v %v", ColorWheelK, ColorNeutralKelvin, colorTemperatureGain, colorTintGain)
	}
	if lumaBT709 != [3]float64{0.2126, 0.7152, 0.0722} {
		t.Fatalf("lumaBT709 = %v", lumaBT709)
	}
}

// The colour formula (plan index §4.20) at its identity points and in its documented directions.
func TestColorFormula(t *testing.T) {
	near := func(label string, got, want float64) {
		t.Helper()
		if math.Abs(got-want) > 1e-12 {
			t.Errorf("%s = %v, want %v", label, got, want)
		}
	}
	identity := defaultColorParams()
	for c, gain := range whiteBalance(ColorNeutralKelvin, 0) {
		near(fmt.Sprintf("white balance %d at 6500 K", c), gain, 1)
	}
	for c, curve := range colorCurves(identity) {
		for v := 0.0; v <= 1; v += 0.0625 {
			near(fmt.Sprintf("identity curve %d at %v", c, v), curve.apply(v), v)
		}
	}
	for i, value := range saturationMatrix(1) {
		want := 0.0
		if i%4 == 0 {
			want = 1
		}
		near(fmt.Sprintf("saturation 1 matrix[%d]", i), value, want)
	}
	rgb := [3]float64{0.8, 0.3, 0.1}
	for c, value := range applyColor(identity, rgb) {
		near(fmt.Sprintf("identity grade channel %d", c), value, rgb[c])
	}
	// An entry with empty params reads the registry defaults, which form the identity grade.
	empty := Clip{Effects: []EffectInstance{{ID: "color", On: true, Params: map[string]any{}}}}
	if got := colorParamsAt(empty, 0, 0); got != identity {
		t.Errorf("colorParamsAt(empty params) = %+v, want the defaults %+v", got, identity)
	}
	keyed := Clip{Effects: []EffectInstance{{ID: "color", On: true, Params: map[string]any{"saturation": 0.5}, Keyframes: map[string][]Keyframe{"exposure": {{F: 0, V: 0, E: EaseLinear}, {F: 10, V: 2, E: EaseLinear}}}}}}
	if got := colorParamsAt(keyed, 0, 5); got.Exposure != 1 || got.Saturation != 0.5 || got.Temperature != ColorNeutralKelvin {
		t.Errorf("colorParamsAt(keyed, frame 5) = %+v, want exposure 1, saturation 0.5, 6500 K", got)
	}
	// Higher K warms and keeps the luma of grey; +tint is magenta.
	warm := whiteBalance(9000, 0)
	if !(warm[0] > 1 && warm[2] < 1) {
		t.Errorf("9000 K gains = %v, want red above 1 and blue below 1", warm)
	}
	near("9000 K luma", lumaBT709[0]*warm[0]+lumaBT709[1]*warm[1]+lumaBT709[2]*warm[2], 1)
	if magenta := whiteBalance(ColorNeutralKelvin, 0.5); !(magenta[1] < magenta[0] && magenta[1] < magenta[2]) {
		t.Errorf("tint +0.5 gains = %v, want green lowest", magenta)
	}
	// Exposure +1 doubles a dark grey; saturation 0 leaves the luma in every channel.
	bright := identity
	bright.Exposure = 1
	near("exposure +1", applyColor(bright, [3]float64{0.2, 0.2, 0.2})[0], 0.4)
	grey := identity
	grey.Saturation = 0
	for c, value := range applyColor(grey, rgb) {
		near(fmt.Sprintf("saturation 0 channel %d", c), value, 0.2126*0.8+0.7152*0.3+0.0722*0.1)
	}
	// Wheels: lift_c = lift + k·cos φ_c and gamma_c = gamma·e^{k·w_c}, with φ = 0°, 120°, 240°.
	lifted := identity
	lifted.LiftX = 1
	curves := colorCurves(lifted)
	near("lift wheel x=1, red", curves[0].Lift, ColorWheelK)
	near("lift wheel x=1, green", curves[1].Lift, -ColorWheelK/2)
	gammaWheel := identity
	gammaWheel.GammaY = 1
	near("gamma wheel y=1, green", 1/colorCurves(gammaWheel)[1].InvGamma, math.Exp(ColorWheelK*math.Sqrt(3)/2))
}

// Crop rectangle, mirroring, vignette angle and blur sigma (plan index §4.20).
func TestPictureGeometry(t *testing.T) {
	for _, c := range []struct {
		crop Crop
		w, h int
		want [4]int
	}{
		{Crop{L: 0.1, T: 0.05, R: 0.1, B: 0.05}, 640, 360, [4]int{64, 18, 512, 324}},
		{Crop{}, 1920, 1080, [4]int{0, 0, 1920, 1080}},
		{Crop{L: 0.01, T: 0.01}, 100, 100, [4]int{2, 2, 98, 98}},     // l·W/2 = 0.5 rounds away from zero
		{Crop{L: 0.3, R: 0.7}, 1920, 1080, [4]int{576, 0, 96, 1080}}, // r is limited to 0.95 − l
		{Crop{L: 0.95}, 64, 36, [4]int{60, 0, 4, 36}},
		// Tiny pictures: the position stops at W − 2 and the size at W − x, so a 1×1 picture keeps its pixel.
		{Crop{L: 0.3, T: 0.3, R: 0.3, B: 0.3}, 1, 1, [4]int{0, 0, 1, 1}},
		{Crop{L: 0.5, T: 0.1, R: 0.3, B: 0.6}, 3, 3, [4]int{1, 0, 2, 2}},
	} {
		x, y, w, h := cropRect(c.crop, c.w, c.h)
		if got := [4]int{x, y, w, h}; got != c.want {
			t.Errorf("cropRect(%+v, %d, %d) = %v, want %v", c.crop, c.w, c.h, got, c.want)
		}
	}
	crop := Crop{L: 0.1, T: 0.2, R: 0.3, B: 0.05}
	if got := mirroredCrop(crop, true, false); got != (Crop{L: 0.3, T: 0.2, R: 0.1, B: 0.05}) {
		t.Errorf("mirroredCrop flip_h = %+v", got)
	}
	if got := mirroredCrop(crop, false, true); got != (Crop{L: 0.1, T: 0.05, R: 0.3, B: 0.2}) {
		t.Errorf("mirroredCrop flip_v = %+v", got)
	}
	if got, want := vignetteAngle(0.5, 1, 1280, 720), math.Pi/4*math.Sqrt(640*640+360*360)/720; math.Abs(got-want) > 1e-12 {
		t.Errorf("vignetteAngle(0.5, 1, 1280x720) = %v, want %v", got, want)
	}
	if got := vignetteAngle(1, 0.25, 1280, 720); got != math.Pi/2 {
		t.Errorf("vignetteAngle is capped at π/2: %v", got)
	}
	if got := vignetteAngle(0, 1, 1280, 720); got != 0 {
		t.Errorf("amount 0 gives angle %v", got)
	}
	if got := blurSigma(0.01, 720); math.Abs(got-7.2) > 1e-12 {
		t.Errorf("blurSigma(0.01, 720) = %v, want 7.2", got)
	}
	if !cropAnimated(Clip{Keyframes: map[string][]Keyframe{"crop.b": {{F: 0, V: 0.1, E: EaseLinear}}}}) || cropAnimated(Clip{Keyframes: map[string][]Keyframe{"x": {{F: 0, V: 0.1, E: EaseLinear}}}}) {
		t.Error("cropAnimated must follow the crop.* keyframes only")
	}
	effects := Clip{Effects: []EffectInstance{{ID: "sharpen", On: true, Params: map[string]any{"strength": 0.8}}}}
	if got := effectValueAt(effects, 0, "strength", 0); got != 0.8 {
		t.Errorf("effectValueAt = %v, want 0.8", got)
	}
}
