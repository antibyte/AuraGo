package render

import (
	"context"
	"encoding/binary"
	"encoding/json"
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

func mixTestNewMeter(t *testing.T) func(context.Context) (*Meter, error) {
	t.Helper()
	ffmpeg := testBinary(t, "ffmpeg")
	return func(ctx context.Context) (*Meter, error) { return StartMeter(ctx, ffmpeg) }
}

func mixTestOutputs(dir string) MixOutputs {
	return MixOutputs{
		Mix:       filepath.Join(dir, "mix.wav"),
		Narration: filepath.Join(dir, "stem-narration.wav"),
		Music:     filepath.Join(dir, "stem-music.wav"),
		SFX:       filepath.Join(dir, "stem-sfx.wav"),
	}
}

func TestMixVoiceMatchLandsTwoVoicesWithinOneDB(t *testing.T) {
	const seconds = 2 * MixRate
	// The first half of the quiet clip is silence outside the spans. If it counted, the voice would
	// read 3 dB lower and its gain would land 3 dB too high.
	quiet := append(make([]int16, seconds*mixChannels), mixTestSine(seconds, 0.4*dbToLin(-4.5), 220)...)
	quietStart := int64(seconds + MixRate/2)
	quietSpeech := quietStart + seconds
	in := &MixInput{
		Frames: quietStart + 2*seconds,
		Narration: []MixClip{
			{Key: "loud", PCM: mixTestSine(seconds, 0.4, 220), Len: seconds, Start: 0, Voice: "p|loud"},
			{Key: "quiet", PCM: quiet, Len: 2 * seconds, Start: quietStart, Voice: "p|quiet"},
		},
		Spans: []Span{{From: 0, To: seconds}, {From: quietSpeech, To: quietSpeech + seconds}},
	}
	gains, err := matchVoices(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	// gain = loudest - voice: 0 for the loud voice and 4.5 dB for the quiet one.
	if math.Abs(gains["p|loud"]) > 0.05 || math.Abs(gains["p|quiet"]-4.5) > 0.05 {
		t.Fatalf("gains = %v, want 0 and 4.5", gains)
	}
	if in.Narration[1].GainDB != gains["p|quiet"] {
		t.Fatalf("the clip holds %.3f dB, the voice %.3f dB", in.Narration[1].GainDB, gains["p|quiet"])
	}
	narration, _, _ := mixTestCollect(t, in, mixPass{})
	loudDB := mixTestRMSDB(narration, 0, seconds)
	quietDB := mixTestRMSDB(narration, quietSpeech, quietSpeech+seconds)
	if math.Abs(loudDB-quietDB) > 1 {
		t.Fatalf("the voices land at %.2f and %.2f dB", loudDB, quietDB)
	}
}

func TestMixVoiceMatchIsCappedAtSixDB(t *testing.T) {
	// 0.5 and 0.05 are 20 dB apart, above the +6 dB cap.
	in := &MixInput{
		Frames: 2 * MixRate,
		Narration: []MixClip{
			{Key: "a", PCM: mixTestSine(MixRate, 0.5, 220), Len: MixRate, Start: 0, Voice: "a"},
			{Key: "b", PCM: mixTestSine(MixRate, 0.05, 220), Len: MixRate, Start: MixRate, Voice: "b"},
		},
		Spans: []Span{{From: 0, To: 2 * MixRate}},
	}
	gains, err := matchVoices(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if gains["b"] != voiceMatchCapDB {
		t.Fatalf("gain = %.3f, want the cap", gains["b"])
	}
}

func TestMixLevels(t *testing.T) {
	silent := &MeterReading{I: math.Inf(-1), TP: math.Inf(-1), Silent: true}
	twice := 20 * math.Log10(2) // volume 0.5 is 20*log10(0.5/0.25), about +6.02 dB
	cases := []struct {
		name        string
		narrI       float64
		hasNarr     bool
		bed         *MeterReading
		volume      float64
		wantBedGain float64
		wantDropped bool
		wantSFXRef  float64
	}{
		// Reference -22-12 = -34; gain -34-(-30) = -4; the sfx reference is the actual bed level.
		{"narration and bed", -22, true, &MeterReading{I: -30}, 0.25, -4, false, -34},
		{"volume 0.5 is +6 dB", -22, true, &MeterReading{I: -30}, 0.5, -4 + twice, false, -34 + twice},
		// Reference -18 without narration; gain -18-(-30) = +12.
		{"no narration", 0, false, &MeterReading{I: -30}, 0.25, 12, false, -18},
		// No music: the sfx reference is narration I-12, or -18 without narration.
		{"no music", -22, true, nil, 0, 0, false, -34},
		{"no music, no narration", 0, false, nil, 0, 0, false, -18},
		// Gain -34-(-60) = 26 stops at +24, so the bed really sits at -60+24 = -36.
		{"quiet bed", -22, true, &MeterReading{I: -60}, 0.25, 24, false, -36},
		{"silent bed", -22, true, silent, 0.25, 0, true, -34},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := computeMixLevels(MeterReading{I: c.narrI}, c.hasNarr, c.bed, c.volume)
			if math.Abs(got.bedGainDB-c.wantBedGain) > 1e-9 || got.bedDropped != c.wantDropped || math.Abs(got.sfxRefDB-c.wantSFXRef) > 1e-9 {
				t.Fatalf("levels = %+v, want bed gain %.3f, dropped %v, sfx reference %.3f", got, c.wantBedGain, c.wantDropped, c.wantSFXRef)
			}
		})
	}
}

func TestMixMasterGainIsTheMinOfThreeBounds(t *testing.T) {
	cases := []struct {
		name string
		pre  MeterReading
		peak float64
		want float64
	}{
		// Loudness -16-(-18) = 2; peak -1-(-10) = 9; stem -0.1-(-20) = 19.9.
		{"loudness binds", MeterReading{I: -18, TP: -10}, 0.1, 2},
		// Loudness -16-(-20) = 4; peak -1-(-3) = 2; stem -0.1+6.02 = 5.92.
		{"true peak binds", MeterReading{I: -20, TP: -3}, 0.5, 2},
		// Loudness 14; peak 19; stem -0.1-20*log10(0.99).
		{"stem peak binds", MeterReading{I: -30, TP: -20}, 0.99, -0.1 - 20*math.Log10(0.99)},
		{"silent", MeterReading{I: math.Inf(-1), TP: math.Inf(-1), Silent: true}, 0, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := masterGainDB(c.pre, c.peak); math.Abs(got-c.want) > 1e-9 {
				t.Fatalf("G = %.4f, want %.4f", got, c.want)
			}
		})
	}
}

