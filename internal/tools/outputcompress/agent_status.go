// Package outputcompress – agent status tool output compressors.
//
// manage_daemon and manage_plan return JSON with "Tool Output: " prefix:
//   - manage_daemon list: {"status":"success","count":N,"daemons":[states]}
//   - manage_daemon status: {"status":"success","daemon":{...}}
//   - manage_plan list: {"status":"success","count":N,"plans":[...]}
//   - manage_plan get: {"status":"success","plan":{...}}
//
// Strategy:
//   - Strip "Tool Output: " prefix
//   - For list operations: compact to summary with key fields per item
//   - For single-item operations: compact to essential fields
//   - Error responses: pass through unchanged
package outputcompress

import (
	"encoding/json"
	"fmt"
	"strings"
)

// compressAgentStatusOutput routes agent status tool outputs to sub-compressors.
func compressAgentStatusOutput(toolName, output string) (string, string) {
	clean := strings.TrimSpace(output)

	// Strip "Tool Output: " prefix
	clean = strings.TrimPrefix(clean, "Tool Output: ")

	if !strings.HasPrefix(clean, "{") {
		return compressGeneric(output), "agent-status-nonjson"
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(clean), &raw); err != nil {
		return compressGeneric(output), "agent-status-parse-err"
	}

	// Error responses: return as-is
	if statusStr := jsonString(raw["status"]); statusStr == "error" {
		return clean, "agent-status-error"
	}

	switch toolName {
	case "manage_daemon":
		return compressDaemonOutput(raw), "agent-daemon"
	case "manage_plan":
		return compressPlanOutput(raw), "agent-plan"
	default:
		return clean, "agent-status-generic"
	}
}

// compressDaemonOutput compacts manage_daemon outputs.
//
// List: From {"status":"success","count":3,"daemons":[{...},{...},{...}]}
//
//	To: "3 daemons:\n  skill-id: running (uptime 2h, restarts: 0)\n  ..."
//
// Status: From {"status":"success","daemon":{...}}
//
//	To: "skill-id: running (uptime 2h, restarts: 0, next run: ...)"
func compressDaemonOutput(raw map[string]json.RawMessage) string {
	// List operation: has "daemons" array
	if daemonsRaw := raw["daemons"]; daemonsRaw != nil {
		var daemons []map[string]interface{}
		if err := json.Unmarshal(daemonsRaw, &daemons); err != nil {
			return compactMap(raw)
		}

		if len(daemons) == 0 {
			return "No daemons registered."
		}

		var sb strings.Builder
		fmt.Fprintf(&sb, "%d daemons:\n", len(daemons))
		for _, d := range daemons {
			sb.WriteString("  ")
			if id, ok := d["skill_id"].(string); ok {
				sb.WriteString(id)
			} else if id, ok := d["id"].(string); ok {
				sb.WriteString(id)
			}
			sb.WriteString(": ")
			if status, ok := d["status"].(string); ok {
				sb.WriteString(status)
			}
			if uptime, ok := d["uptime"].(string); ok && uptime != "" {
				sb.WriteString(" (uptime " + uptime)
				if restarts, ok := d["restart_count"].(float64); ok && restarts > 0 {
					fmt.Fprintf(&sb, ", restarts: %.0f", restarts)
				}
				sb.WriteString(")")
			}
			if nextRun, ok := d["next_run"].(string); ok && nextRun != "" {
				sb.WriteString(" [next: " + nextRun + "]")
			}
			sb.WriteString("\n")
		}
		return sb.String()
	}

	// Single daemon status: has "daemon" object
	if daemonRaw := raw["daemon"]; daemonRaw != nil {
		var daemon map[string]interface{}
		if err := json.Unmarshal(daemonRaw, &daemon); err != nil {
			return compactMap(raw)
		}
		return compactDaemonState(daemon)
	}

	// Simple success message
	if msg := jsonString(raw["message"]); msg != "" {
		return msg
	}

	return compactMap(raw)
}

// compactDaemonState formats a single daemon state compactly.
func compactDaemonState(d map[string]interface{}) string {
	var sb strings.Builder

	if id, ok := d["skill_id"].(string); ok {
		sb.WriteString(id)
	} else if id, ok := d["id"].(string); ok {
		sb.WriteString(id)
	}
	sb.WriteString(": ")

	if status, ok := d["status"].(string); ok {
		sb.WriteString(status)
	}

	details := []string{}
	if uptime, ok := d["uptime"].(string); ok && uptime != "" {
		details = append(details, "uptime "+uptime)
	}
	if restarts, ok := d["restart_count"].(float64); ok && restarts > 0 {
		details = append(details, fmt.Sprintf("restarts: %.0f", restarts))
	}
	if nextRun, ok := d["next_run"].(string); ok && nextRun != "" {
		details = append(details, "next: "+nextRun)
	}
	if interval, ok := d["interval"].(string); ok && interval != "" {
		details = append(details, "interval: "+interval)
	}

	if len(details) > 0 {
		sb.WriteString(" (" + strings.Join(details, ", ") + ")")
	}

	return sb.String()
}

