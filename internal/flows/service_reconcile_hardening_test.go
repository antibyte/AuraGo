package flows

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Task 1c-16: Service.ReconcileMissions heals Mission Control after a crash.

// c16Logs collects log output from any goroutine.
type c16Logs struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (l *c16Logs) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.Write(p)
}

func (l *c16Logs) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.String()
}

func (l *c16Logs) logger() *slog.Logger {
	return slog.New(slog.NewTextHandler(l, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

// c16PlainBridge is the fake bridge without the MissionReconciler extension; it counts syncs.
type c16PlainBridge struct {
	*fakeBridge
	syncs atomic.Int32
}

func (b *c16PlainBridge) SyncFlowMission(missionID, name string, bindings []TriggerBinding) error {
	b.syncs.Add(1)
	return b.fakeBridge.SyncFlowMission(missionID, name, bindings)
}

// c16Bridge adds the MissionReconciler extension to c16PlainBridge. afterSnapshot, when
// set, runs once after the next FlowMissions snapshot was taken.
type c16Bridge struct {
	c16PlainBridge
	afterSnapshot func()
}

func (b *c16Bridge) FlowMissions() map[string]string {
	b.mu.Lock()
	out := map[string]string{}
	for id, m := range b.missions {
		out[id] = m.flowID
	}
	hook := b.afterSnapshot
	b.afterSnapshot = nil
	b.mu.Unlock()
	if hook != nil {
		hook()
	}
	return out
}

func (b *c16Bridge) FlowMissionInSync(missionID, name string, bindings []TriggerBinding) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	m := b.missions[missionID]
	return m != nil && m.name == name && reflect.DeepEqual(m.bindings, bindings)
}

func newC16Bridge() *c16Bridge {
	return &c16Bridge{c16PlainBridge: c16PlainBridge{fakeBridge: newFakeBridge()}}
}

// diverge makes a mission look like the sync after a publish never happened.
func (b *c16PlainBridge) diverge(missionID string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if m := b.missions[missionID]; m != nil {
		m.name, m.bindings = "Alt", nil
	}
}

func (b *c16PlainBridge) setEnabled(missionID string, enabled bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.missions[missionID].enabled = enabled
}

func (b *c16PlainBridge) drop(missionID string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.missions, missionID)
}

func c16Service(t *testing.T, clock Clock, bridge MissionBridge, logger *slog.Logger) *Service {
	t.Helper()
	svc := &Services{Tools: &fakeTools{}, Clock: clock, Location: time.UTC}
	s := NewService(openTestStore(t), catalogRegistry(t, fullEnv()), svc, bridge, ServiceConfig{}, logger)
	t.Cleanup(func() { _ = s.Shutdown(context.Background()) })
	return s
}

// c16Publish creates a flow from doc and publishes it.
func c16Publish(t *testing.T, s *Service, doc *Flow) *FlowRecord {
	t.Helper()
	ctx := context.Background()
	rec, err := s.CreateFlow(ctx, CreateRequest{Import: doc})
	if err != nil {
		t.Fatalf("CreateFlow: %v", err)
	}
	pub, _, err := s.Publish(ctx, rec.ID, rec.DraftRevision)
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}
	return pub
}

