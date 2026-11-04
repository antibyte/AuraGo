package flows

import (
	"context"
	"errors"
	"testing"
	"time"
)

type repairBridge struct {
	*fakeBridge
	checkErr error
	entered  chan struct{}
	release  chan struct{}
}

func (b *repairBridge) FlowMissionExists(id string) (bool, error) {
	if b.checkErr != nil {
		return false, b.checkErr
	}
	_, exists := b.mission(id)
	return exists, nil
}

func (b *repairBridge) RestoreFlowMission(id, flowID, name string, bindings []TriggerBinding, published bool) error {
	if b.entered != nil {
		close(b.entered)
		<-b.release
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if m := b.missions[id]; m != nil && (m.flowID != flowID || m.enabled) {
		return errors.New("mission conflict")
	}
	b.missions[id] = &fakeMission{flowID: flowID, name: name, bindings: bindings}
	return nil
}

func TestRepairMissionSerializesDraftSave(t *testing.T) {
	ctx := context.Background()
	bridge := &repairBridge{fakeBridge: newFakeBridge(), entered: make(chan struct{}), release: make(chan struct{})}
	s := svcNewService(t, nil, bridge, nil)
	rec, err := s.CreateFlow(ctx, CreateRequest{Import: simpleFlow("Repair")})
	if err != nil {
		t.Fatal(err)
	}
	if err := bridge.DeleteFlowMission(rec.MissionID); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- s.RepairMission(ctx, rec.ID, rec.DraftRevision, rec.LiveRevision) }()
	<-bridge.entered
	short, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()
	_, _, err = s.SaveDraft(short, rec.ID, simpleFlow("Changed"), rec.DraftRevision)
	close(bridge.release)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("draft save bypassed repair: %v", err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestRepairMissionAndStaleDelete(t *testing.T) {
	ctx := context.Background()
	bridge := &repairBridge{fakeBridge: newFakeBridge()}
	s := svcNewService(t, nil, bridge, nil)
	rec, err := s.CreateFlow(ctx, CreateRequest{Import: simpleFlow("Repair")})
	if err != nil {
		t.Fatal(err)
	}
	if err := bridge.DeleteFlowMission(rec.MissionID); err != nil {
		t.Fatal(err)
	}
	if err := s.RepairMission(ctx, rec.ID, rec.DraftRevision+1, rec.LiveRevision); !errors.Is(err, ErrRevisionConflict) {
		t.Fatalf("stale repair: %v", err)
	}
	if err := s.RepairMission(ctx, rec.ID, rec.DraftRevision, rec.LiveRevision); err != nil {
		t.Fatal(err)
	}
	mission, ok := bridge.mission(rec.MissionID)
	if !ok || mission.enabled || mission.flowID != rec.ID {
		t.Fatalf("restored mission = %+v %v", mission, ok)
	}
	if err := s.DeleteFlowForMission(ctx, rec.MissionID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetFlow(ctx, rec.ID); err != nil {
		t.Fatalf("stale delete removed repaired flow: %v", err)
	}
	bridge.checkErr = errors.New("unavailable")
	if err := s.DeleteFlowForMission(ctx, rec.MissionID); err == nil {
		t.Fatal("unavailable manager must block deletion")
	}
	if _, err := s.GetFlow(ctx, rec.ID); err != nil {
		t.Fatalf("unavailable manager removed flow: %v", err)
	}
}
