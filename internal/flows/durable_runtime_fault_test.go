package flows

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"
)

func TestDurableRuntimeFaultBlocksOnlyDurableAndPreservesCheckpoint(t *testing.T) {
	ctx := context.Background()
	f := newDurableApprovalFixture(t, openTestStore(t), "flow_runtime_fault", nil, 0)
	f.start(t)
	var before []byte
	var deadline string
	if err := f.store.db.QueryRow(`SELECT ciphertext,deadline FROM flow_checkpoints WHERE run_id=?`, f.started.RunID).Scan(&before, &deadline); err != nil {
		t.Fatal(err)
	}
	r := f.service.runner
	r.faultDurableRuntime("test")
	if !r.durableRuntimeFaulted() {
		t.Fatal("runtime fault was not latched")
	}
	if err := waitDurableRuntimeResources(t, r); err != nil {
		t.Fatal(err)
	}
	var after []byte
	var retainedDeadline string
	if err := f.store.db.QueryRow(`SELECT ciphertext,deadline FROM flow_checkpoints WHERE run_id=?`, f.started.RunID).Scan(&after, &retainedDeadline); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) || retainedDeadline != deadline {
		t.Fatal("runtime fault changed the committed checkpoint or its deadline")
	}
	status, err := f.service.DurableStatus(ctx)
	if err != nil || status.Reason != DurableRuntimeUnavailableCode || status.RecoveryAvailable {
		t.Fatalf("durable status = %+v, %v", status, err)
	}
	runStatus, err := f.service.DurableRunStatus(ctx, f.started.RunID)
	if err != nil || runStatus.State != "blocked" || runStatus.BlockedCode != DurableRuntimeUnavailableCode {
		t.Fatalf("durable run status = %+v, %v", runStatus, err)
	}
	page, err := f.service.ListDurableApprovals(ctx, "")
	if err != nil || len(page.Items) != 1 || page.Items[0].BlockedReason != DurableApprovalBlockRuntimeUnavailable || page.Items[0].CanDecide {
		t.Fatalf("approval page = %+v, %v", page, err)
	}
	if _, err := f.service.DecideDurableApproval(ctx, f.gate.GateID, f.gate.GateGeneration, f.gate.ContentDigest, DurableApprovalDecisionApprove, "local_admin"); !errors.Is(err, ErrDurableApprovalUnavailable) {
		t.Fatalf("faulted runtime accepted an approval: %v", err)
	}
	if _, err := r.StartContext(ctx, f.request); !errors.Is(err, ErrDurableUnavailable) {
		t.Fatalf("faulted runtime accepted a new durable run: %v", err)
	}

	ordinary := newFlow("Ordinary run after durable fault")
	trigger := ordinary.node("start", TypeTriggerManual, nil)
	set := ordinary.node("set", TypeSet, map[string]any{"fields": []any{map[string]any{"name": "ok", "type": "text", "value": "yes"}}, "keep_input": false})
	ordinary.edge(trigger, PortOut, set)
	doc := ordinary.build()
	doc.ID = "flow_ordinary_after_runtime_fault"
	if _, err := f.store.CreateFlow(ctx, doc, durableApprovalTestMission, f.clock.Now()); err != nil {
		t.Fatal(err)
	}
	started, err := r.StartContext(ctx, StartRequest{Flow: doc, Mode: ModeLive, TriggerNode: trigger, TriggerType: "manual"})
	if err != nil {
		t.Fatalf("ordinary run was blocked by durable fault: %v", err)
	}
	if result := waitRun(t, f.service, started.RunID); result.Status != RunSuccess {
		t.Fatalf("ordinary run failed after durable fault: %+v", result)
	}
	if stopped, err := f.service.CancelDurableRunAs(ctx, f.started.RunID, f.gate.GateGeneration, "local_admin"); err != nil || !stopped {
		t.Fatalf("generation-fenced Stop after fault: stopped=%v err=%v", stopped, err)
	}
	header, err := f.store.GetRunHeader(ctx, f.started.RunID)
	if err != nil || header.Status != RunCancelled {
		t.Fatalf("Stop did not cancel the retained run: %+v %v", header, err)
	}
}

