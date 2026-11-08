package server

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/decisions"
	"aurago/internal/decisions/consumers"
)

type helperProbeService struct {
	*fakeDecisions
	mu          sync.Mutex
	now         time.Time
	latency     time.Duration
	perQuestion bool
	err         error
	hook        func(context.Context)
	gen         atomic.Uint64
	calls       atomic.Int64
}

func (s *helperProbeService) clock() time.Time { s.mu.Lock(); defer s.mu.Unlock(); return s.now }
func (s *helperProbeService) advance(d time.Duration) {
	s.mu.Lock()
	s.now = s.now.Add(d)
	s.mu.Unlock()
}
func (s *helperProbeService) CacheGeneration() uint64 { return s.gen.Load() }
func (s *helperProbeService) Load() (int, int)        { return 0, 0 }
func (s *helperProbeService) Evaluate(ctx context.Context, r decisions.Request) (decisions.Response, error) {
	s.calls.Add(1)
	if s.hook != nil {
		s.hook(ctx)
	}
	d := s.latency
	if s.perQuestion {
		d *= time.Duration(len(r.Questions))
	}
	s.advance(d)
	response := decisions.Response{Answers: map[string]decisions.Answer{}}
	for _, q := range r.Questions {
		switch q.Type {
		case "noul":
			p := .95
			response.Answers[q.ID] = decisions.Answer{Noul: &p, Probabilities: map[string]float64{"yes": p, "no": 1 - p}}
		case "score":
			v := 9.0
			response.Answers[q.ID] = decisions.Answer{Score: &v}
		case "choice":
			probs := map[string]float64{}
			for _, o := range q.Options {
				probs[o.ID] = .05 / float64(len(q.Options)-1)
			}
			probs[q.Options[0].ID] = .95
			response.Answers[q.ID] = decisions.Answer{Decision: q.Options[0].ID, Probabilities: probs}
		}
	}
	return response, s.err
}

func helperProbeFixture(t *testing.T) (*Server, *helperProbeService) {
	t.Helper()
	s, fake := hubLocal(t)
	s.Cfg.LLM.HelperEnabled = true
	s.Cfg.LLM.HelperProviderType = "ollama"
	s.Cfg.LLM.HelperResolvedModel = "fixture-helper"
	s.Cfg.LLMRouter.Enabled, s.Cfg.LLMRouter.DecisionsFallback = true, true
	probe := &helperProbeService{fakeDecisions: fake, now: time.Now(), latency: 100 * time.Millisecond}
	s.Decisions = probe
	s.decisionHub().now = probe.clock
	return s, probe
}

func TestHelperDecisionsQualificationIsPassiveAndProfileSpecific(t *testing.T) {
	s, p := helperProbeFixture(t)
	h := s.decisionHub()
	if h.Snapshot(s.Cfg, consumers.HelperRAG).Available || p.calls.Load() != 0 {
		t.Fatal("snapshot performed or permitted unqualified inference")
	}
	h.qualifyHelperProfile(t.Context(), consumers.HelperRAG)
	snap := h.Snapshot(s.Cfg, consumers.HelperRAG)
	if !snap.Available || p.calls.Load() != 6 {
		t.Fatalf("qualification: available=%v calls=%d", snap.Available, p.calls.Load())
	}
	if h.Snapshot(s.Cfg, consumers.Router).Available {
		t.Fatal("RAG qualification leaked to router")
	}
	snap.RecordUse(7, 1)
	status := h.helperStatus()[1]
	if status.Samples != 5 || status.ProbeCalls != 6 || status.Evaluations != 7 || status.AvoidedCalls != 1 {
		t.Fatalf("status: %+v", status)
	}
	if h.Stats()[consumers.HelperRAG].Attempts != 0 {
		t.Fatal("probes counted as production evaluations")
	}
}

func TestHelperDecisionsMeasuresWholeMultiQuestionRequest(t *testing.T) {
	s, p := helperProbeFixture(t)
	p.latency, p.perQuestion = 150*time.Millisecond, true
	h := s.decisionHub()
	h.qualifyHelperProfile(t.Context(), consumers.Router)
	if !h.Snapshot(s.Cfg, consumers.Router).Available {
		t.Fatal("fast routing did not qualify")
	}
	h.qualifyHelperProfile(t.Context(), consumers.HelperRAG)
	if h.Snapshot(s.Cfg, consumers.HelperRAG).Available {
		t.Fatal("seven-question request incorrectly qualified")
	}
	if h.helperStatus()[1].Code != "helper_slow" {
		t.Fatal("missing slow status")
	}
}

func TestHelperDecisionsExpiryFailureAndGenerationRevoke(t *testing.T) {
	s, p := helperProbeFixture(t)
	h := s.decisionHub()
	h.qualifyHelperProfile(t.Context(), consumers.HelperTurn)
	old := h.Snapshot(s.Cfg, consumers.HelperTurn)
	p.advance(consumers.HelperQualificationTTL)
	if old.Current() || h.Snapshot(s.Cfg, consumers.HelperTurn).Available {
		t.Fatal("expired qualification survived")
	}
	h.qualifyHelperProfile(t.Context(), consumers.HelperTurn)
	snap := h.Snapshot(s.Cfg, consumers.HelperTurn)
	p.latency = consumers.HelperTimeout
	if _, err := snap.Evaluate(t.Context(), agentHelperTurnProbe()); err == nil {
		t.Fatal("900ms boundary accepted")
	}
	before := p.calls.Load()
	h.qualifyHelperProfile(t.Context(), consumers.HelperTurn)
	if p.calls.Load() != before {
		t.Fatal("failure cooldown ignored")
	}
	p.latency = 100 * time.Millisecond
	p.advance(consumers.HelperRetryDelay)
	h.qualifyHelperProfile(t.Context(), consumers.HelperTurn)
	if !h.Snapshot(s.Cfg, consumers.HelperTurn).Available {
		t.Fatal("failed to requalify")
	}
	p.gen.Add(1)
	if h.Snapshot(s.Cfg, consumers.HelperTurn).Available {
		t.Fatal("generation change retained qualification")
	}
}

func agentHelperTurnProbe() decisions.Request {
	return decisions.Request{State: "bounded fixture", Questions: []decisions.Question{{ID: "durable", Type: "noul"}}}
}

func TestHelperDecisionsCallerCancellationDoesNotRevoke(t *testing.T) {
	s, p := helperProbeFixture(t)
	h := s.decisionHub()
	h.qualifyHelperProfile(t.Context(), consumers.HelperTurn)
	snap := h.Snapshot(s.Cfg, consumers.HelperTurn)
	ctx, cancel := context.WithCancel(t.Context())
	p.hook = func(context.Context) { cancel() }
	if _, err := snap.Evaluate(ctx, agentHelperTurnProbe()); err == nil {
		t.Fatal("cancelled result accepted")
	}
	if !h.Snapshot(s.Cfg, consumers.HelperTurn).Available {
		t.Fatal("caller cancellation revoked performance qualification")
	}
}

func TestHelperDecisionsWorkerDrainsOnStop(t *testing.T) {
	s, p := helperProbeFixture(t)
	entered := make(chan struct{}, 1)
	p.hook = func(ctx context.Context) {
		select {
		case entered <- struct{}{}:
		default:
		}
		<-ctx.Done()
	}
	h := s.decisionHub()
	h.startHelperQualifications(t.Context())
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("worker did not start")
	}
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	h.stopHelperQualifications(ctx)
	if ctx.Err() != nil {
		t.Fatal("worker did not drain")
	}
}
