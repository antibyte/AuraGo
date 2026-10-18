package videostudio

import (
	"math"
	"strconv"
	"strings"
)

// ffExpr is one node of an FFmpeg expression (libavutil/eval.c) that the export writes into a filter option: scale,
// pad, overlay and rotate get per-frame expressions in t, setpts one in PTS, and 2d's mask geq one in X, Y and T
// (index §4.11, §4.18, §4.22). String writes the
// text; eval computes the same tree in Go with the same IEEE double operations in the same order, so tests check every
// frame without FFmpeg and the export derives command values (the cover crop) from exactly the numbers FFmpeg computes.
//
// op is one of:
//   - "num": the literal num;
//   - "var": the variable name (t, T, PTS, TB, val, and geq's pixel position X, Y);
//   - an infix: "+", "-", "*", "/", or ";" (evaluate the left operand, return the right one);
//   - a function: "if", "lt", "gte", "max", "min", "round", "floor", "ceil", "sqrt", "clip" (clip(x, min, max)),
//     "pow", "hypot", "abs", "cos", "sin" (2d masks), "st" (store in register 0..9) and "ld".
//
// Every infix node is written in parentheses and every literal without an exponent, so neither operator precedence nor
// av_strtod's suffixes ever matter.
type ffExpr struct {
	op   string
	num  float64
	name string
	args []ffExpr
}

// exprNum is a literal; negative zero becomes zero, so the text never reads "-0".
func exprNum(value float64) ffExpr {
	if value == 0 {
		value = 0
	}
	return ffExpr{op: "num", num: value}
}

// exprIsNum reports whether e is the literal value (boxExprs leaves the 2b expressions alone when an animation part is
// a literal rest value).
func exprIsNum(e ffExpr, value float64) bool { return e.op == "num" && e.num == value }

// exprVar is a filter variable such as t.
func exprVar(name string) ffExpr { return ffExpr{op: "var", name: name} }

// exprOp applies an infix operator or a function to its operands.
func exprOp(op string, args ...ffExpr) ffExpr { return ffExpr{op: op, args: args} }

// exprLd reads expression register r; exprSt stores value in it and returns value. FFmpeg keeps ten registers per
// parsed expression; every builder stores a register before it reads it.
func exprLd(register int) ffExpr { return exprOp("ld", exprNum(float64(register))) }

func exprSt(register int, value ffExpr) ffExpr {
	return exprOp("st", exprNum(float64(register)), value)
}

// exprSeq evaluates its parts left to right and returns the last one: (p0;(p1;(…;pn))). It needs at least one part.
func exprSeq(parts ...ffExpr) ffExpr {
	out := parts[len(parts)-1]
	for i := len(parts) - 2; i >= 0; i-- {
		out = exprOp(";", parts[i], out)
	}
	return out
}

// renameExprVar returns e with every variable named from renamed to to; the original tree is unchanged. The mask
// reads the box expressions that scale and pad evaluate in t through geq's T (index §4.22).
func renameExprVar(e ffExpr, from, to string) ffExpr {
	if e.op == "var" && e.name == from {
		e.name = to
		return e
	}
	if len(e.args) == 0 {
		return e
	}
	args := make([]ffExpr, len(e.args))
	for i, arg := range e.args {
		args[i] = renameExprVar(arg, from, to)
	}
	e.args = args
	return e
}

// String returns the expression text; callers put it in single quotes inside the filter graph.
func (e ffExpr) String() string {
	var b strings.Builder
	e.write(&b)
	return b.String()
}

func (e ffExpr) write(b *strings.Builder) {
	switch e.op {
	case "num":
		text := strconv.FormatFloat(e.num, 'f', -1, 64)
		if e.num < 0 {
			text = "(" + text + ")"
		}
		b.WriteString(text)
	case "var":
		b.WriteString(e.name)
	case "+", "-", "*", "/", ";":
		b.WriteByte('(')
		e.args[0].write(b)
		b.WriteString(e.op)
		e.args[1].write(b)
		b.WriteByte(')')
	default:
		b.WriteString(e.op)
		b.WriteByte('(')
		for i, arg := range e.args {
			if i > 0 {
				b.WriteByte(',')
			}
			arg.write(b)
		}
		b.WriteByte(')')
	}
}

// eval computes the expression as libavutil/eval.c does: operands left to right, if() true for any non-zero value
// (NaN included), max and min as `a > b ? a : b` and `a < b ? a : b`, round half away from zero. vars holds the filter
// variables; a missing one is NaN. Registers start at zero.
func (e ffExpr) eval(vars map[string]float64) float64 {
	var registers [10]float64
	return e.evalWith(vars, &registers)
}