func TestDurableRuntimeFaultReleasesQueuedResumeWithoutTerminalizingCheckpoint(t *testing.T) {
	f := newDurableApprovalFixture(t, openTestStore(t), "flow_runtime_fault_queue", nil, 0)
	f.start(t)
	var before []byte
	if err := f.store.db.QueryRow(`SELECT ciphertext FROM flow_checkpoints WHERE run_id=?`, f.started.RunID).Scan(&before); err != nil {
		t.Fatal(err)
	}
	r := f.service.runner
	if len(r.durableSlots) != 0 {
		t.Fatal("fixture still owns a durable capacity slot")
	}
	if r.durableSlots == nil {
		t.Fatal("durable capacity semaphore is missing")
	}
	r.durableSlots <- struct{}{}
	workerCtx, cancel := context.WithCancel(r.durableRuntimeCtx)
	token := r.durableActive.add(f.request.Flow.ID, f.request.missionID, ModeLive, cancel, true)
	p := &pendingRun{
		rec:           RunRecord{ID: f.started.RunID, FlowID: f.request.Flow.ID, Mode: ModeLive, Status: RunQueued, Durable: true},
		counted:       true,
		durableCtx:    workerCtx,
		durableCancel: cancel,
		durableToken:  token,
		durableResume: &durableResume{},
	}
	r.mu.Lock()
	r.waiting = append(r.waiting, p)
	r.mu.Unlock()
	r.faultDurableRuntime("test")
	if err := waitDurableRuntimeResources(t, r); err != nil {
		t.Fatal(err)
	}
	r.mu.Lock()
	_, reserved := r.durable[f.started.RunID]
	_, active := r.cancels[f.started.RunID]
	queued := false
	for _, pending := range r.waiting {
		queued = queued || pending.rec.ID == f.started.RunID
	}
	r.mu.Unlock()
	if !reserved || active || queued {
		t.Fatalf("fault left resumed ownership active: reserved=%v active=%v queued=%v", reserved, active, queued)
	}
	var after []byte
	var state, status string
	if err := f.store.db.QueryRow(`SELECT c.ciphertext,c.state,r.status FROM flow_checkpoints c JOIN flow_runs r ON r.id=c.run_id WHERE c.run_id=?`, f.started.RunID).Scan(&after, &state, &status); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) || state != "waiting" || status != string(RunWaiting) {
		t.Fatalf("fault terminalized or changed the checkpoint: state=%s status=%s", state, status)
	}
}

func TestDurableSchedulerPanicReleasesAdmissionResourcesAndPreservesCheckpoint(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	clock := newFakeClock(time.Now().UTC().Truncate(time.Microsecond))
	req := durableReviewPublished(t, store, "flow_scheduler_panic", 1)
	service, _, bridge := newDurableApprovalService(t, store, req.Flow.ID, clock, 0)
	bridge.missions[req.missionID] = &fakeMission{flowID: req.Flow.ID, enabled: true}
	service.runner.mu.Lock()
	service.runner.durableStarted = true
	service.runner.mu.Unlock()
	started, err := service.runner.StartContext(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	durableReviewWait(t, service.runner, store, started.RunID)
	clock.Advance(time.Second)
	var before []byte
	var deadline string
	if err := store.db.QueryRow(`SELECT ciphertext,deadline FROM flow_checkpoints WHERE run_id=?`, started.RunID).Scan(&before, &deadline); err != nil {
		t.Fatal(err)
	}
	service.runner.testAfterDurableSlot = func() { panic("private checkpoint material") }
	service.recoverDurableDue(ctx)
	if !service.runner.durableRuntimeFaulted() {
		t.Fatal("scheduler panic did not latch the runtime fault")
	}
	if err := waitDurableRuntimeResources(t, service.runner); err != nil {
		t.Fatal(err)
	}
	var after []byte
	var retainedDeadline, state, status string
	if err := store.db.QueryRow(`SELECT c.ciphertext,c.deadline,c.state,r.status FROM flow_checkpoints c JOIN flow_runs r ON r.id=c.run_id WHERE c.run_id=?`, started.RunID).Scan(&after, &retainedDeadline, &state, &status); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) || retainedDeadline != deadline || state != "waiting" || status != string(RunWaiting) {
		t.Fatalf("scheduler panic changed checkpoint state: state=%s status=%s", state, status)
	}
}

