package agent

import "testing"

func TestShouldPersistLoopbackHistory(t *testing.T) {
	tests := []struct {
		name      string
		sessionID string
		want      bool
	}{
		{name: "default session", sessionID: "default", want: true},
		{name: "heartbeat session", sessionID: "heartbeat", want: false},
		{name: "custom chat session", sessionID: "sess-123", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldPersistLoopbackHistory(tt.sessionID); got != tt.want {
				t.Fatalf("shouldPersistLoopbackHistory(%q) = %v, want %v", tt.sessionID, got, tt.want)
			}
		})
	}
}

func TestIsAutonomousLoopback(t *testing.T) {
	tests := []struct {
		name      string
		runCfg    RunConfig
		sessionID string
		want      bool
	}{
		{name: "heartbeat source", runCfg: RunConfig{MessageSource: "heartbeat"}, sessionID: "default", want: true},
		{name: "heartbeat session", runCfg: RunConfig{}, sessionID: "heartbeat", want: true},
		{name: "sms loopback remains visible", runCfg: RunConfig{MessageSource: "sms"}, sessionID: "default", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isAutonomousLoopback(tt.runCfg, tt.sessionID); got != tt.want {
				t.Fatalf("isAutonomousLoopback(%+v, %q) = %v, want %v", tt.runCfg, tt.sessionID, got, tt.want)
			}
		})
	}
}
