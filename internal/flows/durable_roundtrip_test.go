package flows

import (
	"context"
	"errors"
	"testing"
	"time"
)

// This exercises the private coordinator/store boundary without registering a node
// or enabling durable execution in the production runner.
func TestDurablePrivateRoundTripSurvivesCrashAfterCheckpoint(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	clock := newFakeClock(now)
	store := openTestStore(t)
	b := newFlow("Private recovery")
	start := b.node("start", TypeTriggerManual, nil)
	before := b.node("before", TypeSet, map[string]any{"fields": []any{map[string]any{"name": "value", "value": "once"}}})
	wait := b.node("wait", durableWaitType, map[string]any{"seconds": 1.0})
	after := b.node("after", TypeSet, map[string]any{"fields": []any{map[string]any{"name": "value", "value": "{{before.value}}"}}})
	b.edge(start, PortOut, before)
	b.edge(before, PortOut, wait)
	b.edge(wait, PortOut, after)
	doc := b.build()
	doc.ID = "flow_private"
	if _, err := store.CreateFlow(ctx, doc, "mission_private", now); err != nil {
		t.Fatal(err)
	}
	run := RunRecord{ID: "run_private", FlowID: doc.ID, Revision: 1, Mode: ModeLive, Status: RunRunning, TriggerNode: start, TriggerType: "manual", StartedAt: now}
	if err := store.CreateRun(ctx, run, nil); err != nil {
		t.Fatal(err)
	}
	keys := &checkpointTestKeys{}
	engine, err := newDurableEngine(&Services{Clock: clock}, discardLogger())
	if err != nil {
		t.Fatal(err)
	}
	initial := durableControl(store, keys, run.ID, "", 0, clock.Now)
	initial.deadline = now.Add(5 * time.Minute)
	initialSink := func(ev RunEvent) {
		if ev.Type == EventStepFinished && ev.Step != nil && ev.Seq > initial.persistedSeq {
			if err := store.SaveStep(ctx, run.ID, ev.Seq, *ev.Step); err != nil {
				t.Errorf("save initial step: %v", err)
			}
		}
	}
	first := engine.Execute(ctx, RunRequest{RunID: run.ID, Flow: doc, Revision: 1, Mode: ModeLive, TriggerNode: start, TriggerType: "manual", durable: initial}, initialSink)
	if first.Status != RunWaiting || !first.suspended {
		t.Fatalf("initial wait: %+v", first)
	}
	if interrupted, err := store.MarkInterruptedRuns(ctx, now.Add(time.Second), now.Add(time.Second)); err != nil || interrupted != 0 {
		t.Fatalf("waiting run marked interrupted: %d %v", interrupted, err)
	}
	clock.Advance(time.Second)
	claimed, err := claimDurableCheckpoint(ctx, store.db, run.ID, 1, "owner_a", clock.Now(), clock.Now().Add(durableLease))
	if err != nil || !claimed {
		t.Fatalf("first claim: %v %v", claimed, err)
	}
	state, err := store.claimedDurableState(ctx, run.ID, 1, "owner_a", clock.Now(), keys)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SetRunStatus(ctx, run.ID, RunRunning); err != nil {
		t.Fatal(err)
	}
	control := durableControl(store, keys, run.ID, "owner_a", 1, clock.Now)
	checkpoint := control.checkpoint
	control.checkpoint = func(ctx context.Context, state durableCheckpointState, step StepRecord) error {
		if err := checkpoint(ctx, state, step); err != nil {
			return err
		}
		return errors.New("injected crash after commit")
	}
	crashed := resumeDurableCoordinator(ctx, state, &Services{Clock: clock}, discardLogger(), control, nil, nil)
	if !crashed.durabilityLost {
		t.Fatalf("injected crash was finalized: %+v", crashed)
	}
	if interrupted, err := store.MarkInterruptedRuns(ctx, now.Add(time.Minute), clock.Now()); err != nil || interrupted != 0 {
		t.Fatalf("checkpointed run marked interrupted: %d %v", interrupted, err)
	}
	clock.Advance(durableLease + time.Second)
	claimed, err = claimDurableCheckpoint(ctx, store.db, run.ID, 2, "owner_b", clock.Now(), clock.Now().Add(durableLease))
	if err != nil || !claimed {
		t.Fatalf("recovery claim: %v %v", claimed, err)
	}
	state, err = store.claimedDurableState(ctx, run.ID, 2, "owner_b", clock.Now(), keys)
	if err != nil {
		t.Fatal(err)
	}
	if state.WaitingNode != "" {
		t.Fatal("active checkpoint regressed to the completed wait")
	}
	if err := store.SetRunStatus(ctx, run.ID, RunRunning); err != nil {
		t.Fatal(err)
	}
	control = durableControl(store, keys, run.ID, "owner_b", 2, clock.Now)
	final := resumeDurableCoordinator(ctx, state, &Services{Clock: clock}, discardLogger(), control, nil, nil)
	if final.Status != RunSuccess || final.durabilityLost || final.Outputs["after"]["value"] != "once" {
		t.Fatalf("recovered result: %+v", final)
	}
	header, steps, err := store.GetRun(ctx, run.ID)
	if err != nil || header.Status != RunSuccess || len(steps) != 5 {
		t.Fatalf("stored result: %+v steps=%+v err=%v", header, steps, err)
	}
	counts := map[string]int{}
	for _, step := range steps {
		if step.Status == StepSuccess {
			counts[step.NodeID]++
		}
	}
	for _, id := range []string{start, before, wait, after} {
		if counts[id] != 1 {
			t.Fatalf("successful node %s appears %d times", id, counts[id])
		}
	}
}

