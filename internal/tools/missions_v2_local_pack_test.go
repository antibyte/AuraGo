package tools

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestCreateFlowMissionRejectsDuplicateOwnerIncludingOrphans(t *testing.T) {
	mm, _, _ := newFlowTestManager(t)
	const flowID = "flow_aaaaaaaaaa"
	first, err := mm.CreateFlowMission(flowID, "Draft")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := mm.CreateFlowMission(flowID, "Retry"); !errors.Is(err, ErrFlowMissionOwnerConflict) {
		t.Fatalf("duplicate owner = %v", err)
	}
	if got, ok := mm.Get(first); !ok || !isFlowMission(got) || got.FlowID != flowID || got.Enabled {
		t.Fatalf("the retained disabled orphan changed: %+v, %t", got, ok)
	}
	if got := len(mm.List()); got != 1 {
		t.Fatalf("mission count = %d, want 1", got)
	}
	t.Run("non-flow retained claim", func(t *testing.T) {
		mm, _, _ := newFlowTestManager(t)
		mm.mu.Lock()
		mm.missions["legacy_claim"] = &MissionV2{ID: "legacy_claim", FlowID: flowID, ExecutionType: ExecutionManual, Status: MissionStatusIdle}
		mm.mu.Unlock()
		if _, err := mm.CreateFlowMission(flowID, "Draft"); !errors.Is(err, ErrFlowMissionOwnerConflict) {
			t.Fatalf("non-flow retained claim = %v", err)
		}
		if got := len(mm.List()); got != 1 {
			t.Fatalf("retained claim count = %d, want 1", got)
		}
	})
}

func TestDeleteCreatedFlowMissionRequiresExactUnchangedOwner(t *testing.T) {
	t.Run("wrong owner", func(t *testing.T) {
		mm, _, _ := newFlowTestManager(t)
		id, err := mm.CreateFlowMission("flow_aaaaaaaaaa", "Draft")
		if err != nil {
			t.Fatal(err)
		}
		if err := mm.DeleteCreatedFlowMission(id, "flow_bbbbbbbbbb"); !errors.Is(err, ErrFlowMissionOwnerMismatch) {
			t.Fatalf("wrong owner cleanup = %v", err)
		}
		if _, ok := mm.Get(id); !ok {
			t.Fatal("wrong owner cleanup removed the mission")
		}
	})
	for _, mutate := range []struct {
		name string
		fn   func(*MissionV2)
	}{
		{"enabled", func(m *MissionV2) { m.Enabled = true }},
		{"published", func(m *MissionV2) { m.FlowPublished = true }},
		{"has triggers", func(m *MissionV2) { m.FlowTriggers = []FlowTriggerSpec{{NodeID: "n_aaaaaaaa"}} }},
		{"wrong execution type", func(m *MissionV2) { m.ExecutionType = ExecutionManual }},
	} {
		t.Run(mutate.name, func(t *testing.T) {
			mm, _, _ := newFlowTestManager(t)
			const flowID = "flow_aaaaaaaaaa"
			id, err := mm.CreateFlowMission(flowID, "Draft")
			if err != nil {
				t.Fatal(err)
			}
			mm.mu.Lock()
			mutate.fn(mm.missions[id])
			mm.mu.Unlock()
			if err := mm.DeleteCreatedFlowMission(id, flowID); !errors.Is(err, ErrFlowMissionCleanupRefused) && !errors.Is(err, ErrFlowMissionOwnerMismatch) {
				t.Fatalf("changed mission cleanup = %v", err)
			}
			if _, ok := mm.Get(id); !ok {
				t.Fatal("changed mission was removed")
			}
		})
	}
}

func TestDeleteCreatedFlowMissionRestoresMemoryOnSaveFailure(t *testing.T) {
	mm, _, _ := newFlowTestManager(t)
	const flowID = "flow_aaaaaaaaaa"
	id, err := mm.CreateFlowMission(flowID, "Draft")
	if err != nil {
		t.Fatal(err)
	}
	mm.file = filepath.Join(t.TempDir(), "missing", "missions.json")
	if err := mm.DeleteCreatedFlowMission(id, flowID); err == nil {
		t.Fatal("cleanup with an unwritable state path succeeded")
	}
	if got, ok := mm.Get(id); !ok || !isFlowMission(got) || got.FlowID != flowID || got.Enabled {
		t.Fatalf("failed cleanup did not restore the disabled owner: %+v, %t", got, ok)
	}
}
