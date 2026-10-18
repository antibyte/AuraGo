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
	for _, raw := range []string{"{broken", "null", `{}`, `"task"`, ""} {
		t.Run(raw, func(t *testing.T) {
			stm := newTestPlansDB(t)
			plan, err := stm.CreatePlan("session-a", "Plan", "desc", "request", 2, samplePlanTasks())
			if err != nil {
				t.Fatal(err)
			}
			if _, err := stm.db.Exec(`UPDATE plan_tasks SET depends_on_json = ? WHERE plan_id = ? AND task_order = 2`, raw, plan.ID); err != nil {
				t.Fatal(err)
			}
			_, err = stm.SplitPlanTask(plan.ID, plan.Tasks[1].ID, []PlanTaskInput{{Title: "First half"}, {Title: "Second half"}})
			if err == nil || !strings.Contains(err.Error(), "dependencies are unreadable") {
				t.Fatalf("SplitPlanTask error = %v, want unreadable dependencies to be rejected", err)
			}
			unchanged, err := stm.GetPlan(plan.ID)
			if err != nil || len(unchanged.Tasks) != len(plan.Tasks) {
				t.Fatalf("failed split changed tasks: %+v %v", unchanged, err)
			}
		})
	}
}

func TestCreatePlanRejectsInvalidDependencyGraphs(t *testing.T) {
	for _, tc := range []struct {
		name  string
		tasks []PlanTaskInput
	}{
		{"missing", []PlanTaskInput{{Title: "a", DependsOn: []string{"5"}}}},
		{"trailing", []PlanTaskInput{{Title: "a", DependsOn: []string{"1garbage"}}}},
		{"self", []PlanTaskInput{{Title: "a", DependsOn: []string{"1"}}}},
		{"cycle", []PlanTaskInput{{Title: "a", DependsOn: []string{"2"}}, {Title: "b", DependsOn: []string{"1"}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newTestPlansDB(t)
			if _, err := s.CreatePlan("s", "plan", "", "", 2, tc.tasks); err == nil {
				t.Fatal("invalid dependencies accepted")
			}
			var n int
			if err := s.db.QueryRow(`SELECT count(*) FROM plans`).Scan(&n); err != nil || n != 0 {
				t.Fatalf("partial plan left behind: %d %v", n, err)
			}
		})
	}
}

func TestLegacyInvalidDependencyGraphsBecomeVisibleBlockers(t *testing.T) {
	for _, kind := range []string{"missing", "cycle", "null", "object", "string", "malformed", "empty"} {
		t.Run(kind, func(t *testing.T) {
			s := newTestPlansDB(t)
			plan, err := s.CreatePlan("s", "plan", "", "", 2, samplePlanTasks())
			if err != nil {
				t.Fatal(err)
			}
			dep := `["missing"]`
			switch kind {
			case "cycle":
				dep = `["` + plan.Tasks[1].ID + `"]`
			case "null":
				dep = "null"
			case "object":
				dep = `{}`
			case "string":
				dep = `"task"`
			case "malformed":
				dep = "{broken"
			case "empty":
				dep = ""
			}
			if _, err := s.db.Exec(`UPDATE plan_tasks SET depends_on_json=? WHERE id=?`, dep, plan.Tasks[0].ID); err != nil {
				t.Fatal(err)
			}
			plan, err = s.SetPlanStatus(plan.ID, PlanStatusActive, "")
			if err != nil {
				t.Fatal(err)
			}
			if plan.Status != PlanStatusBlocked || plan.BlockedReason != planTaskDependencyBlocker {
				t.Fatalf("invalid legacy plan silently stuck: %+v", plan)
			}
			for _, task := range plan.Tasks {
				if task.Status == PlanTaskInProgress {
					t.Fatalf("invalid dependency plan started task: %+v", task)
				}
			}
		})
	}
}

func TestLegacyEmptyDependencyArrayRemainsRunnable(t *testing.T) {
	s := newTestPlansDB(t)
	plan, err := s.CreatePlan("s", "plan", "", "", 2, samplePlanTasks())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.db.Exec(`UPDATE plan_tasks SET depends_on_json='[]' WHERE plan_id=?`, plan.ID); err != nil {
		t.Fatal(err)
	}
	plan, err = s.SetPlanStatus(plan.ID, PlanStatusActive, "")
	if err != nil || plan.Status != PlanStatusActive || plan.Tasks[0].Status != PlanTaskInProgress {
		t.Fatalf("empty array blocked: %+v %v", plan, err)
	}
	plan, err = s.UpdatePlanTask(plan.ID, plan.Tasks[0].ID, PlanTaskCompleted, "done", "")
	if err != nil || plan.Status != PlanStatusActive || plan.Tasks[1].Status != PlanTaskInProgress {
		t.Fatalf("empty array prevented promotion: %+v %v", plan, err)
	}
}

func TestSkippedTaskWithUnreadableLegacyDependenciesCanResumePlan(t *testing.T) {
	s := newTestPlansDB(t)
	plan, err := s.CreatePlan("s", "plan", "", "", 2, samplePlanTasks())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`UPDATE plan_tasks SET depends_on_json='null' WHERE id=?`, plan.Tasks[0].ID); err != nil {
		t.Fatal(err)
	}
	plan, err = s.SetPlanStatus(plan.ID, PlanStatusActive, "")
	if err != nil || plan.Tasks[0].Status != PlanTaskBlocked {
		t.Fatalf("invalid legacy task was not visibly blocked: %+v %v", plan, err)
	}
	plan, err = s.UpdatePlanTask(plan.ID, plan.Tasks[0].ID, PlanTaskSkipped, "skip corrupt legacy task", "")
	if err != nil {
		t.Fatal(err)
	}
	plan, err = s.SetPlanStatus(plan.ID, PlanStatusActive, "resume after skip")
	if err != nil {
		t.Fatal(err)
	}
	if plan.Status != PlanStatusActive || plan.Tasks[0].Status != PlanTaskSkipped || plan.Tasks[1].Status != PlanTaskInProgress {
		t.Fatalf("skipped terminal leaf prevented plan resumption: status=%s tasks=%+v", plan.Status, plan.Tasks)
	}
}

