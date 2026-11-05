package flows

import (
	"context"
	"encoding/json"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestChildHistoryActualEngineRetryAttempts(t *testing.T) {
	reg := newTestRegistry(t)
	var calls atomic.Int32
	reg.MustRegister(&NodeDef{Type: "test.child_retry", Execute: func(context.Context, ExecInput) (ExecResult, error) {
		if calls.Add(1) < 3 {
			return ExecResult{}, NewNodeError("FLOW_NODE_FAILED", "private message")
		}
		return ExecResult{Output: map[string]any{"ok": true}}, nil
	}})
	b, trigger, node := singleNodeFlow("Retry child", "retry", "test.child_retry", nil)
	doc := b.build()
	doc.NodeByID(node).Settings.Retry = RetryPolicy{Count: 2}
	collector := newChildCollector(&blockRunScope{})
	frame := collector.open("", testNodeID(8), 1, &resolvedBlock{Ref: blockRef{FlowID: "block_retry", Revision: 1}}, nil)
	result := newTestEngine(reg, nil, 1).Execute(context.Background(), RunRequest{Flow: doc, Mode: ModeTest, TriggerNode: trigger, TriggerType: "manual", childCollector: collector, childFrame: frame, childNested: true}, nil)
	trace := collector.freeze(false)
	if result.Status != RunSuccess || calls.Load() != 3 || len(trace.Frames) != 1 {
		t.Fatalf("result=%+v trace=%+v", result, trace)
	}
	var attempts []ChildStep
	for _, entry := range trace.Frames[0].Steps {
		if entry.Step.NodeID == node {
			attempts = append(attempts, entry)
		}
	}
	if len(attempts) != 3 || attempts[0].Step.Attempt != 1 || attempts[1].Step.Attempt != 2 || attempts[2].Step.Attempt != 3 || attempts[0].Step.Status != StepError || attempts[2].Step.Status != StepSuccess {
		t.Fatalf("attempts: %+v", attempts)
	}
	encoded, err := json.Marshal(attempts[0])
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "started_at") || strings.Contains(string(encoded), "private message") {
		t.Fatalf("invented timing or private error: %s", encoded)
	}
}

type childExitJSON struct {
	kind    string
	started chan struct{}
	release chan struct{}
}

func (v childExitJSON) MarshalJSON() ([]byte, error) {
	switch v.kind {
	case "panic":
		panic("private failure")
	case "goexit":
		runtime.Goexit()
	case "abandon":
		close(v.started)
		<-v.release
	}
	return []byte(`"value"`), nil
}

func TestChildHistoryActualWorkerExitAndAbandon(t *testing.T) {
	for _, kind := range []string{"panic", "goexit", "abandon"} {
		t.Run(kind, func(t *testing.T) {
			ctx := context.Background()
			s, _ := newServiceFixture(t, nil)
			block := publishedGreetingBlock(t, s)
			parent, call := blockParent(t, s, block, "Abrupt child")
			plan, err := prepareBlockPlan(ctx, s.store, parent.Draft, s.services)
			if err != nil {
				t.Fatal(err)
			}
			doc, err := parent.Draft.Clone()
			if err != nil {
				t.Fatal(err)
			}
			started, release := make(chan struct{}), make(chan struct{})
			released := false
			if kind == "abandon" {
				defer func() {
					if !released {
						close(release)
					}
				}()
			}
			doc.NodeByID(call).Params["inputs"] = map[string]any{"name": childExitJSON{kind: kind, started: started, release: release}}
			var trigger string
			for _, n := range doc.Nodes {
				if n.Type == TypeTriggerManual {
					trigger = n.ID
					break
				}
			}
			eng := newTestEngine(s.reg, s.services, 1)
			eng.abandonAfter = 30 * time.Millisecond
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()
			if kind == "abandon" {
				go func() {
					select {
					case <-started:
						cancel()
					case <-ctx.Done():
					}
				}()
			}
			done := make(chan RunResult, 1)
			go func() {
				done <- eng.Execute(ctx, RunRequest{Flow: doc, Mode: ModeTest, TriggerNode: trigger, TriggerType: "manual", blocks: plan, scope: &blockRunScope{}}, nil)
			}()
			select {
			case result := <-done:
				if len(result.childTraces) != 1 || result.childTraces[0].History.State != ChildHistoryIncomplete {
					t.Fatalf("%s result=%+v", kind, result)
				}
				before, _ := json.Marshal(result.childTraces[0])
				if kind == "abandon" {
					close(release)
					released = true
					time.Sleep(20 * time.Millisecond)
				}
				after, _ := json.Marshal(result.childTraces[0])
				if string(before) != string(after) {
					t.Fatal("late worker altered frozen trace")
				}
			case <-time.After(5 * time.Second):
				t.Fatal("worker did not end")
			}
		})
	}
}

