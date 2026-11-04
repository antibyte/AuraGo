package flows

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestDurableCoordinatorSuspendsOnlyPureQuiescentGraph(t *testing.T) {
	b := newFlow("Durable")
	start := b.node("start", TypeTriggerManual, nil)
	set := b.node("greet", TypeSet, map[string]any{"fields": []any{map[string]any{"name": "greeting", "value": "hello"}}})
	wait := b.node("wait", durableWaitType, map[string]any{"seconds": 5.0})
	b.edge(start, PortOut, set)
	b.edge(set, PortOut, wait)
	doc := b.build()
	clock := newFakeClock(storeNow)
	engine, err := newDurableEngine(&Services{Clock: clock, Tools: &fakeTools{}}, discardLogger())
	if err != nil {
		t.Fatal(err)
	}
	var saved durableCheckpointState
	var events []RunEvent
	control := &durableRunControl{deadline: storeNow.Add(10 * time.Minute), save: func(_ context.Context, state durableCheckpointState, _ StepRecord) error {
		saved = state
		_, err := encodeDurableState(state)
		return err
	}}
	req := RunRequest{RunID: "run_durable", Flow: doc, Revision: 1, Mode: ModeLive, TriggerNode: start, TriggerType: "manual", durable: control}
	result := engine.Execute(context.Background(), req, func(event RunEvent) { events = append(events, event) })
	if result.Status != RunWaiting || !result.suspended || saved.WaitingNode != wait || !saved.WakeAt.Equal(storeNow.Add(5*time.Second)) {
		t.Fatalf("suspend = %+v, state %+v", result, saved)
	}
	for _, event := range events {
		if event.Type == EventRunFinished {
			t.Fatal("suspension emitted a terminal event")
		}
	}
	if len(events) == 0 || saved.EventSeq != events[len(events)-1].Seq {
		t.Fatalf("event sequence: %+v, saved %d", events, saved.EventSeq)
	}
	control.save = func(context.Context, durableCheckpointState, StepRecord) error {
		return errors.New("storage unavailable")
	}
	failed := engine.Execute(context.Background(), req, nil)
	if failed.Status != RunError || failed.ErrorCode != "FLOW_NODE_UNAVAILABLE" {
		t.Fatalf("failed checkpoint = %+v", failed)
	}
	unsafe, _ := doc.Clone()
	unsafe.Nodes = append(unsafe.Nodes, Node{ID: testNodeID(20), Key: "danger", Type: TypeHTTPRequest, TypeVersion: 1, Settings: NodeSettings{Disabled: true}})
	req.Flow = unsafe
	if got := engine.Execute(context.Background(), req, nil); got.Status != RunError || got.ErrorCode != "FLOW_NODE_UNAVAILABLE" {
		t.Fatalf("unsafe graph = %+v", got)
	}
	req.Flow = doc
	if got := NewEngine(engine.reg, nil, discardLogger(), 1).Execute(context.Background(), req, nil); got.Status != RunError {
		t.Fatalf("ordinary engine accepted durable request: %+v", got)
	}
}

func TestDurableCoordinatorResumesWithoutReplayingCompletedNodes(t *testing.T) {
	b := newFlow("Resume")
	start := b.node("start", TypeTriggerManual, nil)
	before := b.node("before", TypeSet, map[string]any{"fields": []any{map[string]any{"name": "value", "value": "once"}}})
	wait := b.node("wait", durableWaitType, map[string]any{"seconds": 1.0})
	after := b.node("after", TypeSet, map[string]any{"fields": []any{map[string]any{"name": "value", "value": "{{before.value}}"}}})
	b.edge(start, PortOut, before)
	b.edge(before, PortOut, wait)
	b.edge(wait, PortOut, after)
	now := time.Now().UTC().Truncate(time.Microsecond)
	clock := newFakeClock(now)
	engine, err := newDurableEngine(&Services{Clock: clock}, discardLogger())
	if err != nil {
		t.Fatal(err)
	}
	var saved durableCheckpointState
	var checkpoints []durableCheckpointState
	finished := 0
	control := &durableRunControl{deadline: now.Add(30 * time.Second),
		save: func(_ context.Context, state durableCheckpointState, _ StepRecord) error { saved = state; return nil },
		checkpoint: func(_ context.Context, state durableCheckpointState, _ StepRecord) error {
			encoded, err := encodeDurableState(state)
			if err != nil {
				return err
			}
			cloned, err := decodeDurableState(encoded)
			if err != nil {
				return err
			}
			checkpoints = append(checkpoints, cloned)
			return nil
		},
		finish: func(_ context.Context, _ RunResult, _ []RunEvent) error { finished++; return nil },
	}
	req := RunRequest{RunID: "run_resume", Flow: b.build(), Revision: 1, Mode: ModeLive, TriggerNode: start, TriggerType: "manual", durable: control}
	if first := engine.Execute(context.Background(), req, nil); first.Status != RunWaiting {
		t.Fatalf("initial run %+v", first)
	}
	clock.Advance(time.Second)
	res := resumeDurableCoordinator(context.Background(), saved, &Services{Clock: clock}, discardLogger(), control, nil, nil)
	if res.Status != RunSuccess || res.durabilityLost || finished != 1 || len(checkpoints) != 2 {
		t.Fatalf("resumed = %+v, checkpoints %d, finishes %d", res, len(checkpoints), finished)
	}
	if res.Outputs["after"]["value"] != "once" {
		t.Fatalf("restored output = %+v", res.Outputs)
	}
	for _, step := range res.Steps {
		if step.NodeID == start || step.NodeID == before {
			t.Fatalf("completed node replayed: %+v", step)
		}
	}
	if saved.Statuses[wait] != StepWaiting || saved.Outputs["after"] != nil {
		t.Fatal("resume modified the saved checkpoint")
	}
	control2 := &durableRunControl{save: control.save, checkpoint: func(context.Context, durableCheckpointState, StepRecord) error { return nil },
		finish: func(context.Context, RunResult, []RunEvent) error { return nil }}
	fromActive := resumeDurableCoordinator(context.Background(), checkpoints[0], &Services{Clock: clock}, discardLogger(), control2, nil, nil)
	if fromActive.Status != RunSuccess || fromActive.Outputs["after"]["value"] != "once" {
		t.Fatalf("recover active checkpoint: %+v", fromActive)
	}
	for _, step := range fromActive.Steps {
		if step.NodeID == wait || step.NodeID == before || step.NodeID == start {
			t.Fatalf("active recovery replayed step: %+v", step)
		}
	}
	control.checkpoint = func(context.Context, durableCheckpointState, StepRecord) error {
		return errors.New("storage unavailable")
	}
	beforeFinishes := finished
	failed := resumeDurableCoordinator(context.Background(), saved, &Services{Clock: clock}, discardLogger(), control, nil, nil)
	if !failed.durabilityLost || failed.Status != RunError || finished != beforeFinishes || len(failed.Outputs) != 0 {
		t.Fatalf("checkpoint failure was exposed or finalized: %+v", failed)
	}
}