func (e ffExpr) evalWith(vars map[string]float64, registers *[10]float64) float64 {
	arg := func(i int) float64 { return e.args[i].evalWith(vars, registers) }
	register := func() int { return min(max(int(arg(0)), 0), len(registers)-1) }
	switch e.op {
	case "num":
		return e.num
	case "var":
		if value, ok := vars[e.name]; ok {
			return value
		}
		return math.NaN()
	case "+", "-", "*", "/", "lt", "gte", "max", "min":
		a, b := arg(0), arg(1)
		switch e.op {
		case "+":
			return a + b
		case "-":
			return a - b
		case "*":
			return a * b
		case "/":
			return a / b
		case "lt":
			return boolFloat(a < b)
		case "gte":
			return boolFloat(a >= b)
		case "max":
			if a > b {
				return a
			}
			return b
		default: // min
			if a < b {
				return a
			}
			return b
		}
	case ";":
		arg(0)
		return arg(1)
	case "if":
		if arg(0) != 0 {
			return arg(1)
		}
		return arg(2)
	case "round":
		return math.Round(arg(0))
	case "floor":
		return math.Floor(arg(0))
	case "ceil":
		return math.Ceil(arg(0))
	case "sqrt":
		return math.Sqrt(arg(0))
	case "clip":
		// libavutil: NaN when an argument is NaN or min > max, else av_clipd. (It evaluates x twice, which changes
		// nothing for the export's side-effect-free curve expressions.)
		x, low, high := arg(0), arg(1), arg(2)
		if math.IsNaN(x) || math.IsNaN(low) || math.IsNaN(high) || low > high {
			return math.NaN()
		}
		if x < low {
			return low
		}
		if x > high {
			return high
		}
		return x
	case "pow":
		return math.Pow(arg(0), arg(1))
	case "hypot":
		return math.Hypot(arg(0), arg(1))
	case "abs":
		return math.Abs(arg(0))
	case "cos":
		return math.Cos(arg(0))
	case "sin":
		return math.Sin(arg(0))
	case "ld":
		return registers[register()]
	case "st":
		index := register()
		registers[index] = arg(1)
		return registers[index]
	}
	return math.NaN()
}

func boolFloat(value bool) float64 {
	if value {
		return 1
	}
	return 0
}

// balancedTree selects leaf(i) of count leaves by comparing value with threshold(i), the lower bound of leaf i, in a
// balanced if() tree, so one evaluation costs about log2(count) comparisons (index risk 11).
func balancedTree(count int, value ffExpr, leaf func(int) ffExpr, threshold func(int) float64) ffExpr {
	var tree func(lo, hi int) ffExpr
	tree = func(lo, hi int) ffExpr {
		if hi-lo == 1 {
			return leaf(lo)
		}
		mid := (lo + hi) / 2
		return exprOp("if", exprOp("lt", value, exprNum(threshold(mid))), tree(lo, mid), tree(mid, hi))
	}
	return tree(0, count)
}

// easeExpr is the easing curve of index §4.18 at progress p (already clamped to 0..1): the polynomials of Ease.
// ease_out and ease_in_out keep their inner term in register 4.
func easeExpr(easing Easing, p ffExpr) ffExpr {
	cube := func(v ffExpr) ffExpr { return exprOp("*", exprOp("*", v, v), v) }
	switch easing {
	case EaseIn:
		return cube(p)
	case EaseOut:
		return exprOp(";", exprSt(4, exprOp("-", exprNum(1), p)), exprOp("-", exprNum(1), cube(exprLd(4))))
	case EaseInOut:
		return exprOp("if", exprOp("lt", p, exprNum(0.5)),
			exprOp("*", exprOp("*", exprOp("*", exprNum(4), p), p), p),
			exprOp(";", exprSt(4, exprOp("-", exprNum(2), exprOp("*", exprNum(2), p))), exprOp("-", exprNum(1), exprOp("/", cube(exprLd(4)), exprNum(2)))))
	case EaseHold:
		return exprOp("if", exprOp("lt", p, exprNum(1)), exprNum(0), exprNum(1))
	}
	return p
}

// curveExpr samples a keyframe list at the source position source (index §4.18). It is a balanced tree over the spans
// between keys; each span eases from its left key's value with the left key's easing. Progress is clamped to 0..1
// (register 3), so the value holds before the first and after the last key. One key is a constant. keys is never
// empty (Validate).
func curveExpr(keys []Keyframe, source ffExpr) ffExpr {
	if len(keys) == 1 {
		return exprNum(keys[0].V)
	}
	span := func(i int) ffExpr {
		from, to := keys[i], keys[i+1]
		progress := exprOp("max", exprNum(0), exprOp("min", exprNum(1),
			exprOp("/", exprOp("-", source, exprNum(float64(from.F))), exprNum(float64(to.F-from.F)))))
		return exprOp(";", exprSt(3, progress),
			exprOp("+", exprNum(from.V), exprOp("*", exprNum(to.V-from.V), easeExpr(from.E, exprLd(3)))))
	}
	return balancedTree(len(keys)-1, source, span, func(i int) float64 { return float64(keys[i].F) })
}

