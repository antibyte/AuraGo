package agent

import openai "github.com/sashabaranov/go-openai"

// appendPlannerToolSchemas appends the manage_appointments and manage_todos
// tool schemas when the Planner feature is enabled.
func appendPlannerToolSchemas(tools []openai.Tool, ff ToolFeatureFlags) []openai.Tool {
	if ff.PlannerEnabled {
		tools = append(tools, tool("manage_appointments",
			"Manage appointments/calendar entries. Create, update, delete, list, and retrieve appointments with optional notification and agent wake-up support.",
			schema(map[string]interface{}{
				"operation": map[string]interface{}{
					"type":        "string",
					"description": "Operation to perform",
					"enum":        []string{"list", "get", "add", "update", "delete", "complete", "cancel"},
				},
				"id":                prop("string", "Appointment ID (required for get/update/delete/complete/cancel)"),
				"title":             prop("string", "Title of the appointment"),
				"description":       prop("string", "Description or details"),
				"date_time":         prop("string", "Date and time in RFC3339 format (e.g. 2025-03-15T14:00:00Z)"),
				"notification_at":   prop("string", "When to send notification in RFC3339 format"),
				"wake_agent":        prop("boolean", "Whether to wake the agent at notification time"),
				"agent_instruction": prop("string", "Optional instruction for the agent when woken up"),
				"status":            prop("string", "Filter by status (upcoming, completed, cancelled) for list operation"),
				"query":             prop("string", "Search query for list operation"),
			}, "operation"),
		))

		tools = append(tools, tool("manage_todos",
			"Manage the todo list. Create, update, delete, list, and retrieve todos, optional checklist items, progress, and daily reminder settings.",
			schema(map[string]interface{}{
				"operation": map[string]interface{}{
					"type":        "string",
					"description": "Operation to perform",
					"enum":        []string{"list", "get", "add", "update", "delete", "set_status", "complete", "add_item", "update_item", "toggle_item", "delete_item", "reorder_items"},
				},
				"id":          prop("string", "Todo ID (required for get/update/delete/set_status/complete/item operations)"),
				"title":       prop("string", "Title of the todo item"),
				"description": prop("string", "Description or details"),
				"priority":    prop("string", "Priority: low, medium, high"),
				"status":      prop("string", "Status: open, in_progress, done"),
				"due_date":    prop("string", "Due date in RFC3339 format"),
				"remind_daily": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether the agent should proactively remind the user about this todo on the first contact of the day",
				},
				"items": map[string]interface{}{
					"type":        "array",
					"description": "Optional checklist items for add/update todo operations",
					"items": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"id":          prop("string", "Checklist item ID for existing items"),
							"title":       prop("string", "Checklist item title"),
							"description": prop("string", "Checklist item description"),
							"position":    prop("integer", "Checklist item order index"),
							"is_done":     prop("boolean", "Whether the checklist item is completed"),
						},
					},
				},
				"item_id":          prop("string", "Checklist item ID (required for update_item/toggle_item/delete_item)"),
				"item_title":       prop("string", "Checklist item title"),
				"item_description": prop("string", "Checklist item description"),
				"item_position":    prop("integer", "Checklist item order index"),
				"item_is_done":     prop("boolean", "Checklist item completion state"),
				"item_ids": map[string]interface{}{
					"type":        "array",
					"description": "Ordered checklist item IDs for reorder_items",
					"items":       map[string]interface{}{"type": "string"},
				},
				"complete_items_too": map[string]interface{}{
					"type":        "boolean",
					"description": "When completing a todo, also mark all remaining checklist items as done",
				},
				"query": prop("string", "Search query for list operation"),
			}, "operation"),
		))
	}
	return tools
}
