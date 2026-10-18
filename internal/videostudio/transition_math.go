package videostudio

import "math"

// Shared per-frame transition parameters (plan 2e decision D1, index §4.24).
//
// xfade computes every transition in float32 from its progress P = f(1 − f(i/D)) at overlap frame i of D: 1 at frame
// 0, 1/D at the last frame, never 0. It stores 8-bit values by truncation. transitionFrameAt computes the per-frame
// numbers once, the same way:
//   - Every float32 operation is wrapped in an explicit float32(...) conversion, so the compiler cannot fuse a
//     multiply-add and change a value.
//   - Integer edges are trunc(f(size·P)) or trunc(f(size·Q)). A rational edge would be 1 px off in 10–18 % of
//     (size, D, i) (spike snap audit).
//
// Where the numbers go:
//   - JS VideoStudioPicture.transitionParams computes the same numbers with Math.fround (parity topic xfade, exact).
//   - The GL shaders and both painters read them through VideoStudioPicture.transitionUniforms and do only per-pixel
//     arithmetic. Nothing recomputes an edge or a weight from p = i/D (index risk 30).
//   - The export keeps xfade itself.
//   - The probe fixture (Task 10) and fadegraysModel (Task 3) use these numbers too.

// transitionFrame holds the parameters of one overlap frame. Fields a type does not use stay zero:
//   - P, Q: xfade's progress and f(1 − P), the incoming weight of a dissolve;
//   - Edge: the integer edge z of the four wipes;
//   - Shift: the push offset s;
//   - CX, CY: the centre (W>>1, H>>1) of the circles, radial and rectcrop;
//   - R: f(hypot(CX, CY)) of the circles and radial;
//   - C: c of circleopen and circleclose, or f(P·2) of the smooth and diagonal wipes;
//   - Radius: circlecrop's r;
//   - ZW, ZH: rectcrop's half sizes;
//   - U, V: the fade-through weights ss(0.8, 1, P) and ss(0.2, 1, P);
//   - Sq: the pixelize block;
//   - Zf, W: zoomin's zoom and weight;
//   - K: radial's angle offset f(P − 0.5)·2.5π, in float64 as xfade computes it.
type transitionFrame struct {
	P, Q                          float32
	Edge, Shift, CX, CY, ZW, ZH   int
	R, C, Radius, U, V, Sq, Zf, W float32
	K                             float64
}

// transitionFrameAt returns the parameters of overlap frame i of a D-frame transition of kind on a width × height
// frame (index §4.24). The rules follow the xfade name of the registry row: the user's wipeleft is xfade wiperight, the
// user's wiperight is xfade wipeleft. D below 1 counts as 1, i is clamped into 0..D−1 and the sizes are at least 1, as
// in JS. A kind outside the registry gets P and Q only.
func transitionFrameAt(kind TransitionType, i, D, width, height int) transitionFrame {
	D = max(1, D)
	i = min(D-1, max(0, i))
	width, height = max(1, width), max(1, height)
	P := float32(1 - float32(float32(i)/float32(D)))
	Q := float32(1 - P)
	frame := transitionFrame{P: P, Q: Q}
	spec, ok := TransitionByID(kind)
	if !ok {
		return frame
	}
	w, h := float32(width), float32(height)
	cx, cy := width>>1, height>>1
	// hypotf(w/2, h/2) of the integer half sizes: the integer sum is exact, so sqrt rounds once in float64 and once to
	// float32 (JS: Math.fround(Math.sqrt(cx·cx + cy·cy))).
	R := float32(math.Sqrt(float64(cx*cx + cy*cy)))
	switch spec.XFade {
	case "fadeblack", "fadewhite", "fadegrays":
		frame.U, frame.V = smoothstep32(0.8, 1, P), smoothstep32(0.2, 1, P)
	case "wipeleft":
		frame.Edge = int(float32(w * P))
	case "wiperight":
		frame.Edge = int(float32(w * Q))
	case "wipeup":
		frame.Edge = int(float32(h * P))
	case "wipedown":
		frame.Edge = int(float32(h * Q))
	case "slideleft", "slideright":
		frame.Shift = int(float32(P * w))
	case "slideup", "slidedown":
		frame.Shift = int(float32(P * h))
	case "circleopen":
		frame.CX, frame.CY, frame.R, frame.C = cx, cy, R, float32(float32(P-0.5)*3)
	case "circleclose":
		frame.CX, frame.CY, frame.R, frame.C = cx, cy, R, float32(float32(0.5-P)*3)
	case "circlecrop":
		// r = f(f(t³)·R), t = f(2·|f(P − 0.5)|). t·t is exact in float64, so the cube rounds once, then to float32 (JS:
		// Math.fround(t * t * t)).
		t := float64(float32(2 * float32(math.Abs(float64(float32(P-0.5))))))
		frame.CX, frame.CY, frame.R = cx, cy, R
		frame.Radius = float32(float32(t*t*t) * R)
	case "rectcrop":
		e := float32(math.Abs(float64(float32(P - 0.5))))
		frame.CX, frame.CY = cx, cy
		frame.ZW, frame.ZH = int(float32(e*w)), int(float32(e*h))
	case "radial":
		// k is float64, like xfade's (progress − 0.5f)·(M_PI·2.5f). float64(math.Pi)·2.5 is exactly representable, so
		// this is the double of JS Math.PI * 2.5. The untyped math.Pi·2.5 would round to the same double; the explicit
		// conversion states it.
		frame.CX, frame.CY, frame.R = cx, cy, R
		frame.K = float64(float32(P-0.5)) * (float64(math.Pi) * 2.5)
	case "smoothleft", "smoothright", "diagtl", "diagbr":
		frame.C = float32(P * 2)
	case "pixelize":
		d := min(P, Q)
		dist := float32(float32(math.Ceil(float64(float32(d*50)))) / 50)
		frame.Sq = float32(float32(float32(2*dist)*float32(min(width, height))) / 20)
	case "zoomin":
		frame.Zf, frame.W = smoothstep32(0.5, 1, P), smoothstep32(0, 0.5, P)
	}
	return frame
}

// smoothstep32 is xfade's smoothstep in float32: t = clip(f(f(x − e0)/f(e1 − e0)), 0, 1), then f(f(t·t)·f(3 − f(2t))).
// The edges are float32 constants, like the C code's 0.2f and 0.8f (JS passes them through Math.fround).
func smoothstep32(edge0, edge1, x float32) float32 {
	t := float32(float32(x-edge0) / float32(edge1-edge0))
	if t < 0 {
		t = 0
	} else if t > 1 {
		t = 1
	}
	return float32(float32(t*t) * float32(3-float32(2*t)))
}

// mix32 is xfade's mix(a, b, m) = a·m + b·(1 − m) in float32; m weighs the first input.
func mix32(a, b, m float32) float32 {
	return float32(float32(a*m) + float32(b*float32(1-m)))
}
