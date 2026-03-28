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
		{Title: "Inspect current behavior", Kind: "reasoning", Acceptance: "State and root cause captured", Owner: "agent"},
		{Title: "Implement the fix", Kind: "tool", DependsOn: []string{"1"}, Acceptance: "Patch applied and verified", Owner: "agent"},
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
	if plan.Tasks[0].Acceptance == "" || plan.Tasks[0].Owner != "agent" {
		t.Fatalf("expected acceptance/owner fields to round-trip, got acceptance=%q owner=%q", plan.Tasks[0].Acceptance, plan.Tasks[0].Owner)
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

func TestAdvancePlanCompletesCurrentTask(t *testing.T) {
	stm := newTestPlansDB(t)

	plan, err := stm.CreatePlan("session-a", "Plan", "desc", "request", 2, samplePlanTasks())
	if err != nil {
		t.Fatalf("CreatePlan: %v", err)
	}
	plan, err = stm.SetPlanStatus(plan.ID, PlanStatusActive, "")
	if err != nil {
		t.Fatalf("SetPlanStatus active: %v", err)
	}

	plan, err = stm.AdvancePlan(plan.ID, "step one done")
	if err != nil {
		t.Fatalf("AdvancePlan: %v", err)
	}
	if plan.Tasks[0].Status != PlanTaskCompleted {
		t.Fatalf("first task status = %q, want %q", plan.Tasks[0].Status, PlanTaskCompleted)
	}
	if plan.Tasks[1].Status != PlanTaskInProgress {
		t.Fatalf("second task status = %q, want %q", plan.Tasks[1].Status, PlanTaskInProgress)
	}
	if plan.Tasks[0].ResultSummary != "step one done" {
		t.Fatalf("result summary = %q", plan.Tasks[0].ResultSummary)
	}
}

func TestPlanBlockerLifecycle(t *testing.T) {
	stm := newTestPlansDB(t)

	plan, err := stm.CreatePlan("session-a", "Plan", "desc", "request", 2, samplePlanTasks())
	if err != nil {
		t.Fatalf("CreatePlan: %v", err)
	}
	plan, err = stm.SetPlanStatus(plan.ID, PlanStatusActive, "")
	if err != nil {
		t.Fatalf("SetPlanStatus active: %v", err)
	}

	plan, err = stm.SetPlanTaskBlocker(plan.ID, plan.Tasks[0].ID, "waiting for user input")
	if err != nil {
		t.Fatalf("SetPlanTaskBlocker: %v", err)
	}
	if plan.Status != PlanStatusBlocked {
		t.Fatalf("plan status = %q, want %q", plan.Status, PlanStatusBlocked)
	}
	if plan.BlockedReason != "waiting for user input" {
		t.Fatalf("blocked reason = %q", plan.BlockedReason)
	}
	if plan.Tasks[0].Status != PlanTaskBlocked {
		t.Fatalf("task status = %q, want %q", plan.Tasks[0].Status, PlanTaskBlocked)
	}
	if plan.Tasks[0].BlockerReason != "waiting for user input" {
		t.Fatalf("task blocker reason = %q", plan.Tasks[0].BlockerReason)
	}

	prompt, err := stm.BuildSessionPlanPrompt("session-a")
	if err != nil {
		t.Fatalf("BuildSessionPlanPrompt blocked: %v", err)
	}
	for _, want := range []string{"Blocked: waiting for user input", "blocker: waiting for user input"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("blocked prompt missing %q:\n%s", want, prompt)
		}
	}

	plan, err = stm.ClearPlanTaskBlocker(plan.ID, plan.Tasks[0].ID, "user replied")
	if err != nil {
		t.Fatalf("ClearPlanTaskBlocker: %v", err)
	}
	if plan.Status != PlanStatusActive {
		t.Fatalf("plan status = %q, want %q", plan.Status, PlanStatusActive)
	}
	if plan.BlockedReason != "" {
		t.Fatalf("expected cleared blocked reason, got %q", plan.BlockedReason)
	}
	if plan.Tasks[0].Status != PlanTaskInProgress {
		t.Fatalf("task status after unblock = %q, want %q", plan.Tasks[0].Status, PlanTaskInProgress)
	}
}

func TestAttachPlanTaskArtifact(t *testing.T) {
	stm := newTestPlansDB(t)

	plan, err := stm.CreatePlan("session-a", "Plan", "desc", "request", 2, samplePlanTasks())
	if err != nil {
		t.Fatalf("CreatePlan: %v", err)
	}
	plan, err = stm.SetPlanStatus(plan.ID, PlanStatusActive, "")
	if err != nil {
		t.Fatalf("SetPlanStatus active: %v", err)
	}

	plan, err = stm.AttachPlanTaskArtifact(plan.ID, plan.Tasks[0].ID, PlanArtifact{
		Type:  "file",
		Label: "report",
		Value: "reports/result.md",
	})
	if err != nil {
		t.Fatalf("AttachPlanTaskArtifact: %v", err)
	}
	if len(plan.Tasks[0].Artifacts) != 1 {
		t.Fatalf("expected 1 artifact, got %d", len(plan.Tasks[0].Artifacts))
	}
	if plan.Tasks[0].Artifacts[0].Value != "reports/result.md" {
		t.Fatalf("artifact value = %q", plan.Tasks[0].Artifacts[0].Value)
	}

	prompt, err := stm.BuildSessionPlanPrompt("session-a")
	if err != nil {
		t.Fatalf("BuildSessionPlanPrompt artifacts: %v", err)
	}
	if !strings.Contains(prompt, "artifact: report = reports/result.md") {
		t.Fatalf("artifact prompt missing value:\n%s", prompt)
	}
}

