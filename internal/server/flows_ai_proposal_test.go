package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"aurago/internal/flows"
	"aurago/internal/security"
)

type proposalStepper func(context.Context, flows.LLMRequest) (flows.LLMResponse, error)

func (f proposalStepper) Step(ctx context.Context, req flows.LLMRequest) (flows.LLMResponse, error) {
	return f(ctx, req)
}

func TestFlowAIProposalGoalUnicodeLimit(t *testing.T) {
	s, token := newFlowsTestServer(t)
	rec := createTestFlow(t, s, greetFlowJSON)
	path := "/api/desktop/flows/" + rec.ID + "/ai-proposal"
	for _, tc := range []struct {
		goal string
		code string
	}{
		{strings.Repeat("😀", 4000), "FLOW_REVISION_CONFLICT"},
		{strings.Repeat("😀", 4001), "FLOW_BAD_REQUEST"},
		{"   ", "FLOW_BAD_REQUEST"},
	} {
		data, err := json.Marshal(map[string]any{"goal": tc.goal, "base_revision": 999})
		if err != nil {
			t.Fatal(err)
		}
		w := flowsCall(t, s, http.MethodPost, path, token, string(data))
		if got := flowsBody(t, w)["code"]; got != tc.code {
			t.Errorf("%d characters = %d %v, want %s", len([]rune(tc.goal)), w.Code, got, tc.code)
		}
	}
}

func TestFlowAIProposalOnlySendsGoalAndChecksRevision(t *testing.T) {
	s, _ := newFlowsTestServer(t)
	rec := createTestFlow(t, s, strings.ReplaceAll(greetFlowJSON, "Welt", "private-draft-content"))
	stepper := proposalStepper(func(_ context.Context, req flows.LLMRequest) (flows.LLMResponse, error) {
		if strings.Contains(req.System+req.Prompt, "private-draft-content") || !strings.Contains(req.Prompt, "say hello") {
			t.Fatalf("proposal prompt disclosed the draft or omitted the goal")
		}
		for _, part := range []string{req.System, req.Prompt} {
			if scan, blocked := security.GateScanComposedPrompt(flowPromptGuardian, part); blocked || scan.Incomplete {
				t.Fatalf("ordinary proposal prompt was blocked or incompletely scanned: %+v", scan)
			}
		}
		return flows.LLMResponse{Text: greetFlowJSON}, nil
	})
	result, err := s.makeFlowProposal(context.Background(), rec.ID, "say hello", rec.DraftRevision, stepper)
	if err != nil {
		t.Fatal(err)
	}
	if result["base_revision"] != rec.DraftRevision || result["doc"].(*flows.Flow).ID != rec.ID {
		t.Fatalf("proposal = %+v", result)
	}
	after, err := s.Flows.GetFlow(context.Background(), rec.ID)
	if err != nil || after.DraftRevision != rec.DraftRevision {
		t.Fatalf("proposal changed draft: %+v, %v", after, err)
	}
	stepper = func(ctx context.Context, _ flows.LLMRequest) (flows.LLMResponse, error) {
		if _, _, err := s.Flows.SaveDraft(ctx, rec.ID, rec.Draft, rec.DraftRevision); err != nil {
			t.Fatal(err)
		}
		return flows.LLMResponse{Text: greetFlowJSON}, nil
	}
	if _, err := s.makeFlowProposal(context.Background(), rec.ID, "say hello", rec.DraftRevision, stepper); !errors.Is(err, flows.ErrRevisionConflict) {
		t.Fatalf("changed revision = %v", err)
	}
	bad := proposalStepper(func(context.Context, flows.LLMRequest) (flows.LLMResponse, error) {
		return flows.LLMResponse{Text: strings.ReplaceAll(greetFlowJSON, "logic.set", "unsupported.node")}, nil
	})
	if _, err := s.makeFlowProposal(context.Background(), rec.ID, "say hello", rec.DraftRevision+1, bad); err == nil {
		t.Fatal("unsupported model-generated step was accepted")
	}
}
