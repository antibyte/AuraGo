package agent

import (
	"context"
	"errors"
	"log/slog"
	"math"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/decisions"
	"aurago/internal/memory"
)

func appraisalAnswer(options []decisions.Option, top string, p float64) decisions.Answer {
	rest := (1 - p) / float64(len(options)-1)
	probs := map[string]float64{}
	for _, o := range options {
		probs[o.ID] = rest
	}
	probs[top] = p
	return decisions.Answer{Type: "choice", Decision: top, Probabilities: probs, Confidence: p}
}

func appraisalResponse(signal string, ps float64, target string, pt float64) decisions.Response {
	return decisions.Response{Answers: map[string]decisions.Answer{
		"signal": appraisalAnswer(appraisalSignalOptions, signal, ps),
		"target": appraisalAnswer(appraisalTargetOptions, target, pt),
	}}
}

type appraisalFixture struct {
	run   RunConfig
	stm   *memory.SQLiteMemory
	basis *memory.PersonalitySnapshot
}

func newAppraisalFixture(t *testing.T, message string, fake *fakeConsumers) appraisalFixture {
	t.Helper()
	stm := newTestPersonalityRuntimeMemory(t)
	cfg := &config.Config{}
	cfg.Personality.Engine = true
	basis, err := stm.GetPersonalitySnapshotAt(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	run := RunConfig{Config: cfg, ShortTermMem: stm, SessionID: "default", HumanInput: memory.NewHumanInput(message, "default"), DecisionConsumers: fake}
	return appraisalFixture{run: run, stm: stm, basis: &basis}
}

func (f appraisalFixture) affinity(t *testing.T) float64 {
	t.Helper()
	traits, err := f.stm.GetTraits()
	if err != nil {
		t.Fatal(err)
	}
	return traits[memory.TraitAffinity]
}

func TestAppraisalDecisionsOnlyWhenHelperOff(t *testing.T) {
	// Off switch: no call, no change.
	fake := &fakeConsumers{available: false}
	f := newAppraisalFixture(t, "Ich finde dich wirklich klasse heute", fake)
	before := f.affinity(t)
	if appraiseTurnWithDecisions(context.Background(), f.run, f.basis, memory.DefaultPersonalityMeta(), slog.Default()) {
		t.Fatal("unavailable consumer must not record")
	}
	if fake.calls != 0 || f.affinity(t) != before {
		t.Fatalf("calls=%d affinity %v -> %v", fake.calls, before, f.affinity(t))
	}
	// Request shape: untrusted state, no message text in instructions.
	req := appraisalDecisionRequest("hello")
	if !strings.Contains(req.State.(string), "(untrusted)") {
		t.Fatalf("state: %v", req.State)
	}
	for _, q := range req.Questions {
		if strings.Contains(q.Instructions, "hello") {
			t.Fatal("instructions contain message")
		}
	}
}

func TestAppraisalDecisionsSkipsWhenLocalMatched(t *testing.T) {
	fake := &fakeConsumers{available: true, evaluate: func(context.Context, decisions.Request) (decisions.Response, error) {
		return appraisalResponse("praise", 0.8, "agent", 0.8), nil
	}}
	f := newAppraisalFixture(t, "Danke", fake)
	if appraiseTurnWithDecisions(context.Background(), f.run, f.basis, memory.DefaultPersonalityMeta(), slog.Default()) || fake.calls != 0 {
		t.Fatalf("local phrase must skip Decisions (calls=%d)", fake.calls)
	}
}

func TestAppraisalDecisionsRecordsObservation(t *testing.T) {
	meta := memory.DefaultPersonalityMeta()
	fake := &fakeConsumers{available: true, evaluate: func(context.Context, decisions.Request) (decisions.Response, error) {
		return appraisalResponse("criticism", 0.8, "agent", 0.8), nil
	}}
	f := newAppraisalFixture(t, "Das war ehrlich gesagt ziemlich schwach von dir", fake)
	before := f.affinity(t)
	if !appraiseTurnWithDecisions(context.Background(), f.run, f.basis, meta, slog.Default()) {
		t.Fatal("expected an applied observation")
	}
	if got := f.affinity(t); got >= before {
		t.Fatalf("affinity %v -> %v, want a decrease", before, got)
	}
	obs := decisionAppraisalObservation(f.run, f.basis, meta, "criticism", "agent", 0.8)
	if obs.Source != "decisions" || obs.Explicit || obs.Confidence != 0.8 || obs.Signal != "criticism" || obs.Target != "agent" {
		t.Fatalf("observation = %+v", obs)
	}
	if want := -0.015 * meta.EmpathyBias * 0.8; math.Abs(obs.AffinityDelta-want) > 1e-12 {
		t.Fatalf("AffinityDelta = %v, want %v", obs.AffinityDelta, want)
	}
}

func TestAppraisalDecisionsUncertainNoop(t *testing.T) {
	cases := map[string]func(context.Context, decisions.Request) (decisions.Response, error){
		"uncertain": func(context.Context, decisions.Request) (decisions.Response, error) {
			return appraisalResponse("criticism", 0.4, "agent", 0.8), nil
		},
		"none": func(context.Context, decisions.Request) (decisions.Response, error) {
			return appraisalResponse("none", 0.9, "agent", 0.8), nil
		},
		"error": func(context.Context, decisions.Request) (decisions.Response, error) {
			return decisions.Response{}, errors.New("preempted")
		},
	}
	for name, eval := range cases {
		t.Run(name, func(t *testing.T) {
			fake := &fakeConsumers{available: true, evaluate: eval}
			f := newAppraisalFixture(t, "Das war ehrlich gesagt ziemlich schwach von dir", fake)
			before := f.affinity(t)
			if appraiseTurnWithDecisions(context.Background(), f.run, f.basis, memory.DefaultPersonalityMeta(), slog.Default()) || f.affinity(t) != before {
				t.Fatal("expected no observation")
			}
			if fake.calls != 1 {
				t.Fatalf("calls = %d", fake.calls)
			}
		})
	}
}

func TestAppraisalThenSalienceBothGetTheDecisionsSlot(t *testing.T) {
	inflight, deferred := 0, 0
	var f *fakeConsumers
	f = &fakeConsumers{available: true, evaluate: func(_ context.Context, r decisions.Request) (decisions.Response, error) {
		if inflight > 0 {
			deferred++
			return decisions.Response{}, errors.New("admission_deferred")
		}
		inflight++
		defer func() { inflight-- }()
		if len(r.Questions) == 2 {
			return appraisalResponse("none", 0.9, "agent", 0.8), nil
		}
		return durableResponse(0.1), nil
	}}
	fx := newAppraisalFixture(t, "Das war ehrlich gesagt ziemlich schwach von dir", f)
	skipped := false
	runSideEffectSteps(context.Background(), time.Minute,
		func(c context.Context) {
			appraiseTurnWithDecisions(c, fx.run, fx.basis, memory.DefaultPersonalityMeta(), slog.Default())
		},
		func(c context.Context) { skipped = salienceDecisionSkips(c, fx.run, "msg") },
	)
	if f.calls != 2 || deferred != 0 || !skipped {
		t.Fatalf("calls=%d deferred=%d skipped=%v", f.calls, deferred, skipped)
	}
}
