package server

import (
	"io"
	"log/slog"
	"testing"

	"aurago/internal/memory"
)

func TestMissionResponseLooksIncomplete_NoToolsAndPlanningText(t *testing.T) {
	if !assessMissionCompletion("The user is asking me to check the current world news. Let me search first.", missionToolResultCount{Known: true}).Suspicious {
		t.Fatal("expected planning-style mission response without tool activity to be flagged")
	}
}

func TestMissionResponseLooksIncomplete_NoToolsAndEmptyAfterReasoningStrip(t *testing.T) {
	if !assessMissionCompletion("", missionToolResultCount{Value: 2, Known: true}).Suspicious {
		t.Fatal("expected empty mission response without tool activity to be flagged")
	}
}

func TestMissionResponseLooksIncomplete_NoToolsAndMissionPlanLanguage(t *testing.T) {
	content := "According to the mission plan, I will first check the latest world news and then generate the mood image."
	if !assessMissionCompletion(content, missionToolResultCount{Known: true}).Suspicious {
		t.Fatal("expected mission-plan progress language without tool activity to be flagged")
	}
}

func TestMissionResponseLooksIncomplete_NoToolsAndToolFailureText(t *testing.T) {
	content := "The tools did not run:\n- `ddg_search`: \"Query is required\"\n- `generate_image`: \"'prompt' is required\""
	assessment := assessMissionCompletion(content, missionToolResultCount{Value: 2, Known: true})
	if !assessment.Suspicious || assessment.Reason != missionSuspiciousReasonToolError {
		t.Fatal("expected tool failure text without recorded tool activity to be flagged")
	}
}

func TestMissionResponseLooksIncomplete_NoToolsButFinishedResult(t *testing.T) {
	if assessMissionCompletion("Die Seite wurde bereits aktualisiert und ist unter https://example.test erreichbar.", missionToolResultCount{Known: true}).Suspicious {
		t.Fatal("did not expect a concrete completed-result message to be flagged")
	}
}

func TestMissionResponseLooksIncomplete_NoToolsButNoActionRequired(t *testing.T) {
	if assessMissionCompletion("No action is required right now.", missionToolResultCount{Known: true}).Suspicious {
		t.Fatal("did not expect a stable no-action result to be flagged")
	}
}

func TestMissionResponseLooksIncomplete_WithToolActivity(t *testing.T) {
	if assessMissionCompletion("Let me verify the deploy.", missionToolResultCount{Value: 1, Known: true}).Suspicious {
		t.Fatal("tool-backed mission response should not be flagged by the no-tool heuristic")
	}
}

func TestAssessMissionCompletion_LiveStyleFinalWithToolActivityIsAccepted(t *testing.T) {
	content := "## Mission abgeschlossen: KI News aktualisiert\n\nIch habe die lokalen KI News erfolgreich aktualisiert."
	if assessment := assessMissionCompletion(content, missionToolResultCount{Value: 52, Known: true}); assessment.Suspicious {
		t.Fatalf("tool-backed completion was rejected: %+v", assessment)
	}
}

func TestAssessMissionCompletion_UnknownToolCountDoesNotTriggerProgressHeuristic(t *testing.T) {
	if assessMissionCompletion("Let me verify the deploy.", missionToolResultCount{}).Suspicious {
		t.Fatal("unknown tool count must not be treated as confirmed zero")
	}
}

func TestAssessMissionCompletion_RawToolCallsAreAlwaysRejected(t *testing.T) {
	cases := []string{
		`<tool_call>{"name":"ddg_search","arguments":{"query":"news"}}</tool_call>`,
		`<function=ddg_search>{"query":"news"}</function>`,
		"```json\n{\"action\":\"ddg_search\",\"query\":\"news\"}\n```",
	}
	for _, content := range cases {
		assessment := assessMissionCompletion(content, missionToolResultCount{Value: 3, Known: true})
		if !assessment.Suspicious || assessment.Reason != missionSuspiciousReasonRawToolCall {
			t.Fatalf("raw tool response was not rejected: %q (%+v)", content, assessment)
		}
	}
}

func TestResetMissionSessionToolResultBaselineCountsOnlyNewRun(t *testing.T) {
	stm, err := memory.NewSQLiteMemory(":memory:", slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	defer stm.Close()

	const sessionID = "mission-regression"
	for i := 0; i < 5; i++ {
		if _, err := stm.InsertMessage(sessionID, "tool", "old result", false, true); err != nil {
			t.Fatalf("insert old tool result: %v", err)
		}
	}
	baseline, err := resetMissionSessionToolResultBaseline(stm, sessionID)
	if err != nil {
		t.Fatalf("reset mission baseline: %v", err)
	}
	if !baseline.Known || baseline.Value != 0 {
		t.Fatalf("baseline = %+v, want known zero after successful clear", baseline)
	}
	for i := 0; i < 2; i++ {
		if _, err := stm.InsertMessage(sessionID, "tool", "new result", false, true); err != nil {
			t.Fatalf("insert new tool result: %v", err)
		}
	}
	delta := missionToolResultDelta(baseline, readMissionToolResultCount(stm, sessionID))
	if !delta.Known || delta.Value != 2 {
		t.Fatalf("tool result delta = %+v, want current run count 2", delta)
	}
}

func TestReadMissionToolResultCount_DBFailureIsUnknown(t *testing.T) {
	stm, err := memory.NewSQLiteMemory(":memory:", slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	if err := stm.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if count := readMissionToolResultCount(stm, "mission"); count.Known {
		t.Fatalf("count = %+v, want unknown after DB read failure", count)
	}
}
