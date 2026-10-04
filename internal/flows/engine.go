package flows

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"
	"unicode/utf8"
)

// Output limits and engine defaults.
const (
	MaxOutputBytes          = 5 << 20
	MaxStoredOutputBytes    = 256 << 10
	storedPreviewBytes      = 64 << 10
	DefaultMaxParallelNodes = 4
	// maxErrorMessageRunes caps every error message the engine records (steps,
	// error outputs, run results and summaries): node errors can carry
	// arbitrarily large tool output. Cut messages end with an ellipsis.
	maxErrorMessageRunes = 1000
	// maxErrorLabelRunes caps the node label that prefixes a run's error message.
	maxErrorLabelRunes = 80
)

// RunRequest describes one run for the engine.
type RunRequest struct {
	RunID       string
	Flow        *Flow
	Revision    int
	Mode        RunMode
	TriggerNode string
	TriggerType string
	// TriggerData becomes trigger.data. Data that cannot be encoded as JSON or
	// whose trigger output would exceed MaxOutputBytes is replaced with {} (and
	// a warning is logged), so the run still starts.
	TriggerData map[string]any
	// OnlyNode limits a test run to this node and its ancestors.
	OnlyNode string
	// Timeout overrides the flow's max_run_seconds when > 0.
	Timeout time.Duration
}

// EventSink receives run events in order, always from the run's coordinator goroutine.
type EventSink func(RunEvent)

// Engine executes flow runs.
type Engine struct {
	reg      *Registry
	services *Services
	logger   *slog.Logger
	parallel int
}

// NewEngine returns an engine that runs at most parallel nodes of one run at a time.
// A node holds its slot until it returns, including while it waits: a
// logic.wait node can hold one for up to an hour, so with the default of 4
// parallel nodes a few waiting branches block the other branches of the run.
func NewEngine(reg *Registry, services *Services, logger *slog.Logger, parallel int) *Engine {
	if parallel <= 0 {
		parallel = DefaultMaxParallelNodes
	}
	if logger == nil {
		logger = slog.Default()
	}
	if services == nil {
		services = &Services{}
	}
	return &Engine{reg: reg, services: services, logger: logger, parallel: parallel}
}

// Registry returns the engine's node registry.
func (e *Engine) Registry() *Registry { return e.reg }

// Services returns the engine's services.
func (e *Engine) Services() *Services { return e.services }

// Execute runs req to completion. Node failures and panics become run results, never panics.
//
// A missing flow (FLOW_INVALID), one over MaxNodes or MaxEdges (FLOW_TOO_LARGE)
// and one with a loop (FLOW_CYCLE) fail the run before any node runs; test runs
// execute unpublished drafts, so these checks are their only guard.
//
// Known limits of node execution:
//   - A node's parameters are resolved on its worker before its timeout starts,
//     and the resolution cannot be cancelled. Expensive templates (for example
//     100 filter chains over a 5 MiB value) can take about a second.
//   - on_error "continue" routes a failed node into its default port, which is
//     "true" for logic.if and "case_1" for logic.switch, with
//     {"error": {"code": ..., "message": ...}} as its output.
func (e *Engine) Execute(ctx context.Context, req RunRequest, emit EventSink) RunResult {
	if emit == nil {
		emit = func(RunEvent) {}
	}
	return newRunState(e, req, emit).run(ctx)
}

type nodeDone struct {
	nodeID    string
	step      StepRecord
	result    ExecResult
	err       *NodeError
	cancelled bool
}

