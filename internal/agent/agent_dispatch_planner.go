package agent

import (
	"database/sql"
	"log/slog"
	"sort"
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
		kgNote := syncAppointmentToKG(db, id, kg, logger)
		return "Tool Output: " + planner.ToJSON(map[string]interface{}{"status": "success", "message": "Appointment created" + kgNote, "id": id})

	case "update":
		id, _ := tc.Params["id"].(string)
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
			existing.DateTime = strParam(tc.Params, "date_time")
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
		kgNote := syncAppointmentToKG(db, id, kg, logger)
		return "Tool Output: " + planner.ToJSON(map[string]interface{}{"status": "success", "message": "Appointment updated" + kgNote, "id": id})

	case "complete":
		id, _ := tc.Params["id"].(string)
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
		kgNote := syncAppointmentToKG(db, id, kg, logger)
		return "Tool Output: " + planner.ToJSON(map[string]interface{}{"status": "success", "message": "Appointment marked as completed" + kgNote, "id": id})

	case "cancel":
		id, _ := tc.Params["id"].(string)
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
		kgNote := syncAppointmentToKG(db, id, kg, logger)
		return "Tool Output: " + planner.ToJSON(map[string]interface{}{"status": "success", "message": "Appointment cancelled" + kgNote, "id": id})

	case "delete":
		id, _ := tc.Params["id"].(string)
		if id == "" {
			return `Tool Output: {"status":"error","message":"'id' is required for delete operation"}`
		}
		// ISSUE-7: Fetch KGNodeID from DB before deleting to avoid using a hardcoded prefix.
		existingAppt, err := planner.GetAppointment(db, id)
		if err != nil {
			return plannerError(err)
		}
		if err := planner.DeleteAppointment(db, id); err != nil {
			return plannerError(err)
		}
		if kg != nil && existingAppt.KGNodeID != "" {
			_ = kg.DeleteNode(existingAppt.KGNodeID)
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
			RemindDaily: boolParam(tc.Params, "remind_daily"),
			Items:       todoItemsParam(tc.Params, "items"),
		}
		id, err := planner.CreateTodo(db, t)
		if err != nil {
			return plannerError(err)
		}
		kgNote := syncTodoToKG(db, id, kg, logger)
		return "Tool Output: " + planner.ToJSON(map[string]interface{}{"status": "success", "message": "Todo created" + kgNote, "id": id})

	case "update":
		id, _ := tc.Params["id"].(string)
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
		if _, ok := tc.Params["remind_daily"]; ok {
			existing.RemindDaily = boolParam(tc.Params, "remind_daily")
		}
		if _, ok := tc.Params["items"]; ok {
			existing.Items = todoItemsParam(tc.Params, "items")
		}
		if err := planner.UpdateTodo(db, *existing); err != nil {
			return plannerError(err)
		}
		kgNote := syncTodoToKG(db, id, kg, logger)
		return "Tool Output: " + planner.ToJSON(map[string]interface{}{"status": "success", "message": "Todo updated" + kgNote, "id": id})

	case "set_status":
		id, _ := tc.Params["id"].(string)
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
		kgNote := syncTodoToKG(db, id, kg, logger)
		return "Tool Output: " + planner.ToJSON(map[string]interface{}{"status": "success", "message": "Todo status updated to " + status + kgNote, "id": id})

	case "complete":
		id, _ := tc.Params["id"].(string)
		if id == "" {
			return `Tool Output: {"status":"error","message":"'id' is required for complete operation"}`
		}
		if err := planner.CompleteTodo(db, id, boolParam(tc.Params, "complete_items_too")); err != nil {
			return plannerError(err)
		}
		kgNote := syncTodoToKG(db, id, kg, logger)
		return "Tool Output: " + planner.ToJSON(map[string]interface{}{"status": "success", "message": "Todo completed" + kgNote, "id": id})

	case "add_item":
		id, _ := tc.Params["id"].(string)
		if id == "" {
			return `Tool Output: {"status":"error","message":"'id' is required for add_item operation"}`
		}
		itemTitle := strParam(tc.Params, "item_title")
		if itemTitle == "" {
			return `Tool Output: {"status":"error","message":"'item_title' is required for add_item operation"}`
		}
		itemID, err := planner.AddTodoItem(db, id, planner.TodoItem{
			Title:       itemTitle,
			Description: strParam(tc.Params, "item_description"),
			Position:    intParam(tc.Params, "item_position"),
			IsDone:      boolParam(tc.Params, "item_is_done"),
		})
		if err != nil {
			return plannerError(err)
		}
		kgNote := syncTodoToKG(db, id, kg, logger)
		return "Tool Output: " + planner.ToJSON(map[string]interface{}{"status": "success", "message": "Todo item added" + kgNote, "id": id, "item_id": itemID})

	case "update_item":
		id, _ := tc.Params["id"].(string)
		itemID := strParam(tc.Params, "item_id")
		if id == "" || itemID == "" {
			return `Tool Output: {"status":"error","message":"'id' and 'item_id' are required for update_item operation"}`
		}
		existing, err := planner.GetTodo(db, id)
		if err != nil {
			return plannerError(err)
		}
		item, found := todoItemByID(existing.Items, itemID)
		if !found {
			return plannerErrorMsg("todo item not found")
		}
		if _, ok := tc.Params["item_title"]; ok {
			if value := strParam(tc.Params, "item_title"); value != "" {
				item.Title = value
			}
		}
		if _, ok := tc.Params["item_description"]; ok {
			item.Description = strParam(tc.Params, "item_description")
		}
		if _, ok := tc.Params["item_position"]; ok {
			item.Position = intParam(tc.Params, "item_position")
		}
		if _, ok := tc.Params["item_is_done"]; ok {
			item.IsDone = boolParam(tc.Params, "item_is_done")
		}
		if err := planner.UpdateTodoItem(db, item); err != nil {
			return plannerError(err)
		}
		kgNote := syncTodoToKG(db, id, kg, logger)
		return "Tool Output: " + planner.ToJSON(map[string]interface{}{"status": "success", "message": "Todo item updated" + kgNote, "id": id, "item_id": itemID})

	case "toggle_item":
		id, _ := tc.Params["id"].(string)
		itemID := strParam(tc.Params, "item_id")
		if id == "" || itemID == "" {
			return `Tool Output: {"status":"error","message":"'id' and 'item_id' are required for toggle_item operation"}`
		}
		existing, err := planner.GetTodo(db, id)
		if err != nil {
			return plannerError(err)
		}
		item, found := todoItemByID(existing.Items, itemID)
		if !found {
			return plannerErrorMsg("todo item not found")
		}
		if _, ok := tc.Params["item_is_done"]; ok {
			item.IsDone = boolParam(tc.Params, "item_is_done")
		} else {
			item.IsDone = !item.IsDone
		}
		if err := planner.UpdateTodoItem(db, item); err != nil {
			return plannerError(err)
		}
		kgNote := syncTodoToKG(db, id, kg, logger)
		return "Tool Output: " + planner.ToJSON(map[string]interface{}{"status": "success", "message": "Todo item toggled" + kgNote, "id": id, "item_id": itemID, "item_is_done": item.IsDone})

	case "delete_item":
		id, _ := tc.Params["id"].(string)
		itemID := strParam(tc.Params, "item_id")
		if id == "" || itemID == "" {
			return `Tool Output: {"status":"error","message":"'id' and 'item_id' are required for delete_item operation"}`
		}
		if err := planner.DeleteTodoItem(db, id, itemID); err != nil {
			return plannerError(err)
		}
		kgNote := syncTodoToKG(db, id, kg, logger)
		return "Tool Output: " + planner.ToJSON(map[string]interface{}{"status": "success", "message": "Todo item deleted" + kgNote, "id": id, "item_id": itemID})

	case "reorder_items":
		id, _ := tc.Params["id"].(string)
		if id == "" {
			return `Tool Output: {"status":"error","message":"'id' is required for reorder_items operation"}`
		}
		itemIDs := stringSliceParam(tc.Params, "item_ids")
		if len(itemIDs) == 0 {
			return `Tool Output: {"status":"error","message":"'item_ids' is required for reorder_items operation"}`
		}
		if err := planner.ReorderTodoItems(db, id, itemIDs); err != nil {
			return plannerError(err)
		}
		kgNote := syncTodoToKG(db, id, kg, logger)
		return "Tool Output: " + planner.ToJSON(map[string]interface{}{"status": "success", "message": "Todo items reordered" + kgNote, "id": id})

	case "delete":
		id, _ := tc.Params["id"].(string)
		if id == "" {
			return `Tool Output: {"status":"error","message":"'id' is required for delete operation"}`
		}
		// ISSUE-7: Fetch KGNodeID from DB before deleting to avoid using a hardcoded prefix.
		existingTodo, err := planner.GetTodo(db, id)
		if err != nil {
			return plannerError(err)
		}
		if err := planner.DeleteTodo(db, id); err != nil {
			return plannerError(err)
		}
		if kg != nil && existingTodo.KGNodeID != "" {
			_ = kg.DeleteNode(existingTodo.KGNodeID)
		}
		return "Tool Output: " + planner.ToJSON(map[string]interface{}{"status": "success", "message": "Todo deleted", "id": id})

	default:
		return `Tool Output: {"status":"error","message":"Unknown operation. Use: list, get, add, update, delete, set_status, complete, add_item, update_item, toggle_item, delete_item, reorder_items"}`
	}
}

