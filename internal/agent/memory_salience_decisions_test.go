package agent

import (
	"context"
	"errors"
	"testing"

	"aurago/internal/decisions"
)

func durableResponse(yes float64) decisions.Response {
	p := yes
	return decisions.Response{Answers: map[string]decisions.Answer{
		"durable": {Type: "noul", Noul: &p, Probabilities: map[string]float64{"yes": yes, "no": 1 - yes}},
	}}
}

func salienceDecisionGate(t *testing.T, vector []float32, fake *fakeConsumers) (bool, string) {
	t.Helper()
	const msg = "Kannst du dir das kurz ansehen und mir sagen was du meinst?"
	embedder := &fakeSalienceEmbedder{messages: map[string][]float32{msg: vector}}
	resetSalienceCacheForTest(t)
	t.Cleanup(func() { resetSalienceCacheForTest(t) })
	if err := warmSaliencePrototypes(context.Background(), embedder); err != nil {
		t.Fatal(err)
	}
	return memoryExtractionWorthwhileDecided(context.Background(), RunConfig{DecisionConsumers: fake}, embedder, msg)
}

// Prototype vectors are (1,0) salient and (0,1) filler, so the margin is
// (b-a)/norm: {0.7, 0.714} is about 0.014 (gray zone).
var grayVector = []float32{0.7, 0.714}

func TestSalienceGrayZoneAsksDecisions(t *testing.T) {
	fake := &fakeConsumers{available: true, evaluate: func(context.Context, decisions.Request) (decisions.Response, error) {
		return durableResponse(0.1), nil
	}}
	worth, reason := salienceDecisionGate(t, grayVector, fake)
	if worth || reason != "decisions_no_fact" || fake.calls != 1 {
		t.Fatalf("got %v %q calls=%d", worth, reason, fake.calls)
	}
}

func TestSalienceGrayZoneKeepsOnYes(t *testing.T) {
	cases := map[string]func(context.Context, decisions.Request) (decisions.Response, error){
		"yes": func(context.Context, decisions.Request) (decisions.Response, error) { return durableResponse(0.5), nil },
		"error": func(context.Context, decisions.Request) (decisions.Response, error) {
			return decisions.Response{}, errors.New("preempted")
		},
	}
	for name, eval := range cases {
		t.Run(name, func(t *testing.T) {
			fake := &fakeConsumers{available: true, evaluate: eval}
			worth, reason := salienceDecisionGate(t, grayVector, fake)
			if !worth || reason != "" || fake.calls != 1 {
				t.Fatalf("got %v %q calls=%d", worth, reason, fake.calls)
			}
		})
	}
}

func TestSalienceOutsideGrayZoneNoCall(t *testing.T) {
	for name, vector := range map[string][]float32{"filler side": {0.6, 0.7}, "salient side": {0.7, 0.6}} {
		t.Run(name, func(t *testing.T) {
			fake := &fakeConsumers{available: true, evaluate: func(context.Context, decisions.Request) (decisions.Response, error) {
				return durableResponse(0.0), nil
			}}
			worth, reason := salienceDecisionGate(t, vector, fake)
			wantWorth, wantReason := name == "salient side", "filler_prototype"
			if wantWorth {
				wantReason = ""
			}
			if worth != wantWorth || reason != wantReason || fake.calls != 0 {
				t.Fatalf("got %v %q calls=%d", worth, reason, fake.calls)
			}
		})
	}
}

func TestSalienceDecisionsOffUnchanged(t *testing.T) {
	fake := &fakeConsumers{available: false}
	worth, reason := salienceDecisionGate(t, grayVector, fake)
	if !worth || reason != "" || fake.calls != 0 {
		t.Fatalf("got %v %q calls=%d", worth, reason, fake.calls)
	}
}
