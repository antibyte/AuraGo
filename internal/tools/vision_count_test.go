package tools

import (
	"encoding/json"
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
	raw := `{"confirmed_count":2,"possible_additional_count":1,"other_vehicles":["van"],"items":[{"index":1,"type":"car","confidence":0.97,"confirmed":true},{"index":2,"type":"car","confidence":0.91,"confirmed":true}],"uncertainty":"one partially hidden candidate"}`
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
