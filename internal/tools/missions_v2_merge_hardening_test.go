package tools

import (
	"aurago/internal/fileutil"
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
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

// mergeGateHooks blocks the first flow run start until gate closes, so later runs wait in the
// event dispatcher's queue; it reports every start on starts.
type mergeGateHooks struct {
	gate    chan struct{}
	starts  chan flowStartCall
	blocked atomic.Bool
}

func (h *mergeGateHooks) StartFlowRun(missionID, nodeID, triggerType, data string) error {
	h.starts <- flowStartCall{missionID, nodeID, triggerType, data}
	if h.blocked.CompareAndSwap(false, true) {
		<-h.gate
	}
	return nil
}
func (h *mergeGateHooks) FlowMissionDeleted(string)            {}
func (h *mergeGateHooks) FlowEnabledChanged(string, bool)      {}
func (h *mergeGateHooks) NextFlowRun(string) (time.Time, bool) { return time.Time{}, false }

// A flow that waits for an owned run's completion starts only while the Desktop owner is
// valid: an owner revoked while the run waits in the event dispatcher starts nothing. Either
// way the request gives the owner back.
func TestMergeRevokedOwnerStartsNoQueuedFlowDependent(t *testing.T) {
	ConfigureRuntimePermissions(defaultRuntimePermissionsForTests())
	for _, revoke := range []bool{false, true} {
		t.Run(map[bool]string{false: "valid", true: "revoked"}[revoke], func(t *testing.T) {
			m := NewMissionManagerV2(tempSystemTaskDir(t), nil)
			defer m.Stop()
			hooks := &mergeGateHooks{gate: make(chan struct{}), starts: make(chan flowStartCall, 8)}
			m.SetFlowHooks(hooks)
			if err := m.Create(&MissionV2{ID: "first", Name: "First", Prompt: "test", Enabled: true, ExecutionType: ExecutionManual}); err != nil {
				t.Fatal(err)
			}
			flowID := publishTestFlow(t, m,
				FlowTriggerSpec{NodeID: "n_aaaaaaaa", TriggerType: TriggerDeviceConnected, TriggerConfig: &TriggerConfig{}},
				FlowTriggerSpec{NodeID: "n_bbbbbbbb", TriggerType: TriggerMissionCompleted, TriggerConfig: &TriggerConfig{SourceMissionID: "first"}})
			// A device event's run blocks the dispatcher, so the dependent run waits in its queue.
			m.NotifyDeviceEvent("device_connected", "dev-1", "Laptop")
			select {
			case c := <-hooks.starts:
				if c.nodeID != "n_aaaaaaaa" {
					t.Fatalf("first start = %+v", c)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("the device run did not start")
			}
			ownerCtx, revokeOwner := context.WithCancel(context.Background())
			defer revokeOwner()
			var releases atomic.Int32
			if err := m.QueueOwnedMission(ownerCtx, func() { releases.Add(1) }, "first", "manual", ""); err != nil {
				t.Fatal(err)
			}
			item, ok := m.queue.TryStartNext()
			if !ok {
				t.Fatal("not queued")
			}
			m.mu.Lock()
			m.missions["first"].Status = MissionStatusRunning
			m.mu.Unlock()
			m.OnMissionComplete("first", MissionResultSuccess, "done")
			item.releaseOwner()
			if releases.Load() != 0 {
				t.Fatal("the queued flow run did not retain the owner")
			}
			if revoke {
				revokeOwner()
			}
			close(hooks.gate)
			select {
			case c := <-hooks.starts:
				if revoke {
					t.Fatalf("a run started for a revoked owner: %+v", c)
				}
				if c.missionID != flowID || c.nodeID != "n_bbbbbbbb" {
					t.Fatalf("dependent start = %+v", c)
				}
			case <-time.After(200 * time.Millisecond):
				if !revoke {
					t.Fatal("the dependent flow run did not start")
				}
			}
			eventually(t, "the owner is given back", func() bool { return releases.Load() == 1 })
		})
	}
}

// After Stop, a webhook, email or MQTT delivery (fireFlowEvent) and a flow cron job
// (fireFlowSchedule) start no flow run.
func TestMergeStoppedManagerFiresNoFlowTriggers(t *testing.T) {
	dir := tempSystemTaskDir(t)
	cronMgr := NewCronManager(dir)
	t.Cleanup(func() { _ = cronMgr.Close() })
	mm := NewMissionManagerV2(dir, cronMgr)
	hooks := newFakeFlowHooks()
	mm.SetFlowHooks(hooks)
	id := publishTestFlow(t, mm,
		FlowTriggerSpec{NodeID: "n_aaaaaaaa", TriggerType: FlowTriggerSchedule, Schedule: "0 7 * * *"},
		FlowTriggerSpec{NodeID: "n_bbbbbbbb", TriggerType: TriggerWebhook, TriggerConfig: &TriggerConfig{WebhookID: "hook-1"}})
	match := func(c *TriggerConfig) bool { return c.WebhookID == "hook-1" }
	mm.fireFlowEvent(id, "n_bbbbbbbb", TriggerWebhook, "webhook", "{}", match)
	if c := hooks.waitStart(t); c.nodeID != "n_bbbbbbbb" {
		t.Fatalf("start before Stop = %+v", c)
	}
	mm.Stop()
	mm.fireFlowEvent(id, "n_bbbbbbbb", TriggerWebhook, "webhook", "{}", match)
	if !mm.fireFlowSchedule(id, "n_aaaaaaaa") {
		t.Fatal("a stopped manager reported the flow cron job as stale")
	}
	hooks.expectNoStart(t)
}

// mergeCancelOnMarshal cancels the manager while notifyFlowsForOwnerLocked encodes the event
// data, that is between its context check and its sends.
type mergeCancelOnMarshal struct{ cancel context.CancelFunc }

func (c mergeCancelOnMarshal) MarshalJSON() ([]byte, error) {
	c.cancel()
	return []byte(`{}`), nil
}

// A flow run queued after Stop cancelled the manager, when the dispatcher has drained its
// queue and ended, gives its Desktop owner back at once instead of keeping it for good.
func TestMergeLateFlowRequestReleasesItsOwner(t *testing.T) {
	m := NewMissionManagerV2(tempSystemTaskDir(t), nil)
	defer m.Stop()
	m.SetFlowHooks(newFakeFlowHooks())
	publishTestFlow(t, m, FlowTriggerSpec{NodeID: "n_aaaaaaaa", TriggerType: TriggerDeviceConnected, TriggerConfig: &TriggerConfig{}})
	var retained, released atomic.Int32
	owner := QueueItem{RequiresOwner: true, ownerContext: context.Background(), retainOwner: func() context.CancelFunc {
		retained.Add(1)
		return func() { released.Add(1) }
	}}
	m.mu.Lock()
	// A queue no dispatcher reads any more: the dispatcher drained it and ended.
	m.flowEvents = make(chan flowRunRequest, 4)
	m.notifyFlowsForOwnerLocked(TriggerDeviceConnected, flowEvent{}, mergeCancelOnMarshal{cancel: m.cancel}, owner)
	queued := len(m.flowEvents)
	m.mu.Unlock()
	if retained.Load() != 1 || released.Load() != 1 || queued != 0 {
		t.Fatalf("retained %d, released %d, still queued %d; want the late run dropped and its owner given back",
			retained.Load(), released.Load(), queued)
	}
}