func c16Timers(t *testing.T, s *Service, flowID string) []TimerRecord {
	t.Helper()
	all, err := s.Store().ListTimers(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var out []TimerRecord
	for _, tm := range all {
		if tm.FlowID == flowID {
			out = append(out, tm)
		}
	}
	return out
}

func TestC16ReconcileHealsADivergedMission(t *testing.T) {
	logs := &c16Logs{}
	clock := newFakeClock(time.Date(2026, 10, 3, 7, 0, 0, 0, time.UTC))
	bridge := newC16Bridge()
	s := c16Service(t, clock, bridge, logs.logger())
	doc, _, _, _ := scheduleFlow("Plan")
	pub := c16Publish(t, s, doc)
	want, _ := bridge.mission(pub.MissionID)
	if bridge.syncs.Load() != 1 || len(want.bindings) != 3 {
		t.Fatalf("after publish: syncs %d bindings %+v", bridge.syncs.Load(), want.bindings)
	}
	bridge.diverge(pub.MissionID)
	// The one-off date (09:00) has passed: the bindings must still come out as published.
	clock.Advance(3 * time.Hour)

	if err := s.ReconcileMissions(context.Background()); err != nil {
		t.Fatalf("ReconcileMissions: %v", err)
	}
	got, _ := bridge.mission(pub.MissionID)
	if got.name != "Plan" || !reflect.DeepEqual(got.bindings, want.bindings) || bridge.syncs.Load() != 2 {
		t.Fatalf("after reconcile: mission %+v (want bindings %+v), syncs %d", got, want.bindings, bridge.syncs.Load())
	}
	if out := logs.String(); !strings.Contains(out, "were out of date and were updated") || !strings.Contains(out, "flow missions reconciled") {
		t.Fatalf("log:\n%s", out)
	}
	// In sync now: a second pass changes nothing.
	if err := s.ReconcileMissions(context.Background()); err != nil {
		t.Fatalf("second ReconcileMissions: %v", err)
	}
	if bridge.syncs.Load() != 2 {
		t.Fatalf("a mission in sync was synced again: %d syncs", bridge.syncs.Load())
	}
}

func TestC16ReconcileWithoutTheExtensionSyncsEveryPublishedFlow(t *testing.T) {
	clock := newFakeClock(time.Date(2026, 10, 3, 7, 0, 0, 0, time.UTC))
	bridge := &c16PlainBridge{fakeBridge: newFakeBridge()}
	s := c16Service(t, clock, bridge, discardLogger())
	pub := c16Publish(t, s, simpleFlow("Gruss"))
	if _, err := s.CreateFlow(context.Background(), CreateRequest{Name: "Entwurf"}); err != nil {
		t.Fatal(err)
	}
	bridge.diverge(pub.MissionID)
	if err := s.ReconcileMissions(context.Background()); err != nil {
		t.Fatalf("ReconcileMissions: %v", err)
	}
	if m, _ := bridge.mission(pub.MissionID); m.name != "Gruss" || len(m.bindings) != 1 || bridge.syncs.Load() != 2 {
		t.Fatalf("mission %+v, syncs %d (the unpublished flow must not be synced)", m, bridge.syncs.Load())
	}
}

func TestC16ReconcileTakesTheFlowLock(t *testing.T) {
	logs := &c16Logs{}
	clock := newFakeClock(time.Date(2026, 10, 3, 7, 0, 0, 0, time.UTC))
	bridge := newC16Bridge()
	s := c16Service(t, clock, bridge, logs.logger())
	pub := c16Publish(t, s, simpleFlow("Gruss"))
	bridge.diverge(pub.MissionID)
	ctx := context.Background()

	unlock, err := s.locks.lock(ctx, pub.ID)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- s.ReconcileMissions(ctx) }()
	select {
	case err := <-done:
		t.Fatalf("ReconcileMissions returned while the flow was locked: %v", err)
	case <-time.After(150 * time.Millisecond):
	}
	if m, _ := bridge.mission(pub.MissionID); m.name != "Alt" {
		t.Fatalf("the mission changed while the flow was locked: %+v", m)
	}
	unlock()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("ReconcileMissions: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("ReconcileMissions did not finish after the lock was released")
	}
	if m, _ := bridge.mission(pub.MissionID); m.name != "Gruss" {
		t.Fatalf("not healed after the lock was released: %+v", m)
	}

	// A flow that stays busy is skipped after reconcileLockWait.
	previous := reconcileLockWait
	reconcileLockWait = 50 * time.Millisecond
	t.Cleanup(func() { reconcileLockWait = previous })
	bridge.diverge(pub.MissionID)
	unlock, err = s.locks.lock(ctx, pub.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ReconcileMissions(ctx); err != nil {
		t.Fatalf("ReconcileMissions with a busy flow: %v", err)
	}
	if m, _ := bridge.mission(pub.MissionID); m.name != "Alt" || !strings.Contains(logs.String(), "a flow stayed busy") {
		t.Fatalf("busy flow: mission %+v, log:\n%s", m, logs.String())
	}

	// The caller's context ends the wait.
	reconcileLockWait = time.Minute
	cctx, cancel := context.WithCancel(ctx)
	go func() { done <- s.ReconcileMissions(cctx) }()
	time.Sleep(50 * time.Millisecond)
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("ReconcileMissions after cancel = %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("ReconcileMissions ignored its context")
	}
	unlock()
}

func TestC16ReconcileReportsMissingMissionsAndOrphans(t *testing.T) {
	logs := &c16Logs{}
	clock := newFakeClock(time.Date(2026, 10, 3, 7, 0, 0, 0, time.UTC))
	bridge := newC16Bridge()
	s := c16Service(t, clock, bridge, logs.logger())
	ctx := context.Background()
	draft, err := s.CreateFlow(ctx, CreateRequest{Name: "Entwurf"})
	if err != nil {
		t.Fatal(err)
	}
	pub := c16Publish(t, s, simpleFlow("Gruss"))
	healthy := c16Publish(t, s, simpleFlow("Gesund"))
	bridge.drop(draft.MissionID)
	bridge.drop(pub.MissionID)
	bridge.mu.Lock()
	bridge.missions["mission_orphan"] = &fakeMission{flowID: "flow_gone", name: "Waise"}
	bridge.mu.Unlock()
	syncs := bridge.syncs.Load()

	if err := s.ReconcileMissions(ctx); err != nil {
		t.Fatalf("ReconcileMissions: %v", err)
	}
	out := logs.String()
	for _, want := range []string{
		"flow=" + draft.ID, "flow=" + pub.ID, "mission of a flow is gone",
		"mission_id=mission_orphan", "flow=flow_gone", "whose flow is gone", "problems=3",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("log lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "flow="+healthy.ID) {
		t.Fatalf("the healthy flow was reported:\n%s", out)
	}
	// Nothing is deleted or synced.
	for _, id := range []string{draft.ID, pub.ID} {
		if _, err := s.GetFlow(ctx, id); err != nil {
			t.Fatalf("flow %s after reconcile: %v", id, err)
		}
	}
	if _, ok := bridge.mission("mission_orphan"); !ok || bridge.syncs.Load() != syncs {
		t.Fatalf("orphan kept %v, syncs %d -> %d", ok, syncs, bridge.syncs.Load())
	}
}

func TestC16ReconcileSkipsAnOrphanDeletedMeanwhile(t *testing.T) {
	logs := &c16Logs{}
	bridge := newC16Bridge()
	s := c16Service(t, newFakeClock(time.Date(2026, 10, 3, 7, 0, 0, 0, time.UTC)), bridge, logs.logger())
	c16Publish(t, s, simpleFlow("Gruss"))
	bridge.mu.Lock()
	bridge.missions["mission_orphan"] = &fakeMission{flowID: "flow_gone", name: "Waise"}
	// Mission Control deletes the mission right after the first snapshot.
	bridge.afterSnapshot = func() { bridge.drop("mission_orphan") }
	bridge.mu.Unlock()
	if err := s.ReconcileMissions(context.Background()); err != nil {
		t.Fatalf("ReconcileMissions: %v", err)
	}
	if out := logs.String(); strings.Contains(out, "whose flow is gone") || !strings.Contains(out, "problems=0") {
		t.Fatalf("a mission deleted meanwhile was reported:\n%s", out)
	}
}

func TestC16OpenStoreRefusesANewerSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "flows.db")
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	if _, err := raw.Exec(`CREATE TABLE flows_meta (key TEXT PRIMARY KEY, value TEXT NOT NULL);
		INSERT INTO flows_meta (key, value) VALUES ('schema_version', '2')`); err != nil {
		t.Fatal(err)
	}
	setVersion := func(v string) {
		t.Helper()
		if _, err := raw.Exec(`UPDATE flows_meta SET value = ? WHERE key = 'schema_version'`, v); err != nil {
			t.Fatal(err)
		}
	}
	for version, want := range map[string]string{
		"2":    "flows database schema 2 is newer than this AuraGo supports (1)",
		"zwei": "is not a number",
	} {
		setVersion(version)
		if s, err := OpenStore(path, discardLogger()); err == nil || !strings.Contains(err.Error(), want) {
			if s != nil {
				_ = s.Close()
			}
			t.Fatalf("OpenStore at schema %q = %v, want %q", version, err, want)
		}
		// The file is untouched: no migration ran.
		var tables int
		if err := raw.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'flows'`).Scan(&tables); err != nil || tables != 0 {
			t.Fatalf("schema %q: flows table count %d (%v), want 0", version, tables, err)
		}
		var stored string
		if err := raw.QueryRow(`SELECT value FROM flows_meta WHERE key = 'schema_version'`).Scan(&stored); err != nil || stored != version {
			t.Fatalf("schema %q: stored version %q (%v)", version, stored, err)
		}
	}
	// The supported version and a database without the row open and migrate.
	setVersion("1")
	s, err := OpenStore(path, discardLogger())
	if err != nil {
		t.Fatalf("OpenStore at schema 1: %v", err)
	}
	_ = s.Close()
	if _, err := raw.Exec(`DELETE FROM flows_meta`); err != nil {
		t.Fatal(err)
	}
	s, err = OpenStore(path, discardLogger())
	if err != nil {
		t.Fatalf("OpenStore without a version row: %v", err)
	}
	defer s.Close()
	var stored string
	if err := s.db.QueryRow(`SELECT value FROM flows_meta WHERE key = 'schema_version'`).Scan(&stored); err != nil || stored != "1" {
		t.Fatalf("version row after migration = %q (%v)", stored, err)
	}
}

func TestC16ReconcileWithoutTheExtensionReportsAFailedSync(t *testing.T) {
	logs := &c16Logs{}
	bridge := &c16PlainBridge{fakeBridge: newFakeBridge()}
	s := c16Service(t, newFakeClock(time.Date(2026, 10, 3, 7, 0, 0, 0, time.UTC)), bridge, logs.logger())
	pub := c16Publish(t, s, simpleFlow("Gruss"))
	bridge.drop(pub.MissionID)
	if err := s.ReconcileMissions(context.Background()); err != nil {
		t.Fatalf("ReconcileMissions: %v", err)
	}
	if out := logs.String(); !strings.Contains(out, "could not be updated") || !strings.Contains(out, "mission_id="+pub.MissionID) {
		t.Fatalf("log:\n%s", out)
	}
}

func TestC16ReconcileTimersFollowTheMissionSwitch(t *testing.T) {
	clock := newFakeClock(time.Date(2026, 10, 3, 7, 0, 0, 0, time.UTC))
	bridge := newC16Bridge()
	s := c16Service(t, clock, bridge, discardLogger())
	ctx := context.Background()
	doc, _, when, _ := scheduleFlow("Plan")
	pub := c16Publish(t, s, doc)
	if err := s.SetEnabled(ctx, pub.ID, true); err != nil {
		t.Fatal(err)
	}
	nine := time.Date(2026, 10, 3, 9, 0, 0, 0, time.UTC)
	if tm := c16Timers(t, s, pub.ID); len(tm) != 1 || !tm[0].FireAt.Equal(nine) {
		t.Fatalf("timers after enable: %+v", tm)
	}

	// Enabled, but the timers were lost (a crash between the switch and the timers).
	if err := s.Store().ReplaceTimers(ctx, pub.ID, nil); err != nil {
		t.Fatal(err)
	}
	if err := s.ReconcileMissions(ctx); err != nil {
		t.Fatal(err)
	}
	if tm := c16Timers(t, s, pub.ID); len(tm) != 1 || tm[0].NodeID != when || !tm[0].FireAt.Equal(nine) {
		t.Fatalf("timers after reconcile: %+v", tm)
	}

	// A stored timer of an enabled flow is left alone, even one that is due now.
	due := TimerRecord{FlowID: pub.ID, NodeID: when, FireAt: time.Date(2026, 10, 3, 6, 59, 0, 0, time.UTC)}
	if err := s.Store().ReplaceTimers(ctx, pub.ID, []TimerRecord{due}); err != nil {
		t.Fatal(err)
	}
	if err := s.ReconcileMissions(ctx); err != nil {
		t.Fatal(err)
	}
	if tm := c16Timers(t, s, pub.ID); len(tm) != 1 || !tm[0].FireAt.Equal(due.FireAt) {
		t.Fatalf("a stored timer was re-bound: %+v", tm)
	}

	// Stale timers (a crash between the store publish and armTimers) are armed again: a
	// node of an older revision, or the right node with another repeat.
	for name, stale := range map[string]TimerRecord{
		"old node":     {FlowID: pub.ID, NodeID: "n_zzzzzzzz", FireAt: nine.Add(time.Hour)},
		"other repeat": {FlowID: pub.ID, NodeID: when, FireAt: nine.Add(time.Hour), Repeat: RepeatYearly},
	} {
		if err := s.Store().ReplaceTimers(ctx, pub.ID, []TimerRecord{stale}); err != nil {
			t.Fatal(err)
		}
		if err := s.ReconcileMissions(ctx); err != nil {
			t.Fatal(err)
		}
		if tm := c16Timers(t, s, pub.ID); len(tm) != 1 || tm[0].NodeID != when || !tm[0].FireAt.Equal(nine) || tm[0].Repeat != "" {
			t.Fatalf("%s: timers after reconcile: %+v", name, tm)
		}
	}

	// Once the one-off time has passed, its stored timer is due and stays; once it fired
	// (and was deleted) nothing is armed again.
	clock.Advance(3 * time.Hour)
	if err := s.ReconcileMissions(ctx); err != nil {
		t.Fatal(err)
	}
	if tm := c16Timers(t, s, pub.ID); len(tm) != 1 || !tm[0].FireAt.Equal(nine) {
		t.Fatalf("a due one-off timer was dropped: %+v", tm)
	}
	if err := s.Store().ReplaceTimers(ctx, pub.ID, nil); err != nil {
		t.Fatal(err)
	}
	if err := s.ReconcileMissions(ctx); err != nil {
		t.Fatal(err)
	}
	if tm := c16Timers(t, s, pub.ID); len(tm) != 0 {
		t.Fatalf("a fired one-off timer came back: %+v", tm)
	}
	if err := s.Store().ReplaceTimers(ctx, pub.ID, []TimerRecord{{FlowID: pub.ID, NodeID: when, FireAt: nine}}); err != nil {
		t.Fatal(err)
	}

	// Disabled in Mission Control while timers stayed armed.
	bridge.setEnabled(pub.MissionID, false)
	if err := s.ReconcileMissions(ctx); err != nil {
		t.Fatal(err)
	}
	if tm := c16Timers(t, s, pub.ID); len(tm) != 0 {
		t.Fatalf("timers of a disabled flow after reconcile: %+v", tm)
	}
}

func TestC16ReconcileStopsAtShutdown(t *testing.T) {
	bridge := newC16Bridge()
	s := c16Service(t, newFakeClock(time.Date(2026, 10, 3, 7, 0, 0, 0, time.UTC)), bridge, discardLogger())
	pub := c16Publish(t, s, simpleFlow("Gruss"))
	ctx := context.Background()
	unlock, err := s.locks.lock(ctx, pub.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	done := make(chan error, 1)
	go func() { done <- s.ReconcileMissions(ctx) }()
	time.Sleep(50 * time.Millisecond) // waiting for the flow lock now

	sctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	begin := time.Now()
	if err := s.Shutdown(sctx); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	if waited := time.Since(begin); waited > time.Second {
		t.Fatalf("Shutdown waited %s for a reconciliation stuck on a lock", waited)
	}
	select {
	case err := <-done:
		if !errors.Is(err, ErrRunnerClosed) {
			t.Fatalf("ReconcileMissions after Shutdown = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("the reconciliation did not end with Shutdown")
	}
	if err := s.ReconcileMissions(ctx); !errors.Is(err, ErrRunnerClosed) {
		t.Fatalf("ReconcileMissions after Shutdown = %v", err)
	}
}
