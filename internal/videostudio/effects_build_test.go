package videostudio

import (
	"fmt"
	"math"
	"strings"
	"testing"
)

// linearKeys is a linear keyframe pair from source frame 0 to source frame last.
func linearKeys(from, to float64, last int) []Keyframe {
	return []Keyframe{{F: 0, V: from, E: EaseLinear}, {F: last, V: to, E: EaseLinear}}
}

// effectTestClip is a full-canvas contain clip with an effect stack.
func effectTestClip(duration int, effects ...EffectInstance) Clip {
	return Clip{ID: "fx", AssetID: "a", Duration: duration, Width: 1, Height: 1, Opacity: 1, Volume: 1, Fit: FitContain, Effects: effects}
}

// effectTestChain is clip 1 of layer 0, a decoded 640x360 picture, with the whole clip in its window.
func effectTestChain(clip Clip, image bool, luts map[int]*LUT3D) clipChain {
	return clipChain{track: 0, index: 1, item: layerClip{clip: clip, whole: clip, at: clip.Start, input: 1, image: image, luts: luts,
		pictureWidth: 640, pictureHeight: 360, sourceAspect: 16.0 / 9}, canvasWidth: 1280, canvasHeight: 720, rate: Rate{Num: 25, Den: 1}}
}

// partFilters joins the filters of chain parts the way build joins them, without the group sendcmds.
func partFilters(parts []chainStage) string {
	filters := make([]string, 0, len(parts))
	for _, part := range parts {
		if part.filter != "" {
			filters = append(filters, part.filter)
		}
	}
	return strings.Join(filters, ",")
}

// matrixText is a saturation matrix as colorchannelmixer options, row by row.
func matrixText(m [9]float64) string {
	return fmt.Sprintf("rr=%s:rg=%s:rb=%s:gr=%s:gg=%s:gb=%s:br=%s:bg=%s:bb=%s",
		effectNum(m[0]), effectNum(m[1]), effectNum(m[2]), effectNum(m[3]), effectNum(m[4]), effectNum(m[5]), effectNum(m[6]), effectNum(m[7]), effectNum(m[8]))
}

// curveText is the three lutrgb options of a curve triple.
func curveText(c [3]ChannelCurve) string {
	return fmt.Sprintf("r='%s':g='%s':b='%s'", channelCurveExpr(c[0]).String(), channelCurveExpr(c[1]).String(), channelCurveExpr(c[2]).String())
}

