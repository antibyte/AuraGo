package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

func dispatchInvokeTool(ctx context.Context, tc ToolCall, dc *DispatchContext) string {
	// Direct callers share the same normalized policy and hook path. The public
	// dispatcher resolves wrappers before Guardian, so this does not recurse.
	tc.Action = "invoke_tool"
	routed := prepareToolCall(tc, dc)
	if routed.PreparationError != "" {
		return routed.PreparationError
	}
	return dispatchInner(ctx, routed, dc)
}

func flattenedInvokeArgs(params map[string]interface{}) map[string]interface{} {
	args := make(map[string]interface{})
	for key, value := range params {
		switch key {
		// invoke_tool's own metadata keys (and their aliases)
		case "tool_name", "name", "tool", "arguments", "params", "skill_args":
			continue
		// Note: "action" is intentionally NOT filtered here because many
		// target tools (e.g. filesystem, docker, proxmox) use "action" as a
		// legitimate parameter. Only keys that belong to invoke_tool itself
		// should be stripped.
		default:
			args[key] = value
		}
	}
	return args
}

func toolCallFromInvokeArgs(action string, args map[string]interface{}) ToolCall {
	routed := ToolCall{Action: action, Params: args}
	if len(args) == 0 {
		return routed
	}
	raw, err := json.Marshal(args)
	if err == nil {
		_ = json.Unmarshal(raw, &routed)
	}
	routed.Action = action
	routed.Params = args
	if routed.Operation == "" {
		routed.Operation = stringValueFromMap(args, "operation")
	}
	return routed
}

func dispatchCommMustHandle(ctx context.Context, tc ToolCall, dc *DispatchContext) string {
	if result, ok := dispatchComm(ctx, tc, dc); ok {
		return result
	}
	return fmt.Sprintf(`Tool Output: {"status":"error","message":"%s could not be routed"}`, strings.TrimSpace(tc.Action))
}

func dispatchExecMustHandle(ctx context.Context, tc ToolCall, dc *DispatchContext) string {
	if result, ok := dispatchExec(ctx, tc, dc); ok {
		return result
	}
	return fmt.Sprintf(`Tool Output: {"status":"error","message":"%s could not be routed"}`, strings.TrimSpace(tc.Action))
}

func mapValueFromMap(m map[string]interface{}, keys ...string) map[string]interface{} {
	for _, key := range keys {
		raw, ok := m[key]
		if !ok {
			continue
		}
		if value, ok := raw.(map[string]interface{}); ok {
			return value
		}
	}
	return nil
}