// compressPlanOutput compacts manage_plan outputs.
//
// List: From {"status":"success","count":5,"plans":[{...},{...},...]}
//
//	To: "5 plans:\n  [active] Plan title (3/5 tasks, priority 2)\n  ..."
//
// Get: From {"status":"success","plan":{...}}
//
//	To: "[active] Plan title\n  3/5 tasks done | priority 2 | created ..."
func compressPlanOutput(raw map[string]json.RawMessage) string {
	// List operation: has "plans" array
	if plansRaw := raw["plans"]; plansRaw != nil {
		var plans []map[string]interface{}
		if err := json.Unmarshal(plansRaw, &plans); err != nil {
			return compactMap(raw)
		}

		if len(plans) == 0 {
			return "No plans found."
		}

		var sb strings.Builder
		fmt.Fprintf(&sb, "%d plans:\n", len(plans))
		for _, p := range plans {
			sb.WriteString("  ")
			sb.WriteString("[")
			if status, ok := p["status"].(string); ok {
				sb.WriteString(status)
			} else {
				sb.WriteString("?")
			}
			sb.WriteString("] ")

			if title, ok := p["title"].(string); ok {
				sb.WriteString(title)
			}

			// Task progress
			totalTasks := 0
			doneTasks := 0
			if tasks, ok := p["tasks"].([]interface{}); ok {
				totalTasks = len(tasks)
				for _, t := range tasks {
					if tm, ok := t.(map[string]interface{}); ok {
						if ts, ok := tm["status"].(string); ok && ts == "done" {
							doneTasks++
						}
					}
				}
			} else if total, ok := p["total_tasks"].(float64); ok {
				totalTasks = int(total)
				if done, ok := p["done_tasks"].(float64); ok {
					doneTasks = int(done)
				}
			}

			if totalTasks > 0 {
				fmt.Fprintf(&sb, " (%d/%d tasks", doneTasks, totalTasks)
				if pri, ok := p["priority"].(float64); ok && pri > 0 {
					fmt.Fprintf(&sb, ", priority %.0f", pri)
				}
				sb.WriteString(")")
			}

			sb.WriteString("\n")
		}
		return sb.String()
	}

	// Single plan: has "plan" object
	if planRaw := raw["plan"]; planRaw != nil {
		var plan map[string]interface{}
		if err := json.Unmarshal(planRaw, &plan); err != nil {
			return compactMap(raw)
		}
		return compactPlanDetail(plan)
	}

	// Simple success message
	if msg := jsonString(raw["message"]); msg != "" {
		return msg
	}

	return compactMap(raw)
}

// compactPlanDetail formats a single plan with tasks compactly.
func compactPlanDetail(p map[string]interface{}) string {
	var sb strings.Builder

	sb.WriteString("[")
	if status, ok := p["status"].(string); ok {
		sb.WriteString(status)
	} else {
		sb.WriteString("?")
	}
	sb.WriteString("] ")

	if title, ok := p["title"].(string); ok {
		sb.WriteString(title)
	}
	sb.WriteString("\n")

	// Summary line
	details := []string{}
	if pri, ok := p["priority"].(float64); ok && pri > 0 {
		details = append(details, fmt.Sprintf("priority %.0f", pri))
	}
	if created, ok := p["created_at"].(string); ok && created != "" {
		details = append(details, "created "+created)
	}
	if updated, ok := p["updated_at"].(string); ok && updated != "" {
		details = append(details, "updated "+updated)
	}
	if len(details) > 0 {
		sb.WriteString("  " + strings.Join(details, " | ") + "\n")
	}

	// Task list
	if tasks, ok := p["tasks"].([]interface{}); ok && len(tasks) > 0 {
		doneCount := 0
		for _, t := range tasks {
			if tm, ok := t.(map[string]interface{}); ok {
				status := "?"
				if ts, ok := tm["status"].(string); ok {
					status = ts
				}
				if status == "done" {
					doneCount++
				}
				title := ""
				if tt, ok := tm["title"].(string); ok {
					title = tt
				} else if tt, ok := tm["description"].(string); ok {
					title = tt
				}
				if title != "" {
					fmt.Fprintf(&sb, "  [%s] %s\n", status, title)
				}
			}
		}
		// Replace individual tasks with summary if too many
		// (already shown above, so just add count)
		if doneCount > 0 {
			// Summary already visible from individual tasks
		}
	}

	return sb.String()
}

// compactMap is a fallback that compacts a JSON map to key=value pairs.
func compactMap(m map[string]json.RawMessage) string {
	var sb strings.Builder
	first := true
	for k, v := range m {
		if !first {
			sb.WriteString(", ")
		}
		first = false
		sb.WriteString(k + "=" + string(v))
	}
	return sb.String()
}
