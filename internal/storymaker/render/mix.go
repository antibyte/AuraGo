package render

import (
	"bufio"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math"
	"os"
	"sort"
)

const (
	MixRate          = 48000
	mixChannels      = 2
	mixBlockFrames   = 4800
	mixTargetLUFS    = -16.0
	mixCeilingDBTP   = -1.0
	mixStemGuardDBFS = -0.1
	bedBelowNarrLU   = -12.0
	bedNoNarrLUFS    = -18.0
	bedMaxGainDB     = 24.0
	voiceMatchCapDB  = 6.0
	duckDepthDB      = 6.0
	duckAttackMS     = 50.0
	duckReleaseMS    = 400.0
	duckLookaheadMS  = 50.0
	duckThresholdDB  = -45.0
	duckPeakDecayMS  = 20.0
	carveHz          = 2200.0
	carveQ           = 1.0
	carveDB          = -5.0
	carveRampMS      = 20.0
	dipAttackFrames  = 480
	dipHoldFrames    = 3840
	dipReturnFrames  = 14400
	loopXfadeS       = 2.0
	musicFadeInS     = 0.5
	musicFadeOutS    = 1.5
	clipFadeMS       = 5

	clipFadeFrames   = clipFadeMS * MixRate / 1000
	mixBytesPerFrame = mixChannels * 2
	mixFullScale     = 32768.0
	pcmWindowFrames  = 1 << 16
	wavHeaderBytes   = 44
)

// Span is a stretch of speech in film frames, half open.
type Span struct{ From, To int64 }

// Impact is a sound hit that dips the bed.
type Impact struct {
	At      int64
	DepthDB float64
}

// MixClip is one narration piece or one sound effect on the film timeline.
type MixClip struct {
	Key           string
	Path          string  // raw s16le 48 kHz stereo file in staging; "" when PCM is set
	PCM           []int16 // interleaved stereo, SFX read once into memory
	In            int64   // used range start in frames
	Len           int64   // used length in frames
	Start         int64   // film position in frames
	GainDB        float64 // narration: written by the voice match; sfx: empty on input, written in pass 2
	LevelOffsetDB float64 // sfx only: over_db - take.l50_lufs, over_db 0 without a hit
	Fade          int64   // frames faded at both edges when the piece was trimmed
	Voice         string  // provider|voice, narration only
}

// MixBed is the music under the film. Path is a raw s16le 48 kHz stereo file in staging.
type MixBed struct {
	Path   string
	Frames int64
	Volume float64 // above 0; volume 0 never reaches the mixer
}

// MixInput is everything one mix needs.
type MixInput struct {
	Frames    int64 // round(frame_count*48000/fps)
	Narration []MixClip
	SFX       []MixClip
	Bed       *MixBed
	Spans     []Span
	Impacts   []Impact
}

// MixOutputs names the WAV files of a mix. Mix is required; "" skips a stem.
type MixOutputs struct{ Mix, Narration, Music, SFX string }

// MixResult reports what a mix measured and applied. A silent reading is stored as -70, so the
// result stays valid JSON.
type MixResult struct {
	NarrationI, BedI, PreI, PreTP, BedGainDB, MasterGainDB float64
	VoiceGainDB                                            map[string]float64
	StemPeak                                               [3]float64 // narration, music, sfx after the master gain, linear
	Clamps                                                 int64
	BedDropped                                             bool // the bed read silent
}

// mixPass selects what one run over the film computes.
type mixPass struct {
	bedGainDB float64
	masterDB  float64
	rawBed    bool // pass 1: narration and the bare loop at unity, no sfx
}

// mixBlock is one block of a pass: interleaved stems as floats, after all gains.
type mixBlock struct {
	frames                int
	narration, music, sfx []float64
}

func dbToLin(db float64) float64 { return math.Pow(10, db/20) }

// mixIOError keeps the cause of a file error and drops the staging path from its text.
func mixIOError(what string, err error) error {
	var pathErr *fs.PathError
	if errors.As(err, &pathErr) {
		err = pathErr.Err
	}
	return fmt.Errorf("storymaker: %s: %w", what, err)
}

