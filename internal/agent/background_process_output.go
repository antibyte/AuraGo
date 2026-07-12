package agent

import (
	"fmt"
	"strings"
)

func backgroundProcessStartedOutput(label string, pid int) string {
	return fmt.Sprintf(
		"Tool Output: %s started in background. PID=%d. Use {\"action\":\"wait_for_event\",\"event_type\":\"process_exited\",\"pid\":%d,\"task_prompt\":\"Read process logs, verify the exit code, and continue the task.\"} to continue automatically. Do not poll with sleep. Use {\"action\":\"read_process_logs\",\"pid\":%d} for on-demand output.",
		label, pid, pid, pid,
	)
}

func formatManagedProcessList(processes []map[string]interface{}) string {
	if len(processes) == 0 {
		return "Tool Output: No managed background processes."
	}
	var sb strings.Builder
	sb.WriteString("Tool Output: Managed background processes:\n")
	for _, process := range processes {
		fmt.Fprintf(&sb, "- PID: %v, Started: %v", process["pid"], process["started"])
		if state := strings.TrimSpace(fmt.Sprint(process["state"])); state != "" && state != "<nil>" {
			fmt.Fprintf(&sb, ", State: %s", state)
		}
		if exitCode, ok := process["exit_code"]; ok && exitCode != nil {
			fmt.Fprintf(&sb, ", Exit code: %v", exitCode)
		}
		if finished := strings.TrimSpace(fmt.Sprint(process["finished_at"])); finished != "" && finished != "<nil>" {
			fmt.Fprintf(&sb, ", Finished: %s", finished)
		}
		if reason := strings.TrimSpace(fmt.Sprint(process["error_reason"])); reason != "" && reason != "<nil>" {
			fmt.Fprintf(&sb, ", Error: %s", reason)
		}
		sb.WriteByte('\n')
	}
	return sb.String()
}
