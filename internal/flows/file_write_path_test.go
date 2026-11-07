package flows

import (
	"context"
	"path/filepath"
	"testing"
)

func TestFileWriteBoundsUntrustedPathsBeforeAnyToolCall(t *testing.T) {
	for _, mode := range []RunMode{ModeLive, ModeTest} {
		for _, tc := range []struct {
			name, raw, value string
			viaSet, allowed  bool
		}{
			{"whole path", "{{trigger.data.name}}", "notes/report.txt", false, false},
			{"parent traversal", "notes/{{trigger.data.name}}", "../other.txt", false, false},
			{"windows traversal", "notes/{{trigger.data.name}}", `..\other.txt`, false, false},
			{"absolute path", "notes/{{trigger.data.name}}", "/etc/profile", false, false},
			{"drive or stream", "notes/{{trigger.data.name}}", "C:other.txt", false, false},
			{"subdirectory", "notes/{{trigger.data.name}}/report.txt", "other", false, false},
			{"parent name", "notes/{{trigger.data.name}}", "..", false, false},
			{"tainted through set", "notes/{{prep.name}}", "../other.txt", true, false},
			{"safe file name", "notes/{{trigger.data.name}}.txt", "report", false, true},
			{"native Windows folder", `notes\report-{{trigger.data.name}}.txt`, "report", false, filepath.Separator == '\\'},
			{"escaped literal", `notes\{{trigger.data.name}}.txt`, "../ignored", false, true},
			{"safe name through set", "notes/{{prep.name}}.txt", "report", true, true},
			{"static target", "notes/report.txt", "../ignored", false, true},
		} {
			t.Run(string(mode)+"/"+tc.name, func(t *testing.T) {
				r := newSecretRun(t)
				r.tools.respond = func(req ToolRequest) (ToolResponse, error) {
					if req.Args["operation"] == "stat" {
						return ToolResponse{Output: `{"status":"error","message":"stat failed","data":{"error_code":"io_error"}}`, IsError: true, Status: "failed"}, nil
					}
					return ToolResponse{Output: `{"status":"success"}`}, nil
				}
				b := newFlow("file target boundary")
				trigger := b.node("mail", TypeTriggerEmail, nil)
				from := trigger
				if tc.viaSet {
					from = b.node("prep", TypeSet, map[string]any{"fields": []any{map[string]any{"name": "name", "type": "text", "value": "{{trigger.data.name}}"}}})
					b.edge(trigger, PortOut, from)
				}
				write := b.node("write", TypeFileWrite, map[string]any{"path": tc.raw, "content": "test", "if_exists": "unique"})
				b.edge(from, PortOut, write)
				f := b.build()
				f.NodeByID(write).Settings.Retry = RetryPolicy{Count: 3}
				// Bypass publication so existing live revisions get the same guard.
				res, _ := runWith(context.Background(), r.eng, f, RunRequest{TriggerNode: trigger, Mode: mode, Revision: 7, TriggerData: map[string]any{"name": tc.value}})
				step := stepOf(res, write)
				if tc.allowed {
					if step == nil || step.Status != StepSuccess || r.tools.count() != 2 {
						t.Fatalf("safe write failed: %+v, calls=%d", step, r.tools.count())
					}
				} else if step == nil || step.ErrorCode != "FLOW_PARAM_INVALID" || step.Attempt != 1 || r.tools.count() != 0 {
					t.Fatalf("unsafe target reached a tool or retried: %+v, calls=%d", step, r.tools.count())
				}
			})
		}
	}
}

func TestFileWriteKeepsTrustedPathTemplates(t *testing.T) {
	r := newSecretRun(t)
	r.tools.respond = answerWith(ToolResponse{Output: `{"status":"success"}`})
	b := newFlow("trusted path")
	trigger := b.node("start", TypeTriggerManual, nil)
	prep := b.node("prep", TypeSet, map[string]any{"fields": []any{map[string]any{"name": "path", "type": "text", "value": "notes/report.txt"}}})
	write := b.node("write", TypeFileWrite, map[string]any{"path": "{{prep.path}}", "content": "test"})
	b.edge(trigger, PortOut, prep)
	b.edge(prep, PortOut, write)
	res, _ := runWith(context.Background(), r.eng, b.build(), RunRequest{TriggerNode: trigger, Mode: ModeLive})
	if step := stepOf(res, write); step == nil || step.Status != StepSuccess || r.tools.count() != 1 {
		t.Fatalf("trusted path was refused: %+v, calls=%d", step, r.tools.count())
	}
}
