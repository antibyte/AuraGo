package flows

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func TestEngineRejectsMissingAndOversizedFlows(t *testing.T) {
	reg := newTestRegistry(t)
	eng := newTestEngine(reg, nil, 4)
	runOnly := []string{EventRunStarted, EventRunFinished}
	check := func(name string, f *Flow, trigger, wantCode string) {
		t.Helper()
		res, capture := runWith(context.Background(), eng, f, RunRequest{TriggerNode: trigger})
		if res.Status != RunError || res.ErrorCode != wantCode || len(res.Steps) != 0 {
			t.Fatalf("%s = %s %s, %d steps", name, res.Status, res.ErrorCode, len(res.Steps))
		}
		if !reflect.DeepEqual(capture.types(), runOnly) || capture.events[1].Run.ErrorCode != wantCode {
			t.Fatalf("%s events = %v", name, capture.types())
		}
	}
	check("nil flow", nil, testNodeID(1), "FLOW_INVALID")

	b := newFlow("Nodes")
	tr := b.node("start", "test.trigger", nil)
	for i := 1; i <= MaxNodes; i++ {
		b.node(fmt.Sprintf("n%d", i), "test.echo", nil)
	}
	check("too many nodes", b.build(), tr, "FLOW_TOO_LARGE")

	b = newFlow("Edges")
	tr = b.node("start", "test.trigger", nil)
	a := b.node("a", "test.echo", nil)
	for i := 0; i <= MaxEdges; i++ {
		b.edge(tr, PortOut, a)
	}
	check("too many edges", b.build(), tr, "FLOW_TOO_LARGE")
}

func TestEngineDefinitionHookPanicFailsRun(t *testing.T) {
	reg := newTestRegistry(t)
	reg.MustRegister(&NodeDef{Type: "test.badports", OutputsFunc: func(*Node) []string { panic("ports kaputt") },
		Execute: func(context.Context, ExecInput) (ExecResult, error) { return ExecResult{}, nil }})
	reg.MustRegister(&NodeDef{Type: "test.badtrigger", Trigger: true, OutputsFunc: func(*Node) []string { panic("trigger ports kaputt") }})
	eng := newTestEngine(reg, nil, 4)

	b, tr, bad := singleNodeFlow("BadPorts", "bad", "test.badports", nil)
	res, capture := runWith(context.Background(), eng, b.build(), RunRequest{TriggerNode: tr})
	if res.Status != RunError || res.ErrorCode != "FLOW_NODE_PANIC" || res.ErrorNodeID != bad || len(res.Steps) != 0 {
		t.Fatalf("result = %s %s %s, %d steps", res.Status, res.ErrorCode, res.ErrorNodeID, len(res.Steps))
	}
	if !strings.Contains(res.ErrorMessage, `"bad"`) || !strings.Contains(res.ErrorMessage, "ports kaputt") {
		t.Fatalf("message = %q", res.ErrorMessage)
	}
	if !reflect.DeepEqual(capture.types(), []string{EventRunStarted, EventRunFinished}) {
		t.Fatalf("events = %v", capture.types())
	}

	b = newFlow("BadTrigger")
	tr = b.node("start", "test.badtrigger", nil)
	if res, _ := runWith(context.Background(), eng, b.build(), RunRequest{TriggerNode: tr}); res.ErrorCode != "FLOW_NODE_PANIC" || res.ErrorNodeID != tr {
		t.Fatalf("trigger = %s %s", res.ErrorCode, res.ErrorNodeID)
	}
}

