package cyd

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestSpeakerUnavailableWarningOnce(t *testing.T) {
	var output bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&output, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })
	var absent *Speaker
	absent.warnUnavailable()
	available := &Speaker{bin: "configured"}
	available.warnUnavailable()
	if output.Len() != 0 {
		t.Fatal("nil or available speaker emitted a warning")
	}
	speaker := &Speaker{}
	speaker.warnUnavailable()
	speaker.warnUnavailable()
	if count := strings.Count(output.String(), "sanoTTS executable not found"); count != 1 {
		t.Fatalf("warnings = %d, want 1", count)
	}
}