func TestSplitPlanTaskCreatesNestedSubtasks(t *testing.T) {
	stm := newTestPlansDB(t)

	plan, err := stm.CreatePlan("session-a", "Plan", "desc", "request", 2, samplePlanTasks())
	if err != nil {
		t.Fatalf("CreatePlan: %v", err)
	}
	plan, err = stm.SetPlanStatus(plan.ID, PlanStatusActive, "")
	if err != nil {
		t.Fatalf("SetPlanStatus active: %v", err)
	}

	plan, err = stm.SplitPlanTask(plan.ID, plan.Tasks[0].ID, []PlanTaskInput{
		{Title: "Inspect logs"},
		{Title: "Verify assumptions"},
	})
	if err != nil {
		t.Fatalf("SplitPlanTask: %v", err)
	}
	if len(plan.Tasks) != 4 {
		t.Fatalf("expected 4 tasks after split, got %d", len(plan.Tasks))
	}
	if plan.Tasks[0].Status != PlanTaskSkipped {
		t.Fatalf("parent task status = %q, want %q", plan.Tasks[0].Status, PlanTaskSkipped)
	}
	if plan.Tasks[1].ParentTaskID != plan.Tasks[0].ID || plan.Tasks[2].ParentTaskID != plan.Tasks[0].ID {
		t.Fatalf("expected nested subtasks under %s", plan.Tasks[0].ID)
	}
	if plan.Tasks[1].Status != PlanTaskInProgress {
		t.Fatalf("first subtask status = %q, want %q", plan.Tasks[1].Status, PlanTaskInProgress)
	}

	prompt, err := stm.BuildSessionPlanPrompt("session-a")
	if err != nil {
		t.Fatalf("BuildSessionPlanPrompt split: %v", err)
	}
	if !strings.Contains(prompt, "Recommended next step: Continue the current task: Inspect logs.") {
		t.Fatalf("expected recommendation in prompt:\n%s", prompt)
	}
	if !strings.Contains(prompt, "  - [~] Inspect logs") {
		t.Fatalf("expected nested subtask in prompt:\n%s", prompt)
	}
}

func TestReorderPlanTasks(t *testing.T) {
	stm := newTestPlansDB(t)

	plan, err := stm.CreatePlan("session-a", "Plan", "desc", "request", 2, samplePlanTasks())
	if err != nil {
		t.Fatalf("CreatePlan: %v", err)
	}
	reordered, err := stm.ReorderPlanTasks(plan.ID, []string{plan.Tasks[1].ID, plan.Tasks[0].ID})
	if err != nil {
		t.Fatalf("ReorderPlanTasks: %v", err)
	}
	if reordered.Tasks[0].ID != plan.Tasks[1].ID {
		t.Fatalf("first task after reorder = %q, want %q", reordered.Tasks[0].ID, plan.Tasks[1].ID)
	}
}

func TestArchiveCompletedPlan(t *testing.T) {
	stm := newTestPlansDB(t)

	plan, err := stm.CreatePlan("session-a", "Plan", "desc", "request", 2, samplePlanTasks())
	if err != nil {
		t.Fatalf("CreatePlan: %v", err)
	}
	plan, err = stm.SetPlanStatus(plan.ID, PlanStatusActive, "")
	if err != nil {
		t.Fatalf("SetPlanStatus active: %v", err)
	}
	plan, err = stm.AdvancePlan(plan.ID, "done")
	if err != nil {
		t.Fatalf("AdvancePlan first: %v", err)
	}
	plan, err = stm.AdvancePlan(plan.ID, "done")
	if err != nil {
		t.Fatalf("AdvancePlan second: %v", err)
	}
	if plan.Status != PlanStatusCompleted {
		t.Fatalf("status = %q, want completed", plan.Status)
	}

	archived, err := stm.ArchivePlan(plan.ID)
	if err != nil {
		t.Fatalf("ArchivePlan: %v", err)
	}
	if !archived.Archived {
		t.Fatal("expected plan to be archived")
	}

	listed, err := stm.ListPlans("session-a", "all", 20, false)
	if err != nil {
		t.Fatalf("ListPlans hidden archive: %v", err)
	}
	if len(listed) != 0 {
		t.Fatalf("expected archived plan to be hidden, got %d", len(listed))
	}
	listed, err = stm.ListPlans("session-a", "all", 20, true)
	if err != nil {
		t.Fatalf("ListPlans include archive: %v", err)
	}
	if len(listed) != 1 || !listed[0].Archived {
		t.Fatalf("expected archived plan in list, got %+v", listed)
	}
}
