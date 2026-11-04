package tools

import (
	"errors"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
)

func TestEnsureRestoredFlowMission(t *testing.T) {
	mm, _, _ := newFlowTestManager(t)
	const id, flowID = "mission_repair", "flow_repair"
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- mm.EnsureRestoredFlowMission(id, flowID, "Restored", nil, false)
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	mission, ok := mm.Get(id)
	if !ok || mission.FlowID != flowID || mission.Enabled || mission.FlowPublished {
		t.Fatalf("restored mission = %+v", mission)
	}
	if err := mm.EnsureRestoredFlowMission(id, "another_flow", "", nil, false); !errors.Is(err, ErrFlowMissionConflict) {
		t.Fatalf("foreign ID = %v", err)
	}
	if err := mm.EnsureRestoredFlowMission("another_id", flowID, "", nil, false); !errors.Is(err, ErrFlowMissionConflict) {
		t.Fatalf("duplicate flow owner = %v", err)
	}
	mm.mu.Lock()
	mm.missions[id].Enabled = true
	mm.mu.Unlock()
	if err := mm.EnsureRestoredFlowMission(id, flowID, "", nil, false); !errors.Is(err, ErrFlowMissionConflict) {
		t.Fatalf("enabled mission = %v", err)
	}
	mm.mu.Lock()
	mm.missions[id] = &MissionV2{ID: id, ExecutionType: ExecutionManual}
	mm.mu.Unlock()
	if err := mm.EnsureRestoredFlowMission(id, flowID, "", nil, false); !errors.Is(err, ErrFlowMissionConflict) {
		t.Fatalf("foreign mission type = %v", err)
	}
}

func TestEnsureRestoredFlowMissionRollsBackFailedSave(t *testing.T) {
	mm, _, _ := newFlowTestManager(t)
	const id, flowID = "mission_restore_failure", "flow_restore_failure"
	if err := mm.EnsureRestoredFlowMission(id, flowID, "Original", nil, false); err != nil {
		t.Fatal(err)
	}
	before, _ := mm.Get(id)
	mm.mu.Lock()
	file := mm.file
	mm.file = filepath.Join(t.TempDir(), "missing", "missions.json")
	mm.mu.Unlock()
	defer func() {
		mm.mu.Lock()
		mm.file = file
		mm.mu.Unlock()
	}()
	if err := mm.EnsureRestoredFlowMission(id, flowID, "Changed", nil, true); err == nil {
		t.Fatal("existing mission save unexpectedly succeeded")
	}
	after, _ := mm.Get(id)
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("existing mission changed after failed save: before=%+v after=%+v", before, after)
	}
	if err := mm.EnsureRestoredFlowMission("mission_new_failure", "flow_new_failure", "New", nil, false); err == nil {
		t.Fatal("new mission save unexpectedly succeeded")
	}
	if _, ok := mm.Get("mission_new_failure"); ok {
		t.Fatal("new mission survived failed save")
	}
}
