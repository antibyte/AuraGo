package agent

import (
	"database/sql"
	"log/slog"
	"strings"

	"aurago/internal/planner"
)

// plannerError returns a safe JSON error string, properly escaping the message.
func plannerError(err error) string {
	return "Tool Output: " + planner.ToJSON(map[string]string{"status": "error", "message": err.Error()})
}

// plannerErrorMsg returns a safe JSON error string from a plain message.
func plannerErrorMsg(msg string) string {
	return "Tool Output: " + planner.ToJSON(map[string]string{"status": "error", "message": msg})
}

// dispatchManageAppointments handles the manage_appointments tool call.
func dispatchManageAppointments(tc ToolCall, db *sql.DB, kg planner.KnowledgeGraph, logger *slog.Logger) string {
	op := strings.ToLower(tc.Operation)
	if op == "" {
		if v, ok := tc.Params["operation"].(string); ok {
			op = strings.ToLower(v)
		}
	}

	logger.Info("LLM requested appointment operation", "op", op)

	switch op {
	case "list":
		query, _ := tc.Params["query"].(string)
		status, _ := tc.Params["status"].(string)
		list, err := planner.ListAppointments(db, query, status)
		if err != nil {
			return plannerError(err)
		}
		return "Tool Output: " + planner.ToJSON(map[string]interface{}{"status": "success", "appointments": list, "count": len(list)})

	case "get":
		id, _ := tc.Params["id"].(string)
		if id == "" {
			id = tc.ID
		}
		if id == "" {
			return `Tool Output: {"status":"error","message":"'id' is required for get operation"}`
		}
		a, err := planner.GetAppointment(db, id)
		if err != nil {
			return plannerError(err)
		}
		return "Tool Output: " + planner.ToJSON(map[string]interface{}{"status": "success", "appointment": a})

	case "add":
		title, _ := tc.Params["title"].(string)
		if title == "" {
			return `Tool Output: {"status":"error","message":"'title' is required to add an appointment"}`
		}
		a := planner.Appointment{
			Title:            title,
			Description:      strParam(tc.Params, "description"),
			DateTime:         strParam(tc.Params, "date_time"),
			NotificationAt:   strParam(tc.Params, "notification_at"),
			WakeAgent:        boolParam(tc.Params, "wake_agent"),
			AgentInstruction: strParam(tc.Params, "agent_instruction"),
		}
		id, err := planner.CreateAppointment(db, a)
		if err != nil {
			return plannerError(err)
		}
		syncAppointmentToKG(db, id, kg, logger)
		return "Tool Output: " + planner.ToJSON(map[string]interface{}{"status": "success", "message": "Appointment created", "id": id})

	case "update":
		id, _ := tc.Params["id"].(string)
		if id == "" {
			id = tc.ID
		}
		if id == "" {
			return `Tool Output: {"status":"error","message":"'id' is required for update operation"}`
		}
		existing, err := planner.GetAppointment(db, id)
		if err != nil {
			return plannerError(err)
		}
		if _, ok := tc.Params["title"]; ok {
			if v := strParam(tc.Params, "title"); v != "" {
				existing.Title = v
			}
		}
		if _, ok := tc.Params["description"]; ok {
			existing.Description = strParam(tc.Params, "description")
		}
		if _, ok := tc.Params["date_time"]; ok {
			if v := strParam(tc.Params, "date_time"); v != "" {
				existing.DateTime = v
			}
		}
		if _, ok := tc.Params["notification_at"]; ok {
			existing.NotificationAt = strParam(tc.Params, "notification_at")
		}
		if _, ok := tc.Params["wake_agent"]; ok {
			existing.WakeAgent = boolParam(tc.Params, "wake_agent")
		}
		if _, ok := tc.Params["agent_instruction"]; ok {
			existing.AgentInstruction = strParam(tc.Params, "agent_instruction")
		}
		if _, ok := tc.Params["status"]; ok {
			if v := strParam(tc.Params, "status"); v != "" {
				existing.Status = v
			}
		}
		if err := planner.UpdateAppointment(db, *existing); err != nil {
			return plannerError(err)
		}
		syncAppointmentToKG(db, id, kg, logger)
		return "Tool Output: " + planner.ToJSON(map[string]interface{}{"status": "success", "message": "Appointment updated", "id": id})

	case "complete":
		id, _ := tc.Params["id"].(string)
		if id == "" {
			id = tc.ID
		}
		if id == "" {
			return `Tool Output: {"status":"error","message":"'id' is required for complete operation"}`
		}
		existing, err := planner.GetAppointment(db, id)
		if err != nil {
			return plannerError(err)
		}
		existing.Status = "completed"
		if err := planner.UpdateAppointment(db, *existing); err != nil {
			return plannerError(err)
		}
		syncAppointmentToKG(db, id, kg, logger)
		return "Tool Output: " + planner.ToJSON(map[string]interface{}{"status": "success", "message": "Appointment marked as completed", "id": id})

	case "cancel":
		id, _ := tc.Params["id"].(string)
		if id == "" {
			id = tc.ID
		}
		if id == "" {
			return `Tool Output: {"status":"error","message":"'id' is required for cancel operation"}`
		}
		existing, err := planner.GetAppointment(db, id)
		if err != nil {
			return plannerError(err)
		}
		existing.Status = "cancelled"
		if err := planner.UpdateAppointment(db, *existing); err != nil {
			return plannerError(err)
		}
		syncAppointmentToKG(db, id, kg, logger)
		return "Tool Output: " + planner.ToJSON(map[string]interface{}{"status": "success", "message": "Appointment cancelled", "id": id})

	case "delete":
		id, _ := tc.Params["id"].(string)
		if id == "" {
			id = tc.ID
		}
		if id == "" {
			return `Tool Output: {"status":"error","message":"'id' is required for delete operation"}`
		}
		nodeID := "appointment_" + id
		if err := planner.DeleteAppointment(db, id); err != nil {
			return plannerError(err)
		}
		if kg != nil {
			_ = kg.DeleteNode(nodeID)
		}
		return "Tool Output: " + planner.ToJSON(map[string]interface{}{"status": "success", "message": "Appointment deleted", "id": id})

	default:
		return `Tool Output: {"status":"error","message":"Unknown operation. Use: list, get, add, update, delete, complete, cancel"}`
	}
}