func TestMixPassesApplyTheGains(t *testing.T) {
	newMeter := mixTestNewMeter(t)
	dir := t.TempDir()
	clipPath := filepath.Join(dir, "piece.raw")
	bedPath := filepath.Join(dir, "bed.raw")
	mixTestWriteRaw(t, clipPath, mixTestSine(4*MixRate, 0.3, 440))
	mixTestWriteRaw(t, bedPath, mixTestSine(5*MixRate/2, 0.2, 110))
	const offset = 22.5 // over_db 4.5 minus l50_lufs -18, the pop of the SFX set
	frames := int64(6 * MixRate)
	in := &MixInput{
		Frames:    frames,
		Narration: []MixClip{{Key: "piece", Path: clipPath, Len: 4 * MixRate, Start: MixRate, Voice: "p|v"}},
		SFX:       []MixClip{{Key: "sfx.pop", PCM: mixTestSine(MixRate/5, 0.2, 880), Len: MixRate / 5, Start: 3 * MixRate, LevelOffsetDB: offset}},
		Bed:       &MixBed{Path: bedPath, Frames: 5 * MixRate / 2, Volume: 0.5},
		Spans:     []Span{{From: MixRate, To: 5 * MixRate}},
		Impacts:   []Impact{{At: 3 * MixRate, DepthDB: 3}},
	}
	out := mixTestOutputs(dir)
	last := 0.0
	res, err := Mix(context.Background(), in, newMeter, out, func(p float64) {
		if p < last {
			t.Errorf("progress went back from %.3f to %.3f", last, p)
		}
		last = p
	})
	if err != nil {
		t.Fatalf("Mix: %v", err)
	}
	if last != 1 {
		t.Errorf("progress ended at %.3f", last)
	}
	if res.Clamps != 0 || res.BedDropped {
		t.Errorf("clamps %d, dropped %v", res.Clamps, res.BedDropped)
	}
	if res.VoiceGainDB["p|v"] != 0 {
		t.Errorf("a single voice got %.3f dB", res.VoiceGainDB["p|v"])
	}
	// Bed gain: the reference narration I-12+20*log10(0.5/0.25), minus the measured bed.
	wantBed := res.NarrationI + bedBelowNarrLU + 20*math.Log10(2) - res.BedI
	if math.Abs(res.BedGainDB-wantBed) > 1e-9 {
		t.Errorf("bed gain %.3f, want %.3f", res.BedGainDB, wantBed)
	}
	// Sfx gain: the actual bed level plus LevelOffsetDB.
	if want := res.BedI + res.BedGainDB + offset; math.Abs(in.SFX[0].GainDB-want) > 1e-9 {
		t.Errorf("sfx gain %.3f, want %.3f", in.SFX[0].GainDB, want)
	}
	// StemPeak is after the master gain, so the pass 2 peak is that value without it.
	peak := max(res.StemPeak[0], res.StemPeak[1], res.StemPeak[2])
	prePeakDB := 20*math.Log10(peak) - res.MasterGainDB
	wantG := min(mixTargetLUFS-res.PreI, mixCeilingDBTP-res.PreTP, mixStemGuardDBFS-prePeakDB)
	if math.Abs(res.MasterGainDB-wantG) > 0.001 {
		t.Errorf("G = %.4f, want %.4f", res.MasterGainDB, wantG)
	}
	mixTestAssertStemsSum(t, out, frames)
}