// runMixPass streams the film through the graph and hands each block to sink. It stops on ctx at
// block boundaries. The narration of the next block is kept ready, because the duck looks ahead.
func runMixPass(ctx context.Context, in *MixInput, pass mixPass, sink func(*mixBlock) error) error {
	if in.Frames <= 0 {
		return nil
	}
	size := mixBlockFrames * mixChannels
	cur, next := make([]float64, size), make([]float64, size)
	narration, music, sfx := make([]float64, size), make([]float64, size), make([]float64, size)

	narrClips := newClipMixer(usableClips(in.Narration, in.Frames))
	sfxClips := newClipMixer(usableClips(in.SFX, in.Frames))
	var loop *bedLoop
	if in.Bed != nil {
		var err error
		if loop, err = openBedLoop(in.Bed, in.Frames); err != nil {
			return err
		}
		defer loop.close()
	}
	shape := newBedShape(in, pass)
	master := dbToLin(pass.masterDB)

	if err := narrClips.fill(0, int(min(mixBlockFrames, in.Frames)), next); err != nil {
		return err
	}
	block := &mixBlock{}
	for n0 := int64(0); n0 < in.Frames; n0 += mixBlockFrames {
		if err := ctx.Err(); err != nil {
			return err
		}
		f := int(min(mixBlockFrames, in.Frames-n0))
		samples := f * mixChannels
		cur, next = next, cur
		clear(next)
		if rest := in.Frames - n0 - mixBlockFrames; rest > 0 {
			if err := narrClips.fill(n0+mixBlockFrames, int(min(mixBlockFrames, rest)), next); err != nil {
				return err
			}
		}
		if pass.rawBed {
			clear(sfx[:samples])
		} else if err := sfxClips.fill(n0, f, sfx); err != nil {
			return err
		}
		clear(music[:samples])
		if loop != nil {
			if err := loop.fill(n0, f, music); err != nil {
				return err
			}
			if !pass.rawBed {
				shape.apply(n0, f, cur, next, music)
			}
		}
		for i := 0; i < samples; i++ {
			narration[i] = cur[i] * master
			music[i] *= master
			sfx[i] *= master
		}
		block.frames = f
		block.narration, block.music, block.sfx = narration[:samples], music[:samples], sfx[:samples]
		if err := sink(block); err != nil {
			return err
		}
	}
	return nil
}

// usableClips drops clips that never sound, truncates at the film end and sorts by start. A cut edge
// is a trim, so it gets the clip fade.
func usableClips(clips []MixClip, frames int64) []MixClip {
	out := make([]MixClip, 0, len(clips))
	for _, c := range clips {
		if c.Len <= 0 || c.Start >= frames {
			continue
		}
		if c.Start+c.Len > frames {
			c.Len = frames - c.Start
			if c.Fade < clipFadeFrames {
				c.Fade = clipFadeFrames
			}
		}
		out = append(out, c)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Start < out[j].Start })
	return out
}

// loadClipPCM returns the used range of a clip. FFmpeg may decode a few frames fewer than the plan
// measured, so a short file gives a short clip rather than an error.
func loadClipPCM(c MixClip) ([]int16, error) {
	if c.Path == "" {
		last := (c.In + c.Len) * mixChannels
		if c.In < 0 || last > int64(len(c.PCM)) {
			return nil, fmt.Errorf("storymaker: the clip %s is outside its samples", c.Key)
		}
		return c.PCM[c.In*mixChannels : last], nil
	}
	f, err := os.Open(c.Path)
	if err != nil {
		return nil, mixIOError("open a decoded clip", err)
	}
	defer f.Close()
	raw := make([]byte, c.Len*mixBytesPerFrame)
	n, err := f.ReadAt(raw, c.In*mixBytesPerFrame)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, mixIOError("read a decoded clip", err)
	}
	pcm := make([]int16, n/mixBytesPerFrame*mixChannels)
	for i := range pcm {
		pcm[i] = int16(binary.LittleEndian.Uint16(raw[i*2:]))
	}
	return pcm, nil
}

// clipMixer opens clips when they start and drops them when they end.
type clipMixer struct {
	clips  []MixClip
	next   int
	active []*activeClip
}

type activeClip struct {
	data   []int16
	start  int64
	end    int64
	length int64
	fade   int64
	gain   float64
}

func newClipMixer(clips []MixClip) *clipMixer { return &clipMixer{clips: clips} }

