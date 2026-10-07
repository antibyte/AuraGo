package server

import (
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/flows"
	"aurago/internal/tools"
)

func TestFlowTriggerSpecsFromBindings(t *testing.T) {
	specs, err := flowTriggerSpecs([]flows.TriggerBinding{
		{NodeID: "n_aaaaaaaa", Kind: flows.BindingMission, MissionTrigger: "webhook",
			Config: map[string]any{"webhook_id": "h1", "min_interval_seconds": 30.0}},
		{NodeID: "n_bbbbbbbb", Kind: flows.BindingCron, Schedule: "0 7 * * 1-5"},
		{NodeID: "n_cccccccc", Kind: flows.BindingTimer, FireAt: time.Date(2026, 12, 24, 18, 0, 0, 0, time.UTC)},
		{NodeID: "n_dddddddd", Kind: flows.BindingManual},
		{NodeID: "n_eeeeeeee", Kind: flows.BindingMission, MissionTrigger: "mission_completed",
			Config: map[string]any{"source_mission_id": "m1", "require_success": true}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if specs[0].TriggerType != tools.TriggerWebhook || specs[0].TriggerConfig.WebhookID != "h1" || specs[0].TriggerConfig.MinIntervalSeconds != 30 {
		t.Fatalf("webhook spec = %+v %+v", specs[0], specs[0].TriggerConfig)
	}
	if specs[1].TriggerType != tools.FlowTriggerSchedule || specs[1].Schedule != "0 7 * * 1-5" {
		t.Fatalf("cron spec = %+v", specs[1])
	}
	if specs[2].TriggerType != tools.FlowTriggerDateTime || specs[3].TriggerType != tools.FlowTriggerManual {
		t.Fatalf("timer/manual specs = %+v %+v", specs[2], specs[3])
	}
	if !specs[4].TriggerConfig.RequireSuccess || specs[4].TriggerConfig.SourceMissionID != "m1" {
		t.Fatalf("mission spec = %+v", specs[4].TriggerConfig)
	}
	if _, err := flowTriggerSpecs([]flows.TriggerBinding{{NodeID: "n_x", Kind: "bogus"}}); err == nil {
		t.Fatal("unknown bindings must fail")
	}
}

func TestFlowRunOutcome(t *testing.T) {
	ok := flows.RunFinishedInfo{Result: flows.RunResult{Status: flows.RunSuccess}, Outputs: map[string]any{"pdf": "a.pdf"}}
	if r, out := flowRunOutcome(ok); r != tools.MissionResultSuccess || out != `{"pdf":"a.pdf"}` {
		t.Fatalf("success = %q %q", r, out)
	}
	failed := flows.RunFinishedInfo{Result: flows.RunResult{Status: flows.RunError, ErrorCode: "FLOW_TOOL_ERROR", ErrorMessage: "kaputt"}}
	if r, out := flowRunOutcome(failed); r != tools.MissionResultError || out != "FLOW_TOOL_ERROR: kaputt" {
		t.Fatalf("error = %q %q", r, out)
	}
	cancelled := flows.RunFinishedInfo{Result: flows.RunResult{Status: flows.RunCancelled}}
	if r, out := flowRunOutcome(cancelled); r != tools.MissionResultError || out != tools.MissionCancelledOutput {
		t.Fatalf("cancelled = %q %q", r, out)
	}
}

func TestFlowBridgeKeepsMissionControlInSync(t *testing.T) {
	cfg := &config.Config{}
	cfg.Tools.Missions.Enabled = true
	tools.ConfigureRuntimePermissions(tools.RuntimePermissionsFromConfig(cfg))
	s := &Server{Cfg: cfg, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), MissionManagerV2: tools.NewMissionManagerV2(t.TempDir(), nil)}
	bridge := flowMissionBridge{s: s}
	id, err := bridge.CreateFlowMission("flow_aaaaaaaaaa", "Bericht")
	if err != nil {
		t.Fatal(err)
	}
	if err := bridge.SyncFlowMission(id, "Bericht", []flows.TriggerBinding{{NodeID: "n_aaaaaaaa", Kind: flows.BindingManual}}); err != nil {
		t.Fatal(err)
	}
	if err := bridge.SetFlowMissionEnabled(id, true); err != nil || !bridge.FlowMissionEnabled(id) {
		t.Fatalf("enable = %v", err)
	}
	bridge.FlowRunStarted(id, flows.RunRecord{ID: "run_aaaaaaaaaaaa", FlowID: "flow_aaaaaaaaaa", TriggerType: "manual", TriggerData: map[string]any{"x": 1.0}})
	if m, _ := s.MissionManagerV2.Get(id); m.Status != tools.MissionStatusRunning {
		t.Fatalf("status = %q", m.Status)
	}
	bridge.FlowRunFinished(flows.RunFinishedInfo{MissionID: id, FlowName: "Bericht", NotifyOnError: "off", Started: true,
		Record: flows.RunRecord{ID: "run_aaaaaaaaaaaa", FlowID: "flow_aaaaaaaaaa"},
		Result: flows.RunResult{Status: flows.RunError, ErrorCode: "FLOW_TOOL_ERROR", ErrorMessage: "kaputt"}})
	m, _ := s.MissionManagerV2.Get(id)
	if m.Status != tools.MissionStatusIdle || m.LastResult != tools.MissionResultError || !strings.Contains(m.LastOutput, "kaputt") {
		t.Fatalf("mission after run = %+v", m)
	}
	if err := bridge.DeleteFlowMission(id); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.MissionManagerV2.Get(id); ok {
		t.Fatal("the mission must be deleted")
	}
}

func TestFlowFailureNotificationPayload(t *testing.T) {
	info := flows.RunFinishedInfo{FlowName: "Bericht", Record: flows.RunRecord{ID: "run_aaaaaaaaaaaa", FlowID: "flow_aaaaaaaaaa"}}
	payload := flowFailureNotification("en", info, "FLOW_TOOL_ERROR: kaputt")
	ctx, _ := payload["context"].(map[string]any)
	if payload["type"] != "error" || payload["appId"] != "easydrag" || ctx["flow_id"] != "flow_aaaaaaaaaa" || ctx["run_id"] != "run_aaaaaaaaaaaa" {
		t.Fatalf("payload = %+v", payload)
	}
}

func TestFlowMissionHooksWithoutService(t *testing.T) {
	hooks := flowMissionHooks{s: &Server{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}}
	if err := hooks.StartFlowRun("m", "", "manual", ""); err == nil {
		t.Fatal("without the flow service a run cannot start")
	}
	if _, ok := hooks.NextFlowRun("m"); ok {
		t.Fatal("no next run without the flow service")
	}
	hooks.FlowMissionDeleted("m")
	hooks.FlowEnabledChanged("m", true)
}
