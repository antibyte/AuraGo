package server

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/sashabaranov/go-openai"

	"aurago/internal/flows"
)

// An answer that the model cut off at its token limit would be cut off again by the
// same request, so an ai.step under Retry 3 in fields mode (up to six calls without the
// rule) makes exactly one call and ends with FLOW_AI_OUTPUT_CUT, and no repair call
// follows the cut.
func TestAudit23CutAnswerIsNotRetried(t *testing.T) {
	main := &c14ChatClient{answer: func(int, openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
		return c14Answer(`{"title":"Ber`, openai.FinishReasonLength), nil
	}}
	llmStep, _, _, _ := c14FlowLLM(main)

	reg := flows.NewRegistry()
	reg.MustRegister(&flows.NodeDef{Type: "test.trigger", Category: "trigger", Trigger: true})
	if err := flows.RegisterAINodes(reg); err != nil {
		t.Fatal(err)
	}
	const trigger, ask = "n_aaaaaaaa", "n_aaaaaaab"
	flow := &flows.Flow{Schema: flows.SchemaVersion, ID: "flow_audit23", Kind: flows.KindFlow, Name: "Cut",
		Nodes: []flows.Node{
			{ID: trigger, Key: "start", Type: "test.trigger", TypeVersion: 1, Label: "start"},
			{ID: ask, Key: "ask", Type: flows.TypeAIStep, TypeVersion: 1, Label: "ask",
				Params: map[string]any{"prompt": "Title?", "output_mode": "fields",
					"fields": []any{map[string]any{"name": "title", "type": "text"}}},
				Settings: flows.NodeSettings{Retry: flows.RetryPolicy{Count: 3}}},
		},
		Edges: []flows.Edge{{ID: "e_1", Source: flows.PortRef{Node: trigger, Port: flows.PortOut}, Target: flows.PortRef{Node: ask, Port: flows.PortIn}}},
	}
	flow.Normalize()

	eng := flows.NewEngine(reg, &flows.Services{LLM: llmStep, Location: time.UTC}, slog.New(slog.NewTextHandler(io.Discard, nil)), 1)
	res := eng.Execute(context.Background(), flows.RunRequest{RunID: "run_audit23", Flow: flow, Mode: flows.ModeTest,
		TriggerNode: trigger, TriggerType: "manual"}, nil)
	if res.Status != flows.RunError || res.ErrorCode != "FLOW_AI_OUTPUT_CUT" {
		t.Fatalf("run = %s %s %q; want FLOW_AI_OUTPUT_CUT", res.Status, res.ErrorCode, res.ErrorMessage)
	}
	if len(main.requests) != 1 {
		t.Fatalf("the model was called %d times, want 1 (no retry, no repair)", len(main.requests))
	}
}
