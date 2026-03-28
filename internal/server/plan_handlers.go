package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"aurago/internal/memory"
)

type planActionRequest struct {
	Status          string                   `json:"status"`
	Note            string                   `json:"note"`
	Result          string                   `json:"result"`
	Error           string                   `json:"error"`
	Reason          string                   `json:"reason"`
	IncludeArchived bool                     `json:"include_archived"`
	TaskIDs         []string                 `json:"task_ids"`
	Items           []map[string]interface{} `json:"items"`
}

func writePlanJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func handlePlansList(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if s.ShortTermMem == nil {
			writePlanJSON(w, http.StatusServiceUnavailable, map[string]interface{}{"status": "error", "message": "Plan storage not available"})
			return
		}
		sessionID := strings.TrimSpace(r.URL.Query().Get("session_id"))
		if sessionID == "" {
			sessionID = "default"
		}
		statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))
		if statusFilter == "" {
			statusFilter = "all"
		}
		includeArchived := strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("include_archived")), "true") ||
			strings.TrimSpace(r.URL.Query().Get("include_archived")) == "1"
		limit := 20
		if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
			if v, err := strconv.Atoi(raw); err == nil {
				limit = v
			}
		}
		plans, err := s.ShortTermMem.ListPlans(sessionID, statusFilter, limit, includeArchived)
		if err != nil {
			s.Logger.Error("Failed to list plans", "session_id", sessionID, "error", err)
			writePlanJSON(w, http.StatusInternalServerError, map[string]interface{}{"status": "error", "message": err.Error()})
			return
		}
		writePlanJSON(w, http.StatusOK, map[string]interface{}{
			"status":           "ok",
			"session_id":       sessionID,
			"include_archived": includeArchived,
			"plans":            plans,
		})
	}
}

func planTaskInputsFromRequestItems(items []map[string]interface{}) []memory.PlanTaskInput {
	inputs := make([]memory.PlanTaskInput, 0, len(items))
	for _, item := range items {
		input := memory.PlanTaskInput{}
		if v, ok := item["title"].(string); ok {
			input.Title = strings.TrimSpace(v)
		}
		if v, ok := item["description"].(string); ok {
			input.Description = strings.TrimSpace(v)
		}
		if v, ok := item["kind"].(string); ok {
			input.Kind = strings.TrimSpace(v)
		}
		if v, ok := item["tool_name"].(string); ok {
			input.ToolName = strings.TrimSpace(v)
		}
		if v, ok := item["acceptance_criteria"].(string); ok {
			input.Acceptance = strings.TrimSpace(v)
		}
		if v, ok := item["owner"].(string); ok {
			input.Owner = strings.TrimSpace(v)
		}
		if v, ok := item["parent_task_id"].(string); ok {
			input.ParentTaskID = strings.TrimSpace(v)
		}
		if v, ok := item["tool_args"].(map[string]interface{}); ok {
			input.ToolArgs = v
		}
		switch deps := item["depends_on"].(type) {
		case []interface{}:
			for _, dep := range deps {
				input.DependsOn = append(input.DependsOn, strings.TrimSpace(fmt.Sprint(dep)))
			}
		case []string:
			input.DependsOn = append(input.DependsOn, deps...)
		}
		inputs = append(inputs, input)
	}
	return inputs
}

func planTaskIDsFromRequest(req planActionRequest) []string {
	if len(req.TaskIDs) > 0 {
		return req.TaskIDs
	}
	ids := make([]string, 0, len(req.Items))
	for _, item := range req.Items {
		if v, ok := item["task_id"].(string); ok && strings.TrimSpace(v) != "" {
			ids = append(ids, strings.TrimSpace(v))
		}
	}
	return ids
}

