package flows

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestTemplateRootParamKeepsClassification(t *testing.T) {
	for _, size := range []int{40, 41, 64} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			name := strings.Repeat("x", size-4) + "_url"
			reg := newTestRegistry(t)
			reg.MustRegister(&NodeDef{Type: "test.long", Params: []ParamSpec{{Name: name, SensitiveSink: true, URLTarget: true}}})
			b := newFlow("Long URL")
			start := b.node("start", "test.trigger", nil)
			web := b.node("web", "test.web", nil)
			sink := b.node("sink", "test.long", map[string]any{name: "{{web.text}}"})
			b.edge(start, PortOut, web)
			b.edge(web, PortOut, sink)
			issues := LintUntrustedData(b.build(), reg)
			if len(issues) != 1 || issues[0].Code != IssueUntrustedURLTarget || issues[0].Param != truncateForError(name) {
				t.Fatalf("long URL escaped classification: %+v", issues)
			}
		})
	}
	// These keys have identical display paths but different taint semantics.
	first, second := strings.Repeat("a", 40)+"_body", strings.Repeat("a", 40)+"_copy"
	params := map[string]any{first: []any{map[string]any{"value": "{{web.text}}"}}, second: "{{web.text}}"}
	refs, problems := CollectTemplateRefs(params)
	if len(refs) != 2 || len(problems) != 0 || refs[0].RootParam != first || refs[1].RootParam != second {
		t.Fatalf("root identity lost in nested/display paths: %+v %+v", refs, problems)
	}
	reg := newTestRegistry(t)
	reg.MustRegister(&NodeDef{Type: "test.independent", Params: []ParamSpec{{Name: first, OutputIndependent: true}, {Name: second}}})
	for _, copying := range []bool{false, true} {
		b := newFlow("Long independent")
		start := b.node("start", "test.trigger", nil)
		web := b.node("web", "test.web", nil)
		p := map[string]any{first: params[first]}
		if copying {
			p[second] = params[second]
		}
		middle := b.node("middle", "test.independent", p)
		sink := b.node("sink", "test.sink", map[string]any{"command": "{{middle.text}}"})
		b.edge(start, PortOut, web)
		b.edge(web, PortOut, middle)
		b.edge(middle, PortOut, sink)
		issues := LintUntrustedData(b.build(), reg)
		if (len(issues) != 0) != copying {
			t.Fatalf("copying=%v: wrong taint after independent param: %+v", copying, issues)
		}
	}
}

func TestSaveTriggerSampleRefusesIncompleteData(t *testing.T) {
	s, _ := newServiceFixture(t, nil)
	ctx := context.Background()
	rec, err := s.CreateFlow(ctx, CreateRequest{Import: simpleFlow("Sample")})
	if err != nil {
		t.Fatal(err)
	}
	trigger := rec.Draft.Nodes[0].ID
	good := map[string]any{"text": "a [redacted] label"}
	if err := s.SaveTriggerSample(ctx, rec.ID, trigger, good); err != nil {
		t.Fatal(err)
	}
	for _, data := range []map[string]any{
		{"nested": []any{map[string]any{"value": "[redacted]"}}},
		{"nested": map[string]any{"_preview": "partial", "_truncated": true}},
	} {
		if err := s.SaveTriggerSample(ctx, rec.ID, trigger, data); !errors.Is(err, ErrTestDataRedacted) {
			t.Fatalf("incomplete sample accepted: %v", err)
		}
	}
	got, ok, err := s.store.GetTestData(ctx, rec.ID, trigger, TestDataTriggerSample)
	if err != nil || !ok || !reflect.DeepEqual(got, good) {
		t.Fatalf("good sample replaced: %v %v %v", got, ok, err)
	}
	if runs, err := s.Runs(ctx, rec.ID, RunFilter{}); err != nil || len(runs) != 0 {
		t.Fatalf("sample write created a run: %v %v", runs, err)
	}
}

func TestExtractionClassifiesLongParamIdentity(t *testing.T) {
	for _, fenced := range []bool{false, true} {
		t.Run(fmt.Sprint(fenced), func(t *testing.T) {
			s, _ := newServiceFixture(t, nil)
			name := strings.Repeat("a", 60)
			s.reg.MustRegister(&NodeDef{Type: "test.long_receiver", Params: []ParamSpec{{Name: name, FenceUntrusted: fenced}}, Outputs: []string{PortOut}})
			b := newFlow("Extraction boundary")
			start := b.node("start", TypeTriggerManual, nil)
			field := func(value string) map[string]any {
				return map[string]any{"keep_input": false, "fields": []any{map[string]any{"name": "text", "type": "text", "value": value}}}
			}
			first := b.node("first", TypeSet, field("{{trigger.data.name}}"))
			after := b.node("after", TypeSet, field("{{first.text}}"))
			sink := b.node("sink", "test.long_receiver", map[string]any{name: "{{after.text}}"})
			b.edge(start, PortOut, first)
			b.edge(first, PortOut, after)
			b.edge(after, PortOut, sink)
			rec, err := s.CreateFlow(context.Background(), CreateRequest{Import: b.build()})
			if err != nil {
				t.Fatal(err)
			}
			_, err = s.PreviewExtraction(context.Background(), rec.ID, ExtractionRequest{BaseRevision: rec.DraftRevision, NodeIDs: []string{first}, Name: "Boundary"})
			var reason *ExtractionError
			if fenced {
				if !errors.As(err, &reason) || reason.Reason != "taint" {
					t.Fatalf("changed fence accepted: %v", err)
				}
			} else if err != nil {
				t.Fatalf("known harmless parameter refused: %v", err)
			}
		})
	}
}

