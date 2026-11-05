package main

import (
	"math"
	"sort"
)

const blockFrames = 4800

type Clip struct {
	Off, Len int64 // optional sub-range of the file in frames
	Start int64
	Path  string
	Gain  float32
}

type Impact struct {
	At      int64
	DepthDB float64
}

type Span struct{ A, B int64 }

type Plan struct {
	N       int64
	Narr    []Clip
	Sfx     []Clip
	BedPath string
	Impacts []Impact
	Spans   []Span
}

type Params struct {
	BedGainDB     float64
	BedRaw        bool // loop only, unity, no fades/duck/carve/dips
	DuckDepthDB   float64
	DuckAttackMs  float64
	DuckReleaseMs float64
	LookaheadMs   float64
	CarveDB       float64 // negative cut
	CarveRampMs   float64
	Dips          bool
	FadeInS       float64
	FadeOutS      float64
	LoopXfadeS    float64
	MasterDB      float64
	Tee           func(mixF []float32) error // pass-2 streaming of the float mix
}

type Outputs struct {
	Narr, Bed, Sfx, Mix string
	FloatMix            bool
}

type Stats struct {
	Blocks                     int
	PeakNarr, PeakBed, PeakSfx float64 // after master gain, linear
	PeakMix                    float64
	StemClamps, MixClamps      int64
	NullMismatch               int64 // samples where clamp(sum of stems) != sum of stems
	SpeechFrames               int64
	SpeechNarrSq, SpeechBedSq  float64
	MaxActiveClips             int
	MeanDuckDBWhenSpeech       float64
	duckDBSum                  float64
}

type active struct {
	c    Clip
	data []int16
	end  int64
}

type clipMixer struct {
	list   []Clip
	next   int
	act    []*active
	maxAct int
}

func newClipMixer(c []Clip) *clipMixer {
	l := append([]Clip(nil), c...)
	sort.Slice(l, func(i, j int) bool { return l[i].Start < l[j].Start })
	return &clipMixer{list: l}
}

// fill writes the sum of the clips into out (interleaved stereo float) for frames [n0, n0+f).
func (m *clipMixer) fill(n0 int64, f int, out []float32) error {
	for i := range out[:f*2] {
		out[i] = 0
	}
	n1 := n0 + int64(f)
	for m.next < len(m.list) && m.list[m.next].Start < n1 {
		c := m.list[m.next]
		m.next++
		d, err := cachedS16(c.Path)
		if err != nil {
			return err
		}
		if c.Len > 0 {
			d = d[c.Off*2 : (c.Off+c.Len)*2]
		}
		m.act = append(m.act, &active{c: c, data: d, end: c.Start + int64(len(d)/2)})
	}
	if len(m.act) > m.maxAct {
		m.maxAct = len(m.act)
	}
	keep := m.act[:0]
	for _, a := range m.act {
		lo, hi := a.c.Start, a.end
		if lo < n0 {
			lo = n0
		}
		if hi > n1 {
			hi = n1
		}
		g := a.c.Gain / 32768
		for t := lo; t < hi; t++ {
			si := (t - a.c.Start) * 2
			oi := (t - n0) * 2
			out[oi] += float32(a.data[si]) * g
			out[oi+1] += float32(a.data[si+1]) * g
		}
		if a.end > n1 {
			keep = append(keep, a)
		}
	}
	m.act = keep
	return nil
}

func dbToLin(db float64) float64 { return math.Pow(10, db/20) }

// peaking EQ (RBJ), direct form I
type biquad struct {
	b0, b1, b2, a1, a2 float64
	x1, x2, y1, y2     float64
}

func peaking(f0, q, gainDB float64) *biquad {
	A := math.Pow(10, gainDB/40)
	w := 2 * math.Pi * f0 / rate
	al := math.Sin(w) / (2 * q)
	a0 := 1 + al/A
	return &biquad{
		b0: (1 + al*A) / a0, b1: -2 * math.Cos(w) / a0, b2: (1 - al*A) / a0,
		a1: -2 * math.Cos(w) / a0, a2: (1 - al/A) / a0,
	}
}

func (b *biquad) run(x float64) float64 {
	y := b.b0*x + b.b1*b.x1 + b.b2*b.x2 - b.a1*b.y1 - b.a2*b.y2
	b.x2, b.x1, b.y2, b.y1 = b.x1, x, b.y1, y
	return y
}

func clampS16(v float64) (int16, bool) {
	r := math.Round(v * 32768)
	if r > 32767 {
		return 32767, true
	}
	if r < -32768 {
		return -32768, true
	}
	return int16(r), false
}

