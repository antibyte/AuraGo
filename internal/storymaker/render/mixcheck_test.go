package render

import (
	"reflect"
	"testing"
)

// mixCheckRun feeds a mixCheck the way pass 3 does: blocks of mixBlockFrames frames, stereo, each stem a constant per
// frame given by its function of the time in seconds.
func mixCheckRun(frames int64, narration, music func(sec float64) float64) *mixCheck {
	c := newMixCheck(frames)
	narr, mus, sfx := make([]float64, mixBlockFrames*mixChannels), make([]float64, mixBlockFrames*mixChannels), make([]float64, mixBlockFrames*mixChannels)
	for n0 := int64(0); n0 < frames; n0 += mixBlockFrames {
		f := int(min(mixBlockFrames, frames-n0))
		for i := 0; i < f; i++ {
			sec := float64(n0+int64(i)) / MixRate
			for ch := 0; ch < mixChannels; ch++ {
				narr[i*mixChannels+ch], mus[i*mixChannels+ch] = narration(sec), music(sec)
			}
		}
		c.observe(&mixBlock{frames: f, narration: narr[:f*mixChannels], music: mus[:f*mixChannels], sfx: sfx[:f*mixChannels]})
	}
	return c
}

func mixCheckConst(v float64) func(float64) float64 { return func(float64) float64 { return v } }

// mixCheckSpeech speaks (0.1, -20 dBFS) from 1 s to 9 s of a 12 s film, with pauses of 1 s.
func mixCheckSpeech(sec float64) float64 {
	if sec >= 1 && sec < 9 {
		return 0.1
	}
	return 0
}

func TestMixCheckBedTooLoud(t *testing.T) {
	// The bed at 0.05 is 6 dB under the speech, less than the 12 dB that the check demands.
	c := mixCheckRun(12*MixRate, mixCheckSpeech, mixCheckConst(0.05))
	warnings, at := c.bedWarnings()
	if !reflect.DeepEqual(warnings, []string{"bed_too_loud"}) {
		t.Fatalf("warnings %v", warnings)
	}
	// The first window of 6 s that is half speech: it starts at 0 s, because 5 s of its 6 s are speech.
	if got := at["bed_too_loud"]; got != 0 {
		t.Errorf("at %v", got)
	}
}

func TestMixCheckBedTooLoudStaysQuietForAGoodBed(t *testing.T) {
	// 0.0125 is 18 dB under the speech, where the mix aims. A loud bed under silence is not judged: the narration
	// is active in under half of every window. Speech from 1 s to 3 s only.
	short := func(sec float64) float64 {
		if sec >= 1 && sec < 3 {
			return 0.1
		}
		return 0
	}
	for name, c := range map[string]*mixCheck{
		"good bed":        mixCheckRun(12*MixRate, mixCheckSpeech, mixCheckConst(0.0125)),
		"loud bed, quiet": mixCheckRun(12*MixRate, short, mixCheckConst(0.05)),
		"no bed":          mixCheckRun(12*MixRate, mixCheckSpeech, mixCheckConst(0)),
	} {
		if warnings, _ := c.bedWarnings(); len(warnings) != 0 {
			t.Errorf("%s: %v", name, warnings)
		}
	}
}

func TestMixCheckBedGap(t *testing.T) {
	// A steady bed that stops dead at 3 s of a 10 s film.
	stops := func(sec float64) float64 {
		if sec < 3 {
			return 0.05
		}
		return 0
	}
	warnings, at := mixCheckRun(10*MixRate, mixCheckConst(0), stops).bedWarnings()
	if !reflect.DeepEqual(warnings, []string{"bed_gap"}) {
		t.Fatalf("warnings %v", warnings)
	}
	if got := at["bed_gap"]; got < 2.99 || got > 3.01 {
		t.Errorf("at %v", got)
	}
}

func TestMixCheckBedGapIgnoresNormalMusic(t *testing.T) {
	// 0.5 s steady, then 14 dB down, over and over: it never holds steady for 1 s.
	pulsing := func(sec float64) float64 {
		if int(sec*2)%2 == 0 {
			return 0.05
		}
		return 0.01
	}
	// The planned fade-out: steady until 8.5 s, then a stop inside the last 1.5 s of the film.
	fadeOut := func(sec float64) float64 {
		if sec < 9 {
			return 0.05
		}
		return 0
	}
	// A slow fade is no step.
	fade := func(sec float64) float64 { return 0.05 * (1 - sec/10) }
	for name, c := range map[string]*mixCheck{
		"pulsing":   mixCheckRun(10*MixRate, mixCheckConst(0), pulsing),
		"fade out":  mixCheckRun(10*MixRate, mixCheckConst(0), fadeOut),
		"slow fade": mixCheckRun(10*MixRate, mixCheckConst(0), fade),
	} {
		if warnings, _ := c.bedWarnings(); len(warnings) != 0 {
			t.Errorf("%s: %v", name, warnings)
		}
	}
}

func TestMixWarningsAreInTheOrderOfTheSpec(t *testing.T) {
	// Speech from 1 s to 9 s over a bed at 6 dB under it that stops dead at 5 s: too loud, then a gap, and a clamp.
	bed := func(sec float64) float64 {
		if sec < 5 {
			return 0.05
		}
		return 0
	}
	c := mixCheckRun(12*MixRate, mixCheckSpeech, bed)
	warnings, at := mixWarnings(c, 3)
	if want := []string{"bed_too_loud", "bed_gap", "mix_clipped"}; !reflect.DeepEqual(warnings, want) {
		t.Fatalf("warnings %v, want %v", warnings, want)
	}
	if at["bed_gap"] < 4.99 || at["bed_gap"] > 5.01 {
		t.Errorf("bed_gap at %v", at["bed_gap"])
	}
}

func TestMixWarningsClampsAloneGiveMixClipped(t *testing.T) {
	c := mixCheckRun(12*MixRate, mixCheckSpeech, mixCheckConst(0.0125))
	if warnings, _ := mixWarnings(c, 1); !reflect.DeepEqual(warnings, []string{"mix_clipped"}) {
		t.Errorf("one clamp: %v", warnings)
	}
	if warnings, _ := mixWarnings(c, 0); len(warnings) != 0 {
		t.Errorf("no clamp: %v", warnings)
	}
}

func TestMixWarningMessages(t *testing.T) {
	for code, want := range map[string]string{
		warnBedTooLoud: "The music is loud under the narration (first at 12.5 s), so speech may be hard to follow.",
		warnBedGap:     "The music stops abruptly at 12.5 s.",
		warnMixClipped: "The final mix clipped at least one sample, so it may sound distorted.",
	} {
		if got := mixWarningMessage(code, 12.5); got != want {
			t.Errorf("%s: %q", code, got)
		}
	}
}
