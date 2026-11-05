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

func TestNewFlowAIProposalDoesNotCreateFlow(t *testing.T) {
	s, _ := newFlowsTestServer(t)
	ctx := context.Background()
	before, err := s.Flows.ListFlows(ctx)
	if err != nil {
		t.Fatal(err)
	}
	stepper := proposalStepper(func(_ context.Context, req flows.LLMRequest) (flows.LLMResponse, error) {
		if req.FlowID != "flow_ai_proposal" || !strings.Contains(req.Prompt, "say hello") {
			t.Fatalf("unexpected proposal request: %+v", req)
		}
		return flows.LLMResponse{Text: greetFlowJSON}, nil
	})
	result, err := s.makeNewFlowProposal(ctx, "say hello", stepper)
	if err != nil {
		t.Fatal(err)
	}
	doc := result["doc"].(*flows.Flow)
	if doc.ID != "" || doc.Kind != flows.KindFlow || result["effects"] == nil {
		t.Fatalf("unexpected proposal: %+v", result)
	}
	after, err := s.Flows.ListFlows(ctx)
	if err != nil || len(after) != len(before) {
		t.Fatalf("proposal changed flow collection: before=%d after=%d err=%v", len(before), len(after), err)
	}
	bad := proposalStepper(func(context.Context, flows.LLMRequest) (flows.LLMResponse, error) {
		return flows.LLMResponse{Text: strings.ReplaceAll(greetFlowJSON, "logic.set", "unsupported.node")}, nil
	})
	if _, err := s.makeNewFlowProposal(ctx, "say hello", bad); err == nil {
		t.Fatal("unsupported model-generated step was accepted")
	}
}

func TestFlowAIProposalRejectsInventedBlockReferences(t *testing.T) {
	s, _ := newFlowsTestServer(t)
	for _, typ := range []string{"component.call", "component.each"} {
		stepper := proposalStepper(func(_ context.Context, req flows.LLMRequest) (flows.LLMResponse, error) {
			if strings.Contains(req.System, `"type":"component.call"`) || strings.Contains(req.System, `"type":"component.each"`) {
				t.Fatal("proposal catalog disclosed block node types")
			}
			return flows.LLMResponse{Text: strings.ReplaceAll(greetFlowJSON, "logic.set", typ)}, nil
		})
		_, err := s.makeNewFlowProposal(context.Background(), "say hello", stepper)
		var nodeErr *flows.NodeError
		if !errors.As(err, &nodeErr) || nodeErr.Code != "FLOW_AI_OUTPUT_INVALID" {
			t.Fatalf("proposal %s: %v", typ, err)
		}
	}
}

func TestNewFlowAIProposalRouteRequiresAdmin(t *testing.T) {
	s, token := newFlowsTestServer(t)
	path := "/api/desktop/flows/ai-proposal"
	readToken, _, err := s.TokenManager.Create("read only", []string{desktopScopeRead}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if w := flowsCall(t, s, http.MethodPost, path, readToken, `{"goal":"hello"}`); w.Code != http.StatusForbidden {
		t.Fatalf("read-only token = %d %s", w.Code, w.Body.String())
	}
	if w := flowsCall(t, s, http.MethodPost, path, token, `{"goal":""}`); w.Code != http.StatusBadRequest {
		t.Fatalf("empty goal = %d %s", w.Code, w.Body.String())
	}
}
