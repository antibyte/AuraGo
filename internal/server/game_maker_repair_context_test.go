package server

import (
	"encoding/json"
	"strings"
	"testing"

	"aurago/internal/gamemaker"
)

func TestGameMakerRepairContextRetainsFailedSteps(t *testing.T) {
	run := gamemaker.JobRun{Stage: "repair", Checks: []gamemaker.CheckResult{
		{ID: "required_move", Status: "passed", Steps: []gamemaker.GameTestStep{{Action: "key", Key: "RIGHT", MS: 300}}},
		{ID: "required_hit", Status: "failed", Expected: "hits increased", Observed: "before=0 after=0", Steps: []gamemaker.GameTestStep{{Action: "target", Target: "enemy", Mode: "aim", MS: 1200}, {Action: "key", Key: "SPACE", MS: 800}}},
	}, Diagnostics: []gamemaker.Diagnostic{{Level: "warning", Message: "optional decoration"}}}
	context := compactGameMakerContext(run)
	checks := context["checks"].([]map[string]any)
	if checks[0]["steps"] != nil || len(checks[1]["steps"].([]gamemaker.GameTestStep)) != 2 {
		t.Fatalf("repair steps were lost or passing steps expanded: %+v", checks)
	}
	packet := context["repair_packet"].(map[string]any)
	failure := packet["first_failure"].(map[string]any)
	if failure["id"] != "required_hit" || len(failure["steps"].([]gamemaker.GameTestStep)) != 2 {
		t.Fatalf("warning obscured failed check: %+v", packet)
	}
	encoded, _ := json.Marshal(packet)
	if !strings.Contains(string(encoded), `"preserve_passing_checks":["required_move"]`) || !strings.Contains(string(encoded), `"ms":1200`) {
		t.Fatalf("incomplete repair evidence: %s", encoded)
	}
}
