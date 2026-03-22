package tools

import (
	"testing"
)

func TestPiperContainerName(t *testing.T) {
	if piperContainerName != "aurago-piper-tts" {
		t.Errorf("expected container name 'aurago-piper-tts', got %q", piperContainerName)
	}
}

func TestPiperHealthDisabled(t *testing.T) {
	// Port 0 should fail gracefully (no container listening)
	health := PiperHealth(0)
	if health["status"] == "running" {
		t.Error("expected non-running status for port 0")
	}
}

func TestPiperListVoicesNoContainer(t *testing.T) {
	// Calling with an invalid port should return an error
	_, err := PiperListVoices(1) // Port 1 is unlikely to have Piper listening
	if err == nil {
		t.Error("expected error when connecting to non-existent Piper on port 1")
	}
}
