package flows

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

// c10Bridge is svcBridge with a one-shot gate on FlowMissionEnabled: the next read for a
// held mission announces itself on reading and waits until the test releases it. Every
// read is logged as "read:<mission>" in svcBridge's call log, after the gate. All fields
// are guarded by mu, because the Service calls the bridge from several goroutines.
type c10Bridge struct {
	*svcBridge
	mu        sync.Mutex
	readGates map[string]chan struct{}
	reading   chan string
}

func newC10Bridge() *c10Bridge {
	return &c10Bridge{svcBridge: newSvcBridge(), readGates: map[string]chan struct{}{}, reading: make(chan string, 8)}
}

func (b *c10Bridge) FlowMissionEnabled(missionID string) bool {
	b.mu.Lock()
	gate := b.readGates[missionID]
	delete(b.readGates, missionID)
	b.mu.Unlock()
	if gate != nil {
		b.reading <- missionID
		<-gate
	}
	b.record("read:" + missionID)
	return b.svcBridge.FlowMissionEnabled(missionID)
}

// holdRead makes the next FlowMissionEnabled call for missionID wait until the returned
// release is called. Calling release more than once is harmless.
func (b *c10Bridge) holdRead(missionID string) (release func()) {
	gate := make(chan struct{})
	b.mu.Lock()
	b.readGates[missionID] = gate
	b.mu.Unlock()
	var once sync.Once
	return func() {
		once.Do(func() {
			b.mu.Lock()
			if b.readGates[missionID] == gate {
				delete(b.readGates, missionID)
			}
			b.mu.Unlock()
			close(gate)
		})
	}
}

func (b *c10Bridge) waitReading(t *testing.T, missionID string) {
	t.Helper()
	select {
	case got := <-b.reading:
		if got != missionID {
			t.Fatalf("FlowMissionEnabled held for %s, want %s", got, missionID)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("FlowMissionEnabled(%s) was not called", missionID)
	}
}

// newC10Fixture is newSvcFixture with a c10Bridge and the given logger (nil discards).
func newC10Fixture(t *testing.T, logger *slog.Logger) (*svcFixture, *c10Bridge) {
	t.Helper()
	clock := newFakeClock(time.Date(2026, 10, 3, 7, 0, 0, 0, time.UTC))
	bridge := newC10Bridge()
	s := svcNewService(t, &Services{Tools: &fakeTools{}, Clock: clock, Location: time.UTC}, bridge, logger)
	return &svcFixture{s: s, bridge: bridge.svcBridge, clock: clock}, bridge
}

// c10GateTools reports every tool call on called and holds it until the test lets go
// (the call then succeeds with an empty result list) or the run's context ends.
type c10GateTools struct {
	called  chan ToolRequest
	release chan struct{}
	once    sync.Once
}

func newC10GateTools() *c10GateTools {
	return &c10GateTools{called: make(chan ToolRequest, 8), release: make(chan struct{})}
}

func (g *c10GateTools) InvokeTool(ctx context.Context, req ToolRequest) (ToolResponse, error) {
	g.called <- req
	select {
	case <-g.release:
		return ToolResponse{Output: `{"status":"success","results":[]}`, Status: "success"}, nil
	case <-ctx.Done():
		return ToolResponse{}, ctx.Err()
	}
}

func (g *c10GateTools) letGo() { g.once.Do(func() { close(g.release) }) }

func (g *c10GateTools) waitCalled(t *testing.T) ToolRequest {
	t.Helper()
	select {
	case req := <-g.called:
		return req
	case <-time.After(5 * time.Second):
		t.Fatal("no run reached its tool")
	}
	return ToolRequest{}
}

// newC10Service builds a Service with the full catalog, the real clock, tools and cfg on
// a fresh store; it is shut down at the end of the test.
func newC10Service(t *testing.T, tools ToolInvoker, bridge MissionBridge, cfg ServiceConfig, logger *slog.Logger) *Service {
	t.Helper()
	if logger == nil {
		logger = discardLogger()
	}
	s := NewService(openTestStore(t), catalogRegistry(t, fullEnv()), &Services{Tools: tools, Location: time.UTC}, bridge, cfg, logger)
	t.Cleanup(func() { _ = s.Shutdown(context.Background()) })
	return s
}

// c10PublishSearch creates, saves and publishes "manual trigger -> web.search" and
// returns the record and the trigger node.
func c10PublishSearch(t *testing.T, s *Service, name string) (*FlowRecord, string) {
	t.Helper()
	ctx := context.Background()
	b := newFlow(name)
	start := b.node("start", TypeTriggerManual, nil)
	search := b.node("search", TypeWebSearch, map[string]any{"query": "wetter"})
	b.edge(start, PortOut, search)
	rec, err := s.CreateFlow(ctx, CreateRequest{Name: name})
	if err != nil {
		t.Fatalf("CreateFlow: %v", err)
	}
	rev, _, err := s.SaveDraft(ctx, rec.ID, b.build(), rec.DraftRevision)
	if err != nil {
		t.Fatalf("SaveDraft: %v", err)
	}
	pub, _, err := s.Publish(ctx, rec.ID, rev)
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}
	return pub, start
}

