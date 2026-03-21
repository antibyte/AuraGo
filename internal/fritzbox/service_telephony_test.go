package fritzbox

import "testing"

func TestTamAudioURLCandidates_AddsWAVVariantForPathWithoutExtension(t *testing.T) {
	raw := "http://fritz.box/download.lua?path=/data/tam/rec/rec.0.001"
	got := tamAudioURLCandidates(raw)
	if len(got) != 2 {
		t.Fatalf("expected 2 candidates, got %d: %#v", len(got), got)
	}
	if got[0] != raw {
		t.Fatalf("first candidate mismatch: %q", got[0])
	}
	wantSecond := "http://fritz.box/download.lua?path=%2Fdata%2Ftam%2Frec%2Frec.0.001.wav"
	if got[1] != wantSecond {
		t.Fatalf("second candidate mismatch: got %q want %q", got[1], wantSecond)
	}
}

func TestTamAudioURLCandidates_DoesNotDuplicateWhenExtensionExists(t *testing.T) {
	raw := "http://fritz.box/download.lua?path=/data/tam/rec/rec.0.001.wav"
	got := tamAudioURLCandidates(raw)
	if len(got) != 1 {
		t.Fatalf("expected 1 candidate, got %d: %#v", len(got), got)
	}
	if got[0] != raw {
		t.Fatalf("candidate mismatch: %q", got[0])
	}
}
