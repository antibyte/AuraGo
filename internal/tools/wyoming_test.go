package tools

import (
	"testing"
)

func TestPCMToWAV(t *testing.T) {
	// 100 samples of silence (16-bit, mono, 22050 Hz)
	pcm := make([]byte, 200)
	rate := 22050
	width := 2
	channels := 1

	wav := PCMToWAV(pcm, rate, width, channels)

	// WAV header is 44 bytes
	if len(wav) != 44+len(pcm) {
		t.Fatalf("expected WAV length %d, got %d", 44+len(pcm), len(wav))
	}

	// Check RIFF header
	if string(wav[0:4]) != "RIFF" {
		t.Errorf("expected RIFF magic, got %q", string(wav[0:4]))
	}

	// Check WAVE format
	if string(wav[8:12]) != "WAVE" {
		t.Errorf("expected WAVE format, got %q", string(wav[8:12]))
	}

	// Check fmt chunk
	if string(wav[12:16]) != "fmt " {
		t.Errorf("expected fmt chunk, got %q", string(wav[12:16]))
	}

	// Check data chunk
	if string(wav[36:40]) != "data" {
		t.Errorf("expected data chunk, got %q", string(wav[36:40]))
	}

	// Check sample rate (bytes 24-27, little-endian)
	sampleRate := int(wav[24]) | int(wav[25])<<8 | int(wav[26])<<16 | int(wav[27])<<24
	if sampleRate != rate {
		t.Errorf("expected sample rate %d, got %d", rate, sampleRate)
	}

	// Check channels (bytes 22-23, little-endian)
	numChannels := int(wav[22]) | int(wav[23])<<8
	if numChannels != channels {
		t.Errorf("expected channels %d, got %d", channels, numChannels)
	}

	// Check bits per sample (bytes 34-35, little-endian)
	bitsPerSample := int(wav[34]) | int(wav[35])<<8
	if bitsPerSample != width*8 {
		t.Errorf("expected bits per sample %d, got %d", width*8, bitsPerSample)
	}
}

func TestPCMToWAVStereo(t *testing.T) {
	pcm := make([]byte, 400) // 100 stereo samples
	rate := 44100
	width := 2
	channels := 2

	wav := PCMToWAV(pcm, rate, width, channels)

	if len(wav) != 44+len(pcm) {
		t.Fatalf("expected WAV length %d, got %d", 44+len(pcm), len(wav))
	}

	numChannels := int(wav[22]) | int(wav[23])<<8
	if numChannels != 2 {
		t.Errorf("expected 2 channels, got %d", numChannels)
	}

	sampleRate := int(wav[24]) | int(wav[25])<<8 | int(wav[26])<<16 | int(wav[27])<<24
	if sampleRate != 44100 {
		t.Errorf("expected sample rate 44100, got %d", sampleRate)
	}
}

func TestPCMToWAVEmpty(t *testing.T) {
	pcm := []byte{}
	wav := PCMToWAV(pcm, 16000, 2, 1)

	if len(wav) != 44 {
		t.Fatalf("expected 44 bytes (header only), got %d", len(wav))
	}

	if string(wav[0:4]) != "RIFF" {
		t.Errorf("expected RIFF magic")
	}
}

func TestWyomingEventRoundTrip(t *testing.T) {
	// Test that WyomingEvent struct holds expected data
	evt := WyomingEvent{
		Type: "synthesize",
		Data: map[string]interface{}{
			"text":  "Hello world",
			"voice": map[string]interface{}{"name": "test-voice"},
		},
	}

	if evt.Type != "synthesize" {
		t.Errorf("expected type 'synthesize', got %q", evt.Type)
	}

	data, ok := evt.Data["text"]
	if !ok || data != "Hello world" {
		t.Errorf("expected text 'Hello world', got %v", data)
	}
}

func TestWyomingVoiceStruct(t *testing.T) {
	v := WyomingVoice{
		Name:        "de_DE-thorsten-high",
		Description: "German male voice",
		Languages:   []string{"de_DE"},
		Installed:   true,
		Version:     "1.0",
	}

	if v.Name != "de_DE-thorsten-high" {
		t.Errorf("unexpected name: %s", v.Name)
	}
	if len(v.Languages) != 1 || v.Languages[0] != "de_DE" {
		t.Errorf("unexpected languages: %v", v.Languages)
	}
	if !v.Installed {
		t.Error("expected installed=true")
	}
}