// c10ObserveFinished reports every run the runner finishes, after the Service's own hook,
// on a channel with room for n runs. Call it before the first run starts.
func c10ObserveFinished(s *Service, n int) <-chan RunRecord {
	ch := make(chan RunRecord, n)
	hook := s.runner.hooks.OnRunFinished
	s.runner.hooks.OnRunFinished = func(rec RunRecord, res RunResult) {
		if hook != nil {
			hook(rec, res)
		}
		ch <- rec
	}
	return ch
}

func c10WaitFinished(t *testing.T, ch <-chan RunRecord) RunRecord {
	t.Helper()
	select {
	case rec := <-ch:
		return rec
	case <-time.After(5 * time.Second):
		t.Fatal("no run finished")
	}
	return RunRecord{}
}

// c10WaitLockWaiters waits until n callers hold or wait for the lock of flow id. It only
// observes the lock's own bookkeeping; it does not sleep.
func c10WaitLockWaiters(t *testing.T, s *Service, id string, n int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		s.locks.mu.Lock()
		refs := 0
		if fl := s.locks.locks[id]; fl != nil {
			refs = fl.refs
		}
		s.locks.mu.Unlock()
		if refs >= n {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("lock of %s: %d holders and waiters, want %d", id, refs, n)
		}
		runtime.Gosched()
	}
}