// rampPieceEnd returns S and the speed at the end (t1) of a ramp piece.
func rampPieceEnd(piece rampPiece) (source, speed float64) {
	d := piece.t1 - piece.t0
	return piece.s0 + d*(piece.a+piece.b/2*d), piece.a + piece.b*d
}

// rampSourceExpr is S(u) for clip time u (index §4.18). It is a balanced tree over the pieces; piece i gives
// s0 + d·(a + b/2·d) with d = u − t0 in register 3. Before the first piece the first piece's speed holds, after the
// last piece its end speed.
func rampSourceExpr(pieces []rampPiece, u ffExpr) ffExpr {
	first, last := pieces[0], pieces[len(pieces)-1]
	endSource, endSpeed := rampPieceEnd(last)
	leaf := func(i int) ffExpr {
		switch {
		case i == 0:
			return exprOp("+", exprNum(first.s0), exprOp("*", exprOp("-", u, exprNum(first.t0)), exprNum(first.a)))
		case i == len(pieces)+1:
			return exprOp("+", exprNum(endSource), exprOp("*", exprOp("-", u, exprNum(last.t1)), exprNum(endSpeed)))
		}
		p := pieces[i-1]
		return exprOp(";", exprSt(3, exprOp("-", u, exprNum(p.t0))),
			exprOp("+", exprNum(p.s0), exprOp("*", exprLd(3), exprOp("+", exprNum(p.a), exprOp("*", exprNum(p.b/2), exprLd(3))))))
	}
	threshold := func(i int) float64 {
		if i == len(pieces)+1 {
			return last.t1
		}
		return pieces[i-1].t0
	}
	return balancedTree(len(pieces)+2, u, leaf, threshold)
}

// rampInverseExpr is Sinv(u), the clip time at which the clip has consumed u source frames (index §4.18). It is a
// balanced tree over the pieces' source ranges:
//   - Piece i gives t0 + 2d/(a + sqrt(a² + 2b·d)), with d = u − s0 in register 1. The form is stable, exact for b = 0,
//     and its square root stays positive inside the piece, because speeds are at least 0.25.
//   - Below the first piece the first piece's speed holds, above the last piece its end speed. Source frames before
//     the offset or past the clip therefore map monotonically outside the clip.
func rampInverseExpr(pieces []rampPiece, u ffExpr) ffExpr {
	first, last := pieces[0], pieces[len(pieces)-1]
	endSource, endSpeed := rampPieceEnd(last)
	leaf := func(i int) ffExpr {
		switch {
		case i == 0:
			return exprOp("+", exprNum(first.t0), exprOp("/", exprOp("-", u, exprNum(first.s0)), exprNum(first.a)))
		case i == len(pieces)+1:
			return exprOp("+", exprNum(last.t1), exprOp("/", exprOp("-", u, exprNum(endSource)), exprNum(endSpeed)))
		}
		p := pieces[i-1]
		return exprOp(";", exprSt(1, exprOp("-", u, exprNum(p.s0))),
			exprOp("+", exprNum(p.t0), exprOp("/", exprOp("*", exprNum(2), exprLd(1)),
				exprOp("+", exprNum(p.a), exprOp("sqrt", exprOp("+", exprNum(p.a*p.a), exprOp("*", exprNum(2*p.b), exprLd(1))))))))
	}
	threshold := func(i int) float64 {
		if i == len(pieces)+1 {
			return endSource
		}
		return pieces[i-1].s0
	}
	return balancedTree(len(pieces)+2, u, leaf, threshold)
}

// windowSourceExprVar stores the absolute frame and the source position of a window frame (index §4.11):
//   - Register 0 holds T = at + round(variable·Num/Den), where variable is the filter's time in seconds: t for
//     scale, overlay, rotate and vignette, and T for blend's all_expr.
//   - Register 1 holds offset + clipSource(T − start): linear for a constant speed, otherwise S by the ramp pieces
//     (register 2 then holds the clip time).
//
// max(variable, 0) keeps the evaluation at configuration time (t is NaN in overlay then) on the window's first frame,
// and does not change any real frame.
func windowSourceExprVar(clip Clip, at int, pieces []rampPiece, rate Rate, variable string) ffExpr {
	frame := exprOp("+", exprNum(float64(at)), exprOp("round", exprOp("/",
		exprOp("*", exprOp("max", exprVar(variable), exprNum(0)), exprNum(float64(rate.Num))), exprNum(float64(rate.Den)))))
	clipTime := exprOp("-", exprLd(0), exprNum(float64(clip.Start)))
	consumed := exprOp("*", clipTime, exprNum(ClipSpeed(clip)))
	if len(pieces) > 0 {
		consumed = exprOp(";", exprSt(2, clipTime), rampSourceExpr(pieces, exprLd(2)))
	}
	return exprOp(";", exprSt(0, frame), exprSt(1, exprOp("+", exprNum(float64(clip.Offset)), consumed)))
}

