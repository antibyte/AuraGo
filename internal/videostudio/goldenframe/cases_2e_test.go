package goldenframe

import (
	"fmt"
	"math"
	"reflect"
	"slices"
	"strings"
	"testing"

	"aurago/internal/videostudio"
)

// anState is a clip's in/out animation state at a timeline frame, per the law of index §4.24. videostudio's animAt
// is unexported, so the test states the law itself, as cases_2d_test.go does for the key. Fields:
//   - side and p: the rest progress (1 at rest, 0 fully hidden);
//   - opacity;
//   - dx, dy: the offset in canvas fractions;
//   - scale;
//   - the blur_in masks;
//   - the wipe side (1 in, -1 out, 0 none).
type anState struct {
	side                   string // "in", "out", or "" at rest
	p                      float64
	opacity, dx, dy, scale float64
	blur                   bool
	m1, m2                 int
	wipe                   int
}

// anLaw evaluates index §4.24 for clip at timeline frame frame.
//   - Effective spans: min(stored, floor(duration/2)), in timeline frames from the clip edges.
//   - Then the preset formulas, with e_out(t) = 1 − (1 − t)³ and sm(t) = t²(3 − 2t).
func anLaw(clip videostudio.Clip, frame int) anState {
	spanIn, spanOut := 0, 0
	if clip.AnimIn != nil {
		spanIn = min(clip.AnimIn.Duration, clip.Duration/2)
	}
	if clip.AnimOut != nil {
		spanOut = min(clip.AnimOut.Duration, clip.Duration/2)
	}
	s := anState{p: 1, opacity: 1, scale: 1}
	j := frame - clip.Start
	var anim *videostudio.ClipAnim
	switch {
	case spanIn > 0 && j < spanIn:
		s.side, s.p, anim = "in", float64(j)/float64(spanIn), clip.AnimIn
	case spanOut > 0 && j >= clip.Duration-spanOut:
		s.side, s.p, anim = "out", float64(clip.Duration-1-j)/float64(spanOut), clip.AnimOut
	default:
		return s
	}
	eOut := func(t float64) float64 { return 1 - math.Pow(1-t, 3) }
	sm := func(t float64) float64 { return t * t * (3 - 2*t) }
	sigma, p := -1.0, s.p
	if s.side == "out" {
		sigma = 1
	}
	switch anim.Preset {
	case "fade":
		s.opacity = p
	case "slide_up", "slide_down", "slide_left", "slide_right":
		d := sigma * (1 - eOut(p)) * 0.15
		switch anim.Preset {
		case "slide_up":
			s.dy = -d
		case "slide_down":
			s.dy = d
		case "slide_left":
			s.dx = -d
		case "slide_right":
			s.dx = d
		}
		s.opacity = p
	case "pop":
		if p < 0.7 {
			s.scale = 0.6 + 0.48*eOut(p/0.7)
		} else {
			s.scale = 1.08 - 0.08*sm((p-0.7)/0.3)
		}
		s.opacity = math.Min(1, p/0.3)
	case "wipe":
		s.wipe = 1
		if s.side == "out" {
			s.wipe = -1
		}
	case "blur_in":
		s.blur, s.opacity = true, p
		s.m1 = int(math.Round(255 * math.Min(p/0.75, 1)))
		s.m2 = int(math.Round(255 * math.Max((p-0.75)/0.25, 0)))
	}
	return s
}

func anNear(a, b anState) bool {
	near := func(x, y float64) bool { return math.Abs(x-y) <= 1e-6 }
	return a.side == b.side && near(a.p, b.p) && near(a.opacity, b.opacity) && near(a.dx, b.dx) && near(a.dy, b.dy) &&
		near(a.scale, b.scale) && a.blur == b.blur && a.m1 == b.m1 && a.m2 == b.m2 && a.wipe == b.wipe
}

