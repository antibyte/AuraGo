package videostudio

import (
	"fmt"
	"math"
	"sort"
)

// Speed ramp limits (spec §5.5, §5.8, plan index §4.18).
const (
	MaxRampPoints = 16
	// MaxRampPieces bounds the linear-speed pieces of one ramp inside a clip: 15 eased spans of four sub-pieces plus the
	// constant pieces before the first and after the last point make 62.
	MaxRampPieces = 64
)

// RampPoint is one point of a clip's speed ramp. F is clip time (timeline frames from the clip start; points outside
// the clip are kept, so trims never change the curve inside it), Speed the playback speed there, and E the easing
// towards the next point.
type RampPoint struct {
	F     int     `json:"f"`
	Speed float64 `json:"speed"`
	E     Easing  `json:"e"`
}

// validateRamp checks a ramp's shape (index §4.18). A ramp is allowed only on a video-track clip with a video asset.
// It has 2..MaxRampPoints points with strictly increasing integer frames within ±maxFrames, speeds MinClipSpeed..
// MaxClipSpeed and known easings, and Clip.Speed absent or 1. validateClip calls it before the source extent rule
// reads the ramp.
func validateRamp(track Track, clip Clip, asset Asset, maxFrames int) error {
	if len(clip.Ramp) == 0 {
		return nil
	}
	if track.Kind != TrackVideo || asset.Kind != AssetVideo {
		return fmt.Errorf("clip %q can have a speed ramp only on a video track with a video asset", clip.ID)
	}
	if len(clip.Ramp) < 2 || len(clip.Ramp) > MaxRampPoints {
		return fmt.Errorf("clip %q speed ramp needs 2 to %d points", clip.ID, MaxRampPoints)
	}
	if clip.Speed != 0 && clip.Speed != 1 {
		return fmt.Errorf("clip %q cannot combine a speed ramp with speed %v", clip.ID, clip.Speed)
	}
	for i, point := range clip.Ramp {
		if i > 0 && point.F <= clip.Ramp[i-1].F {
			return fmt.Errorf("clip %q speed ramp points must have strictly increasing frames", clip.ID)
		}
		if point.F < -maxFrames || point.F > maxFrames {
			return fmt.Errorf("clip %q speed ramp point frame %d is out of range", clip.ID, point.F)
		}
		if !finite(point.Speed) || point.Speed < MinClipSpeed || point.Speed > MaxClipSpeed {
			return fmt.Errorf("clip %q ramp speed %v must be between 0.25 and 16", clip.ID, point.Speed)
		}
		if !validEasing(point.E) {
			return fmt.Errorf("clip %q has an unsupported ramp easing %q", clip.ID, point.E)
		}
	}
	return nil
}

// rampEaseSteps is the number of uniform linear-speed sub-pieces of one eased ramp span (index §4.18).
const rampEaseSteps = 4

// rampPiece is one span of a speed ramp with linearly changing speed. For clip time t in [t0, t1] the speed is
// a + b·(t − t0). s0 = S(t0), the source frames consumed from clip time 0 to t0 (S(0) = 0).
type rampPiece struct {
	t0, t1 float64 // clip time span
	s0     float64 // S(t0)
	a, b   float64 // speed at t0 and its slope
}