// dispatchManageTodos handles the manage_todos tool call.
func dispatchManageTodos(tc ToolCall, db *sql.DB, kg planner.KnowledgeGraph, logger *slog.Logger) string {
	op := strings.ToLower(tc.Operation)
	if op == "" {
		if v, ok := tc.Params["operation"].(string); ok {
			op = strings.ToLower(v)
		}
	}

	logger.Info("LLM requested todo operation", "op", op)

	switch op {
	case "list":
		query, _ := tc.Params["query"].(string)
		status, _ := tc.Params["status"].(string)
		list, err := planner.ListTodos(db, query, status)
		if err != nil {
			return plannerError(err)
		}
		return "Tool Output: " + planner.ToJSON(map[string]interface{}{"status": "success", "todos": list, "count": len(list)})

	case "get":
		id, _ := tc.Params["id"].(string)
		if id == "" {
			id = tc.ID
		}
		if id == "" {
			return `Tool Output: {"status":"error","message":"'id' is required for get operation"}`
		}
		t, err := planner.GetTodo(db, id)
		if err != nil {
			return plannerError(err)
		}
		return "Tool Output: " + planner.ToJSON(map[string]interface{}{"status": "success", "todo": t})

	case "add":
		title, _ := tc.Params["title"].(string)
		if title == "" {
			return `Tool Output: {"status":"error","message":"'title' is required to add a todo"}`
		}
		t := planner.Todo{
			Title:       title,
			Description: strParam(tc.Params, "description"),
			Priority:    strParam(tc.Params, "priority"),
			Status:      strParam(tc.Params, "status"),
			DueDate:     strParam(tc.Params, "due_date"),
		}
		id, err := planner.CreateTodo(db, t)
		if err != nil {
			return plannerError(err)
		}
		syncTodoToKG(db, id, kg, logger)
		return "Tool Output: " + planner.ToJSON(map[string]interface{}{"status": "success", "message": "Todo created", "id": id})

	case "update":
		id, _ := tc.Params["id"].(string)
		if id == "" {
			id = tc.ID
		}
		if id == "" {
			return `Tool Output: {"status":"error","message":"'id' is required for update operation"}`
		}
		existing, err := planner.GetTodo(db, id)
		if err != nil {
			return plannerError(err)
		}
		if _, ok := tc.Params["title"]; ok {
			if v := strParam(tc.Params, "title"); v != "" {
				existing.Title = v
			}
		}
		if _, ok := tc.Params["description"]; ok {
			existing.Description = strParam(tc.Params, "description")
		}
		if _, ok := tc.Params["priority"]; ok {
			existing.Priority = strParam(tc.Params, "priority")
		}
		if _, ok := tc.Params["status"]; ok {
			if v := strParam(tc.Params, "status"); v != "" {
				existing.Status = v
			}
		}
		if _, ok := tc.Params["due_date"]; ok {
			existing.DueDate = strParam(tc.Params, "due_date")
		}
		if err := planner.UpdateTodo(db, *existing); err != nil {
			return plannerError(err)
		}
		syncTodoToKG(db, id, kg, logger)
		return "Tool Output: " + planner.ToJSON(map[string]interface{}{"status": "success", "message": "Todo updated", "id": id})

	case "set_status":
		id, _ := tc.Params["id"].(string)
		if id == "" {
			id = tc.ID
		}
		if id == "" {
			return `Tool Output: {"status":"error","message":"'id' is required for set_status operation"}`
		}
		status, _ := tc.Params["status"].(string)
		if status == "" {
			return `Tool Output: {"status":"error","message":"'status' is required for set_status (open, in_progress, done)"}`
		}
		existing, err := planner.GetTodo(db, id)
		if err != nil {
			return plannerError(err)
		}
		existing.Status = status
		if err := planner.UpdateTodo(db, *existing); err != nil {
			return plannerError(err)
		}
		syncTodoToKG(db, id, kg, logger)
		return "Tool Output: " + planner.ToJSON(map[string]interface{}{"status": "success", "message": "Todo status updated to " + status, "id": id})

	case "delete":
		id, _ := tc.Params["id"].(string)
		if id == "" {
			id = tc.ID
		}
		if id == "" {
			return `Tool Output: {"status":"error","message":"'id' is required for delete operation"}`
		}
		nodeID := "todo_" + id
		if err := planner.DeleteTodo(db, id); err != nil {
			return plannerError(err)
		}
		if kg != nil {
			_ = kg.DeleteNode(nodeID)
		}
		return "Tool Output: " + planner.ToJSON(map[string]interface{}{"status": "success", "message": "Todo deleted", "id": id})

	default:
		return `Tool Output: {"status":"error","message":"Unknown operation. Use: list, get, add, update, delete, set_status"}`
	}
}

// syncAppointmentToKG syncs an appointment to the knowledge graph.
func syncAppointmentToKG(db *sql.DB, id string, kg planner.KnowledgeGraph, logger *slog.Logger) {
	if err := planner.SyncAppointmentToKG(kg, db, id); err != nil {
		logger.Warn("Failed to sync appointment to KG", "id", id, "error", err)
	}
}

// syncTodoToKG syncs a todo to the knowledge graph.
func syncTodoToKG(db *sql.DB, id string, kg planner.KnowledgeGraph, logger *slog.Logger) {
	if err := planner.SyncTodoToKG(kg, db, id); err != nil {
		logger.Warn("Failed to sync todo to KG", "id", id, "error", err)
	}
}

// strParam extracts a string parameter from the tool call params map.
func strParam(params map[string]interface{}, key string) string {
	if v, ok := params[key].(string); ok {
		return v
	}
	return ""
}

// boolParam extracts a boolean parameter from the tool call params map.
func boolParam(params map[string]interface{}, key string) bool {
	if v, ok := params[key].(bool); ok {
		return v
	}
	return false
}
