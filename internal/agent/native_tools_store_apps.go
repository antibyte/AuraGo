package agent

import openai "github.com/sashabaranov/go-openai"

func appendStoreAppSchemas(out []openai.Tool, ff ToolFeatureFlags) []openai.Tool {
	page := func(operations ...string) map[string]interface{} {
		return map[string]interface{}{
			"operation": map[string]interface{}{"type": "string", "enum": operations},
			"limit":     map[string]interface{}{"type": "integer", "minimum": 1, "maximum": 50, "description": "Page size (default 25)."},
			"offset":    map[string]interface{}{"type": "integer", "minimum": 0, "description": "Page offset (default 0)."},
		}
	}
	if ff.NodeREDEnabled {
		properties := page("list_flows", "trigger")
		properties["node_id"] = prop("string", "Administratively allowed Inject ID, required for trigger; uses the node's existing payload.")
		out = append(out, tool("node_red", "List existing Node-RED flow names and allowed Inject nodes, or trigger one. A trigger acknowledges acceptance, not completion. No flow editing or runtime deployment.", schema(properties, "operation")))
	}
	if ff.OliveTinEnabled {
		properties := page("list_actions", "execute", "get_execution")
		properties["action_id"] = prop("string", "Allowed action binding ID, required for execute and get_execution.")
		properties["execution_id"] = prop("string", "Execution tracking ID returned by execute; required for get_execution.")
		properties["arguments"] = map[string]interface{}{"type": "array", "maxItems": 64, "description": "Configured action arguments only; no shell commands.", "items": schema(map[string]interface{}{"name": prop("string", "Argument name from the action definition."), "value": prop("string", "Argument value, validated by OliveTin.")}, "name", "value")}
		out = append(out, tool("olivetin", "List allowed OliveTin actions, execute a predefined action, or inspect its execution. Unknown outcomes must not be retried automatically.", schema(properties, "operation")))
	}
	return out
}
