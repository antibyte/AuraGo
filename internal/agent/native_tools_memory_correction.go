package agent

import openai "github.com/sashabaranov/go-openai"

func memoryCorrectionToolSchema() openai.Tool {
	reference := schema(map[string]interface{}{
		"store":    operationProperty("Memory store", []string{"core", "ltm", "profile", "kg_node", "kg_edge"}),
		"id":       prop("string", "Core numeric ID, LTM document ID, node ID or edge source ID from query_memory."),
		"field":    prop("string", "Exact graph property, or label; required for kg_node."),
		"category": prop("string", "Profile category; required for profile."),
		"key":      prop("string", "Profile key; required for profile."),
		"target":   prop("string", "Existing edge target; required for kg_edge."),
		"relation": prop("string", "Exact existing predicate; required for kg_edge."),
	}, "store")
	return tool("correct_memory", "Correct or archive precisely identified current facts across core, long-term memory, graph and user profile. Preview requires real current-run evidence. Apply only the saved ID, then check status. On clear evidence perform preview and apply without another confirmation. Personal facts require user statements. Similarity alone is never authorization; report unresolved candidates and partial progress. Never rewrite historical conversations or source documents.", schema(map[string]interface{}{
		"operation": operationProperty("preview, apply or status", []string{"preview", "apply", "status"}),
		"id":        prop("string", "Saved correction ID; apply/status accept only operation and id."),
		"reason":    prop("string", "Why the current fact is wrong or outdated; preview only."),
		"changes": map[string]interface{}{"type": "array", "maxItems": 32, "items": schema(map[string]interface{}{
			"reference": reference,
			"operation": operationProperty("Change", []string{"replace", "archive"}),
			"value":     prop("string", "Replacement text; for an edge the new existing target ID. Omit for archive."),
		}, "reference", "operation")},
		"evidence": map[string]interface{}{"type": "array", "maxItems": 8, "items": schema(map[string]interface{}{
			"reference": prop("string", "user:current for the actual current user message, or a runtime-issued tool: reference from a successful system_metrics or remote sysinfo result."),
			"quote":     prop("string", "Exact supporting quote from that message/result; invented evidence and confirmation flags are rejected."),
		}, "reference", "quote")},
	}, "operation"))
}