// fill writes the sum of the clips into out for the frames n0 up to n0+f.
func (m *clipMixer) fill(n0 int64, f int, out []float64) error {
	clear(out[:f*mixChannels])
	n1 := n0 + int64(f)
	for m.next < len(m.clips) && m.clips[m.next].Start < n1 {
		c := m.clips[m.next]
		m.next++
		data, err := loadClipPCM(c)
		if err != nil {
			return err
		}
		length := int64(len(data) / mixChannels)
		m.active = append(m.active, &activeClip{
			data:   data,
			start:  c.Start,
			end:    c.Start + length,
			length: length,
			fade:   min(c.Fade, length/2),
			gain:   dbToLin(c.GainDB),
		})
	}
	keep := m.active[:0]
	for _, a := range m.active {
		a.mixInto(n0, n1, out)
		if a.end > n1 {
			keep = append(keep, a)
		}
	}
	clear(m.active[len(keep):])
	m.active = keep
	return nil
}

func (a *activeClip) mixInto(n0, n1 int64, out []float64) {
	for t := max(a.start, n0); t < min(a.end, n1); t++ {
		i := t - a.start
		g := a.gain * clipEdge(i, a.length, a.fade) / mixFullScale
		o := (t - n0) * mixChannels
		out[o] += float64(a.data[i*mixChannels]) * g
		out[o+1] += float64(a.data[i*mixChannels+1]) * g
	}
}

// clipEdge is the linear fade at both edges of a clip: 0 on its first and last frame.
func clipEdge(i, length, fade int64) float64 {
	if fade <= 0 {
		return 1
	}
	if i < fade {
		return float64(i) / float64(fade)
	}
	if tail := length - 1 - i; tail < fade {
		return float64(tail) / float64(fade)
	}
	return 1
}

// pcmWindow reads a raw stereo file through a sequential read-ahead window.
type pcmWindow struct {
	f      *os.File
	frames int64
	base   int64
	count  int64
	raw    []byte
	pcm    []int16
}

func openPCMWindow(path string) (*pcmWindow, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, mixIOError("open the music", err)
	}
	st, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, mixIOError("read the music", err)
	}
	return &pcmWindow{
		f:      f,
		frames: st.Size() / mixBytesPerFrame,
		raw:    make([]byte, pcmWindowFrames*mixBytesPerFrame),
		pcm:    make([]int16, pcmWindowFrames*mixChannels),
	}, nil
}

func (w *pcmWindow) load(frame int64) error {
	count := min(pcmWindowFrames, w.frames-frame)
	if frame < 0 || count <= 0 {
		return errors.New("storymaker: the music ended early")
	}
	raw := w.raw[:count*mixBytesPerFrame]
	if _, err := w.f.ReadAt(raw, frame*mixBytesPerFrame); err != nil {
		return mixIOError("read the music", err)
	}
	for i := int64(0); i < count*mixChannels; i++ {
		w.pcm[i] = int16(binary.LittleEndian.Uint16(raw[i*2:]))
	}
	w.base, w.count = frame, count
	return nil
}

func (w *pcmWindow) at(frame int64) (float64, float64, error) {
	if frame < w.base || frame >= w.base+w.count {
		if err := w.load(frame); err != nil {
			return 0, 0, err
		}
	}
	i := (frame - w.base) * mixChannels
	return float64(w.pcm[i]) / mixFullScale, float64(w.pcm[i+1]) / mixFullScale, nil
}

// bedLoop streams the bare looped bed through two read windows, because one window thrashes in the
// crossfade.
type bedLoop struct {
	main, tail    *pcmWindow
	period, xfade int64
}

// loopShape returns the loop period and the crossfade, both in frames. The crossfade is at most half
// the bed. A bed as long as the film never loops, so it needs no crossfade.
func loopShape(bedFrames, film int64) (period, xfade int64) {
	if film <= bedFrames {
		return bedFrames, 0
	}
	xfade = min(int64(loopXfadeS*MixRate), bedFrames/2)
	return bedFrames - xfade, xfade
}

func openBedLoop(bed *MixBed, film int64) (*bedLoop, error) {
	main, err := openPCMWindow(bed.Path)
	if err != nil {
		return nil, err
	}
	tail, err := openPCMWindow(bed.Path)
	if err != nil {
		_ = main.f.Close()
		return nil, err
	}
	frames := min(bed.Frames, main.frames)
	if frames <= 0 {
		_ = main.f.Close()
		_ = tail.f.Close()
		return nil, errors.New("storymaker: the music file is empty")
	}
	period, xfade := loopShape(frames, film)
	return &bedLoop{main: main, tail: tail, period: period, xfade: xfade}, nil
}

