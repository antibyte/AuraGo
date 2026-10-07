package tools

import (
	"aurago/internal/fileutil"
	"context"
	"errors"
	"sync/atomic"
	"testing"
)

// Tests for the merge of main's Desktop-owned missions and keyed webhook registrations
// with the flow missions of EasyDrag.

// QueueOwnedMission never queues a flow mission for the agent; the dispatcher would drop it.
func TestMergeQueueOwnedMissionRefusesFlowMissions(t *testing.T) {
	ConfigureRuntimePermissions(defaultRuntimePermissionsForTests())
	mm := NewMissionManagerV2(tempSystemTaskDir(t), nil)
	defer mm.Stop()
	hooks := newFakeFlowHooks()
	mm.SetFlowHooks(hooks)
	id := publishTestFlow(t, mm, FlowTriggerSpec{NodeID: "n_aaaaaaaa", TriggerType: FlowTriggerManual})
	var releases atomic.Int32
	err := mm.QueueOwnedMission(context.Background(), func() { releases.Add(1) }, id, "manual", "")
	if !errors.Is(err, errFlowMissionNotOwnable) {
		t.Fatalf("QueueOwnedMission(flow) = %v, want errFlowMissionNotOwnable", err)
	}
	if releases.Load() != 1 {
		t.Fatalf("owner released %d times, want once", releases.Load())
	}
	if q, running := mm.GetQueue(); len(q.List()) != 0 || running != "" {
		t.Fatalf("agent queue = %+v, running %q", q.List(), running)
	}
	if m, _ := mm.Get(id); m.Status != MissionStatusIdle {
		t.Fatalf("flow mission status = %q", m.Status)
	}
	hooks.expectNoStart(t)
}

// An owned run's completion queues its prompt dependents with the owner and starts the flows
// that wait for it; a revoked owner fires neither.
func TestMergeOwnedCompletionFiresPromptAndFlowDependents(t *testing.T) {
	ConfigureRuntimePermissions(defaultRuntimePermissionsForTests())
	for _, revoke := range []bool{false, true} {
		t.Run(map[bool]string{false: "complete", true: "revoke"}[revoke], func(t *testing.T) {
			m := NewMissionManagerV2(tempSystemTaskDir(t), nil)
			defer m.Stop()
			hooks := newFakeFlowHooks()
			m.SetFlowHooks(hooks)
			for _, mission := range []*MissionV2{
				{ID: "first", Name: "First", Prompt: "test", Enabled: true, ExecutionType: ExecutionManual},
				{ID: "next", Name: "Next", Prompt: "test", Enabled: true, ExecutionType: ExecutionTriggered, TriggerType: TriggerMissionCompleted, TriggerConfig: &TriggerConfig{SourceMissionID: "first"}},
			} {
				if err := m.Create(mission); err != nil {
					t.Fatal(err)
				}
			}
			flowID := publishTestFlow(t, m, FlowTriggerSpec{NodeID: "n_bbbbbbbb", TriggerType: TriggerMissionCompleted,
				TriggerConfig: &TriggerConfig{SourceMissionID: "first"}})
			var refused atomic.Bool
			ctx := fileutil.WithPublicationGate(context.Background(), func(publish func() error) error {
				if refused.Load() {
					return context.Canceled
				}
				return publish()
			})
			var releases atomic.Int32
			if err := m.QueueOwnedMission(ctx, func() { releases.Add(1) }, "first", "manual", ""); err != nil {
				t.Fatal(err)
			}
			item, ok := m.queue.TryStartNext()
			if !ok {
				t.Fatal("not queued")
			}
			m.mu.Lock()
			m.missions["first"].Status = MissionStatusRunning
			m.mu.Unlock()
			refused.Store(revoke)
			m.OnMissionComplete("first", MissionResultSuccess, "done")
			item.releaseOwner()
			q, _ := m.GetQueue()
			if revoke {
				if len(q.List()) != 0 || releases.Load() != 1 {
					t.Fatalf("a revoked owner queued %+v or leaked its owner (releases %d)", q.List(), releases.Load())
				}
				hooks.expectNoStart(t)
				return
			}
			items := q.List()
			if len(items) != 1 || items[0].MissionID != "next" || !items[0].RequiresOwner || items[0].ownerContext.Err() != nil || releases.Load() != 0 {
				t.Fatalf("prompt dependent = %+v (releases %d), want it queued with the live owner", items, releases.Load())
			}
			if c := hooks.waitStart(t); c.missionID != flowID || c.nodeID != "n_bbbbbbbb" || c.triggerType != string(TriggerMissionCompleted) {
				t.Fatalf("flow dependent start = %+v", c)
			}
			hooks.expectNoStart(t)
		})
	}
}

// Flow webhook registrations are keyed by their slot: replacing the webhook manager moves them
// to the new one, and Stop removes them, as for prompt missions.
func TestMergeFlowWebhooksFollowManagerReplacementAndStop(t *testing.T) {
	mm := NewMissionManagerV2(tempSystemTaskDir(t), nil)
	hooks := newFakeFlowHooks()
	mm.SetFlowHooks(hooks)
	first := &c07Webhooks{}
	mm.SetWebhookManager(first)
	publishTestFlow(t, mm, FlowTriggerSpec{NodeID: "n_aaaaaaaa", TriggerType: TriggerWebhook, TriggerConfig: &TriggerConfig{WebhookID: "hook-1"}})
	if first.count("hook-1") != 1 {
		t.Fatalf("first manager holds %d registrations, want 1", first.count("hook-1"))
	}
	second := &c07Webhooks{}
	mm.SetWebhookManager(second)
	if first.count("hook-1") != 0 || second.count("hook-1") != 1 {
		t.Fatalf("after the replacement: first %d, second %d registrations", first.count("hook-1"), second.count("hook-1"))
	}
	second.fire("hook-1", []byte(`{}`))
	if c := hooks.waitStart(t); c.nodeID != "n_aaaaaaaa" {
		t.Fatalf("webhook start = %+v", c)
	}
	mm.Stop()
	if n := second.count("hook-1"); n != 0 {
		t.Fatalf("%d flow webhook registrations left after Stop", n)
	}
}
