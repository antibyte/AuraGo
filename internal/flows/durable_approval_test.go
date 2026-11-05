package flows

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const durableApprovalTestMission = "mission_approval_test"

type durableApprovalCatalogEnv struct {
	StaticEnv
	authReady bool
}

func (e durableApprovalCatalogEnv) ApprovalAuthAvailable() bool { return e.authReady }

type durableApprovalTestKeys struct{ unavailable atomic.Bool }

func (k *durableApprovalTestKeys) CheckpointKey(ctx context.Context, _ bool) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if k.unavailable.Load() {
		return nil, errors.New("checkpoint key unavailable")
	}
	return bytes.Repeat([]byte{11}, 32), nil
}

type durableApprovalTestBridge struct {
	*durableStatusBridge
	approvalCalls atomic.Int32
	failAt        int32
	projections   atomic.Int32
}

func (b *durableApprovalTestBridge) DurableApprovalAllowed(ctx context.Context) error {
	if b.failAt > 0 && b.approvalCalls.Add(1) >= b.failAt {
		return errors.New("owner login unavailable")
	}
	return ctx.Err()
}

func (b *durableApprovalTestBridge) ProjectDurableRun(context.Context, string) error {
	b.projections.Add(1)
	return nil
}

type durableApprovalFixture struct {
	store   *Store
	service *Service
	clock   *fakeClock
	keys    *durableApprovalTestKeys
	bridge  *durableApprovalTestBridge
	request StartRequest
	started StartResult
	gate    DurableApprovalItem
}

func newDurableApprovalService(t *testing.T, store *Store, flowID string, clock *fakeClock, failAt int32) (*Service, *durableApprovalTestKeys, *durableApprovalTestBridge) {
	t.Helper()
	if clock == nil {
		clock = newFakeClock(time.Now().UTC().Truncate(time.Microsecond))
	}
	base := &durableStatusBridge{fakeBridge: newFakeBridge()}
	base.missions[durableApprovalTestMission] = &fakeMission{flowID: flowID, enabled: true}
	bridge := &durableApprovalTestBridge{durableStatusBridge: base, failAt: failAt}
	keys := &durableApprovalTestKeys{}
	services := &Services{CheckpointKeys: keys, Clock: clock, Location: time.UTC}
	service := NewService(store, catalogRegistry(t, durableApprovalCatalogEnv{StaticEnv: fullEnv(), authReady: true}), services, bridge,
		ServiceConfig{DurableEnabled: true}, discardLogger())
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = service.Shutdown(ctx)
	})
	return service, keys, bridge
}

func newDurableApprovalFixture(t *testing.T, store *Store, flowID string, clock *fakeClock, failAt int32) *durableApprovalFixture {
	t.Helper()
	if clock == nil {
		clock = newFakeClock(time.Now().UTC().Truncate(time.Microsecond))
	}
	request := publishDurableApprovalFlow(t, store, flowID, clock.Now())
	service, keys, bridge := newDurableApprovalService(t, store, request.Flow.ID, clock, failAt)
	service.runner.mu.Lock()
	service.runner.durableStarted = true
	service.runner.mu.Unlock()
	return &durableApprovalFixture{store: store, service: service, clock: clock, keys: keys, bridge: bridge, request: request}
}

func publishDurableApprovalFlow(t *testing.T, store *Store, id string, now time.Time) StartRequest {
	t.Helper()
	b := newFlow("Durable approval")
	start := b.node("start", TypeTriggerManual, nil)
	before := b.node("before", TypeSet, map[string]any{"fields": []any{map[string]any{"name": "marker", "type": "text", "value": "committed before review"}}, "keep_input": false})
	approval := b.node("approval", durableApprovalType, map[string]any{
		"summary":     "Review the saved request",
		"review_data": map[string]any{"amount": 42},
	})
	after := b.node("after", TypeSet, map[string]any{"fields": []any{map[string]any{"name": "continued", "type": "text", "value": "approved"}}, "keep_input": false})
	b.edge(start, PortOut, before)
	b.edge(before, PortOut, approval)
	b.edge(approval, PortOut, after)
	doc := b.build()
	doc.ID = id
	doc.Settings.MaxRunSeconds = 120
	if _, err := store.CreateFlow(context.Background(), doc, durableApprovalTestMission, now); err != nil {
		t.Fatal(err)
	}
	published, err := store.Publish(context.Background(), id, 1, now)
	if err != nil {
		t.Fatal(err)
	}
	return StartRequest{Flow: published.Live, Revision: published.LiveRevision, Mode: ModeLive, TriggerNode: start, TriggerType: "manual", missionID: durableApprovalTestMission}
}