func TestChildHistorySharedParallelLimits(t *testing.T) {
	for _, framesOnly := range []bool{true, false} {
		scope := &blockRunScope{}
		const workers = 10
		collectors := make([]*childCollector, workers)
		var wg sync.WaitGroup
		for i := range collectors {
			collectors[i] = newChildCollector(scope)
			wg.Add(1)
			go func(c *childCollector) {
				defer wg.Done()
				block := &resolvedBlock{Ref: blockRef{FlowID: "block_limits", Revision: 1}}
				if framesOnly {
					for j := 0; j < 110; j++ {
						c.open("", testNodeID(7), 1, block, nil)
					}
					return
				}
				frame := c.open("", testNodeID(7), 1, block, nil)
				for j := 0; j < 210; j++ {
					c.step(frame, StepRecord{NodeID: testNodeID(8), NodeKey: "step", Status: StepSuccess})
				}
			}(collectors[i])
		}
		wg.Wait()
		frames, steps := 0, 0
		truncated := false
		for _, c := range collectors {
			trace := c.freeze(false)
			frames += trace.History.RetainedInvocations
			steps += trace.History.RetainedSteps
			truncated = truncated || trace.History.Truncated
		}
		if frames > maxChildHistoryFrames || steps > maxChildHistorySteps || !truncated {
			t.Fatalf("framesOnly=%v frames=%d steps=%d truncated=%v", framesOnly, frames, steps, truncated)
		}
	}
}

func TestChildHistoryLateScrubDropsStructuralCollision(t *testing.T) {
	secret := "private_key_collision"
	var active atomic.Bool
	scrub := Scrubber{Text: func(s string) string {
		if active.Load() {
			return strings.ReplaceAll(s, secret, "[redacted]")
		}
		return s
	}, JSON: func(data []byte) ([]byte, error) {
		if active.Load() {
			return []byte(strings.ReplaceAll(string(data), secret, "[redacted]")), nil
		}
		return data, nil
	}}
	collector := newChildCollector(&blockRunScope{historyScrubber: scrub})
	frame := collector.open("", testNodeID(7), 1, &resolvedBlock{Ref: blockRef{FlowID: "block_private", Revision: 1}}, nil)
	collector.step(frame, StepRecord{NodeID: testNodeID(8), NodeKey: "step", Status: StepSuccess, Output: map[string]any{secret: "first", "[redacted]": "second"}})
	trace := collector.freeze(false)
	trace.ParentSeq = 1
	trace.History.ParentSeq = 1
	active.Store(true)
	data, err := EncodeChildHistoryResponse(trace, scrub)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), secret) {
		t.Fatalf("late secret leaked: %s", data)
	}
	var got ChildTrace
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Frames) != 1 || len(got.Frames[0].Steps) != 1 || !got.Frames[0].Steps[0].Step.OutputTruncated || got.Frames[0].Steps[0].Step.Output["_omitted"] == nil {
		t.Fatalf("collision not omitted: %+v", got)
	}
	data, err = EncodeChildHistoryResponse(trace, Scrubber{Text: func(s string) string {
		if s == "block_private" {
			return "[redacted]"
		}
		return s
	}})
	if err != nil || json.Unmarshal(data, &got) != nil || len(got.Frames) != 0 || got.History.State != ChildHistoryIncomplete {
		t.Fatalf("redacted structural identity: %s %v", data, err)
	}
}