func handlePlanByID(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.ShortTermMem == nil {
			writePlanJSON(w, http.StatusServiceUnavailable, map[string]interface{}{"status": "error", "message": "Plan storage not available"})
			return
		}
		path := strings.TrimPrefix(r.URL.Path, "/api/plans/")
		parts := strings.Split(path, "/")
		if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" {
			writePlanJSON(w, http.StatusBadRequest, map[string]interface{}{"status": "error", "message": "plan id required"})
			return
		}
		planID := strings.TrimSpace(parts[0])

		if len(parts) == 1 {
			switch r.Method {
			case http.MethodGet:
				plan, err := s.ShortTermMem.GetPlan(planID)
				if err != nil {
					writePlanJSON(w, http.StatusNotFound, map[string]interface{}{"status": "error", "message": err.Error()})
					return
				}
				writePlanJSON(w, http.StatusOK, map[string]interface{}{"status": "ok", "plan": plan})
				return
			case http.MethodDelete:
				if err := s.ShortTermMem.DeletePlan(planID); err != nil {
					writePlanJSON(w, http.StatusNotFound, map[string]interface{}{"status": "error", "message": err.Error()})
					return
				}
				writePlanJSON(w, http.StatusOK, map[string]interface{}{"status": "ok", "message": "Plan deleted"})
				return
			default:
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}
		}

		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req planActionRequest
		if r.Body != nil {
			_ = json.NewDecoder(r.Body).Decode(&req)
		}

		switch {
		case len(parts) == 2 && parts[1] == "status":
			plan, err := s.ShortTermMem.SetPlanStatus(planID, req.Status, req.Note)
			if err != nil {
				writePlanJSON(w, http.StatusBadRequest, map[string]interface{}{"status": "error", "message": err.Error()})
				return
			}
			writePlanJSON(w, http.StatusOK, map[string]interface{}{"status": "ok", "plan": plan})
			return
		case len(parts) == 2 && parts[1] == "advance":
			plan, err := s.ShortTermMem.AdvancePlan(planID, req.Result)
			if err != nil {
				writePlanJSON(w, http.StatusBadRequest, map[string]interface{}{"status": "error", "message": err.Error()})
				return
			}
			writePlanJSON(w, http.StatusOK, map[string]interface{}{"status": "ok", "plan": plan})
			return
		case len(parts) == 2 && parts[1] == "reorder":
			plan, err := s.ShortTermMem.ReorderPlanTasks(planID, planTaskIDsFromRequest(req))
			if err != nil {
				writePlanJSON(w, http.StatusBadRequest, map[string]interface{}{"status": "error", "message": err.Error()})
				return
			}
			writePlanJSON(w, http.StatusOK, map[string]interface{}{"status": "ok", "plan": plan})
			return
		case len(parts) == 2 && parts[1] == "archive":
			plan, err := s.ShortTermMem.ArchivePlan(planID)
			if err != nil {
				writePlanJSON(w, http.StatusBadRequest, map[string]interface{}{"status": "error", "message": err.Error()})
				return
			}
			writePlanJSON(w, http.StatusOK, map[string]interface{}{"status": "ok", "plan": plan})
			return
		case len(parts) == 4 && parts[1] == "tasks" && parts[3] == "status":
			taskID := strings.TrimSpace(parts[2])
			plan, err := s.ShortTermMem.UpdatePlanTask(planID, taskID, req.Status, req.Result, req.Error)
			if err != nil {
				writePlanJSON(w, http.StatusBadRequest, map[string]interface{}{"status": "error", "message": err.Error()})
				return
			}
			writePlanJSON(w, http.StatusOK, map[string]interface{}{"status": "ok", "plan": plan})
			return
		case len(parts) == 4 && parts[1] == "tasks" && parts[3] == "block":
			taskID := strings.TrimSpace(parts[2])
			plan, err := s.ShortTermMem.SetPlanTaskBlocker(planID, taskID, req.Reason)
			if err != nil {
				writePlanJSON(w, http.StatusBadRequest, map[string]interface{}{"status": "error", "message": err.Error()})
				return
			}
			writePlanJSON(w, http.StatusOK, map[string]interface{}{"status": "ok", "plan": plan})
			return
		case len(parts) == 4 && parts[1] == "tasks" && parts[3] == "unblock":
			taskID := strings.TrimSpace(parts[2])
			plan, err := s.ShortTermMem.ClearPlanTaskBlocker(planID, taskID, req.Note)
			if err != nil {
				writePlanJSON(w, http.StatusBadRequest, map[string]interface{}{"status": "error", "message": err.Error()})
				return
			}
			writePlanJSON(w, http.StatusOK, map[string]interface{}{"status": "ok", "plan": plan})
			return
		case len(parts) == 4 && parts[1] == "tasks" && parts[3] == "split":
			taskID := strings.TrimSpace(parts[2])
			plan, err := s.ShortTermMem.SplitPlanTask(planID, taskID, planTaskInputsFromRequestItems(req.Items))
			if err != nil {
				writePlanJSON(w, http.StatusBadRequest, map[string]interface{}{"status": "error", "message": err.Error()})
				return
			}
			writePlanJSON(w, http.StatusOK, map[string]interface{}{"status": "ok", "plan": plan})
			return
		default:
			writePlanJSON(w, http.StatusNotFound, map[string]interface{}{"status": "error", "message": "unknown plan action"})
			return
		}
	}
}