func (b *bedLoop) close() {
	_ = b.main.f.Close()
	_ = b.tail.f.Close()
}

// fill writes the looped bed into out: an equal-power crossfade from the bed's tail to its head.
func (b *bedLoop) fill(n0 int64, f int, out []float64) error {
	for i := 0; i < f; i++ {
		n := n0 + int64(i)
		pos := n % b.period
		l, r, err := b.main.at(pos)
		if err != nil {
			return err
		}
		if n >= b.period && pos < b.xfade {
			tailL, tailR, err := b.tail.at(pos + b.period)
			if err != nil {
				return err
			}
			u := float64(pos) / float64(b.xfade) * math.Pi / 2
			rise, fall := math.Sin(u), math.Cos(u)
			l, r = l*rise+tailL*fall, r*rise+tailR*fall
		}
		out[i*mixChannels], out[i*mixChannels+1] = l, r
	}
	return nil
}

// biquad is a stateful direct-form-I filter.
type biquad struct {
	b0, b1, b2, a1, a2 float64
	x1, x2, y1, y2     float64
}

// peakingBiquad is the RBJ peaking filter at the mix rate.
func peakingBiquad(hz, q, gainDB float64) biquad {
	a := math.Pow(10, gainDB/40)
	w := 2 * math.Pi * hz / MixRate
	alpha := math.Sin(w) / (2 * q)
	a0 := 1 + alpha/a
	return biquad{
		b0: (1 + alpha*a) / a0,
		b1: -2 * math.Cos(w) / a0,
		b2: (1 - alpha*a) / a0,
		a1: -2 * math.Cos(w) / a0,
		a2: (1 - alpha/a) / a0,
	}
}

func (b *biquad) run(x float64) float64 {
	y := b.b0*x + b.b1*b.x1 + b.b2*b.x2 - b.a1*b.y1 - b.a2*b.y2
	b.x2, b.x1, b.y2, b.y1 = b.x1, x, b.y1, y
	return y
}

// dipGainDB is the impact dip in dB, dt frames after the hit.
func dipGainDB(dt int64, depth float64) float64 {
	switch {
	case dt < 0:
		return 0
	case dt < dipAttackFrames:
		return -depth * float64(dt) / dipAttackFrames
	case dt < dipAttackFrames+dipHoldFrames:
		return -depth
	case dt < dipAttackFrames+dipHoldFrames+dipReturnFrames:
		return -depth * (1 - float64(dt-dipAttackFrames-dipHoldFrames)/dipReturnFrames)
	}
	return 0
}

// bedShape turns the raw loop into the bed stem: the gain to the reference and the fades, the duck
// keyed by the narration, the carve inside the speech spans, and the impact dips.
type bedShape struct {
	gain    float64
	film    int64
	fadeIn  int64
	fadeOut int64

	lookahead   int
	attackStep  float64
	releaseStep float64
	peakDecay   float64
	threshold   float64
	peak        float64
	duck        float64

	carve    [mixChannels]biquad
	spans    []Span
	nextSpan int
	rampLead int64
	rampStep float64
	wet      float64

	impacts    []Impact
	nextImpact int
	activeHits []Impact
}

func newBedShape(in *MixInput, pass mixPass) *bedShape {
	spans := append([]Span(nil), in.Spans...)
	sort.Slice(spans, func(i, j int) bool { return spans[i].From < spans[j].From })
	impacts := append([]Impact(nil), in.Impacts...)
	sort.Slice(impacts, func(i, j int) bool { return impacts[i].At < impacts[j].At })
	carve := peakingBiquad(carveHz, carveQ, carveDB)
	return &bedShape{
		gain:        dbToLin(pass.bedGainDB),
		film:        in.Frames,
		fadeIn:      int64(musicFadeInS * MixRate),
		fadeOut:     int64(musicFadeOutS * MixRate),
		lookahead:   int(duckLookaheadMS * MixRate / 1000),
		attackStep:  1 / (duckAttackMS * MixRate / 1000),
		releaseStep: 1 / (duckReleaseMS * MixRate / 1000),
		peakDecay:   math.Exp(-1 / (duckPeakDecayMS * MixRate / 1000)),
		threshold:   dbToLin(duckThresholdDB),
		carve:       [mixChannels]biquad{carve, carve},
		spans:       spans,
		rampLead:    int64(carveRampMS * MixRate / 1000),
		rampStep:    1 / (carveRampMS * MixRate / 1000),
		impacts:     impacts,
	}
}