func TestChildHistoryShortErrorForwardingIsOmitted(t *testing.T) {
	reg := newTestRegistry(t)
	secret := "late-secret-crossing-boundary"
	message := strings.Repeat("x", 990) + secret + strings.Repeat("y", 50)
	reg.MustRegister(&NodeDef{Type: "test.child_error", Execute: func(context.Context, ExecInput) (ExecResult, error) {
		return ExecResult{}, NewNodeError("FLOW_NODE_FAILED", "%s", message)
	}})
	b := newFlow("Error forwarding")
	trigger := b.node("start", "test.trigger", nil)
	fail := b.node("bad", "test.child_error", nil)
	after := b.node("after", "test.echo", map[string]any{"value": "{{bad.error.message}}"})
	doc := b.build()
	doc.NodeByID(fail).Settings.OnError = ErrorContinue
	b.edge(trigger, PortOut, fail)
	b.edge(fail, PortOut, after)
	collector := newChildCollector(&blockRunScope{})
	frame := collector.open("", testNodeID(7), 1, &resolvedBlock{Ref: blockRef{FlowID: "block_error", Revision: 1}}, nil)
	result := newTestEngine(reg, nil, 1).Execute(context.Background(), RunRequest{Flow: doc, Mode: ModeTest, TriggerNode: trigger, TriggerType: "manual", childCollector: collector, childFrame: frame, childNested: true}, nil)
	trace := collector.freeze(false)
	if result.Status != RunSuccess || !strings.Contains(result.Outputs["after"]["value"].(string), "late-secr") {
		t.Fatalf("execution changed: %+v", result)
	}
	for _, entry := range trace.Frames[0].Steps {
		if entry.Step.NodeID == fail && (entry.Step.ErrorMessage != "" || !entry.Step.OutputTruncated || entry.Step.Output["_omitted"] == nil) {
			t.Fatalf("error output retained: %+v", entry.Step)
		}
		if entry.Step.NodeID == after && (!entry.Step.ParamsTruncated || !entry.Step.OutputTruncated || entry.Step.Params["_omitted"] == nil || entry.Step.Output["_omitted"] == nil) {
			t.Fatalf("derived value retained: %+v", entry.Step)
		}
	}
	trace.ParentSeq = 1
	trace.History.ParentSeq = 1
	scrub := Scrubber{Text: func(s string) string { return strings.ReplaceAll(s, secret, "[redacted]") }, JSON: func(b []byte) ([]byte, error) {
		return []byte(strings.ReplaceAll(string(b), secret, "[redacted]")), nil
	}}
	encoded, err := EncodeChildHistoryResponse(trace, scrub)
	if err != nil || strings.Contains(string(encoded), "late-secr") {
		t.Fatalf("partial secret leaked: %s %v", encoded, err)
	}
}

func TestChildHistoryParentErrorOmitsFollowingCallValues(t *testing.T) {
	ctx := context.Background()
	s, _ := newServiceFixture(t, nil)
	block := publishedGreetingBlock(t, s)
	b := newFlow("Forwarded outer error")
	trigger := b.node("start", "test.trigger", nil)
	first := b.node("first", blockCallType, map[string]any{"flow_id": block.ID, "revision": float64(block.LiveRevision), "inputs": map[string]any{"name": float64(42)}})
	second := b.node("second", blockCallType, map[string]any{"flow_id": block.ID, "revision": float64(block.LiveRevision), "inputs": map[string]any{"name": "{{first.error.message}}"}})
	doc := b.build()
	doc.NodeByID(first).Settings.OnError = ErrorPort
	b.edge(trigger, PortOut, first)
	b.edge(first, PortError, second)
	plan, err := prepareBlockPlan(ctx, s.store, doc, s.services)
	if err != nil {
		t.Fatal(err)
	}
	result := newTestEngine(newTestRegistry(t), nil, 1).Execute(ctx, RunRequest{Flow: doc, Mode: ModeTest, TriggerNode: trigger, TriggerType: "manual", blocks: plan, scope: &blockRunScope{}}, nil)
	if result.Status != RunSuccess || len(result.childTraces) != 2 {
		t.Fatalf("outer run: %+v", result)
	}
	trace := result.childTraces[1]
	if len(trace.Frames) != 1 || len(trace.Frames[0].Steps) == 0 {
		t.Fatalf("second trace: %+v", trace)
	}
	for _, entry := range trace.Frames[0].Steps {
		if entry.Step.Output != nil && (!entry.Step.OutputTruncated || entry.Step.Output["_omitted"] == nil) {
			t.Fatalf("forwarded output retained: %+v", entry.Step)
		}
		if entry.Step.Params != nil && (!entry.Step.ParamsTruncated || entry.Step.Params["_omitted"] == nil) {
			t.Fatalf("forwarded params retained: %+v", entry.Step)
		}
	}
}
