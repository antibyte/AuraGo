package flows

import (
	"context"
	"testing"
)

// on_error "continue" hands the failure ({error}) to the steps behind a node as data. On a
// node that branches it must not pick a branch that did not earn it: a failed logic.if is
// not true and a failed logic.switch is no case, so both continue on the fail-closed port
// (false / default) and the steps behind true / case_1 are skipped. Every other node
// continues on its first output, as before.
func TestContinueOnErrorUsesTheFailClosedPort(t *testing.T) {
	reg := newTestRegistry(t)
	// "before" needs two dates, so this condition fails (FLOW_CONDITION_FAILED) instead of being false.
	broken := cond("x", "before", "y")

	type outcome struct{ ran, skipped []string }
	run := func(typ string, params map[string]any, ports ...string) outcome {
		t.Helper()
		b, tr, branch := singleNodeFlow("Continue port", "branch", typ, params)
		targets := map[string]string{}
		for _, port := range ports {
			targets[port] = b.node("after_"+port, "test.echo", map[string]any{"value": "{{branch.error.code}}"})
			b.edge(branch, port, targets[port])
		}
		f := b.build()
		f.NodeByID(branch).Settings.OnError = ErrorContinue
		res, _ := runWith(context.Background(), newTestEngine(reg, nil, 4), f, RunRequest{TriggerNode: tr})
		if res.Status != RunSuccess || stepOf(res, branch).Status != StepError {
			t.Fatalf("%s: run = %s %s %s, branch step = %+v", typ, res.Status, res.ErrorCode, res.ErrorMessage, stepOf(res, branch))
		}
		var out outcome
		for _, port := range ports {
			s := stepOf(res, targets[port])
			switch {
			case s == nil:
				t.Fatalf("%s: no step for the node behind %s", typ, port)
			case s.Status == StepSuccess:
				out.ran = append(out.ran, port)
			case s.Status == StepSkipped:
				out.skipped = append(out.skipped, port)
			default:
				t.Fatalf("%s: node behind %s ended %s", typ, port, s.Status)
			}
		}
		return out
	}
	same := func(what string, got, want []string) {
		t.Helper()
		if len(got) != len(want) || (len(want) == 1 && got[0] != want[0]) {
			t.Errorf("%s = %v, want %v", what, got, want)
		}
	}

	got := run(TypeIf, map[string]any{"condition": broken}, PortTrue, PortFalse)
	same("if: continued on", got.ran, []string{PortFalse})
	same("if: skipped", got.skipped, []string{PortTrue})

	cases := []any{map[string]any{"condition": broken}, map[string]any{"condition": cond(1.0, "eq", 1.0)}}
	got = run(TypeSwitch, map[string]any{"cases": cases}, "case_1", "case_2", PortDefault)
	same("switch: continued on", got.ran, []string{PortDefault})
	same("switch: skipped", got.skipped, []string{"case_1", "case_2"})

	// Any other node keeps continuing on its first output.
	got = run("test.fail", nil, PortOut)
	same("other node: continued on", got.ran, []string{PortOut})
}
