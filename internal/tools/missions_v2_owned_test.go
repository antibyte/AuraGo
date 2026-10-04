package tools

import (
	"aurago/internal/fileutil"
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestOwnedMissionCompletionRetainsFollowupOwner(t *testing.T) {
	ConfigureRuntimePermissions(defaultRuntimePermissionsForTests())
	for _, revoke := range []bool{false, true} {
		t.Run(map[bool]string{false: "complete", true: "revoke"}[revoke], func(t *testing.T) {
			m := NewMissionManagerV2(tempSystemTaskDir(t), nil)
			defer m.Stop()
			for _, mission := range []*MissionV2{
				{ID: "first", Name: "First", Prompt: "test", Enabled: true, ExecutionType: ExecutionManual},
				{ID: "next", Name: "Next", Prompt: "test", Enabled: true, ExecutionType: ExecutionTriggered, TriggerType: TriggerMissionCompleted, TriggerConfig: &TriggerConfig{SourceMissionID: "first"}},
			} {
				if err := m.Create(mission); err != nil {
					t.Fatal(err)
				}
			}
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
			m.missions["first"].Status = MissionStatusRunning
			refused.Store(revoke)
			m.OnMissionComplete("first", MissionResultSuccess, "late success")
			item.releaseOwner()
			q, _ := m.GetQueue()
			if revoke {
				if len(q.List()) != 0 || m.missions["first"].LastResult != MissionResultError || releases.Load() != 1 {
					t.Fatal("revocation allowed success, followup, or leaked ownership")
				}
				return
			}
			items := q.List()
			if len(items) != 1 || !items[0].RequiresOwner || items[0].ownerContext.Err() != nil || releases.Load() != 0 {
				t.Fatal("followup lost the live owner")
			}
			next, ok := q.TryStartNext()
			if !ok {
				t.Fatal("followup did not start")
			}
			m.missions["next"].Status = MissionStatusRunning
			refused.Store(true)
			m.OnMissionComplete("next", MissionResultSuccess, "revoked followup")
			next.releaseOwner()
			if m.missions["next"].LastResult != MissionResultError || releases.Load() != 1 {
				t.Fatal("followup ignored revocation or leaked owner")
			}
		})
	}
}

func TestOwnedMissionRevocationAndRestartNeverReplay(t *testing.T) {
	ConfigureRuntimePermissions(defaultRuntimePermissionsForTests())
	for _, running := range []bool{false, true} {
		t.Run(map[bool]string{false: "queued", true: "running"}[running], func(t *testing.T) {
			dir := tempSystemTaskDir(t)
			m := NewMissionManagerV2(dir, nil)
			defer m.Stop()
			if err := m.Create(&MissionV2{ID: "owned", Name: "Owned", Prompt: "test", Enabled: true, ExecutionType: ExecutionManual}); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			var released atomic.Bool
			if err := m.QueueOwnedMission(ctx, func() { cancel(); released.Store(true) }, "owned", "manual", ""); err != nil {
				t.Fatal(err)
			}
			if running {
				if _, ok := m.queue.TryStartNext(); !ok {
					t.Fatal("not queued")
				}
				if err := m.saveQueueLocked(); err != nil {
					t.Fatal(err)
				}
				if active, owned := m.ActiveOwnerContext("owned"); !owned || active == nil {
					t.Fatal("owner lost")
				}
			}
			cancel()
			called := make(chan struct{}, 1)
			m.SetCallback(func(string, string) { called <- struct{}{} })
			if !running {
				m.processNext()
				if !released.Load() {
					t.Fatal("owner leaked")
				}
			}
			select {
			case <-called:
				t.Fatal("revoked invocation ran")
			default:
			}
			restarted := NewMissionManagerV2(dir, nil)
			restarted.SetCallback(func(string, string) { called <- struct{}{} })
			if err := restarted.Start(); err != nil {
				t.Fatal(err)
			}
			defer restarted.Stop()
			if q, current := restarted.GetQueue(); current != "" || len(q.List()) != 0 {
				t.Fatal("ephemeral invocation replayed after restart")
			}
			select {
			case <-called:
				t.Fatal("restart invoked ownerless work")
			case <-time.After(20 * time.Millisecond):
			}
		})
	}
}

func TestOwnedMissionCarriesCancellationThroughCallback(t *testing.T) {
	ConfigureRuntimePermissions(defaultRuntimePermissionsForTests())
	m := NewMissionManagerV2(tempSystemTaskDir(t), nil)
	defer m.Stop()
	if err := m.Create(&MissionV2{ID: "owned", Name: "Owned", Prompt: "test", Enabled: true, ExecutionType: ExecutionManual}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	released := make(chan struct{})
	if err := m.QueueOwnedMission(ctx, func() { cancel(); close(released) }, "owned", "manual", ""); err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{})
	m.SetCallback(func(_ string, id string) {
		owner, ok := m.ActiveOwnerContext(id)
		if !ok || owner == nil {
			t.Error("missing owner")
			return
		}
		close(started)
		<-owner.Done()
		m.OnMissionComplete(id, MissionResultError, "cancelled")
	})
	m.processNext()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("not started")
	}
	cancel()
	select {
	case <-released:
	case <-time.After(time.Second):
		t.Fatal("owner did not drain")
	}
}

