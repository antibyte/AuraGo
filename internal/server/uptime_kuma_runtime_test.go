package server

import (
	"strings"
	"testing"

	"aurago/internal/tools"
)

func TestFormatUptimeKumaTransitionPromptIncludesRelayInstruction(t *testing.T) {
	event := tools.UptimeKumaTransition{
		Event:          "DOWN",
		PreviousStatus: "up",
		CurrentStatus:  "down",
		Monitor: tools.UptimeKumaMonitorSnapshot{
			MonitorName:    "Main Website",
			MonitorType:    "http",
			MonitorURL:     "https://example.com",
			ResponseTimeMS: 321,
		},
	}

	prompt := formatUptimeKumaTransitionPrompt(event, "Try restarting the service once, then notify me.")

	for _, want := range []string{
		"[UPTIME KUMA EVENT: DOWN]",
		"Monitor: Main Website",
		"Target: https://example.com",
		"Configured outage instruction from the user:",
		"Try restarting the service once, then notify me.",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q:\n%s", want, prompt)
		}
	}
}

func TestFormatUptimeKumaTransitionPromptOmitsBlankRelayInstruction(t *testing.T) {
	event := tools.UptimeKumaTransition{
		Event:          "UP",
		PreviousStatus: "down",
		CurrentStatus:  "up",
		Monitor: tools.UptimeKumaMonitorSnapshot{
			MonitorName: "Main Website",
		},
	}

	prompt := formatUptimeKumaTransitionPrompt(event, "   ")
	if strings.Contains(prompt, "Configured outage instruction from the user:") {
		t.Fatalf("expected blank relay instruction to be omitted, got:\n%s", prompt)
	}
}