func (s *bedShape) fade(n int64) float64 {
	fade := 1.0
	if n < s.fadeIn {
		fade = float64(n) / float64(s.fadeIn)
	}
	if left := s.film - n; left < s.fadeOut {
		fade = math.Min(fade, float64(left)/float64(s.fadeOut))
	}
	return fade
}

// apply shapes one block of the raw loop in place. cur and next are the narration of this block and
// the next one, before the master gain, so the duck is the same in pass 2 and pass 3.
func (s *bedShape) apply(n0 int64, f int, cur, next, bed []float64) {
	for i := 0; i < f; i++ {
		n := n0 + int64(i)
		duckDB := s.stepDuck(i, f, cur, next)
		dipDB := s.stepDip(n)
		wet := s.stepCarve(n)
		g := s.gain * s.fade(n) * dbToLin(duckDB+dipDB)
		for ch := 0; ch < mixChannels; ch++ {
			x := bed[i*mixChannels+ch]
			y := s.carve[ch].run(x)
			bed[i*mixChannels+ch] = (x + wet*(y-x)) * g
		}
	}
}

// stepDuck is the duck in dB for one frame. The detector reads the narration 50 ms ahead and holds its
// peak with a 20 ms decay. Attack and release are linear ramps in dB, so the duck starts 50 ms before
// speech, is at full depth when speech starts, and returns over exactly 400 ms.
func (s *bedShape) stepDuck(i, f int, cur, next []float64) float64 {
	j := i + s.lookahead
	var l, r float64
	if j < f {
		l, r = cur[j*mixChannels], cur[j*mixChannels+1]
	} else if j-f < mixBlockFrames {
		l, r = next[(j-f)*mixChannels], next[(j-f)*mixChannels+1]
	}
	s.peak = math.Max(math.Abs(l+r)*0.5, s.peak*s.peakDecay)
	if s.peak > s.threshold {
		s.duck = math.Min(1, s.duck+s.attackStep)
	} else {
		s.duck = math.Max(0, s.duck-s.releaseStep)
	}
	return -duckDepthDB * s.duck
}

// stepCarve is the wet share of the carve for one frame: fully wet inside a span, with a 20 ms ramp
// before it and after it.
func (s *bedShape) stepCarve(n int64) float64 {
	for s.nextSpan < len(s.spans) && s.spans[s.nextSpan].To <= n {
		s.nextSpan++
	}
	if s.nextSpan < len(s.spans) && n >= s.spans[s.nextSpan].From-s.rampLead {
		s.wet = math.Min(1, s.wet+s.rampStep)
	} else {
		s.wet = math.Max(0, s.wet-s.rampStep)
	}
	return s.wet
}

// stepDip is the deepest active impact dip in dB for one frame.
func (s *bedShape) stepDip(n int64) float64 {
	for s.nextImpact < len(s.impacts) && s.impacts[s.nextImpact].At <= n {
		s.activeHits = append(s.activeHits, s.impacts[s.nextImpact])
		s.nextImpact++
	}
	deepest := 0.0
	kept := s.activeHits[:0]
	for _, hit := range s.activeHits {
		deepest = math.Min(deepest, dipGainDB(n-hit.At, hit.DepthDB))
		if n-hit.At < dipAttackFrames+dipHoldFrames+dipReturnFrames {
			kept = append(kept, hit)
		}
	}
	s.activeHits = kept
	return deepest
}

// wavWriter writes a canonical 48 kHz stereo s16 WAV. close writes the header.
type wavWriter struct {
	f      *os.File
	bw     *bufio.Writer
	frames int64
	buf    []byte
}

func newWavWriter(path string) (*wavWriter, error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, mixIOError("create a mix file", err)
	}
	w := &wavWriter{f: f, bw: bufio.NewWriterSize(f, 1<<20), buf: make([]byte, mixBlockFrames*mixBytesPerFrame)}
	if _, err := w.bw.Write(make([]byte, wavHeaderBytes)); err != nil {
		_ = f.Close()
		return nil, mixIOError("write a mix file", err)
	}
	return w, nil
}

