package server

import (
	"time"

	"aurago/internal/memory"
	"aurago/internal/planner"
)

func buildPlannerOverview(s *Server) map[string]interface{} {
	summary := map[string]interface{}{
		"appointments": map[string]int{
			"total":     0,
			"upcoming":  0,
			"completed": 0,
			"cancelled": 0,
		},
		"todos": map[string]int{
			"total":       0,
			"open":        0,
			"in_progress": 0,
			"done":        0,
			"overdue":     0,
		},
		"plans": map[string]interface{}{
			"total":        0,
			"active":       0,
			"blocked":      0,
			"progress_pct": 0,
			"current_task": "",
			"status":       "",
		},
	}

	if s.PlannerDB != nil {
		if appointments, err := planner.ListAppointments(s.PlannerDB, "", "all"); err == nil {
			appointmentSummary := summary["appointments"].(map[string]int)
			appointmentSummary["total"] = len(appointments)
			for _, appointment := range appointments {
				switch appointment.Status {
				case "completed":
					appointmentSummary["completed"]++
				case "cancelled":
					appointmentSummary["cancelled"]++
				default:
					appointmentSummary["upcoming"]++
				}
			}
		}

		if todos, err := planner.ListTodos(s.PlannerDB, "", "all"); err == nil {
			todoSummary := summary["todos"].(map[string]int)
			todoSummary["total"] = len(todos)
			now := time.Now()
			for _, todo := range todos {
				switch todo.Status {
				case "in_progress":
					todoSummary["in_progress"]++
				case "done":
					todoSummary["done"]++
				default:
					todoSummary["open"]++
				}
				if todo.Status != "done" && todo.DueDate != "" {
					if dueAt, err := time.Parse(time.RFC3339, todo.DueDate); err == nil && dueAt.Before(now) {
						todoSummary["overdue"]++
					}
				}
			}
		}
	}

	if s.ShortTermMem != nil {
		planSummary := summary["plans"].(map[string]interface{})
		if plans, err := s.ShortTermMem.ListPlans("default", "all", 20, false); err == nil {
			planSummary["total"] = len(plans)
			activeCount := 0
			blockedCount := 0
			for _, plan := range plans {
				switch plan.Status {
				case memory.PlanStatusDraft, memory.PlanStatusActive, memory.PlanStatusPaused, memory.PlanStatusBlocked:
					activeCount++
				}
				if plan.Status == memory.PlanStatusBlocked {
					blockedCount++
				}
			}
			planSummary["active"] = activeCount
			planSummary["blocked"] = blockedCount
		}
		if currentPlan, err := s.ShortTermMem.GetSessionPlan("default"); err == nil && currentPlan != nil {
			planSummary["progress_pct"] = currentPlan.ProgressPct
			planSummary["current_task"] = currentPlan.CurrentTask
			planSummary["status"] = currentPlan.Status
		}
	}

	return summary
}