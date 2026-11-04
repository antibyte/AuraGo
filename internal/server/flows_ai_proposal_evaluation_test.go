package server

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"aurago/internal/flows"
)

// Controlled responses check the proposal-to-simulation contract, not model quality.
func TestFlowAIProposalReferenceOutcomes(t *testing.T) {
	for _, tc := range []struct {
		name, goal, expression, expected string
		input                            map[string]any
	}{
		{"greeting", "Greet the supplied name locally", "Hallo {{trigger.data.name}}", "Hallo Ada", map[string]any{"name": "Ada"}},
		{"uppercase", "Make the name uppercase before greeting", "Hallo {{trigger.data.name | upper}}", "Hallo ADA", map[string]any{"name": "Ada"}},
		{"missing-input", "Use Gast when no name was supplied", `Hallo {{trigger.data.name | default("Gast")}}`, "Hallo Gast", map[string]any{}},
		{"unicode", "Preserve the supplied international name", "Hallo {{trigger.data.name}}", "Hallo 世界", map[string]any{"name": "世界"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, token := newFlowsTestServer(t)
			doc, err := flows.ParseFlow([]byte(greetFlowJSON))
			if err != nil {
				t.Fatal(err)
			}
			doc.Nodes[1].Params["fields"] = []map[string]any{{"name": "greeting", "value": tc.expression}}
			response, err := json.Marshal(doc)
			if err != nil {
				t.Fatal(err)
			}
			proposal, err := s.makeNewFlowProposal(context.Background(), tc.goal, proposalStepper(func(context.Context, flows.LLMRequest) (flows.LLMResponse, error) {
				return flows.LLMResponse{Text: string(response)}, nil
			}))
			if err != nil {
				t.Fatal(err)
			}
			body, err := json.Marshal(map[string]any{"import": proposal["doc"]})
			if err != nil {
				t.Fatal(err)
			}
			created := flowsCall(t, s, http.MethodPost, "/api/desktop/flows", token, string(body))
			if created.Code != http.StatusCreated {
				t.Fatalf("apply proposal: %d %s", created.Code, created.Body.String())
			}
			var applied struct {
				Flow struct {
					ID            string `json:"id"`
					DraftRevision int    `json:"draft_revision"`
				} `json:"flow"`
			}
			if err := json.Unmarshal(created.Body.Bytes(), &applied); err != nil || applied.Flow.ID == "" {
				t.Fatalf("read imported draft: %v %s", err, created.Body.String())
			}
			result, err := s.Flows.Simulate(context.Background(), applied.Flow.ID, flows.SimulationRequest{
				TriggerNode: "n_aaaaaaaa", TriggerData: tc.input, BaseRevision: applied.Flow.DraftRevision,
			})
			if err != nil || result.Status != flows.RunSuccess {
				t.Fatalf("simulate proposal: %+v %v", result, err)
			}
			var got any
			for _, step := range result.Steps {
				if step.NodeID == "n_bbbbbbbb" {
					got = step.Output["greeting"]
					if step.Status != flows.StepSuccess {
						t.Fatalf("greeting step: %+v", step)
					}
				}
			}
			if got != tc.expected {
				t.Fatalf("reference result = %#v, want %q", got, tc.expected)
			}
			saved, err := s.Flows.GetFlow(context.Background(), applied.Flow.ID)
			if err != nil || saved.LiveRevision != 0 {
				t.Fatalf("proposal test published a version: %+v %v", saved, err)
			}
		})
	}
}
