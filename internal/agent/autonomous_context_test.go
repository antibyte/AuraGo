package agent

import (
	"testing"

	"aurago/internal/prompts"
)

func TestIsAutonomousAgentRunRecognizesHeartbeat(t *testing.T) {
	tests := []struct {
		name      string
		runCfg    RunConfig
		sessionID string
		want      bool
	}{
		{name: "heartbeat source", runCfg: RunConfig{MessageSource: "heartbeat"}, sessionID: "default", want: true},
		{name: "heartbeat session", runCfg: RunConfig{}, sessionID: "heartbeat", want: true},
		{name: "web chat default", runCfg: RunConfig{MessageSource: "web_chat"}, sessionID: "default", want: false},
		{name: "mission is handled separately", runCfg: RunConfig{MessageSource: "mission", IsMission: true}, sessionID: "mission-1", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isAutonomousAgentRun(tt.runCfg, tt.sessionID); got != tt.want {
				t.Fatalf("isAutonomousAgentRun(%+v, %q) = %v, want %v", tt.runCfg, tt.sessionID, got, tt.want)
			}
		})
	}
}

func TestShouldRunTurnSideEffectsBlocksHeartbeatArtifacts(t *testing.T) {
	if shouldRunTurnSideEffects(RunConfig{MessageSource: "heartbeat"}, "heartbeat", prompts.ContextFlags{}) {
		t.Fatal("heartbeat runs must not write activity, memory analysis, journal, or reuse artifacts")
	}
	if shouldRunTurnSideEffects(RunConfig{IsMission: true, MessageSource: "mission"}, "mission-1", prompts.ContextFlags{IsMission: true}) {
		t.Fatal("mission runs must not write normal chat side effects")
	}
	if !shouldRunTurnSideEffects(RunConfig{MessageSource: "web_chat"}, "default", prompts.ContextFlags{}) {
		t.Fatal("regular web chat should keep normal side effects")
	}
	if shouldRunTurnSideEffects(RunConfig{IsMaintenance: true, MessageSource: "maintenance"}, "maintenance", prompts.ContextFlags{}) {
		t.Fatal("maintenance runs must not write normal chat side effects")
	}
	if shouldRunTurnSideEffects(RunConfig{MessageSource: "maintenance"}, "maintenance", prompts.ContextFlags{}) {
		t.Fatal("maintenance runs must not write normal chat side effects")
	}
}
