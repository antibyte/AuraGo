package ui

import (
	"strings"
	"testing"
)

func TestGalaxaMusicRecoversFromRejectedInitialPlayback(t *testing.T) {
	t.Parallel()

	audio := readEmbeddedText(t, "js/desktop/apps/galaxa-audio.js")
	for _, marker := range []string{
		"_playPending: null",
		"this._playPending = pending",
		"pending.then(() => {",
		"if (this._playPending !== pending) return",
		"(!fromGesture && this._playPending)",
		"this._playing = false",
		"err.name === 'NotAllowedError'",
		"resumeFromGesture()",
		"this.play(true)",
	} {
		if !strings.Contains(audio, marker) {
			t.Fatalf("Galaxa music recovery missing marker %q", marker)
		}
	}

	playBody := sectionBetween(t, audio, "play(fromGesture) {", "resumeFromGesture() {")
	if strings.Contains(playBody, "p.catch(() => {})") || strings.Contains(playBody, "this._playing = true; } catch") {
		t.Fatal("Galaxa music must not report playback before the play promise resolves")
	}

	game := readEmbeddedText(t, "js/desktop/apps/galaxa-game.js")
	for _, handler := range []string{"function onKey(e)", "function onTouchStart(e)"} {
		body := sectionBetween(t, game, handler, "\n        }")
		if !strings.Contains(body, "ctx.GalagaMusic.resumeFromGesture()") {
			t.Fatalf("%s must retry blocked music inside the user gesture", handler)
		}
	}
}