// c10StoredLiveRuns counts the stored live runs of a flow with status, leaving out the
// run except.
func c10StoredLiveRuns(t *testing.T, s *Service, flowID string, status RunStatus, except string) int {
	t.Helper()
	var n int
	err := s.Store().db.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM flow_runs WHERE flow_id = ? AND mode = ? AND status = ? AND id <> ?`,
		flowID, ModeLive, status, except).Scan(&n)
	if err != nil {
		t.Fatalf("counting runs: %v", err)
	}
	return n
}

// MissionEnabledChanged holds the flow's lock while it re-arms the timers. While it waits
// in the bridge, a Publish of the same flow waits for it (and gives up with its context),
// and a Publish of the next revision runs only after it, so the timers end up those of
// the final live revision. Without the lock MissionEnabledChanged would arm the timers of
// the revision it read last, over those of the newer one.
func TestServiceMissionEnabledChangedHoldsTheFlowLock(t *testing.T) {
	fx, bridge := newC10Fixture(t, nil)
	ctx := context.Background()
	a := fx.published(t, "A", svcDateTimeFlow("A", 10, "2026-10-04 09:00"))
	// Mission Control switches the mission on directly, then tells the Service.
	if err := bridge.fakeBridge.SetFlowMissionEnabled(a.MissionID, true); err != nil {
		t.Fatal(err)
	}
	mark := len(bridge.callLog())
	release := bridge.holdRead(a.MissionID)
	t.Cleanup(release) // runs before the fixture's Shutdown
	changed := make(chan error, 1)
	go func() { changed <- fx.s.MissionEnabledChanged(ctx, a.MissionID) }()
	bridge.waitReading(t, a.MissionID)

	short, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
	_, _, err := fx.s.Publish(short, a.ID, a.DraftRevision)
	cancel()
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Publish during MissionEnabledChanged = %v, want it to wait for the flow lock and give up", err)
	}
	rev, _, err := fx.s.SaveDraft(ctx, a.ID, svcDateTimeFlow("A", 11, "2026-10-05 09:00"), a.DraftRevision)
	if err != nil {
		t.Fatalf("SaveDraft: %v", err)
	}
	published := make(chan error, 1)
	go func() {
		_, _, err := fx.s.Publish(ctx, a.ID, rev)
		published <- err
	}()
	c10WaitLockWaiters(t, fx.s, a.ID, 2) // the Publish waits for the lock now
	release()
	for _, op := range []struct {
		name string
		ch   chan error
	}{{"MissionEnabledChanged", changed}, {"Publish", published}} {
		select {
		case err := <-op.ch:
			if err != nil {
				t.Fatalf("%s: %v", op.name, err)
			}
		case <-time.After(10 * time.Second):
			t.Fatalf("%s did not return", op.name)
		}
	}
	calls := bridge.callLog()[mark:]
	readAt, syncAt := -1, -1
	for i, call := range calls {
		if call == "read:"+a.MissionID && readAt < 0 {
			readAt = i
		}
		if call == "sync:"+a.MissionID && syncAt < 0 {
			syncAt = i
		}
	}
	if readAt < 0 || syncAt < readAt {
		t.Fatalf("bridge calls = %v; the waiting Publish must sync after MissionEnabledChanged", calls)
	}
	if got := svcTimers(t, fx.s); len(got) != 1 || got[0] != testNodeID(11)+"@2026-10-05T09:00:00Z" {
		t.Fatalf("timers = %v, want those of revision 2", got)
	}
	svcCheckArmed(t, fx, a.ID, true)
	if n := svcLockEntries(fx.s); n != 0 {
		t.Fatalf("%d lock entries left", n)
	}
}

// MissionEnabledChanged waits for the flow lock with the caller's context, and reads the
// flow again once it holds the lock: a revision published meanwhile is the one it arms,
// and a flow deleted meanwhile is no error and arms nothing.
func TestServiceMissionEnabledChangedRereadsTheFlowInsideTheLock(t *testing.T) {
	logs := &svcLogs{}
	fx, bridge := newC10Fixture(t, slog.New(logs))
	ctx := context.Background()
	a := fx.published(t, "A", svcDateTimeFlow("A", 10, "2026-10-04 09:00"))
	if err := bridge.fakeBridge.SetFlowMissionEnabled(a.MissionID, true); err != nil {
		t.Fatal(err)
	}

	unlock, err := fx.s.locks.lock(ctx, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	short, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	err = fx.s.MissionEnabledChanged(short, a.MissionID)
	cancel()
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("MissionEnabledChanged while the flow is locked = %v, want it to wait and give up", err)
	}
	if got := svcTimers(t, fx.s); len(got) != 0 {
		t.Fatalf("timers after the given-up call = %v", got)
	}

	// Revision 2 goes live (straight in the store) while the call waits for the lock.
	done := make(chan error, 1)
	go func() { done <- fx.s.MissionEnabledChanged(ctx, a.MissionID) }()
	c10WaitLockWaiters(t, fx.s, a.ID, 2)
	rev, _, err := fx.s.SaveDraft(ctx, a.ID, svcDateTimeFlow("A", 11, "2026-10-05 09:00"), a.DraftRevision)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fx.s.Store().Publish(ctx, a.ID, rev, fx.clock.Now()); err != nil {
		t.Fatal(err)
	}
	unlock()
	if err := svcWithin(t, 10*time.Second, "MissionEnabledChanged", func() error { return <-done }); err != nil {
		t.Fatalf("MissionEnabledChanged: %v", err)
	}
	if got := svcTimers(t, fx.s); len(got) != 1 || got[0] != testNodeID(11)+"@2026-10-05T09:00:00Z" {
		t.Fatalf("timers = %v, want those of the revision published while the call waited", got)
	}

	// The flow is deleted while the call waits for the lock.
	if unlock, err = fx.s.locks.lock(ctx, a.ID); err != nil {
		t.Fatal(err)
	}
	go func() { done <- fx.s.MissionEnabledChanged(ctx, a.MissionID) }()
	c10WaitLockWaiters(t, fx.s, a.ID, 2)
	if err := fx.s.Store().DeleteFlow(ctx, a.ID); err != nil {
		t.Fatal(err)
	}
	unlock()
	if err := svcWithin(t, 10*time.Second, "MissionEnabledChanged", func() error { return <-done }); err != nil {
		t.Fatalf("MissionEnabledChanged for a flow deleted meanwhile = %v, want nil", err)
	}
	if got := svcTimers(t, fx.s); len(got) != 0 {
		t.Fatalf("timers after the delete = %v", got)
	}
	if warned := logs.messages(slog.LevelWarn, ""); len(warned) != 0 {
		t.Fatalf("logged: %v", warned)
	}
	if n := svcLockEntries(fx.s); n != 0 {
		t.Fatalf("%d lock entries left", n)
	}
}

// Unknown and ambiguous missions: MissionEnabledChanged ignores an unknown mission
// without logging, CancelMissionRuns fails with ErrNotFound, and both pass
// ErrMissionAmbiguous through; NextTimer reports no timer for either.
func TestServiceMissionHelpersUnknownAndAmbiguousMissions(t *testing.T) {
	logs := &svcLogs{}
	fx, _ := newC10Fixture(t, slog.New(logs))
	ctx := context.Background()
	if err := fx.s.MissionEnabledChanged(ctx, "mission_missing"); err != nil {
		t.Fatalf("MissionEnabledChanged(unknown) = %v", err)
	}
	if n, err := fx.s.CancelMissionRuns(ctx, "mission_missing"); !errors.Is(err, ErrNotFound) || n != 0 {
		t.Fatalf("CancelMissionRuns(unknown) = %d, %v; want ErrNotFound", n, err)
	}
	if _, ok := fx.s.NextTimer(ctx, "mission_missing"); ok {
		t.Fatal("NextTimer(unknown) reported a timer")
	}

	a := fx.published(t, "A", svcDateTimeFlow("A", 10, "2026-10-04 09:00"))
	b, err := fx.s.CreateFlow(ctx, CreateRequest{Name: "B"})
	if err != nil {
		t.Fatal(err)
	}
	if err := fx.s.Store().SetMissionID(ctx, b.ID, a.MissionID); err != nil {
		t.Fatal(err)
	}
	if err := fx.s.MissionEnabledChanged(ctx, a.MissionID); !errors.Is(err, ErrMissionAmbiguous) {
		t.Fatalf("MissionEnabledChanged(ambiguous) = %v", err)
	}
	if n, err := fx.s.CancelMissionRuns(ctx, a.MissionID); !errors.Is(err, ErrMissionAmbiguous) || n != 0 {
		t.Fatalf("CancelMissionRuns(ambiguous) = %d, %v", n, err)
	}
	if _, ok := fx.s.NextTimer(ctx, a.MissionID); ok {
		t.Fatal("NextTimer(ambiguous) reported a timer")
	}
	if warned := logs.messages(slog.LevelWarn, ""); len(warned) != 0 {
		t.Fatalf("logged: %v", warned)
	}
}

// CancelMissionRuns cancels the flow's live runs wherever they wait: one waiting for the
// global slot and one queued behind it. The flow's test run that holds the slot is left
// alone and finishes normally afterwards.
func TestServiceCancelMissionRunsCancelsWaitingLiveRunsOnly(t *testing.T) {
	tools := newC10GateTools()
	s := newC10Service(t, tools, newSvcBridge(), ServiceConfig{MaxParallelRuns: 1}, nil)
	t.Cleanup(tools.letGo) // runs before the Shutdown
	finished := c10ObserveFinished(s, 8)
	ctx := context.Background()
	rec, _ := c10PublishSearch(t, s, "Suche")

	testRun, err := s.StartTestRun(ctx, rec.ID, TestRunRequest{})
	if err != nil || testRun.Status != StartStarted {
		t.Fatalf("StartTestRun = %+v, %v", testRun, err)
	}
	if req := tools.waitCalled(t); req.RunID != testRun.RunID || req.Mode != ModeTest {
		t.Fatalf("tool called by %s (%s), want the test run %s", req.RunID, req.Mode, testRun.RunID)
	}
	waiting, err := s.RunNow(ctx, rec.ID) // waits for the global slot
	if err != nil || waiting.Status != StartQueued {
		t.Fatalf("first RunNow = %+v, %v", waiting, err)
	}
	queued, err := s.RunNow(ctx, rec.ID) // queued behind the flow's admitted live run
	if err != nil || queued.Status != StartQueued {
		t.Fatalf("second RunNow = %+v, %v", queued, err)
	}

	n, err := s.CancelMissionRuns(ctx, rec.MissionID)
	if err != nil || n != 2 {
		t.Fatalf("CancelMissionRuns = %d, %v; want the 2 live runs", n, err)
	}
	for _, id := range []string{waiting.RunID, queued.RunID} {
		detail, err := s.Run(ctx, id, false)
		if err != nil || detail.Run.Status != RunCancelled || detail.Run.ErrorCode != "FLOW_CANCELLED" {
			t.Fatalf("live run %s when CancelMissionRuns returned = %+v, %v", id, detail, err)
		}
	}
	if s.Runner().IsBusy(rec.ID) {
		t.Fatal("the flow still has live runs")
	}
	if detail, err := s.Run(ctx, testRun.RunID, false); err != nil || detail.Run.Status != RunRunning {
		t.Fatalf("test run after CancelMissionRuns = %+v, %v", detail, err)
	}
	for i := 0; i < 2; i++ { // the cancelled live runs, reported inside the call
		if got := c10WaitFinished(t, finished); got.Mode != ModeLive || got.Status != RunCancelled {
			t.Fatalf("finished = %+v, want a cancelled live run", got)
		}
	}

	tools.letGo()
	if got := c10WaitFinished(t, finished); got.ID != testRun.RunID || got.Status != RunSuccess {
		t.Fatalf("finished = %+v, want the test run to succeed", got)
	}
	if n, err := s.CancelMissionRuns(ctx, rec.MissionID); err != nil || n != 0 {
		t.Fatalf("CancelMissionRuns of an idle flow = %d, %v", n, err)
	}
}

// CancelMissionRuns does not page through the run list: with one running and 205 queued
// live runs (more than ListRuns returns per query) it cancels all 206, and the queued
// ones have ended when it returns.
func TestServiceCancelMissionRunsBeyondTheListPage(t *testing.T) {
	const queuedRuns = 205
	tools := newC10GateTools()
	bridge := newSvcBridge()
	s := newC10Service(t, tools, bridge, ServiceConfig{MaxQueuedPerFlow: queuedRuns}, nil)
	t.Cleanup(tools.letGo) // runs before the Shutdown
	finished := c10ObserveFinished(s, queuedRuns+8)
	ctx := context.Background()
	rec, start := c10PublishSearch(t, s, "Viele")
	startRun := func() StartResult {
		t.Helper()
		res, err := s.Runner().Start(StartRequest{Flow: rec.Live, Revision: rec.LiveRevision, Mode: ModeLive,
			TriggerNode: start, TriggerType: TypeTriggerManual})
		if err != nil {
			t.Fatalf("Start: %v", err)
		}
		return res
	}
	running := startRun()
	if req := tools.waitCalled(t); req.RunID != running.RunID {
		t.Fatalf("tool called by %s, want %s", req.RunID, running.RunID)
	}
	for i := 0; i < queuedRuns; i++ {
		if res := startRun(); res.Status != StartQueued {
			t.Fatalf("start %d = %+v, want queued", i+2, res)
		}
	}

	n, err := s.CancelMissionRuns(ctx, rec.MissionID)
	if err != nil || n != queuedRuns+1 {
		t.Fatalf("CancelMissionRuns = %d, %v; want %d", n, err, queuedRuns+1)
	}
	// The running run ends in the background, so it is left out of the counts.
	if got := c10StoredLiveRuns(t, s, rec.ID, RunCancelled, running.RunID); got != queuedRuns {
		t.Fatalf("%d live runs stored as cancelled when CancelMissionRuns returned, want the %d queued ones", got, queuedRuns)
	}
	if got := c10StoredLiveRuns(t, s, rec.ID, RunQueued, running.RunID); got != 0 {
		t.Fatalf("%d live runs still stored as queued", got)
	}
	ended := map[string]bool{}
	for len(ended) < queuedRuns+1 {
		got := c10WaitFinished(t, finished)
		if got.Status != RunCancelled {
			t.Fatalf("finished = %+v, want cancelled", got)
		}
		ended[got.ID] = true
	}
	if !ended[running.RunID] {
		t.Fatal("the running run did not end")
	}
	reported := 0
	for _, call := range bridge.callLog() {
		if strings.HasPrefix(call, "finished:") {
			reported++
		}
	}
	if reported != queuedRuns+1 {
		t.Fatalf("Mission Control was told about %d finished runs, want %d", reported, queuedRuns+1)
	}
	if n, err := s.CancelMissionRuns(ctx, rec.MissionID); err != nil || n != 0 {
		t.Fatalf("second CancelMissionRuns = %d, %v", n, err)
	}
	if s.Runner().IsBusy(rec.ID) {
		t.Fatal("the flow is still busy")
	}
}

// NextTimer returns the earliest timer of the flow behind each mission, with timers of
// many flows interleaved in time, skips stored rows whose time cannot be read, and does
// not wait for the flow's lock.
func TestServiceNextTimerPerFlow(t *testing.T) {
	fx := newSvcFixture(t)
	ctx := context.Background()
	base := time.Date(2026, 11, 1, 6, 0, 0, 0, time.UTC)
	type flowCase struct {
		rec  *FlowRecord
		want time.Time
	}
	var cases []flowCase
	for i := 0; i < 50; i++ {
		rec, err := fx.s.CreateFlow(ctx, CreateRequest{Name: fmt.Sprintf("Flow %d", i)})
		if err != nil {
			t.Fatal(err)
		}
		// Later flows have earlier timers, and the earliest node rotates, so neither the
		// global order nor the node order gives the answer.
		var timers []TimerRecord
		for k := 0; k < 3; k++ {
			at := base.Add(time.Duration(50-i)*time.Hour + time.Duration((k+i)%3)*time.Minute)
			timers = append(timers, TimerRecord{NodeID: testNodeID(10 + k), FireAt: at})
		}
		if err := fx.s.Store().ReplaceTimers(ctx, rec.ID, timers); err != nil {
			t.Fatal(err)
		}
		cases = append(cases, flowCase{rec: rec, want: base.Add(time.Duration(50-i) * time.Hour)})
	}
	for _, c := range cases {
		if got, ok := fx.s.NextTimer(ctx, c.rec.MissionID); !ok || !got.Equal(c.want) {
			t.Fatalf("NextTimer(%s) = %v %v, want %v", c.rec.Name, got, ok, c.want)
		}
	}

	empty, err := fx.s.CreateFlow(ctx, CreateRequest{Name: "Leer"})
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := fx.s.NextTimer(ctx, empty.MissionID); ok {
		t.Fatalf("NextTimer of a flow without timers = %v", got)
	}

	damaged, err := fx.s.CreateFlow(ctx, CreateRequest{Name: "Kaputt"})
	if err != nil {
		t.Fatal(err)
	}
	insert := func(node, fireAt string) {
		t.Helper()
		if _, err := fx.s.Store().db.ExecContext(ctx, `INSERT INTO flow_timers (flow_id, node_id, fire_at) VALUES (?, ?, ?)`,
			damaged.ID, node, fireAt); err != nil {
			t.Fatal(err)
		}
	}
	insert(testNodeID(20), "")
	insert(testNodeID(21), "not a time")
	if got, ok := fx.s.NextTimer(ctx, damaged.MissionID); ok {
		t.Fatalf("NextTimer with only unreadable rows = %v", got)
	}
	insert(testNodeID(22), formatTime(base.Add(90*time.Minute)))
	if got, ok := fx.s.NextTimer(ctx, damaged.MissionID); !ok || !got.Equal(base.Add(90*time.Minute)) {
		t.Fatalf("NextTimer next to unreadable rows = %v %v", got, ok)
	}

	unlock, err := fx.s.locks.lock(ctx, cases[0].rec.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	err = svcWithin(t, 5*time.Second, "NextTimer of a locked flow", func() error {
		if got, ok := fx.s.NextTimer(ctx, cases[0].rec.MissionID); !ok || !got.Equal(cases[0].want) {
			return fmt.Errorf("NextTimer = %v %v, want %v", got, ok, cases[0].want)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// The timer aggregate reads only the flow's rows through the primary key of flow_timers.
// This runs EXPLAIN QUERY PLAN on the real statement.
func TestStoreNextTimerAtUsesThePrimaryKey(t *testing.T) {
	s := openTestStore(t)
	rows, err := s.db.Query(`EXPLAIN QUERY PLAN `+nextTimerSQL, "flow_x")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var details []string
	for rows.Next() {
		var id, parent, notUsed int
		var detail string
		if err := rows.Scan(&id, &parent, &notUsed, &detail); err != nil {
			t.Fatal(err)
		}
		details = append(details, detail)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	plan := strings.Join(details, "; ")
	if strings.Contains(plan, "SCAN") || !strings.Contains(plan, "sqlite_autoindex_flow_timers_1 (flow_id=?)") {
		t.Fatalf("the timer aggregate does not search the primary key by flow: %s", plan)
	}
}

// c10DrainFinished collects the runs the runner fixture has finished so far, without waiting.
func c10DrainFinished(fx *runnerFixture) map[string]RunRecord {
	got := map[string]RunRecord{}
	for {
		select {
		case rec := <-fx.finished:
			got[rec.ID] = rec
		default:
			return got
		}
	}
}

// c10RunnerIdle fails unless the runner holds no slot, flow slot, queue, waiting or active run.
func c10RunnerIdle(t *testing.T, r *Runner) {
	t.Helper()
	r.mu.Lock()
	state := struct{ slots, live, queues, waiting, active int }{r.slots, len(r.live), len(r.flowQueue), len(r.waiting), len(r.cancels)}
	r.mu.Unlock()
	if state != (struct{ slots, live, queues, waiting, active int }{}) {
		t.Fatalf("runner state after all runs ended = %+v, want empty", state)
	}
}

// CancelFlowMode cancels only the flow's runs of one mode, wherever they are, and runs of
// other modes keep their places and run normally.
func TestRunnerCancelFlowModeKeepsOtherModes(t *testing.T) {
	// A running live run and a live run queued behind it are cancelled. The agent run
	// queued between them and the test run waiting for the global slot stay; the freed
	// slot goes to the test run (it waited first), then to the agent run.
	t.Run("running and queued", func(t *testing.T) {
		fx := newRunnerFixture(t, RunnerConfig{MaxParallelRuns: 1})
		a := fx.flow(t, "flow_aaaaaaaaka", ConcurrencyQueue)
		running := fx.start(t, a, ModeLive, "a1")
		fx.gate.waitStarted(t, "a1")
		agent := fx.start(t, a, ModeAgent, "g1")  // behind the flow's active run
		queued := fx.start(t, a, ModeLive, "a2")  // behind the agent run
		testRun := fx.start(t, a, ModeTest, "t1") // waits for the global slot
		for name, res := range map[string]StartResult{"agent": agent, "queued": queued, "test": testRun} {
			if res.Status != StartQueued {
				t.Fatalf("%s start = %+v, want queued", name, res)
			}
		}

		if n := fx.r.CancelFlowMode(a.ID, ModeLive); n != 2 {
			t.Fatalf("CancelFlowMode(live) = %d, want 2", n)
		}
		got := c10DrainFinished(fx)
		if rec, ok := got[queued.RunID]; !ok || rec.Status != RunCancelled || rec.ErrorCode != "FLOW_CANCELLED" {
			t.Fatalf("queued live run when CancelFlowMode returned = %+v (finished: %v)", rec, ok)
		}
		for _, id := range []string{agent.RunID, testRun.RunID} {
			if rec, ok := got[id]; ok {
				t.Fatalf("run %s of another mode ended: %+v", id, rec)
			}
		}
		rec, ok := got[running.RunID] // it ends in the background, maybe already
		if !ok {
			rec = fx.waitFinished(t)
		}
		if rec.ID != running.RunID || rec.Status != RunCancelled {
			t.Fatalf("finished = %+v, want the running live run cancelled", rec)
		}
		for _, next := range []struct {
			gate string
			id   string
		}{{"t1", testRun.RunID}, {"g1", agent.RunID}} {
			fx.gate.waitStarted(t, next.gate)
			fx.gate.release(next.gate)
			if rec := fx.waitFinished(t); rec.ID != next.id || rec.Status != RunSuccess {
				t.Fatalf("finished = %+v, want run %s to succeed", rec, next.id)
			}
		}
		c10RunnerIdle(t, fx.r)
		if n := fx.r.CancelFlowMode(a.ID, ModeLive); n != 0 {
			t.Fatalf("CancelFlowMode of an idle flow = %d", n)
		}
	})

	// A live run that waits for the global slot holds its flow's slot. Cancelling it
	// frees that slot for the agent run queued behind it, which then waits for the global
	// slot in its turn instead of being stuck in the flow's queue.
	t.Run("waiting run holding the flow slot", func(t *testing.T) {
		fx := newRunnerFixture(t, RunnerConfig{MaxParallelRuns: 1})
		a := fx.flow(t, "flow_aaaaaaaakb", ConcurrencyQueue)
		b := fx.flow(t, "flow_aaaaaaaakc", ConcurrencyParallel)
		other := fx.start(t, b, ModeLive, "b1")
		fx.gate.waitStarted(t, "b1")
		waiting := fx.start(t, a, ModeLive, "a1") // holds a's flow slot, waits for the global one
		agent := fx.start(t, a, ModeAgent, "g1")  // behind it in a's queue
		if waiting.Status != StartQueued || agent.Status != StartQueued {
			t.Fatalf("starts = %+v, %+v, want queued", waiting, agent)
		}

		if n := fx.r.CancelFlowMode(a.ID, ModeLive); n != 1 {
			t.Fatalf("CancelFlowMode(live) = %d, want 1", n)
		}
		got := c10DrainFinished(fx)
		if rec, ok := got[waiting.RunID]; !ok || rec.Status != RunCancelled || len(got) != 1 {
			t.Fatalf("finished when CancelFlowMode returned = %v, want only the waiting live run", got)
		}
		fx.r.mu.Lock()
		admitted := len(fx.r.flowQueue) == 0 && fx.r.live[a.ID] == 1 && len(fx.r.waiting) == 1 && fx.r.waiting[0].rec.ID == agent.RunID
		fx.r.mu.Unlock()
		if !admitted {
			t.Fatal("the agent run did not take the flow's slot freed by the cancel")
		}

		fx.gate.release("b1")
		if rec := fx.waitFinished(t); rec.ID != other.RunID || rec.Status != RunSuccess {
			t.Fatalf("finished = %+v, want the other flow's run", rec)
		}
		fx.gate.waitStarted(t, "g1")
		fx.gate.release("g1")
		if rec := fx.waitFinished(t); rec.ID != agent.RunID || rec.Status != RunSuccess {
			t.Fatalf("finished = %+v, want the agent run to succeed", rec)
		}
		c10RunnerIdle(t, fx.r)
	})
}