// TestTransitionAnimCasesCoverGroups pins the 2e cases (index §4.12).
//   - Transitions: every registry transition beyond the 2a four has one case at its progress point, GL only, with a
//     dissolve baseline and its delta bounds. No case sits on the overlap's first or last frame (D3 quirks). The two
//     overlay cases are there.
//   - Animations: every preset has an in case. Every case has a Delta whose baseline is the case without any
//     animation. Each case's state at its frame follows the law (anLaw), and its stored lengths lie inside
//     AnimFrameRange.
//   - Every case and baseline validates and covers its frame, and Cases lists them all.
func TestTransitionAnimCasesCoverGroups(t *testing.T) {
	m := fakeMedia()
	cases, groups := map[string]Case{}, map[string]int{}
	for _, c := range transitionAnimCases(m) {
		if _, dup := cases[c.Name]; dup {
			t.Errorf("duplicate 2e case %q", c.Name)
		}
		cases[c.Name] = c
		group, _, _ := strings.Cut(c.Name, "_")
		groups[group]++
		if err := videostudio.Validate(c.Project); err != nil {
			t.Errorf("%s: invalid project: %v", c.Name, err)
		}
		if c.Delta == nil {
			t.Errorf("%s: no Delta; every 2e case carries one (index §4.12)", c.Name)
			continue
		}
		base := c.Delta.Baseline
		if err := videostudio.Validate(base); err != nil {
			t.Errorf("%s: invalid baseline: %v", c.Name, err)
		}
		if c.Frame < 0 || c.Frame >= videostudio.EnabledDuration(c.Project) || c.Frame >= videostudio.EnabledDuration(base) {
			t.Errorf("%s: frame %d lies outside the case or its baseline", c.Name, c.Frame)
		}
		if base.Width != c.Project.Width || base.Height != c.Project.Height || base.FPS != c.Project.FPS || len(base.Assets) != len(c.Project.Assets) {
			t.Errorf("%s: the baseline's canvas, rate or assets differ from the case", c.Name)
		}
		if reflect.DeepEqual(base.Tracks, c.Project.Tracks) {
			t.Errorf("%s: the baseline equals the case, so the delta measures nothing", c.Name)
		}
	}
	if len(groups) != 2 || groups["transition"] != 22 || groups["anim"] != 16 {
		t.Errorf("groups %v, want 22 transition and 16 anim cases", groups)
	}

	// Transitions: the progress points and delta bounds of cases_2e.go, stated again.
	percent := map[string]int{"circlecrop": 30, "rectcrop": 30, "zoomin": 30, "pixelize": 25}
	moving := map[string]bool{"pixelize": true, "slideleft": true, "slideright": true, "slideup": true, "slidedown": true, "squeezeh": true, "zoomin": true}
	type txWant struct {
		kind    string
		pct     int
		overlay bool
	}
	wantTx := map[string]txWant{}
	for _, spec := range videostudio.EffectRegistry().Transitions {
		switch spec.ID {
		case videostudio.TransitionDissolve, videostudio.TransitionBlack, videostudio.TransitionWipeLeft, videostudio.TransitionWipeRight:
			continue
		}
		pct := percent[string(spec.ID)]
		if pct == 0 {
			pct = 50
		}
		wantTx[fmt.Sprintf("transition_%s_%d", spec.ID, pct)] = txWant{kind: string(spec.ID), pct: pct}
	}
	if len(wantTx) != 20 {
		t.Errorf("the registry has %d transitions beyond the 2a four, want 20 (Task 1)", len(wantTx))
	}
	wantTx["transition_circlecrop_overlay"] = txWant{kind: "circlecrop", pct: 30, overlay: true}
	wantTx["transition_slideleft_overlay"] = txWant{kind: "slideleft", pct: 50, overlay: true}
	for name, want := range wantTx {
		c, ok := cases[name]
		if !ok {
			t.Errorf("transition case %q is missing", name)
			continue
		}
		if c.Threshold != ThresholdTransition || !slices.Equal(c.Painters, []string{PainterGL}) {
			t.Errorf("%s: threshold %.2f, painters %v; want %.2f and [gl] (the 2D painter draws 2e transitions as a dissolve)", name, c.Threshold, c.Painters, ThresholdTransition)
		}
		last := len(c.Project.Tracks) - 1
		top := c.Project.Tracks[last]
		if (top.Kind == videostudio.TrackOverlay) != want.overlay {
			t.Errorf("%s: the pair sits on a %s track", name, top.Kind)
		}
		if len(top.Clips) != 2 || top.Clips[0].Transition == nil || string(top.Clips[0].Transition.Type) != want.kind || top.Clips[0].Transition.Duration != txDuration {
			t.Errorf("%s: top track clips %+v, want a %s transition over %d frames into a second clip", name, top.Clips, want.kind, txDuration)
			continue
		}
		i, d := c.Frame-top.Clips[1].Start, top.Clips[0].Transition.Duration
		if i*100 != want.pct*d {
			t.Errorf("%s: overlap frame %d of %d, want %d %%", name, i, d, want.pct)
		}
		// D3: no golden sits on overlap frame 0 or on the last frame. That keeps out the 1 px quirks, the rectcrop
		// border, the slideleft/slideup out-of-range column and row, and the radial wedge (the probe fixture covers them).
		if i < 1 || i > d-2 {
			t.Errorf("%s: overlap frame %d of %d is a quirk frame", name, i, d)
		}
		wantDelta := DeltaBlend
		if moving[want.kind] {
			wantDelta = DeltaMask
		}
		bounds := *c.Delta
		bounds.Baseline = videostudio.Project{}
		if !reflect.DeepEqual(bounds, wantDelta) {
			t.Errorf("%s: delta bounds %+v, want %+v", name, bounds, wantDelta)
		}
		// The baseline is the same pair dissolving at the same frame.
		dissolved := slices.Clone(c.Project.Tracks)
		dissolved[last].Clips = slices.Clone(dissolved[last].Clips)
		dissolved[last].Clips[0].Transition = &videostudio.Transition{Type: videostudio.TransitionDissolve, Duration: txDuration}
		if !reflect.DeepEqual(dissolved, c.Delta.Baseline.Tracks) {
			t.Errorf("%s: the baseline is not the same pair dissolving", name)
		}
	}

	// Animations: the state at each case frame, from the law (cases_2e.go comments give the same numbers).
	const slide = 0.15 * 0.343 // 0.15·(1 − e_out(0.3)): a slide's offset at p = 0.3
	const pop = 1.0688047      // 0.6 + 0.48·e_out(0.5/0.7): pop at p = 0.5
	gl, both := []string{PainterGL}, []string{PainterGL, Painter2D}
	type anWant struct {
		clip, asset string
		threshold   float64
		painters    []string
		delta       Delta
		state       anState
	}
	wantAnim := map[string]anWant{
		"anim_fade_in":         {"l", MediaLogo, ThresholdGeometry, both, DeltaBlend, anState{side: "in", p: 0.5, opacity: 0.5, scale: 1}},
		"anim_slide_up_in":     {"l", MediaLogo, ThresholdGeometry, both, DeltaMask, anState{side: "in", p: 0.3, opacity: 0.3, dy: slide, scale: 1}},
		"anim_slide_down_in":   {"l", MediaLogo, ThresholdGeometry, both, DeltaMask, anState{side: "in", p: 0.3, opacity: 0.3, dy: -slide, scale: 1}},
		"anim_slide_left_in":   {"l", MediaLogo, ThresholdGeometry, both, DeltaMask, anState{side: "in", p: 0.3, opacity: 0.3, dx: slide, scale: 1}},
		"anim_slide_right_in":  {"l", MediaLogo, ThresholdGeometry, both, DeltaMask, anState{side: "in", p: 0.3, opacity: 0.3, dx: -slide, scale: 1}},
		"anim_pop_in":          {"l", MediaLogo, ThresholdGeometry, both, DeltaMask, anState{side: "in", p: 0.5, opacity: 1, scale: pop}},
		"anim_wipe_in":         {"l", MediaLogo, ThresholdGeometry, both, DeltaMask, anState{side: "in", p: 0.5, opacity: 1, scale: 1, wipe: 1}},
		"anim_blur_in_in":      {"l", MediaDetail, ThresholdDetail, gl, DeltaMask, anState{side: "in", p: 0.5, opacity: 0.5, scale: 1, blur: true, m1: 170}},
		"anim_slide_left_out":  {"l", MediaLogo, ThresholdGeometry, both, DeltaMask, anState{side: "out", p: 0.3, opacity: 0.3, dx: -slide, scale: 1}},
		"anim_pop_out":         {"l", MediaLogo, ThresholdGeometry, both, DeltaMask, anState{side: "out", p: 0.5, opacity: 1, scale: pop}},
		"anim_pop_mask":        {"l", MediaChart, ThresholdDetail, gl, DeltaMask, anState{side: "in", p: 0.5, opacity: 1, scale: pop}},
		"anim_slide_keyframed": {"l", MediaLogo, ThresholdGeometry, both, DeltaMask, anState{side: "in", p: 0.3, opacity: 0.3, dx: -slide, scale: 1}},
		"anim_wipe_rotated":    {"l", MediaLogo, ThresholdGeometry, both, DeltaMask, anState{side: "out", p: 0.5, opacity: 1, scale: 1, wipe: -1}},
		"anim_fade_dissolve":   {"b", MediaDetail, ThresholdGeometry, both, DeltaBlend, anState{side: "in", p: 0.5, opacity: 0.5, scale: 1}},
		"anim_pop_ramp":        {"k", MediaTicker, ThresholdGeometry, both, DeltaMask, anState{side: "in", p: 25.0 / 35, opacity: 1, scale: 1.0794731}},
		"anim_blur_in_video":   {"k", MediaTicker, ThresholdDetail, gl, DeltaMask, anState{side: "in", p: 0.5, opacity: 0.5, scale: 1, blur: true, m1: 170}},
	}
	lo, hi, _ := videostudio.AnimFrameRange(videostudio.Rate{Num: ProjectFPS, Den: 1})
	for name, want := range wantAnim {
		c, ok := cases[name]
		if !ok {
			t.Errorf("animation case %q is missing", name)
			continue
		}
		if c.Threshold != want.threshold || !slices.Equal(c.Painters, want.painters) {
			t.Errorf("%s: threshold %.2f, painters %v; want %.2f and %v", name, c.Threshold, c.Painters, want.threshold, want.painters)
		}
		if bounds := *c.Delta; !reflect.DeepEqual(Delta{MinLevels: bounds.MinLevels, MinChanged: bounds.MinChanged, MinSlope: bounds.MinSlope, MaxSlope: bounds.MaxSlope, MinR: bounds.MinR, ReportOnly: bounds.ReportOnly}, want.delta) {
			t.Errorf("%s: delta bounds %+v, want %+v", name, bounds, want.delta)
		}
		var animated []videostudio.Clip
		for _, track := range c.Project.Tracks {
			for _, clip := range track.Clips {
				if clip.AnimIn != nil || clip.AnimOut != nil {
					animated = append(animated, clip)
				}
			}
		}
		if len(animated) != 1 || animated[0].ID != want.clip || animated[0].AssetID != want.asset {
			t.Errorf("%s: animated clips %+v, want only %s of %s", name, animated, want.clip, want.asset)
			continue
		}
		clip := animated[0]
		clamped := false
		for _, anim := range []*videostudio.ClipAnim{clip.AnimIn, clip.AnimOut} {
			if anim == nil {
				continue
			}
			if anim.Duration < lo || anim.Duration > hi {
				t.Errorf("%s: length %d lies outside AnimFrameRange %d..%d", name, anim.Duration, lo, hi)
			}
			if _, known := videostudio.AnimPresetByID(anim.Preset); !known {
				t.Errorf("%s: %q is not a registry preset", name, anim.Preset)
			}
			clamped = clamped || anim.Duration > clip.Duration/2
		}
		// Only anim_pop_ramp exercises the render's clamp to half the clip.
		if clamped != (name == "anim_pop_ramp") {
			t.Errorf("%s: clamped %v, want %v", name, clamped, name == "anim_pop_ramp")
		}
		if got := anLaw(clip, c.Frame); !anNear(got, want.state) {
			t.Errorf("%s: animation state at frame %d is %+v, want %+v", name, c.Frame, got, want.state)
		}
		// The baseline is the same project without any animation.
		still := make([]videostudio.Track, len(c.Project.Tracks))
		for i, track := range c.Project.Tracks {
			track.Clips = slices.Clone(track.Clips)
			for j := range track.Clips {
				track.Clips[j].AnimIn, track.Clips[j].AnimOut = nil, nil
			}
			still[i] = track
		}
		if !reflect.DeepEqual(still, c.Delta.Baseline.Tracks) {
			t.Errorf("%s: the baseline is not the case without animation", name)
		}
		// What the combined cases combine.
		switch name {
		case "anim_pop_mask":
			if clip.Mask == nil || clip.Mask.Shape != videostudio.MaskEllipse || c.Project.Tracks[0].Clips[0].AssetID != MediaBlue {
				t.Errorf("%s: want an ellipse-masked chart over the blue still", name)
			}
		case "anim_slide_keyframed":
			if x, ok := videostudio.ValueAt(clip, "x", c.Frame); !ok || math.Abs(x-0.095) > 1e-9 || clip.X != 0.2 {
				t.Errorf("%s: x at frame %d is %v (static %v), want 0.095 keyed over a static 0.2", name, c.Frame, x, clip.X)
			}
		case "anim_wipe_rotated":
			if clip.Rotation != 20 {
				t.Errorf("%s: rotation %v, want 20", name, clip.Rotation)
			}
		case "anim_fade_dissolve":
			if p, found := foundationProgressAt(c); !found || p != 0.5 {
				t.Errorf("%s: dissolve progress %v (found %v), want 0.5", name, p, found)
			}
		case "anim_pop_ramp":
			if s := videostudio.ClipSourceAt(clip, float64(c.Frame)); len(clip.Ramp) != 2 || math.Abs(s-32.8125) > 1e-9 {
				t.Errorf("%s: ramp %+v, S(%d) = %v; want ramp_up_mid's ramp and 32.8125", name, clip.Ramp, c.Frame, s)
			}
		case "anim_blur_in_video":
			if clip.Fit != videostudio.FitCover || c.Project.Tracks[len(c.Project.Tracks)-1].Kind != videostudio.TrackVideo {
				t.Errorf("%s: want the ticker cover-fitted on a video track", name)
			}
		}
	}
	for _, preset := range videostudio.EffectRegistry().AnimPresets {
		c, ok := cases["anim_"+preset.ID+"_in"]
		if !ok {
			t.Errorf("preset %s has no in case", preset.ID)
			continue
		}
		found := false
		for _, track := range c.Project.Tracks {
			for _, clip := range track.Clips {
				found = found || (clip.AnimIn != nil && clip.AnimIn.Preset == preset.ID)
			}
		}
		if !found {
			t.Errorf("anim_%s_in does not animate in with %s", preset.ID, preset.ID)
		}
	}
	if n := len(videostudio.EffectRegistry().AnimPresets); n != 8 {
		t.Errorf("the registry has %d animation presets, want 8 (Task 1)", n)
	}
	for name := range cases {
		if _, transition := wantTx[name]; !transition {
			if _, anim := wantAnim[name]; !anim {
				t.Errorf("unexpected 2e case %q", name)
			}
		}
	}
	listed := map[string]bool{}
	for _, c := range Cases(m) {
		listed[c.Name] = true
	}
	for name := range cases {
		if !listed[name] {
			t.Errorf("Cases does not list %q: transitionAnimCases is not registered", name)
		}
	}
}