func TestMixSilentNarrationFails(t *testing.T) {
	newMeter := mixTestNewMeter(t)
	out := mixTestOutputs(t.TempDir())
	in := &MixInput{
		Frames:    2 * MixRate,
		Narration: []MixClip{{Key: "piece", PCM: make([]int16, MixRate*mixChannels), Len: MixRate, Voice: "p|v"}},
		Spans:     []Span{{From: 0, To: MixRate}},
	}
	_, err := Mix(context.Background(), in, newMeter, out, nil)
	var jobErr *JobError
	if !errors.As(err, &jobErr) || jobErr.Code != CodeNarrationFailed {
		t.Fatalf("err = %v, want %s", err, CodeNarrationFailed)
	}
	if _, statErr := os.Stat(out.Mix); !os.IsNotExist(statErr) {
		t.Fatalf("a failed mix left its file: %v", statErr)
	}
}

func TestMixDropsASilentBed(t *testing.T) {
	newMeter := mixTestNewMeter(t)
	dir := t.TempDir()
	bedPath := filepath.Join(dir, "bed.raw")
	mixTestWriteRaw(t, bedPath, make([]int16, 3*MixRate*mixChannels))
	in := &MixInput{
		Frames:    3 * MixRate,
		Narration: []MixClip{{Key: "piece", PCM: mixTestSine(2*MixRate, 0.3, 440), Len: 2 * MixRate, Voice: "p|v"}},
		Bed:       &MixBed{Path: bedPath, Frames: 3 * MixRate, Volume: 0.25},
		Spans:     []Span{{From: 0, To: 2 * MixRate}},
	}
	out := mixTestOutputs(dir)
	res, err := Mix(context.Background(), in, newMeter, out, nil)
	if err != nil {
		t.Fatalf("Mix: %v", err)
	}
	if !res.BedDropped || res.BedGainDB != 0 || res.StemPeak[1] != 0 {
		t.Fatalf("result = %+v, want a dropped bed with no gain", res)
	}
	if _, statErr := os.Stat(out.Music); !os.IsNotExist(statErr) {
		t.Fatalf("a dropped bed left a music stem: %v", statErr)
	}
	if _, statErr := os.Stat(out.Narration); statErr != nil {
		t.Fatalf("narration stem: %v", statErr)
	}
}

func TestMixSilentFilmKeepsUnityGain(t *testing.T) {
	newMeter := mixTestNewMeter(t)
	dir := t.TempDir()
	bedPath := filepath.Join(dir, "bed.raw")
	mixTestWriteRaw(t, bedPath, make([]int16, 2*MixRate*mixChannels))
	in := &MixInput{Frames: 2 * MixRate, Bed: &MixBed{Path: bedPath, Frames: 2 * MixRate, Volume: 0.25}}
	out := mixTestOutputs(dir)
	res, err := Mix(context.Background(), in, newMeter, out, nil)
	if err != nil {
		t.Fatalf("no narration events, so a silent film must not fail: %v", err)
	}
	if res.MasterGainDB != 0 || !res.BedDropped {
		t.Fatalf("result = %+v, want G 0 and a dropped bed", res)
	}
	// The job record stores these fields, and JSON has no -Inf.
	if _, err := json.Marshal(res); err != nil {
		t.Fatalf("the result is not valid JSON: %v", err)
	}
	if got := len(mixTestReadWAV(t, out.Mix)); got != 2*MixRate*mixChannels {
		t.Fatalf("mix has %d samples", got)
	}
}