// TestEffectChainSnapshots pins the colour and LUT stages (index §4.10, §4.11):
//   - identity entries add nothing;
//   - formats are inserted only on a change;
//   - keyframed params become named instances with command tracks;
//   - commands after a LUT blend move to command group 1.
//
// The curve texts come from channelCurveExpr, which TestChannelCurveExprMatchesApply checks; the keyframed LUT mix is
// evaluated here.
func TestEffectChainSnapshots(t *testing.T) {
	table := &LUT3D{Size: 2, DomainMax: [3]float64{1, 1, 1}, Data: make([]float32, 2*2*2*3)}
	color := func(params map[string]any, keys map[string][]Keyframe) EffectInstance {
		return EffectInstance{ID: "color", On: true, Params: params, Keyframes: keys}
	}
	lutEntry := func(ref string, intensity float64, keys []Keyframe) EffectInstance {
		entry := EffectInstance{ID: "lut", On: true, Params: map[string]any{"lut": ref, "intensity": intensity}}
		if keys != nil {
			entry.Keyframes = map[string][]Keyframe{"intensity": keys}
		}
		return entry
	}

	// Identity entries and entries that are off add nothing.
	for name, c := range map[string]struct {
		entry EffectInstance
		luts  map[int]*LUT3D
	}{
		"color defaults":    {color(map[string]any{}, nil), nil},
		"color at defaults": {color(map[string]any{"temperature": 6500.0, "gamma": 1.0, "saturation": 1.0}, nil), nil},
		"color off":         {EffectInstance{ID: "color", On: false, Params: map[string]any{"exposure": 1.0}}, nil},
		"lut without table": {lutEntry("", 1, nil), nil},
		"lut at zero":       {lutEntry("builtin:warm", 0, nil), map[int]*LUT3D{0: table}},
	} {
		if parts := effectTestChain(effectTestClip(10, c.entry), false, c.luts).effects(); len(parts) != 0 {
			t.Errorf("%s adds %q", name, partFilters(parts))
		}
	}

	// Static colour: lutrgb with the three curve texts and the saturation matrix, behind one format.
	static := effectTestClip(10, color(map[string]any{"exposure": 0.5, "saturation": 1.2}, nil))
	want := "format=rgba,lutrgb=" + curveText(colorCurves(colorParamsAt(static, 0, 0))) + ",colorchannelmixer=" + matrixText(saturationMatrix(1.2))
	if got := partFilters(effectTestChain(static, false, nil).effects()); got != want {
		t.Fatalf("static colour:\n got %s\nwant %s", got, want)
	}

	// Keyframed exposure: a named lutrgb starting at frame 0's curves, with curve tracks r, g, b for every window frame.
	keyed := effectTestClip(11, color(map[string]any{"contrast": 1.2}, map[string][]Keyframe{"exposure": linearKeys(0, 1, 10)}))
	parts := effectTestChain(keyed, false, nil).effects()
	if want := "format=rgba,lutrgb@vs_color_0_1_0=" + curveText(colorCurves(colorParamsAt(keyed, 0, 0))); partFilters(parts) != want || len(parts) != 1 || len(parts[0].tracks) != 3 {
		t.Fatalf("keyframed colour = %q with %d parts", partFilters(parts), len(parts))
	}
	last := colorCurves(colorParamsAt(keyed, 0, 10))
	for ch, option := range []string{"r", "g", "b"} {
		track := parts[0].tracks[ch]
		if track.Target != "lutrgb@vs_color_0_1_0" || track.Option != option || track.Values != nil || len(track.Curves) != 11 || track.Curves[10] != last[ch] {
			t.Fatalf("curve track %d = %+v", ch, track)
		}
	}

	// Keyframed saturation alone: an unchanged curve adds no lutrgb; the named colorchannelmixer gets nine tracks.
	sat := effectTestClip(11, color(map[string]any{}, map[string][]Keyframe{"saturation": linearKeys(1, 0, 10)}))
	parts = effectTestChain(sat, false, nil).effects()
	if want := "format=rgba,colorchannelmixer@vs_sat_0_1_0=" + matrixText(saturationMatrix(1)); partFilters(parts) != want || len(parts) != 1 || len(parts[0].tracks) != 9 {
		t.Fatalf("keyframed saturation = %q", partFilters(parts))
	}
	end := saturationMatrix(0)
	for i, option := range []string{"rr", "rg", "rb", "gr", "gg", "gb", "br", "bg", "bb"} {
		if track := parts[0].tracks[i]; track.Target != "colorchannelmixer@vs_sat_0_1_0" || track.Option != option || len(track.Values) != 11 || track.Values[10] != end[i] {
			t.Fatalf("saturation track %d = %+v", i, track)
		}
	}

	// LUT intensities: lut3d alone at 1, all_opacity in between (capped at 0.999), every lut3d trilinear.
	split := "format=gbrap,split=2[vs_fx_0_1_0_in][vs_fx_0_1_0_orig];[vs_fx_0_1_0_in]lut3d=file=vs_lut_0_1_0.cube:interp=trilinear[vs_fx_0_1_0_lut];[vs_fx_0_1_0_lut][vs_fx_0_1_0_orig]"
	for name, c := range map[string]struct {
		intensity float64
		want      string
		twoInput  bool
	}{
		"full":      {1, "format=gbrap,lut3d=file=vs_lut_0_1_0.cube:interp=trilinear", false},
		"partial":   {0.6, split + "blend=all_mode=normal:all_opacity=0.6", true},
		"near full": {0.9995, split + "blend=all_mode=normal:all_opacity=0.999", true},
	} {
		parts := effectTestChain(effectTestClip(10, lutEntry("builtin:warm", c.intensity, nil)), false, map[int]*LUT3D{0: table}).effects()
		if partFilters(parts) != c.want || len(parts) != 1 || parts[0].twoInput != c.twoInput || len(parts[0].luts) != 1 || parts[0].luts[0].name != "vs_lut_0_1_0.cube" || parts[0].luts[0].lut != table {
			t.Fatalf("lut %s = %q, parts %+v", name, partFilters(parts), parts)
		}
	}

	// Keyframed intensity: blend all_expr B + (A − B)·k(T), evaluated at every window frame.
	keyedLUT := effectTestClip(11, lutEntry("builtin:warm", 1, linearKeys(0, 1, 10)))
	chain := effectTestChain(keyedLUT, false, map[int]*LUT3D{0: table})
	spec, _ := EffectByID("lut")
	mix := lutMixExpr(chain.entryInput(0, spec))
	parts = chain.effects()
	if want := split + "blend=all_expr='" + mix.String() + "'"; partFilters(parts) != want || len(parts) != 1 || !parts[0].twoInput || len(parts[0].tracks) != 0 {
		t.Fatalf("keyframed lut = %q", partFilters(parts))
	}
	for k := range 11 {
		intensity := effectValueAt(keyedLUT, 0, "intensity", k)
		if got := mix.eval(map[string]float64{"T": float64(k) / 25, "A": 200, "B": 100}); math.Abs(got-(100+100*intensity)) > 1e-9 {
			t.Fatalf("frame %d: mix = %v, want %v", k, got, 100+100*intensity)
		}
	}

	// Formats change only where the next builder needs another one; an entry that is off is skipped.
	stack := effectTestClip(10,
		color(map[string]any{"contrast": 1.2}, nil),
		lutEntry("builtin:warm", 1, nil), lutEntry("builtin:cool", 1, nil),
		EffectInstance{ID: "color", On: false, Params: map[string]any{"contrast": 0.5}},
		color(map[string]any{"saturation": 0.5}, nil))
	want = "format=rgba,lutrgb=" + curveText(colorCurves(colorParamsAt(stack, 0, 0))) +
		",format=gbrap,lut3d=file=vs_lut_0_1_1.cube:interp=trilinear,lut3d=file=vs_lut_0_1_2.cube:interp=trilinear" +
		",format=rgba,colorchannelmixer=" + matrixText(saturationMatrix(0.5))
	if got := partFilters(effectTestChain(stack, false, map[int]*LUT3D{1: table, 2: table}).effects()); got != want {
		t.Fatalf("format insertion:\n got %s\nwant %s", got, want)
	}

	// Commands after a LUT blend: the second colour entry's matrix and the clip's opacity read group 1, which sits right
	// after the blend; the first colour entry stays in group 0 after the window.
	grouped := effectTestClip(11,
		color(map[string]any{}, map[string][]Keyframe{"exposure": linearKeys(0, 1, 10)}),
		lutEntry("builtin:warm", 0.6, nil),
		color(map[string]any{}, map[string][]Keyframe{"saturation": linearKeys(1, 0.5, 10)}))
	grouped.Keyframes = map[string][]Keyframe{"opacity": linearKeys(1, 0.5, 10)}
	built := effectTestChain(grouped, false, map[int]*LUT3D{1: table}).build("vs_clip_0_1")
	if !strings.Contains(built.filter, "setpts=PTS-STARTPTS,sendcmd=f=vs_cmd_0_1.txt,format=rgba,lutrgb@vs_color_0_1_0=") ||
		!strings.Contains(built.filter, "blend=all_mode=normal:all_opacity=0.6,sendcmd=f=vs_cmd_0_1_1.txt,format=rgba,colorchannelmixer@vs_sat_0_1_2=") {
		t.Fatalf("command groups:\n%s", built.filter)
	}
	if len(built.commands) != 2 || built.commands[0].name != "vs_cmd_0_1.txt" || len(built.commands[0].tracks) != 3 ||
		built.commands[1].name != "vs_cmd_0_1_1.txt" || len(built.commands[1].tracks) != 10 ||
		built.commands[1].tracks[9].Target != "colorchannelmixer@vs_opacity_0_1" {
		t.Fatalf("command files = %+v", built.commands)
	}
	if len(built.luts) != 1 || built.luts[0].name != "vs_lut_0_1_1.cube" || built.luts[0].lut != table {
		t.Fatalf("lut files = %+v", built.luts)
	}
}