func TestDurableHistoryReconcilePanicStopsBothMaintenanceWorkers(t *testing.T) {
	ctx := context.Background()
	f := newDurableApprovalFixture(t, openTestStore(t), "flow_history_panic", nil, 0)
	f.start(t)
	if stopped, err := f.service.CancelDurableRunAs(ctx, f.started.RunID, f.gate.GateGeneration, "local_admin"); err != nil || !stopped {
		t.Fatalf("fixture stop failed: stopped=%v err=%v", stopped, err)
	}
	f.service.runner.cfg.DurableHistory = panicDurableRuntimeProjector{}
	f.service.startDurableScheduler()
	f.service.startDurableHistoryReconcile()
	if !waitDurableRuntimeFault(t, f.service.runner) {
		t.Fatal("history reconcile panic did not latch the runtime fault")
	}
	shutdownCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := f.service.Shutdown(shutdownCtx); err != nil {
		t.Fatalf("maintenance workers did not stop after the fault: %v", err)
	}
}

type panicDurableRuntimeProjector struct{}

func (panicDurableRuntimeProjector) ProjectDurableRun(context.Context, string) error {
	panic("private review data")
}

func TestDurableApprovalListPageBoundsKeyAndDiagnosticWait(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	f := newDurableApprovalFixture(t, openTestStore(t), "flow_approval_page_bounds", nil, 0)
	doc, err := f.request.Flow.Clone()
	if err != nil {
		t.Fatal(err)
	}
	doc.Settings.Concurrency = ConcurrencyParallel
	revision, err := f.store.SaveDraft(ctx, doc.ID, doc, f.request.Revision, f.clock.Now())
	if err != nil {
		t.Fatal(err)
	}
	published, err := f.store.Publish(ctx, doc.ID, revision, f.clock.Now())
	if err != nil {
		t.Fatal(err)
	}
	f.request.Flow, f.request.Revision = published.Live, published.LiveRevision
	f.start(t)
	second, err := f.service.runner.StartContext(ctx, f.request)
	if err != nil {
		t.Fatal(err)
	}
	durableReviewWait(t, f.service.runner, f.store, second.RunID)

	entered, release := make(chan struct{}, 1), make(chan struct{})
	f.keys.calls.Store(0)
	f.keys.entered, f.keys.block = entered, release
	f.bridge.authorizeCalls.Store(0)
	f.bridge.denyAuthorizationAt.Store(2)
	firstDone := make(chan struct {
		page DurableApprovalPage
		err  error
	}, 1)
	go func() {
		page, err := f.service.ListDurableApprovals(ctx, "")
		firstDone <- struct {
			page DurableApprovalPage
			err  error
		}{page, err}
	}()
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal("approval list did not reach its single page key probe")
	}
	waitingCtx, cancelWaiting := context.WithCancel(ctx)
	waitingDone := make(chan error, 1)
	go func() {
		_, err := f.service.ListDurableApprovals(waitingCtx, "")
		waitingDone <- err
	}()
	cancelWaiting()
	if err := <-waitingDone; !errors.Is(err, context.Canceled) {
		t.Fatalf("concurrent approval page did not honor cancellation while waiting: %v", err)
	}
	close(release)
	first := <-firstDone
	if first.err != nil {
		t.Fatal(first.err)
	}
	if len(first.page.Items) != 2 || first.page.Items[0].FlowID != first.page.Items[1].FlowID {
		t.Fatalf("page does not cover both same-flow approvals: %+v", first.page)
	}
	if f.keys.calls.Load() != 1 {
		t.Fatalf("page key probes = %d, want one", f.keys.calls.Load())
	}
	if got := f.bridge.authorizeCalls.Load(); got != 2 {
		t.Fatalf("per-item authority checks = %d, want two", got)
	}
	decidable, stale := 0, 0
	for _, item := range first.page.Items {
		if item.CanDecide {
			decidable++
		} else if item.BlockedReason == DurableApprovalBlockStale {
			stale++
		}
	}
	if decidable != 1 || stale != 1 {
		t.Fatalf("per-item revocation was not reflected: %+v", first.page.Items)
	}
}

func waitDurableRuntimeResources(t *testing.T, r *Runner) error {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		r.durableActive.mu.Lock()
		active := len(r.durableActive.active)
		r.durableActive.mu.Unlock()
		if active == 0 && len(r.durableSlots) == 0 {
			return nil
		}
		time.Sleep(time.Millisecond)
	}
	return errors.New("durable runtime resources did not drain")
}

func waitDurableRuntimeFault(t *testing.T, r *Runner) bool {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if r.durableRuntimeFaulted() {
			return true
		}
		time.Sleep(time.Millisecond)
	}
	return r.durableRuntimeFaulted()
}
