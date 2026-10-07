package tools

import (
	"testing"
	"time"
)

// Review of 1c-19b: a prompt mission that is enqueued again while it runs (a flow source
// completes, RunNow, any trigger) sets its status to queued. Its running completion must
// still release the agent queue, and the new queue item then runs.

// c19bStartRecorder registers an agent callback that only reports which mission started;
// the test completes the runs itself.
func c19bStartRecorder(mm *MissionManagerV2) chan string {
	started := make(chan string, 8)
	mm.SetCallback(func(_ string, missionID string) { started <- missionID })
	return started
}

func c19bAwaitStart(t *testing.T, started chan string, want string) {
	t.Helper()
	select {
	case got := <-started:
		if got != want {
			t.Fatalf("started %q, want %q", got, want)
		}
	case <-time.After(c07DeadlockGuard):
		t.Fatalf("%q did not start", want)
	}
}

func TestC19bMissionEnqueuedWhileRunningReleasesTheQueue(t *testing.T) {
	finishFlow := func(mm *MissionManagerV2, flow string) {
		run := mm.FlowRunStarted(flow, "manual", "")
		mm.FlowRunFinishedAtDepth(flow, run, MissionResultSuccess, "ok", nil, 0)
	}
	for _, c := range []struct {
		name    string
		enqueue func(t *testing.T, mm *MissionManagerV2, flow string)
	}{
		{"flow source completes", func(_ *testing.T, mm *MissionManagerV2, flow string) { finishFlow(mm, flow) }},
		{"RunNow", func(t *testing.T, mm *MissionManagerV2, _ string) {
			if err := mm.RunNow("c19b_x"); err != nil {
				t.Fatalf("RunNow: %v", err)
			}
		}},
		{"webhook trigger", func(t *testing.T, mm *MissionManagerV2, _ string) {
			if err := mm.TriggerMission("c19b_x", "webhook", `{}`); err != nil {
				t.Fatalf("TriggerMission: %v", err)
			}
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			mm, _, _ := newFlowTestManager(t)
			started := c19bStartRecorder(mm)
			flow := publishTestFlow(t, mm, FlowTriggerSpec{NodeID: "n_aaaaaaaa", TriggerType: FlowTriggerManual})
			c08AddPromptDependent(mm, "c19b_x", flow)
			finishFlow(mm, flow)
			mm.processNext()
			c19bAwaitStart(t, started, "c19b_x")

			c.enqueue(t, mm, flow)
			mm.OnMissionComplete("c19b_x", MissionResultSuccess, "first")
			if _, running := mm.GetQueue(); running != "" {
				t.Fatalf("the agent queue stays occupied by %q after its run completed", running)
			}
			if m, _ := mm.Get("c19b_x"); m.RunCount != 1 || m.Status != MissionStatusQueued || m.LastOutput != "first" {
				t.Fatalf("after the first run: runs %d, status %s, output %q", m.RunCount, m.Status, m.LastOutput)
			}

			mm.processNext()
			c19bAwaitStart(t, started, "c19b_x")
			mm.OnMissionComplete("c19b_x", MissionResultSuccess, "second")
			queue, running := mm.GetQueue()
			m, _ := mm.Get("c19b_x")
			if running != "" || len(queue.List()) != 0 || m.RunCount != 2 || m.Status != MissionStatusIdle || m.LastOutput != "second" {
				t.Fatalf("after the second run: running %q, queued %d, runs %d, status %s, output %q",
					running, len(queue.List()), m.RunCount, m.Status, m.LastOutput)
			}
			mm.mu.RLock()
			guards := len(mm.missionGuards)
			mm.mu.RUnlock()
			if guards != 0 {
				t.Fatalf("%d timeout guards left", guards)
			}

			// A late second completion of the finished run (the timeout guard's) changes nothing.
			mm.OnMissionComplete("c19b_x", MissionResultError, "late")
			if m, _ := mm.Get("c19b_x"); m.RunCount != 2 || m.LastOutput != "second" {
				t.Fatalf("a late completion was counted: runs %d, output %q", m.RunCount, m.LastOutput)
			}
		})
	}
}

// Re-review of 1c-19b: TryStartNext claims the agent queue before dispatchQueuedMission
// marks the next run running and sets its timeout guard. A late callback of an earlier run
// (whose timeout guard completed it already) that lands in that window is refused: it
// counts no run, fires no dependent and leaves the claimed slot to the next run.
func TestC19bLateCallbackInTheDispatchWindowIsRefused(t *testing.T) {
	mm, _, _ := newFlowTestManager(t)
	started := c19bStartRecorder(mm)
	mm.mu.Lock()
	mm.missions["c19b_x"] = &MissionV2{ID: "c19b_x", Name: "x", Prompt: "p", ExecutionType: ExecutionManual,
		Enabled: true, Priority: "medium", Status: MissionStatusIdle}
	mm.mu.Unlock()
	c08AddPromptDependent(mm, "c19b_y", "c19b_x")

	// Run 1 of x, deep in a chain; RunNow queues x again while it runs, and the timeout
	// guard completes run 1.
	mm.queue.Enqueue("c19b_x", "medium", "mission_completed", `{"chain_depth":9}`)
	mm.processNext()
	c19bAwaitStart(t, started, "c19b_x")
	if err := mm.RunNow("c19b_x"); err != nil {
		t.Fatalf("RunNow: %v", err)
	}
	mm.OnMissionComplete("c19b_x", MissionResultError, "timeout")
	if !mm.queue.Remove("c19b_y") {
		t.Fatal("run 1 queued no dependent")
	}

	// The queue claims x's RunNow item; run 1's late callback lands before the dispatch.
	item, ok := mm.queue.TryStartNext()
	if !ok || item.MissionID != "c19b_x" {
		t.Fatalf("claimed %+v (%v)", item, ok)
	}
	mm.OnMissionComplete("c19b_x", MissionResultSuccess, "late run 1")
	_, running := mm.GetQueue()
	if m, _ := mm.Get("c19b_x"); m.RunCount != 1 || m.LastOutput != "timeout" || running != "c19b_x" || len(mm.queue.List()) != 0 {
		t.Fatalf("the late callback was accepted: runs %d, output %q, running %q, queued %+v",
			m.RunCount, m.LastOutput, running, mm.queue.List())
	}

	// Run 2 (RunNow, depth 0) runs and completes normally; its dependent is at depth 1.
	mm.dispatchQueuedMission(item)
	c19bAwaitStart(t, started, "c19b_x")
	if m, _ := mm.Get("c19b_x"); m.Status != MissionStatusRunning {
		t.Fatalf("run 2 status %s", m.Status)
	}
	mm.OnMissionComplete("c19b_x", MissionResultSuccess, "run 2")
	_, running = mm.GetQueue()
	m, _ := mm.Get("c19b_x")
	items := mm.queue.List()
	if m.RunCount != 2 || m.LastOutput != "run 2" || m.Status != MissionStatusIdle || running != "" || len(items) != 1 ||
		items[0].MissionID != "c19b_y" || completionChainDepthRaw(items[0].TriggerType, items[0].TriggerData) != 1 {
		t.Fatalf("after run 2: runs %d, output %q, status %s, running %q, queued %+v", m.RunCount, m.LastOutput, m.Status, running, items)
	}
}
