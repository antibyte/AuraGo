package heartbeat

import (
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
)

func TestHeartbeatPromptPreventsAutonomousProjectChanges(t *testing.T) {
	prompt := buildHeartbeatPrompt(config.HeartbeatConfig{CheckTasks: true}, time.Date(2026, 4, 26, 7, 27, 0, 0, time.UTC))

	for _, required := range []string{
		"Do not edit homepage or project files",
		"do not build or deploy websites",
		"report the issue instead",
	} {
		if !strings.Contains(prompt, required) {
			t.Fatalf("heartbeat prompt should contain %q, got:\n%s", required, prompt)
		}
	}
}
