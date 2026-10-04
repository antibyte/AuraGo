package flows

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"
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

// panicMarshaler panics when the engine encodes it, outside the node's Execute.
type panicMarshaler struct{}

func (panicMarshaler) MarshalJSON() ([]byte, error) { panic("marshal kaputt") }

func TestEnginePanicOutsideExecuteIsReported(t *testing.T) {
	reg := newTestRegistry(t)
	reg.MustRegister(&NodeDef{Type: "test.badjson", Execute: func(context.Context, ExecInput) (ExecResult, error) {
		return ExecResult{Output: map[string]any{"v": panicMarshaler{}}}, nil
	}})
	b, tr, n := singleNodeFlow("BadJSON", "bad", "test.badjson", nil)
	after := b.node("after", "test.echo", nil)
	b.edge(n, PortOut, after)
	res, capture := runWith(context.Background(), newTestEngine(reg, nil, 4), b.build(), RunRequest{TriggerNode: tr})
	if res.Status != RunError || res.ErrorCode != "FLOW_NODE_PANIC" || res.ErrorNodeID != n || !strings.Contains(res.ErrorMessage, "marshal kaputt") {
		t.Fatalf("result = %s %s %s %q", res.Status, res.ErrorCode, res.ErrorNodeID, res.ErrorMessage)
	}
	if s := mustStep(t, res, n); s.Status != StepError || s.ErrorCode != "FLOW_NODE_PANIC" {
		t.Fatalf("step = %+v", s)
	}
	if s := mustStep(t, res, after); s.Status != StepSkipped {
		t.Fatalf("after = %+v", s)
	}
	if types := capture.types(); types[len(types)-1] != EventRunFinished {
		t.Fatalf("events = %v", types)
	}
}

func TestEngineBoundsErrorMessages(t *testing.T) {
	huge := strings.Repeat("ä", 1<<19) // 1 MiB
	shared := &NodeError{Code: "BIG", Message: huge}
	reg := newTestRegistry(t)
	reg.MustRegister(&NodeDef{Type: "test.bigerr", Execute: func(context.Context, ExecInput) (ExecResult, error) {
		return ExecResult{}, shared
	}})
	reg.MustRegister(&NodeDef{Type: "test.bigpanic", Execute: func(context.Context, ExecInput) (ExecResult, error) {
		panic(huge)
	}})
	reg.MustRegister(&NodeDef{Type: "test.bigstop", Execute: func(context.Context, ExecInput) (ExecResult, error) {
		return ExecResult{Stop: &StopSignal{Status: RunError, Message: huge}}, nil
	}})
	eng := newTestEngine(reg, nil, 4)
	bounded := func(what, s string) {
		t.Helper()
		if n := utf8.RuneCountInString(s); n > 1001 || n < 100 || !utf8.ValidString(s) {
			t.Fatalf("%s has %d runes (valid UTF-8: %v)", what, n, utf8.ValidString(s))
		}
	}
	label := strings.Repeat("L", 200)
	for _, typ := range []string{"test.bigerr", "test.bigpanic", "test.bigstop"} {
		b, tr, n := singleNodeFlow("Bounds", "big", typ, nil)
		f := b.build()
		f.NodeByID(n).Label = label
		res, capture := runWith(context.Background(), eng, f, RunRequest{TriggerNode: tr})
		if res.Status != RunError {
			t.Fatalf("%s: status = %s", typ, res.Status)
		}
		bounded(typ+" result", res.ErrorMessage)
		last := capture.events[len(capture.events)-1]
		bounded(typ+" summary", last.Run.ErrorMessage)
		if typ == "test.bigstop" {
			continue
		}
		bounded(typ+" step", mustStep(t, res, n).ErrorMessage)
		for _, ev := range capture.events {
			if ev.Type == EventStepFinished && ev.NodeID == n {
				bounded(typ+" step event", ev.Step.ErrorMessage)
			}
		}
		if prefix := strings.Repeat("L", 80) + "…: "; !strings.HasPrefix(res.ErrorMessage, prefix) {
			t.Fatalf("%s: the label must be cut to 80 runes, message starts %q", typ, truncateRunes(res.ErrorMessage, 100))
		}
	}
	if shared.Message != huge {
		t.Fatal("the node's own error value must not be modified")
	}

	// With on_error continue the error output that later nodes read is bounded too.
	b, tr, n := singleNodeFlow("Continue", "big", "test.bigerr", nil)
	f := b.build()
	f.NodeByID(n).Settings.OnError = ErrorContinue
	res, _ := runWith(context.Background(), eng, f, RunRequest{TriggerNode: tr})
	errOut, _ := res.Outputs["big"]["error"].(map[string]any)
	if res.Status != RunSuccess || errOut == nil {
		t.Fatalf("continue = %s %#v", res.Status, errOut)
	}
	bounded("error output", Stringify(errOut["message"]))
}

