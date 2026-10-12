package decisions

import (
	"strings"
	"testing"
)

func TestBuildPromptMapsStableCandidateCodesAndImageMarkers(t *testing.T) {
	question := Question{
		ID: "route", Type: "choice", Instructions: "choose a route",
		Options: []Option{{ID: "route_north", Label: "North"}, {ID: "route_south", Label: "South"}},
	}
	options := optionsForQuestion(question)
	prompt, err := buildPrompt(map[string]any{"hazard": "bridge"}, question, options, "<|image|>", 2)
	if err != nil {
		t.Fatal(err)
	}
	candidates, err := candidatesForOptions(options)
	if err != nil {
		t.Fatal(err)
	}
	if candidates[0].Code != "route_north" || candidates[0].TokenText != "A" || candidates[1].TokenText != "B" {
		t.Fatalf("candidate mapping = %#v", candidates)
	}
	want := "Task: choose a route\nState: {\"hazard\":\"bridge\"}\n<|image|>\n<|image|>\nOptions: [A: North, B: South]\n### Decision:\n"
	if prompt != want || strings.Count(prompt, "<|image|>") != 2 {
		t.Fatalf("prompt = %q, want exact JEFF template %q", prompt, want)
	}
}

func TestBuildPromptUsesTypeWhenTaskInstructionsAreBlank(t *testing.T) {
	question := Question{ID: "score", Type: "score", Options: []Option{{ID: "low", Label: "Low"}, {ID: "high", Label: "High"}}}
	prompt, err := buildPrompt(map[string]any{"balance": 7}, question, question.Options, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	want := "Task: score\nState: {\"balance\":7}\nOptions: [A: Low, B: High]\n### Decision:\n"
	if prompt != want {
		t.Fatalf("prompt = %q, want %q", prompt, want)
	}
}

func TestBuildPromptMatchesPublishedJEFFRoutingExample(t *testing.T) {
	question := Question{
		ID: "routing", Type: "choice", Instructions: "routing",
		Options: []Option{{ID: "execute", Label: "Execute tool"}, {ID: "chat", Label: "Chat directly"}},
	}
	prompt, err := buildPrompt("Query user account balance", question, question.Options, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	want := "Task: routing\nState: Query user account balance\nOptions: [A: Execute tool, B: Chat directly]\n### Decision:\n"
	if prompt != want {
		t.Fatalf("prompt = %q, want %q", prompt, want)
	}
}

func TestAnswerFromScoresReturnsOnlyTypeSpecificField(t *testing.T) {
	cases := []struct {
		name       string
		question   Question
		wantChoice string
		wantScore  bool
		wantNoul   bool
	}{
		{
			name:       "choice",
			question:   Question{ID: "q", Type: "choice", Options: []Option{{ID: "left", Label: "Left"}, {ID: "right", Label: "Right"}}},
			wantChoice: "Left",
		},
		{
			name:      "score",
			question:  Question{ID: "q", Type: "score", Options: []Option{{ID: "low", Label: "Low"}, {ID: "high", Label: "High", Value: floatPointer(5)}}},
			wantScore: true,
		},
		{
			name:     "noul",
			question: Question{ID: "q", Type: "noul"},
			wantNoul: true,
		},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			options := optionsForQuestion(test.question)
			candidates, err := candidatesForOptions(options)
			if err != nil {
				t.Fatal(err)
			}
			scores := []scoredCandidate{
				{Code: options[0].ID, TokenID: 11, Logit: 1},
				{Code: options[1].ID, TokenID: 12, Logit: 0},
			}
			answer, err := answerFromScores(test.question, options, candidates, scores)
			if err != nil {
				t.Fatal(err)
			}
			if answer.Decision != options[0].ID || answer.Confidence <= 0.5 {
				t.Fatalf("decision/confidence = %q/%f", answer.Decision, answer.Confidence)
			}
			if (answer.Choice != "") != (test.wantChoice != "") || (answer.Score != nil) != test.wantScore || (answer.Noul != nil) != test.wantNoul {
				t.Fatalf("type-specific answer fields mismatch: %#v", answer)
			}
			if test.wantChoice != "" && answer.Choice != test.wantChoice {
				t.Fatalf("choice = %q, want %q", answer.Choice, test.wantChoice)
			}
			if test.wantNoul && (*answer.Noul <= 0 || *answer.Noul >= 1) {
				t.Fatalf("noul probability = %v", *answer.Noul)
			}
		})
	}
}

func floatPointer(value float64) *float64 {
	return &value
}