// windowSourceExpr is windowSourceExprVar in t.
func windowSourceExpr(clip Clip, at int, pieces []rampPiece, rate Rate) ffExpr {
	return windowSourceExprVar(clip, at, pieces, rate, "t")
}

// boxExpr holds the per-frame expressions of a keyframed clip's box on the canvas (index §4.11):
//   - w and h follow evenPixels;
//   - cx and cy are the box centre (evenPosition plus half the size, whole pixels);
//   - angle is the rotation in radians.
//
// Each expression starts with windowSourceExpr, so every option of every filter evaluates the same absolute frames.
type boxExpr struct {
	w, h, cx, cy, angle ffExpr
}

// boxExprs builds the box expressions of clip for the window whose first frame is absolute frame at. Paths without
// keyframes are the static fields. The size sits in register 5 (width) or 6 (height) while the centre is computed.
//
// 2e (index §4.24): an effective slide or pop applies after the even clamp, from the absolute frame in register 0:
//   - the size becomes max(2, 2·round(BW·s/2)) about the box centre, with no canvas cap;
//   - the centre moves by 2·round(d·canvas/2); it keeps the clamped size BW (registers 5 and 6);
//   - the terms come from animExprs and work in registers 7 and 8 (4 inside easeExpr).
//
// TestBoxExprsMatchAnimBox evaluates them against animBox. Clips without such an animation get the 2b expressions byte
// for byte.
func boxExprs(clip Clip, at int, pieces []rampPiece, canvasWidth, canvasHeight int, rate Rate) boxExpr {
	source := windowSourceExpr(clip, at, pieces, rate)
	value := func(path string, static float64) ffExpr {
		if keys := clip.Keyframes[path]; len(keys) > 0 {
			return curveExpr(keys, exprLd(1))
		}
		return exprNum(static)
	}
	even := func(v ffExpr, bound int) ffExpr { // round(v·bound / 2)·2, as evenPixels, evenPosition and animBox
		return exprOp("*", exprOp("round", exprOp("/", exprOp("*", v, exprNum(float64(bound))), exprNum(2))), exprNum(2))
	}
	size := func(v ffExpr, bound int) ffExpr { // evenPixels: at least 2, at most bound
		return exprOp("min", exprOp("max", even(v, bound), exprNum(2)), exprNum(float64(bound)))
	}
	centre := func(v ffExpr, register, bound int) ffExpr { // evenPosition in [0, bound − size], plus half the size
		position := exprOp("max", exprOp("min", even(v, bound), exprOp("-", exprNum(float64(bound)), exprLd(register))), exprNum(0))
		return exprOp("+", position, exprOp("/", exprLd(register), exprNum(2)))
	}
	width, height := size(value("width", clip.Width), canvasWidth), size(value("height", clip.Height), canvasHeight)
	w, h := width, height
	cx, cy := centre(value("x", clip.X), 5, canvasWidth), centre(value("y", clip.Y), 6, canvasHeight)
	anim := animExprs(clip, exprLd(0))
	if !exprIsNum(anim.scale, 1) { // animBox: max(2, 2·round(BW·s/2))
		scaled := func(v ffExpr) ffExpr {
			return exprOp("max", exprOp("*", exprOp("round", exprOp("/", exprOp("*", v, anim.scale), exprNum(2))), exprNum(2)), exprNum(2))
		}
		w, h = scaled(width), scaled(height)
	}
	if !exprIsNum(anim.dx, 0) { // animBox: + 2·round(dx·canvasWidth/2)
		cx = exprOp("+", cx, even(anim.dx, canvasWidth))
	}
	if !exprIsNum(anim.dy, 0) {
		cy = exprOp("+", cy, even(anim.dy, canvasHeight))
	}
	return boxExpr{
		w:     exprOp(";", source, w),
		h:     exprOp(";", source, h),
		cx:    exprOp(";", source, exprOp(";", exprSt(5, width), cx)),
		cy:    exprOp(";", source, exprOp(";", exprSt(6, height), cy)),
		angle: exprOp(";", source, exprOp("*", value("rotation", clip.Rotation), exprNum(math.Pi/180))),
	}
}
