package storymaker

import (
	"math"
	"reflect"
	"testing"
)

const pieceTestRate = 24000

// pieceTestSpeech is a mono 24 kHz sine of the given amplitude (a 220 Hz tone stands in for a voice).
func pieceTestSpeech(seconds, amp float64) DecodedAudio {
	samples := make([]float32, int(seconds*pieceTestRate))
	for i := range samples {
		samples[i] = float32(amp * math.Sin(2*math.Pi*220*float64(i)/pieceTestRate))
	}
	return DecodedAudio{Rate: pieceTestRate, Channels: 1, Samples: samples}
}

// pieceTestZero silences the samples from startMS for lengthMS.
func pieceTestZero(a DecodedAudio, startMS, lengthMS int) {
	for i := startMS * pieceTestRate / 1000; i < (startMS+lengthMS)*pieceTestRate/1000; i++ {
		a.Samples[i] = 0
	}
}

func pieceTestCheck(a DecodedAudio) []string {
	return CheckPiece(a, ComputeEnvelope(a))
}

func TestCheckPieceDropout(t *testing.T) {
	// A 0.1 sine is -23 dBFS. 100 ms of digital silence in the middle falls 57 dB in one step and stays under -80 dBFS.
	a := pieceTestSpeech(2, 0.1)
	pieceTestZero(a, 1000, 100)
	if got := pieceTestCheck(a); !reflect.DeepEqual(got, []string{"dropout"}) {
		t.Errorf("a 100 ms hole: %v", got)
	}
}

func TestCheckPieceDropoutIgnoresWhatSpeechDoesNaturally(t *testing.T) {
	clean := pieceTestSpeech(2, 0.1)
	short := pieceTestSpeech(2, 0.1)
	pieceTestZero(short, 1000, 30)
	quiet := pieceTestSpeech(2, 0.1)
	for i := 1000 * pieceTestRate / 1000; i < 1100*pieceTestRate/1000; i++ {
		quiet.Samples[i] *= 0.01 // -63 dBFS: a fall of 40 dB, but not below -80 dBFS
	}
	// Speech that ends in digital silence is an ending, not a hole: nothing speaks afterwards.
	ending := pieceTestSpeech(2, 0.1)
	pieceTestZero(ending, 1500, 500)
	// A hole that lasts 3.5 s is a pause, not a dropout.
	long := pieceTestSpeech(8, 0.1)
	pieceTestZero(long, 2000, 3500)
	for name, a := range map[string]DecodedAudio{"clean": clean, "30 ms hole": short, "shallow hole": quiet, "ending": ending, "long pause": long} {
		if got := pieceTestCheck(a); len(got) != 0 {
			t.Errorf("%s: %v", name, got)
		}
	}
}
