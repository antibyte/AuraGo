package agent

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"aurago/internal/gamemaker"
)

func recordGameMakerToolResult(ctx context.Context, service *gamemaker.Service, jobID string, call ToolCall, output string, elapsed time.Duration) {
	operation := firstNonEmptyToolString(call.Operation, toolArgString(call.Params, "operation"))
	if operation == "" && call.Action == "game_maker_validate" {
		operation = "validate"
	}
	if operation == "" && call.Action == "game_maker_file" && gamemaker.JobIDFromContext(ctx) != "" {
		if _, ok := call.Params["content"].(string); ok || call.Content != "" {
			operation = "write"
		}
	}
	switch operation {
	case "inspect", "list_files", "get_plan", "set_plan", "set_design", "scene_inspect", "scene_set", "scene_patch", "scene_generate", "read", "search", "write", "replace", "replace_many", "generate", "list_packs", "describe_pack", "import_pack", "search_assets", "describe_asset", "validate":
	default:
		operation = "unknown"
	}
	var envelope struct {
		Status string `json:"status"`
	}
	_ = json.Unmarshal([]byte(strings.TrimPrefix(output, "Tool Output: ")), &envelope)
	status := "error"
	if envelope.Status == "ok" || envelope.Status == "fallback" {
		status = envelope.Status
	}
	// Emit metadata only. Tool arguments, results, source and errors stay out of
	// progress events; status describes the tool result, never publication.
	emitCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
	defer cancel()
	job, err := service.GetJob(emitCtx, jobID)
	if err != nil {
		return
	}
	_ = service.EmitAgentEvent(emitCtx, job.ProjectID, jobID, "tool_result", map[string]any{"tool": call.Action, "operation": operation, "status": status, "phase": job.Phase, "duration_ms": elapsed.Milliseconds()})
}
