package flows

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"
)

// retryProbe registers a node that fails with err on every call and counts its calls.
func retryProbe(reg *Registry, err error) *int32 {
	var calls int32
	reg.MustRegister(&NodeDef{Type: "test.retryprobe", Execute: func(context.Context, ExecInput) (ExecResult, error) {
		atomic.AddInt32(&calls, 1)
		return ExecResult{}, err
	}})
	return &calls
}

// runRetryProbe runs a single node that fails with err under Retry 3 and returns the
// number of executions and the run result.
func runRetryProbe(t *testing.T, err error, delaySeconds int) (int32, RunResult) {
	t.Helper()
	reg := newTestRegistry(t)
	calls := retryProbe(reg, err)
	b, tr, n := singleNodeFlow("RetryCodes", "probe", "test.retryprobe", nil)
	f := b.build()
	f.NodeByID(n).Settings.Retry = RetryPolicy{Count: 3, DelaySeconds: delaySeconds}
	// A fake clock that nobody advances: a retry delay that is wrongly slept would hang the run.
	clock := newFakeClock(time.Date(2026, 10, 3, 7, 0, 0, 0, time.UTC))
	eng := newTestEngine(reg, &Services{Clock: clock, Location: time.UTC}, 4)
	done := make(chan RunResult, 1)
	go func() {
		res, _ := runWith(context.Background(), eng, f, RunRequest{TriggerNode: tr})
		done <- res
	}()
	select {
	case res := <-done:
		return atomic.LoadInt32(calls), res
	case <-time.After(10 * time.Second):
		t.Fatalf("the run did not finish: a retry delay was probably slept for a hopeless error")
		return 0, RunResult{}
	}
}

// Retrying only helps when a later attempt can behave differently. An error about the
// node's own parameters, its permissions or its configuration, or one that says what it
// received is too big, fails the same way every time: retrying it would only pay for
// the same refusal again (an ai.step under Retry 5 could otherwise make 12 model calls).
func TestEngineDoesNotRetryHopelessErrors(t *testing.T) {
	hopeless := []string{
		"FLOW_PARAM_INVALID", "FLOW_BUDGET_EXCEEDED", "FLOW_TOOL_DENIED", "FLOW_NODE_UNAVAILABLE",
		"FLOW_TOOLS_UNAVAILABLE", "FLOW_AI_UNAVAILABLE", "FLOW_OUTPUT_TOO_LARGE", "FLOW_CONDITION_INVALID",
	}
	for _, code := range hopeless {
		t.Run(code, func(t *testing.T) {
			calls, res := runRetryProbe(t, NewNodeError(code, "no use"), 0)
			if calls != 1 {
				t.Fatalf("executed %d times, want 1", calls)
			}
			if res.Status != RunError || res.ErrorCode != code || res.ErrorMessage == "" {
				t.Fatalf("result = %s %s %q", res.Status, res.ErrorCode, res.ErrorMessage)
			}
			if s := stepOf(res, res.ErrorNodeID); s == nil || s.Attempt != 1 || s.Status != StepError || s.ErrorCode != code {
				t.Fatalf("step = %+v", s)
			}
		})
	}

	// Wrapped, and with a retry delay that must not be waited for.
	calls, res := runRetryProbe(t, fmt.Errorf("provider call: %w", NewNodeError("FLOW_PARAM_INVALID", "bad")), 30)
	if calls != 1 || res.ErrorCode != "FLOW_PARAM_INVALID" {
		t.Fatalf("wrapped error: %d executions, code %q", calls, res.ErrorCode)
	}
}

// The errors that can go away on their own are retried as before, Retry + 1 executions in all.
func TestEngineStillRetriesTransientErrors(t *testing.T) {
	cases := map[string]error{
		"FLOW_NODE_FAILED":       errors.New("provider down"),
		"FLOW_NODE_FAILED coded": NewNodeError("FLOW_NODE_FAILED", "again"),
		"FLOW_AI_OUTPUT_INVALID": NewNodeError("FLOW_AI_OUTPUT_INVALID", "not json"),
		"FLOW_TOOL_ERROR":        NewNodeError("FLOW_TOOL_ERROR", "tool failed"),
		"FLOW_NODE_TIMEOUT":      NewNodeError("FLOW_NODE_TIMEOUT", "slow"),
		"FLOW_NODE_PANIC":        NewNodeError("FLOW_NODE_PANIC", "crashed"),
		"empty code":             &NodeError{Message: "x"},
		"typed nil":              (*NodeError)(nil),
		"unknown code":           NewNodeError("SOMETHING_ELSE", "x"),
	}
	for name, err := range cases {
		t.Run(name, func(t *testing.T) {
			calls, res := runRetryProbe(t, err, 0)
			if calls != 4 {
				t.Fatalf("executed %d times, want 4", calls)
			}
			if res.Status != RunError {
				t.Fatalf("status = %s", res.Status)
			}
			if s := stepOf(res, res.ErrorNodeID); s == nil || s.Attempt != 4 {
				t.Fatalf("step = %+v", s)
			}
		})
	}
}

// The set is a contract that the node guide documents; changing it is a decision.
func TestNonRetryableCodesAreTheDocumentedSet(t *testing.T) {
	want := map[string]bool{
		"FLOW_PARAM_INVALID": true, "FLOW_BUDGET_EXCEEDED": true, "FLOW_TOOL_DENIED": true,
		"FLOW_NODE_UNAVAILABLE": true, "FLOW_TOOLS_UNAVAILABLE": true, "FLOW_AI_UNAVAILABLE": true,
		"FLOW_OUTPUT_TOO_LARGE": true, "FLOW_CONDITION_INVALID": true,
	}
	if len(nonRetryableCodes) != len(want) {
		t.Fatalf("nonRetryableCodes = %v", nonRetryableCodes)
	}
	for code := range want {
		if !nonRetryableCodes[code] {
			t.Errorf("%s is missing from nonRetryableCodes", code)
		}
	}
	for _, code := range []string{"FLOW_AI_OUTPUT_INVALID", "FLOW_TOOL_ERROR", "FLOW_NODE_FAILED", "FLOW_NODE_TIMEOUT", ""} {
		if nonRetryableCodes[code] {
			t.Errorf("%q must stay retryable", code)
		}
	}
}