func withDefaults(def *NodeDef, params map[string]any) map[string]any {
	out := make(map[string]any, len(params)+len(def.Params))
	for k, v := range params {
		out[k] = v
	}
	for _, spec := range def.Params {
		if _, ok := out[spec.Name]; !ok && spec.Default != nil {
			out[spec.Name] = spec.Default
		}
	}
	return out
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// executeNode runs one node with retries and timeouts. It runs on a worker
// goroutine and must not touch the run state.
func (e *Engine) executeNode(ctx context.Context, def *NodeDef, n *Node, in ExecInput) nodeDone {
	d := nodeDone{nodeID: n.ID}
	step := StepRecord{NodeID: n.ID, NodeKey: n.Key, Attempt: 1, StartedAt: e.services.Now()}
	finish := func(status StepStatus) nodeDone {
		step.Status = status
		step.FinishedAt = e.services.Now()
		step.DurationMS = step.FinishedAt.Sub(step.StartedAt).Milliseconds()
		d.step = step
		return d
	}
	fail := func(ne *NodeError) nodeDone {
		// A copy, not a cut in place: the node may return a shared error value.
		ne = &NodeError{Code: ne.Code, Message: truncateRunes(ne.Message, maxErrorMessageRunes)}
		d.err = ne
		step.ErrorCode, step.ErrorMessage = ne.Code, ne.Message
		return finish(StepError)
	}
	if def == nil || def.Execute == nil {
		return fail(&NodeError{Code: "NODE_TYPE_UNKNOWN", Message: "node type " + quoteForError(n.Type) + " is not available"})
	}
	params, err := ResolveParams(withDefaults(def, n.Params), in.Env)
	if err != nil {
		return fail(&NodeError{Code: "FLOW_TEMPLATE_ERROR", Message: err.Error()})
	}
	in.Params = params
	step.Params, step.ParamsTruncated = storedParams(params)
	attempts := 1 + clampInt(n.Settings.Retry.Count, 0, MaxRetryCount)
	// Clamp before multiplying: a huge value would overflow the duration.
	delay := time.Duration(clampInt(n.Settings.Retry.DelaySeconds, 0, MaxRetryDelaySeconds)) * time.Second
	timeout := def.Timeout(n)
	var res ExecResult
	var runErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		step.Attempt = attempt
		if attempt > 1 && delay > 0 {
			if err := e.services.Sleep(ctx, delay); err != nil {
				runErr = err
				break
			}
		}
		nodeCtx, cancel := context.WithTimeout(ctx, timeout)
		res, runErr = safeExecute(nodeCtx, def.Execute, in)
		timedOut := errors.Is(nodeCtx.Err(), context.DeadlineExceeded) && ctx.Err() == nil
		cancel()
		if runErr == nil || ctx.Err() != nil {
			break
		}
		if timedOut {
			runErr = &NodeError{Code: "FLOW_NODE_TIMEOUT", Message: fmt.Sprintf("the node did not finish within %s", timeout)}
		}
	}
	if runErr != nil {
		if ctx.Err() != nil {
			d.cancelled = true
			step.ErrorCode, step.ErrorMessage = "FLOW_CANCELLED", "the run was stopped"
			return finish(StepCancelled)
		}
		return fail(asNodeError(runErr))
	}
	output, size, err := normalizeOutput(res.Output)
	if err != nil {
		return fail(&NodeError{Code: "FLOW_OUTPUT_INVALID", Message: err.Error()})
	}
	if size > MaxOutputBytes {
		return fail(&NodeError{Code: "FLOW_OUTPUT_TOO_LARGE", Message: fmt.Sprintf("the node produced %d bytes; the limit is %d", size, MaxOutputBytes)})
	}
	res.Output = output
	d.result = res
	step.Output, step.OutputTruncated = storedOutput(output, size)
	step.ItemCount = res.ItemCount
	if step.ItemCount == 0 {
		if items, ok := output["items"].([]any); ok {
			step.ItemCount = len(items)
		}
	}
	return finish(StepSuccess)
}

// runNode is the whole body of a node's worker goroutine. It turns a panic
// anywhere in executeNode (parameter resolution, output encoding through a
// custom MarshalJSON, ...) into a FLOW_NODE_PANIC failure, so the process never
// crashes and the coordinator always receives a result.
func (e *Engine) runNode(ctx context.Context, def *NodeDef, n *Node, in ExecInput) (d nodeDone) {
	started := e.services.Now()
	defer func() {
		if r := recover(); r != nil {
			ne := panicError(r)
			end := e.services.Now()
			d = nodeDone{nodeID: n.ID, err: ne, step: StepRecord{NodeID: n.ID, NodeKey: n.Key, Attempt: 1,
				Status: StepError, StartedAt: started, FinishedAt: end, DurationMS: end.Sub(started).Milliseconds(),
				ErrorCode: ne.Code, ErrorMessage: ne.Message}}
		}
	}()
	return e.executeNode(ctx, def, n, in)
}

func safeExecute(ctx context.Context, fn ExecuteFunc, in ExecInput) (res ExecResult, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = panicError(r)
		}
	}()
	return fn(ctx, in)
}

// panicError reports a recovered panic with a bounded message.
func panicError(r any) *NodeError {
	return &NodeError{Code: "FLOW_NODE_PANIC", Message: truncateRunes(fmt.Sprintf("the node crashed: %v", r), maxErrorMessageRunes)}
}

// normalizeOutput turns the output into plain JSON values and reports its encoded size.
// Oversized outputs are returned as nil with their size.
func normalizeOutput(out map[string]any) (map[string]any, int, error) {
	if out == nil {
		return map[string]any{}, 2, nil
	}
	data, err := json.Marshal(out)
	if err != nil {
		return nil, 0, err
	}
	if len(data) > MaxOutputBytes {
		return nil, len(data), nil
	}
	var norm map[string]any
	if err := json.Unmarshal(data, &norm); err != nil {
		return nil, 0, err
	}
	if norm == nil {
		norm = map[string]any{}
	}
	return norm, len(data), nil
}

// storedOutput returns what the run log keeps: the output, or a preview when it is large.
func storedOutput(out map[string]any, size int) (map[string]any, bool) {
	if size <= MaxStoredOutputBytes {
		return out, false
	}
	data, _ := json.Marshal(out)
	return previewOf(data), true
}

// storedParams returns what the run log keeps of a node's resolved parameters,
// bounded like outputs. The parameters can share memory with the run's data
// (see ExecInput), so they are stored as they are and never modified.
// Parameters that cannot be encoded get a small placeholder; that does not fail
// the node.
func storedParams(params map[string]any) (map[string]any, bool) {
	data, err := json.Marshal(params)
	if err != nil {
		return map[string]any{"_preview": "<unserializable>"}, true
	}
	if len(data) <= MaxStoredOutputBytes {
		return params, false
	}
	return previewOf(data), true
}

// previewOf returns the stored preview of encoded JSON: its first
// storedPreviewBytes bytes, cut back to a rune boundary so the text stays valid
// UTF-8 (json.Marshal output is valid UTF-8).
func previewOf(data []byte) map[string]any {
	if len(data) > storedPreviewBytes {
		cut := storedPreviewBytes
		for cut > 0 && !utf8.RuneStart(data[cut]) {
			cut--
		}
		data = data[:cut]
	}
	return map[string]any{"_preview": string(data)}
}
