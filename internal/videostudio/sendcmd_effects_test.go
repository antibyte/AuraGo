package videostudio

import (
	"math"
	"math/rand/v2"
	"reflect"
	"strings"
	"testing"
)

// Effect targets in sendcmd scripts (index §4.11, 2c):
//   - filter names may contain digits and targets may carry the stack index;
//   - options are whitelisted per filter and role;
//   - colour curves go only to lutrgb@vs_color and are written single-quoted through the fixed template;
//   - numbers on a curve option are refused, and so is anything else.
func TestSendCmdScriptEffectTargets(t *testing.T) {
	c1 := ChannelCurve{Mul: 1.25, Contrast: 1.1, Lift: 0.05, Gain: 0.9, InvGamma: 1}
	c2 := ChannelCurve{Mul: 1.5, Contrast: 1, Lift: -0.02, Gain: 1, InvGamma: 0.8}
	const text1 = "clip(round((255*pow(clip((((clip((((((val/255)*1.25)-0.5)*1.1)+0.5),0,1)*(1-0.05))+0.05)*0.9),0,1),1))),0,255)"
	if got := channelCurveExpr(c1).String(); got != text1 {
		t.Fatalf("curve template:\n got %s\nwant %s", got, text1)
	}
	tracks := []SendCmdTrack{
		{Target: "lutrgb@vs_color_1_2_0", Option: "r", Curves: []ChannelCurve{c1, c1, c2}},
		{Target: "colorchannelmixer@vs_sat_1_2_0", Option: "rr", Values: []float64{1, 1, 1.2}},
		{Target: "cas@vs_sharpen_1_2_3", Option: "strength", Values: []float64{0.5, 0.6, 0.6}},
		{Target: "crop@vs_picture_1_2", Option: "x", Values: []float64{0, 0, 2}},
	}
	got, err := sendCmdScript(Rate{Num: 25, Den: 1}, tracks)
	if err != nil {
		t.Fatal(err)
	}
	want := "0.000000 lutrgb@vs_color_1_2_0 r '" + text1 + "', colorchannelmixer@vs_sat_1_2_0 rr 1, cas@vs_sharpen_1_2_3 strength 0.5, crop@vs_picture_1_2 x 0;\n" +
		"0.020000 cas@vs_sharpen_1_2_3 strength 0.6;\n" +
		"0.060000 lutrgb@vs_color_1_2_0 r '" + channelCurveExpr(c2).String() + "', colorchannelmixer@vs_sat_1_2_0 rr 1.2, crop@vs_picture_1_2 x 2;\n"
	if got != want {
		t.Fatalf("script:\n%s\nwant:\n%s", got, want)
	}
	// Every quoted value lies inside the curve grammar.
	for _, line := range strings.Split(strings.TrimSuffix(got, "\n"), "\n") {
		parts := strings.Split(line, "'")
		for i := 1; i < len(parts); i += 2 {
			if !validSendCmdCurve(parts[i]) {
				t.Fatalf("quoted value outside the grammar: %s", parts[i])
			}
		}
	}
	for target, want := range map[string]bool{
		"lut3d@vs_lut_0_1_2": true, "cas@vs_sharpen_0_1": true, "colorchannelmixer@vs_opacity_0_1": true,
		"cas@vs_sharpen_0_1_2_3": false, "3d@vs_lut_0_1": false, "Cas@vs_sharpen_0_1": false, "cas@vs_sharpen2_0_1": false, "cas@vs_sharpen_0": false,
	} {
		if sendCmdTargetPattern.MatchString(target) != want {
			t.Errorf("target %q accepted = %v", target, !want)
		}
	}
	values, curves := []float64{1, 2, 3}, []ChannelCurve{c1, c1, c2}
	for name, track := range map[string]SendCmdTrack{
		"numbers on a curve option":         {Target: "lutrgb@vs_color_1_2_0", Option: "g", Values: values},
		"curves on a numeric option":        {Target: "cas@vs_sharpen_1_2_3", Option: "strength", Curves: curves},
		"curves and numbers":                {Target: "lutrgb@vs_color_1_2_0", Option: "b", Values: values, Curves: curves},
		"the alpha curve":                   {Target: "lutrgb@vs_color_1_2_0", Option: "a", Curves: curves},
		"a filter without commands":         {Target: "lut3d@vs_lut_1_2_1", Option: "file", Values: values},
		"saturation option of another role": {Target: "colorchannelmixer@vs_opacity_1_2", Option: "rr", Values: values},
		"position on the cover crop":        {Target: "crop@vs_crop_1_2", Option: "x", Values: values},
		"a non-finite curve":                {Target: "lutrgb@vs_color_1_2_0", Option: "r", Curves: []ChannelCurve{c1, {Mul: math.NaN(), Contrast: 1, Gain: 1, InvGamma: 1}, c2}},
	} {
		if _, err := sendCmdScript(Rate{Num: 25, Den: 1}, []SendCmdTrack{track}); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	for text, want := range map[string]bool{"clip(val,0,255)": true, "round((val*(-0.5)))": true, "pow(val,2)": true,
		"system(1)": false, "val;st(0,1)": false, "VAL": false, "val'": false, "1e5": false, "": false} {
		if validSendCmdCurve(text) != want {
			t.Errorf("validSendCmdCurve(%q) = %v", text, !want)
		}
	}
}

// The lutrgb template evaluates, in Go's libavutil-faithful ffExpr, to round(255·apply(val/255)) for every input level.
// It does so for the identity and for 50 random curves across the parameter ranges. Two mathematically equal forms may
// round differently only exactly at a half step.
func TestChannelCurveExprMatchesApply(t *testing.T) {
	rng := rand.New(rand.NewPCG(2026, 1017))
	curves := []ChannelCurve{{Mul: 1, Contrast: 1, Lift: 0, Gain: 1, InvGamma: 1}}
	for range 50 {
		curves = append(curves, ChannelCurve{
			Mul: 0.1 + rng.Float64()*7.9, Contrast: rng.Float64() * 3, Lift: -0.7 + rng.Float64()*1.4,
			Gain: -0.2 + rng.Float64()*4.4, InvGamma: 0.16 + rng.Float64()*6,
		})
	}
	for i, c := range curves {
		expr := channelCurveExpr(c)
		if !validSendCmdCurve(expr.String()) {
			t.Fatalf("curve %d outside the grammar: %s", i, expr.String())
		}
		for val := 0; val <= 255; val++ {
			got := expr.eval(map[string]float64{"val": float64(val)})
			exact := 255 * c.apply(float64(val)/255)
			want := math.Round(exact)
			if got != want && (math.Abs(exact-math.Floor(exact)-0.5) > 1e-9 || math.Abs(got-want) != 1) {
				t.Fatalf("curve %d %+v at %d: expression %v, apply %v", i, c, val, got, exact)
			}
			if i == 0 && got != float64(val) {
				t.Fatalf("the identity curve maps %d to %v", val, got)
			}
		}
	}
	// libavutil's clip is NaN for a NaN argument or min > max; pow is C pow.
	if got := exprOp("clip", exprVar("x"), exprNum(0), exprNum(1)).eval(nil); !math.IsNaN(got) {
		t.Fatalf("clip of a missing variable = %v", got)
	}
	if got := exprOp("clip", exprNum(0.5), exprNum(1), exprNum(0)).eval(nil); !math.IsNaN(got) {
		t.Fatalf("clip with min above max = %v", got)
	}
	if got := exprOp("clip", exprNum(-2), exprNum(0), exprNum(1)).eval(nil); got != 0 {
		t.Fatalf("clip below = %v", got)
	}
	if got := exprOp("pow", exprNum(0.25), exprNum(0.5)).eval(nil); got != 0.5 {
		t.Fatalf("pow = %v", got)
	}
}

// windowSourceExprVar is windowSourceExpr with another time variable: blend's all_expr reads T (seconds), as t is
// read elsewhere.
func TestWindowSourceExprVar(t *testing.T) {
	clip := Clip{Start: 10, Offset: 7, Duration: 40}
	rate := Rate{Num: 25, Den: 1}
	plain := windowSourceExpr(clip, 15, nil, rate).String()
	blend := windowSourceExprVar(clip, 15, nil, rate, "T")
	if blend.String() != strings.Replace(plain, "max(t,0)", "max(T,0)", 1) || plain != windowSourceExprVar(clip, 15, nil, rate, "t").String() {
		t.Fatalf("T form %s, t form %s", blend.String(), plain)
	}
	// Window frame 3 is absolute frame 18, source 7 + (18 − 10).
	if got := exprOp(";", blend, exprLd(1)).eval(map[string]float64{"T": 3.0 / 25}); got != 15 {
		t.Fatalf("source at T = 3 frames: %v, want 15", got)
	}
}

// Command groups (index §4.11):
//   - group 0 is read after window;
//   - a track belongs to the group of the last two-input stage before it;
//   - the sendcmd of group g follows the g-th two-input stage;
//   - targets inside a two-input stage are read by the group in front of it;
//   - empty groups have neither a file nor a sendcmd.
func TestCommandGroupAssembly(t *testing.T) {
	track := func(target, option string) SendCmdTrack {
		return SendCmdTrack{Target: target, Option: option, Values: []float64{1, 2}}
	}
	saturation := track("colorchannelmixer@vs_sat_1_2_0", "rr")
	sharpen := track("cas@vs_sharpen_1_2_2", "strength")
	opacity := track("colorchannelmixer@vs_opacity_1_2", "aa")
	table := &LUT3D{Size: 2, DomainMax: [3]float64{1, 1, 1}, Data: make([]float32, 24)}
	blend := "format=gbrap,split=2[vs_a_1_2_1][vs_b_1_2_1];[vs_b_1_2_1]lut3d=file=vs_lut_0.cube:interp=trilinear[vs_l_1_2_1];[vs_l_1_2_1][vs_a_1_2_1]blend=all_mode=normal:all_opacity=0.6"
	filters, commands, luts := assembleStages("1_2", []chainStage{
		{filter: "colorchannelmixer@vs_sat_1_2_0=rr=1", tracks: []SendCmdTrack{saturation}},
		{filter: blend, twoInput: true, luts: []lutFile{{name: "vs_lut_0.cube", lut: table}}},
		{},
		{filter: "cas@vs_sharpen_1_2_2=strength=1", tracks: []SendCmdTrack{sharpen}},
		{filter: "format=rgba"},
		{filter: "colorchannelmixer@vs_opacity_1_2=aa=1", tracks: []SendCmdTrack{opacity}},
	})
	if want := []string{"sendcmd=f=vs_cmd_1_2.txt", "colorchannelmixer@vs_sat_1_2_0=rr=1", blend, "sendcmd=f=vs_cmd_1_2_1.txt",
		"cas@vs_sharpen_1_2_2=strength=1", "format=rgba", "colorchannelmixer@vs_opacity_1_2=aa=1"}; !reflect.DeepEqual(filters, want) {
		t.Fatalf("filters:\n got %q\nwant %q", filters, want)
	}
	if want := []sendCmdFile{{name: "vs_cmd_1_2.txt", tracks: []SendCmdTrack{saturation}}, {name: "vs_cmd_1_2_1.txt", tracks: []SendCmdTrack{sharpen, opacity}}}; !reflect.DeepEqual(commands, want) {
		t.Fatalf("command files = %+v", commands)
	}
	if !reflect.DeepEqual(luts, []lutFile{{name: "vs_lut_0.cube", lut: table}}) {
		t.Fatalf("LUT files = %+v", luts)
	}
	for _, file := range commands {
		if !sendCmdFileName.MatchString(file.name) {
			t.Fatalf("%s does not match sendCmdFileName", file.name)
		}
	}
	filters, commands, _ = assembleStages("0_0", []chainStage{
		{filter: "first", twoInput: true, tracks: []SendCmdTrack{saturation}},
		{filter: "second", twoInput: true},
		{filter: "after", tracks: []SendCmdTrack{opacity}},
	})
	if !reflect.DeepEqual(filters, []string{"sendcmd=f=vs_cmd_0_0.txt", "first", "second", "sendcmd=f=vs_cmd_0_0_2.txt", "after"}) ||
		!reflect.DeepEqual(commands, []sendCmdFile{{name: "vs_cmd_0_0.txt", tracks: []SendCmdTrack{saturation}}, {name: "vs_cmd_0_0_2.txt", tracks: []SendCmdTrack{opacity}}}) {
		t.Fatalf("two merges: filters %q, files %+v", filters, commands)
	}
	filters, commands, luts = assembleStages("0_0", []chainStage{{filter: "a"}, {filter: "b", twoInput: true}, {}})
	if !reflect.DeepEqual(filters, []string{"a", "b"}) || commands != nil || luts != nil {
		t.Fatalf("no tracks: filters %q, files %+v, luts %+v", filters, commands, luts)
	}
}
