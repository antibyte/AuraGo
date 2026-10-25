package flows

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// E4: the run log keeps at most MaxStoredRunBytes of one run; later steps keep a marker, while
// templates and outputs stay whole.
func TestFB4RunLogKeepsAtMostMaxStoredRunBytes(t *testing.T) {
	reg := newTestRegistry(t)
	blob := strings.Repeat("x", 200<<10)
	reg.MustRegister(&NodeDef{Type: "test.blob", Execute: func(context.Context, ExecInput) (ExecResult, error) {
		return ExecResult{Output: map[string]any{"blob": blob}}, nil
	}})
	b := newFlow("Budget")
	tr := b.node("start", "test.trigger", nil)
	prev, ids := tr, make([]string, 30)
	for i := range ids {
		ids[i] = b.node(fmt.Sprintf("b%d", i), "test.blob", nil)
		b.edge(prev, PortOut, ids[i])
		prev = ids[i]
	}
	res, _ := runWith(context.Background(), newTestEngine(reg, nil, 4), b.build(), RunRequest{TriggerNode: tr})
	if res.Status != RunSuccess {
		t.Fatalf("run = %s %s %s", res.Status, res.ErrorCode, res.ErrorMessage)
	}
	kept, omitted := 0, 0
	for _, id := range ids {
		s := stepOf(res, id)
		switch {
		case s.Output["_omitted"] == storedOmittedReason && s.OutputTruncated && s.ParamsTruncated:
			omitted++
		case s.Output["blob"] == blob:
			if omitted > 0 {
				t.Fatalf("step %s kept its output after an omitted one", s.NodeKey)
			}
			kept++
		default:
			t.Fatalf("step %s output = %.80v", s.NodeKey, s.Output)
		}
	}
	if kept != 20 || omitted != 10 {
		t.Fatalf("kept %d, omitted %d; want 20 and 10 (about 205 KB per step, 4 MiB per run)", kept, omitted)
	}
	if got := res.Outputs["b29"]["blob"]; got != blob {
		t.Fatal("outputs must stay whole for templates and later steps")
	}
}
