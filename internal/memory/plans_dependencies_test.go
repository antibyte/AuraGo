package memory

import (
	"strings"
	"testing"
)

func TestPromoteNextPlanTaskBlocksUnreadableDependencies(t *testing.T) {
	stm := newTestPlansDB(t)
	plan, err := stm.CreatePlan("session-a", "Plan", "desc", "request", 2, samplePlanTasks())
	if err != nil {
		t.Fatalf("CreatePlan: %v", err)
	}
	if _, err := stm.db.Exec(`UPDATE plan_tasks SET depends_on_json = '{broken' WHERE plan_id = ? AND task_order = 2`, plan.ID); err != nil {
		t.Fatalf("corrupt dependencies: %v", err)
	}
	plan, err = stm.SetPlanStatus(plan.ID, PlanStatusActive, "")
	if err != nil {
		t.Fatalf("SetPlanStatus active: %v", err)
	}
	plan, err = stm.UpdatePlanTask(plan.ID, plan.Tasks[0].ID, PlanTaskCompleted, "inspection done", "")
	if err != nil {
		t.Fatalf("UpdatePlanTask: %v", err)
	}
	if plan.Tasks[1].Status != PlanTaskBlocked {
		t.Fatalf("task with unreadable dependencies status = %q, want %q", plan.Tasks[1].Status, PlanTaskBlocked)
	}
	if !strings.Contains(plan.Tasks[1].BlockerReason, "dependency data is unreadable") {
		t.Fatalf("blocker reason = %q, want the unreadable dependency data named", plan.Tasks[1].BlockerReason)
	}
	if plan.Status != PlanStatusBlocked {
		t.Fatalf("plan status = %q, want %q so the problem is visible", plan.Status, PlanStatusBlocked)
	}
}

func TestSplitPlanTaskRejectsUnreadableDependencies(t *testing.T) {
	stm := newTestPlansDB(t)
	plan, err := stm.CreatePlan("session-a", "Plan", "desc", "request", 2, samplePlanTasks())
	if err != nil {
		t.Fatalf("CreatePlan: %v", err)
	}
	if _, err := stm.db.Exec(`UPDATE plan_tasks SET depends_on_json = '{broken' WHERE plan_id = ? AND task_order = 2`, plan.ID); err != nil {
		t.Fatalf("corrupt dependencies: %v", err)
	}
	_, err = stm.SplitPlanTask(plan.ID, plan.Tasks[1].ID, []PlanTaskInput{{Title: "First half"}, {Title: "Second half"}})
	if err == nil || !strings.Contains(err.Error(), "dependencies are unreadable") {
		t.Fatalf("SplitPlanTask error = %v, want unreadable dependencies to be rejected", err)
	}
}