func TestAIStepFinalRequestBudget(t *testing.T) {
	params := aiFieldsParams()
	probe := &fakeLLM{responses: []LLMResponse{{JSON: aiAnswer(aiFields(params["fields"]))}}}
	if _, err := execDef(aiDef(t), params, &Services{LLM: probe}); err != nil {
		t.Fatal(err)
	}
	request := probe.calls[0]
	schema, err := json.Marshal(request.JSONSchema)
	if err != nil {
		t.Fatal(err)
	}
	remaining := maxAIPromptBytes - len(request.System) - len(schema)
	for _, tc := range []struct {
		name      string
		size      int
		repair    bool
		calls     int
		wantError bool
	}{
		{"at final limit", remaining, false, 1, false},
		{"schema and hints exceed limit", remaining + 1, false, 0, true},
		{"repair exceeds limit", remaining, true, 1, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := aiFieldsParams()
			p["prompt"] = strings.Repeat("x", tc.size)
			answer := LLMResponse{JSON: aiAnswer(aiFields(p["fields"]))}
			if tc.repair {
				answer = LLMResponse{Text: "invalid"}
			}
			llm := &fakeLLM{responses: []LLMResponse{answer}}
			_, err := execDef(aiDef(t), p, &Services{LLM: llm})
			if tc.wantError {
				if aiCode(t, err) != "FLOW_PARAM_INVALID" {
					t.Fatalf("request cap error: %v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if llm.callCount() != tc.calls {
				t.Fatalf("provider calls=%d, want %d", llm.callCount(), tc.calls)
			}
		})
	}
}

func TestOutputIndependentApprovalReview(t *testing.T) {
	const marker = "REVIEW-MARKER-7f3a91"
	ctx := context.Background()
	f := newDurableApprovalFixture(t, openTestStore(t), "flow_approval_independent", nil, 0)
	doc, err := f.request.Flow.Clone()
	if err != nil {
		t.Fatal(err)
	}
	for i := range doc.Nodes {
		if doc.Nodes[i].Type == durableApprovalType {
			doc.Nodes[i].Params = map[string]any{"summary": marker, "review_data": map[string]any{"marker": marker}}
		}
	}
	revision, err := f.store.SaveDraft(ctx, doc.ID, doc, 1, f.clock.Now())
	if err != nil {
		t.Fatal(err)
	}
	published, err := f.store.Publish(ctx, doc.ID, revision, f.clock.Now())
	if err != nil {
		t.Fatal(err)
	}
	f.request.Flow, f.request.Revision = published.Live, published.LiveRevision
	f.start(t)
	review, err := f.service.GetDurableApproval(ctx, f.gate.GateID)
	if err != nil || review.Review == nil || review.Review.Summary != marker || !strings.Contains(string(review.Review.ReviewData), marker) {
		t.Fatalf("review did not reach encrypted checkpoint: %+v %v", review, err)
	}
	var sealed []byte
	if err := f.store.db.QueryRow(`SELECT ciphertext FROM flow_checkpoints WHERE run_id=?`, f.started.RunID).Scan(&sealed); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(sealed), marker) {
		t.Fatal("review stored in plaintext")
	}
	if _, err := f.service.DecideDurableApproval(ctx, f.gate.GateID, f.gate.GateGeneration, f.gate.ContentDigest, DurableApprovalDecisionApprove, "local_admin"); err != nil {
		t.Fatal(err)
	}
	f.service.recoverDurableDue(ctx)
	result := waitRun(t, f.service, f.started.RunID)
	if result.Status != RunSuccess {
		t.Fatalf("approval did not finish: %+v", result)
	}
	_, steps, err := f.store.GetRun(ctx, f.started.RunID)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, step := range steps {
		if step.NodeKey == "approval" && step.Status == StepSuccess {
			found = true
			if !reflect.DeepEqual(step.Output, map[string]any{"approved": true}) {
				t.Fatalf("review leaked into approval output: %+v", step.Output)
			}
		}
	}
	if !found {
		t.Fatal("successful approval output missing")
	}
}
