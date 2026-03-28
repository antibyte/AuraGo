package memory

import (
	"log/slog"
	"os"
	"strings"
	"testing"
)

func newTestPlansDB(t *testing.T) *SQLiteMemory {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	stm, err := NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { stm.Close() })
	return stm
}

func samplePlanTasks() []PlanTaskInput {
	return []PlanTaskInput{
		{Title: "Inspect current behavior", Kind: "reasoning"},
		{Title: "Implement the fix", Kind: "tool", DependsOn: []string{"1"}},
	}
}

func TestCreatePlanRejectsSecondUnfinishedPlan(t *testing.T) {
	stm := newTestPlansDB(t)

	_, err := stm.CreatePlan("session-a", "First plan", "desc", "request", 2, samplePlanTasks())
	if err != nil {
		t.Fatalf("CreatePlan first: %v", err)
	}

	_, err = stm.CreatePlan("session-a", "Second plan", "desc", "request", 2, samplePlanTasks())
	if err == nil || !strings.Contains(err.Error(), "unfinished plan") {
		t.Fatalf("expected unfinished plan error, got %v", err)
	}

	_, err = stm.CreatePlan("session-b", "Other session plan", "desc", "request", 2, samplePlanTasks())
	if err != nil {
		t.Fatalf("CreatePlan other session: %v", err)
	}
}

func TestSetPlanStatusActivePromotesFirstTask(t *testing.T) {
	stm := newTestPlansDB(t)

	plan, err := stm.CreatePlan("session-a", "Plan", "desc", "request", 2, samplePlanTasks())
	if err != nil {
		t.Fatalf("CreatePlan: %v", err)
	}

	plan, err = stm.SetPlanStatus(plan.ID, PlanStatusActive, "")
	if err != nil {
		t.Fatalf("SetPlanStatus active: %v", err)
	}

	if plan.Status != PlanStatusActive {
		t.Fatalf("status = %q, want %q", plan.Status, PlanStatusActive)
	}
	if plan.CurrentTask == "" {
		t.Fatal("expected current task after activation")
	}
	if len(plan.Tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(plan.Tasks))
	}
	if plan.Tasks[0].Status != PlanTaskInProgress {
		t.Fatalf("first task status = %q, want %q", plan.Tasks[0].Status, PlanTaskInProgress)
	}
	if plan.Tasks[1].Status != PlanTaskPending {
		t.Fatalf("second task status = %q, want %q", plan.Tasks[1].Status, PlanTaskPending)
	}
}

func TestUpdatePlanTaskAdvancesAndCompletesPlan(t *testing.T) {
	stm := newTestPlansDB(t)

	plan, err := stm.CreatePlan("session-a", "Plan", "desc", "request", 2, samplePlanTasks())
	if err != nil {
		t.Fatalf("CreatePlan: %v", err)
	}
	plan, err = stm.SetPlanStatus(plan.ID, PlanStatusActive, "")
	if err != nil {
		t.Fatalf("SetPlanStatus active: %v", err)
	}

	prompt, err := stm.BuildSessionPlanPrompt("session-a")
	if err != nil {
		t.Fatalf("BuildSessionPlanPrompt: %v", err)
	}
	for _, want := range []string{"Plan: Plan", "Inspect current behavior", "Implement the fix"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q:\n%s", want, prompt)
		}
	}

	plan, err = stm.UpdatePlanTask(plan.ID, plan.Tasks[0].ID, PlanTaskCompleted, "inspection done", "")
	if err != nil {
		t.Fatalf("UpdatePlanTask first complete: %v", err)
	}
	if plan.Tasks[0].Status != PlanTaskCompleted {
		t.Fatalf("first task status = %q, want %q", plan.Tasks[0].Status, PlanTaskCompleted)
	}
	if plan.Tasks[1].Status != PlanTaskInProgress {
		t.Fatalf("second task status = %q, want %q", plan.Tasks[1].Status, PlanTaskInProgress)
	}
	if plan.CurrentTask != "Implement the fix" {
		t.Fatalf("current task = %q, want %q", plan.CurrentTask, "Implement the fix")
	}

	plan, err = stm.UpdatePlanTask(plan.ID, plan.Tasks[1].ID, PlanTaskCompleted, "fix applied", "")
	if err != nil {
		t.Fatalf("UpdatePlanTask second complete: %v", err)
	}
	if plan.Status != PlanStatusCompleted {
		t.Fatalf("plan status = %q, want %q", plan.Status, PlanStatusCompleted)
	}
	active, err := stm.GetSessionPlan("session-a")
	if err != nil {
		t.Fatalf("GetSessionPlan: %v", err)
	}
	if active != nil {
		t.Fatalf("expected no unfinished session plan after completion, got %s", active.ID)
	}
}

func TestPlanMutationRequiresExistingPlan(t *testing.T) {
	stm := newTestPlansDB(t)

	if err := stm.AppendPlanNote("missing", "note"); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not found from AppendPlanNote, got %v", err)
	}
	if _, err := stm.SetPlanStatus("missing", PlanStatusActive, ""); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not found from SetPlanStatus, got %v", err)
	}
	if _, err := stm.UpdatePlanTask("missing", "task", PlanTaskCompleted, "", ""); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not found from UpdatePlanTask, got %v", err)
	}
	if err := stm.DeletePlan("missing"); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not found from DeletePlan, got %v", err)
	}
}
