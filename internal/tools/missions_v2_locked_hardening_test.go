package tools

import (
	"errors"
	"testing"
)

// TestC17DeletingALockedMissionIsErrMissionLocked pins the sentinel the flow API maps to
// FLOW_LOCKED (409) and the unchanged text other callers show.
func TestC17DeletingALockedMissionIsErrMissionLocked(t *testing.T) {
	mm, _, _ := newFlowTestManager(t)
	if err := mm.ApplySyncedMission(&MissionV2{ID: "mission_c17_locked", Name: "Locked", Prompt: "x",
		ExecutionType: ExecutionManual, Enabled: true, Locked: true}); err != nil {
		t.Fatalf("ApplySyncedMission: %v", err)
	}
	err := mm.Delete("mission_c17_locked")
	if !errors.Is(err, ErrMissionLocked) || err.Error() != "mission is locked" {
		t.Fatalf("Delete of a locked mission = %v", err)
	}
	if _, ok := mm.Get("mission_c17_locked"); !ok {
		t.Fatal("the locked mission was deleted")
	}

	flow := publishTestFlow(t, mm, FlowTriggerSpec{NodeID: "n_aaaaaaaa", TriggerType: FlowTriggerManual})
	mm.mu.Lock()
	mm.missions[flow].Locked = true
	mm.mu.Unlock()
	if err := mm.DeleteFlowMission(flow); !errors.Is(err, ErrMissionLocked) {
		t.Fatalf("DeleteFlowMission of a locked flow mission = %v", err)
	}
	if err := mm.DeleteWithOptions(flow, DeleteMissionOptions{ForceRemote: true}); !errors.Is(err, ErrMissionLocked) {
		t.Fatalf("DeleteWithOptions of a locked flow mission = %v", err)
	}
}