func TestEngineBoundsStoredParams(t *testing.T) {
	reg := newTestRegistry(t)
	blob := strings.Repeat("x", MaxStoredOutputBytes+10)
	reg.MustRegister(&NodeDef{Type: "test.blob", Execute: func(context.Context, ExecInput) (ExecResult, error) {
		return ExecResult{Output: map[string]any{"blob": blob}}, nil
	}})
	reg.MustRegister(&NodeDef{Type: "test.nan", Params: []ParamSpec{{Name: "n", Kind: ParamNumber, Default: math.NaN()}},
		Execute: func(context.Context, ExecInput) (ExecResult, error) {
			return ExecResult{Output: map[string]any{"ok": true}}, nil
		}})
	b, tr, src := singleNodeFlow("Params", "src", "test.blob", nil)
	big := b.node("big", "test.echo", map[string]any{"value": "{{src.blob}}"})
	small := b.node("small", "test.echo", map[string]any{"value": "hi"})
	nan := b.node("nan", "test.nan", nil)
	b.edge(src, PortOut, big)
	b.edge(tr, PortOut, small)
	b.edge(tr, PortOut, nan)
	res, _ := runWith(context.Background(), newTestEngine(reg, nil, 4), b.build(), RunRequest{TriggerNode: tr})
	if res.Status != RunSuccess {
		t.Fatalf("status = %s %s %s", res.Status, res.ErrorCode, res.ErrorMessage)
	}
	s := mustStep(t, res, big)
	preview, _ := s.Params["_preview"].(string)
	if !s.ParamsTruncated || len(s.Params) != 1 || preview == "" || len(preview) > storedPreviewBytes {
		t.Fatalf("big params: truncated=%v keys=%d preview=%d bytes", s.ParamsTruncated, len(s.Params), len(preview))
	}
	if got, _ := res.Outputs["big"]["value"].(string); got != blob {
		t.Fatalf("the node must receive the full parameters, got %d bytes", len(got))
	}
	if s := mustStep(t, res, small); s.ParamsTruncated || s.Params["value"] != "hi" {
		t.Fatalf("small params = %v truncated=%v", s.Params, s.ParamsTruncated)
	}
	if s := mustStep(t, res, nan); s.Status != StepSuccess || !s.ParamsTruncated || s.Params["_preview"] != "<unserializable>" {
		t.Fatalf("unserializable params = %s %v truncated=%v", s.Status, s.Params, s.ParamsTruncated)
	}
}

func TestEngineStoredPreviewIsValidUTF8(t *testing.T) {
	// Shifting a run of 3-byte runes moves the preview cut through every byte position of a rune.
	for shift := 0; shift < 3; shift++ {
		out := map[string]any{"blob": strings.Repeat("x", shift) + strings.Repeat("€", MaxStoredOutputBytes/3+10)}
		norm, data, err := normalizeOutput(out)
		if err != nil {
			t.Fatal(err)
		}
		stored, truncated := storedOutput(norm, data)
		preview, _ := stored["_preview"].(string)
		if !truncated || !utf8.ValidString(preview) || len(preview) > storedPreviewBytes || len(preview) < storedPreviewBytes-3 {
			t.Fatalf("shift %d: truncated=%v valid=%v len=%d", shift, truncated, utf8.ValidString(preview), len(preview))
		}
	}
	// The preview comes from the encoding it is given; the output is not encoded again.
	data := []byte(`{"blob":"` + strings.Repeat("y", MaxStoredOutputBytes) + `"}`)
	stored, _ := storedOutput(map[string]any{"other": true}, data)
	if preview, _ := stored["_preview"].(string); !strings.HasPrefix(string(data), preview) || len(preview) != storedPreviewBytes {
		t.Fatalf("preview = %d bytes, not taken from the given encoding", len(preview))
	}
}

