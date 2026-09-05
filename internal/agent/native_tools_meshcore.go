package agent

import openai "github.com/sashabaranov/go-openai"

func meshCoreToolSchema() openai.Tool {
	return tool("meshcore", "Read MeshCore status, contacts and channels, or proactively send a short text to an explicitly allowed destination. Sending requires meshcore.proactive_send and a destination allowlist; replies to inbound messages are managed internally. Never manage radio firmware or keys.", schema(map[string]interface{}{
		"operation": operationProperty("Operation", []string{"status", "contacts", "channels", "send_direct", "send_channel"}),
		"node_key":  prop("string", "Full 64-character public key for send_direct."),
		"channel":   prop("integer", "Configured channel slot for send_channel."),
		"text":      prop("string", "Short message, at most three radio packets."),
	}, "operation"))
}
