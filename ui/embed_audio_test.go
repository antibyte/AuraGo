package ui

import (
	"io/fs"
	"testing"
)

func TestEmbedContainsGalagaMp3(t *testing.T) {
	f, err := fs.ReadFile(Content, "img/audio/galaga.mp3")
	if err != nil {
		t.Fatalf("galaga.mp3 must be embedded at img/audio/galaga.mp3: %v", err)
	}
	if len(f) < 100000 {
		t.Fatalf("galaga.mp3 too small: %d bytes", len(f))
	}
	t.Logf("galaga.mp3 embedded: %d bytes", len(f))
}
