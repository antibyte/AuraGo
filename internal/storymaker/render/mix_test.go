package render

import (
	"context"
	"encoding/binary"
	"errors"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func mixTestSine(frames int, amp, hz float64) []int16 {
	pcm := make([]int16, frames*mixChannels)
	for i := 0; i < frames; i++ {
		v := int16(math.Round(amp * 32767 * math.Sin(2*math.Pi*hz*float64(i)/MixRate)))
		pcm[i*2], pcm[i*2+1] = v, v
	}
	return pcm
}

func mixTestConst(frames int, v int16) []int16 {
	pcm := make([]int16, frames*mixChannels)
	for i := range pcm {
		pcm[i] = v
	}
	return pcm
}

func mixTestWriteRaw(t *testing.T, path string, pcm []int16) {
	t.Helper()
	raw := make([]byte, len(pcm)*2)
	for i, v := range pcm {
		binary.LittleEndian.PutUint16(raw[i*2:], uint16(v))
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
}

func mixTestReadWAV(t *testing.T, path string) []int16 {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) < wavHeaderBytes || string(raw[:4]) != "RIFF" || string(raw[36:40]) != "data" {
		t.Fatalf("%s is not a canonical WAV", path)
	}
	data := raw[wavHeaderBytes:]
	if got := binary.LittleEndian.Uint32(raw[40:]); int(got) != len(data) {
		t.Fatalf("%s: the header says %d data bytes, the file has %d", path, got, len(data))
	}
	pcm := make([]int16, len(data)/2)
	for i := range pcm {
		pcm[i] = int16(binary.LittleEndian.Uint16(data[i*2:]))
	}
	return pcm
}

func mixTestCollect(t *testing.T, in *MixInput, pass mixPass) (narration, music, sfx []float64) {
	t.Helper()
	err := runMixPass(context.Background(), in, pass, func(b *mixBlock) error {
		narration = append(narration, b.narration...)
		music = append(music, b.music...)
		sfx = append(sfx, b.sfx...)
		return nil
	})
	if err != nil {
		t.Fatalf("runMixPass: %v", err)
	}
	return narration, music, sfx
}

func mixTestAssertStemsSum(t *testing.T, out MixOutputs, frames int64) {
	t.Helper()
	mix := mixTestReadWAV(t, out.Mix)
	narration := mixTestReadWAV(t, out.Narration)
	music := mixTestReadWAV(t, out.Music)
	sfx := mixTestReadWAV(t, out.SFX)
	if int64(len(mix)) != frames*mixChannels {
		t.Fatalf("mix has %d samples, want %d", len(mix), frames*mixChannels)
	}
	if len(narration) != len(mix) || len(music) != len(mix) || len(sfx) != len(mix) {
		t.Fatalf("stem lengths %d, %d, %d differ from the mix %d", len(narration), len(music), len(sfx), len(mix))
	}
	var sounding [3]int
	for i := range mix {
		if int(mix[i]) != int(narration[i])+int(music[i])+int(sfx[i]) {
			t.Fatalf("sample %d: mix %d, stems %d + %d + %d", i, mix[i], narration[i], music[i], sfx[i])
		}
		for k, v := range [3]int16{narration[i], music[i], sfx[i]} {
			if v != 0 {
				sounding[k]++
			}
		}
	}
	for k, n := range sounding {
		if n == 0 {
			t.Errorf("stem %d is silent, so the sum proved nothing for it", k)
		}
	}
}

func TestMixStemsSumToTheMix(t *testing.T) {
	dir := t.TempDir()
	clipPath := filepath.Join(dir, "piece.raw")
	bedPath := filepath.Join(dir, "bed.raw")
	mixTestWriteRaw(t, clipPath, mixTestSine(MixRate, 0.4, 330))
	// One second of bed under a longer film, so the stems also cross the loop.
	mixTestWriteRaw(t, bedPath, mixTestSine(MixRate, 0.3, 110))
	// The 1234 extra frames make the last block a short one.
	frames := int64(3*MixRate + 1234)
	in := &MixInput{
		Frames:    frames,
		Narration: []MixClip{{Key: "piece", Path: clipPath, Len: MixRate, Start: MixRate / 2, Voice: "p|v"}},
		SFX:       []MixClip{{Key: "sfx.pop", PCM: mixTestSine(MixRate/4, 0.2, 880), Len: MixRate / 4, Start: MixRate, GainDB: -3}},
		Bed:       &MixBed{Path: bedPath, Frames: MixRate, Volume: 0.25},
	}
	out := MixOutputs{
		Mix:       filepath.Join(dir, "mix.wav"),
		Narration: filepath.Join(dir, "stem-narration.wav"),
		Music:     filepath.Join(dir, "stem-music.wav"),
		SFX:       filepath.Join(dir, "stem-sfx.wav"),
	}
	w, err := newStemWriter(out)
	if err != nil {
		t.Fatal(err)
	}
	if err := runMixPass(context.Background(), in, mixPass{bedGainDB: -6, masterDB: -2}, w.write); err != nil {
		t.Fatal(err)
	}
	if err := w.close(); err != nil {
		t.Fatal(err)
	}
	if w.clamps != 0 {
		t.Fatalf("%d clamps at these low levels", w.clamps)
	}
	mixTestAssertStemsSum(t, out, frames)
}

func TestMixLoopCrossfadeIsEqualPowerAndAtMostHalfTheBed(t *testing.T) {
	cases := []struct {
		name              string
		bed, film         int64
		wantPeriod, wantX int64
	}{
		// 2 s is below half of 6 s, so the crossfade is 2 s and the period 6 - 2 = 4 s.
		{"long bed", 6 * MixRate, 12 * MixRate, 4 * MixRate, 2 * MixRate},
		// Half of 2 s is 1 s, below the 2 s crossfade, so the crossfade is 1 s and the period 1 s.
		{"short bed", 2 * MixRate, 6 * MixRate, 1 * MixRate, 1 * MixRate},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			period, xfade := loopShape(c.bed, c.film)
			if period != c.wantPeriod || xfade != c.wantX {
				t.Fatalf("loopShape = %d, %d, want %d, %d", period, xfade, c.wantPeriod, c.wantX)
			}
			// The head of the bed holds 0.25 and its tail 0.5, so the blend shows both weights.
			pcm := make([]int16, c.bed*mixChannels)
			for i := int64(0); i < c.bed; i++ {
				v := int16(0)
				if i < xfade {
					v = 8192
				} else if i >= period {
					v = 16384
				}
				pcm[i*2], pcm[i*2+1] = v, v
			}
			path := filepath.Join(t.TempDir(), "bed.raw")
			mixTestWriteRaw(t, path, pcm)
			in := &MixInput{Frames: c.film, Bed: &MixBed{Path: path, Frames: c.bed, Volume: 0.25}}
			_, music, _ := mixTestCollect(t, in, mixPass{rawBed: true})
			for _, pos := range []int64{0, xfade / 4, xfade / 2, xfade * 3 / 4} {
				// Equal power: the rising head has the sine weight and the falling tail the cosine,
				// so at the middle the sum is (0.25+0.5)*sqrt(0.5), not (0.25+0.5)/2.
				u := float64(pos) / float64(xfade) * math.Pi / 2
				want := 0.25*math.Sin(u) + 0.5*math.Cos(u)
				for _, n := range []int64{period + pos, 2*period + pos} {
					if got := music[n*mixChannels]; math.Abs(got-want) > 1e-9 {
						t.Errorf("frame %d = %.6f, want %.6f", n, got, want)
					}
				}
			}
		})
	}
	// A bed as long as the film never loops.
	if period, xfade := loopShape(6*MixRate, 6*MixRate); period != 6*MixRate || xfade != 0 {
		t.Fatalf("a bed of film length loops: %d, %d", period, xfade)
	}
}

