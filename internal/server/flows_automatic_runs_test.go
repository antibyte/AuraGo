package server

import (
	"context"
	"errors"
	"testing"

	"aurago/internal/flows"
	"aurago/internal/tools"
)

// Runs that nobody asks for (cron and event triggers through StartFlowRun, Date/Time timers
// through the bridge's RunGate) stop at once when missions are read-only or the flows are
// switched off, without a restart; they used to keep going until the next start. A cancel
// is not a run and stays open (TestFUReadOnlyMissionsRefuseFlowRunCancel).
func TestAudit23AutomaticRunsFollowReadOnlyAndTheFlowsSwitch(t *testing.T) {
	s := c16StartedServer(t, nil)
	rec := c16PublishedFlow(t, s, greetFlowJSON)
	ctx := context.Background()
	if err := s.Flows.SetEnabled(ctx, rec.ID, true); err != nil {
		t.Fatalf("SetEnabled: %v", err)
	}
	hooks, bridge := flowMissionHooks{s: s}, flowMissionBridge{s: s}
	runs := func() int {
		t.Helper()
		list, err := s.Flows.Store().ListRuns(ctx, rec.ID, flows.RunFilter{Mode: flows.ModeLive, Limit: 50})
		if err != nil {
			t.Fatal(err)
		}
		return len(list)
	}
	var gate flows.RunGate = bridge // the Service finds the gate by type assertion
	if err := gate.AutomaticRunsAllowed(); err != nil {
		t.Fatalf("gate with flows on and missions writable: %v", err)
	}

	refused := func(what string, want error) {
		t.Helper()
		before := runs()
		err := hooks.StartFlowRun(rec.MissionID, "", "manual", "")
		if err == nil || (want != nil && !errors.Is(err, want)) {
			t.Errorf("%s: StartFlowRun = %v, want a refusal %v", what, err, want)
		}
		if gateErr := gate.AutomaticRunsAllowed(); gateErr == nil || gateErr.Error() == "" {
			t.Errorf("%s: the gate allowed automatic runs", what)
		}
		if runs() != before {
			t.Errorf("%s: a run was started", what)
		}
	}
	s.Cfg.Tools.Missions.ReadOnly = true
	refused("read-only missions", tools.ErrFlowRunsPaused)
	s.Cfg.Tools.Missions.ReadOnly = false
	s.Cfg.Flows.Enabled = false
	refused("flows switched off", tools.ErrFlowsUnavailable)
	s.Cfg.Flows.Enabled = true
	s.Cfg.Tools.Missions.Enabled = false
	refused("missions switched off", tools.ErrFlowsUnavailable)
	s.Cfg.Tools.Missions.Enabled = true

	before := runs()
	if err := hooks.StartFlowRun(rec.MissionID, "", "manual", ""); err != nil {
		t.Fatalf("StartFlowRun with everything on: %v", err)
	}
	if runs() != before+1 {
		t.Errorf("runs = %d, want %d", runs(), before+1)
	}
}
