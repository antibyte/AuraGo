package cyd

import (
	"encoding/binary"
	"strings"
	"testing"

	"aurago/internal/security"
)

// The glass speaker turns notification text into audio that leaves AuraGo, so a
// registered secret in the text must reach the synthesizer already redacted.
func TestSpeakerRenderScrubsRegisteredSecrets(t *testing.T) {
	const secret = "cyd-speech-secret-0123456789"
	defer security.RegisterScopedSensitiveExact(secret)()

	data := make([]byte, 160*2)
	for i := 0; i < 160; i++ {
		binary.LittleEndian.PutUint16(data[i*2:], uint16(int16(12000*(i%10-5)/5)))
	}
	wav := makeWav(1, 16, 8000, data)

	var got string
	s := NewSpeaker()
	s.SetSynth(func(text string) ([]byte, error) {
		got = text
		return wav, nil
	}, func() bool { return true })

	s.render("ntf_scrub", "Code "+secret+" ende")

	if strings.Contains(got, secret) {
		t.Fatalf("synthesizer received the registered secret: %q", got)
	}
	if want := "Code [redacted] ende"; got != want {
		t.Fatalf("synthesizer text = %q, want %q", got, want)
	}
	if len(s.Get("ntf_scrub")) == 0 {
		t.Fatal("scrubbed text produced no clip")
	}
}