// rampPieces approximates a ramp's speed curve by linear pieces over clip time [from, to] (index §4.18). JS
// VideoStudioEffects.rampPieces builds the same numbers.
//   - A linear span is one piece, and a hold span one constant piece.
//   - An eased span is rampEaseSteps uniform sub-pieces whose node speeds come from Ease.
//   - The speed is constant before the first and after the last point.
//   - Nodes depend only on their span, so trims and splits that shift the points keep the curve exactly.
//   - S is measured from clip time 0 even when from > 0.
//
// It returns nil without points or when to <= from.
func rampPieces(points []RampPoint, from, to float64) []rampPiece {
	if len(points) == 0 || !(to > from) {
		return nil
	}
	lo, hi := math.Min(from, 0), math.Max(to, 0)
	var raw []rampPiece
	// add appends the part inside [lo, hi] of a segment whose speed runs linearly from v0 at u0 to v1 at u1.
	add := func(u0, u1, v0, v1 float64) {
		c0, c1 := math.Max(u0, lo), math.Min(u1, hi)
		if !(c1 > c0) {
			return
		}
		piece := rampPiece{t0: c0, t1: c1, a: v0}
		if v1 != v0 {
			piece.b = (v1 - v0) / (u1 - u0)
			piece.a = v0 + piece.b*(c0-u0)
		}
		raw = append(raw, piece)
	}
	first, last := points[0], points[len(points)-1]
	add(math.Inf(-1), float64(first.F), first.Speed, first.Speed)
	for i := 0; i+1 < len(points); i++ {
		p, q := points[i], points[i+1]
		u0, u1 := float64(p.F), float64(q.F)
		switch p.E {
		case EaseHold:
			add(u0, u1, p.Speed, p.Speed)
		case EaseIn, EaseOut, EaseInOut:
			for k := 0; k < rampEaseSteps; k++ {
				w0 := u0 + (u1-u0)*float64(k)/rampEaseSteps
				w1 := u0 + (u1-u0)*float64(k+1)/rampEaseSteps
				v0 := p.Speed + (q.Speed-p.Speed)*Ease(p.E, float64(k)/rampEaseSteps)
				v1 := p.Speed + (q.Speed-p.Speed)*Ease(p.E, float64(k+1)/rampEaseSteps)
				add(w0, w1, v0, v1)
			}
		default:
			add(u0, u1, p.Speed, q.Speed)
		}
	}
	add(float64(last.F), math.Inf(1), last.Speed, last.Speed)
	// S from lo; 0 lies in [lo, hi], so one piece holds clip time 0 and S is shifted to S(0) = 0.
	s, zero, found := 0.0, 0.0, false
	for i := range raw {
		p := &raw[i]
		p.s0 = s
		if !found && p.t0 <= 0 && 0 <= p.t1 {
			lead := -p.t0
			zero = s + lead*(p.a+p.b*lead/2)
			found = true
		}
		d := p.t1 - p.t0
		s += d * (p.a + p.b*d/2)
	}
	pieces := make([]rampPiece, 0, len(raw))
	for _, p := range raw {
		c0, c1 := math.Max(p.t0, from), math.Min(p.t1, to)
		if !(c1 > c0) {
			continue
		}
		p.s0 -= zero
		if d := c0 - p.t0; d > 0 {
			p.s0 += d * (p.a + p.b*d/2)
			p.a += p.b * d
			p.t0 = c0
		}
		p.t1 = c1
		pieces = append(pieces, p)
	}
	return pieces
}

// rampPieceAt is the index of the piece holding clip time t: the last piece that starts at or before t, or the first.
func rampPieceAt(pieces []rampPiece, t float64) int {
	i := sort.Search(len(pieces), func(i int) bool { return pieces[i].t0 > t }) - 1
	return max(i, 0)
}

// rampSource is S(t), the source frames consumed from clip time 0 to t. Outside the pieces the first piece's start
// speed and the last piece's end speed continue. Without pieces the clip plays at speed 1.
func rampSource(pieces []rampPiece, t float64) float64 {
	if len(pieces) == 0 {
		return t
	}
	p := pieces[rampPieceAt(pieces, t)]
	if t < p.t0 {
		return p.s0 - p.a*(p.t0-t)
	}
	if t > p.t1 {
		d := p.t1 - p.t0
		return p.s0 + d*(p.a+p.b*d/2) + (p.a+p.b*d)*(t-p.t1)
	}
	d := t - p.t0
	return p.s0 + d*(p.a+p.b*d/2)
}

