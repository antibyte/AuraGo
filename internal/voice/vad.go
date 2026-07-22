package voice

import "math"

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
	minimumAmplitude float64
}

func NewTurnDetector(frameMS, startMS, endMS, preRollMS int) *TurnDetector {
	if frameMS < 1 {
		frameMS = 20
	}
	return &TurnDetector{
		noiseFloor:       120,
		startFrames:      max(1, startMS/frameMS),
		endFrames:        max(1, endMS/frameMS),
		preRollFrames:    max(0, preRollMS/frameMS),
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
			for _, frame := range v.preRoll {
				v.utterance = append(v.utterance, frame...)
			}
			v.preRoll = nil
		}
		return started, nil
	}

	v.utterance = append(v.utterance, samples...)
	if speech {
		v.silenceFrames = 0
	} else {
		v.silenceFrames++
	}
	if v.silenceFrames < v.endFrames {
		return false, nil
	}
	result := append([]int16(nil), v.utterance...)
	v.active = false
	v.speechFrames = 0
	v.silenceFrames = 0
	v.utterance = nil
	v.preRoll = nil
	return false, result
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
