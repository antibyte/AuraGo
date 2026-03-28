package fritzbox

import "testing"

func TestTamAudioURLCandidates_AddsWAVVariantForPathWithoutExtension(t *testing.T) {
	raw := "http://fritz.box/download.lua?path=/data/tam/rec/rec.0.001"
	got := tamAudioURLCandidates(raw, "")
	if len(got) != 3 {
		t.Fatalf("expected 3 candidates, got %d: %#v", len(got), got)
	}
	if got[0] != raw {
		t.Fatalf("first candidate mismatch: %q", got[0])
	}
	wantWAV := "http://fritz.box/download.lua?path=/data/tam/rec/rec.0.001.wav"
	if got[1] != wantWAV {
		t.Fatalf("second candidate mismatch: got %q want %q", got[1], wantWAV)
	}
	wantWAVUpper := "http://fritz.box/download.lua?path=/data/tam/rec/rec.0.001.WAV"
	if got[2] != wantWAVUpper {
		t.Fatalf("third candidate mismatch: got %q want %q", got[2], wantWAVUpper)
	}
}

func TestTamAudioURLCandidates_DoesNotDuplicateWhenExtensionExists(t *testing.T) {
	raw := "http://fritz.box/download.lua?path=/data/tam/rec/rec.0.001.wav"
	got := tamAudioURLCandidates(raw, "")
	if len(got) != 1 {
		t.Fatalf("expected 1 candidate, got %d: %#v", len(got), got)
	}
	if got[0] != raw {
		t.Fatalf("candidate mismatch: %q", got[0])
	}
}