// dip curve in dB (negative) at dt samples after the hit.
func dipDB(dt int64, depth float64) float64 {
	const atk, hold, ret = 480, 3840, 14400
	switch {
	case dt < 0:
		return 0
	case dt < atk:
		return -depth * float64(dt) / atk
	case dt < atk+hold:
		return -depth
	case dt < atk+hold+ret:
		return -depth * (1 - float64(dt-atk-hold)/ret)
	}
	return 0
}

func runMix(plan *Plan, p Params, out Outputs) (Stats, error) {
	var st Stats
	src, err := openSrc(plan.BedPath)
	if err != nil {
		return st, err
	}
	defer src.f.Close()
	src2, err := openSrc(plan.BedPath)
	if err != nil {
		return st, err
	}
	defer src2.f.Close()
	L := src.info.frames
	X := int64(p.LoopXfadeS * rate)
	if X > L/3 {
		X = L / 3
	}
	P := L - X
	if P < blockFrames {
		P = L
		X = 0
	}

	nm := newClipMixer(plan.Narr)
	sm := newClipMixer(plan.Sfx)
	var wn, wb, ws, wm *wavWriter
	if out.Narr != "" {
		wn, err = newWav(out.Narr, false)
		if err != nil {
			return st, err
		}
	}
	if out.Bed != "" {
		wb, err = newWav(out.Bed, false)
		if err != nil {
			return st, err
		}
	}
	if out.Sfx != "" {
		ws, err = newWav(out.Sfx, false)
		if err != nil {
			return st, err
		}
	}
	if out.Mix != "" {
		wm, err = newWav(out.Mix, out.FloatMix)
		if err != nil {
			return st, err
		}
	}

	curN := make([]float32, blockFrames*2)
	nextN := make([]float32, blockFrames*2)
	sfx := make([]float32, blockFrames*2)
	bed := make([]float64, blockFrames*2)
	q16 := [3][]int16{make([]int16, blockFrames*2), make([]int16, blockFrames*2), make([]int16, blockFrames*2)}
	mix16 := make([]int16, blockFrames*2)
	mixF := make([]float32, blockFrames*2)

	if err := nm.fill(0, minInt(blockFrames, int(plan.N)), nextN); err != nil {
		return st, err
	}

	bedLin := dbToLin(p.BedGainDB)
	master := dbToLin(p.MasterDB)
	atk := 1 - math.Exp(-1/(p.DuckAttackMs/1000*rate))
	rel := 1 - math.Exp(-1/(p.DuckReleaseMs/1000*rate))
	pkDecay := math.Exp(-1 / (0.020 * rate))
	const thr = 0.0056 // -45 dBFS
	look := int(p.LookaheadMs / 1000 * rate)
	carve := [2]*biquad{peaking(2200, 1.0, p.CarveDB), peaking(2200, 1.0, p.CarveDB)}
	ramp := p.CarveRampMs / 1000 * rate
	var pk, s, w float64
	spanIdx := 0
	spans := plan.Spans
	imp := append([]Impact(nil), plan.Impacts...)
	sort.Slice(imp, func(i, j int) bool { return imp[i].At < imp[j].At })
	impNext := 0
	var impAct []Impact
	fadeIn := int64(p.FadeInS * rate)
	fadeOut := int64(p.FadeOutS * rate)

	for n0 := int64(0); n0 < plan.N; n0 += blockFrames {
		F := int(minInt64(blockFrames, plan.N-n0))
		st.Blocks++
		curN, nextN = nextN, curN
		for i := range nextN {
			nextN[i] = 0
		}
		if n0+blockFrames < plan.N {
			if err := nm.fill(n0+blockFrames, int(minInt64(blockFrames, plan.N-n0-blockFrames)), nextN); err != nil {
				return st, err
			}
		}
		if err := sm.fill(n0, F, sfx); err != nil {
			return st, err
		}

		// raw bed loop
		for i := 0; i < F; i++ {
			n := n0 + int64(i)
			k := n / P
			pos := n % P
			l, r := src.at(pos)
			wMain := 1.0
			if k >= 1 && pos < X {
				u := float64(pos) / float64(X)
				wMain = math.Sin(u * math.Pi / 2)
				l2, r2 := src2.at(pos + P)
				wo := math.Cos(u * math.Pi / 2)
				bed[i*2] = float64(l)*wMain + float64(l2)*wo
				bed[i*2+1] = float64(r)*wMain + float64(r2)*wo
			} else {
				bed[i*2] = float64(l)
				bed[i*2+1] = float64(r)
			}
		}

		if !p.BedRaw {
			for i := 0; i < F; i++ {
				n := n0 + int64(i)
				// detector looks ahead
				j := i + look
				var dl, dr float32
				if j < F {
					dl, dr = curN[j*2], curN[j*2+1]
				} else if j-F < blockFrames {
					dl, dr = nextN[(j-F)*2], nextN[(j-F)*2+1]
				}
				a := math.Abs(float64(dl+dr) * 0.5)
				pk = math.Max(a, pk*pkDecay)
				ind := 0.0
				if pk > thr {
					ind = 1
				}
				c := rel
				if ind > s {
					c = atk
				}
				s += (ind - s) * c
				duck := -p.DuckDepthDB * s

				// impact dips
				for impNext < len(imp) && imp[impNext].At <= n {
					impAct = append(impAct, imp[impNext])
					impNext++
				}
				dip := 0.0
				if p.Dips && len(impAct) > 0 {
					kept := impAct[:0]
					for _, h := range impAct {
						d := dipDB(n-h.At, h.DepthDB)
						if d < dip {
							dip = d
						}
						if n-h.At < 480+3840+14400 {
							kept = append(kept, h)
						}
					}
					impAct = kept
				}

				// carve gate
				for spanIdx < len(spans) && spans[spanIdx].B+int64(ramp) < n {
					spanIdx++
				}
				target := 0.0
				if spanIdx < len(spans) && n >= spans[spanIdx].A-int64(ramp) && n <= spans[spanIdx].B {
					target = 1
				}
				if ramp > 0 {
					if w < target {
						w = math.Min(target, w+1/ramp)
					} else if w > target {
						w = math.Max(target, w-1/ramp)
					}
				} else {
					w = target
				}

				fade := 1.0
				if fadeIn > 0 && n < fadeIn {
					fade = float64(n) / float64(fadeIn)
				}
				if fadeOut > 0 && n > plan.N-fadeOut {
					f2 := float64(plan.N-n) / float64(fadeOut)
					if f2 < fade {
						fade = f2
					}
				}
				g := bedLin * fade * dbToLin(duck+dip)
				for ch := 0; ch < 2; ch++ {
					x := bed[i*2+ch]
					y := carve[ch].run(x)
					x += w * (y - x)
					bed[i*2+ch] = x * g
				}
				if s > 0.5 {
					st.SpeechFrames++
					st.duckDBSum += duck
					l, r := float64(curN[i*2]), float64(curN[i*2+1])
					st.SpeechNarrSq += l*l + r*r
					st.SpeechBedSq += bed[i*2]*bed[i*2] + bed[i*2+1]*bed[i*2+1]
				}
			}
		} else {
			for i := 0; i < F*2; i++ {
				bed[i] *= 1
			}
		}

		// master gain, quantise, mix
		for i := 0; i < F*2; i++ {
			vals := [3]float64{float64(curN[i]) * master, bed[i] * master, float64(sfx[i]) * master}
			if p.BedRaw {
				vals[0] = float64(curN[i]) * master
			}
			var sum int
			var fsum float32
			for k := 0; k < 3; k++ {
				v, clamped := clampS16(vals[k])
				if clamped {
					st.StemClamps++
				}
				q16[k][i] = v
				sum += int(v)
				a := math.Abs(vals[k])
				switch k {
				case 0:
					st.PeakNarr = math.Max(st.PeakNarr, a)
				case 1:
					st.PeakBed = math.Max(st.PeakBed, a)
				case 2:
					st.PeakSfx = math.Max(st.PeakSfx, a)
				}
			}
			fsum = float32(vals[0]) + float32(vals[1]) + float32(vals[2])
			mixF[i] = fsum
			st.PeakMix = math.Max(st.PeakMix, math.Abs(float64(fsum)))
			m := sum
			if m > 32767 {
				m = 32767
				st.MixClamps++
			} else if m < -32768 {
				m = -32768
				st.MixClamps++
			}
			if m != sum {
				st.NullMismatch++
			}
			mix16[i] = int16(m)
		}
		if wn != nil {
			if err := wn.writeS16(q16[0][:F*2]); err != nil {
				return st, err
			}
		}
		if wb != nil {
			if err := wb.writeS16(q16[1][:F*2]); err != nil {
				return st, err
			}
		}
		if ws != nil {
			if err := ws.writeS16(q16[2][:F*2]); err != nil {
				return st, err
			}
		}
		if p.Tee != nil {
			if err := p.Tee(mixF[:F*2]); err != nil {
				return st, err
			}
		}
		if wm != nil {
			if out.FloatMix {
				err = wm.writeF32(mixF[:F*2])
			} else {
				err = wm.writeS16(mix16[:F*2])
			}
			if err != nil {
				return st, err
			}
		}
	}
	for _, w := range []*wavWriter{wn, wb, ws, wm} {
		if w != nil {
			if err := w.Close(); err != nil {
				return st, err
			}
		}
	}
	st.MaxActiveClips = nm.maxAct + sm.maxAct
	if st.SpeechFrames > 0 {
		st.MeanDuckDBWhenSpeech = st.duckDBSum / float64(st.SpeechFrames)
	}
	return st, nil
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
func minInt64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}
