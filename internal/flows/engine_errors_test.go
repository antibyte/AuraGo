package flows

import (
	"context"
	"math"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func singleNodeFlow(name, key, typ string, params map[string]any) (*flowBuilder, string, string) {
	b := newFlow(name)
	tr := b.node("start", "test.trigger", nil)
	n := b.node(key, typ, params)
	b.edge(tr, PortOut, n)
	return b, tr, n
}

func TestEngineStopsOnError(t *testing.T) {
	reg := newTestRegistry(t)
	b, tr, fail := singleNodeFlow("Stop", "fail", "test.fail", nil)
	after := b.node("after", "test.echo", nil)
	b.edge(fail, PortOut, after)
	res, _ := runWith(context.Background(), newTestEngine(reg, nil, 4), b.build(), RunRequest{TriggerNode: tr})
	if res.Status != RunError || res.ErrorCode != "TEST_FAILED" || res.ErrorNodeID != fail || !strings.Contains(res.ErrorMessage, "fail: boom") {
		t.Fatalf("result = %+v", res)
	}
	if s := stepOf(res, after); s == nil || s.Status != StepSkipped {
		t.Fatalf("after = %+v", s)
	}
}

func TestEngineContinueOnError(t *testing.T) {
	reg := newTestRegistry(t)
	b, tr, fail := singleNodeFlow("Continue", "fail", "test.fail", nil)
	after := b.node("after", "test.echo", map[string]any{"value": "{{fail.error.code}}"})
	b.edge(fail, PortOut, after)
	f := b.build()
	f.NodeByID(fail).Settings.OnError = ErrorContinue
	res, _ := runWith(context.Background(), newTestEngine(reg, nil, 4), f, RunRequest{TriggerNode: tr})
	if res.Status != RunSuccess || res.Outputs["after"]["value"] != "TEST_FAILED" || stepOf(res, fail).Status != StepError {
		t.Fatalf("result = %s %#v", res.Status, res.Outputs)
	}
}

func TestEngineErrorPort(t *testing.T) {
	reg := newTestRegistry(t)
	b, tr, fail := singleNodeFlow("Port", "fail", "test.fail", nil)
	handler := b.node("handler", "test.echo", map[string]any{"value": "{{fail.error.message}}"})
	normal := b.node("normal", "test.echo", nil)
	b.edge(fail, PortError, handler)
	b.edge(fail, PortOut, normal)
	f := b.build()
	f.NodeByID(fail).Settings.OnError = ErrorPort
	res, _ := runWith(context.Background(), newTestEngine(reg, nil, 4), f, RunRequest{TriggerNode: tr})
	if res.Status != RunSuccess || res.Outputs["handler"]["value"] != "boom" {
		t.Fatalf("result = %s %#v", res.Status, res.Outputs)
	}
	if stepOf(res, normal).Status != StepSkipped {
		t.Fatal("the normal port must be skipped after an error")
	}
}

func registerFlaky(reg *Registry, failures int32) *int32 {
	var calls int32
	reg.MustRegister(&NodeDef{Type: "test.flaky", Execute: func(context.Context, ExecInput) (ExecResult, error) {
		if atomic.AddInt32(&calls, 1) <= failures {
			return ExecResult{}, NewNodeError("FLAKY", "not yet")
		}
		return ExecResult{Output: map[string]any{"ok": true}}, nil
	}})
	return &calls
}

func TestEngineRetries(t *testing.T) {
	reg := newTestRegistry(t)
	registerFlaky(reg, 2)
	b, tr, n := singleNodeFlow("Retry", "flaky", "test.flaky", nil)
	f := b.build()
	f.NodeByID(n).Settings.Retry = RetryPolicy{Count: 2}
	res, _ := runWith(context.Background(), newTestEngine(reg, nil, 4), f, RunRequest{TriggerNode: tr})
	if res.Status != RunSuccess || stepOf(res, n).Attempt != 3 {
		t.Fatalf("retry success = %s attempt %d", res.Status, stepOf(res, n).Attempt)
	}

	reg2 := newTestRegistry(t)
	registerFlaky(reg2, 5)
	f.NodeByID(n).Settings.Retry = RetryPolicy{Count: 1}
	res, _ = runWith(context.Background(), newTestEngine(reg2, nil, 4), f, RunRequest{TriggerNode: tr})
	if res.Status != RunError || res.ErrorCode != "FLAKY" || stepOf(res, n).Attempt != 2 {
		t.Fatalf("retry exhausted = %s %s attempt %d", res.Status, res.ErrorCode, stepOf(res, n).Attempt)
	}
}

func TestEngineRetryDelayUsesClock(t *testing.T) {
	reg := newTestRegistry(t)
	registerFlaky(reg, 1)
	clock := newFakeClock(time.Date(2026, 10, 3, 7, 0, 0, 0, time.UTC))
	eng := newTestEngine(reg, &Services{Clock: clock, Location: time.UTC}, 4)
	b, tr, n := singleNodeFlow("Delay", "flaky", "test.flaky", nil)
	f := b.build()
	f.NodeByID(n).Settings.Retry = RetryPolicy{Count: 1, DelaySeconds: 30}
	done := make(chan RunResult, 1)
	go func() {
		res, _ := runWith(context.Background(), eng, f, RunRequest{TriggerNode: tr})
		done <- res
	}()
	clock.WaitForWaiters(t, 1)
	clock.Advance(30 * time.Second)
	select {
	case res := <-done:
		if res.Status != RunSuccess || stepOf(res, n).Attempt != 2 {
			t.Fatalf("result = %s attempt %d", res.Status, stepOf(res, n).Attempt)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("the retry did not happen after the delay")
	}
}

func registerHang(reg *Registry, timeout time.Duration, started chan<- struct{}) {
	var once sync.Once
	reg.MustRegister(&NodeDef{Type: "test.hang", DefaultTimeout: timeout, Execute: func(ctx context.Context, _ ExecInput) (ExecResult, error) {
		if started != nil {
			once.Do(func() { close(started) })
		}
		<-ctx.Done()
		return ExecResult{}, ctx.Err()
	}})
}

func TestEngineNodeTimeout(t *testing.T) {
	reg := newTestRegistry(t)
	registerHang(reg, 50*time.Millisecond, nil)
	b, tr, _ := singleNodeFlow("Timeout", "hang", "test.hang", nil)
	res, _ := runWith(context.Background(), newTestEngine(reg, nil, 4), b.build(), RunRequest{TriggerNode: tr})
	if res.Status != RunError || res.ErrorCode != "FLOW_NODE_TIMEOUT" {
		t.Fatalf("result = %s %s", res.Status, res.ErrorCode)
	}
}

func TestEngineRunTimeout(t *testing.T) {
	reg := newTestRegistry(t)
	registerHang(reg, 10*time.Second, nil)
	b, tr, n := singleNodeFlow("RunTimeout", "hang", "test.hang", nil)
	res, _ := runWith(context.Background(), newTestEngine(reg, nil, 4), b.build(), RunRequest{TriggerNode: tr, Timeout: 50 * time.Millisecond})
	if res.Status != RunError || res.ErrorCode != "FLOW_RUN_TIMEOUT" || stepOf(res, n).Status != StepCancelled {
		t.Fatalf("result = %s %s step %+v", res.Status, res.ErrorCode, stepOf(res, n))
	}
}

func TestEngineCancel(t *testing.T) {
	reg := newTestRegistry(t)
	started := make(chan struct{})
	registerHang(reg, 10*time.Second, started)
	b, tr, n := singleNodeFlow("Cancel", "hang", "test.hang", nil)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-started
		cancel()
	}()
	res, _ := runWith(ctx, newTestEngine(reg, nil, 4), b.build(), RunRequest{TriggerNode: tr})
	if res.Status != RunCancelled || res.ErrorCode != "FLOW_CANCELLED" || stepOf(res, n).Status != StepCancelled {
		t.Fatalf("result = %s %s step %+v", res.Status, res.ErrorCode, stepOf(res, n))
	}
}

func TestEngineStopNode(t *testing.T) {
	reg := newTestRegistry(t)
	registerHang(reg, 10*time.Second, nil)
	eng := newTestEngine(reg, nil, 4)

	b, tr, _ := singleNodeFlow("StopError", "halt", TypeStop, map[string]any{"status": "error", "message": "Abbruch: {{trigger.data.why}}"})
	res, _ := runWith(context.Background(), eng, b.build(), RunRequest{TriggerNode: tr, TriggerData: map[string]any{"why": "test"}})
	if res.Status != RunError || res.ErrorCode != "FLOW_STOPPED" || res.ErrorMessage != "Abbruch: test" {
		t.Fatalf("stop error = %s %s %q", res.Status, res.ErrorCode, res.ErrorMessage)
	}

	b, tr, _ = singleNodeFlow("StopOK", "halt", TypeStop, nil)
	hang := b.node("hang", "test.hang", nil)
	b.edge(tr, PortOut, hang)
	start := time.Now()
	res, _ = runWith(context.Background(), eng, b.build(), RunRequest{TriggerNode: tr})
	if res.Status != RunSuccess || time.Since(start) > 5*time.Second {
		t.Fatalf("stop success = %s after %s", res.Status, time.Since(start))
	}
	if s := stepOf(res, hang); s == nil || s.Status != StepCancelled {
		t.Fatalf("parallel branch must be cancelled, got %+v", s)
	}
}

func TestEngineOutputLimits(t *testing.T) {
	reg := newTestRegistry(t)
	reg.MustRegister(&NodeDef{Type: "test.big", Execute: func(context.Context, ExecInput) (ExecResult, error) {
		return ExecResult{Output: map[string]any{"blob": strings.Repeat("x", MaxOutputBytes)}}, nil
	}})
	reg.MustRegister(&NodeDef{Type: "test.medium", Execute: func(context.Context, ExecInput) (ExecResult, error) {
		return ExecResult{Output: map[string]any{"blob": strings.Repeat("x", MaxStoredOutputBytes+10)}}, nil
	}})
	eng := newTestEngine(reg, nil, 4)

	b, tr, _ := singleNodeFlow("Big", "big", "test.big", nil)
	if res, _ := runWith(context.Background(), eng, b.build(), RunRequest{TriggerNode: tr}); res.ErrorCode != "FLOW_OUTPUT_TOO_LARGE" {
		t.Fatalf("big = %s", res.ErrorCode)
	}

	b, tr, medium := singleNodeFlow("Medium", "medium", "test.medium", nil)
	count := b.node("count", "test.echo", map[string]any{"value": "{{medium.blob | count}}"})
	b.edge(medium, PortOut, count)
	res, _ := runWith(context.Background(), eng, b.build(), RunRequest{TriggerNode: tr})
	if res.Status != RunSuccess || !stepOf(res, medium).OutputTruncated {
		t.Fatalf("medium = %s truncated=%v", res.Status, stepOf(res, medium).OutputTruncated)
	}
	if got := res.Outputs["count"]["value"]; got != float64(MaxStoredOutputBytes+10) {
		t.Fatalf("downstream must see the full output, count = %#v", got)
	}
}

func TestEnginePanicIsReported(t *testing.T) {
	reg := newTestRegistry(t)
	reg.MustRegister(&NodeDef{Type: "test.panic", Execute: func(context.Context, ExecInput) (ExecResult, error) {
		panic("kaputt")
	}})
	b, tr, _ := singleNodeFlow("Panic", "p", "test.panic", nil)
	res, _ := runWith(context.Background(), newTestEngine(reg, nil, 4), b.build(), RunRequest{TriggerNode: tr})
	if res.ErrorCode != "FLOW_NODE_PANIC" || !strings.Contains(res.ErrorMessage, "kaputt") {
		t.Fatalf("panic = %s %s", res.ErrorCode, res.ErrorMessage)
	}
}

func TestEngineRejectsUndeclaredPorts(t *testing.T) {
	reg := newTestRegistry(t)
	reg.MustRegister(&NodeDef{Type: "test.badport", Execute: func(context.Context, ExecInput) (ExecResult, error) {
		return ExecResult{Output: map[string]any{"ok": true}, Ports: []string{PortOut, "nope"}}, nil
	}})
	eng := newTestEngine(reg, nil, 4)
	b, tr, n := singleNodeFlow("BadPort", "bad", "test.badport", nil)
	after := b.node("after", "test.echo", nil)
	b.edge(n, PortOut, after)
	f := b.build()
	res, _ := runWith(context.Background(), eng, f, RunRequest{TriggerNode: tr})
	if res.Status != RunError || res.ErrorCode != "FLOW_PORT_INVALID" || res.ErrorNodeID != n || !strings.Contains(res.ErrorMessage, `node returned unknown output "nope"`) {
		t.Fatalf("result = %s %s %s %q", res.Status, res.ErrorCode, res.ErrorNodeID, res.ErrorMessage)
	}
	if s := mustStep(t, res, n); s.Status != StepError || s.ErrorCode != "FLOW_PORT_INVALID" || s.Output != nil || s.Ports != nil {
		t.Fatalf("step = %+v", s)
	}
	if s := mustStep(t, res, after); s.Status != StepSkipped {
		t.Fatalf("after = %+v", s)
	}

	// The node's on_error setting applies as for any other failure.
	handler := b.node("handler", "test.echo", map[string]any{"value": "{{bad.error.code}}"})
	b.edge(n, PortError, handler)
	f = b.build()
	f.NodeByID(n).Settings.OnError = ErrorPort
	res, _ = runWith(context.Background(), eng, f, RunRequest{TriggerNode: tr})
	if res.Status != RunSuccess || res.Outputs["handler"]["value"] != "FLOW_PORT_INVALID" || mustStep(t, res, after).Status != StepSkipped {
		t.Fatalf("error port = %s %#v", res.Status, res.Outputs)
	}
}

func TestEngineEmptyDefaultPort(t *testing.T) {
	reg := newTestRegistry(t)
	reg.MustRegister(&NodeDef{Type: "test.noout", Outputs: []string{}, Execute: func(context.Context, ExecInput) (ExecResult, error) {
		return ExecResult{}, NewNodeError("X", "y")
	}})
	reg.MustRegister(&NodeDef{Type: "test.noouttrigger", Trigger: true, Outputs: []string{}})
	eng := newTestEngine(reg, nil, 4)

	// on_error continue on a node without outputs: no port, nothing delivered.
	b, tr, n := singleNodeFlow("NoOut", "noout", "test.noout", nil)
	after := b.node("after", "test.echo", nil)
	b.edge(n, "", after)
	f := b.build()
	f.NodeByID(n).Settings.OnError = ErrorContinue
	res, _ := runWith(context.Background(), eng, f, RunRequest{TriggerNode: tr})
	if s := mustStep(t, res, n); res.Status != RunSuccess || s.Status != StepError || s.Ports != nil {
		t.Fatalf("continue = %s, step %s ports %q", res.Status, s.Status, s.Ports)
	}
	if s := mustStep(t, res, after); s.Status != StepSkipped {
		t.Fatalf("after = %s, want skipped", s.Status)
	}

	// A trigger without outputs.
	b = newFlow("NoOutTrigger")
	tr = b.node("start", "test.noouttrigger", nil)
	a := b.node("a", "test.echo", nil)
	b.edge(tr, "", a)
	res, _ = runWith(context.Background(), eng, b.build(), RunRequest{TriggerNode: tr})
	if s := mustStep(t, res, tr); res.Status != RunSuccess || s.Ports != nil {
		t.Fatalf("trigger = %s ports %q", res.Status, s.Ports)
	}
	if s := mustStep(t, res, a); s.Status != StepSkipped {
		t.Fatalf("a = %s, want skipped", s.Status)
	}
}

func TestEngineRejectsErrorPortOnSuccess(t *testing.T) {
	reg := newTestRegistry(t)
	reg.MustRegister(&NodeDef{Type: "test.errsuccess", Execute: func(context.Context, ExecInput) (ExecResult, error) {
		return ExecResult{Output: map[string]any{"ok": true}, Ports: []string{PortError}}, nil
	}})
	eng := newTestEngine(reg, nil, 4)
	b, tr, n := singleNodeFlow("ErrSuccess", "bad", "test.errsuccess", nil)
	handler := b.node("handler", "test.echo", map[string]any{"value": "{{bad.error.code}}/{{bad.ok}}"})
	b.edge(n, PortError, handler)
	f := b.build()
	f.NodeByID(n).Settings.OnError = ErrorPort
	res, _ := runWith(context.Background(), eng, f, RunRequest{TriggerNode: tr})
	if res.Status != RunSuccess || res.Outputs["handler"]["value"] != "FLOW_PORT_INVALID/" {
		t.Fatalf("error port = %s %#v", res.Status, res.Outputs["handler"])
	}
	f.NodeByID(n).Settings.OnError = ErrorStop
	res, _ = runWith(context.Background(), eng, f, RunRequest{TriggerNode: tr})
	if res.Status != RunError || res.ErrorCode != "FLOW_PORT_INVALID" || res.ErrorNodeID != n {
		t.Fatalf("stop = %s %s %s", res.Status, res.ErrorCode, res.ErrorNodeID)
	}
}

func TestEngineCancelDuringRetryDelay(t *testing.T) {
	reg := newTestRegistry(t)
	calls := registerFlaky(reg, 1)
	clock := newFakeClock(time.Date(2026, 10, 3, 7, 0, 0, 0, time.UTC))
	eng := newTestEngine(reg, &Services{Clock: clock, Location: time.UTC}, 4)
	b, tr, n := singleNodeFlow("CancelDelay", "flaky", "test.flaky", nil)
	f := b.build()
	f.NodeByID(n).Settings.Retry = RetryPolicy{Count: 1, DelaySeconds: 30}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan RunResult, 1)
	go func() {
		res, _ := runWith(ctx, eng, f, RunRequest{TriggerNode: tr})
		done <- res
	}()
	clock.WaitForWaiters(t, 1)
	cancel()
	select {
	case res := <-done:
		s := mustStep(t, res, n)
		if res.Status != RunCancelled || s.Status != StepCancelled || s.Attempt != 1 || atomic.LoadInt32(calls) != 1 {
			t.Fatalf("result = %s, step %s attempt %d, %d calls", res.Status, s.Status, s.Attempt, atomic.LoadInt32(calls))
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the cancel did not end the retry delay")
	}
}

func TestEngineOutputInvalid(t *testing.T) {
	reg := newTestRegistry(t)
	reg.MustRegister(&NodeDef{Type: "test.nanout", Execute: func(context.Context, ExecInput) (ExecResult, error) {
		return ExecResult{Output: map[string]any{"v": math.NaN()}}, nil
	}})
	b, tr, n := singleNodeFlow("NaN", "nan", "test.nanout", nil)
	res, _ := runWith(context.Background(), newTestEngine(reg, nil, 4), b.build(), RunRequest{TriggerNode: tr})
	if s := mustStep(t, res, n); res.Status != RunError || res.ErrorCode != "FLOW_OUTPUT_INVALID" || res.ErrorNodeID != n || s.Status != StepError {
		t.Fatalf("result = %s %s %s, step %s", res.Status, res.ErrorCode, res.ErrorNodeID, s.Status)
	}
}

func TestEngineRetriesAfterNodeTimeout(t *testing.T) {
	reg := newTestRegistry(t)
	var calls int32
	reg.MustRegister(&NodeDef{Type: "test.slowfirst", DefaultTimeout: 20 * time.Millisecond, Execute: func(ctx context.Context, _ ExecInput) (ExecResult, error) {
		if atomic.AddInt32(&calls, 1) == 1 {
			<-ctx.Done() // the first attempt runs into the node timeout
			return ExecResult{}, ctx.Err()
		}
		return ExecResult{Output: map[string]any{"ok": true}}, nil
	}})
	b, tr, n := singleNodeFlow("TimeoutRetry", "slow", "test.slowfirst", nil)
	f := b.build()
	f.NodeByID(n).Settings.Retry = RetryPolicy{Count: 1}
	res, _ := runWith(context.Background(), newTestEngine(reg, nil, 4), f, RunRequest{TriggerNode: tr})
	if s := mustStep(t, res, n); res.Status != RunSuccess || s.Status != StepSuccess || s.Attempt != 2 {
		t.Fatalf("result = %s %s, step %s attempt %d", res.Status, res.ErrorCode, s.Status, s.Attempt)
	}
}
