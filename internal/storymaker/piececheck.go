package storymaker

import (
	"math"
	"sort"
)

const (
	// A dropout is a fall from at least dropoutFromPower to below dropoutToPower in one 10 ms step inside speech,
	// followed by dropoutMinWindows to dropoutMaxWindows of silence and then speech again.
	dropoutFromPower  = 3.1622776601683795e-5 // -45 dBFS
	dropoutToPower    = 1e-8                  // -80 dBFS
	dropoutMinWindows = 5                     // 50 ms
	dropoutMaxWindows = 300                   // 3 s

	// A clipped piece has clipRunsPerSecond runs of clipRunHard samples at or above clipFull, or of clipRunFlat equal
	// samples at or above clipFlatFloor, inside one second.
	clipFull          = 0.999
	clipFlatFloor     = 0.05
	clipRunHard       = 3
	clipRunFlat       = 4
	clipRunsPerSecond = 3
)

// CheckPiece reports the faults of one synthesized piece: "clipped", "dropout" (sorted, each at most once). env is the
// envelope of the same audio; its speech runs say where a fall counts as a hole and not as the end of the speech.
func CheckPiece(audio DecodedAudio, env Envelope) []string {
	var faults []string
	if pieceClipped(audio) {
		faults = append(faults, "clipped")
	}
	if pieceDropout(audio, env) {
		faults = append(faults, "dropout")
	}
	return faults
}

// windowPowers is the mean power per 10 ms window with the windowing of ComputeEnvelope, but unclamped, because the
// envelope's level byte stops at -60 dBFS and a dropout lies below -80 dBFS.
func windowPowers(a DecodedAudio) []float64 {
	if a.Rate <= 0 || a.Channels <= 0 || len(a.Samples) < a.Channels {
		return nil
	}
	channels := int64(a.Channels)
	frames := int64(len(a.Samples)) / channels
	rate := int64(a.Rate)
	out := make([]float64, (frames*envelopeRate+rate-1)/rate)
	for i := range out {
		first := int64(i) * rate / envelopeRate
		end := min(int64(i+1)*rate/envelopeRate, frames)
		if end <= first {
			continue
		}
		sum := 0.0
		samples := a.Samples[first*channels : end*channels]
		for _, s := range samples {
			sum += float64(s) * float64(s)
		}
		out[i] = sum / float64(len(samples))
	}
	return out
}

// pieceDropout finds a hole inside speech: the window before the fall lies in a speech run, the quiet stretch lasts
// 50 ms to 3 s, and a speech level follows it. Speech that simply ends has nothing after it.
func pieceDropout(audio DecodedAudio, env Envelope) bool {
	power := windowPowers(audio)
	runs := env.SpeechRuns()
	inRun := func(window int) bool {
		ms := window * envelopeWindowMS
		for _, r := range runs {
			if ms >= r[0] && ms < r[1] {
				return true
			}
		}
		return false
	}
	for i := 1; i < len(power); i++ {
		if power[i-1] < dropoutFromPower || power[i] >= dropoutToPower || !inRun(i-1) {
			continue
		}
		j := i
		for j < len(power) && power[j] < dropoutToPower {
			j++
		}
		n := j - i
		if n >= dropoutMinWindows && n <= dropoutMaxWindows && j < len(env.Levels) && env.Levels[j] >= speechLevel {
			return true
		}
		i = j
	}
	return false
}

// pieceClipped finds runs of consecutive samples of one channel: at least clipRunHard at full scale, or at least
// clipRunFlat equal ones above clipFlatFloor. A sample can belong to both kinds, so a run is a maximal stretch of
// flagged samples and counts once. The piece is clipped with clipRunsPerSecond runs inside one second.
func pieceClipped(a DecodedAudio) bool {
	if a.Rate <= 0 || a.Channels <= 0 {
		return false
	}
	frames := len(a.Samples) / a.Channels
	var starts []int
	for ch := 0; ch < a.Channels; ch++ {
		at := func(f int) float32 { return a.Samples[f*a.Channels+ch] }
		flagged := make([]bool, frames)
		for i := 0; i < frames; {
			j := i + 1
			for j < frames && at(j) == at(i) {
				j++
			}
			if j-i >= clipRunFlat && math.Abs(float64(at(i))) >= clipFlatFloor {
				for k := i; k < j; k++ {
					flagged[k] = true
				}
			}
			i = j
		}
		for i := 0; i < frames; {
			if math.Abs(float64(at(i))) < clipFull {
				i++
				continue
			}
			j := i
			for j < frames && math.Abs(float64(at(j))) >= clipFull {
				j++
			}
			if j-i >= clipRunHard {
				for k := i; k < j; k++ {
					flagged[k] = true
				}
			}
			i = j
		}
		for i := 0; i < frames; i++ {
			if flagged[i] && (i == 0 || !flagged[i-1]) {
				starts = append(starts, i)
			}
		}
	}
	sort.Ints(starts)
	for k := 0; k+clipRunsPerSecond <= len(starts); k++ {
		if starts[k+clipRunsPerSecond-1]-starts[k] < a.Rate {
			return true
		}
	}
	return false
}
