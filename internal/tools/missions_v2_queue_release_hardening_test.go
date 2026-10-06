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
