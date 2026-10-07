package tools

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFlowRunsRecordHistoryWithoutTouchingTheQueue(t *testing.T) {
	dir := tempSystemTaskDir(t)
	mm := NewMissionManagerV2(dir, nil)
	hist, err := InitMissionHistoryDB(filepath.Join(dir, "history.db"))
	if err != nil {
		t.Fatalf("history db: %v", err)
	}
	t.Cleanup(func() { hist.Close() })
	mm.SetHistoryDB(hist)
	mm.SetFlowHooks(newFakeFlowHooks())
	id := publishTestFlow(t, mm, FlowTriggerSpec{NodeID: "n_aaaaaaaa", TriggerType: FlowTriggerManual})

	mm.mu.Lock()
	mm.missions["agent"] = &MissionV2{ID: "agent", Name: "Agent", Prompt: "p", ExecutionType: ExecutionManual, Enabled: true, Priority: "medium"}
	mm.mu.Unlock()
	mm.queue.Enqueue("agent", "medium", "manual", "")
	if _, ok := mm.queue.TryStartNext(); !ok {
		t.Fatal("the agent mission did not start")
	}

	runA := mm.FlowRunStarted(id, "manual", `{"x":1}`)
	runB := mm.FlowRunStarted(id, "webhook", `{}`)
	if runA == "" || runB == "" {
		t.Fatalf("history ids = %q %q", runA, runB)
	}
	if m, _ := mm.Get(id); m.Status != MissionStatusRunning {
		t.Fatalf("status = %q", m.Status)
	}
	mm.FlowRunFinished(id, runA, MissionResultSuccess, `{"pdf":"a.pdf"}`, map[string]any{"pdf": "a.pdf"})
	if m, _ := mm.Get(id); m.Status != MissionStatusRunning || m.RunCount != 1 || m.LastResult != MissionResultSuccess {
		t.Fatalf("after first finish = %+v", m)
	}
	mm.FlowRunFinished(id, runB, MissionResultError, "FLOW_TOOL_ERROR: kaputt", nil)
	if m, _ := mm.Get(id); m.Status != MissionStatusIdle || m.RunCount != 2 || m.LastResult != MissionResultError {
		t.Fatalf("after second finish = %+v", m)
	}
	if _, running := mm.GetQueue(); running != "agent" {
		t.Fatalf("a flow run must not release the agent queue, running = %q", running)
	}
	run, err := GetMissionRun(hist, runA)
	if err != nil || run.Status != "success" || run.TriggerType != "manual" {
		t.Fatalf("history run = %+v, %v", run, err)
	}
	failed, err := GetMissionRun(hist, runB)
	if err != nil || failed.Status != "error" {
		t.Fatalf("failed history run = %+v, %v", failed, err)
	}
}

func TestMissionCompletedCarriesOutputToPromptsAndFlows(t *testing.T) {
	mm, hooks, _ := newFlowTestManager(t)
	source := publishTestFlow(t, mm, FlowTriggerSpec{NodeID: "n_aaaaaaaa", TriggerType: FlowTriggerManual})
	mm.mu.Lock()
	mm.missions["chained"] = &MissionV2{ID: "chained", Name: "Chained", Prompt: "p", ExecutionType: ExecutionTriggered,
		TriggerType: TriggerMissionCompleted, TriggerConfig: &TriggerConfig{SourceMissionID: source, RequireSuccess: true},
		Enabled: true, Priority: "medium", Status: MissionStatusIdle}
	mm.mu.Unlock()
	follower, err := mm.CreateFlowMission("flow_bbbbbbbbbb", "Folge")
	if err != nil {
		t.Fatal(err)
	}
	if err := mm.SyncFlowMission(follower, "Folge", []FlowTriggerSpec{{NodeID: "n_bbbbbbbb", TriggerType: TriggerMissionCompleted,
		TriggerConfig: &TriggerConfig{SourceMissionID: source, RequireSuccess: true}}}); err != nil {
		t.Fatal(err)
	}
	if err := mm.SetFlowMissionEnabled(follower, true); err != nil {
		t.Fatal(err)
	}

	run := mm.FlowRunStarted(source, "manual", "")
	mm.FlowRunFinished(source, run, MissionResultSuccess, "Fertig.", map[string]any{"report": "ok"})
	items := mm.queue.List()
	if len(items) != 1 || items[0].MissionID != "chained" || !strings.Contains(items[0].TriggerData, `"output":"Fertig."`) ||
		!strings.Contains(items[0].TriggerData, `"outputs":{"report":"ok"}`) {
		t.Fatalf("queued dependents = %+v", items)
	}
	c := hooks.waitStart(t)
	if c.missionID != follower || c.nodeID != "n_bbbbbbbb" || c.triggerType != "mission_completed" ||
		!strings.Contains(c.data, `"source_mission":"`+source+`"`) {
		t.Fatalf("follower start = %+v", c)
	}

	run = mm.FlowRunStarted(source, "manual", "")
	mm.FlowRunFinished(source, run, MissionResultError, "kaputt", nil)
	hooks.expectNoStart(t)
}

