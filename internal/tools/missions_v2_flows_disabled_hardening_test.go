package tools

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// Task 1c-16: when EasyDrag is off (or its store cannot be opened) the server never calls
// SetFlowHooks. Flow missions in missions.json must then never run as agent missions, and
// Mission Control must show why their triggers do nothing.

// c16AgentCalls records the missions the agent callback was asked to run.
type c16AgentCalls struct {
	mu  sync.Mutex
	ids []string
}

func (c *c16AgentCalls) add(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ids = append(c.ids, id)
}

func (c *c16AgentCalls) list() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.ids...)
}

// c16ExpectUnavailable waits until the flow mission shows the "flows are not available" result.
func c16ExpectUnavailable(t *testing.T, mm *MissionManagerV2, missionID string) {
	t.Helper()
	c07Await(t, "the flow mission to show that flows are not available", func() bool {
		m, ok := mm.Get(missionID)
		return ok && m.LastResult == MissionResultError && m.LastOutput == flowsUnavailableOutput
	})
	if m, _ := mm.Get(missionID); m.RunCount != 0 || m.Status != MissionStatusIdle {
		t.Fatalf("flow mission after a refused trigger: run count %d, status %q", m.RunCount, m.Status)
	}
}

func c16ExpectNoAgentRun(t *testing.T, calls *c16AgentCalls, flowMissions ...string) {
	t.Helper()
	for _, id := range calls.list() {
		for _, flow := range flowMissions {
			if id == flow {
				t.Fatalf("the agent ran flow mission %s", id)
			}
		}
	}
}

