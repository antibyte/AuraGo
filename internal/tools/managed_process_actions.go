package tools

import (
	"context"
	"encoding/json"
	"fmt"
)

// ManageRegisteredProcesses never enumerates or signals unrelated host PIDs.
func ManageRegisteredProcesses(ctx context.Context, operation string, pid int, label string, registry *ProcessRegistry) string {
	encode := func(value any) string { data, _ := json.Marshal(value); return string(data) }
	refuse := func(message string) string { return encode(ProcResult{Status: "error", Message: message}) }
	if err := ctx.Err(); err != nil {
		return refuse(err.Error())
	}
	if registry == nil {
		return refuse("Managed process registry is unavailable")
	}
	if operation == "kill" {
		permissions, configured := EffectiveRuntimePermissions(ctx)
		if !configured || !permissions.AllowShell {
			return refuse("Process termination is disabled by the shell capability gate")
		}
	}
	items := registry.List()
	for _, item := range items {
		item["label"] = fmt.Sprintf("process-%v", item["pid"])
	}
	if operation == "list" {
		return encode(ProcResult{Status: "success", Data: items})
	}
	if operation != "status" && operation != "kill" {
		return refuse("operation must be list, status or kill")
	}
	for _, item := range items {
		itemPID, _ := item["pid"].(int)
		if (pid > 0 && itemPID != pid) || (label != "" && item["label"] != label) || (pid <= 0 && label == "") {
			continue
		}
		if operation == "status" {
			return encode(ProcResult{Status: "success", Data: item})
		}
		if err := ctx.Err(); err != nil {
			return refuse(err.Error())
		}
		if err := registry.Terminate(itemPID); err != nil {
			return refuse(err.Error())
		}
		return encode(ProcResult{Status: "success", Message: "Managed process terminated", Data: map[string]any{"pid": itemPID}})
	}
	return refuse("Managed process was not found; use list to select its PID or label")
}