func (f *durableApprovalFixture) start(t *testing.T) {
	t.Helper()
	started, err := f.service.runner.StartContext(context.Background(), f.request)
	if err != nil {
		t.Fatal(err)
	}
	f.started = started
	durableReviewWait(t, f.service.runner, f.store, started.RunID)
	item, err := f.service.getDurableApprovalByRun(context.Background(), started.RunID)
	if err != nil {
		t.Fatal(err)
	}
	f.gate = item
}

type durableApprovalCrashManifest struct {
	RunID         string `json:"run_id"`
	FlowID        string `json:"flow_id"`
	Revision      int    `json:"revision"`
	GateID        string `json:"gate_id"`
	Generation    int64  `json:"gate_generation"`
	ContentDigest string `json:"content_digest"`
}

func TestDurableApprovalProcessLossResumesSameRunOnce(t *testing.T) {
	const phaseEnv, dirEnv = "AURAGO_DURABLE_APPROVAL_CHILD_PHASE", "AURAGO_DURABLE_APPROVAL_CHILD_DIR"
	if phase := os.Getenv(phaseEnv); phase != "" {
		dir := os.Getenv(dirEnv)
		store, err := OpenStore(filepath.Join(dir, "flows.db"), discardLogger())
		if err != nil {
			t.Fatal(err)
		}
		// A 1 ns offset exercises wake-ceiling/deadline-floor rounding under a real wall clock.
		clock := newFakeClock(time.Now().UTC().Truncate(time.Microsecond).Add(time.Nanosecond))
		if phase == "pending" {
			fixture := newDurableApprovalFixture(t, store, "flow_approval_process", clock, 0)
			if err := fixture.service.Start(context.Background()); err != nil {
				t.Fatal(err)
			}
			started, err := fixture.service.runner.StartContext(context.Background(), fixture.request)
			if err != nil {
				t.Fatal(err)
			}
			durableReviewWait(t, fixture.service.runner, store, started.RunID)
			item, err := fixture.service.getDurableApprovalByRun(context.Background(), started.RunID)
			if err != nil {
				t.Fatal(err)
			}
			status, err := fixture.service.DurableRunStatus(context.Background(), started.RunID)
			if err != nil || status == nil || status.State != "approval_pending" || status.WakeAt != nil || status.Deadline == nil {
				t.Fatalf("pending approval run status exposes a timed wake: %+v err=%v", status, err)
			}
			manifest, err := json.Marshal(durableApprovalCrashManifest{started.RunID, item.FlowID, item.Revision, item.GateID, item.GateGeneration, item.ContentDigest})
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "approval.json"), manifest, 0600); err != nil {
				t.Fatal(err)
			}
			// Exit without cleanup to model loss after the review checkpoint commits.
			os.Exit(29)
		}
		if phase == "approve" {
			var before durableApprovalCrashManifest
			manifest, err := os.ReadFile(filepath.Join(dir, "approval.json"))
			if err != nil || json.Unmarshal(manifest, &before) != nil {
				t.Fatalf("read pending crash manifest: %v", err)
			}
			service, _, _ := newDurableApprovalService(t, store, before.FlowID, clock, 0)
			ctx := context.Background()
			detail, err := service.GetDurableApproval(ctx, before.GateID)
			if err != nil || detail.Review == nil || detail.GateID != before.GateID || detail.RunID != before.RunID || detail.FlowID != before.FlowID ||
				detail.Revision != before.Revision || detail.GateGeneration != before.Generation || detail.ContentDigest != before.ContentDigest || detail.Review.Summary != "Review the saved request" ||
				!bytes.Equal(detail.Review.ReviewData, []byte("{\"amount\":42}")) {
				t.Fatalf("restarted pending review changed: %+v err=%v", detail, err)
			}
			result, err := service.DecideDurableApproval(ctx, before.GateID, before.Generation, before.ContentDigest, DurableApprovalDecisionApprove, "local_admin")
			if err != nil || result.State != DurableApprovalApproved || result.Revision != before.Revision {
				t.Fatalf("commit approval before scheduler starts: %+v err=%v", result, err)
			}
			var generation int64
			var checkpointState, runStatus string
			if err := store.db.QueryRow("SELECT c.generation,c.state,r.status FROM flow_checkpoints c JOIN flow_runs r ON r.id=c.run_id WHERE c.run_id=?", before.RunID).Scan(&generation, &checkpointState, &runStatus); err != nil {
				t.Fatal(err)
			}
			if generation != before.Generation+1 || checkpointState != "waiting" || runStatus != string(RunWaiting) {
				t.Fatalf("approval child reached continuation before exit: generation=%d state=%q run=%q", generation, checkpointState, runStatus)
			}
			// No Service.Start call: the approved checkpoint is committed but cannot be claimed yet.
			os.Exit(31)
		}
		t.Fatalf("unknown process-loss phase %q", phase)
	}

	dir := t.TempDir()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	runChild := func(phase string, exitCode int) {
		t.Helper()
		cmd := exec.CommandContext(ctx, executable, "-test.run=^TestDurableApprovalProcessLossResumesSameRunOnce$")
		cmd.Env = append(os.Environ(), phaseEnv+"="+phase, dirEnv+"="+dir)
		output, err := cmd.CombinedOutput()
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != exitCode {
			t.Fatalf("%s process did not reach exit boundary %d: %v\n%s", phase, exitCode, err, output)
		}
	}
	runChild("pending", 29)
	var before durableApprovalCrashManifest
	manifest, err := os.ReadFile(filepath.Join(dir, "approval.json"))
	if err != nil || json.Unmarshal(manifest, &before) != nil {
		t.Fatalf("read crash manifest: %v", err)
	}
	store, err := OpenStore(filepath.Join(dir, "flows.db"), discardLogger())
	if err != nil {
		t.Fatal(err)
	}
	clock := newFakeClock(time.Now().UTC().Truncate(time.Microsecond))
	service, _, _ := newDurableApprovalService(t, store, before.FlowID, clock, 0)
	if err := service.Start(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := service.GetDurableApproval(ctx, before.GateID); err != nil {
		t.Fatal(err)
	}
	rec, err := store.GetFlow(ctx, before.FlowID)
	if err != nil {
		t.Fatal(err)
	}
	draft := rec.Draft
	draft.Name = "New revision while the old run waits"
	newRevision, err := store.SaveDraft(ctx, before.FlowID, draft, rec.DraftRevision, clock.Now())
	if err != nil || newRevision != 2 {
		t.Fatalf("save newer draft: revision=%d err=%v", newRevision, err)
	}
	if _, _, err := service.Publish(ctx, before.FlowID, newRevision); err != nil {
		t.Fatalf("publish new revision while v1 waits for approval: %v", err)
	}
	detail, err := service.GetDurableApproval(ctx, before.GateID)
	if err != nil || detail.Review == nil || detail.Revision != before.Revision || detail.ContentDigest != before.ContentDigest {
		t.Fatalf("publishing v2 changed the pinned review: %+v err=%v", detail, err)
	}
	stopCtx, stop := context.WithTimeout(context.Background(), 10*time.Second)
	if err := service.Shutdown(stopCtx); err != nil {
		stop()
		t.Fatal(err)
	}
	stop()
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	runChild("approve", 31)

	store, err = OpenStore(filepath.Join(dir, "flows.db"), discardLogger())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	clock = newFakeClock(time.Now().UTC().Truncate(time.Microsecond))
	service, _, _ = newDurableApprovalService(t, store, before.FlowID, clock, 0)
	if err := service.Start(ctx); err != nil {
		t.Fatal(err)
	}
	final, err := service.GetDurableApproval(ctx, before.GateID)
	if err != nil || final.State != DurableApprovalApproved || final.GateGeneration != before.Generation || final.ContentDigest != before.ContentDigest || final.Review != nil {
		t.Fatalf("approved metadata changed or retained its review after restart: %+v err=%v", final, err)
	}
	until := time.Now().Add(10 * time.Second)
	for time.Now().Before(until) {
		run, steps, err := store.GetRun(ctx, before.RunID)
		if err != nil {
			t.Fatal(err)
		}
		if !run.Status.Terminal() {
			time.Sleep(20 * time.Millisecond)
			continue
		}
		if run.Status != RunSuccess || run.Revision != before.Revision || !run.Durable {
			t.Fatalf("same pinned run did not finish successfully: %+v", run)
		}
		counts := map[string]int{}
		for _, step := range steps {
			if step.Status == StepSuccess {
				counts[step.NodeKey]++
			}
		}
		for _, key := range []string{"start", "before", "approval", "after"} {
			if counts[key] != 1 {
				t.Fatalf("step %q executed %d times: %+v", key, counts[key], steps)
			}
		}
		var runs, checkpoints int
		if err := store.db.QueryRow("SELECT COUNT(*) FROM flow_runs WHERE flow_id=?", before.FlowID).Scan(&runs); err != nil {
			t.Fatal(err)
		}
		if err := store.db.QueryRow("SELECT COUNT(*) FROM flow_checkpoints WHERE run_id=?", before.RunID).Scan(&checkpoints); err != nil || runs != 1 || checkpoints != 0 {
			t.Fatalf("continuation did not close the original run: runs=%d checkpoints=%d err=%v", runs, checkpoints, err)
		}
		final, err := service.GetDurableApproval(ctx, before.GateID)
		if err != nil || final.State != DurableApprovalApproved || final.GateGeneration != before.Generation || final.ContentDigest != before.ContentDigest || final.Review != nil {
			t.Fatalf("terminal approval metadata changed: %+v err=%v", final, err)
		}
		status, err := service.DurableRunStatus(ctx, before.RunID)
		if err != nil || status == nil || status.State != string(RunSuccess) || status.ApprovalGateID != before.GateID || status.CancelAvailable {
			t.Fatalf("terminal approval run status = %+v err=%v", status, err)
		}
		return
	}
	t.Fatal("the approved same-run continuation did not finish")
}