// clockWaiterAt returns the deadline of the clock's first pending waiter.
func clockWaiterAt(c *fakeClock) time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.waiters[0].at
}

func TestEngineClampsRetryDelay(t *testing.T) {
	for _, delay := range []int{MaxRetryDelaySeconds + 1, math.MaxInt} {
		reg := newTestRegistry(t)
		registerFlaky(reg, 1)
		start := time.Date(2026, 10, 3, 7, 0, 0, 0, time.UTC)
		clock := newFakeClock(start)
		eng := newTestEngine(reg, &Services{Clock: clock, Location: time.UTC}, 4)
		b, tr, n := singleNodeFlow("Clamp", "flaky", "test.flaky", nil)
		f := b.build()
		// Set after Normalize, as for a flow that was never normalized.
		f.NodeByID(n).Settings.Retry = RetryPolicy{Count: 1, DelaySeconds: delay}
		done := make(chan RunResult, 1)
		go func() {
			res, _ := runWith(context.Background(), eng, f, RunRequest{TriggerNode: tr})
			done <- res
		}()
		clock.WaitForWaiters(t, 1)
		if at, want := clockWaiterAt(clock), start.Add(MaxRetryDelaySeconds*time.Second); !at.Equal(want) {
			t.Fatalf("delay %d: retry waits until %s, want %s", delay, at, want)
		}
		clock.Advance(MaxRetryDelaySeconds * time.Second)
		select {
		case res := <-done:
			if res.Status != RunSuccess || mustStep(t, res, n).Attempt != 2 {
				t.Fatalf("delay %d: result = %s attempt %d", delay, res.Status, mustStep(t, res, n).Attempt)
			}
		case <-time.After(10 * time.Second):
			t.Fatalf("delay %d: the retry did not happen after the clamped delay", delay)
		}
	}
}