func TestC16FlowMissionsWithoutHooksNeverRunAsAgentMissions(t *testing.T) {
	dir := tempSystemTaskDir(t)
	setup := NewMissionManagerV2(dir, nil)
	const sourceID = "mission_c16_source"
	if err := setup.Create(&MissionV2{ID: sourceID, Name: "Quelle", Prompt: "c16 source prompt", ExecutionType: ExecutionManual}); err != nil {
		t.Fatal(err)
	}
	startupFlow := publishTestFlow(t, setup,
		FlowTriggerSpec{NodeID: "n_aaaaaaaa", TriggerType: FlowTriggerManual},
		FlowTriggerSpec{NodeID: "n_bbbbbbbb", TriggerType: TriggerSystemStartup})
	dependentFlow := publishTestFlow(t, setup,
		FlowTriggerSpec{NodeID: "n_cccccccc", TriggerType: TriggerMissionCompleted, TriggerConfig: &TriggerConfig{SourceMissionID: sourceID}})
	// The last process stopped while both flows ran and one was queued as an agent item.
	setup.FlowRunStarted(startupFlow, "manual", "")
	setup.FlowRunStarted(dependentFlow, "manual", "")
	data, err := json.Marshal(missionQueueSnapshot{
		Items:   []QueueItem{{MissionID: dependentFlow, Priority: 3, EnqueuedAt: time.Now(), TriggerType: "manual"}},
		Running: startupFlow,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "missions_v2_queue.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}

	mm := NewMissionManagerV2(dir, nil) // no SetFlowHooks: EasyDrag is off
	calls := &c16AgentCalls{}
	var callbacks sync.WaitGroup
	mm.SetCallback(func(_ string, missionID string) {
		callbacks.Add(1)
		defer callbacks.Done()
		calls.add(missionID)
		mm.OnMissionComplete(missionID, MissionResultSuccess, "ok")
	})
	if err := mm.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { mm.Stop(); callbacks.Wait() })

	// Restart recovery: neither flow is running, queued or restored into the agent queue.
	queue, running := mm.GetQueue()
	if running == startupFlow || running == dependentFlow {
		t.Fatalf("a flow mission was restored as the running item %q", running)
	}
	for _, item := range queue.List() {
		if item.MissionID == startupFlow || item.MissionID == dependentFlow {
			t.Fatalf("a flow mission was restored into the queue: %+v", queue.List())
		}
	}
	// The startup trigger fired during Start and was refused visibly.
	c16ExpectUnavailable(t, mm, startupFlow)

	// Run now and the trigger API refuse with a clear error.
	for name, run := range map[string]func() error{
		"RunNow":         func() error { return mm.RunNow(startupFlow) },
		"TriggerMission": func() error { return mm.TriggerMission(startupFlow, "api", `{"x":1}`) },
	} {
		if err := run(); err == nil || !strings.Contains(err.Error(), "flows are not available") {
			t.Fatalf("%s on a flow mission without hooks: %v", name, err)
		}
	}

	// A finished prompt mission fires its mission_completed dependents: the flow does not run.
	if m, _ := mm.Get(dependentFlow); m.LastOutput == flowsUnavailableOutput {
		t.Fatal("the dependent flow showed the note before its trigger fired")
	}
	if err := mm.RunNow(sourceID); err != nil {
		t.Fatalf("RunNow source: %v", err)
	}
	c07Await(t, "the source mission to run through the agent", func() bool {
		for _, id := range calls.list() {
			if id == sourceID {
				return true
			}
		}
		return false
	})
	c16ExpectUnavailable(t, mm, dependentFlow)
	queue, running = mm.GetQueue()
	if len(queue.List()) != 0 || (running != "" && running != sourceID) {
		t.Fatalf("queue after the dependents fired: items %+v running %q", queue.List(), running)
	}
	c16ExpectNoAgentRun(t, calls, startupFlow, dependentFlow)
}

func TestC16FlowEventTriggersWithoutHooksOnlyShowTheReason(t *testing.T) {
	mm := NewMissionManagerV2(tempSystemTaskDir(t), nil)
	t.Cleanup(mm.Stop)
	webhooks := &c07Webhooks{}
	mm.SetWebhookManager(webhooks)
	calls := &c16AgentCalls{}
	mm.SetCallback(func(_ string, missionID string) { calls.add(missionID) })
	flowID := publishTestFlow(t, mm, FlowTriggerSpec{NodeID: "n_aaaaaaaa", TriggerType: TriggerWebhook,
		TriggerConfig: &TriggerConfig{WebhookID: "c16-hook"}})
	if webhooks.count("c16-hook") != 1 {
		t.Fatalf("webhook registrations = %d, want 1", webhooks.count("c16-hook"))
	}
	webhooks.fire("c16-hook", []byte(`{"c16":true}`))
	c16ExpectUnavailable(t, mm, flowID)
	// A second event finds the note in place and changes nothing.
	before, _ := mm.Get(flowID)
	webhooks.fire("c16-hook", []byte(`{"c16":true}`))
	after, _ := mm.Get(flowID)
	if after.LastResult != before.LastResult || after.LastOutput != before.LastOutput || after.RunCount != 0 {
		t.Fatalf("second refused event changed the mission: %+v", after)
	}
	queue, running := mm.GetQueue()
	if len(queue.List()) != 0 || running != "" {
		t.Fatalf("queue after refused events: items %+v running %q", queue.List(), running)
	}
	time.Sleep(100 * time.Millisecond) // a dispatched agent run would show up by now
	c16ExpectNoAgentRun(t, calls, flowID)
}

func TestC16FlowCronJobWithoutHooksNeverReachesTheAgent(t *testing.T) {
	dir := tempSystemTaskDir(t)
	setupCron := NewCronManager(dir)
	setup := NewMissionManagerV2(dir, setupCron)
	flowID := publishTestFlow(t, setup, FlowTriggerSpec{NodeID: "n_aaaaaaaa", TriggerType: FlowTriggerSchedule, Schedule: "* * * * * *"})
	if err := setupCron.Close(); err != nil {
		t.Fatal(err)
	}

	cronMgr := NewCronManager(dir)
	var fallbackMu sync.Mutex
	var fallback []string
	if err := cronMgr.Start(func(prompt string) {
		fallbackMu.Lock()
		defer fallbackMu.Unlock()
		fallback = append(fallback, prompt)
	}); err != nil {
		t.Fatalf("cron start: %v", err)
	}
	mm := NewMissionManagerV2(dir, cronMgr) // no SetFlowHooks: EasyDrag is off
	calls := &c16AgentCalls{}
	var callbacks sync.WaitGroup
	mm.SetCallback(func(_ string, missionID string) {
		callbacks.Add(1)
		defer callbacks.Done()
		calls.add(missionID)
		mm.OnMissionComplete(missionID, MissionResultSuccess, "ok")
	})
	t.Cleanup(func() { c07StopAndDrain(t, cronMgr, mm, &callbacks) })
	if err := mm.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if _, ok := c07CronJob(cronMgr, flowCronJobID(flowID, "n_aaaaaaaa")); !ok {
		t.Fatal("the flow schedule job is not registered")
	}
	c16ExpectUnavailable(t, mm, flowID) // the job fired through the flow runner
	fallbackMu.Lock()
	got := append([]string(nil), fallback...)
	fallbackMu.Unlock()
	if len(got) != 0 {
		t.Fatalf("the agent fallback received %q", got)
	}
	c16ExpectNoAgentRun(t, calls, flowID)
}