func TestDurableApprovalDecisionCASAndStopBoundaries(t *testing.T) {
	t.Run("duplicate and conflicting decisions; pending gate cannot be timed-claimed", func(t *testing.T) {
		f := newDurableApprovalFixture(t, openTestStore(t), "flow_approval_cas", nil, 0)
		f.start(t)
		ctx := context.Background()
		page, err := f.service.ListDurableApprovals(ctx, "")
		if err != nil || len(page.Items) != 1 || page.Items[0].GateID != f.gate.GateID {
			t.Fatalf("pending inbox = %+v err=%v", page, err)
		}
		if _, err := f.service.DecideDurableApproval(ctx, f.gate.GateID, f.gate.GateGeneration, strings.Repeat("0", 64), DurableApprovalDecisionApprove, "local_admin"); !errors.Is(err, ErrDurableApprovalConflict) {
			t.Fatalf("wrong digest = %v", err)
		}
		if _, err := f.service.DecideDurableApproval(ctx, f.gate.GateID, f.gate.GateGeneration+1, f.gate.ContentDigest, DurableApprovalDecisionApprove, "local_admin"); !errors.Is(err, ErrDurableApprovalConflict) {
			t.Fatalf("wrong generation = %v", err)
		}
		if _, err := f.service.DecideDurableApproval(ctx, f.gate.GateID, f.gate.GateGeneration, f.gate.ContentDigest, DurableApprovalDecisionApprove, "spoofed_actor"); !errors.Is(err, ErrDurableApprovalAuthUnavailable) {
			t.Fatalf("non-owner actor = %v", err)
		}
		var wake string
		if err := f.store.db.QueryRow(`SELECT wake_at FROM flow_checkpoints WHERE run_id=?`, f.started.RunID).Scan(&wake); err != nil {
			t.Fatal(err)
		}
		if _, err := f.store.db.Exec(`UPDATE flow_checkpoints SET wake_at=? WHERE run_id=?`, formatTime(f.clock.Now()), f.started.RunID); err != nil {
			t.Fatal(err)
		}
		claimed, err := claimDurableCheckpoint(ctx, f.store.db, f.started.RunID, f.gate.GateGeneration, "ordinary-claim", f.clock.Now(), f.clock.Now().Add(durableLease))
		if err != nil || claimed {
			t.Fatalf("pending approval was timed-claimed: claimed=%v err=%v", claimed, err)
		}
		if _, err := f.store.db.Exec(`UPDATE flow_checkpoints SET wake_at=? WHERE run_id=?`, wake, f.started.RunID); err != nil {
			t.Fatal(err)
		}
		f.clock.Advance(time.Nanosecond)
		first, err := f.service.DecideDurableApproval(ctx, f.gate.GateID, f.gate.GateGeneration, f.gate.ContentDigest, DurableApprovalDecisionApprove, "local_admin")
		if err != nil || first.Idempotent || first.State != DurableApprovalApproved {
			t.Fatalf("first approval = %+v %v", first, err)
		}
		var approvedWake string
		if err := f.store.db.QueryRow(`SELECT wake_at FROM flow_checkpoints WHERE run_id=?`, f.started.RunID).Scan(&approvedWake); err != nil {
			t.Fatal(err)
		}
		wantWake := formatTime(f.clock.Now().Truncate(time.Microsecond).Add(time.Microsecond))
		if approvedWake != wantWake {
			t.Fatalf("nanosecond approval wake=%q want %q", approvedWake, wantWake)
		}
		repeat, err := f.service.DecideDurableApproval(ctx, f.gate.GateID, f.gate.GateGeneration, f.gate.ContentDigest, DurableApprovalDecisionApprove, "local_admin")
		if err != nil || !repeat.Idempotent || repeat.State != DurableApprovalApproved {
			t.Fatalf("duplicate approval = %+v %v", repeat, err)
		}
		if _, err := f.service.DecideDurableApproval(ctx, f.gate.GateID, f.gate.GateGeneration, f.gate.ContentDigest, DurableApprovalDecisionReject, "local_admin"); !errors.Is(err, ErrDurableApprovalConflict) {
			t.Fatalf("conflicting decision = %v", err)
		}
		var state, decision, runStatus, checkpointState string
		var generation int64
		if err := f.store.db.QueryRow(`SELECT g.state,g.decision,r.status,c.state,c.generation FROM flow_durable_gates g JOIN flow_runs r ON r.id=g.run_id JOIN flow_checkpoints c ON c.run_id=r.id WHERE g.gate_id=?`, f.gate.GateID).Scan(&state, &decision, &runStatus, &checkpointState, &generation); err != nil {
			t.Fatal(err)
		}
		if state != DurableApprovalApproved || decision != DurableApprovalDecisionApprove || runStatus != string(RunWaiting) || checkpointState != "waiting" || generation != f.gate.GateGeneration+1 {
			t.Fatalf("approval CAS state=%q decision=%q run=%q checkpoint=%q generation=%d", state, decision, runStatus, checkpointState, generation)
		}
		page, err = f.service.ListDurableApprovals(ctx, "")
		if err != nil || len(page.Items) != 0 {
			t.Fatalf("terminal approval remained in inbox: %+v err=%v", page, err)
		}
	})

	t.Run("automatic expiry leaves no human decision and releases run ownership", func(t *testing.T) {
		f := newDurableApprovalFixture(t, openTestStore(t), "flow_approval_expiry", nil, 0)
		f.start(t)
		f.clock.Advance(f.gate.ExpiresAt.Sub(f.clock.Now()) + time.Microsecond)
		_, err := f.service.DecideDurableApproval(context.Background(), f.gate.GateID, f.gate.GateGeneration, f.gate.ContentDigest, DurableApprovalDecisionApprove, "local_admin")
		if !errors.Is(err, ErrDurableApprovalConflict) {
			t.Fatalf("expired approval = %v", err)
		}
		var state, decision, actor, status, code string
		var checkpoints int
		if err := f.store.db.QueryRow(`SELECT g.state,g.decision,g.actor,r.status,r.error_code,(SELECT COUNT(*) FROM flow_checkpoints WHERE run_id=r.id) FROM flow_durable_gates g JOIN flow_runs r ON r.id=g.run_id WHERE g.gate_id=?`, f.gate.GateID).Scan(&state, &decision, &actor, &status, &code, &checkpoints); err != nil {
			t.Fatal(err)
		}
		if state != DurableApprovalExpired || decision != "" || actor != "" || status != string(RunError) || code != "FLOW_APPROVAL_EXPIRED" || checkpoints != 0 || f.bridge.projections.Load() == 0 {
			t.Fatalf("expired gate=%q decision=%q actor=%q run=%q/%q checkpoints=%d projections=%d", state, decision, actor, status, code, checkpoints, f.bridge.projections.Load())
		}
		f.service.runner.mu.Lock()
		reserved, active := f.service.runner.live[f.request.Flow.ID], len(f.service.runner.durable)
		f.service.runner.mu.Unlock()
		if reserved != 0 || active != 0 {
			t.Fatalf("expiry retained flow ownership: reserved=%d durable=%d", reserved, active)
		}
	})

	t.Run("generation-fenced Stop is key-independent and clears only metadata", func(t *testing.T) {
		f := newDurableApprovalFixture(t, openTestStore(t), "flow_approval_stop", nil, 0)
		f.start(t)
		f.keys.unavailable.Store(true)
		stopped, err := f.service.CancelDurableRunAs(context.Background(), f.started.RunID, f.gate.GateGeneration, "local_admin")
		if err != nil || !stopped {
			t.Fatalf("Stop=%v err=%v", stopped, err)
		}
		var state, decision, actor, status, code string
		var checkpoints int
		if err := f.store.db.QueryRow(`SELECT g.state,g.decision,g.actor,r.status,r.error_code,(SELECT COUNT(*) FROM flow_checkpoints WHERE run_id=r.id) FROM flow_durable_gates g JOIN flow_runs r ON r.id=g.run_id WHERE g.gate_id=?`, f.gate.GateID).Scan(&state, &decision, &actor, &status, &code, &checkpoints); err != nil {
			t.Fatal(err)
		}
		if state != DurableApprovalCancelled || decision != "" || actor != "local_admin" || status != string(RunCancelled) || code != "FLOW_CANCELLED" || checkpoints != 0 || f.bridge.projections.Load() == 0 {
			t.Fatalf("stopped gate=%q decision=%q actor=%q run=%q/%q checkpoints=%d projections=%d", state, decision, actor, status, code, checkpoints, f.bridge.projections.Load())
		}
	})
}

