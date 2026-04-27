package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

func dispatchInvokeTool(ctx context.Context, tc ToolCall, dc *DispatchContext) string {
	toolName := stringValueFromMap(tc.Params, "tool_name", "name")
	if toolName == "" {
		logInvalidToolCommand(dc.Logger, "invoke_tool", "", "missing_tool_name", tc.Params)
		return `Tool Output: {"status":"error","message":"invoke_tool requires tool_name"}`
	}
	args := mapValueFromMap(tc.Params, "arguments", "params", "skill_args")
	if args == nil {
		args = flattenedInvokeArgs(tc.Params)
		logInvokeToolArgumentSource(dc.Logger, toolName, "flattened", tc.Params, args)
	} else if dc.Logger != nil {
		dc.Logger.Debug("[ToolCommand] invoke_tool received nested arguments",
			"tool", toolName,
			"input_keys", safeToolParamKeys(tc.Params),
			"arg_keys", safeToolParamKeys(args),
		)
	}

	catalog := GetToolCatalogState()
	if catalog == nil {
		logInvalidToolCommand(dc.Logger, toolName, stringValueFromMap(args, "operation"), "catalog_unavailable", args)
		return `Tool Output: {"status":"error","message":"tool catalog is not available yet; call discover_tools first"}`
	}
	entry, ok := catalog.Get(toolName)
	if !ok {
		logInvalidToolCommand(dc.Logger, toolName, stringValueFromMap(args, "operation"), "tool_not_found", args)
		b, _ := json.Marshal(fmt.Sprintf("Tool '%s' not found in catalog", toolName))
		return fmt.Sprintf(`Tool Output: {"status":"error","message":%s}`, b)
	}
	if !entry.Enabled || entry.Status == ToolStatusDisabled {
		logInvalidToolCommand(dc.Logger, entry.Name, stringValueFromMap(args, "operation"), "tool_disabled", args)
		b, _ := json.Marshal(fmt.Sprintf("Tool '%s' is disabled in config", entry.Name))
		return fmt.Sprintf(`Tool Output: {"status":"error","message":%s}`, b)
	}
	if entry.Name == "invoke_tool" {
		logInvalidToolCommand(dc.Logger, entry.Name, stringValueFromMap(args, "operation"), "self_invocation", args)
		return `Tool Output: {"status":"error","message":"invoke_tool cannot invoke itself"}`
	}
	if strings.HasPrefix(entry.Name, "yepapi_") && stringValueFromMap(args, "operation") == "" {
		logInvalidToolCommand(dc.Logger, entry.Name, "", "missing_operation", args)
	}

	if entry.Kind == ToolKindNative && entry.Status == ToolStatusHidden {
		MarkDiscoverRequestedTool(dc.SessionID, entry.Name)
	}

	switch entry.Kind {
	case ToolKindNative, ToolKindMCP:
		action := entry.Routing.NativeAction
		if action == "" {
			action = entry.Name
		}
		routed := toolCallFromInvokeArgs(action, args)
		if result, ok := dispatchExec(ctx, routed, dc); ok {
			return result
		}
		if result, ok := dispatchComm(ctx, routed, dc); ok {
			return result
		}
		if result, ok := dispatchServices(ctx, routed, dc); ok {
			return result
		}
		if result, ok := dispatchInfra(ctx, routed, dc); ok {
			return result
		}
	case ToolKindSkill:
		return dispatchCommMustHandle(ctx, ToolCall{Action: "execute_skill", Skill: entry.Routing.SkillName, SkillArgs: args, Params: args}, dc)
	case ToolKindCustom:
		return dispatchExecMustHandle(ctx, ToolCall{Action: "run_tool", Name: entry.Routing.CustomName, Params: map[string]interface{}{"name": entry.Routing.CustomName, "args": args}}, dc)
	}

	b, _ := json.Marshal(fmt.Sprintf("Tool '%s' cannot be invoked through invoke_tool", entry.Name))
	return fmt.Sprintf(`Tool Output: {"status":"error","message":%s}`, b)
}

func flattenedInvokeArgs(params map[string]interface{}) map[string]interface{} {
	args := make(map[string]interface{})
	for key, value := range params {
		switch key {
		case "tool_name", "name", "action", "tool", "arguments", "params", "skill_args":
			continue
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
