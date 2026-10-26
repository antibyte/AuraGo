package server

import (
	"strings"
	"testing"

	"aurago/internal/flows"
	"aurago/internal/tools"
)

// R1: a queued run the runner's gate stopped (FLOW_RUNS_PAUSED, never started) is noted on the
// flow mission like a refused trigger; a run cancelled while queued is not.
func TestB3BridgeNotesAPausedQueuedRun(t *testing.T) {
	s := c16StartedServer(t, nil)
	rec := c16PublishedFlow(t, s, greetFlowJSON)
	bridge := flowMissionBridge{s: s}
	unstarted := func(code string) flows.RunFinishedInfo {
		return flows.RunFinishedInfo{MissionID: rec.MissionID, Record: flows.RunRecord{ID: "run_b3", FlowID: rec.ID, Mode: flows.ModeLive},
			Result: flows.RunResult{Status: flows.RunCancelled, ErrorCode: code}, Outputs: map[string]any{}}
	}
	bridge.FlowRunFinished(unstarted("FLOW_CANCELLED"))
	if m, _ := s.MissionManagerV2.Get(rec.MissionID); m.LastResult != "" {
		t.Fatalf("a run cancelled while queued noted the mission: %+v", m)
	}
	s.Cfg.Tools.Missions.ReadOnly = true
	bridge.FlowRunFinished(unstarted(flows.RunsPausedCode))
	m, _ := s.MissionManagerV2.Get(rec.MissionID)
	if m.LastResult != tools.MissionResultError || !strings.Contains(m.LastOutput, "paused") || m.RunCount != 0 {
		t.Fatalf("mission after a paused queued run: %+v", m)
	}
}