func TestEngineClampsRunTimeout(t *testing.T) {
	reg := newTestRegistry(t)
	// The node's own timeout is far longer than any run, so its context ends with the run.
	reg.MustRegister(&NodeDef{Type: "test.deadline", DefaultTimeout: 1000 * time.Hour, Execute: func(ctx context.Context, _ ExecInput) (ExecResult, error) {
		deadline, ok := ctx.Deadline()
		if !ok {
			return ExecResult{}, NewNodeError("NO_DEADLINE", "the context has no deadline")
		}
		return ExecResult{Output: map[string]any{"left": time.Until(deadline).Seconds()}}, nil
	}})
	eng := newTestEngine(reg, nil, 4)
	for _, secs := range []int{MaxRunSecondsLimit * 2, math.MaxInt} {
		b, tr, _ := singleNodeFlow("RunClamp", "dl", "test.deadline", nil)
		f := b.build()
		f.Settings.MaxRunSeconds = secs // after Normalize
		res, _ := runWith(context.Background(), eng, f, RunRequest{TriggerNode: tr})
		left, _ := res.Outputs["dl"]["left"].(float64)
		if limit := float64(MaxRunSecondsLimit); res.Status != RunSuccess || left > limit || left < limit-3600 {
			t.Fatalf("max_run_seconds %d: %s, %.0f s left, want about %.0f", secs, res.Status, left, limit)
		}
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

func TestEngineDropsOversizedTriggerData(t *testing.T) {
	reg := newTestRegistry(t)
	var logs bytes.Buffer
	eng := NewEngine(reg, &Services{Location: time.UTC}, slog.New(slog.NewTextHandler(&logs, nil)), 4)
	b, tr, _ := singleNodeFlow("Huge", "echo", "test.echo", map[string]any{"value": "{{trigger.data | count}}"})
	res, _ := runWith(context.Background(), eng, b.build(), RunRequest{TriggerNode: tr,
		TriggerData: map[string]any{"blob": strings.Repeat("x", MaxOutputBytes)}})
	if res.Status != RunSuccess || res.Outputs["echo"]["value"] != 0.0 {
		t.Fatalf("result = %s %#v", res.Status, res.Outputs["echo"])
	}
	if !strings.Contains(logs.String(), "trigger data") {
		t.Fatalf("dropping the trigger data must be logged, log = %q", logs.String())
	}
}

func TestEngineBoundsTriggerStepOutput(t *testing.T) {
	reg := newTestRegistry(t)
	eng := newTestEngine(reg, nil, 4)
	b, tr, _ := singleNodeFlow("BigTrigger", "x", "test.echo", map[string]any{"value": "{{trigger.data.body | count}}"})
	f := b.build()

	body := strings.Repeat("x", 4<<20)
	res, capture := runWith(context.Background(), eng, f, RunRequest{TriggerNode: tr, TriggerData: map[string]any{"body": body}})
	if res.Status != RunSuccess || res.Outputs["x"]["value"] != float64(len(body)) {
		t.Fatalf("templates must see the whole trigger output: %s %#v", res.Status, res.Outputs["x"])
	}
	s := mustStep(t, res, tr)
	preview, _ := s.Output["_preview"].(string)
	if !s.OutputTruncated || len(s.Output) != 1 || preview == "" || len(preview) > storedPreviewBytes {
		t.Fatalf("trigger step: truncated=%v keys=%d preview=%d bytes", s.OutputTruncated, len(s.Output), len(preview))
	}
	for _, ev := range capture.events {
		if ev.NodeID == tr && ev.Step != nil {
			if data, _ := json.Marshal(ev); len(data) > MaxStoredOutputBytes {
				t.Fatalf("trigger event has %d bytes", len(data))
			}
		}
	}

	// Oversized data is replaced with {}; the step then holds that small output whole.
	res, _ = runWith(context.Background(), eng, f, RunRequest{TriggerNode: tr, TriggerData: map[string]any{"body": strings.Repeat("x", MaxOutputBytes)}})
	s = mustStep(t, res, tr)
	if data, _ := s.Output["data"].(map[string]any); res.Status != RunSuccess || s.OutputTruncated || data == nil || len(data) != 0 {
		t.Fatalf("replaced trigger data: %s truncated=%v output=%v", res.Status, s.OutputTruncated, s.Output)
	}
}

func TestEngineRunOutputBudget(t *testing.T) {
	reg := newTestRegistry(t)
	blob := strings.Repeat("x", 3<<20)
	reg.MustRegister(&NodeDef{Type: "test.three", Execute: func(context.Context, ExecInput) (ExecResult, error) {
		return ExecResult{Output: map[string]any{"blob": blob}}, nil
	}})
	build := func() (*flowBuilder, string, []string) {
		b := newFlow("Budget")
		tr := b.node("start", "test.trigger", nil)
		chain := make([]string, 12)
		prev := tr
		for i := range chain {
			chain[i] = b.node(fmt.Sprintf("c%d", i), "test.three", nil)
			b.edge(prev, PortOut, chain[i])
			prev = chain[i]
		}
		return b, tr, chain
	}
	eng := newTestEngine(reg, nil, 4)

	// Ten outputs of 3 MiB fit into the 32 MiB budget; the eleventh does not.
	b, tr, chain := build()
	res, _ := runWith(context.Background(), eng, b.build(), RunRequest{TriggerNode: tr})
	want := fmt.Sprintf("the run's outputs exceed %d bytes in total", MaxRunOutputBytes)
	if res.Status != RunError || res.ErrorCode != "FLOW_RUN_OUTPUT_TOO_LARGE" || res.ErrorNodeID != chain[10] || !strings.HasSuffix(res.ErrorMessage, want) {
		t.Fatalf("result = %s %s %s %q", res.Status, res.ErrorCode, res.ErrorNodeID, res.ErrorMessage)
	}
	for _, id := range chain[:10] {
		if s := mustStep(t, res, id); s.Status != StepSuccess {
			t.Fatalf("node %s = %s", id, s.Status)
		}
	}
	if s := mustStep(t, res, chain[10]); s.Status != StepError || s.Output != nil {
		t.Fatalf("over budget step = %s output=%v", s.Status, s.Output != nil)
	}
	if _, kept := res.Outputs["c10"]; kept || mustStep(t, res, chain[11]).Status != StepSkipped {
		t.Fatal("the rejected output must not be kept, and the next node must be skipped")
	}

	// The node's on_error setting applies as for any other failure.
	b, tr, chain = build()
	handler := b.node("handler", "test.echo", map[string]any{"value": "{{c10.error.code}}"})
	b.edge(chain[10], PortError, handler)
	f := b.build()
	f.NodeByID(chain[10]).Settings.OnError = ErrorPort
	res, _ = runWith(context.Background(), eng, f, RunRequest{TriggerNode: tr})
	if res.Status != RunSuccess || res.Outputs["handler"]["value"] != "FLOW_RUN_OUTPUT_TOO_LARGE" || mustStep(t, res, chain[11]).Status != StepSkipped {
		t.Fatalf("error port = %s %#v", res.Status, res.Outputs["handler"])
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

func TestEngineAbandonsStuckNodes(t *testing.T) {
	reg := newTestRegistry(t)
	release := make(chan struct{})
	defer close(release) // lets the abandoned worker finish; its late result is never read
	reg.MustRegister(&NodeDef{Type: "test.deaf", Execute: func(context.Context, ExecInput) (ExecResult, error) {
		<-release // ignores its context
		return ExecResult{}, nil
	}})
	b, tr, n := singleNodeFlow("Deaf", "deaf", "test.deaf", nil)
	after := b.node("after", "test.echo", nil)
	b.edge(n, PortOut, after)
	f := b.build()
	var logs bytes.Buffer
	eng := NewEngine(reg, &Services{Location: time.UTC}, slog.New(slog.NewTextHandler(&logs, nil)), 4)
	eng.abandonAfter = 100 * time.Millisecond
	type outcome struct {
		res     RunResult
		capture *runCapture
	}
	done := make(chan outcome, 1)
	go func() {
		res, capture := runWith(context.Background(), eng, f, RunRequest{TriggerNode: tr, Timeout: 50 * time.Millisecond})
		done <- outcome{res, capture}
	}()
	var o outcome
	select {
	case o = <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("the run did not give up on a node that ignores its context")
	}
	res := o.res
	if res.Status != RunError || res.ErrorCode != "FLOW_RUN_TIMEOUT" {
		t.Fatalf("result = %s %s", res.Status, res.ErrorCode)
	}
	if s := mustStep(t, res, n); s.Status != StepCancelled || s.ErrorCode != "FLOW_NODE_ABANDONED" || s.ErrorMessage != "the node did not stop after the run ended" {
		t.Fatalf("deaf step = %+v", s)
	}
	if s := mustStep(t, res, after); s.Status != StepCancelled {
		t.Fatalf("after = %s, want cancelled", s.Status)
	}
	if types := o.capture.types(); types[len(types)-1] != EventRunFinished {
		t.Fatalf("events = %v", types)
	}
	if !strings.Contains(logs.String(), "abandoned") {
		t.Fatalf("abandoning must be logged, log = %q", logs.String())
	}
}

func TestEngineGoexitNodeDoesNotHang(t *testing.T) {
	reg := newTestRegistry(t)
	reg.MustRegister(&NodeDef{Type: "test.goexit", Execute: func(context.Context, ExecInput) (ExecResult, error) {
		runtime.Goexit()
		return ExecResult{}, nil
	}})
	b, tr, n := singleNodeFlow("Goexit", "g", "test.goexit", nil)
	done := make(chan RunResult, 1)
	go func() {
		res, _ := runWith(context.Background(), newTestEngine(reg, nil, 4), b.build(), RunRequest{TriggerNode: tr})
		done <- res
	}()
	select {
	case res := <-done:
		if s := mustStep(t, res, n); res.Status != RunError || res.ErrorCode != "FLOW_NODE_PANIC" || res.ErrorNodeID != n || s.Status != StepError ||
			!strings.Contains(res.ErrorMessage, "exited without a result") {
			t.Fatalf("result = %s %s %s %q", res.Status, res.ErrorCode, res.ErrorNodeID, res.ErrorMessage)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("a node that exits its goroutine must not hang the run")
	}
}