func TestFailedDependencyDoesNotReleaseDependentTask(t *testing.T) {
	s := newTestPlansDB(t)
	plan, err := s.CreatePlan("s", "plan", "", "", 2, samplePlanTasks())
	if err != nil {
		t.Fatal(err)
	}
	plan, err = s.SetPlanStatus(plan.ID, PlanStatusActive, "")
	if err != nil {
		t.Fatal(err)
	}
	plan, err = s.UpdatePlanTask(plan.ID, plan.Tasks[0].ID, PlanTaskFailed, "step failed", "failure")
	if err != nil {
		t.Fatal(err)
	}
	if plan.Tasks[0].Status != PlanTaskFailed || plan.Tasks[1].Status != PlanTaskPending {
		t.Fatalf("failed prerequisite incorrectly released dependent: %+v", plan.Tasks)
	}
}

func TestAutomaticDependencyBlockPreservesTerminalFailure(t *testing.T) {
	s := newTestPlansDB(t)
	plan, err := s.CreatePlan("s", "plan", "", "", 2, samplePlanTasks())
	if err != nil {
		t.Fatal(err)
	}
	plan, err = s.SetPlanStatus(plan.ID, PlanStatusActive, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`UPDATE plan_tasks SET depends_on_json='{broken' WHERE id=?`, plan.Tasks[0].ID); err != nil {
		t.Fatal(err)
	}
	plan, err = s.UpdatePlanTask(plan.ID, plan.Tasks[0].ID, PlanTaskFailed, "failed", "failure")
	if err != nil {
		t.Fatal(err)
	}
	plan, err = s.SetPlanStatus(plan.ID, PlanStatusActive, "retry plan")
	if err != nil {
		t.Fatal(err)
	}
	if plan.Tasks[0].Status != PlanTaskFailed || plan.Status != PlanStatusBlocked {
		t.Fatalf("dependency blocker overwrote terminal failure or failed to expose plan error: status=%s task=%+v", plan.Status, plan.Tasks[0])
	}
}

func TestReopeningTaskRevalidatesDependenciesAndDirectStart(t *testing.T) {
	t.Run("reopen with corrupt dependencies is blocked", func(t *testing.T) {
		s := newTestPlansDB(t)
		plan, err := s.CreatePlan("s", "plan", "", "", 2, samplePlanTasks())
		if err != nil {
			t.Fatal(err)
		}
		plan, err = s.SetPlanStatus(plan.ID, PlanStatusActive, "")
		if err != nil {
			t.Fatal(err)
		}
		for i := range plan.Tasks {
			plan, err = s.UpdatePlanTask(plan.ID, plan.Tasks[i].ID, PlanTaskCompleted, "done", "")
			if err != nil {
				t.Fatal(err)
			}
		}
		if plan.Status != PlanStatusCompleted {
			t.Fatalf("plan did not complete: %+v", plan)
		}
		if _, err := s.db.Exec(`UPDATE plan_tasks SET depends_on_json='null' WHERE id=?`, plan.Tasks[0].ID); err != nil {
			t.Fatal(err)
		}
		plan, err = s.UpdatePlanTask(plan.ID, plan.Tasks[0].ID, PlanTaskPending, "retry", "")
		if err != nil {
			t.Fatal(err)
		}
		if plan.Status != PlanStatusBlocked || plan.Tasks[0].Status != PlanTaskBlocked {
			t.Fatalf("reopened task bypassed invalid dependency validation: %+v", plan)
		}
	})

	t.Run("direct start cannot bypass dependency status", func(t *testing.T) {
		s := newTestPlansDB(t)
		plan, err := s.CreatePlan("s", "plan", "", "", 2, samplePlanTasks())
		if err != nil {
			t.Fatal(err)
		}
		plan, err = s.SetPlanStatus(plan.ID, PlanStatusActive, "")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := s.UpdatePlanTask(plan.ID, plan.Tasks[1].ID, PlanTaskInProgress, "", ""); err == nil || !strings.Contains(err.Error(), "dependencies") {
			t.Fatalf("direct start error = %v, want unmet dependency rejection", err)
		}
		plan, err = s.GetPlan(plan.ID)
		if err != nil || plan.Tasks[1].Status != PlanTaskPending {
			t.Fatalf("rejected direct start mutated dependent task: %+v %v", plan, err)
		}
	})
}

func TestSetPlanTaskBlockerDoesNotOverwriteTerminalTask(t *testing.T) {
	s := newTestPlansDB(t)
	plan, err := s.CreatePlan("s", "plan", "", "", 2, samplePlanTasks())
	if err != nil {
		t.Fatal(err)
	}
	plan, err = s.SetPlanStatus(plan.ID, PlanStatusActive, "")
	if err != nil {
		t.Fatal(err)
	}
	plan, err = s.UpdatePlanTask(plan.ID, plan.Tasks[0].ID, PlanTaskCompleted, "done", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetPlanTaskBlocker(plan.ID, plan.Tasks[0].ID, "late blocker"); err == nil {
		t.Fatal("SetPlanTaskBlocker accepted a completed task")
	}
	plan, err = s.GetPlan(plan.ID)
	if err != nil || plan.Tasks[0].Status != PlanTaskCompleted || plan.Status != PlanStatusActive {
		t.Fatalf("terminal blocker attempt changed task or plan: %+v %v", plan, err)
	}
}