func TestDurablePrivateWaitCommitUncertainDoesNotFinalize(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	clock := newFakeClock(now)
	store := openTestStore(t)
	b := newFlow("Uncertain")
	start := b.node("start", TypeTriggerManual, nil)
	wait := b.node("wait", durableWaitType, map[string]any{"seconds": 1.0})
	b.edge(start, PortOut, wait)
	doc := b.build()
	doc.ID = "flow_uncertain"
	if _, err := store.CreateFlow(ctx, doc, "mission_uncertain", now); err != nil {
		t.Fatal(err)
	}
	run := RunRecord{ID: "run_uncertain", FlowID: doc.ID, Revision: 1, Mode: ModeLive, Status: RunRunning, TriggerNode: start, TriggerType: "manual", StartedAt: now}
	if err := store.CreateRun(ctx, run, nil); err != nil {
		t.Fatal(err)
	}
	engine, err := newDurableEngine(&Services{Clock: clock}, discardLogger())
	if err != nil {
		t.Fatal(err)
	}
	control := durableControl(store, &checkpointTestKeys{}, run.ID, "", 0, clock.Now)
	control.deadline = now.Add(time.Minute)
	save := control.save
	control.save = func(ctx context.Context, state durableCheckpointState, step StepRecord) error {
		if err := save(ctx, state, step); err != nil {
			return err
		}
		return errors.New("injected unknown commit outcome")
	}
	var events []RunEvent
	result := engine.Execute(ctx, RunRequest{RunID: run.ID, Flow: doc, Revision: 1, Mode: ModeLive, TriggerNode: start, TriggerType: "manual", durable: control}, func(ev RunEvent) { events = append(events, ev) })
	if !result.durabilityLost || result.Status != RunError {
		t.Fatalf("uncertain checkpoint finalized: %+v", result)
	}
	for _, ev := range events {
		if ev.Type == EventRunFinished {
			t.Fatal("uncertain commit emitted terminal result")
		}
	}
	header, err := store.GetRunHeader(ctx, run.ID)
	if err != nil || header.Status != RunWaiting {
		t.Fatalf("committed wait lost: %+v %v", header, err)
	}
	reservations, err := store.durableReservations(ctx)
	if err != nil || len(reservations) != 1 {
		t.Fatalf("committed checkpoint missing: %+v %v", reservations, err)
	}
}
