package speechlab

import (
	"encoding/binary"
	"math"
	"testing"
	"time"
)

func pcm16WAVWithSamples(samples []int16) []byte {
	data := make([]byte, 44+len(samples)*2)
	copy(data[0:4], "RIFF")
	binary.LittleEndian.PutUint32(data[4:8], uint32(len(data)-8))
	copy(data[8:12], "WAVE")
	copy(data[12:16], "fmt ")
	binary.LittleEndian.PutUint32(data[16:20], 16)
	binary.LittleEndian.PutUint16(data[20:22], 1)
	binary.LittleEndian.PutUint16(data[22:24], 1)
	binary.LittleEndian.PutUint32(data[24:28], 16000)
	binary.LittleEndian.PutUint32(data[28:32], 32000)
	binary.LittleEndian.PutUint16(data[32:34], 2)
	binary.LittleEndian.PutUint16(data[34:36], 16)
	copy(data[36:40], "data")
	binary.LittleEndian.PutUint32(data[40:44], uint32(len(samples)*2))
	for i, sample := range samples {
		binary.LittleEndian.PutUint16(data[44+i*2:46+i*2], uint16(sample))
	}
	return data
}

func TestAnalyzePCM16WAVReturnsPrivacySafeSignalMetrics(t *testing.T) {
	wav := pcm16WAVWithSamples([]int16{0, 16384, -32768, 8192})
	metrics, err := AnalyzePCM16WAV(wav)
	if err != nil {
		t.Fatal(err)
	}
	if metrics.SampleCount != 4 || metrics.Duration != 250*time.Microsecond {
		t.Fatalf("unexpected duration metrics: %+v", metrics)
	}
	if math.Abs(metrics.PeakLevel-1) > 1e-9 {
		t.Fatalf("peak level = %f, want 1", metrics.PeakLevel)
	}
	wantRMS := math.Sqrt((0.25 + 1 + 0.0625) / 4)
	if math.Abs(metrics.RMSLevel-wantRMS) > 1e-9 {
		t.Fatalf("RMS level = %f, want %f", metrics.RMSLevel, wantRMS)
	}
}

func TestAnalyzePCM16WAVRejectsIncompleteSample(t *testing.T) {
	wav := pcm16WAVWithSamples([]int16{1})
	wav = append(wav, 0)
	binary.LittleEndian.PutUint32(wav[4:8], uint32(len(wav)-8))
	binary.LittleEndian.PutUint32(wav[40:44], 3)
	if _, err := AnalyzePCM16WAV(wav); err == nil {
		t.Fatal("odd PCM16 data length was accepted")
	}
}
