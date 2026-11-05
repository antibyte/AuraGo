package tools

import (
	"os"
	"testing"
)

// The optional synchronous hook must only signal a bounded cancellation tracker;
// it is deliberately distinct from the asynchronous flow lifecycle callbacks.
type admissionRevocationTestHooks struct {
	*fakeFlowHooks
	manager *MissionManagerV2
	called  []string
	held    bool
}

func (h *admissionRevocationTestHooks) InvalidateFlowAdmissions(id string) {
	h.called = append(h.called, id)
	if h.manager.mu.TryRLock() {
		h.manager.mu.RUnlock()
	} else {
		h.held = true
	}
}

func TestFlowAdmissionRevocationSignalsSynchronousStateChanges(t *testing.T) {
	for _, tc := range []struct {
		name string
		run  func(*MissionManagerV2, string) error
	}{
		{"manager disable", func(m *MissionManagerV2, id string) error { return m.Update(id, &MissionV2{Enabled: false}) }},
		{"service disable", func(m *MissionManagerV2, id string) error { return m.SetFlowMissionEnabled(id, false) }},
		{"manager delete", func(m *MissionManagerV2, id string) error { return m.Delete(id) }},
		{"service delete", func(m *MissionManagerV2, id string) error { return m.DeleteFlowMission(id) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, _, _ := newFlowTestManager(t)
			h := &admissionRevocationTestHooks{fakeFlowHooks: newFakeFlowHooks(), manager: m}
			m.SetFlowHooks(h)
			id := publishTestFlow(t, m)
			if len(h.called) != 0 {
				t.Fatalf("enable invalidated admissions: %v", h.called)
			}
			if err := tc.run(m, id); err != nil {
				t.Fatal(err)
			}
			if len(h.called) != 1 || h.called[0] != id || !h.held {
				t.Fatalf("revocation calls=%v manager-lock-held=%v", h.called, h.held)
			}
		})
	}
}

func TestFlowAdmissionRevocationFollowsVisibleFailedSaves(t *testing.T) {
	for _, tc := range []struct {
		name                                 string
		run                                  func(*MissionManagerV2, string) error
		wantSignal, wantMission, wantEnabled bool
	}{
		{"manager disable", func(m *MissionManagerV2, id string) error { return m.Update(id, &MissionV2{Enabled: false}) }, true, true, false},
		{"service disable", func(m *MissionManagerV2, id string) error { return m.SetFlowMissionEnabled(id, false) }, true, true, false},
		{"manager delete rolls back", func(m *MissionManagerV2, id string) error { return m.Delete(id) }, false, true, true},
		{"service delete remains removed", func(m *MissionManagerV2, id string) error { return m.DeleteFlowMission(id) }, true, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, _, _ := newFlowTestManager(t)
			h := &admissionRevocationTestHooks{fakeFlowHooks: newFakeFlowHooks(), manager: m}
			m.SetFlowHooks(h)
			id := publishTestFlow(t, m)
			if err := os.Mkdir(m.file+".tmp", 0o755); err != nil {
				t.Fatal(err)
			}
			if err := tc.run(m, id); err == nil {
				t.Fatal("expected a save failure")
			}
			mission, exists := m.Get(id)
			if exists != tc.wantMission || exists && mission.Enabled != tc.wantEnabled {
				t.Fatalf("mission after failed save = %+v, exists=%v", mission, exists)
			}
			if got := len(h.called) == 1 && h.called[0] == id && h.held; got != tc.wantSignal {
				t.Fatalf("revocation calls=%v manager-lock-held=%v", h.called, h.held)
			}
		})
	}
}