// syncAppointmentToKG syncs an appointment to the knowledge graph.
// Returns a note string if sync failed, empty string on success (ISSUE-13).
func syncAppointmentToKG(db *sql.DB, id string, kg planner.KnowledgeGraph, logger *slog.Logger) string {
	if err := planner.SyncAppointmentToKG(kg, db, id); err != nil {
		logger.Warn("Failed to sync appointment to KG", "id", id, "error", err)
		return "; knowledge graph sync failed"
	}
	return ""
}

// syncTodoToKG syncs a todo to the knowledge graph.
// Returns a note string if sync failed, empty string on success (ISSUE-13).
func syncTodoToKG(db *sql.DB, id string, kg planner.KnowledgeGraph, logger *slog.Logger) string {
	if err := planner.SyncTodoToKG(kg, db, id); err != nil {
		logger.Warn("Failed to sync todo to KG", "id", id, "error", err)
		return "; knowledge graph sync failed"
	}
	return ""
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

func intParam(params map[string]interface{}, key string) int {
	switch value := params[key].(type) {
	case int:
		return value
	case int32:
		return int(value)
	case int64:
		return int(value)
	case float64:
		return int(value)
	default:
		return 0
	}
}

func stringSliceParam(params map[string]interface{}, key string) []string {
	raw, ok := params[key]
	if !ok || raw == nil {
		return nil
	}
	switch value := raw.(type) {
	case []string:
		return append([]string(nil), value...)
	case []interface{}:
		result := make([]string, 0, len(value))
		for _, entry := range value {
			if text, ok := entry.(string); ok && strings.TrimSpace(text) != "" {
				result = append(result, text)
			}
		}
		return result
	default:
		return nil
	}
}

func todoItemsParam(params map[string]interface{}, key string) []planner.TodoItem {
	raw, ok := params[key]
	if !ok || raw == nil {
		return nil
	}
	var source []interface{}
	switch value := raw.(type) {
	case []interface{}:
		source = value
	case []map[string]interface{}:
		source = make([]interface{}, 0, len(value))
		for _, entry := range value {
			source = append(source, entry)
		}
	default:
		return nil
	}

	items := make([]planner.TodoItem, 0, len(source))
	for index, entry := range source {
		itemMap, ok := entry.(map[string]interface{})
		if !ok {
			continue
		}
		title := strings.TrimSpace(strValue(itemMap["title"]))
		if title == "" {
			continue
		}
		items = append(items, planner.TodoItem{
			ID:          strValue(itemMap["id"]),
			Title:       title,
			Description: strValue(itemMap["description"]),
			Position:    intValueWithDefault(itemMap["position"], index),
			IsDone:      boolValue(itemMap["is_done"]),
		})
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].Position < items[j].Position })
	return items
}

func todoItemByID(items []planner.TodoItem, itemID string) (planner.TodoItem, bool) {
	for _, item := range items {
		if item.ID == itemID {
			return item, true
		}
	}
	return planner.TodoItem{}, false
}

func strValue(raw interface{}) string {
	if value, ok := raw.(string); ok {
		return value
	}
	return ""
}

func boolValue(raw interface{}) bool {
	if value, ok := raw.(bool); ok {
		return value
	}
	return false
}

func intValueWithDefault(raw interface{}, fallback int) int {
	switch value := raw.(type) {
	case int:
		return value
	case int32:
		return int(value)
	case int64:
		return int(value)
	case float64:
		return int(value)
	default:
		return fallback
	}
}
