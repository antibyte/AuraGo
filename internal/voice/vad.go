package voice

import "math"

const defaultMaxUtteranceMS = 120000

// TurnDetector implements an adaptive telephone VAD with bounded pre-roll and
// silence-based end-of-turn detection.
type TurnDetector struct {
	noiseFloor       float64
	active           bool
	speechFrames     int
	silenceFrames    int
	startFrames      int
	endFrames        int
	preRollFrames    int
	preRoll          [][]int16
	utterance        []int16
	captureUtterance bool
	maxFrames        int
	maxSamples       int
	overflowed       bool
	overflowPending  bool
	minimumAmplitude float64
}

func NewTurnDetector(frameMS, startMS, endMS, preRollMS int) *TurnDetector {
	return newTurnDetector(frameMS, startMS, endMS, preRollMS, defaultMaxUtteranceMS, true)
}

// NewActivityDetector tracks speech activity without retaining full utterance
// audio. It is used by duplex providers that consume frames directly.
func NewActivityDetector(frameMS, startMS, endMS, preRollMS int) *TurnDetector {
	return newTurnDetector(frameMS, startMS, endMS, preRollMS, 0, false)
}

func newTurnDetector(frameMS, startMS, endMS, preRollMS, maxUtteranceMS int, captureUtterance bool) *TurnDetector {
	if frameMS < 1 {
		frameMS = 20
	}
	return &TurnDetector{
		noiseFloor:       120,
		startFrames:      max(1, startMS/frameMS),
		endFrames:        max(1, endMS/frameMS),
		preRollFrames:    max(0, preRollMS/frameMS),
		captureUtterance: captureUtterance,
		maxFrames:        max(1, maxUtteranceMS/frameMS),
		minimumAmplitude: 280,
	}
}

// Push returns a complete utterance when trailing silence closes a turn.
func (v *TurnDetector) Push(samples []int16) (started bool, utterance []int16) {
	if len(samples) == 0 {
		return false, nil
	}
	rms := frameRMS(samples)
	threshold := math.Max(v.minimumAmplitude, v.noiseFloor*2.4)
	speech := rms >= threshold
	if !v.active && !speech {
		v.noiseFloor = v.noiseFloor*0.97 + rms*0.03
	}

	if !v.active {
		v.pushPreRoll(samples)
		if speech {
			v.speechFrames++
		} else {
			v.speechFrames = 0
		}
		if v.speechFrames >= v.startFrames {
			v.active = true
			started = true
			if v.captureUtterance {
				for _, frame := range v.preRoll {
					v.appendUtterance(frame)
				}
			}
			v.preRoll = nil
		}
		return started, nil
	}

	v.appendUtterance(samples)
	if speech {
		v.silenceFrames = 0
	} else {
		v.silenceFrames++
	}
	if v.silenceFrames < v.endFrames {
		return false, nil
	}
	result := make([]int16, 0)
	if v.captureUtterance {
		result = append(result, v.utterance...)
	}
	v.active = false
	v.speechFrames = 0
	v.silenceFrames = 0
	v.utterance = nil
	v.preRoll = nil
	v.maxSamples = 0
	v.overflowed = false
	return false, result
}

// TakeOverflow reports once per utterance that old audio was discarded to
// preserve the detector's memory bound.
func (v *TurnDetector) TakeOverflow() bool {
	if !v.overflowPending {
		return false
	}
	v.overflowPending = false
	return true
}

func (v *TurnDetector) appendUtterance(samples []int16) {
	if !v.captureUtterance || len(samples) == 0 {
		return
	}
	if v.maxSamples == 0 {
		v.maxSamples = v.maxFrames * len(samples)
	}
	if len(samples) >= v.maxSamples {
		v.utterance = append(v.utterance[:0], samples[len(samples)-v.maxSamples:]...)
		v.markOverflow()
		return
	}
	if excess := len(v.utterance) + len(samples) - v.maxSamples; excess > 0 {
		copy(v.utterance, v.utterance[excess:])
		v.utterance = v.utterance[:len(v.utterance)-excess]
		v.markOverflow()
	}
	v.utterance = append(v.utterance, samples...)
}

func (v *TurnDetector) markOverflow() {
	if !v.overflowed {
		v.overflowed = true
		v.overflowPending = true
	}
}

func (v *TurnDetector) pushPreRoll(samples []int16) {
	if v.preRollFrames == 0 {
		return
	}
	v.preRoll = append(v.preRoll, append([]int16(nil), samples...))
	if len(v.preRoll) > v.preRollFrames {
		v.preRoll = v.preRoll[len(v.preRoll)-v.preRollFrames:]
	}
}

func frameRMS(samples []int16) float64 {
	var sum float64
	for _, sample := range samples {
		value := float64(sample)
		sum += value * value
	}
	return math.Sqrt(sum / float64(len(samples)))
}
