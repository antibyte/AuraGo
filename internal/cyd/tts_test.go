package cyd

import (
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"testing"

	"aurago/internal/sanotts"
)

func TestSpeakerUsesManagedRuntime(t *testing.T) {
	dir := t.TempDir()
	bin := sanotts.CLI(dir)
	if err := os.MkdirAll(filepath.Dir(bin), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	s := NewSpeakerWithData(dir)
	if !s.Available() || s.bin != bin {
		t.Fatalf("managed bin=%q available=%v want %q", s.bin, s.Available(), bin)
	}
	wantCache := filepath.Join(dir, "sanotts", "voices")
	if s.cacheDir != wantCache {
		t.Fatalf("cacheDir=%q want %q", s.cacheDir, wantCache)
	}
}

func TestSpeakLineEnglish(t *testing.T) {
	got := SpeakLine("Backup failed", "disk /data 98%")
	if got != "Backup failed. disk data 98" {
		t.Fatalf("got %q", got)
	}
}

func TestMeshSpeakParts(t *testing.T) {
	title, body := MeshSpeakParts("Alice", "direct", "hello there", false)
	if title != "Mesh Alice" || body != "hello there" {
		t.Fatalf("open = %q %q", title, body)
	}
	title, body = MeshSpeakParts("Alice", "direct", "secret", true)
	if title != "Mesh Alice" || body != "locked" {
		t.Fatalf("protected = %q %q", title, body)
	}
	title, body = MeshSpeakParts("MeshCore", "", "", false)
	if title != "MeshCore" || body != "incoming" {
		t.Fatalf("empty = %q %q", title, body)
	}
	title, body = MeshSpeakParts("0123456789abcdef0123456789abcdef", "direct", "ping", false)
	if title != "MeshCore" || body != "ping" {
		t.Fatalf("key name = %q %q", title, body)
	}
}

func TestPCM8kFromWAVPeakNormalizesQuietSource(t *testing.T) {
	const from = 24000
	const n = 2400 // 100 ms
	data := make([]byte, n*2)
	for i := 0; i < n; i++ {
		// Quiet sanoTTS-like peak (~8000 of 32767), not full scale.
		v := int16(8000 * (i%20 - 10) / 10)
		binary.LittleEndian.PutUint16(data[i*2:], uint16(v))
	}
	pcm, err := PCM8kFromWAV(makeWav(1, 16, from, data))
	if err != nil {
		t.Fatal(err)
	}
	peak := 0
	for _, s := range pcm {
		d := int(s) - 128
		if d < 0 {
			d = -d
		}
		if d > peak {
			peak = d
		}
	}
	if peak < 80 {
		t.Fatalf("quiet source was not normalized, peakdev=%d", peak)
	}
}

func TestDownsampleAttenuatesAboveNyquist(t *testing.T) {
	const rate = 24000
	const n = 24000
	low := downsampleU8(sine16(rate, n, 1000, 12000), rate, SpeakRate)
	high := downsampleU8(sine16(rate, n, 6000, 12000), rate, SpeakRate)
	if rmsDev(high) > rmsDev(low)*0.5 {
		t.Fatalf("6 kHz not filtered: 1kHz rms=%.1f 6kHz rms=%.1f", rmsDev(low), rmsDev(high))
	}
}

func sine16(rate, n int, hz, amp float64) []int16 {
	out := make([]int16, n)
	for i := 0; i < n; i++ {
		out[i] = int16(amp * math.Sin(2*math.Pi*hz*float64(i)/float64(rate)))
	}
	return out
}

func rmsDev(p []uint8) float64 {
	if len(p) == 0 {
		return 0
	}
	var s float64
	for _, v := range p {
		d := float64(int(v) - 128)
		s += d * d
	}
	return math.Sqrt(s / float64(len(p)))
}

func TestPCM8kFromWAVDownsamples(t *testing.T) {
	const from = 24000
	const n = 240 // 10 ms
	data := make([]byte, n*2)
	for i := 0; i < n; i++ {
		binary.LittleEndian.PutUint16(data[i*2:], uint16(int16(i*20)))
	}
	wav := makeWav(1, 16, from, data)
	pcm, err := PCM8kFromWAV(wav)
	if err != nil {
		t.Fatal(err)
	}
	want := n / (from / SpeakRate)
	if len(pcm) != want {
		t.Fatalf("len=%d want %d", len(pcm), want)
	}
}

func makeWav(ch, bits, rate int, pcm []byte) []byte {
	buf := make([]byte, 44+len(pcm))
	copy(buf[0:4], "RIFF")
	binary.LittleEndian.PutUint32(buf[4:8], uint32(36+len(pcm)))
	copy(buf[8:12], "WAVE")
	copy(buf[12:16], "fmt ")
	binary.LittleEndian.PutUint32(buf[16:20], 16)
	binary.LittleEndian.PutUint16(buf[20:22], 1)
	binary.LittleEndian.PutUint16(buf[22:24], uint16(ch))
	binary.LittleEndian.PutUint32(buf[24:28], uint32(rate))
	byteRate := rate * ch * bits / 8
	binary.LittleEndian.PutUint32(buf[28:32], uint32(byteRate))
	binary.LittleEndian.PutUint16(buf[32:34], uint16(ch*bits/8))
	binary.LittleEndian.PutUint16(buf[34:36], uint16(bits))
	copy(buf[36:40], "data")
	binary.LittleEndian.PutUint32(buf[40:44], uint32(len(pcm)))
	copy(buf[44:], pcm)
	return buf
}
