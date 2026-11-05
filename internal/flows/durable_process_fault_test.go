package flows

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDurableRunnerCrashAfterActiveCheckpoint(t *testing.T) {
	const envName = "AURAGO_DURABLE_ACTIVE_FAULT_CHILD"
	if dir := os.Getenv(envName); dir != "" {
		ctx := context.Background()
		store, err := OpenStore(filepath.Join(dir, "flows.db"), discardLogger())
		if err != nil {
			t.Fatal(err)
		}
		b := newFlow("Active checkpoint crash")
		start := b.node("start", TypeTriggerManual, nil)
		before := b.node("before", TypeSet, map[string]any{"fields": []any{map[string]any{"name": "value", "type": "text", "value": "before"}}, "keep_input": false})
		wait := b.node("wait", durableWaitType, map[string]any{"seconds": 1.0})
		after := b.node("after", TypeSet, map[string]any{"fields": []any{map[string]any{"name": "value", "type": "text", "value": "after"}}, "keep_input": false})
		tail := b.node("tail", TypeSet, map[string]any{"fields": []any{map[string]any{"name": "value", "type": "text", "value": "{{after.value}}"}}, "keep_input": false})
		b.edge(start, PortOut, before)
		b.edge(before, PortOut, wait)
		b.edge(wait, PortOut, after)
		b.edge(after, PortOut, tail)
		doc := b.build()
		doc.ID = "flow_active_process"
		doc.Settings.MaxRunSeconds = 120
		if _, err := store.CreateFlow(ctx, doc, "mission_active_process", time.Now()); err != nil {
			t.Fatal(err)
		}
		pub, err := store.Publish(ctx, doc.ID, 1, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		bridge := &durableStatusBridge{fakeBridge: newFakeBridge()}
		bridge.missions["mission_active_process"] = &fakeMission{flowID: doc.ID, enabled: true}
		clock := newFakeClock(time.Now().UTC())
		svc := &Services{CheckpointKeys: &durableReviewKeys{}, Clock: clock, Location: time.UTC}
		s := NewService(store, catalogRegistry(t, fullEnv()), svc, bridge, ServiceConfig{DurableEnabled: true}, discardLogger())
		s.runner.testOnEvent = func(ev RunEvent) {
			if ev.Type == EventStepFinished && ev.Step != nil && ev.Step.NodeKey == "after" && ev.Step.Status == StepSuccess {
				os.Exit(29)
			}
		}
		if err := s.Start(ctx); err != nil {
			t.Fatal(err)
		}
		started, err := s.runner.StartContext(ctx, StartRequest{Flow: pub.Live, Revision: pub.LiveRevision, Mode: ModeLive, TriggerNode: start, TriggerType: "manual", missionID: "mission_active_process"})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "run-id"), []byte(started.RunID), 0600); err != nil {
			t.Fatal(err)
		}
		durableReviewWait(t, s.runner, store, started.RunID)
		for i := 0; i < 200; i++ {
			clock.Advance(time.Second)
			time.Sleep(10 * time.Millisecond)
		}
		t.Fatal("child did not crash after committed active step")
	}
	dir := t.TempDir()
	cmd := exec.Command(os.Args[0], "-test.run=^TestDurableRunnerCrashAfterActiveCheckpoint$")
	cmd.Env = append(os.Environ(), envName+"="+dir)
	output, err := cmd.CombinedOutput()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 29 {
		t.Fatalf("child exit=%v output=%s", err, output)
	}
	idBytes, err := os.ReadFile(filepath.Join(dir, "run-id"))
	if err != nil {
		t.Fatal(err)
	}
	runID := strings.TrimSpace(string(idBytes))
	store, err := OpenStore(filepath.Join(dir, "flows.db"), discardLogger())
	if err != nil {
		t.Fatal(err)
	}
	before, steps, err := store.GetRun(context.Background(), runID)
	if err != nil {
		t.Fatal(err)
	}
	if before.Status != RunRunning {
		t.Fatalf("unexpected crash state %+v", before)
	}
	var committed int
	for _, step := range steps {
		if step.NodeKey == "after" && step.Status == StepSuccess {
			committed++
		}
	}
	if committed != 1 {
		t.Fatalf("active step was not committed: %+v", steps)
	}
	bridge := &durableStatusBridge{fakeBridge: newFakeBridge()}
	bridge.missions["mission_active_process"] = &fakeMission{flowID: before.FlowID, enabled: true}
	clock := newFakeClock(time.Now().UTC().Add(45 * time.Second))
	s := NewService(store, catalogRegistry(t, fullEnv()), &Services{CheckpointKeys: &durableReviewKeys{}, Clock: clock, Location: time.UTC}, bridge, ServiceConfig{DurableEnabled: true}, discardLogger())
	t.Cleanup(func() { _ = s.Shutdown(context.Background()); _ = store.Close() })
	if err := s.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	until := time.Now().Add(8 * time.Second)
	for time.Now().Before(until) {
		header, steps, err := store.GetRun(context.Background(), runID)
		if err != nil {
			t.Fatal(err)
		}
		if header.Status.Terminal() {
			if header.Status != RunSuccess || !header.StartedAt.Equal(before.StartedAt) {
				t.Fatalf("recovered run %+v", header)
			}
			var after, tail int
			for _, step := range steps {
				if step.NodeKey == "after" {
					after++
				}
				if step.NodeKey == "tail" {
					tail++
				}
			}
			if after != 1 || tail != 1 {
				t.Fatalf("steps repeated or missing: %+v", steps)
			}
			return
		}
		clock.Advance(time.Second)
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("claimed checkpoint did not resume after process exit")
}
