package flows

import (
	"context"
	"testing"
)

// sharedEmptyCodeError is returned by every run of test.emptycode, so the test can
// check that the engine copies it instead of filling in the code in place.
var sharedEmptyCodeError = &NodeError{Message: "x"}

func TestEngineDefaultsEmptyNodeErrorCode(t *testing.T) {
	reg := newTestRegistry(t)
	reg.MustRegister(&NodeDef{Type: "test.emptycode", Execute: func(context.Context, ExecInput) (ExecResult, error) {
		return ExecResult{}, sharedEmptyCodeError
	}})
	eng := newTestEngine(reg, nil, 4)

	b, tr, n := singleNodeFlow("EmptyCode", "bad", "test.emptycode", nil)
	res, _ := runWith(context.Background(), eng, b.build(), RunRequest{TriggerNode: tr})
	if res.Status != RunError || res.ErrorCode != "FLOW_NODE_FAILED" || res.ErrorNodeID != n {
		t.Fatalf("result = %s %q node %s", res.Status, res.ErrorCode, res.ErrorNodeID)
	}
	if s := mustStep(t, res, n); s.Status != StepError || s.ErrorCode != "FLOW_NODE_FAILED" || s.ErrorMessage != "x" {
		t.Fatalf("step = %+v", s)
	}
	if sharedEmptyCodeError.Code != "" {
		t.Fatalf("the node's own error value was modified: %q", sharedEmptyCodeError.Code)
	}

	// The code a downstream node reads from an on_error=continue delivery is the default too.
	b, tr, n = singleNodeFlow("EmptyCodeContinue", "bad", "test.emptycode", nil)
	after := b.node("after", "test.echo", map[string]any{"value": "{{bad.error.code}}"})
	b.edge(n, PortOut, after)
	f := b.build()
	f.NodeByID(n).Settings.OnError = ErrorContinue
	res, _ = runWith(context.Background(), eng, f, RunRequest{TriggerNode: tr})
	if res.Status != RunSuccess || res.Outputs["after"]["value"] != "FLOW_NODE_FAILED" {
		t.Fatalf("continue = %s %#v", res.Status, res.Outputs)
	}
}
