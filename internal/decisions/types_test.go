package decisions

import (
	"math"
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestNormalizeLogitsUsesStableSoftmax(t *testing.T) {
	probabilities, err := NormalizeLogits([]float64{1000, 1001, 999})
	if err != nil {
		t.Fatal(err)
	}
	if len(probabilities) != 3 || math.Abs(probabilities[0]+probabilities[1]+probabilities[2]-1) > 1e-12 {
		t.Fatalf("probabilities do not sum to one: %#v", probabilities)
	}
	if probabilities[1] <= probabilities[0] || probabilities[0] <= probabilities[2] {
		t.Fatalf("probability ordering does not match logits: %#v", probabilities)
	}
}

func TestNormalizeLogitsRejectsInvalidScores(t *testing.T) {
	for _, logits := range [][]float64{{}, {1}, {1, math.NaN()}, {1, math.Inf(1)}} {
		if _, err := NormalizeLogits(logits); err == nil {
			t.Fatalf("NormalizeLogits(%v) succeeded", logits)
		}
	}
}

func TestValidateRequestRequiresDistinctCandidatesAndBoundsImages(t *testing.T) {
	value := 2.0
	request := Request{
		State: map[string]any{"mode": "test"},
		Questions: []Question{{
			ID: "q1", Type: "score", Instructions: "select one",
			Options: []Option{{ID: "a", Label: "A", Value: &value}, {ID: "b", Label: "B"}},
		}},
	}
	if err := ValidateRequest(request); err != nil {
		t.Fatalf("valid request rejected: %v", err)
	}
	request.Questions[0].Options[1].ID = "a"
	if err := ValidateRequest(request); err == nil {
		t.Fatal("duplicate option ID accepted")
	}
	request.Questions[0].Options[1].ID = "b"
	request.Images = []Image{{Data: []byte{1, 2, 3}, MIME: "image/svg+xml"}}
	if err := ValidateRequest(request); err == nil {
		t.Fatal("active SVG image accepted")
	}
}

func TestValidateRequestBoundsRuntimeCandidateCodesAndPromptLabels(t *testing.T) {
	request := Request{Questions: []Question{{
		ID: "q", Type: "choice",
		Options: []Option{{ID: "a", Label: "A"}, {ID: "b", Label: "B"}},
	}}}
	request.Questions[0].ID = strings.Repeat("q", maxQuestionIDBytes+1)
	if err := ValidateRequest(request); err == nil {
		t.Fatal("question id longer than the public contract limit was accepted")
	}
	request.Questions[0].ID = "q"
	request.Questions[0].Options[0].ID = strings.Repeat("x", maxOptionIDBytes+1)
	if err := ValidateRequest(request); err == nil {
		t.Fatal("option code longer than the native runtime limit was accepted")
	}
	request.Questions[0].Options[0].ID = "a"
	request.Questions[0].Options[0].Label = strings.Repeat("x", maxInstructions+1)
	if err := ValidateRequest(request); err == nil {
		t.Fatal("option label longer than the bounded prompt limit was accepted")
	}
}

func TestValidateRequestAcceptsNoulWithoutOptionsAndRejectsCustomNoulOptions(t *testing.T) {
	request := Request{Questions: []Question{{ID: "q1", Type: "noul", Instructions: "does it meet the criterion?"}}}
	if err := ValidateRequest(request); err != nil {
		t.Fatalf("noul defaults rejected: %v", err)
	}
	request.Questions[0].Options = []Option{{ID: "false", Label: "No"}, {ID: "yes", Label: "Yes"}}
	if err := ValidateRequest(request); err == nil {
		t.Fatal("custom noul candidate IDs accepted")
	}
}

func TestValidateRequestEnforcesAcceptedQuestionImageAndTypeContracts(t *testing.T) {
	request := Request{}
	for index := 0; index < maxQuestions; index++ {
		request.Questions = append(request.Questions, Question{
			ID:   "q" + string(rune('a'+index)),
			Type: "choice",
			Options: []Option{
				{ID: "left", Label: "Left"},
				{ID: "right", Label: "Right"},
			},
		})
	}
	for index := 0; index < maxImages; index++ {
		request.Images = append(request.Images, Image{Data: []byte{1}, MIME: "image/png"})
	}
	if err := ValidateRequest(request); err != nil {
		t.Fatalf("approved limits were rejected: %v", err)
	}
	request.Questions = append(request.Questions, Question{ID: "q-over-limit", Type: "noul"})
	if err := ValidateRequest(request); err == nil {
		t.Fatal("question limit exceeded without error")
	}
	request.Questions = request.Questions[:maxQuestions]
	request.Images = append(request.Images, Image{Data: []byte{1}, MIME: "image/png"})
	if err := ValidateRequest(request); err == nil {
		t.Fatal("image limit exceeded without error")
	}
}

func TestValidateRequestEnforcesScoreTypeNumericSemantics(t *testing.T) {
	first, second := 0.0, 0.0
	request := Request{Questions: []Question{{
		ID: "rating", Type: "score",
		Options: []Option{{ID: "low", Label: "Low", Value: &first}, {ID: "high", Label: "High", Value: &second}},
	}}}
	if err := ValidateRequest(request); err == nil {
		t.Fatal("duplicate effective score values accepted")
	}
	request.Questions[0].Options[1].Value = nil
	if err := ValidateRequest(request); err != nil {
		t.Fatalf("score values with index default rejected: %v", err)
	}
	first = 1
	if err := ValidateRequest(request); err == nil {
		t.Fatal("explicit score duplicated by index default accepted")
	}
	first = 0
	if err := ValidateRequest(request); err != nil {
		t.Fatalf("distinct score values with index default rejected: %v", err)
	}
}

func TestDecisionsConfigDefaultsDisabledAndCPUConsentOff(t *testing.T) {
	cfg := config.DecisionsConfig{}
	config.NormalizeDecisionsConfig(&cfg)
	if cfg.Enabled {
		t.Fatal("normalization enabled the service")
	}
	if cfg.CPUFallbackAllowed {
		t.Fatal("normalization enabled CPU fallback")
	}
	if cfg.Deployment != config.DecisionsDeploymentAuto {
		t.Fatalf("deployment = %q, want auto", cfg.Deployment)
	}
	if err := config.ValidateDecisionsConfig(cfg); err != nil {
		t.Fatalf("normalized config rejected: %v", err)
	}
	cfg.Deployment = "remote"
	if err := config.ValidateDecisionsConfig(cfg); err == nil {
		t.Fatal("unsupported deployment accepted")
	}
}
