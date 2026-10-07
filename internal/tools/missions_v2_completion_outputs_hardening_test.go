package tools

import (
	"encoding/json"
	"strings"
	"testing"
)

// FF1: a flow's outputs reach mission_completed dependents of AGENT missions capped at
// agentCompletionOutputsMaxBytes encoded, with a truncation marker, because that data goes
// into the agent's prompt, its history row and the persisted queue. Flow dependents keep
// the outputs as boundedCompletionOutputs made them, and small outputs pass unchanged.
func TestFF1AgentDependentsGetCappedFlowOutputs(t *testing.T) {
	mm, hooks, _ := newFlowTestManager(t)
	source := publishTestFlow(t, mm, FlowTriggerSpec{NodeID: "n_aaaaaaaa", TriggerType: FlowTriggerManual})
	mm.mu.Lock()
	mm.missions["ff1_agent"] = &MissionV2{ID: "ff1_agent", Name: "Agent", Prompt: "p", ExecutionType: ExecutionTriggered,
		TriggerType: TriggerMissionCompleted, TriggerConfig: &TriggerConfig{SourceMissionID: source},
		Enabled: true, Priority: "medium", Status: MissionStatusIdle}
	mm.mu.Unlock()
	follower, err := mm.CreateFlowMission("flow_bbbbbbbbbb", "Folge")
	if err != nil {
		t.Fatal(err)
	}
	if err := mm.SyncFlowMission(follower, "Folge", []FlowTriggerSpec{{NodeID: "n_bbbbbbbb", TriggerType: TriggerMissionCompleted,
		TriggerConfig: &TriggerConfig{SourceMissionID: source}}}); err != nil {
		t.Fatal(err)
	}
	if err := mm.SetFlowMissionEnabled(follower, true); err != nil {
		t.Fatal(err)
	}

	// About 33 KB of encoded outputs (each `ab"c<` encodes to 11 bytes): under the 64 KiB
	// flow cap, over the 8 KiB agent cap. The escapes grow again when the preview is encoded.
	big := map[string]any{"report": strings.Repeat(`ab"c<`, 3<<10)}
	run := mm.FlowRunStarted(source, "manual", "")
	mm.FlowRunFinished(source, run, MissionResultSuccess, "Fertig.", big)

	items := mm.queue.List()
	if len(items) != 1 || items[0].MissionID != "ff1_agent" {
		t.Fatalf("queued dependents = %+v", items)
	}
	var agentData struct {
		Output  string          `json:"output"`
		Outputs json.RawMessage `json:"outputs"`
	}
	if err := json.Unmarshal([]byte(items[0].TriggerData), &agentData); err != nil {
		t.Fatal(err)
	}
	if agentData.Output != "Fertig." || len(agentData.Outputs) > agentCompletionOutputsMaxBytes {
		t.Fatalf("agent outputs: %d bytes (cap %d), output %q", len(agentData.Outputs), agentCompletionOutputsMaxBytes, agentData.Output)
	}
	var capped struct {
		Truncated bool   `json:"_truncated"`
		Preview   string `json:"_preview"`
	}
	if err := json.Unmarshal(agentData.Outputs, &capped); err != nil || !capped.Truncated || !strings.HasPrefix(capped.Preview, `{"report":"ab`) {
		t.Fatalf("agent outputs = %.200s (%v)", agentData.Outputs, err)
	}

	c := hooks.waitStart(t)
	var flowData struct {
		Outputs map[string]any `json:"outputs"`
	}
	if err := json.Unmarshal([]byte(c.data), &flowData); err != nil {
		t.Fatal(err)
	}
	if c.missionID != follower || flowData.Outputs["report"] != big["report"] {
		t.Fatalf("the flow dependent did not get the full outputs: %s %.100v", c.missionID, flowData.Outputs)
	}

	// Small outputs reach agent dependents unchanged.
	mm.mu.Lock()
	mm.missions["ff1_agent"].Status = MissionStatusIdle
	mm.queue = NewMissionQueue()
	mm.mu.Unlock()
	run = mm.FlowRunStarted(source, "manual", "")
	mm.FlowRunFinished(source, run, MissionResultSuccess, "ok", map[string]any{"report": "klein"})
	if items := mm.queue.List(); len(items) != 1 || !strings.Contains(items[0].TriggerData, `"outputs":{"report":"klein"}`) {
		t.Fatalf("small outputs = %+v", items)
	}
	hooks.waitStart(t)
}

// FF1 review: outputs that boundedCompletionOutputs already cut ({"_truncated": true,
// "_preview": …}) and that still exceed the agent cap once encoded (a preview full of
// escaped quotes doubles) get their preview cut, never a preview of the marker object.
func TestFF1AgentCapCutsAnExistingPreview(t *testing.T) {
	outputs := boundedCompletionOutputs(map[string]any{"report": strings.Repeat(`"`, 70<<10)})
	if len(outputs) <= agentCompletionOutputsMaxBytes {
		t.Fatalf("the flow cap left %d bytes; the test needs more than %d", len(outputs), agentCompletionOutputsMaxBytes)
	}
	capped := capCompletionOutputsForAgents(outputs)
	var marker struct {
		Truncated bool   `json:"_truncated"`
		Preview   string `json:"_preview"`
	}
	if err := json.Unmarshal(capped, &marker); err != nil || len(capped) > agentCompletionOutputsMaxBytes || !marker.Truncated {
		t.Fatalf("capped = %d bytes, %v", len(capped), err)
	}
	if !strings.HasPrefix(marker.Preview, `{"report":"\"`) || strings.Contains(marker.Preview, "_truncated") {
		t.Fatalf("preview = %.80q", marker.Preview)
	}
}
