package tools

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
)

func TestPrepareVisionPromptRequestsStructuredObjectCount(t *testing.T) {
	prompt, structured := PrepareVisionPrompt("Wie viele PKW sind auf dem Bild?")
	if !structured || !strings.Contains(prompt, visionCountSchemaMarker) {
		t.Fatalf("structured prompt not requested: %q", prompt)
	}
	unchanged, structured := PrepareVisionPrompt("Beschreibe das Wetter im Bild")
	if structured || unchanged != "Beschreibe das Wetter im Bild" {
		t.Fatalf("non-count prompt changed: %q, structured=%v", unchanged, structured)
	}
}

func TestNormalizeVisionAnalysisKeepsConsistentCountAndUncertaintySeparate(t *testing.T) {
	raw := `{"confirmed_count":2,"possible_additional_count":1,"other_vehicles":["van"],"items":[{"index":1,"type":"car","confidence":0.97,"confirmed":true},{"index":2,"type":"car","confidence":0.91,"confirmed":true},{"index":3,"type":"car","confidence":0.45,"confirmed":false}],"uncertainty":"one partially hidden candidate"}`
	normalized, handled := NormalizeVisionAnalysis("Wie viele PKW?", raw)
	if !handled {
		t.Fatal("count response was not handled")
	}
	var got map[string]interface{}
	if err := json.Unmarshal([]byte(normalized), &got); err != nil {
		t.Fatalf("normalized JSON: %v", err)
	}
	if got["confirmed_count"] != float64(2) || got["possible_additional_count"] != float64(1) || got["consistent"] != true {
		t.Fatalf("unexpected normalized count: %#v", got)
	}
}

func TestNormalizeVisionAnalysisAcceptsZeroAndNestedCounts(t *testing.T) {
	valid := `{"confirmed_count":0,"possible_additional_count":0,"other_vehicles":[],"items":[],"uncertainty":""}`
	for name, raw := range map[string]string{
		"zero":          valid,
		"nested object": `{"analysis":` + valid + `}`,
		"nested string": `{"analysis":` + strconv.Quote(valid) + `}`,
	} {
		t.Run(name, func(t *testing.T) {
			normalized, handled := NormalizeVisionAnalysis("How many cars?", raw)
			if !handled {
				t.Fatal("count response was not handled")
			}
			var got map[string]interface{}
			if err := json.Unmarshal([]byte(normalized), &got); err != nil {
				t.Fatalf("normalized JSON: %v", err)
			}
			if got["confirmed_count"] != float64(0) || got["possible_additional_count"] != float64(0) || got["consistent"] != true {
				t.Fatalf("unexpected normalized zero count: %#v", got)
			}
		})
	}
}

func TestNormalizeVisionAnalysisRejectsInvalidStructuredCounts(t *testing.T) {
	cases := map[string]string{
		"missing confirmed": `{"possible_additional_count":0,"items":[]}`,
		"null confirmed":    `{"confirmed_count":null,"possible_additional_count":0,"items":[]}`,
		"missing possible":  `{"confirmed_count":0,"items":[]}`,
		"null possible":     `{"confirmed_count":0,"possible_additional_count":null,"items":[]}`,
		"negative":          `{"confirmed_count":-1,"possible_additional_count":0,"items":[]}`,
		"fractional":        `{"confirmed_count":1.5,"possible_additional_count":0,"items":[]}`,
		"missing items":     `{"confirmed_count":0,"possible_additional_count":0}`,
		"missing flag":      `{"confirmed_count":1,"possible_additional_count":0,"items":[{"index":1}]}`,
		"duplicate index":   `{"confirmed_count":1,"possible_additional_count":1,"items":[{"index":1,"confirmed":true},{"index":1,"confirmed":false}]}`,
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			normalized, handled := NormalizeVisionAnalysis("Wie viele PKW?", raw)
			if !handled {
				t.Fatal("invalid count response was not handled")
			}
			var got map[string]interface{}
			if err := json.Unmarshal([]byte(normalized), &got); err != nil {
				t.Fatalf("normalized JSON: %v", err)
			}
			if got["confirmed_count"] != nil || got["consistent"] != false || got["status"] != visionInvalidCountStatus {
				t.Fatalf("unsafe fallback: %#v", got)
			}
		})
	}
}

func TestNormalizeVisionAnalysisDropsContradictoryItemList(t *testing.T) {
	raw := `{"confirmed_count":3,"possible_additional_count":1,"other_vehicles":[],"items":[{"index":1,"type":"car","confidence":0.9,"confirmed":true},{"index":2,"type":"car","confidence":0.8,"confirmed":true}],"uncertainty":""}`
	normalized, handled := NormalizeVisionAnalysis("How many cars?", raw)
	if !handled {
		t.Fatal("count response was not handled")
	}
	var got map[string]interface{}
	if err := json.Unmarshal([]byte(normalized), &got); err != nil {
		t.Fatalf("normalized JSON: %v", err)
	}
	if got["confirmed_count"] != float64(3) || got["consistent"] != false {
		t.Fatalf("confirmed count was not retained safely: %#v", got)
	}
	if _, exists := got["items"]; exists {
		t.Fatalf("contradictory item list was retained: %#v", got)
	}
	if _, exists := got["possible_additional_count"]; exists {
		t.Fatalf("contradictory possible count was retained: %#v", got)
	}
	if _, exists := got["other_vehicles"]; exists {
		t.Fatalf("contradictory vehicle list was retained: %#v", got)
	}
	if !strings.Contains(got["uncertainty"].(string), "conflicted") {
		t.Fatalf("missing contradiction uncertainty: %#v", got)
	}
}

func TestNormalizeVisionAnalysisCanonicalFailuresAreIdempotent(t *testing.T) {
	inputs := []string{
		`not valid JSON`,
		`{"confirmed_count":3,"possible_additional_count":1,"other_vehicles":[],"items":[{"index":1,"confirmed":true}],"uncertainty":"partly obscured"}`,
	}
	for _, input := range inputs {
		first, handled := NormalizeVisionAnalysis("How many cars?", input)
		if !handled {
			t.Fatal("first normalization was not handled")
		}
		second, handled := NormalizeVisionAnalysis("How many cars?", first)
		if !handled || second != first {
			t.Fatalf("normalization was not idempotent:\nfirst:  %s\nsecond: %s", first, second)
		}
		if strings.Count(second, visionConflictCountText) > 1 {
			t.Fatalf("conflict text was duplicated: %s", second)
		}
	}
}
