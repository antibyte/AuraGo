package server

import "testing"

func TestMissionResponseLooksIncomplete_NoToolsAndPlanningText(t *testing.T) {
	if !missionResponseLooksIncomplete("The user is asking me to check the current world news. Let me search first.", 0) {
		t.Fatal("expected planning-style mission response without tool activity to be flagged")
	}
}

func TestMissionResponseLooksIncomplete_NoToolsButFinishedResult(t *testing.T) {
	if missionResponseLooksIncomplete("Die Seite wurde bereits aktualisiert und ist unter https://example.test erreichbar.", 0) {
		t.Fatal("did not expect a concrete completed-result message to be flagged")
	}
}

func TestMissionResponseLooksIncomplete_WithToolActivity(t *testing.T) {
	if missionResponseLooksIncomplete("Let me verify the deploy.", 1) {
		t.Fatal("tool-backed mission response should not be flagged by the no-tool heuristic")
	}
}