func TestPromptMissionCompletionDataIncludesOutput(t *testing.T) {
	mm := NewMissionManagerV2(tempSystemTaskDir(t), nil)
	mm.missions["src"] = &MissionV2{ID: "src", Name: "Src", Prompt: "p", ExecutionType: ExecutionManual, Enabled: true,
		Priority: "medium", Status: MissionStatusRunning}
	mm.missions["dep"] = &MissionV2{ID: "dep", Name: "Dep", Prompt: "p", ExecutionType: ExecutionTriggered,
		TriggerType: TriggerMissionCompleted, TriggerConfig: &TriggerConfig{SourceMissionID: "src"}, Enabled: true,
		Priority: "medium", Status: MissionStatusIdle}
	mm.OnMissionComplete("src", MissionResultSuccess, "Ergebnis")
	items := mm.queue.List()
	if len(items) != 1 || !strings.Contains(items[0].TriggerData, `"output":"Ergebnis"`) ||
		!strings.Contains(items[0].TriggerData, `"source_mission":"src"`) || strings.Contains(items[0].TriggerData, `"outputs"`) {
		t.Fatalf("dependent trigger data = %+v", items)
	}
}

func TestOnMissionCompleteIgnoresFlowMissions(t *testing.T) {
	mm, _, _ := newFlowTestManager(t)
	id := publishTestFlow(t, mm, FlowTriggerSpec{NodeID: "n_aaaaaaaa", TriggerType: FlowTriggerManual})
	mm.mu.Lock()
	mm.missions["agent"] = &MissionV2{ID: "agent", Name: "Agent", Prompt: "p", ExecutionType: ExecutionManual, Enabled: true, Priority: "medium"}
	mm.mu.Unlock()
	mm.queue.Enqueue("agent", "medium", "manual", "")
	mm.queue.TryStartNext()
	mm.FlowRunStarted(id, "manual", "")
	mm.OnMissionComplete(id, MissionResultSuccess, "x")
	if _, running := mm.GetQueue(); running != "agent" {
		t.Fatalf("OnMissionComplete on a flow released the agent queue: %q", running)
	}
	if m, _ := mm.Get(id); m.Status != MissionStatusRunning || m.RunCount != 0 {
		t.Fatalf("flow mission = %+v", m)
	}
}

func TestRunNowAndTriggerStartFlowRuns(t *testing.T) {
	mm, hooks, _ := newFlowTestManager(t)
	id := publishTestFlow(t, mm, FlowTriggerSpec{NodeID: "n_aaaaaaaa", TriggerType: FlowTriggerManual})
	if err := mm.RunNow(id); err != nil {
		t.Fatalf("RunNow: %v", err)
	}
	if c := hooks.waitStart(t); c != (flowStartCall{id, "", "manual", ""}) {
		t.Fatalf("run now = %+v", c)
	}
	if err := mm.TriggerMission(id, "daemon_wake", `{"a":1}`); err != nil {
		t.Fatalf("TriggerMission: %v", err)
	}
	if c := hooks.waitStart(t); c.triggerType != "daemon_wake" || c.data != `{"a":1}` {
		t.Fatalf("trigger = %+v", c)
	}
	hooks.setErr(errors.New("the flow has not been published yet"))
	if err := mm.RunNow(id); err == nil || !strings.Contains(err.Error(), "published") {
		t.Fatalf("hook errors must surface: %v", err)
	}
	hooks.waitStart(t)
	hooks.setErr(nil)
	if err := mm.SetFlowMissionEnabled(id, false); err != nil {
		t.Fatal(err)
	}
	if err := mm.RunNow(id); err == nil || !strings.Contains(err.Error(), "disabled") {
		t.Fatalf("disabled flow = %v", err)
	}
	if queue, _ := mm.GetQueue(); len(queue.List()) != 0 {
		t.Fatalf("flow runs must not use the agent queue: %+v", queue.List())
	}
}

