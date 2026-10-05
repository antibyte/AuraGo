package flows

import (
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"
)

// svcRunGateTools holds every tool call until open is called (or the run ends), then
// answers like a search that found nothing. It reports each call's run id on called.
type svcRunGateTools struct {
	called  chan string
	release chan struct{}
	once    sync.Once
}

func newSvcRunGateTools() *svcRunGateTools {
	return &svcRunGateTools{called: make(chan string, 8), release: make(chan struct{})}
}

func (g *svcRunGateTools) InvokeTool(ctx context.Context, req ToolRequest) (ToolResponse, error) {
	g.called <- req.RunID
	select {
	case <-g.release:
		return ToolResponse{Output: `Tool Output: {"status":"success","results":[]}`, Status: "success"}, nil
	case <-ctx.Done():
		return ToolResponse{}, ctx.Err()
	}
}

func (g *svcRunGateTools) open() { g.once.Do(func() { close(g.release) }) }

func (g *svcRunGateTools) waitCalled(t *testing.T, runID string) {
	t.Helper()
	select {
	case got := <-g.called:
		if got != runID {
			t.Fatalf("tool called by %s, want %s", got, runID)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("run %s did not reach its tool", runID)
	}
}

// svcRunLockProbe is svcRunBridge that checks, in every run report, that the Service's
// mu is free: the Service never calls the bridge while it holds mu. Use it only where
// nothing else takes mu at the same time.
type svcRunLockProbe struct {
	*svcRunBridge
	probeMu sync.Mutex
	s       *Service
	held    []string
}

func (b *svcRunLockProbe) check(call string) {
	b.probeMu.Lock()
	defer b.probeMu.Unlock()
	if b.s == nil {
		return
	}
	if !b.s.mu.TryLock() {
		b.held = append(b.held, call)
		return
	}
	b.s.mu.Unlock()
}

func (b *svcRunLockProbe) FlowRunStarted(missionID string, rec RunRecord) string {
	b.check("FlowRunStarted")
	return b.svcRunBridge.FlowRunStarted(missionID, rec)
}

func (b *svcRunLockProbe) FlowRunFinished(info RunFinishedInfo) {
	b.check("FlowRunFinished")
	b.svcRunBridge.FlowRunFinished(info)
}

func svcRunHistoryLen(s *Service) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.history)
}

// A run reports the final nodes and the notify setting of the revision it executed,
// even when the flow was republished with other final nodes while it ran.
func TestServiceRunFinishedUsesTheRunsOwnRevision(t *testing.T) {
	tools := newSvcRunGateTools()
	bridge := &svcRunLockProbe{svcRunBridge: newSvcRunBridge()}
	s := svcRunNewService(t, tools, bridge, nil, ServiceConfig{})
	t.Cleanup(tools.open) // runs before the Shutdown
	bridge.probeMu.Lock()
	bridge.s = s
	bridge.probeMu.Unlock()
	ctx := context.Background()
	pub := svcRunPublish(t, s, svcRunSearchFlow("Version", "first"))
	old, err := s.RunNow(ctx, pub.ID)
	if err != nil {
		t.Fatal(err)
	}
	tools.waitCalled(t, old.RunID)

	next := svcRunSearchFlow("Version", "second")
	next.Settings.NotifyOnError = "none"
	rev, _, err := s.SaveDraft(ctx, pub.ID, next, pub.DraftRevision)
	if err != nil {
		t.Fatal(err)
	}
	if again, _, err := s.Publish(ctx, pub.ID, rev); err != nil || again.LiveRevision != 2 {
		t.Fatalf("Publish of revision 2 = %+v, %v", again, err)
	}
	tools.open()
	info := bridge.waitInfo(t, old.RunID)
	first, _ := info.Outputs["first"].(map[string]any)
	if info.Result.Status != RunSuccess || info.Record.Revision != 1 || info.NotifyOnError != DefaultNotifyOnError ||
		len(info.Outputs) != 1 || first["v"] != "first" {
		t.Fatalf("the run of revision 1 reported %s, revision %d, notify %q, outputs %v",
			info.Result.Status, info.Record.Revision, info.NotifyOnError, info.Outputs)
	}

	// A run of revision 2 reports revision 2's final node and setting.
	cur, err := s.RunNow(ctx, pub.ID)
	if err != nil {
		t.Fatal(err)
	}
	info = bridge.waitInfo(t, cur.RunID)
	if second, _ := info.Outputs["second"].(map[string]any); info.Record.Revision != 2 || info.NotifyOnError != "none" ||
		len(info.Outputs) != 1 || second["v"] != "second" {
		t.Fatalf("the run of revision 2 reported revision %d, notify %q, outputs %v", info.Record.Revision, info.NotifyOnError, info.Outputs)
	}
	if info.HistoryID != "hist_"+cur.RunID || info.MissionID != pub.MissionID || info.FlowName != "Version" {
		t.Fatalf("history of the run of revision 2 = %q in %q (%q)", info.HistoryID, info.MissionID, info.FlowName)
	}
	bridge.probeMu.Lock()
	held := append([]string(nil), bridge.held...)
	bridge.probeMu.Unlock()
	if len(held) != 0 {
		t.Fatalf("the bridge was called while the Service held mu: %v", held)
	}
	if n := svcRunHistoryLen(s); n != 0 {
		t.Fatalf("%d history entries left after the runs finished", n)
	}
}

