package agent

import (
	"log/slog"
	"os"
	"strings"
	"testing"

	"aurago/internal/memory"
)

func newPlanRuntimeTestDB(t *testing.T) *memory.SQLiteMemory {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { stm.Close() })
	return stm
}

func TestRecordPlanToolProgressAddsEventAndArtifact(t *testing.T) {
	stm := newPlanRuntimeTestDB(t)
	plan, err := stm.CreatePlan("default", "Plan", "desc", "request", 2, []memory.PlanTaskInput{
		{Title: "Write report", Kind: "tool"},
	})
	if err != nil {
		t.Fatalf("CreatePlan: %v", err)
	}
	plan, err = stm.SetPlanStatus(plan.ID, memory.PlanStatusActive, "")
	if err != nil {
		t.Fatalf("SetPlanStatus active: %v", err)
	}

	tc := ToolCall{
		Action:      "filesystem",
		Operation:   "write_file",
		FilePath:    "reports/output.md",
		Destination: "reports/output.md",
	}
	result := `Tool Output: {"status":"success","message":"Wrote 123 bytes to reports/output.md","path":"reports/output.md"}`

	recordPlanToolProgress(stm, "default", tc, result, nil)

	plan, err = stm.GetPlan(plan.ID)
	if err != nil {
		t.Fatalf("GetPlan: %v", err)
	}
	if len(plan.Events) == 0 {
		t.Fatal("expected plan events after tool progress")
	}
	foundEvent := false
	for _, evt := range plan.Events {
		if strings.Contains(evt.Message, "filesystem") {
			foundEvent = true
			break
		}
	}
	if !foundEvent {
		t.Fatalf("expected filesystem event in %+v", plan.Events)
	}
	if len(plan.Tasks) == 0 || len(plan.Tasks[0].Artifacts) == 0 {
		t.Fatal("expected artifact on current task")
	}
	foundArtifact := false
	for _, artifact := range plan.Tasks[0].Artifacts {
		if artifact.Value == "reports/output.md" {
			foundArtifact = true
			break
		}
	}
	if !foundArtifact {
		t.Fatalf("expected report artifact in %+v", plan.Tasks[0].Artifacts)
	}
}