func TestUpdateOnlyTogglesFlowMissions(t *testing.T) {
	mm, hooks, _ := newFlowTestManager(t)
	draft, _ := mm.CreateFlowMission("flow_cccccccccc", "Entwurf")
	m, _ := mm.Get(draft)
	m.Enabled = true
	if err := mm.Update(draft, m); err == nil || !strings.Contains(err.Error(), "not supported") {
		t.Fatalf("enabling an unpublished flow = %v", err)
	}
	id := publishTestFlow(t, mm, FlowTriggerSpec{NodeID: "n_aaaaaaaa", TriggerType: FlowTriggerManual})
	m, _ = mm.Get(id)
	m.Name, m.Prompt, m.Enabled, m.Locked, m.Priority = "Umbenannt", "neuer Prompt", false, true, "high"
	if err := mm.Update(id, m); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got, _ := mm.Get(id)
	if got.Name != "Morgenbericht" || got.Prompt != "" || got.Enabled || !got.Locked || got.Priority != "medium" ||
		got.ExecutionType != ExecutionFlow || len(got.FlowTriggers) != 1 {
		t.Fatalf("flow mission after update = %+v", got)
	}
	eventually(t, "FlowEnabledChanged", func() bool {
		hooks.mu.Lock()
		defer hooks.mu.Unlock()
		return len(hooks.enabled) == 1 && hooks.enabled[0] == id+"=false"
	})
	m.ExecutionType = ExecutionManual
	if err := mm.Update(id, m); !errors.Is(err, ErrFlowMissionManaged) {
		t.Fatalf("converting a flow mission = %v", err)
	}
}

func TestDeleteFlowMissionCascades(t *testing.T) {
	mm, hooks, _ := newFlowTestManager(t)
	id := publishTestFlow(t, mm, FlowTriggerSpec{NodeID: "n_aaaaaaaa", TriggerType: FlowTriggerManual})
	if err := mm.Delete(id); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	eventually(t, "FlowMissionDeleted", func() bool {
		hooks.mu.Lock()
		defer hooks.mu.Unlock()
		return len(hooks.deleted) == 1 && hooks.deleted[0] == id
	})
	other := publishTestFlow(t, mm, FlowTriggerSpec{NodeID: "n_aaaaaaaa", TriggerType: FlowTriggerManual})
	if err := mm.DeleteFlowMission(other); err != nil {
		t.Fatalf("DeleteFlowMission: %v", err)
	}
	if _, ok := mm.Get(other); ok {
		t.Fatal("the flow mission still exists")
	}
	time.Sleep(50 * time.Millisecond)
	hooks.mu.Lock()
	deleted := len(hooks.deleted)
	hooks.mu.Unlock()
	if deleted != 1 {
		t.Fatal("DeleteFlowMission must not call FlowMissionDeleted")
	}
	if err := mm.DeleteFlowMission("missing"); err != nil {
		t.Fatalf("deleting a missing flow mission must be a no-op: %v", err)
	}
	locked := publishTestFlow(t, mm, FlowTriggerSpec{NodeID: "n_aaaaaaaa", TriggerType: FlowTriggerManual})
	mm.mu.Lock()
	mm.missions[locked].Locked = true
	mm.mu.Unlock()
	if err := mm.DeleteFlowMission(locked); err == nil {
		t.Fatal("locked flow missions must not be deleted")
	}
}

func TestNextRunForFlowMissions(t *testing.T) {
	mm, hooks, _ := newFlowTestManager(t)
	id := publishTestFlow(t, mm, FlowTriggerSpec{NodeID: "n_aaaaaaaa", TriggerType: FlowTriggerDateTime})
	want := time.Date(2026, 12, 24, 18, 0, 0, 0, time.UTC)
	hooks.setNext(want)
	if next, ok := mm.NextRun(id); !ok || !next.Equal(want) {
		t.Fatalf("NextRun = %v %v", next, ok)
	}
	if err := mm.SetFlowMissionEnabled(id, false); err != nil {
		t.Fatal(err)
	}
	if _, ok := mm.NextRun(id); ok {
		t.Fatal("a disabled flow has no next run")
	}
}
