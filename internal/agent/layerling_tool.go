package agent

import (
	"aurago/internal/layerling"
	"aurago/internal/security"
	"context"
	"encoding/json"
	openai "github.com/sashabaranov/go-openai"
)

func layerlingSchema() openai.Tool {
	return tool("layerling", "Control an open Layerling 3D CAD editor in the authenticated Desktop session. List editors, then use an explicit editor_id. Describe an operation before using its JSON arguments. No headless CAD.", schema(map[string]interface{}{
		"operation": operationProperty("CAD or Desktop file operation; describe returns its pinned parameter schema", layerling.Operations()),
		"editor_id": prop("string", "Exact editor_id from list_editors or this window context; never guess the active editor"),
		"arguments": prop("string", "JSON object matching the operation schema. describe: {\"operation\":\"create_shape\"}; list_editors: {}"),
	}, "operation"))
}
func dispatchLayerling(ctx context.Context, tc ToolCall, dc *DispatchContext) string {
	operation := firstNonEmptyToolString(toolArgString(tc.Params, "operation"), tc.Operation)
	if dc.Cfg == nil {
		return `Tool Output: {"status":"error","message":"Layerling unavailable"}`
	}
	d := dc.Cfg.VirtualDesktop
	read := layerling.ReadOnly(operation) || operation == "list_editors" || operation == "describe"
	if !d.Enabled || !d.Layerling.Enabled || !d.AllowAgentControl || !dc.Cfg.Tools.VirtualDesktop.Enabled || (d.Layerling.AgentAccess != "read" && d.Layerling.AgentAccess != "write") || (!read && (d.ReadOnly || d.Layerling.AgentAccess != "write")) {
		return `Tool Output: {"status":"error","message":"Layerling access denied"}`
	}
	args := toolArgString(tc.Params, "arguments")
	if args == "" {
		args = "{}"
	}
	data, err := layerling.Execute(ctx, operation, toolArgString(tc.Params, "editor_id"), json.RawMessage(args))
	if err != nil {
		data, _ = json.Marshal(map[string]string{"status": "error", "message": err.Error()})
	}
	return "Tool Output: " + security.IsolateExternalData(string(data))
}
