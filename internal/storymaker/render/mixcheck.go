package render

import "math"

// The mix checks measure the pass-3 stem blocks, after all gains, in steps of 10 ms. They only detect: the mix is never
// changed, because the stems must sum to it exactly.
const (
	mixCheckStepFrames = MixRate / 100 // frames in one 10 ms step
	// bed_too_loud: a 6 s window in which the narration speaks at least half the time, and a bed that is less than 12 dB
	// under the narration while it speaks. The mix aims at 18 dB (12 LU plus 6 dB of ducking).
	mixCheckWindowSteps  = 600
	mixCheckBedMarginDB  = 12.0
	mixCheckSpeakingDB   = duckThresholdDB // a step with the narration above this level counts as speaking
	mixCheckMinWindowSec = 1               // a film shorter than this has no window
	// bed_gap: a bed that held steady for 1 s (its 10 ms levels within 3 dB) and then falls by 12 dB in one step.
	mixCheckSteadySteps  = 100
	mixCheckSteadySpread = 3.0
	mixCheckGapDropDB    = 12.0
	mixCheckBedFloorDB   = -90.0 // a bed below this is silence, not music that stopped
	mixCheckSilenceDB    = -120.0
)

// mixCheck collects the mean power of the narration and the music stem per 10 ms step.
type mixCheck struct {
	film             int64
	narration, music []float64
	frames           int     // frames in the open step
	nSum, mSum       float64 // sum of squares in the open step, both channels
}

func newMixCheck(film int64) *mixCheck { return &mixCheck{film: film} }

// observe adds one block of pass 3. The stems are interleaved stereo.
func (c *mixCheck) observe(b *mixBlock) {
	for i := 0; i < b.frames; i++ {
		for ch := 0; ch < mixChannels; ch++ {
			n, m := b.narration[i*mixChannels+ch], b.music[i*mixChannels+ch]
			c.nSum += n * n
			c.mSum += m * m
		}
		if c.frames++; c.frames == mixCheckStepFrames {
			per := float64(c.frames * mixChannels)
			c.narration = append(c.narration, c.nSum/per)
			c.music = append(c.music, c.mSum/per)
			c.frames, c.nSum, c.mSum = 0, 0, 0
		}
	}
}

func powerDB(p float64) float64 {
	if p <= 0 {
		return mixCheckSilenceDB
	}
	return max(10*math.Log10(p), mixCheckSilenceDB)
}

// bedWarnings returns the codes of the bed checks in the order bed_too_loud, bed_gap, and the second at which each
// began.
func (c *mixCheck) bedWarnings() ([]string, map[string]float64) {
	var codes []string
	at := map[string]float64{}
	if sec, ok := c.bedTooLoud(); ok {
		codes = append(codes, warnBedTooLoud)
		at[warnBedTooLoud] = sec
	}
	if sec, ok := c.bedGap(); ok {
		codes = append(codes, warnBedGap)
		at[warnBedGap] = sec
	}
	return codes, at
}

// bedTooLoud slides a window of 6 s (the whole film when it is shorter) over the steps. In a window where the narration
// speaks in at least half the steps, the RMS of the music over those steps is compared to the RMS of the narration.
func (c *mixCheck) bedTooLoud() (float64, bool) {
	steps := len(c.narration)
	window := min(mixCheckWindowSteps, steps)
	if steps < mixCheckMinWindowSec*100 {
		return 0, false
	}
	speaking := math.Pow(10, mixCheckSpeakingDB/10)
	// Prefix sums over the speaking steps: how many, and their narration and music power.
	count, narr, mus := make([]int, steps+1), make([]float64, steps+1), make([]float64, steps+1)
	for i := 0; i < steps; i++ {
		count[i+1], narr[i+1], mus[i+1] = count[i], narr[i], mus[i]
		if c.narration[i] > speaking {
			count[i+1]++
			narr[i+1] += c.narration[i]
			mus[i+1] += c.music[i]
		}
	}
	for s := 0; s+window <= steps; s++ {
		n := count[s+window] - count[s]
		if n == 0 || 2*n < window {
			continue
		}
		bed, voice := mus[s+window]-mus[s], narr[s+window]-narr[s]
		if bed > 0 && powerDB(bed/float64(n))-powerDB(voice/float64(n)) > -mixCheckBedMarginDB {
			return float64(s) / 100, true
		}
	}
	return 0, false
}

// bedGap finds a bed that was steady for 1 s and then fell by 12 dB in one step. The planned fade-out of the last
// musicFadeOutS is left out: the bed ends there on purpose.
func (c *mixCheck) bedGap() (float64, bool) {
	fadeOutFrom := c.film - int64(musicFadeOutS*MixRate)
	level := make([]float64, len(c.music))
	for i, p := range c.music {
		level[i] = powerDB(p)
	}
	for i := mixCheckSteadySteps; i < len(level); i++ {
		if int64(i)*mixCheckStepFrames >= fadeOutFrom {
			break
		}
		if level[i-1] < mixCheckBedFloorDB || level[i-1]-level[i] < mixCheckGapDropDB {
			continue
		}
		lo, hi := level[i-1], level[i-1]
		for _, v := range level[i-mixCheckSteadySteps : i] {
			lo, hi = min(lo, v), max(hi, v)
		}
		if hi-lo < mixCheckSteadySpread {
			return float64(i) / 100, true
		}
	}
	return 0, false
}
