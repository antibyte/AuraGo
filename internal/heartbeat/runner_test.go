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
		`manage_todos using operation="list" and status="open"`,
		"manage_notes only for scratch notes",
		"Never invent tool names",
		"run_tool is only for saved custom Python tools",
		"discover_tools/get_tool_info",
		"Do not call send_telegram",
		"skip notifications entirely",
		"Do not edit homepage or project files",
		"do not build or deploy websites",
		"report the issue instead",
	} {
		if !strings.Contains(prompt, required) {
			t.Fatalf("heartbeat prompt should contain %q, got:\n%s", required, prompt)
		}
	}
}
