package flows

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

// svcLogs keeps every log record for assertions.
type svcLogs struct {
	mu      sync.Mutex
	records []slog.Record
}

func (l *svcLogs) Enabled(context.Context, slog.Level) bool { return true }

func (l *svcLogs) Handle(_ context.Context, r slog.Record) error {
	l.mu.Lock()
	l.records = append(l.records, r.Clone())
	l.mu.Unlock()
	return nil
}

func (l *svcLogs) WithAttrs([]slog.Attr) slog.Handler { return l }

func (l *svcLogs) WithGroup(string) slog.Handler { return l }

// messages returns the messages of the records at level min or above that contain part.
func (l *svcLogs) messages(min slog.Level, part string) []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	var out []string
	for _, r := range l.records {
		if r.Level >= min && strings.Contains(r.Message, part) {
			out = append(out, r.Level.String()+" "+r.Message)
		}
	}
	return out
}

// svcBlockingTools holds every tool call until the run is cancelled and the test lets go,
// so a cancelled node returns exactly when the test wants it to.
type svcBlockingTools struct {
	called  chan string
	release chan struct{}
	once    sync.Once
}

func newSvcBlockingTools() *svcBlockingTools {
	return &svcBlockingTools{called: make(chan string, 8), release: make(chan struct{})}
}

func (b *svcBlockingTools) InvokeTool(ctx context.Context, req ToolRequest) (ToolResponse, error) {
	b.called <- req.RunID
	<-ctx.Done()
	<-b.release
	return ToolResponse{}, ctx.Err()
}

func (b *svcBlockingTools) letGo() { b.once.Do(func() { close(b.release) }) }

// svcObserveFinished reports every run the runner finishes, after the Service's own
// hook has run. Call it before the first run starts.
func svcObserveFinished(s *Service) <-chan RunRecord {
	ch := make(chan RunRecord, 16)
	hook := s.runner.hooks.OnRunFinished
	s.runner.hooks.OnRunFinished = func(rec RunRecord, res RunResult) {
		if hook != nil {
			hook(rec, res)
		}
		ch <- rec
	}
	return ch
}

func svcLockEntries(s *Service) int {
	s.locks.mu.Lock()
	defer s.locks.mu.Unlock()
	return len(s.locks.locks)
}

