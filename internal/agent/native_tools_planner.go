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
			"Manage the todo list. Create, update, delete, list, and retrieve todo items with priority and status tracking.",
			schema(map[string]interface{}{
				"operation": map[string]interface{}{
					"type":        "string",
					"description": "Operation to perform",
					"enum":        []string{"list", "get", "add", "update", "delete", "set_status"},
				},
				"id":          prop("string", "Todo ID (required for get/update/delete/set_status)"),
				"title":       prop("string", "Title of the todo item"),
				"description": prop("string", "Description or details"),
				"priority":    prop("string", "Priority: low, medium, high"),
				"status":      prop("string", "Status: open, in_progress, done, cancelled"),
				"due_date":    prop("string", "Due date in RFC3339 format"),
				"query":       prop("string", "Search query for list operation"),
			}, "operation"),
		))
	}
	return tools
}