func (w *wavWriter) write(samples []int16) error {
	b := w.buf[:len(samples)*2]
	for i, v := range samples {
		binary.LittleEndian.PutUint16(b[i*2:], uint16(v))
	}
	w.frames += int64(len(samples) / mixChannels)
	if _, err := w.bw.Write(b); err != nil {
		return mixIOError("write a mix file", err)
	}
	return nil
}

// close is safe on a nil writer and safe to call twice.
func (w *wavWriter) close() error {
	if w == nil || w.f == nil {
		return nil
	}
	f := w.f
	w.f = nil
	err := w.bw.Flush()
	if err == nil {
		_, err = f.WriteAt(wavHeader(w.frames), 0)
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return mixIOError("finish a mix file", err)
	}
	return nil
}

func wavHeader(frames int64) []byte {
	data := uint32(frames * mixBytesPerFrame)
	h := make([]byte, wavHeaderBytes)
	copy(h[0:], "RIFF")
	binary.LittleEndian.PutUint32(h[4:], 36+data)
	copy(h[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(h[16:], 16)
	binary.LittleEndian.PutUint16(h[20:], 1)
	binary.LittleEndian.PutUint16(h[22:], mixChannels)
	binary.LittleEndian.PutUint32(h[24:], MixRate)
	binary.LittleEndian.PutUint32(h[28:], MixRate*mixBytesPerFrame)
	binary.LittleEndian.PutUint16(h[32:], mixBytesPerFrame)
	binary.LittleEndian.PutUint16(h[34:], 16)
	copy(h[36:], "data")
	binary.LittleEndian.PutUint32(h[40:], data)
	return h
}

func quantiseS16(v float64) (int16, bool) {
	r := math.Round(v * mixFullScale)
	switch {
	case r > math.MaxInt16:
		return math.MaxInt16, true
	case r < math.MinInt16:
		return math.MinInt16, true
	}
	return int16(r), false
}

// stemWriter quantises each stem after the gains and writes the mix as the clamp of their integer
// sum, so the stems sum to the mix exactly.
type stemWriter struct {
	mix    *wavWriter
	stems  [3]*wavWriter
	pcm    [3][]int16
	mixPCM []int16
	peak   [3]float64
	clamps int64
}

func newStemWriter(out MixOutputs) (*stemWriter, error) {
	size := mixBlockFrames * mixChannels
	w := &stemWriter{mixPCM: make([]int16, size)}
	for k := range w.pcm {
		w.pcm[k] = make([]int16, size)
	}
	var err error
	if w.mix, err = newWavWriter(out.Mix); err != nil {
		return nil, err
	}
	for k, path := range [3]string{out.Narration, out.Music, out.SFX} {
		if path == "" {
			continue
		}
		if w.stems[k], err = newWavWriter(path); err != nil {
			_ = w.close()
			return nil, err
		}
	}
	return w, nil
}

func (w *stemWriter) write(b *mixBlock) error {
	samples := b.frames * mixChannels
	for i := 0; i < samples; i++ {
		sum := 0
		for k, v := range [3]float64{b.narration[i], b.music[i], b.sfx[i]} {
			if a := math.Abs(v); a > w.peak[k] {
				w.peak[k] = a
			}
			q, clamped := quantiseS16(v)
			if clamped {
				w.clamps++
			}
			w.pcm[k][i] = q
			sum += int(q)
		}
		if sum > math.MaxInt16 {
			sum = math.MaxInt16
			w.clamps++
		} else if sum < math.MinInt16 {
			sum = math.MinInt16
			w.clamps++
		}
		w.mixPCM[i] = int16(sum)
	}
	if err := w.mix.write(w.mixPCM[:samples]); err != nil {
		return err
	}
	for k, stem := range w.stems {
		if stem == nil {
			continue
		}
		if err := stem.write(w.pcm[k][:samples]); err != nil {
			return err
		}
	}
	return nil
}

// close finishes every file and returns the first error. It is safe to call twice.
func (w *stemWriter) close() error {
	err := w.mix.close()
	for _, stem := range w.stems {
		if closeErr := stem.close(); err == nil {
			err = closeErr
		}
	}
	return err
}