// rampInverse is the clip time t with S(t) = s. Inside a piece it is t = t0 + 2(s − s0)/(a + sqrt(a² + 2b(s − s0))),
// which is stable and also holds for b = 0. Outside the pieces the edge speeds continue.
func rampInverse(pieces []rampPiece, s float64) float64 {
	if len(pieces) == 0 {
		return s
	}
	i := sort.Search(len(pieces), func(i int) bool { return pieces[i].s0 > s }) - 1
	if i < 0 {
		p := pieces[0]
		return p.t0 - (p.s0-s)/p.a
	}
	p := pieces[i]
	d := p.t1 - p.t0
	if end := p.s0 + d*(p.a+p.b*d/2); i == len(pieces)-1 && s > end {
		return p.t1 + (s-end)/(p.a+p.b*d)
	}
	ds := s - p.s0
	return p.t0 + 2*ds/(p.a+math.Sqrt(p.a*p.a+2*p.b*ds))
}

// rampSpeed is the speed at clip time t (the edge speeds outside the pieces); 1 without pieces.
func rampSpeed(pieces []rampPiece, t float64) float64 {
	if len(pieces) == 0 {
		return 1
	}
	p := pieces[rampPieceAt(pieces, t)]
	return p.a + p.b*math.Max(0, math.Min(t, p.t1)-p.t0)
}

// clipRampPieces returns the pieces of a clip's ramp over the clip and every point, ending one frame past the last
// point, so S is exact at any clip time. Inside [0, duration] they agree with rampPieces(clip.Ramp, 0, duration). It
// returns nil without a ramp.
func clipRampPieces(clip Clip) []rampPiece {
	if len(clip.Ramp) == 0 {
		return nil
	}
	first, last := clip.Ramp[0], clip.Ramp[len(clip.Ramp)-1]
	return rampPieces(clip.Ramp, math.Min(0, float64(first.F)), math.Max(float64(clip.Duration), float64(last.F+1)))
}

// clipSource is the number of source frames a clip consumes from its first frame to clip time t (fractional, any
// sign): S(t) with a ramp, t·speed without. JS: VideoStudioEffects.clipSource.
func clipSource(clip Clip, t float64) float64 {
	if pieces := clipRampPieces(clip); len(pieces) > 0 {
		return rampSource(pieces, t)
	}
	return t * ClipSpeed(clip)
}

// ClipSourceAt is the source position (project frames, fractional) a clip shows at timeline frame frame:
// offset + clipSource(frame − start). Keyframes are sampled there (spec §5.1, index §4.18). JS:
// VideoStudioEffects.sourceAt.
func ClipSourceAt(clip Clip, frame float64) float64 {
	return float64(clip.Offset) + clipSource(clip, frame-float64(clip.Start))
}

// clipSourceExtent is S(duration − 1): the source frames after Offset that the clip's last frame reaches. The source
// extent rule is offset + clipSourceExtent <= duration_frames − 1. JS: VideoStudioEffects.sourceExtent.
func clipSourceExtent(clip Clip) float64 {
	return clipSource(clip, float64(clip.Duration-1))
}

// rampResample samples a ramped video clip's source on the project frame grid (index §4.18). It works like
// videoResample does for a constant speed:
//   - The first decoded frame moves back to from when the stream starts late.
//   - Every source frame m moves to the clip frame offset + ceil(Sinv(m − offset)) where it first shows. This uses
//     videoResample's 1 ms tolerance and works in microseconds.
//   - fps fills the grid.
//
// Output frame n then shows floor(offset + S(n)): the largest source frame the clip has reached. The continuous
// inverse alone rounds to the nearest frame and shows floor(S(n)) + 0/1/2 on FFmpeg 5.1 and 8.1 (spike 2026-10-16).
// Sinv is rampInverseExpr's balanced if() tree on register 0.
func rampResample(from float64, offset int, pieces []rampPiece, rate Rate) string {
	return fmt.Sprintf("setpts='if(eq(N,0),min(PTS,floor(%.9f/TB)),PTS)',settb=AVTB,setpts='st(0,(PTS*TB-0.001)*%d/%d-%d);(%d+ceil(%s))*%d/%d/TB',fps=%s",
		rate.SecondsFloat(from), rate.Num, rate.Den, offset, offset, rampInverseExpr(pieces, exprLd(0)).String(), rate.Den, rate.Num, rate)
}
