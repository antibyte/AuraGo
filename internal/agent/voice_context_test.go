package agent

import (
	"context"
	"testing"
)

func TestVoiceOutputSuppressionIsRequestLocal(t *testing.T) {
	plain := context.Background()
	suppressed := WithVoiceOutputSuppressed(plain)
	if VoiceOutputSuppressed(plain) {
		t.Fatal("plain context was modified")
	}
	if !VoiceOutputSuppressed(suppressed) {
		t.Fatal("suppression marker was not preserved")
	}
	if !VoiceOutputSuppressed(WithVoiceOutputSuppressed(suppressed)) {
		t.Fatal("nested suppression marker was lost")
	}
}