func TestFlowLocksAreKeyedAndCancellable(t *testing.T) {
	var l flowLocks
	ctx := context.Background()
	unlockA, err := l.lock(ctx, "a")
	if err != nil {
		t.Fatal(err)
	}
	unlockB, err := l.lock(ctx, "b") // another flow is not blocked
	if err != nil {
		t.Fatal(err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := l.lock(cancelled, "a"); !errors.Is(err, context.Canceled) {
		t.Fatalf("lock of a held flow with an ended context = %v", err)
	}
	l.mu.Lock()
	refs := l.locks["a"].refs
	l.mu.Unlock()
	if refs != 1 {
		t.Fatalf("refs after a waiter gave up = %d, want 1", refs)
	}
	got := make(chan func(), 1)
	go func() {
		unlock, err := l.lock(ctx, "a")
		if err != nil {
			unlock = func() {}
		}
		got <- unlock
	}()
	unlockA()
	select {
	case unlock := <-got:
		unlock()
	case <-time.After(5 * time.Second):
		t.Fatal("the lock was not handed on after its release")
	}
	unlockB()
	if n := len(l.locks); n != 0 {
		t.Fatalf("%d lock entries left after all locks were released", n)
	}
}

// A context that has ended never takes a lock, not even a free one, and leaves no entry.
// Releasing twice is harmless: the second call neither blocks nor frees the lock of the
// next holder.
func TestFlowLocksRefuseEndedContextsAndReleaseOnce(t *testing.T) {
	var l flowLocks
	ctx := context.Background()
	ended, cancel := context.WithCancel(ctx)
	cancel()
	if unlock, err := l.lock(ended, "a"); !errors.Is(err, context.Canceled) || unlock != nil {
		t.Fatalf("lock of a free flow with an ended context = %v (unlock set: %v)", err, unlock != nil)
	}
	if n := len(l.locks); n != 0 {
		t.Fatalf("%d lock entries after a refused lock", n)
	}

	first, err := l.lock(ctx, "a")
	if err != nil {
		t.Fatal(err)
	}
	first()
	next, err := l.lock(ctx, "a")
	if err != nil {
		t.Fatal(err)
	}
	if err := svcWithin(t, 5*time.Second, "a second release", func() error { first(); return nil }); err != nil {
		t.Fatal(err)
	}
	short, cancelShort := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancelShort()
	if _, err := l.lock(short, "a"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("lock while the next holder has it = %v; a repeated release freed it", err)
	}
	next()
	next()
	if n := len(l.locks); n != 0 {
		t.Fatalf("%d lock entries after all releases", n)
	}
	again, err := l.lock(ctx, "a")
	if err != nil {
		t.Fatalf("lock after repeated releases: %v", err)
	}
	again()
}

// While SetEnabled of flow A waits in the bridge, a Publish of A waits for it (and gives
// up with its context), operations on flow B go ahead, and a Publish of A's next
// revision that waited runs after SetEnabled finished, so the enabled flow ends up with
// the timers and bindings of the new revision.
func TestServiceSerializesOperationsPerFlow(t *testing.T) {
	fx := newSvcFixture(t)
	ctx := context.Background()
	a := fx.published(t, "A", svcDateTimeFlow("A", 10, "2026-10-04 09:00"))
	b := fx.published(t, "B", svcDateTimeFlow("B", 20, "2026-10-04 10:00"))
	release := fx.bridge.hold(a.MissionID)
	t.Cleanup(release) // runs before the fixture's Shutdown
	enabled := make(chan error, 1)
	go func() { enabled <- fx.s.SetEnabled(ctx, a.ID, true) }()
	fx.bridge.waitEntered(t, a.MissionID)

	short, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
	_, _, err := fx.s.Publish(short, a.ID, a.DraftRevision)
	cancel()
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Publish of the busy flow = %v, want it to wait for the lock and give up", err)
	}
	err = svcWithin(t, 10*time.Second, "the operations on another flow", func() error {
		if err := fx.s.SetEnabled(ctx, b.ID, true); err != nil {
			return err
		}
		if _, _, err := fx.s.Publish(ctx, b.ID, b.DraftRevision); err != nil {
			return err
		}
		return fx.s.DeleteFlow(ctx, b.ID)
	})
	if err != nil {
		t.Fatalf("another flow: %v", err)
	}

	// SetEnabled read revision 1. A revision 2 with another Date/Time node is saved now
	// (SaveDraft takes no lock) and published once SetEnabled is done, so the timers end
	// up those of revision 2; without the lock SetEnabled would arm revision 1's node last.
	rev, _, err := fx.s.SaveDraft(ctx, a.ID, svcDateTimeFlow("A", 11, "2026-10-05 09:00"), a.DraftRevision)
	if err != nil {
		t.Fatalf("SaveDraft while the flow is busy: %v", err)
	}
	published := make(chan error, 1)
	go func() {
		_, _, err := fx.s.Publish(ctx, a.ID, rev)
		published <- err
	}()
	release()
	for _, op := range []struct {
		name string
		ch   chan error
	}{{"SetEnabled", enabled}, {"Publish", published}} {
		select {
		case err := <-op.ch:
			if err != nil {
				t.Fatalf("%s: %v", op.name, err)
			}
		case <-time.After(10 * time.Second):
			t.Fatalf("%s did not return", op.name)
		}
	}
	calls := fx.bridge.callLog()
	enabledAt, syncAt := -1, -1
	for i, call := range calls {
		switch call {
		case "enabled:" + a.MissionID + ":true":
			enabledAt = i
		case "sync:" + a.MissionID:
			syncAt = i
		}
	}
	if enabledAt < 0 || syncAt < enabledAt {
		t.Fatalf("bridge calls = %v; the waiting Publish must sync after SetEnabled", calls)
	}
	if got := svcTimers(t, fx.s); len(got) != 1 || got[0] != testNodeID(11)+"@2026-10-05T09:00:00Z" {
		t.Fatalf("timers = %v, want those of flow A's revision 2", got)
	}
	if m, _ := fx.bridge.mission(a.MissionID); !m.enabled || strings.Join(svcBindingNodes(m), ",") != testNodeID(1)+","+testNodeID(11) {
		t.Fatalf("mission = %+v", m)
	}
	if n := svcLockEntries(fx.s); n != 0 {
		t.Fatalf("%d lock entries left", n)
	}
}

// Publish of a new revision and SetEnabled race, round after round. After each round
// the timers and the mission's bindings belong to the live revision and follow the
// switch. The bridge holds SetEnabled for a moment after its read of the flow, so
// without the flow lock a Publish often lands in between and SetEnabled then arms the
// previous revision's timers.
func TestServiceConcurrentPublishAndEnable(t *testing.T) {
	fx := newSvcFixture(t)
	ctx := context.Background()
	rec := fx.published(t, "Wechsel", svcDateTimeFlow("Wechsel", 100, "2026-11-01 08:00"))
	fx.bridge.mu.Lock()
	fx.bridge.enableDelay = 2 * time.Millisecond
	fx.bridge.mu.Unlock()
	rev := rec.DraftRevision
	for round := 1; round <= 12; round++ {
		doc := svcDateTimeFlow("Wechsel", 100+round, fmt.Sprintf("2026-11-%02d 08:00", round+1))
		next, _, err := fx.s.SaveDraft(ctx, rec.ID, doc, rev)
		if err != nil {
			t.Fatalf("round %d: SaveDraft: %v", round, err)
		}
		rev = next
		enable := round%4 != 0
		begin := make(chan struct{})
		errs := make(chan error, 2)
		go func() {
			<-begin
			errs <- fx.s.SetEnabled(ctx, rec.ID, enable)
		}()
		go func() {
			<-begin
			_, _, err := fx.s.Publish(ctx, rec.ID, next)
			errs <- err
		}()
		close(begin)
		for i := 0; i < 2; i++ {
			select {
			case err := <-errs:
				if err != nil {
					t.Fatalf("round %d: %v", round, err)
				}
			case <-time.After(30 * time.Second):
				t.Fatalf("round %d did not finish", round)
			}
		}
		svcCheckArmed(t, fx, rec.ID, enable)
	}
	if n := svcLockEntries(fx.s); n != 0 {
		t.Fatalf("%d lock entries left", n)
	}
}

// svcCheckArmed checks that the flow's mission switch is enabled, its bindings are the
// trigger nodes of the live revision, and its timers are the live revision's Date/Time
// triggers when enabled and none otherwise.
func svcCheckArmed(t *testing.T, fx *svcFixture, id string, enabled bool) {
	t.Helper()
	rec, err := fx.s.GetFlow(context.Background(), id)
	if err != nil || rec.Live == nil {
		t.Fatalf("GetFlow = %+v, %v", rec, err)
	}
	m, _ := fx.bridge.mission(rec.MissionID)
	if m.enabled != enabled {
		t.Fatalf("mission enabled = %v, want %v", m.enabled, enabled)
	}
	wantTimers, wantBindings := []string{}, []string{}
	for i := range rec.Live.Nodes {
		n := &rec.Live.Nodes[i]
		if def, ok := fx.s.Registry().Lookup(n.Type); ok && def.Trigger {
			wantBindings = append(wantBindings, n.ID)
		}
		if enabled && n.Type == TypeTriggerDateTime {
			b, err := bindDateTime(n, time.UTC, fx.clock.Now())
			if err != nil {
				t.Fatal(err)
			}
			wantTimers = append(wantTimers, n.ID+"@"+b.FireAt.UTC().Format(time.RFC3339))
		}
	}
	sort.Strings(wantBindings)
	if got := svcTimers(t, fx.s); strings.Join(got, ",") != strings.Join(wantTimers, ",") {
		t.Fatalf("live revision %d: timers = %v, want %v", rec.LiveRevision, got, wantTimers)
	}
	if got := svcBindingNodes(m); strings.Join(got, ",") != strings.Join(wantBindings, ",") {
		t.Fatalf("live revision %d: bindings = %v, want %v", rec.LiveRevision, got, wantBindings)
	}
}

// Deleting a flow cancels its runs: the queued run ends inside the delete, the running
// one when its node returns. That node returns only after the flow is gone, so its last
// writes find no rows; the runner logs that at Debug, and nothing is logged as a warning.
func TestServiceDeleteCancelsTheFlowsRuns(t *testing.T) {
	for _, via := range []string{"DeleteFlow", "DeleteFlowForMission"} {
		t.Run(via, func(t *testing.T) {
			logs := &svcLogs{}
			tools := newSvcBlockingTools()
			bridge := newSvcBridge()
			s := svcNewService(t, &Services{Tools: tools, Location: time.UTC}, bridge, slog.New(logs))
			t.Cleanup(tools.letGo) // runs before the Shutdown
			finished := svcObserveFinished(s)
			ctx := context.Background()
			b := newFlow("Suche")
			start := b.node("start", TypeTriggerManual, nil)
			search := b.node("search", TypeWebSearch, map[string]any{"query": "wetter"})
			b.edge(start, PortOut, search)
			rec, err := s.CreateFlow(ctx, CreateRequest{Name: "Suche"})
			if err != nil {
				t.Fatal(err)
			}
			rev, _, err := s.SaveDraft(ctx, rec.ID, b.build(), rec.DraftRevision)
			if err != nil {
				t.Fatal(err)
			}
			pub, _, err := s.Publish(ctx, rec.ID, rev)
			if err != nil {
				t.Fatal(err)
			}
			run := func() StartResult {
				t.Helper()
				res, err := s.Runner().Start(StartRequest{Flow: pub.Live, Revision: pub.LiveRevision, Mode: ModeLive,
					TriggerNode: start, TriggerType: TypeTriggerManual})
				if err != nil {
					t.Fatalf("Start: %v", err)
				}
				return res
			}
			running := run()
			select {
			case id := <-tools.called:
				if id != running.RunID {
					t.Fatalf("tool called by %s, want %s", id, running.RunID)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("the run did not reach its tool")
			}
			queued := run()
			if queued.Status != StartQueued {
				t.Fatalf("second run = %+v, want queued", queued)
			}

			if via == "DeleteFlow" {
				err = s.DeleteFlow(ctx, rec.ID)
			} else {
				err = s.DeleteFlowForMission(ctx, rec.MissionID)
			}
			if err != nil {
				t.Fatalf("%s: %v", via, err)
			}
			select {
			case got := <-finished:
				if got.ID != queued.RunID || got.Status != RunCancelled {
					t.Fatalf("finished during the delete = %+v, want the queued run cancelled", got)
				}
			default:
				t.Fatal("the queued run did not end inside the delete")
			}
			if _, err := s.GetFlow(ctx, rec.ID); !errors.Is(err, ErrNotFound) {
				t.Fatalf("GetFlow after the delete = %v", err)
			}
			if _, ok := bridge.mission(rec.MissionID); ok == (via == "DeleteFlow") {
				t.Fatalf("mission still there = %v after %s", ok, via)
			}

			tools.letGo()
			select {
			case got := <-finished:
				if got.ID != running.RunID || got.Status != RunCancelled {
					t.Fatalf("finished = %+v, want the running run cancelled", got)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("the running run did not end after the delete cancelled it")
			}
			if s.Runner().IsBusy(rec.ID) {
				t.Fatal("the deleted flow is still busy")
			}
			if warned := logs.messages(slog.LevelWarn, ""); len(warned) != 0 {
				t.Fatalf("logged: %v", warned)
			}
			if late := logs.messages(slog.LevelDebug, "not saved"); len(late) == 0 {
				t.Fatal("the cancelled run's last writes were expected after the delete")
			}
		})
	}
}

// When the store delete fails, the mission and the timers are already gone; a second
// DeleteFlow finishes the job.
func TestServiceDeleteRetriesAfterAStoreError(t *testing.T) {
	fx := newSvcFixture(t)
	ctx := context.Background()
	rec := fx.published(t, "Weg", svcDateTimeFlow("Weg", 10, "2026-10-04 09:00"))
	if err := fx.s.SetEnabled(ctx, rec.ID, true); err != nil {
		t.Fatal(err)
	}
	exec := func(stmt string) {
		t.Helper()
		if _, err := fx.s.Store().db.ExecContext(ctx, stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	exec(`CREATE TRIGGER svc_keep_flows BEFORE DELETE ON flows BEGIN SELECT RAISE(ABORT, 'injected delete failure'); END`)
	if err := fx.s.DeleteFlow(ctx, rec.ID); err == nil || !strings.Contains(err.Error(), "injected delete failure") {
		t.Fatalf("DeleteFlow with a failing store = %v", err)
	}
	if _, ok := fx.bridge.mission(rec.MissionID); ok {
		t.Fatal("the mission is deleted before the store")
	}
	if got := svcTimers(t, fx.s); len(got) != 0 {
		t.Fatalf("timers after the failed delete = %v", got)
	}
	if _, err := fx.s.GetFlow(ctx, rec.ID); err != nil {
		t.Fatalf("the flow must survive the failed delete: %v", err)
	}
	exec(`DROP TRIGGER svc_keep_flows`)
	if err := fx.s.DeleteFlow(ctx, rec.ID); err != nil {
		t.Fatalf("retried DeleteFlow: %v", err)
	}
	if _, err := fx.s.GetFlow(ctx, rec.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetFlow after the retry = %v", err)
	}
}

func TestServiceDeleteFlowForAmbiguousMission(t *testing.T) {
	fx := newSvcFixture(t)
	ctx := context.Background()
	a, err := fx.s.CreateFlow(ctx, CreateRequest{Name: "A"})
	if err != nil {
		t.Fatal(err)
	}
	b, err := fx.s.CreateFlow(ctx, CreateRequest{Name: "B"})
	if err != nil {
		t.Fatal(err)
	}
	if err := fx.s.Store().SetMissionID(ctx, b.ID, a.MissionID); err != nil {
		t.Fatal(err)
	}
	if err := fx.s.DeleteFlowForMission(ctx, a.MissionID); !errors.Is(err, ErrMissionAmbiguous) {
		t.Fatalf("DeleteFlowForMission = %v, want ErrMissionAmbiguous", err)
	}
	for _, id := range []string{a.ID, b.ID} {
		if _, err := fx.s.GetFlow(ctx, id); err != nil {
			t.Fatalf("flow %s after the refused delete: %v", id, err)
		}
	}
}
