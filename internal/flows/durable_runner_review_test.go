package flows

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type durableReviewKeys struct{ calls atomic.Int32 }

func (k *durableReviewKeys) CheckpointKey(context.Context, bool) ([]byte, error) {
	k.calls.Add(1)
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	return key, nil
}

func durableReviewPublished(t *testing.T, store *Store, id string, seconds float64) StartRequest {
	t.Helper()
	b := newFlow("Durable process recovery")
	start := b.node("start", TypeTriggerManual, nil)
	before := b.node("before", TypeSet, map[string]any{"fields": []any{map[string]any{"name": "value", "type": "text", "value": "committed before process exit"}}, "keep_input": false})
	wait := b.node("wait", durableWaitType, map[string]any{"seconds": seconds})
	after := b.node("after", TypeSet, map[string]any{"fields": []any{map[string]any{"name": "value", "type": "text", "value": "{{before.value}}"}}, "keep_input": false})
	b.edge(start, PortOut, before)
	b.edge(before, PortOut, wait)
	b.edge(wait, PortOut, after)
	doc := b.build()
	doc.ID = id
	doc.Settings.MaxRunSeconds = 120
	if _, err := store.CreateFlow(context.Background(), doc, "mission_process_review", time.Now()); err != nil {
		t.Fatal(err)
	}
	pub, err := store.Publish(context.Background(), doc.ID, 1, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	return StartRequest{Flow: pub.Live, Revision: pub.LiveRevision, Mode: ModeLive, TriggerNode: start, TriggerType: "manual", missionID: "mission_process_review"}
}

func durableReviewWait(t *testing.T, r *Runner, store *Store, runID string) {
	t.Helper()
	until := time.Now().Add(10 * time.Second)
	for time.Now().Before(until) {
		run, err := store.GetRunHeader(context.Background(), runID)
		if err != nil {
			t.Fatal(err)
		}
		r.mu.Lock()
		slots := r.slots
		_, reserved := r.durable[runID]
		r.mu.Unlock()
		if run.Status == RunWaiting && slots == 0 && reserved {
			return
		}
		if run.Status.Terminal() {
			t.Fatalf("run terminated before waiting: %s/%s", run.Status, run.ErrorCode)
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("run did not commit and release its worker")
}

func TestDurableRunnerReviewProcessExitPreservesStateAndResumes(t *testing.T) {
	const childEnv = "AURAGO_DURABLE_REVIEW_CHILD"
	if dir := os.Getenv(childEnv); dir != "" {
		store, err := OpenStore(filepath.Join(dir, "flows.db"), discardLogger())
		if err != nil {
			t.Fatal(err)
		}
		req := durableReviewPublished(t, store, "flow_process_review", 1)
		services := &Services{CheckpointKeys: &durableReviewKeys{}, Location: time.UTC}
		r := NewRunner(NewEngine(newTestRegistry(t), services, discardLogger(), 1), store, RunnerHooks{}, RunnerConfig{DurableEnabled: true, DurableAuthority: durableTestAuthority{}, DurableHistory: &durableTestProjection{}}, discardLogger())
		r.mu.Lock()
		r.durableStarted = true
		r.mu.Unlock()
		started, err := r.StartContext(context.Background(), req)
		if err != nil {
			t.Fatal(err)
		}
		durableReviewWait(t, r, store, started.RunID)
		if err := os.WriteFile(filepath.Join(dir, "run-id"), []byte(started.RunID), 0600); err != nil {
			t.Fatal(err)
		}
		// A process exit bypasses Runner shutdown, Store.Close and test cleanups.
		os.Exit(29)
	}

	dir := t.TempDir()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, executable, "-test.run=^TestDurableRunnerReviewProcessExitPreservesStateAndResumes$")
	cmd.Env = append(os.Environ(), childEnv+"="+dir)
	output, err := cmd.CombinedOutput()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 29 {
		t.Fatalf("child did not reach committed crash boundary: %v\n%s", err, output)
	}
	data, err := os.ReadFile(filepath.Join(dir, "run-id"))
	if err != nil {
		t.Fatal(err)
	}
	runID := strings.TrimSpace(string(data))
	store, err := OpenStore(filepath.Join(dir, "flows.db"), discardLogger())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	before, steps, err := store.GetRun(context.Background(), runID)
	if err != nil || !before.Durable || before.Status != RunWaiting || len(steps) != 3 {
		t.Fatalf("committed run did not survive process loss: record=%+v steps=%d err=%v", before, len(steps), err)
	}
	if steps[1].NodeKey != "before" || steps[1].Status != StepSuccess || steps[1].Output["value"] != "committed before process exit" || steps[2].Status != StepWaiting {
		t.Fatalf("unexpected recovered steps: %+v", steps)
	}
	var ciphertext []byte
	if err := store.db.QueryRow(`SELECT ciphertext FROM flow_checkpoints WHERE run_id=?`, runID).Scan(&ciphertext); err != nil {
		t.Fatal(err)
	}
	keys := &durableReviewKeys{}
	var ordinaryHooks atomic.Int32
	r := NewRunner(NewEngine(newTestRegistry(t), &Services{CheckpointKeys: keys, Location: time.UTC}, discardLogger(), 1), store, RunnerHooks{
		OnRunStarted:  func(RunRecord, *Flow) { ordinaryHooks.Add(1) },
		OnRunFinished: func(RunRecord, RunResult) { ordinaryHooks.Add(1) },
	}, RunnerConfig{}, discardLogger())
	t.Cleanup(func() { r.Shutdown(context.Background()) })
	r.mu.Lock()
	reservations, slots := r.live[before.FlowID], r.slots
	_, tracked := r.durable[runID]
	r.mu.Unlock()
	if reservations != 1 || slots != 0 || !tracked {
		t.Fatalf("disabled recovery lost its reservation: live=%d slots=%d tracked=%v", reservations, slots, tracked)
	}
	r.Shutdown(context.Background())
	after, afterSteps, err := store.GetRun(context.Background(), runID)
	if err != nil || after.Status != RunWaiting || !after.StartedAt.Equal(before.StartedAt) || len(afterSteps) != len(steps) {
		t.Fatalf("disabled shutdown changed saved work: record=%+v steps=%d err=%v", after, len(afterSteps), err)
	}
	var retained []byte
	if err := store.db.QueryRow(`SELECT ciphertext FROM flow_checkpoints WHERE run_id=?`, runID).Scan(&retained); err != nil {
		t.Fatal(err)
	}
	if string(retained) != string(ciphertext) || ordinaryHooks.Load() != 0 || keys.calls.Load() != 0 {
		t.Fatal("disabled restart rewrote/decrypted state or invoked ordinary mission hooks")
	}

	bridge := &durableStatusBridge{fakeBridge: newFakeBridge()}
	bridge.missions["mission_process_review"] = &fakeMission{flowID: before.FlowID, enabled: true}
	s := NewService(store, catalogRegistry(t, fullEnv()), &Services{CheckpointKeys: keys, Location: time.UTC}, bridge, ServiceConfig{DurableEnabled: true}, discardLogger())
	t.Cleanup(func() { _ = s.Shutdown(context.Background()) })
	if err := s.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	until := time.Now().Add(10 * time.Second)
	for time.Now().Before(until) {
		finished, completedSteps, err := store.GetRun(context.Background(), runID)
		if err != nil {
			t.Fatal(err)
		}
		if !finished.Status.Terminal() {
			time.Sleep(20 * time.Millisecond)
			continue
		}
		if finished.Status != RunSuccess || !finished.Durable || finished.FinishedAt == nil || !finished.StartedAt.Equal(before.StartedAt) {
			t.Fatalf("same-run recovery did not preserve identity: %+v", finished)
		}
		if len(completedSteps) < 4 || !reflect.DeepEqual(completedSteps[:2], steps[:2]) {
			t.Fatalf("recovery changed previously committed steps: %+v", completedSteps)
		}
		counts := map[string]int{}
		for _, step := range completedSteps {
			if step.Status == StepSuccess {
				counts[step.NodeKey]++
			}
			if step.NodeKey == "after" && step.Output["value"] != "committed before process exit" {
				t.Fatal("post-recovery step did not use the exact saved output")
			}
		}
		for _, key := range []string{"start", "before", "wait", "after"} {
			if counts[key] != 1 {
				t.Fatalf("step %s completed %d times", key, counts[key])
			}
		}
		var runs, checkpoints int
		if err := store.db.QueryRow(`SELECT COUNT(*) FROM flow_runs WHERE flow_id=?`, before.FlowID).Scan(&runs); err != nil {
			t.Fatal(err)
		}
		if err := store.db.QueryRow(`SELECT COUNT(*) FROM flow_checkpoints WHERE run_id=?`, runID).Scan(&checkpoints); err != nil {
			t.Fatal(err)
		}
		if runs != 1 || checkpoints != 0 {
			t.Fatalf("recovery created another run or retained terminal state: runs=%d checkpoints=%d", runs, checkpoints)
		}
		return
	}
	t.Fatal("committed run did not recover after the real process exit")
}

func TestDurableRunnerReviewWaitingKeepsEventStreamAndSkipsOrdinaryHooks(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	req := durableReviewPublished(t, store, "flow_stream_review", 60)
	var ordinaryHooks atomic.Int32
	r := NewRunner(NewEngine(newTestRegistry(t), &Services{CheckpointKeys: &durableReviewKeys{}, Location: time.UTC}, discardLogger(), 1), store, RunnerHooks{
		OnRunStarted:  func(RunRecord, *Flow) { ordinaryHooks.Add(1) },
		OnRunFinished: func(RunRecord, RunResult) { ordinaryHooks.Add(1) },
	}, RunnerConfig{DurableEnabled: true, DurableAuthority: durableTestAuthority{}, DurableHistory: &durableTestProjection{}}, discardLogger())
	t.Cleanup(func() { r.Shutdown(ctx) })
	r.mu.Lock()
	r.durableStarted = true
	r.mu.Unlock()
	started, err := r.StartContext(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	durableReviewWait(t, r, store, started.RunID)
	backlog, events, unsubscribe, ok := r.Bus().Subscribe(started.RunID, 0)
	defer unsubscribe()
	if !ok || len(backlog) == 0 {
		t.Fatal("saved waiting run lost its event history")
	}
	lastSeq, waiting := 0, 0
	for _, event := range backlog {
		if event.RunID != started.RunID || event.Seq <= lastSeq || event.Type == EventRunFinished {
			t.Fatalf("invalid waiting stream: %+v", event)
		}
		lastSeq = event.Seq
		if event.Step != nil && event.Step.Status == StepWaiting {
			waiting++
		}
	}
	if waiting != 1 || ordinaryHooks.Load() != 0 {
		t.Fatalf("waiting events=%d ordinary hooks=%d", waiting, ordinaryHooks.Load())
	}
	select {
	case _, open := <-events:
		t.Fatalf("waiting stream unexpectedly changed; open=%v", open)
	default:
	}
	replay, follow, stop, ok := r.Bus().Subscribe(started.RunID, lastSeq)
	defer stop()
	if !ok || len(replay) != 0 {
		t.Fatal("same-run cursor reconnect duplicated retained events")
	}
	select {
	case _, open := <-follow:
		t.Fatalf("reconnected waiting stream unexpectedly changed; open=%v", open)
	default:
	}
}

func TestDurableRunnerReviewFirstWaitCommitBoundaries(t *testing.T) {
	for _, beforeCommit := range []bool{true, false} {
		name := "committed_wait_event_lost"
		if beforeCommit {
			name = "waiting_transaction_rejected"
		}
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			store := openTestStore(t)
			req := durableReviewPublished(t, store, "flow_wait_boundary", 1)
			if beforeCommit {
				if _, err := store.db.Exec(`CREATE TRIGGER review_reject_wait BEFORE INSERT ON flow_checkpoints
					WHEN NEW.state='waiting' BEGIN SELECT RAISE(ABORT, 'injected wait write failure'); END`); err != nil {
					t.Fatal(err)
				}
			}
			var hooks, lostEvent atomic.Int32
			services := &Services{CheckpointKeys: &durableReviewKeys{}, Location: time.UTC}
			r := NewRunner(NewEngine(newTestRegistry(t), services, discardLogger(), 1), store, RunnerHooks{
				OnRunStarted:  func(RunRecord, *Flow) { hooks.Add(1) },
				OnRunFinished: func(RunRecord, RunResult) { hooks.Add(1) },
			}, RunnerConfig{DurableEnabled: true, DurableAuthority: durableTestAuthority{}, DurableHistory: &durableTestProjection{}}, discardLogger())
			t.Cleanup(func() { _ = r.Shutdown(ctx) })
			if !beforeCommit {
				r.testOnEvent = func(event RunEvent) {
					if event.Step != nil && event.Step.Status == StepWaiting {
						lostEvent.Add(1)
						panic("injected loss after wait commit, before event delivery")
					}
				}
			}
			r.mu.Lock()
			r.durableStarted = true
			r.mu.Unlock()
			started, err := r.StartContext(ctx, req)
			if err != nil {
				t.Fatal(err)
			}
			until := time.Now().Add(5 * time.Second)
			released := false
			for time.Now().Before(until) {
				r.mu.Lock()
				_, tracked := r.durable[started.RunID]
				released = tracked && r.slots == 0
				r.mu.Unlock()
				if released {
					break
				}
				time.Sleep(10 * time.Millisecond)
			}
			if !released {
				t.Fatal("failed owner did not release the physical slot")
			}
			run, steps, err := store.GetRun(ctx, started.RunID)
			if err != nil || run.Status.Terminal() || !run.Durable || hooks.Load() != 0 {
				t.Fatalf("wait fault fabricated completion: run=%+v hooks=%d err=%v", run, hooks.Load(), err)
			}
			var checkpoints int
			if err := store.db.QueryRow(`SELECT COUNT(*) FROM flow_checkpoints WHERE run_id=?`, started.RunID).Scan(&checkpoints); err != nil {
				t.Fatal(err)
			}
			if beforeCommit {
				if checkpoints != 0 || len(steps) != 2 || run.Status == RunWaiting {
					t.Fatalf("failed transaction left partial waiting state: checkpoints=%d steps=%+v run=%+v", checkpoints, steps, run)
				}
			} else if checkpoints != 1 || len(steps) != 3 || run.Status != RunWaiting || lostEvent.Load() != 1 {
				t.Fatalf("lost event changed committed state: checkpoints=%d steps=%+v run=%+v", checkpoints, steps, run)
			}
			backlog, stream, unsubscribe, known := r.Bus().Subscribe(started.RunID, 0)
			defer unsubscribe()
			if !known {
				t.Fatal("uncertain run stream disappeared")
			}
			for _, event := range backlog {
				if event.Type == EventRunFinished || event.Step != nil && event.Step.Status == StepWaiting {
					t.Fatalf("unconfirmed event was published: %+v", event)
				}
			}
			select {
			case _, open := <-stream:
				t.Fatalf("uncertain stream unexpectedly changed; open=%v", open)
			default:
			}
			if err := r.Shutdown(ctx); err != nil {
				t.Fatal(err)
			}
			if beforeCommit {
				if _, err := store.db.Exec(`DROP TRIGGER review_reject_wait`); err != nil {
					t.Fatal(err)
				}
			}
			bridge := &durableStatusBridge{fakeBridge: newFakeBridge()}
			bridge.missions[req.missionID] = &fakeMission{flowID: req.Flow.ID, enabled: true}
			s := NewService(store, catalogRegistry(t, fullEnv()), services, bridge, ServiceConfig{DurableEnabled: true}, discardLogger())
			t.Cleanup(func() { _ = s.Shutdown(ctx) })
			if err := s.Start(ctx); err != nil {
				t.Fatal(err)
			}
			until = time.Now().Add(10 * time.Second)
			for time.Now().Before(until) {
				finished, finalSteps, err := store.GetRun(ctx, started.RunID)
				if err != nil {
					t.Fatal(err)
				}
				if !finished.Status.Terminal() {
					time.Sleep(20 * time.Millisecond)
					continue
				}
				if beforeCommit {
					if finished.Status != RunCancelled || finished.ErrorCode != "FLOW_RESTARTED" || !reflect.DeepEqual(finalSteps, steps) {
						t.Fatalf("uncommitted run was replayed or not classified as interrupted: %+v %+v", finished, finalSteps)
					}
				} else {
					if finished.Status != RunSuccess || len(finalSteps) < 4 || !reflect.DeepEqual(finalSteps[:2], steps[:2]) {
						t.Fatalf("committed wait did not resume without changing accepted work: %+v %+v", finished, finalSteps)
					}
					counts := map[string]int{}
					for _, step := range finalSteps {
						if step.Status == StepSuccess {
							counts[step.NodeKey]++
						}
					}
					for _, key := range []string{"start", "before", "wait", "after"} {
						if counts[key] != 1 {
							t.Fatalf("repeated or missing successful step: %s=%d", key, counts[key])
						}
					}
				}
				var runs int
				if err := store.db.QueryRow(`SELECT COUNT(*) FROM flow_runs WHERE flow_id=?`, req.Flow.ID).Scan(&runs); err != nil || runs != 1 {
					t.Fatalf("fault recovery created another run: count=%d err=%v", runs, err)
				}
				return
			}
			t.Fatal("fault boundary did not reach its documented restart outcome")
		})
	}
}