func TestMixTrimmedClipFades5ms(t *testing.T) {
	// 5 ms at 48 kHz is 240 frames. The clip reads 0.5, and its head before In reads -0.5.
	const fade = clipFadeMS * MixRate / 1000
	pcm := append(mixTestConst(100, -16384), mixTestConst(4800, 16384)...)
	in := &MixInput{
		Frames: 12000,
		Narration: []MixClip{
			{Key: "trimmed", PCM: pcm, In: 100, Len: 4800, Start: 1000, Fade: fade},
			{Key: "whole", PCM: mixTestConst(1000, 16384), Len: 1000, Start: 7000},
		},
	}
	narration, _, _ := mixTestCollect(t, in, mixPass{})
	for _, c := range []struct {
		offset int64
		want   float64
	}{
		{0, 0},
		{fade / 2, 0.25},
		{fade, 0.5},
		{2400, 0.5},
		{4799 - fade/2, 0.25},
		{4799, 0},
	} {
		if got := narration[(1000+c.offset)*mixChannels]; math.Abs(got-c.want) > 1e-9 {
			t.Errorf("offset %d = %.6f, want %.6f", c.offset, got, c.want)
		}
	}
	if got := narration[7000*mixChannels]; got != 0.5 {
		t.Errorf("a clip without Fade starts at %.6f, want 0.5", got)
	}
}