// svcRunDeleteFixture is a published flow (manual trigger, web search on blocking tools)
// with one run inside its tool call and one queued behind it, on an svcRunBridge, with
// every log record kept.
type svcRunDeleteFixture struct {
	s       *Service
	tools   *svcBlockingTools
	bridge  *svcRunBridge
	logs    *svcLogs
	pub     *FlowRecord
	running StartResult
	queued  StartResult
}

func newSvcRunDeleteFixture(t *testing.T) *svcRunDeleteFixture {
	t.Helper()
	fx := &svcRunDeleteFixture{tools: newSvcBlockingTools(), bridge: newSvcRunBridge(), logs: &svcLogs{}}
	fx.s = svcRunNewService(t, fx.tools, fx.bridge, slog.New(fx.logs), ServiceConfig{})
	t.Cleanup(fx.tools.letGo) // runs before the Shutdown
	fx.pub = svcRunPublish(t, fx.s, svcRunSearchFlow("Weg", "done"))
	var err error
	if fx.running, err = fx.s.RunNow(context.Background(), fx.pub.ID); err != nil {
		t.Fatal(err)
	}
	select {
	case id := <-fx.tools.called:
		if id != fx.running.RunID {
			t.Fatalf("tool called by %s, want %s", id, fx.running.RunID)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the run did not reach its tool")
	}
	if fx.queued, err = fx.s.RunNow(context.Background(), fx.pub.ID); err != nil || fx.queued.Status != StartQueued {
		t.Fatalf("second RunNow = %+v, %v", fx.queued, err)
	}
	return fx
}

// A flow deleted while its runs are active or queued: Mission Control still hears of
// both. The run that started completes its history entry with the mission it was
// recorded in, although the flow is gone; the queued run never started and has no entry.
// Nothing is logged as a warning.
func TestServiceRunFinishedReportsRunsOfADeletedFlow(t *testing.T) {
	fx := newSvcRunDeleteFixture(t)
	if err := fx.s.DeleteFlow(context.Background(), fx.pub.ID); err != nil {
		t.Fatalf("DeleteFlow: %v", err)
	}
	queued := fx.bridge.reports(fx.queued.RunID)
	if len(queued) != 1 {
		t.Fatalf("the queued run was reported %d times inside the delete", len(queued))
	}
	if q := queued[0]; q.HistoryID != "" || q.MissionID != fx.pub.MissionID || q.Result.Status != RunCancelled ||
		q.Outputs == nil || len(q.Outputs) != 0 || q.FlowName != "Weg" || q.NotifyOnError != DefaultNotifyOnError {
		t.Fatalf("queued run report = %+v", q)
	}

	fx.tools.letGo()
	info := fx.bridge.waitInfo(t, fx.running.RunID)
	if info.MissionID != fx.pub.MissionID || info.HistoryID != "hist_"+fx.running.RunID || info.Result.Status != RunCancelled ||
		info.Outputs == nil || len(info.Outputs) != 0 || info.FlowName != "" || info.NotifyOnError != "" {
		t.Fatalf("report of the run whose flow is gone = %+v", info)
	}
	if n := svcRunHistoryLen(fx.s); n != 0 {
		t.Fatalf("%d history entries left", n)
	}
	if warned := fx.logs.messages(slog.LevelWarn, ""); len(warned) != 0 {
		t.Fatalf("logged: %v", warned)
	}
	if gone := fx.logs.messages(slog.LevelDebug, "of a finished run could not be read"); len(gone) != 2 {
		t.Fatalf("debug records about the gone flow = %v, want the document and the flow", gone)
	}
}

// A run recorded during the delete, after the first cancel, is ended by the second one
// once the flow is gone. Nothing is known about it then, and it is still reported.
func TestServiceRunFinishedReportsARunThatNeverStartedOnAGoneFlow(t *testing.T) {
	fx := newSvcRunDeleteFixture(t)
	var mu sync.Mutex
	inject := true
	var late StartResult
	var lateErr error
	hook := fx.s.runner.hooks.OnRunFinished
	fx.s.runner.hooks.OnRunFinished = func(rec RunRecord, res RunResult) {
		if hook != nil {
			hook(rec, res)
		}
		mu.Lock()
		doInject := inject
		inject = false
		mu.Unlock()
		if doInject { // the queued run, ended by the first CancelFlow
			r, err := fx.s.RunNow(context.Background(), fx.pub.ID)
			mu.Lock()
			late, lateErr = r, err
			mu.Unlock()
		}
	}
	if err := fx.s.DeleteFlow(context.Background(), fx.pub.ID); err != nil {
		t.Fatalf("DeleteFlow: %v", err)
	}
	mu.Lock()
	lateRun, err := late, lateErr
	mu.Unlock()
	if err != nil || lateRun.RunID == "" {
		t.Fatalf("the late RunNow = %+v, %v; it must record its run before the store delete", lateRun, err)
	}
	reports := fx.bridge.reports(lateRun.RunID)
	if len(reports) != 1 {
		t.Fatalf("the late run was reported %d times when DeleteFlow returned", len(reports))
	}
	if r := reports[0]; r.MissionID != "" || r.HistoryID != "" || r.Result.Status != RunCancelled || r.Outputs == nil ||
		len(r.Outputs) != 0 || r.FlowName != "" {
		t.Fatalf("report of a never-started run on a gone flow = %+v", r)
	}
	if warned := fx.logs.messages(slog.LevelWarn, ""); len(warned) != 0 {
		t.Fatalf("logged: %v", warned)
	}
}

// The hooks log a flow that is gone at Debug and any other read error at Warn; a run is
// reported whatever the reads say, a test run never.
func TestServiceRunHooksLogLevels(t *testing.T) {
	logs := &svcLogs{}
	bridge := newSvcRunBridge()
	s := svcRunNewService(t, &fakeTools{}, bridge, slog.New(logs), ServiceConfig{})
	ctx := context.Background()
	at := time.Date(2026, 10, 3, 8, 0, 0, 0, time.UTC)
	gone := RunRecord{ID: "run_aaaaaaaaagne", FlowID: "flow_gone", Mode: ModeLive, Status: RunCancelled}

	s.onTimerFired("flow_gone", testNodeID(1), at)
	s.onRunStarted(gone)
	s.onRunFinished(gone, RunResult{Status: RunCancelled})
	if warned := logs.messages(slog.LevelWarn, ""); len(warned) != 0 {
		t.Fatalf("a gone flow logged %v", warned)
	}
	if debug := logs.messages(slog.LevelDebug, "could not"); len(debug) != 4 {
		t.Fatalf("debug records for a gone flow = %v", debug)
	}
	if got := bridge.reports(gone.ID); len(got) != 1 || got[0].MissionID != "" || got[0].Outputs == nil {
		t.Fatalf("reports of a run of a gone flow = %+v", got)
	}
	if n := len(bridge.startedRuns()); n != 0 {
		t.Fatalf("FlowRunStarted was called %d times for a gone flow", n)
	}

	pub := svcRunPublish(t, s, simpleFlow("Kaputt"))
	if _, err := s.Store().db.ExecContext(ctx, `UPDATE flows SET draft_json = 'x' WHERE id = ?`, pub.ID); err != nil {
		t.Fatal(err)
	}
	damaged := RunRecord{ID: "run_aaaaaaaaadmg", FlowID: pub.ID, Mode: ModeLive, Status: RunSuccess}
	s.onTimerFired(pub.ID, testNodeID(1), at)
	s.onRunStarted(damaged)
	s.onRunFinished(damaged, RunResult{Status: RunSuccess})
	for _, msg := range []string{"a date and time trigger could not read its flow", "the flow of a started run could not be read",
		"the flow of a finished run could not be read"} {
		if got := logs.messages(slog.LevelWarn, msg); len(got) != 1 {
			t.Fatalf("warnings %q = %v", msg, got)
		}
	}
	if got := bridge.reports(damaged.ID); len(got) != 1 {
		t.Fatalf("a run whose flow cannot be read was reported %d times", len(got))
	}

	test := RunRecord{ID: "run_aaaaaaaaatst", FlowID: pub.ID, Mode: ModeTest}
	s.onRunStarted(test)
	s.onRunFinished(test, RunResult{Status: RunSuccess})
	if got := bridge.reports(test.ID); len(got) != 0 {
		t.Fatalf("a test run was reported to Mission Control: %+v", got)
	}
}
