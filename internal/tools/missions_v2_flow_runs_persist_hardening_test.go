package tools

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// ff1PersistedMission reads a mission as the missions file holds it.
func ff1PersistedMission(t *testing.T, dir, id string) MissionV2 {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, "missions_v2.json"))
	if err != nil {
		t.Fatal(err)
	}
	var missions []MissionV2
	if err := json.Unmarshal(data, &missions); err != nil {
		t.Fatal(err)
	}
	for _, m := range missions {
		if m.ID == id {
			return m
		}
	}
	t.Fatalf("mission %s is not in the missions file", id)
	return MissionV2{}
}

// FF1: a flow run's start does not rewrite the missions file, and no save writes a flow
// mission's running state: Mission Control reads the state from memory, Start sets flow
// missions idle anyway, and an older AuraGo would re-queue a persisted running mission as
// an agent mission with an empty prompt. The finish still persists the result.
func TestFF1FlowRunningStateIsNotPersisted(t *testing.T) {
	dir := tempSystemTaskDir(t)
	mm := NewMissionManagerV2(dir, nil)
	id := publishTestFlow(t, mm, FlowTriggerSpec{NodeID: "n_aaaaaaaa", TriggerType: FlowTriggerManual})
	file := filepath.Join(dir, "missions_v2.json")
	before, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}

	mm.FlowRunStarted(id, "manual", "")
	if m, _ := mm.Get(id); m.Status != MissionStatusRunning {
		t.Fatalf("in-memory status = %q, want running", m.Status)
	}
	after, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("FlowRunStarted rewrote the missions file")
	}

	// Another change saves the file while the flow runs: the flow mission is written idle,
	// while a running agent mission (the negative control) is still written as running.
	if err := mm.Create(&MissionV2{ID: "mission_ff1_agent", Name: "Agent", Prompt: "x", ExecutionType: ExecutionManual}); err != nil {
		t.Fatal(err)
	}
	mm.mu.Lock()
	mm.missions["mission_ff1_agent"].Status = MissionStatusRunning
	mm.mu.Unlock()
	if err := mm.Create(&MissionV2{ID: "mission_ff1_other", Name: "Andere", Prompt: "x", ExecutionType: ExecutionManual}); err != nil {
		t.Fatal(err)
	}
	if got := ff1PersistedMission(t, dir, id); got.Status != MissionStatusIdle {
		t.Fatalf("persisted status while running = %q, want idle", got.Status)
	}
	if got := ff1PersistedMission(t, dir, "mission_ff1_agent"); got.Status != MissionStatusRunning {
		t.Fatalf("a running agent mission was persisted as %q", got.Status)
	}
	if m, _ := mm.Get(id); m.Status != MissionStatusRunning {
		t.Fatalf("the save changed the in-memory status to %q", m.Status)
	}

	mm.FlowRunFinishedAtDepth(id, "", MissionResultSuccess, "fertig", nil, 0)
	got := ff1PersistedMission(t, dir, id)
	if got.Status != MissionStatusIdle || got.LastResult != MissionResultSuccess || got.RunCount != 1 || got.LastRun.IsZero() {
		t.Fatalf("persisted after the finish = status %q, result %q, runs %d, last run %v",
			got.Status, got.LastResult, got.RunCount, got.LastRun)
	}
}