// TestEngineKeepsDefinitionsForTheRun checks that a definition replaced during
// a run does not affect that run: each node keeps the definition it started with.
func TestEngineKeepsDefinitionsForTheRun(t *testing.T) {
	reg := newTestRegistry(t)
	version := func(v string) *NodeDef {
		return &NodeDef{Type: "test.version", Execute: func(context.Context, ExecInput) (ExecResult, error) {
			return ExecResult{Output: map[string]any{"v": v}}, nil
		}}
	}
	reg.MustRegister(version("old"))
	reg.MustRegister(&NodeDef{Type: "test.replace", Execute: func(context.Context, ExecInput) (ExecResult, error) {
		reg.Replace(version("new"))
		return ExecResult{}, nil
	}})
	b, tr, swap := singleNodeFlow("Swap", "swap", "test.replace", nil)
	v := b.node("v", "test.version", nil)
	b.edge(swap, PortOut, v)
	res, _ := runWith(context.Background(), newTestEngine(reg, nil, 4), b.build(), RunRequest{TriggerNode: tr})
	if res.Status != RunSuccess || res.Outputs["v"]["v"] != "old" {
		t.Fatalf("result = %s %#v", res.Status, res.Outputs["v"])
	}
}

func TestEngineNewEngineNeedsRegistry(t *testing.T) {
	var r any
	func() {
		defer func() { r = recover() }()
		NewEngine(nil, nil, nil, 0)
	}()
	if r != "flows: NewEngine needs a registry" {
		t.Fatalf("panic = %v", r)
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

func TestEngineAbandonsStuckNodes(t *testing.T) {
	reg := newTestRegistry(t)
	release := make(chan struct{})
	defer close(release) // lets the abandoned worker finish; its late result is never read
	started := make(chan struct{})
	reg.MustRegister(&NodeDef{Type: "test.deaf", Execute: func(context.Context, ExecInput) (ExecResult, error) {
		close(started)
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
	// The run ends only once the deaf node runs: a run timeout could expire
	// before the node starts (the first local-time format in a process is slow
	// on Windows), and then the node would just be closed, not abandoned.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		select {
		case <-started:
			cancel()
		case <-ctx.Done():
		}
	}()
	type outcome struct {
		res     RunResult
		capture *runCapture
	}
	done := make(chan outcome, 1)
	go func() {
		res, capture := runWith(ctx, eng, f, RunRequest{TriggerNode: tr})
		done <- outcome{res, capture}
	}()
	var o outcome
	select {
	case o = <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("the run did not give up on a node that ignores its context")
	}
	res := o.res
	if res.Status != RunCancelled || res.ErrorCode != "FLOW_CANCELLED" {
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

// RunRequest.Timeout overrides max_run_seconds but is held to MaxRunSecondsLimit as well, so
// no run outlives the horizon after which the event bus treats an open log as leaked.
func TestEngineClampsRequestTimeout(t *testing.T) {
	reg := newTestRegistry(t)
	reg.MustRegister(&NodeDef{Type: "test.deadline", DefaultTimeout: 1000 * time.Hour, Execute: func(ctx context.Context, _ ExecInput) (ExecResult, error) {
		deadline, ok := ctx.Deadline()
		if !ok {
			return ExecResult{}, NewNodeError("NO_DEADLINE", "the context has no deadline")
		}
		return ExecResult{Output: map[string]any{"left": time.Until(deadline).Seconds()}}, nil
	}})
	eng := newTestEngine(reg, nil, 4)
	limit := time.Duration(MaxRunSecondsLimit) * time.Second
	cases := []struct {
		name    string
		timeout time.Duration
		want    time.Duration // the run's time budget
	}{
		{"above the limit", 2 * limit, limit},
		{"just above the limit", limit + time.Second, limit},
		{"huge", time.Duration(math.MaxInt64), limit},
		{"at the limit", limit, limit},
		{"below the limit", 2 * time.Hour, 2 * time.Hour},
	}
	for _, tc := range cases {
		b, tr, _ := singleNodeFlow("ReqClamp", "dl", "test.deadline", nil)
		res, _ := runWith(context.Background(), eng, b.build(), RunRequest{TriggerNode: tr, Timeout: tc.timeout})
		left, _ := res.Outputs["dl"]["left"].(float64)
		if want := tc.want.Seconds(); res.Status != RunSuccess || left > want || left < want-3600 {
			t.Fatalf("%s: %s, %.0f s left, want about %.0f", tc.name, res.Status, left, want)
		}
	}
}
