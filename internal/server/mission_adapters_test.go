package server

import "testing"

func TestMissionResponseLooksIncomplete_NoToolsAndPlanningText(t *testing.T) {
	if !missionResponseLooksIncomplete("The user is asking me to check the current world news. Let me search first.", 0) {
		t.Fatal("expected planning-style mission response without tool activity to be flagged")
	}
}

func TestMissionResponseLooksIncomplete_NoToolsAndEmptyAfterReasoningStrip(t *testing.T) {
	if !missionResponseLooksIncomplete("", 0) {
		t.Fatal("expected empty mission response without tool activity to be flagged")
	}
}

func TestMissionResponseLooksIncomplete_NoToolsAndMissionPlanLanguage(t *testing.T) {
	content := "According to the mission plan, I will first check the latest world news and then generate the mood image."
	if !missionResponseLooksIncomplete(content, 0) {
		t.Fatal("expected mission-plan progress language without tool activity to be flagged")
	}
}

func TestMissionResponseLooksIncomplete_NoToolsAndToolFailureText(t *testing.T) {
	content := "The tools did not run:\n- `ddg_search`: \"Query is required\"\n- `generate_image`: \"'prompt' is required\""
	if !missionResponseLooksIncomplete(content, 0) {
		t.Fatal("expected tool failure text without recorded tool activity to be flagged")
	}
}

func TestMissionResponseLooksIncomplete_NoToolsButFinishedResult(t *testing.T) {
	if missionResponseLooksIncomplete("Die Seite wurde bereits aktualisiert und ist unter https://example.test erreichbar.", 0) {
		t.Fatal("did not expect a concrete completed-result message to be flagged")
	}
}

func TestMissionResponseLooksIncomplete_NoToolsButNoActionRequired(t *testing.T) {
	if missionResponseLooksIncomplete("No action is required right now.", 0) {
		t.Fatal("did not expect a stable no-action result to be flagged")
	}
}

func TestMissionResponseLooksIncomplete_WithToolActivity(t *testing.T) {
	if missionResponseLooksIncomplete("Let me verify the deploy.", 1) {
		t.Fatal("tool-backed mission response should not be flagged by the no-tool heuristic")
	}
}
