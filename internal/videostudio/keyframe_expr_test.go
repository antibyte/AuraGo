package videostudio

import (
	"math"
	"strings"
	"testing"
)

func TestFFExprStringAndEval(t *testing.T) {
	for _, c := range []struct {
		name string
		expr ffExpr
		text string
		vars map[string]float64
		want float64
	}{
		{"negative literal in parentheses", exprOp("+", exprNum(1.5), exprOp("*", exprVar("t"), exprNum(-2))), "(1.5+(t*(-2)))", map[string]float64{"t": 3}, -4.5},
		{"registers and sequence", exprOp(";", exprSt(0, exprNum(0.1)), exprOp("round", exprOp("/", exprLd(0), exprNum(3)))), "(st(0,0.1);round((ld(0)/3)))", nil, 0},
		{"no exponent, no negative zero", exprOp("-", exprNum(0.0000001), exprNum(math.Copysign(0, -1))), "(0.0000001-0)", nil, 0.0000001},
		{"if, lt and gte", exprOp("if", exprOp("lt", exprVar("t"), exprNum(2)), exprNum(10), exprOp("gte", exprVar("t"), exprNum(2))), "if(lt(t,2),10,gte(t,2))", map[string]float64{"t": 2}, 1},
		{"max skips NaN like libavutil", exprOp("max", exprVar("t"), exprNum(0)), "max(t,0)", map[string]float64{"t": math.NaN()}, 0},
		{"min, floor, ceil, sqrt", exprOp("min", exprOp("floor", exprNum(2.5)), exprOp("+", exprOp("ceil", exprNum(0.2)), exprOp("sqrt", exprNum(9)))), "min(floor(2.5),(ceil(0.2)+sqrt(9)))", nil, 2},
	} {
		if got := c.expr.String(); got != c.text {
			t.Errorf("%s: String() = %q, want %q", c.name, got, c.text)
		}
		if got := c.expr.eval(c.vars); got != c.want {
			t.Errorf("%s: eval = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestKeyframeExprBuildersText(t *testing.T) {
	clip := Clip{Start: 10, Offset: 7, Duration: 40}
	if got, want := windowSourceExpr(clip, 15, nil, Rate{Num: 25, Den: 1}).String(), "(st(0,(15+round(((max(t,0)*25)/1))));st(1,(7+((ld(0)-10)*1))))"; got != want {
		t.Fatalf("windowSourceExpr:\n got %s\nwant %s", got, want)
	}
	keys := []Keyframe{{F: 0, V: 0.25, E: EaseLinear}, {F: 10, V: 0.75, E: EaseLinear}}
	if got, want := curveExpr(keys, exprLd(1)).String(), "(st(3,max(0,min(1,((ld(1)-0)/10))));(0.25+(0.5*ld(3))))"; got != want {
		t.Fatalf("curveExpr:\n got %s\nwant %s", got, want)
	}
	if got := curveExpr(keys[:1], exprLd(1)).String(); got != "0.25" {
		t.Fatalf("one key = %s, want the constant 0.25", got)
	}
	// Three spans: the balanced tree splits at the middle key first.
	three := []Keyframe{{F: 0, V: 0, E: EaseHold}, {F: 10, V: 1, E: EaseLinear}, {F: 20, V: 0, E: EaseIn}, {F: 30, V: 1, E: EaseOut}}
	if text := curveExpr(three, exprLd(1)).String(); !strings.HasPrefix(text, "if(lt(ld(1),10),") || !strings.Contains(text, "if(lt(ld(1),20),") {
		t.Fatalf("three spans: %s", text)
	}
	// The curve holds before the first and after the last key, and every easing reaches its key values. At 25 the span
	// runs from 0 (key 20, ease_in) to 1 (key 30): 0 + 1·0.5³ = 0.125 (the same number sampleKeyframes gives).
	for _, c := range []struct{ source, want float64 }{{-5, 0}, {5, 0}, {10, 1}, {15, 0.5}, {25, 0.125}, {30, 1}, {99, 1}} {
		got := exprOp(";", exprSt(1, exprNum(c.source)), curveExpr(three, exprLd(1))).eval(nil)
		if math.Abs(got-c.want) > 1e-12 {
			t.Errorf("curve at source %v = %v, want %v", c.source, got, c.want)
		}
	}
}

// TestKeyframeExprMatchesValueAt evaluates the export's per-frame expressions in Go over every frame of windows that
// start at and inside a clip, at three rates, with speed 1, speed 1.5 and a ramp: the source position, every keyframe
// curve, the even box (clipBox) and the angle must match ValueAt.
func TestKeyframeExprMatchesValueAt(t *testing.T) {
	base := Clip{ID: "k", AssetID: "v", Start: 10, Offset: 7, Duration: 90, X: 0.1, Y: 0.2, Width: 0.5, Height: 0.5, Rotation: 12, Opacity: 1, Volume: 1, Fit: FitContain,
		Keyframes: map[string][]Keyframe{
			"x":        {{F: 7, V: 0.05, E: EaseInOut}, {F: 40, V: 0.42, E: EaseOut}, {F: 70, V: 0.3, E: EaseHold}, {F: 90, V: 0.1, E: EaseLinear}},
			"width":    {{F: 10, V: 0.3, E: EaseIn}, {F: 60, V: 0.75, E: EaseLinear}},
			"height":   {{F: 20, V: 0.6, E: EaseLinear}},
			"rotation": {{F: 0, V: -30, E: EaseOut}, {F: 80, V: 45, E: EaseInOut}},
		}}
	fast := base
	fast.Speed = 1.5
	ramped := base
	ramped.Ramp = []RampPoint{{F: 0, Speed: 1, E: EaseInOut}, {F: 30, Speed: 2.5, E: EaseLinear}, {F: 60, Speed: 0.5, E: EaseHold}, {F: 80, Speed: 1, E: EaseLinear}}
	for _, rate := range []Rate{{Num: 25, Den: 1}, {Num: 30000, Den: 1001}, {Num: 60, Den: 1}} {
		for name, clip := range map[string]Clip{"speed 1": base, "speed 1.5": fast, "ramp": ramped} {
			var pieces []rampPiece
			if len(clip.Ramp) > 0 {
				pieces = rampPieces(clip.Ramp, 0, float64(clip.Duration))
			}
			for _, at := range []int{clip.Start, clip.Start + 23} {
				box := boxExprs(clip, at, pieces, 1280, 720, rate)
				source := windowSourceExpr(clip, at, pieces, rate)
				for k := 0; at+k < clip.Start+clip.Duration; k++ {
					frame := at + k
					vars := map[string]float64{"t": float64(k) * (float64(rate.Den) / float64(rate.Num))}
					wantSource := float64(clip.Offset) + float64(frame-clip.Start)*ClipSpeed(clip)
					if pieces != nil {
						wantSource = float64(clip.Offset) + rampSource(pieces, float64(frame-clip.Start))
					}
					if got := exprOp(";", source, exprLd(1)).eval(vars); math.Abs(got-wantSource) > 1e-9 {
						t.Fatalf("%s at %s, window %d, frame %d: source %v, want %v", name, rate, at, frame, got, wantSource)
					}
					value := func(path string) float64 {
						v, ok := ValueAt(clip, path, frame)
						if !ok {
							t.Fatalf("ValueAt(%q) reports an unknown path", path)
						}
						return v
					}
					for _, path := range []string{"x", "width", "height", "rotation"} {
						if keys := clip.Keyframes[path]; len(keys) > 0 {
							if got := exprOp(";", source, curveExpr(keys, exprLd(1))).eval(vars); math.Abs(got-value(path)) > 1e-9 {
								t.Fatalf("%s at %s, window %d, frame %d: %s = %v, ValueAt %v", name, rate, at, frame, path, got, value(path))
							}
						}
					}
					x, y, w, h := clipBox(Clip{X: value("x"), Y: value("y"), Width: value("width"), Height: value("height")}, 1280, 720)
					if gw, gh := box.w.eval(vars), box.h.eval(vars); gw != float64(w) || gh != float64(h) {
						t.Fatalf("%s at %s, window %d, frame %d: box %vx%v, want %dx%d", name, rate, at, frame, gw, gh, w, h)
					}
					if cx, cy := box.cx.eval(vars), box.cy.eval(vars); cx != float64(x+w/2) || cy != float64(y+h/2) {
						t.Fatalf("%s at %s, window %d, frame %d: centre (%v, %v), want (%d, %d)", name, rate, at, frame, cx, cy, x+w/2, y+h/2)
					}
					if angle := box.angle.eval(vars); math.Abs(angle-value("rotation")*math.Pi/180) > 1e-12 {
						t.Fatalf("%s at %s, window %d, frame %d: angle %v, want %v", name, rate, at, frame, angle, value("rotation")*math.Pi/180)
					}
				}
			}
		}
	}
}

// The 2d mask expressions use hypot, abs, cos and sin, and geq's variables X, Y and T (index §4.11). eval computes them as
// libavutil does. exprSeq chains parts with ";". renameExprVar moves an expression to another time variable and leaves
// the original tree as it was.
func TestFFExprTrigAndHypot(t *testing.T) {
	vars := map[string]float64{"X": 3, "Y": 4, "T": 0.5}
	for _, c := range []struct {
		name string
		expr ffExpr
		text string
		want float64
	}{
		{"hypot of the geq coordinates", exprOp("hypot", exprVar("X"), exprVar("Y")), "hypot(X,Y)", 5},
		{"abs", exprOp("abs", exprOp("-", exprVar("X"), exprNum(10))), "abs((X-10))", 7},
		{"cos", exprOp("cos", exprNum(0)), "cos(0)", 1},
		{"sin of T", exprOp("sin", exprOp("*", exprVar("T"), exprNum(math.Pi))), "sin((T*" + exprNum(math.Pi).String() + "))", math.Sin(0.5 * math.Pi)},
		{"a sequence returns its last part", exprSeq(exprSt(2, exprVar("X")), exprSt(3, exprVar("Y")), exprOp("+", exprLd(2), exprLd(3))), "(st(2,X);(st(3,Y);(ld(2)+ld(3))))", 7},
	} {
		if got := c.expr.String(); got != c.text {
			t.Errorf("%s: String() = %q, want %q", c.name, got, c.text)
		}
		if got := c.expr.eval(vars); got != c.want {
			t.Errorf("%s: eval = %v, want %v", c.name, got, c.want)
		}
	}
	if got := exprOp("hypot", exprVar("Z"), exprNum(1)).eval(vars); !math.IsNaN(got) {
		t.Fatalf("hypot of a missing variable = %v, want NaN", got)
	}
	clip := Clip{Start: 10, Offset: 7, Duration: 40}
	rate := Rate{Num: 25, Den: 1}
	original := windowSourceExpr(clip, 15, nil, rate)
	before := original.String()
	if got, want := renameExprVar(original, "t", "T").String(), windowSourceExprVar(clip, 15, nil, rate, "T").String(); got != want {
		t.Fatalf("renamed window source %s, want %s", got, want)
	}
	if original.String() != before || !strings.Contains(before, "max(t,0)") {
		t.Fatalf("renameExprVar changed the original: %s", original.String())
	}
}
