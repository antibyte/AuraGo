package agent

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"testing"

	"aurago/internal/decisions"
)

func rerankScoreResponse(scores ...float64) decisions.Response {
	answers := map[string]decisions.Answer{}
	for i, s := range scores {
		v := s
		answers["m"+string(rune('1'+i))] = decisions.Answer{Score: &v}
	}
	return decisions.Response{Answers: answers}
}

func rerankFixture() []rankedMemory {
	return []rankedMemory{
		{text: "first", docID: "1", score: 0.6},
		{text: "second", docID: "2", score: 0.6},
		{text: "third", docID: "3", score: 0.6},
	}
}

func rerankLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func TestRerankDecisionsOffKeepsLLMPath(t *testing.T) {
	f := &fakeConsumers{available: false}
	cfg := newTestConfig(true, false, true)
	got := rerankRecalled(context.Background(), RunConfig{DecisionConsumers: f}, cfg, rerankLogger(), rerankFixture(), "q", nil)
	if f.calls != 0 {
		t.Fatalf("calls = %d, want 0", f.calls)
	}
	if got[0].docID != "1" || got[1].docID != "2" || got[2].docID != "3" {
		t.Fatalf("order changed: %+v", got)
	}
}

func TestRerankDecisionsBlendsScores(t *testing.T) {
	f := &fakeConsumers{available: true, evaluate: func(context.Context, decisions.Request) (decisions.Response, error) {
		return rerankScoreResponse(9, 2, 5), nil
	}}
	cfg := newTestConfig(true, false, true)
	got := rerankRecalled(context.Background(), RunConfig{DecisionConsumers: f}, cfg, rerankLogger(), rerankFixture(), "q", nil)
	if f.calls != 1 {
		t.Fatalf("calls = %d, want 1", f.calls)
	}
	if got[0].docID != "1" || got[1].docID != "3" || got[2].docID != "2" {
		t.Fatalf("order = %+v", got)
	}
	if want := 0.9*0.7 + 0.6*0.3; got[0].score < want-1e-9 || got[0].score > want+1e-9 {
		t.Fatalf("score = %v, want %v", got[0].score, want)
	}
}

func TestRerankDecisionsPartialFallsBack(t *testing.T) {
	cases := map[string]func(context.Context, decisions.Request) (decisions.Response, error){
		"partial": func(context.Context, decisions.Request) (decisions.Response, error) {
			return rerankScoreResponse(9, 2), nil
		},
		"error": func(context.Context, decisions.Request) (decisions.Response, error) {
			return decisions.Response{}, errors.New("admission_deferred")
		},
	}
	for name, eval := range cases {
		t.Run(name, func(t *testing.T) {
			f := &fakeConsumers{available: true, evaluate: eval}
			cfg := newTestConfig(true, false, true)
			in := rerankFixture()
			got, ok := rerankWithDecisions(context.Background(), RunConfig{DecisionConsumers: f}, in, "q")
			if ok || got != nil {
				t.Fatalf("expected fallback, got %+v ok=%v", got, ok)
			}
			out := rerankRecalled(context.Background(), RunConfig{DecisionConsumers: f}, cfg, rerankLogger(), in, "q", nil)
			if out[0].docID != "1" || out[1].score != 0.6 {
				t.Fatalf("fallback changed input: %+v", out)
			}
		})
	}
}

func TestRerankDecisionsRequestBounded(t *testing.T) {
	var cands []rankedMemory
	for i := 0; i < 9; i++ {
		cands = append(cands, rankedMemory{text: strings.Repeat("Q", 500), docID: "d", score: 0.5})
	}
	f := &fakeConsumers{available: true, evaluate: func(_ context.Context, r decisions.Request) (decisions.Response, error) {
		if len(r.Questions) > rerankDecisionMax {
			t.Errorf("questions = %d", len(r.Questions))
		}
		if !strings.Contains(fmt.Sprint(r.State), "(untrusted)") {
			t.Errorf("state lacks untrusted marker: %q", r.State)
		}
		if strings.Count(fmt.Sprint(r.State), "Q") > rerankDecisionMax*200 {
			t.Errorf("candidate text not bounded")
		}
		for _, q := range r.Questions {
			if strings.Contains(q.Instructions, "QQQ") {
				t.Errorf("instructions contain candidate text")
			}
		}
		return decisions.Response{}, errors.New("stop")
	}}
	cfg := newTestConfig(true, false, true)
	rerankRecalled(context.Background(), RunConfig{DecisionConsumers: f}, cfg, rerankLogger(), cands, "q", nil)
	if f.calls != 1 {
		t.Fatalf("calls = %d", f.calls)
	}
}