func TestMixDropsAClipPastTheFilmEnd(t *testing.T) {
	// The path does not exist, so opening this clip would fail the pass.
	in := &MixInput{
		Frames: 9600,
		Narration: []MixClip{
			{Key: "late", Path: filepath.Join(t.TempDir(), "missing.raw"), Len: 4800, Start: 9600},
			{Key: "cut", PCM: mixTestConst(4800, 16384), Len: 4800, Start: 7200},
		},
	}
	narration, _, _ := mixTestCollect(t, in, mixPass{})
	if len(narration) != 9600*mixChannels {
		t.Fatalf("the pass gave %d samples", len(narration))
	}
	// 9600 - 7200 = 2400 frames of the second clip remain.
	usable := usableClips(in.Narration, in.Frames)
	if len(usable) != 1 || usable[0].Key != "cut" || usable[0].Len != 2400 {
		t.Fatalf("usable clips = %+v", usable)
	}
	// The cut edge is a trim, so it fades to 0 on the last frame.
	if got := narration[9599*mixChannels]; got != 0 {
		t.Fatalf("the last frame of the cut clip = %.6f, want 0", got)
	}
}

func TestMixBedGainAndFades(t *testing.T) {
	// 4 s of a constant 0.5, at -6.02 dB, so the flat part reads 0.25. The fade in is 0.5 s
	// (24000 frames) and the fade out 1.5 s (72000 frames), so their midpoints read 0.125.
	const frames = 4 * MixRate
	path := filepath.Join(t.TempDir(), "bed.raw")
	mixTestWriteRaw(t, path, mixTestConst(frames, 16384))
	in := &MixInput{Frames: frames, Bed: &MixBed{Path: path, Frames: frames, Volume: 0.25}}
	_, music, _ := mixTestCollect(t, in, mixPass{bedGainDB: 20 * math.Log10(0.5)})
	for _, c := range []struct {
		n    int64
		want float64
	}{
		{0, 0},
		{12000, 0.125},
		{60000, 0.25},
		{frames - 36000, 0.125},
	} {
		if got := music[c.n*mixChannels]; math.Abs(got-c.want) > 1e-9 {
			t.Errorf("frame %d = %.6f, want %.6f", c.n, got, c.want)
		}
	}
}

func TestMixPassStopsAtABlockBoundaryOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	blocks := 0
	err := runMixPass(ctx, &MixInput{Frames: mixBlockFrames * 10}, mixPass{}, func(b *mixBlock) error {
		blocks++
		if blocks == 3 {
			cancel()
		}
		return nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if blocks != 3 {
		t.Fatalf("%d blocks ran, want the 3 before the cancel", blocks)
	}
}

// mixTestConstBed builds an input whose bed is a constant 0.5, so the music stem reads the gain directly.
func mixTestConstBed(t *testing.T, frames int64) *MixInput {
	t.Helper()
	path := filepath.Join(t.TempDir(), "bed.raw")
	mixTestWriteRaw(t, path, mixTestConst(int(frames), 16384))
	return &MixInput{Frames: frames, Bed: &MixBed{Path: path, Frames: frames, Volume: 0.25}}
}

func mixTestAssertGainDB(t *testing.T, music []float64, n int64, want float64) {
	t.Helper()
	got := 20 * math.Log10(music[n*mixChannels]/0.5)
	if math.Abs(got-want) > 0.02 {
		t.Errorf("gain at frame %d = %.3f dB, want %.3f", n, got, want)
	}
}

// mixTestRMSDB is the RMS level of the left channel over the frames from up to to.
func mixTestRMSDB(stem []float64, from, to int64) float64 {
	sum := 0.0
	for n := from; n < to; n++ {
		v := stem[n*mixChannels]
		sum += v * v
	}
	return 10 * math.Log10(sum/float64(to-from))
}

func TestMixDuckReachesDepthAtSpeechAndReleasesOver400ms(t *testing.T) {
	const (
		speechStart = int64(3 * MixRate)
		speechLen   = int64(MixRate)
		lookahead   = int64(duckLookaheadMS * MixRate / 1000) // 50 ms is 2400 frames
		release     = int64(duckReleaseMS * MixRate / 1000)   // 400 ms is 19200 frames
		decayFrames = duckPeakDecayMS * MixRate / 1000        // 20 ms is 960 frames
	)
	// The film is 6 s, so the bed is flat from 0.5 s to 6 - 1.5 = 4.5 s, past the end of the release.
	in := mixTestConstBed(t, 6*MixRate)
	in.Narration = []MixClip{{Key: "speech", PCM: mixTestConst(MixRate, 16384), Len: speechLen, Start: speechStart}}
	_, music, _ := mixTestCollect(t, in, mixPass{})

	// The detector sees speech 50 ms early. The attack is a 50 ms ramp, so it ends as speech starts.
	duckStart := speechStart - lookahead
	mixTestAssertGainDB(t, music, duckStart-1, 0)
	mixTestAssertGainDB(t, music, duckStart+lookahead/2-1, -3)
	mixTestAssertGainDB(t, music, speechStart-1, -6)
	mixTestAssertGainDB(t, music, speechStart, -6)

	// The detector sees the last speech sample 50 ms early. Its peak then falls from 0.5 by
	// exp(-k/960) and stays above the -45 dB threshold for hold more frames. Then the 400 ms ramp runs.
	lastSeen := speechStart + speechLen - 1 - lookahead
	hold := int64(decayFrames * math.Log(0.5/dbToLin(duckThresholdDB)))
	releaseStart := lastSeen + hold + 1
	mixTestAssertGainDB(t, music, releaseStart-1, -6)
	mixTestAssertGainDB(t, music, releaseStart+release/2-1, -3)
	mixTestAssertGainDB(t, music, releaseStart+release-1, 0)
	mixTestAssertGainDB(t, music, releaseStart+release+100, 0)
}

func TestMixCarveCutsOnlyInsideSpans(t *testing.T) {
	// The film is 4 s, so the bed is flat from 24000 to 192000 - 72000 = 120000.
	const frames = int64(4 * MixRate)
	path := filepath.Join(t.TempDir(), "bed.raw")
	mixTestWriteRaw(t, path, mixTestSine(int(frames), 0.25, carveHz))
	in := &MixInput{
		Frames: frames,
		Bed:    &MixBed{Path: path, Frames: frames, Volume: 0.25},
		Spans:  []Span{{From: 60000, To: 72000}},
	}
	_, music, _ := mixTestCollect(t, in, mixPass{})

	// 4800 frames hold 220 whole cycles of 2.2 kHz. The RMS of a sine of amplitude a is a/sqrt(2).
	// A peaking filter has exactly its gain at its centre, so inside the span the level is 5 dB lower.
	// The ramps reach 20 ms (960 frames) past each edge, and these windows stay clear of them.
	full := 20 * math.Log10(0.25/math.Sqrt2)
	for _, c := range []struct {
		name string
		from int64
		want float64
	}{
		{"before the span", 40000, full},
		{"inside the span", 62000, full + carveDB},
		{"after the span", 90000, full},
	} {
		if got := mixTestRMSDB(music, c.from, c.from+4800); math.Abs(got-c.want) > 0.05 {
			t.Errorf("%s: %.3f dB, want %.3f", c.name, got, c.want)
		}
	}
}

func TestMixImpactDipFollowsAttackHoldReturn(t *testing.T) {
	// 10 ms is 480 frames, 80 ms is 3840 frames and 0.3 s is 14400 frames. 60000 + 18720 stays inside
	// the flat part of a 4 s film, which ends at 120000.
	const at = int64(60000)
	in := mixTestConstBed(t, 4*MixRate)
	in.Impacts = []Impact{{At: at, DepthDB: 3}}
	_, music, _ := mixTestCollect(t, in, mixPass{})

	mixTestAssertGainDB(t, music, at-1, 0)
	mixTestAssertGainDB(t, music, at+dipAttackFrames/2, -1.5)
	mixTestAssertGainDB(t, music, at+dipAttackFrames, -3)
	mixTestAssertGainDB(t, music, at+dipAttackFrames+dipHoldFrames-1, -3)
	mixTestAssertGainDB(t, music, at+dipAttackFrames+dipHoldFrames+dipReturnFrames/2, -1.5)
	mixTestAssertGainDB(t, music, at+dipAttackFrames+dipHoldFrames+dipReturnFrames, 0)
}