func TestOwnedMissionCompletionCrashNeverReplaysStaleStatus(t *testing.T) {
	ConfigureRuntimePermissions(defaultRuntimePermissionsForTests())
	dir := tempSystemTaskDir(t)
	m := NewMissionManagerV2(dir, nil)
	defer m.Stop()
	if err := m.Create(&MissionV2{ID: "owned", Name: "Owned", Prompt: "test", Enabled: true, ExecutionType: ExecutionManual}); err != nil {
		t.Fatal(err)
	}
	if err := m.QueueOwnedMission(context.Background(), nil, "owned", "manual", ""); err != nil {
		t.Fatal(err)
	}
	item, _ := m.queue.TryStartNext()
	defer item.releaseOwner()
	m.missions["owned"].Status = MissionStatusRunning
	if err := m.save(); err != nil {
		t.Fatal(err)
	}
	// Crash after persisting completion in the queue, before persisting idle in missions.json.
	m.queue.Done()
	if err := m.saveQueueLocked(); err != nil {
		t.Fatal(err)
	}
	restarted := NewMissionManagerV2(dir, nil)
	defer restarted.Stop()
	if err := restarted.Start(); err != nil {
		t.Fatal(err)
	}
	q, running := restarted.GetQueue()
	if running != "" || len(q.List()) != 0 {
		t.Fatal("stale status resurrected a completed Desktop invocation")
	}
	// A deliberate independent invocation can supersede the tombstone.
	if err := restarted.RunNow("owned"); err != nil {
		t.Fatal(err)
	}
	if q.persistedSnapshot().NonReplayableIDs != nil && len(q.persistedSnapshot().NonReplayableIDs) != 0 {
		t.Fatal("independent invocation retained old owner marker")
	}
}

func TestOwnedMissionCompletionDoesNotInvertPublicationLock(t *testing.T) {
	ConfigureRuntimePermissions(defaultRuntimePermissionsForTests())
	m := NewMissionManagerV2(tempSystemTaskDir(t), nil)
	defer m.Stop()
	if err := m.Create(&MissionV2{ID: "owned", Name: "Owned", Prompt: "test", Enabled: true, ExecutionType: ExecutionManual}); err != nil {
		t.Fatal(err)
	}
	var gate sync.Mutex
	entered := make(chan struct{})
	ctx := fileutil.WithPublicationGate(context.Background(), func(publish func() error) error {
		close(entered)
		gate.Lock()
		defer gate.Unlock()
		return publish()
	})
	if err := m.QueueOwnedMission(ctx, nil, "owned", "manual", ""); err != nil {
		t.Fatal(err)
	}
	item, _ := m.queue.TryStartNext()
	defer item.releaseOwner()
	m.missions["owned"].Status = MissionStatusRunning
	gate.Lock()
	completed := make(chan struct{})
	go func() { m.OnMissionComplete("owned", MissionResultSuccess, "done"); close(completed) }()
	<-entered
	prepared := make(chan struct{})
	go func() { m.SetPreparationStatus("owned", string(PrepStatusPreparing)); close(prepared) }()
	select {
	case <-prepared:
		gate.Unlock()
	case <-time.After(time.Second):
		gate.Unlock()
		<-completed
		t.Fatal("completion held the mission lock while waiting for publication")
	}
	select {
	case <-completed:
	case <-time.After(time.Second):
		t.Fatal("completion did not drain")
	}
}
