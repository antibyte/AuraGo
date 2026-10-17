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
	split := "format=gbrap,split=2[vs_fx_0_1_0_in][vs_fx_0_1_0_orig];[vs_fx_0_1_0_in]lut3d=file=vs_lut_0.cube:interp=trilinear[vs_fx_0_1_0_lut];[vs_fx_0_1_0_lut][vs_fx_0_1_0_orig]"
	for name, c := range map[string]struct {
		intensity float64
		want      string
		twoInput  bool
	}{
		"full":      {1, "format=gbrap,lut3d=file=vs_lut_0.cube:interp=trilinear", false},
		"partial":   {0.6, split + "blend=all_mode=normal:all_opacity=0.6", true},
		"near full": {0.9995, split + "blend=all_mode=normal:all_opacity=0.999", true},
	} {
		parts := effectTestChain(effectTestClip(10, lutEntry("builtin:warm", c.intensity, nil)), false, map[int]*LUT3D{0: table}).effects()
		if partFilters(parts) != c.want || len(parts) != 1 || parts[0].twoInput != c.twoInput || len(parts[0].luts) != 1 || parts[0].luts[0].name != "vs_lut_0.cube" || parts[0].luts[0].lut != table {
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
		",format=gbrap,lut3d=file=vs_lut_0.cube:interp=trilinear,lut3d=file=vs_lut_1.cube:interp=trilinear" +
		",format=rgba,colorchannelmixer=" + matrixText(saturationMatrix(0.5))
	other := &LUT3D{Size: 2, DomainMax: [3]float64{1, 1, 1}, Data: make([]float32, 2*2*2*3)}
	if got := partFilters(effectTestChain(stack, false, map[int]*LUT3D{1: table, 2: other}).effects()); got != want {
		t.Fatalf("format insertion:\n got %s\nwant %s", got, want)
	}

	// Two entries of one table read one file; a second table gets the next number (index §4.11).
	shared := partFilters(effectTestChain(stack, false, map[int]*LUT3D{1: table, 2: table}).effects())
	if !strings.Contains(shared, "lut3d=file=vs_lut_0.cube:interp=trilinear,lut3d=file=vs_lut_0.cube:interp=trilinear") || strings.Contains(shared, "vs_lut_1.cube") {
		t.Fatalf("one table twice:\n%s", shared)
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
	if len(built.luts) != 1 || built.luts[0].name != "vs_lut_0.cube" || built.luts[0].lut != table {
		t.Fatalf("lut files = %+v", built.luts)
	}
}

// TestDetailEffectChainSnapshots pins the blur, sharpen and vignette stages (index §4.10, §4.20):
//   - blur on gbrap, premultiplied on a still;
//   - sharpen never left out, and commanded when keyframed;
//   - vignette with dither off: constants evaluated once, or a t expression per frame;
//   - the alpha wrapper on a still, after which commands use group 1.
func TestDetailEffectChainSnapshots(t *testing.T) {
	entry := func(id string, params map[string]any, keys map[string][]Keyframe) EffectInstance {
		return EffectInstance{ID: id, On: true, Params: params, Keyframes: keys}
	}
	sigma := effectNum(blurSigma(0.01, 360))
	for name, c := range map[string]struct {
		entry EffectInstance
		image bool
		want  string
	}{
		"blur video":   {entry("blur", map[string]any{"radius": 0.01}, nil), false, "format=gbrap,gblur=sigma=" + sigma + ":steps=4"},
		"blur still":   {entry("blur", map[string]any{"radius": 0.01}, nil), true, "format=gbrap,premultiply=inplace=1,gblur=sigma=" + sigma + ":steps=4,unpremultiply=inplace=1"},
		"blur zero":    {entry("blur", map[string]any{"radius": 0.0}, nil), false, ""},
		"sharpen zero": {entry("sharpen", map[string]any{"strength": 0.0}, nil), false, "format=gbrap,cas=strength=0"},
		"sharpen":      {entry("sharpen", map[string]any{"strength": 0.35}, nil), false, "format=gbrap,cas=strength=0.35"},
		"vignette":     {entry("vignette", map[string]any{"amount": 0.5, "size": 1.0}, nil), false, "format=rgb24,vignette=angle=" + effectNum(vignetteAngle(0.5, 1, 640, 360)) + ":eval=init:dither=0"},
		"vignette off": {entry("vignette", map[string]any{"amount": 0.0}, nil), false, ""},
	} {
		if got := partFilters(effectTestChain(effectTestClip(10, c.entry), c.image, nil).effects()); got != c.want {
			t.Errorf("%s:\n got %s\nwant %s", name, got, c.want)
		}
	}

	// Keyframed sharpen: a named cas whose strength follows the commands.
	sharpen := effectTestClip(11, entry("sharpen", map[string]any{}, map[string][]Keyframe{"strength": linearKeys(0.2, 1, 10)}))
	parts := effectTestChain(sharpen, false, nil).effects()
	if partFilters(parts) != "format=gbrap,cas@vs_sharpen_0_1_0=strength=0.2" || len(parts) != 1 || len(parts[0].tracks) != 1 {
		t.Fatalf("keyframed sharpen = %q", partFilters(parts))
	}
	if track := parts[0].tracks[0]; track.Target != "cas@vs_sharpen_0_1_0" || track.Option != "strength" || len(track.Values) != 11 || track.Values[10] != 1 || math.Abs(track.Values[5]-0.6) > 1e-12 {
		t.Fatalf("sharpen track = %+v", track)
	}

	// Keyframed vignette: the angle expression evaluates vignetteAngle at every window frame.
	vignette := effectTestClip(11, entry("vignette", map[string]any{"size": 0.8}, map[string][]Keyframe{"amount": linearKeys(0.2, 1, 10)}))
	chain := effectTestChain(vignette, false, nil)
	spec, _ := EffectByID("vignette")
	angle := vignetteAngleExpr(chain.entryInput(0, spec))
	if got := partFilters(chain.effects()); got != "format=rgb24,vignette=angle='"+angle.String()+"':eval=frame:dither=0" {
		t.Fatalf("keyframed vignette = %s", got)
	}
	for k := range 11 {
		want := vignetteAngle(effectValueAt(vignette, 0, "amount", k), 0.8, 640, 360)
		if got := angle.eval(map[string]float64{"t": float64(k) / 25}); math.Abs(got-want) > 1e-9 {
			t.Fatalf("frame %d: angle %v, want %v", k, got, want)
		}
	}

	// A still's vignette is a two-input stage. The keyframed sharpen after it reads command group 1, right behind
	// alphamerge; group 0 stays empty.
	still := effectTestClip(11,
		entry("vignette", map[string]any{"amount": 0.6, "size": 1.0}, nil),
		entry("sharpen", map[string]any{}, map[string][]Keyframe{"strength": linearKeys(0.2, 1, 10)}))
	built := effectTestChain(still, true, nil).build("vs_clip_0_1")
	shade := "vignette=angle=" + effectNum(vignetteAngle(0.6, 1, 640, 360)) + ":eval=init:dither=0"
	wrapper := "format=gbrap,split=2[vs_fx_0_1_0_rgb][vs_fx_0_1_0_alpha];[vs_fx_0_1_0_alpha]alphaextract[vs_fx_0_1_0_mask];[vs_fx_0_1_0_rgb]format=rgb24," + shade +
		",format=gbrap[vs_fx_0_1_0_shaded];[vs_fx_0_1_0_shaded][vs_fx_0_1_0_mask]alphamerge,sendcmd=f=vs_cmd_0_1_1.txt,cas@vs_sharpen_0_1_1=strength=0.2"
	if !strings.Contains(built.filter, wrapper) || strings.Contains(built.filter, "sendcmd=f=vs_cmd_0_1.txt") {
		t.Fatalf("vignette wrapper and group 1:\n%s", built.filter)
	}
	if len(built.commands) != 1 || built.commands[0].name != "vs_cmd_0_1_1.txt" || len(built.commands[0].tracks) != 1 || built.commands[0].tracks[0].Target != "cas@vs_sharpen_0_1_1" {
		t.Fatalf("command files = %+v", built.commands)
	}
}

// TestBuildChromaKeyEffect pins the chroma_key stage (index §4.22):
//   - colorkey at the Go-written key colour, and despill with the type and the screen channel's scale taken from it;
//   - despill left out at spill 0; a key colour that is neither green nor blue is despilled as blue when its blue is larger;
//   - keyed similarity or smoothness: the named colorkey@vs_key_L_C_I with similarity and blend tracks;
//   - where the input alpha can be below 255 (a still, a second key), the alpha wrapper when colorkeyReplacesAlpha; it
//     leaves rgba, its keyed colorkey reads the group in front of it, and later commands read the next group.
func TestBuildChromaKeyEffect(t *testing.T) {
	key := func(params map[string]any, keys map[string][]Keyframe) EffectInstance {
		return EffectInstance{ID: "chroma_key", On: true, Params: params, Keyframes: keys}
	}
	params := func(changes map[string]any) map[string]any {
		out := map[string]any{"color": "#00b140", "similarity": 0.25, "smoothness": 0.1, "spill": 0.5}
		for name, value := range changes {
			out[name] = value
		}
		return out
	}
	green := "colorkey=color=0x00B140:similarity=0.25:blend=0.1"
	spill := "despill=type=green:mix=0.5:expand=0:red=0:green=-0.5:blue=0:brightness=0:alpha=0"
	for name, c := range map[string]struct {
		entry EffectInstance
		want  string
	}{
		"green key":  {key(params(nil), nil), "format=rgba," + green + "," + spill},
		"spill zero": {key(params(map[string]any{"spill": 0.0}), nil), "format=rgba," + green},
		"blue key": {key(params(map[string]any{"color": "#0047bb", "similarity": 0.3, "smoothness": 0.0, "spill": 0.8}), nil),
			"format=rgba,colorkey=color=0x0047BB:similarity=0.3:blend=0,despill=type=blue:mix=0.5:expand=0:red=0:green=0:blue=-0.8:brightness=0:alpha=0"},
		"neither green nor blue": {key(params(map[string]any{"color": "#c03070", "spill": 1.0}), nil),
			"format=rgba,colorkey=color=0xC03070:similarity=0.25:blend=0.1,despill=type=blue:mix=0.5:expand=0:red=0:green=0:blue=-1:brightness=0:alpha=0"},
	} {
		parts := effectTestChain(effectTestClip(10, c.entry), false, nil).effects()
		if got := partFilters(parts); got != c.want || len(parts) != 1 || parts[0].twoInput || len(parts[0].tracks) != 0 {
			t.Errorf("%s:\n got %s (%d parts)\nwant %s", name, got, len(parts), c.want)
		}
	}

	// An entry without params builds exactly what writing the registry defaults out builds.
	spec, _ := EffectByID("chroma_key")
	explicit := map[string]any{}
	for _, p := range spec.Params {
		explicit[p.ID] = p.Default
	}
	defaults := partFilters(effectTestChain(effectTestClip(10, key(map[string]any{}, nil)), false, nil).effects())
	if want := partFilters(effectTestChain(effectTestClip(10, key(explicit, nil)), false, nil).effects()); defaults != want || defaults == "" {
		t.Fatalf("defaults:\n got %s\nwant %s", defaults, want)
	}

	// Keyed similarity: the named colorkey starts at frame 0's values, and both options get a track for every window frame.
	keyed := effectTestClip(11, key(params(map[string]any{"smoothness": 0.2}), map[string][]Keyframe{"similarity": linearKeys(0.1, 0.5, 10)}))
	parts := effectTestChain(keyed, false, nil).effects()
	if want := "format=rgba,colorkey@vs_key_0_1_0=color=0x00B140:similarity=0.1:blend=0.2," + spill; partFilters(parts) != want || len(parts) != 1 || len(parts[0].tracks) != 2 {
		t.Fatalf("keyed similarity = %q, parts %+v", partFilters(parts), parts)
	}
	similarity, blend := parts[0].tracks[0], parts[0].tracks[1]
	if similarity.Target != "colorkey@vs_key_0_1_0" || similarity.Option != "similarity" || len(similarity.Values) != 11 || math.Abs(similarity.Values[5]-0.3) > 1e-12 || similarity.Values[10] != 0.5 {
		t.Fatalf("similarity track = %+v", similarity)
	}
	if blend.Target != similarity.Target || blend.Option != "blend" || len(blend.Values) != 11 || blend.Values[0] != 0.2 || blend.Values[10] != 0.2 {
		t.Fatalf("blend track = %+v", blend)
	}

	// A still, and a second key on a video, can carry alpha below 255.
	wrapper := func(id, keyer string) string {
		return fmt.Sprintf("split=2[vs_fx_%[1]s_in][vs_fx_%[1]s_orig];[vs_fx_%[1]s_orig]alphaextract[vs_fx_%[1]s_a];[vs_fx_%[1]s_in]%[2]s[vs_fx_%[1]s_keyed];[vs_fx_%[1]s_keyed]split=2[vs_fx_%[1]s_k0][vs_fx_%[1]s_k1];[vs_fx_%[1]s_k1]alphaextract[vs_fx_%[1]s_ka];[vs_fx_%[1]s_ka][vs_fx_%[1]s_a]blend=all_mode=multiply[vs_fx_%[1]s_m];[vs_fx_%[1]s_k0][vs_fx_%[1]s_m]alphamerge", id, keyer)
	}
	still := effectTestChain(effectTestClip(10, key(params(nil), nil)), true, nil).effects()
	second := effectTestChain(effectTestClip(10, key(params(map[string]any{"spill": 0.0}), nil), key(params(map[string]any{"color": "#0047bb"}), nil)), false, nil).effects()
	blueKeyer := "colorkey=color=0x0047BB:similarity=0.25:blend=0.1,despill=type=blue:mix=0.5:expand=0:red=0:green=0:blue=-0.5:brightness=0:alpha=0"
	if colorkeyReplacesAlpha {
		if got, want := partFilters(still), "format=rgba,"+wrapper("0_1_0", green+","+spill); got != want || len(still) != 1 || !still[0].twoInput {
			t.Fatalf("still:\n got %s\nwant %s", got, want)
		}
		if got, want := partFilters(second), "format=rgba,"+green+","+wrapper("0_1_1", blueKeyer); got != want || len(second) != 2 || second[0].twoInput || !second[1].twoInput {
			t.Fatalf("second key:\n got %s\nwant %s", got, want)
		}
	} else {
		if got, want := partFilters(still), "format=rgba,"+green+","+spill; got != want || still[0].twoInput {
			t.Fatalf("still without the wrapper:\n got %s\nwant %s", got, want)
		}
		if got, want := partFilters(second), "format=rgba,"+green+","+blueKeyer; got != want || second[1].twoInput {
			t.Fatalf("second key without the wrapper:\n got %s\nwant %s", got, want)
		}
	}

	// Command groups around the wrapper: the keyed colorkey in the split branch reads group 0 in front of the stage.
	// The colour entry after it takes the wrapper's rgba without a format, and the opacity commands read group 1 behind the merge.
	stack := effectTestClip(11,
		key(params(nil), map[string][]Keyframe{"similarity": linearKeys(0.1, 0.5, 10)}),
		EffectInstance{ID: "color", On: true, Params: map[string]any{"saturation": 0.5}})
	stack.Keyframes = map[string][]Keyframe{"opacity": linearKeys(1, 0.5, 10)}
	built := effectTestChain(stack, true, nil).build("vs_clip_0_1")
	if colorkeyReplacesAlpha {
		if !strings.Contains(built.filter, "setpts=PTS-STARTPTS,sendcmd=f=vs_cmd_0_1.txt,format=rgba,split=2[vs_fx_0_1_0_in][vs_fx_0_1_0_orig];[vs_fx_0_1_0_orig]alphaextract[vs_fx_0_1_0_a];[vs_fx_0_1_0_in]colorkey@vs_key_0_1_0=") ||
			!strings.Contains(built.filter, "[vs_fx_0_1_0_k0][vs_fx_0_1_0_m]alphamerge,sendcmd=f=vs_cmd_0_1_1.txt,colorchannelmixer=") {
			t.Fatalf("command groups around the wrapper:\n%s", built.filter)
		}
		if len(built.commands) != 2 || built.commands[0].name != "vs_cmd_0_1.txt" || len(built.commands[0].tracks) != 2 ||
			built.commands[1].name != "vs_cmd_0_1_1.txt" || len(built.commands[1].tracks) != 1 || built.commands[1].tracks[0].Target != "colorchannelmixer@vs_opacity_0_1" {
			t.Fatalf("command files = %+v", built.commands)
		}
	} else if len(built.commands) != 1 || len(built.commands[0].tracks) != 3 {
		t.Fatalf("command files without the wrapper = %+v", built.commands)
	}

	// Keyed spill (the despill command gate passed): despill@vs_spill_L_C_I with the screen channel's scale per frame.
	spilled := effectTestClip(11, key(params(nil), map[string][]Keyframe{"spill": linearKeys(1, 0, 10)}))
	parts = effectTestChain(spilled, false, nil).effects()
	if want := "format=rgba," + green + ",despill@vs_spill_0_1_0=type=green:mix=0.5:expand=0:red=0:green=-1:blue=0:brightness=0:alpha=0"; partFilters(parts) != want || len(parts[0].tracks) != 1 {
		t.Fatalf("keyed spill = %q, parts %+v", partFilters(parts), parts)
	}
	if track := parts[0].tracks[0]; track.Target != "despill@vs_spill_0_1_0" || track.Option != "green" || len(track.Values) != 11 || track.Values[0] != -1 || track.Values[10] != 0 || math.Signbit(track.Values[10]) {
		t.Fatalf("spill track = %+v", track)
	}
}
