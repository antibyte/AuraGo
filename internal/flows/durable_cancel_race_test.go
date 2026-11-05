package flows

import (
	"bytes"
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type claimedBlockingKeys struct {
	calls   atomic.Int32
	reached chan struct{}
}

type switchableCheckpointKeys struct{ fail, wrong atomic.Bool }

func (k *switchableCheckpointKeys) CheckpointKey(context.Context, bool) ([]byte, error) {
	if k.fail.Load() {
		return nil, errors.New("injected key outage")
	}
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	if k.wrong.Load() {
		key[0] ^= 0xff
	}
	return key, nil
}

func (k *claimedBlockingKeys) CheckpointKey(ctx context.Context, create bool) ([]byte, error) {
	if !create && k.calls.Add(1) == 2 {
		close(k.reached)
		<-ctx.Done()
		return nil, ctx.Err()
	}
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	return key, nil
}

type durableQueryProjection struct{ store *Store }

func (p durableQueryProjection) ProjectDurableRun(ctx context.Context, id string) error {
	_, err := p.store.GetRunHeader(ctx, id)
	return err
}

func durableQueuedFixture(t *testing.T) (*Service, StartRequest, string) {
	t.Helper()
	store := openTestStore(t)
	req := durableReviewPublished(t, store, "flow_cancel_race", 1)
	bridge := &durableStatusBridge{fakeBridge: newFakeBridge()}
	bridge.missions[req.missionID] = &fakeMission{flowID: req.Flow.ID, enabled: true}
	clock := newFakeClock(time.Now().UTC())
	services := &Services{CheckpointKeys: &durableReviewKeys{}, Clock: clock, Location: time.UTC}
	s := NewService(store, catalogRegistry(t, fullEnv()), services, bridge, ServiceConfig{DurableEnabled: true, MaxParallelRuns: 1}, discardLogger())
	t.Cleanup(func() { _ = s.Shutdown(context.Background()) })
	s.runner.mu.Lock()
	s.runner.durableStarted = true
	s.runner.mu.Unlock()
	started, err := s.runner.StartContext(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	durableReviewWait(t, s.runner, store, started.RunID)
	until := time.Now().Add(3 * time.Second)
	for len(s.runner.durableSlots) != 0 && time.Now().Before(until) {
		time.Sleep(time.Millisecond)
	}
	clock.Advance(2 * time.Second)
	return s, req, started.RunID
}

func TestDurableCancelFencesStaleWakeSnapshot(t *testing.T) {
	s, req, runID := durableQueuedFixture(t)
	ctx := context.Background()
	r := s.runner
	r.startMu.Lock()
	done := make(chan struct{})
	go func() { defer close(done); s.recoverDurableDue(ctx) }()
	until := time.Now().Add(3 * time.Second)
	for len(r.durableSlots) == 0 && time.Now().Before(until) {
		time.Sleep(time.Millisecond)
	}
	if len(r.durableSlots) == 0 {
		r.startMu.Unlock()
		t.Fatal("wake did not reach admission fence")
	}
	accepted, err := s.CancelDurableRun(ctx, runID, 1)
	r.startMu.Unlock()
	if !accepted || err != nil {
		t.Fatalf("cancel %v %v", accepted, err)
	}
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("wake did not release admission")
	}
	if r.IsBusy(req.Flow.ID) || len(r.durableSlots) != 0 {
		t.Fatal("stale wake resurrected a reservation or token")
	}
	r.mu.Lock()
	queued := len(r.waiting)
	r.mu.Unlock()
	if queued != 0 {
		t.Fatalf("stale wake queued %d runs", queued)
	}
}

func TestDurableCancelQueuedWakeReleasesToken(t *testing.T) {
	s, req, runID := durableQueuedFixture(t)
	r := s.runner
	r.mu.Lock()
	r.slots = 1
	r.mu.Unlock()
	s.recoverDurableDue(context.Background())
	r.mu.Lock()
	queued := len(r.waiting)
	r.mu.Unlock()
	if queued != 1 {
		t.Fatalf("queued=%d", queued)
	}
	accepted, err := s.CancelDurableRun(context.Background(), runID, 1)
	r.mu.Lock()
	r.slots = 0
	r.mu.Unlock()
	if !accepted || err != nil {
		t.Fatalf("cancel %v %v", accepted, err)
	}
	if r.IsBusy(req.Flow.ID) || len(r.durableSlots) != 0 {
		t.Fatal("cancel leaked the queued wake's reservation or token")
	}
}

func TestDurableClaimFencesLiveAndPublishedVersion(t *testing.T) {
	s, req, runID := durableQueuedFixture(t)
	ctx := context.Background()
	stored, err := s.store.GetFlow(ctx, req.Flow.ID)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := stored.Live.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	altered, err := ParseFlow(doc)
	if err != nil {
		t.Fatal(err)
	}
	altered.Name = "altered after publication"
	bad, err := altered.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	guard := durableClaimGuard{flowID: req.Flow.ID, missionID: req.missionID, revision: req.Revision, document: doc}
	claim := func() (bool, error) {
		now := s.now()
		return claimDurableCheckpoint(ctx, s.store.db, runID, 1, "owner_claim", now, now.Add(time.Minute), guard)
	}
	if _, err := s.store.db.Exec(`UPDATE flows SET live_json=? WHERE id=?`, string(bad), req.Flow.ID); err != nil {
		t.Fatal(err)
	}
	if ok, err := claim(); ok || err == nil {
		t.Fatalf("changed live accepted %v %v", ok, err)
	}
	if _, err := s.store.db.Exec(`UPDATE flows SET live_json=? WHERE id=?`, string(doc), req.Flow.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.store.db.Exec(`UPDATE flow_versions SET json=? WHERE flow_id=? AND revision=?`, string(bad), req.Flow.ID, req.Revision); err != nil {
		t.Fatal(err)
	}
	if ok, err := claim(); ok || err == nil {
		t.Fatalf("changed version accepted %v %v", ok, err)
	}
	if _, err := s.store.db.Exec(`UPDATE flow_versions SET json=? WHERE flow_id=? AND revision=?`, string(doc), req.Flow.ID, req.Revision); err != nil {
		t.Fatal(err)
	}
	if _, err := s.store.db.Exec(`DELETE FROM flow_versions WHERE flow_id=? AND revision=?`, req.Flow.ID, req.Revision); err != nil {
		t.Fatal(err)
	}
	if ok, err := claim(); ok || err == nil {
		t.Fatalf("missing version accepted %v %v", ok, err)
	}
	if _, err := s.store.db.Exec(`INSERT INTO flow_versions(flow_id,revision,json,published_at) VALUES(?,?,?,?)`, req.Flow.ID, req.Revision, string(doc), formatTime(s.now())); err != nil {
		t.Fatal(err)
	}
	if ok, err := claim(); !ok || err != nil {
		t.Fatalf("unchanged snapshot refused %v %v", ok, err)
	}
	if _, err := s.store.db.Exec(`UPDATE flows SET live_json=? WHERE id=?`, string(bad), req.Flow.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.store.startClaimedDurableRun(ctx, runID, 1, "owner_claim", guard, s.now()); err == nil {
		t.Fatal("changed live accepted after claim")
	}
	header, err := s.store.GetRunHeader(ctx, runID)
	if err != nil || header.Status != RunQueued {
		t.Fatalf("failed start changed run: %+v %v", header, err)
	}
}

func TestDurableCancelReleasesConnectionBeforeHistoryProjection(t *testing.T) {
	s, _, runID := durableQueuedFixture(t)
	s.store.db.SetMaxOpenConns(1)
	s.runner.cfg.DurableHistory = durableQueryProjection{store: s.store}
	done := make(chan error, 1)
	go func() {
		ok, err := s.CancelDurableRun(context.Background(), runID, 1)
		if err == nil && !ok {
			err = ErrDurableUnavailable
		}
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("history projection could not acquire the only store connection")
	}
}

func TestDurableHeartbeatRenewsAndStopsStaleOwner(t *testing.T) {
	s, req, runID := durableQueuedFixture(t)
	ctx := context.Background()
	doc, err := req.Flow.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	guard := durableClaimGuard{flowID: req.Flow.ID, missionID: req.missionID, revision: req.Revision, document: doc}
	now := s.now()
	if ok, err := claimDurableCheckpoint(ctx, s.store.db, runID, 1, "owner_heartbeat", now, now.Add(durableLease), guard); !ok || err != nil {
		t.Fatalf("claim %v %v", ok, err)
	}
	if err := s.store.startClaimedDurableRun(ctx, runID, 1, "owner_heartbeat", guard, now); err != nil {
		t.Fatal(err)
	}
	control := durableControl(s.store, s.services.CheckpointKeys, runID, "owner_heartbeat", 1, s.now)
	p := &pendingRun{rec: RunRecord{ID: runID, FlowID: req.Flow.ID, Revision: req.Revision}, req: StartRequest{missionID: req.missionID}}
	worker, cancel := context.WithCancel(ctx)
	defer cancel()
	stop := s.runner.startDurableHeartbeat(worker, cancel, p, control)
	defer stop()
	clock := s.services.Clock.(*fakeClock)
	until := time.Now().Add(3 * time.Second)
	for clock.pending() == 0 && time.Now().Before(until) {
		time.Sleep(time.Millisecond)
	}
	clock.Advance(durableLease / 3)
	for time.Now().Before(until) {
		var lease string
		if err := s.store.db.QueryRow(`SELECT lease_until FROM flow_checkpoints WHERE run_id=?`, runID).Scan(&lease); err != nil {
			t.Fatal(err)
		}
		if parseTime(lease).After(now.Add(durableLease)) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	var lease string
	if err := s.store.db.QueryRow(`SELECT lease_until FROM flow_checkpoints WHERE run_id=?`, runID).Scan(&lease); err != nil || !parseTime(lease).After(now.Add(durableLease)) {
		t.Fatalf("lease was not renewed %s %v", lease, err)
	}
	if _, err := s.store.db.Exec(`UPDATE flow_checkpoints SET lease_owner='new_owner' WHERE run_id=?`, runID); err != nil {
		t.Fatal(err)
	}
	// Wait until the next fake-clock timer is installed before advancing it.
	for clock.pending() == 0 && time.Now().Before(until) {
		time.Sleep(time.Millisecond)
	}
	clock.Advance(durableLease / 3)
	select {
	case <-worker.Done():
	case <-time.After(3 * time.Second):
		t.Fatal("stale lease owner kept running")
	}
	if err := control.renew(ctx); err == nil {
		t.Fatal("stale owner renewed checkpoint")
	}
}

func TestDurableRevokedWorkerPreservesClaimedCheckpoint(t *testing.T) {
	s, req, runID := durableQueuedFixture(t)
	ctx := context.Background()
	doc, err := req.Flow.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	guard := durableClaimGuard{flowID: req.Flow.ID, missionID: req.missionID, revision: req.Revision, document: doc}
	now := s.now()
	if ok, err := claimDurableCheckpoint(ctx, s.store.db, runID, 1, "owner_revoked", now, now.Add(durableLease), guard); !ok || err != nil {
		t.Fatalf("claim %v %v", ok, err)
	}
	state, err := s.store.claimedDurableState(ctx, runID, 1, "owner_revoked", now, s.services.CheckpointKeys)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.store.startClaimedDurableRun(ctx, runID, 1, "owner_revoked", guard, now); err != nil {
		t.Fatal(err)
	}
	worker, cancel := context.WithCancel(ctx)
	cancel()
	control := durableControl(s.store, s.services.CheckpointKeys, runID, "owner_revoked", 1, s.now)
	control.ownerCtx = worker
	var emitted int
	res := resumeDurableCoordinator(worker, state, s.services, discardLogger(), control, nil, func(RunEvent) { emitted++ })
	if !res.durabilityLost || emitted != 0 {
		t.Fatalf("revoked worker produced result %+v events=%d", res, emitted)
	}
	var generation int
	var status string
	if err := s.store.db.QueryRow(`SELECT c.generation,r.status FROM flow_checkpoints c JOIN flow_runs r ON r.id=c.run_id WHERE c.run_id=?`, runID).Scan(&generation, &status); err != nil || generation != 1 || status != "running" {
		t.Fatalf("checkpoint changed: generation=%d status=%s err=%v", generation, status, err)
	}
}

func TestDurableCancelDuringClaimDoesNotResurrectRun(t *testing.T) {
	s, req, runID := durableQueuedFixture(t)
	ctx := context.Background()
	keys := &claimedBlockingKeys{reached: make(chan struct{})}
	s.services.CheckpointKeys = keys
	s.recoverDurableDue(ctx)
	select {
	case <-keys.reached:
	case <-time.After(3 * time.Second):
		t.Fatal("worker did not reach claimed checkpoint read")
	}
	if ok, err := s.CancelDurableRun(ctx, runID, 1); !ok || err != nil {
		t.Fatalf("cancel %v %v", ok, err)
	}
	until := time.Now().Add(3 * time.Second)
	for s.runner.IsBusy(req.Flow.ID) && time.Now().Before(until) {
		time.Sleep(10 * time.Millisecond)
	}
	header, err := s.store.GetRunHeader(ctx, runID)
	if err != nil || header.Status != RunCancelled || s.runner.IsBusy(req.Flow.ID) {
		t.Fatalf("run=%+v busy=%v err=%v", header, s.runner.IsBusy(req.Flow.ID), err)
	}
	var count int
	if err := s.store.db.QueryRow(`SELECT COUNT(*) FROM flow_checkpoints WHERE run_id=?`, runID).Scan(&count); err != nil || count != 0 {
		t.Fatalf("checkpoint=%d err=%v", count, err)
	}
}

func TestDurableShutdownDuringClaimPreservesCheckpoint(t *testing.T) {
	s, req, runID := durableQueuedFixture(t)
	ctx := context.Background()
	keys := &claimedBlockingKeys{reached: make(chan struct{})}
	s.services.CheckpointKeys = keys
	s.recoverDurableDue(ctx)
	select {
	case <-keys.reached:
	case <-time.After(3 * time.Second):
		t.Fatal("worker did not reach claimed checkpoint read")
	}
	shutdown, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := s.Shutdown(shutdown); err != nil {
		t.Fatal(err)
	}
	var state, status string
	if err := s.store.db.QueryRow(`SELECT c.state,r.status FROM flow_checkpoints c JOIN flow_runs r ON r.id=c.run_id WHERE c.run_id=?`, runID).Scan(&state, &status); err != nil || state != "claimed" || status != "queued" {
		t.Fatalf("state=%s status=%s err=%v", state, status, err)
	}
	if !s.runner.IsBusy(req.Flow.ID) {
		t.Fatal("shutdown dropped checkpoint reservation")
	}
}

func TestDurableDisableDuringClaimPreservesCheckpointAcrossReenable(t *testing.T) {
	s, req, runID := durableQueuedFixture(t)
	ctx := context.Background()
	keys := &claimedBlockingKeys{reached: make(chan struct{})}
	s.services.CheckpointKeys = keys
	s.recoverDurableDue(ctx)
	select {
	case <-keys.reached:
	case <-time.After(3 * time.Second):
		t.Fatal("worker did not reach claimed checkpoint read")
	}
	s.InvalidateDurableProcessing()
	until := time.Now().Add(3 * time.Second)
	for len(s.runner.durableSlots) != 0 && time.Now().Before(until) {
		time.Sleep(10 * time.Millisecond)
	}
	var state, status string
	if err := s.store.db.QueryRow(`SELECT c.state,r.status FROM flow_checkpoints c JOIN flow_runs r ON r.id=c.run_id WHERE c.run_id=?`, runID).Scan(&state, &status); err != nil || state != "claimed" || status != "queued" {
		t.Fatalf("state=%s status=%s err=%v", state, status, err)
	}
	if !s.runner.IsBusy(req.Flow.ID) {
		t.Fatal("disable lost the flow reservation")
	}
	// Re-enable only after the revoked owner has stopped; an expired lease is reclaimed.
	s.services.CheckpointKeys = &durableReviewKeys{}
	s.services.Clock.(*fakeClock).Advance(durableLease + time.Second)
	s.recoverDurableDue(ctx)
	until = time.Now().Add(4 * time.Second)
	for time.Now().Before(until) {
		header, err := s.store.GetRunHeader(ctx, runID)
		if err != nil {
			t.Fatal(err)
		}
		if header.Status.Terminal() {
			if header.Status != RunSuccess {
				t.Fatalf("resumed run %+v", header)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("run did not resume after re-enable and lease expiry")
}

func TestDurableDeleteDuringClaimStopsWorkerAndReleasesReservation(t *testing.T) {
	s, req, runID := durableQueuedFixture(t)
	ctx := context.Background()
	keys := &claimedBlockingKeys{reached: make(chan struct{})}
	s.services.CheckpointKeys = keys
	s.recoverDurableDue(ctx)
	select {
	case <-keys.reached:
	case <-time.After(3 * time.Second):
		t.Fatal("worker did not reach claimed checkpoint read")
	}
	if err := s.DeleteFlow(ctx, req.Flow.ID); err != nil {
		t.Fatal(err)
	}
	until := time.Now().Add(3 * time.Second)
	for s.runner.IsBusy(req.Flow.ID) && time.Now().Before(until) {
		time.Sleep(10 * time.Millisecond)
	}
	if s.runner.IsBusy(req.Flow.ID) {
		t.Fatal("deleted flow retained an active reservation")
	}
	if _, err := s.store.GetRunHeader(ctx, runID); !errors.Is(err, ErrRunNotFound) {
		t.Fatalf("deleted run remained: %v", err)
	}
}

func TestDurableStaleExecutingWorkerCannotPublishOrPersist(t *testing.T) {
	s, req, runID := durableQueuedFixture(t)
	ctx := context.Background()
	entered := make(chan struct{})
	cancelled := make(chan struct{})
	release := make(chan struct{})
	var releaseOnce sync.Once
	defer releaseOnce.Do(func() { close(release) })
	s.runner.testWrapDurableSet = func(original ExecuteFunc) ExecuteFunc {
		return func(workerCtx context.Context, in ExecInput) (ExecResult, error) {
			if in.Node.Key == "after" {
				close(entered)
				<-workerCtx.Done()
				close(cancelled)
				<-release
			}
			return original(context.Background(), in)
		}
	}
	s.recoverDurableDue(ctx)
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("real set worker did not start")
	}
	var generation int
	if err := s.store.db.QueryRow(`SELECT generation FROM flow_checkpoints WHERE run_id=?`, runID).Scan(&generation); err != nil {
		t.Fatal(err)
	}
	if _, err := s.store.db.Exec(`UPDATE flow_checkpoints SET lease_owner='replacement_owner' WHERE run_id=?`, runID); err != nil {
		t.Fatal(err)
	}
	clock := s.services.Clock.(*fakeClock)
	until := time.Now().Add(3 * time.Second)
	for clock.pending() == 0 && time.Now().Before(until) {
		time.Sleep(time.Millisecond)
	}
	clock.Advance(durableLease / 3)
	select {
	case <-cancelled:
	case <-time.After(3 * time.Second):
		t.Fatal("stale executing worker was not fenced")
	}
	releaseOnce.Do(func() { close(release) })
	until = time.Now().Add(3 * time.Second)
	for len(s.runner.durableSlots) != 0 && time.Now().Before(until) {
		time.Sleep(10 * time.Millisecond)
	}
	header, steps, err := s.store.GetRun(ctx, runID)
	if err != nil {
		t.Fatal(err)
	}
	if header.Status.Terminal() {
		t.Fatalf("stale worker terminalized run: %+v", header)
	}
	for _, step := range steps {
		if step.NodeKey == "after" {
			t.Fatalf("stale result persisted: %+v", step)
		}
	}
	var persisted int
	var owner string
	if err := s.store.db.QueryRow(`SELECT generation,lease_owner FROM flow_checkpoints WHERE run_id=?`, runID).Scan(&persisted, &owner); err != nil || persisted != generation || owner != "replacement_owner" {
		t.Fatalf("checkpoint gen=%d owner=%s err=%v", persisted, owner, err)
	}
	backlog, _, stop, ok := s.runner.Bus().Subscribe(runID, 0)
	if ok {
		defer stop()
		for _, event := range backlog {
			if event.Type == EventRunFinished || (event.Step != nil && event.Step.NodeKey == "after") {
				t.Fatalf("stale event published: %+v", event)
			}
		}
	}
	if !s.runner.IsBusy(req.Flow.ID) {
		t.Fatal("lost ownership dropped the flow reservation")
	}
}

func TestDurableExecutingWorkerRevocationAndStop(t *testing.T) {
	for _, action := range []string{"disable", "cancel", "delete", "shutdown"} {
		t.Run(action, func(t *testing.T) {
			s, req, runID := durableQueuedFixture(t)
			ctx := context.Background()
			entered, revoked, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
			var once sync.Once
			defer once.Do(func() { close(release) })
			s.runner.testWrapDurableSet = func(original ExecuteFunc) ExecuteFunc {
				return func(workerCtx context.Context, in ExecInput) (ExecResult, error) {
					if in.Node.Key == "after" {
						close(entered)
						<-workerCtx.Done()
						close(revoked)
						<-release
					}
					return original(context.Background(), in)
				}
			}
			s.recoverDurableDue(ctx)
			select {
			case <-entered:
			case <-time.After(3 * time.Second):
				t.Fatal("real worker did not start")
			}
			var generation int
			if err := s.store.db.QueryRow(`SELECT generation FROM flow_checkpoints WHERE run_id=?`, runID).Scan(&generation); err != nil {
				t.Fatal(err)
			}
			done := make(chan error, 1)
			go func() {
				switch action {
				case "disable":
					s.InvalidateDurableProcessing()
					done <- nil
				case "cancel":
					ok, err := s.CancelDurableRun(ctx, runID, int64(generation))
					if err == nil && !ok {
						err = ErrDurableUnavailable
					}
					done <- err
				case "delete":
					done <- s.DeleteFlow(ctx, req.Flow.ID)
				case "shutdown":
					bounded, cancel := context.WithTimeout(ctx, 3*time.Second)
					defer cancel()
					done <- s.Shutdown(bounded)
				}
			}()
			select {
			case <-revoked:
			case <-time.After(3 * time.Second):
				t.Fatal("worker was not revoked")
			}
			once.Do(func() { close(release) })
			select {
			case err := <-done:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("stop did not finish")
			}
			until := time.Now().Add(3 * time.Second)
			for len(s.runner.durableSlots) != 0 && time.Now().Before(until) {
				time.Sleep(10 * time.Millisecond)
			}
			if action == "delete" {
				if _, err := s.store.GetRunHeader(ctx, runID); !errors.Is(err, ErrRunNotFound) {
					t.Fatalf("deleted run: %v", err)
				}
				if s.runner.IsBusy(req.Flow.ID) {
					t.Fatal("deleted worker kept reservation")
				}
				return
			}
			header, steps, err := s.store.GetRun(ctx, runID)
			if err != nil {
				t.Fatal(err)
			}
			for _, step := range steps {
				if step.NodeKey == "after" {
					t.Fatalf("revoked result persisted: %+v", step)
				}
			}
			if action == "cancel" {
				if header.Status != RunCancelled {
					t.Fatalf("cancelled run %+v", header)
				}
			} else if header.Status.Terminal() {
				t.Fatalf("revocation fabricated terminal run %+v", header)
			}
			backlog, _, stop, ok := s.runner.Bus().Subscribe(runID, 0)
			if ok {
				defer stop()
				for _, event := range backlog {
					if event.Step != nil && event.Step.NodeKey == "after" {
						t.Fatalf("revoked result published: %+v", event)
					}
				}
			}
		})
	}
}

func TestDurableExecutingWorkerKeyLossPreservesCheckpoint(t *testing.T) {
	s, _, runID := durableQueuedFixture(t)
	ctx := context.Background()
	keys := &switchableCheckpointKeys{}
	s.services.CheckpointKeys = keys
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	defer once.Do(func() { close(release) })
	s.runner.testWrapDurableSet = func(original ExecuteFunc) ExecuteFunc {
		return func(workerCtx context.Context, in ExecInput) (ExecResult, error) {
			if in.Node.Key == "after" {
				close(entered)
				<-release
			}
			return original(workerCtx, in)
		}
	}
	s.recoverDurableDue(ctx)
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("real worker did not start")
	}
	keys.fail.Store(true)
	once.Do(func() { close(release) })
	until := time.Now().Add(3 * time.Second)
	for len(s.runner.durableSlots) != 0 && time.Now().Before(until) {
		time.Sleep(10 * time.Millisecond)
	}
	header, steps, err := s.store.GetRun(ctx, runID)
	if err != nil {
		t.Fatal(err)
	}
	if header.Status.Terminal() {
		t.Fatalf("key loss fabricated terminal result: %+v", header)
	}
	for _, step := range steps {
		if step.NodeKey == "after" {
			t.Fatalf("unsaved result persisted: %+v", step)
		}
	}
	var count int
	if err := s.store.db.QueryRow(`SELECT COUNT(*) FROM flow_checkpoints WHERE run_id=?`, runID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("checkpoint=%d err=%v", count, err)
	}
	keys.fail.Store(false)
	s.runner.testWrapDurableSet = nil
	s.services.Clock.(*fakeClock).Advance(durableLease + time.Second)
	s.recoverDurableDue(ctx)
	until = time.Now().Add(4 * time.Second)
	for time.Now().Before(until) {
		header, steps, err = s.store.GetRun(ctx, runID)
		if err != nil {
			t.Fatal(err)
		}
		if header.Status.Terminal() {
			if header.Status != RunSuccess {
				t.Fatalf("reclaimed run %+v", header)
			}
			var after int
			for _, step := range steps {
				if step.NodeKey == "after" {
					after++
				}
			}
			if after != 1 {
				t.Fatalf("step count=%d", after)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("run did not recover after key restoration")
}

func TestDurableExecutingWorkerChangedKeyPreservesAllTerminalPaths(t *testing.T) {
	for _, outcome := range []string{"success", "error", "stop"} {
		for _, change := range []string{"missing", "replaced"} {
			t.Run(outcome+"/"+change, func(t *testing.T) {
				s, _, runID := durableQueuedFixture(t)
				ctx := context.Background()
				keys := &switchableCheckpointKeys{}
				s.services.CheckpointKeys = keys
				entered, release := make(chan struct{}), make(chan struct{})
				var once sync.Once
				defer once.Do(func() { close(release) })
				s.runner.testWrapDurableSet = func(original ExecuteFunc) ExecuteFunc {
					return func(workerCtx context.Context, in ExecInput) (ExecResult, error) {
						if in.Node.Key != "after" {
							return original(workerCtx, in)
						}
						close(entered)
						<-release
						switch outcome {
						case "error":
							return ExecResult{}, NewNodeError("FLOW_PARAM_INVALID", "injected terminal error")
						case "stop":
							return ExecResult{Output: map[string]any{}, Stop: &StopSignal{Status: RunSuccess}}, nil
						default:
							return original(workerCtx, in)
						}
					}
				}
				s.recoverDurableDue(ctx)
				select {
				case <-entered:
				case <-time.After(3 * time.Second):
					t.Fatal("real worker did not start")
				}
				var before []byte
				var generation int
				if err := s.store.db.QueryRow(`SELECT ciphertext,generation FROM flow_checkpoints WHERE run_id=?`, runID).Scan(&before, &generation); err != nil {
					t.Fatal(err)
				}
				if change == "missing" {
					keys.fail.Store(true)
				} else {
					keys.wrong.Store(true)
				}
				once.Do(func() { close(release) })
				until := time.Now().Add(3 * time.Second)
				for len(s.runner.durableSlots) != 0 && time.Now().Before(until) {
					time.Sleep(10 * time.Millisecond)
				}
				header, steps, err := s.store.GetRun(ctx, runID)
				if err != nil {
					t.Fatal(err)
				}
				if header.Status.Terminal() {
					t.Fatalf("changed key caused terminal status %+v", header)
				}
				for _, step := range steps {
					if step.NodeKey == "after" {
						t.Fatalf("changed key persisted terminal step %+v", step)
					}
				}
				var after []byte
				var actual int
				if err := s.store.db.QueryRow(`SELECT ciphertext,generation FROM flow_checkpoints WHERE run_id=?`, runID).Scan(&after, &actual); err != nil || generation != actual || !bytes.Equal(before, after) {
					t.Fatalf("checkpoint changed gen=%d/%d err=%v", generation, actual, err)
				}
				backlog, _, stop, ok := s.runner.Bus().Subscribe(runID, 0)
				if ok {
					defer stop()
					for _, ev := range backlog {
						if ev.Type == EventRunFinished || (ev.Step != nil && ev.Step.NodeKey == "after") {
							t.Fatalf("changed key published result %+v", ev)
						}
					}
				}
				keys.fail.Store(false)
				keys.wrong.Store(false)
				s.runner.testWrapDurableSet = nil
				s.services.Clock.(*fakeClock).Advance(durableLease + time.Second)
				s.recoverDurableDue(ctx)
				until = time.Now().Add(4 * time.Second)
				for time.Now().Before(until) {
					header, _, err = s.store.GetRun(ctx, runID)
					if err != nil {
						t.Fatal(err)
					}
					if header.Status.Terminal() {
						if header.Status != RunSuccess {
							t.Fatalf("recovered run %+v", header)
						}
						return
					}
					time.Sleep(10 * time.Millisecond)
				}
				t.Fatal("original key did not recover the unchanged checkpoint")
			})
		}
	}
}

func TestDurableExecutingWorkerHonorsOriginalDeadline(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	req := durableReviewPublished(t, store, "flow_deadline_worker", 1)
	updated, err := req.Flow.Clone()
	if err != nil {
		t.Fatal(err)
	}
	updated.Settings.MaxRunSeconds = 3
	if _, err := store.SaveDraft(ctx, updated.ID, updated, 1, time.Now()); err != nil {
		t.Fatal(err)
	}
	pub, err := store.Publish(ctx, updated.ID, 2, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	req.Flow, req.Revision = pub.Live, pub.LiveRevision
	bridge := &durableStatusBridge{fakeBridge: newFakeBridge()}
	bridge.missions[req.missionID] = &fakeMission{flowID: req.Flow.ID, enabled: true}
	clock := newFakeClock(time.Now().UTC())
	services := &Services{CheckpointKeys: &durableReviewKeys{}, Clock: clock, Location: time.UTC}
	s := NewService(store, catalogRegistry(t, fullEnv()), services, bridge, ServiceConfig{DurableEnabled: true}, discardLogger())
	t.Cleanup(func() { _ = s.Shutdown(ctx) })
	s.runner.mu.Lock()
	s.runner.durableStarted = true
	s.runner.mu.Unlock()
	started, err := s.runner.StartContext(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	durableReviewWait(t, s.runner, store, started.RunID)
	clock.Advance(2 * time.Second)
	entered, expired, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var once sync.Once
	defer once.Do(func() { close(release) })
	s.runner.testWrapDurableSet = func(original ExecuteFunc) ExecuteFunc {
		return func(workerCtx context.Context, in ExecInput) (ExecResult, error) {
			if in.Node.Key == "after" {
				close(entered)
				<-workerCtx.Done()
				close(expired)
				<-release
			}
			return original(context.Background(), in)
		}
	}
	s.recoverDurableDue(ctx)
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("real worker did not start")
	}
	select {
	case <-expired:
	case <-time.After(5 * time.Second):
		t.Fatal("original deadline did not stop worker")
	}
	once.Do(func() { close(release) })
	// A late result cannot checkpoint with the expired execution context. The
	// next owner records timeout only after the old lease has safely expired.
	until := time.Now().Add(3 * time.Second)
	for len(s.runner.durableSlots) != 0 && time.Now().Before(until) {
		time.Sleep(10 * time.Millisecond)
	}
	clock.Advance(durableLease + time.Second)
	s.recoverDurableDue(ctx)
	until = time.Now().Add(4 * time.Second)
	for time.Now().Before(until) {
		header, steps, err := store.GetRun(ctx, started.RunID)
		if err != nil {
			t.Fatal(err)
		}
		if header.Status.Terminal() {
			if header.Status != RunError || header.ErrorCode != "FLOW_RUN_TIMEOUT" {
				t.Fatalf("deadline run %+v", header)
			}
			for _, step := range steps {
				if step.NodeKey == "after" && step.Status == StepSuccess {
					t.Fatalf("late success persisted %+v", step)
				}
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("deadline did not produce a fenced terminal result")
}