func TestDurableApprovalAuthOwnerRollbackFailClosed(t *testing.T) {
	t.Run("catalog distinguishes durable-disabled and approval-auth-unavailable", func(t *testing.T) {
		for _, tc := range []struct {
			name      string
			enabled   bool
			authReady bool
			failAt    int32
			wantState string
			wantCode  string
		}{
			{name: "durable disabled", enabled: false, authReady: true, wantState: NeedsSetupState, wantCode: "FLOW_DURABLE_DISABLED"},
			{name: "owner login unavailable", enabled: true, authReady: false, wantState: NeedsSetupState, wantCode: "FLOW_APPROVAL_AUTH_UNAVAILABLE"},
			{name: "availability is a config projection", enabled: true, authReady: true, failAt: 1, wantState: AvailableState},
		} {
			t.Run(tc.name, func(t *testing.T) {
				store := openTestStore(t)
				base := &durableStatusBridge{fakeBridge: newFakeBridge()}
				bridge := &durableApprovalTestBridge{durableStatusBridge: base, failAt: tc.failAt}
				reg := catalogRegistry(t, durableApprovalCatalogEnv{StaticEnv: fullEnv(), authReady: tc.authReady})
				service := NewService(store, reg, &Services{CheckpointKeys: &durableApprovalTestKeys{}}, bridge,
					ServiceConfig{DurableEnabled: tc.enabled}, discardLogger())
				t.Cleanup(func() {
					ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
					defer cancel()
					_ = service.Shutdown(ctx)
				})
				def, ok := reg.Lookup(durableApprovalType)
				if !ok {
					t.Fatal("durable approval definition missing")
				}
				got := def.AvailabilityFunc()
				if got.State != tc.wantState || got.Code != tc.wantCode {
					t.Fatalf("availability = %+v, want state=%q code=%q", got, tc.wantState, tc.wantCode)
				}
				if bridge.approvalCalls.Load() != 0 {
					t.Fatalf("availability invoked runtime authority %d times", bridge.approvalCalls.Load())
				}
			})
		}
	})
	t.Run("rechecks login immediately before suspension", func(t *testing.T) {
		f := newDurableApprovalFixture(t, openTestStore(t), "flow_approval_auth_recheck", nil, 2)
		started, err := f.service.runner.StartContext(context.Background(), f.request)
		if err != nil {
			t.Fatal(err)
		}
		until := time.Now().Add(5 * time.Second)
		for time.Now().Before(until) {
			run, _, err := f.store.GetRun(context.Background(), started.RunID)
			if err != nil {
				t.Fatal(err)
			}
			if run.Status.Terminal() {
				if run.Status != RunError || run.ErrorCode != "FLOW_APPROVAL_AUTH_UNAVAILABLE" {
					t.Fatalf("revoked auth did not fail closed: %+v", run)
				}
				var gates, checkpoints int
				if err := f.store.db.QueryRow(`SELECT COUNT(*) FROM flow_durable_gates`).Scan(&gates); err != nil {
					t.Fatal(err)
				}
				if err := f.store.db.QueryRow(`SELECT COUNT(*) FROM flow_checkpoints WHERE run_id=?`, started.RunID).Scan(&checkpoints); err != nil || gates != 0 || checkpoints != 0 {
					t.Fatalf("auth revocation parked review state: gates=%d checkpoints=%d err=%v", gates, checkpoints, err)
				}
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
		t.Fatal("auth-revoked run did not terminate")
	})

	t.Run("owner mismatch and checkpoint rollback retain pending evidence", func(t *testing.T) {
		t.Run("owner mismatch", func(t *testing.T) {
			f := newDurableApprovalFixture(t, openTestStore(t), "flow_approval_owner", nil, 0)
			f.start(t)
			if _, err := f.store.db.Exec(`UPDATE flows SET mission_id='mission_replaced' WHERE id=?`, f.request.Flow.ID); err != nil {
				t.Fatal(err)
			}
			if _, err := f.service.DecideDurableApproval(context.Background(), f.gate.GateID, f.gate.GateGeneration, f.gate.ContentDigest, DurableApprovalDecisionApprove, "local_admin"); !errors.Is(err, ErrDurableApprovalUnavailable) {
				t.Fatalf("owner mismatch decision = %v", err)
			}
			var state, decision, checkpointState string
			var checkpoints int
			if err := f.store.db.QueryRow(`SELECT g.state,g.decision,c.state,(SELECT COUNT(*) FROM flow_checkpoints WHERE run_id=g.run_id) FROM flow_durable_gates g JOIN flow_checkpoints c ON c.run_id=g.run_id WHERE g.gate_id=?`, f.gate.GateID).Scan(&state, &decision, &checkpointState, &checkpoints); err != nil || state != DurableApprovalPending || decision != "" || checkpointState != "waiting" || checkpoints != 1 {
				t.Fatalf("owner mismatch changed evidence: state=%q decision=%q checkpoint=%q count=%d err=%v", state, decision, checkpointState, checkpoints, err)
			}
		})

		t.Run("checkpoint update rollback", func(t *testing.T) {
			f := newDurableApprovalFixture(t, openTestStore(t), "flow_approval_rollback", nil, 0)
			f.start(t)
			if _, err := f.store.db.Exec(`CREATE TRIGGER reject_approval_resume BEFORE UPDATE OF generation ON flow_checkpoints WHEN NEW.generation=OLD.generation+1 BEGIN SELECT RAISE(ABORT,'injected approval checkpoint failure'); END`); err != nil {
				t.Fatal(err)
			}
			if _, err := f.service.DecideDurableApproval(context.Background(), f.gate.GateID, f.gate.GateGeneration, f.gate.ContentDigest, DurableApprovalDecisionApprove, "local_admin"); err == nil {
				t.Fatal("checkpoint failure reported a committed decision")
			}
			var state, decision, actor, checkpointState, runStatus string
			var generation int64
			if err := f.store.db.QueryRow(`SELECT g.state,g.decision,g.actor,c.state,c.generation,r.status FROM flow_durable_gates g JOIN flow_checkpoints c ON c.run_id=g.run_id JOIN flow_runs r ON r.id=g.run_id WHERE g.gate_id=?`, f.gate.GateID).Scan(&state, &decision, &actor, &checkpointState, &generation, &runStatus); err != nil {
				t.Fatal(err)
			}
			if state != DurableApprovalPending || decision != "" || actor != "" || checkpointState != "waiting" || generation != f.gate.GateGeneration || runStatus != string(RunWaiting) {
				t.Fatalf("decision transaction partially committed: gate=%q/%q/%q checkpoint=%q/%d run=%q", state, decision, actor, checkpointState, generation, runStatus)
			}
		})
	})

	t.Run("unsafe integer review values including exponent notation are rejected", func(t *testing.T) {
		for _, tc := range []struct {
			json string
			want bool
		}{
			{`{"id":9007199254740991}`, true},
			{`{"id":-9007199254740991}`, true},
			{`{"id":9007199254740992}`, false},
			{`{"id":-9007199254740992}`, false},
			{`{"id":9.007199254740992e15}`, false},
		} {
			if got := jsSafeReviewNumbers([]byte(tc.json)); got != tc.want {
				t.Errorf("jsSafeReviewNumbers(%s)=%v want %v", tc.json, got, tc.want)
			}
		}
	})
}
