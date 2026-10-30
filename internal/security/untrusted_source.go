package security

import "strings"

// UntrustedContentTool reports whether a tool returns text that a third party
// wrote: everything ExternalToolSource covers (web and mail) plus chat channels,
// collaboration and code hosting services, MCP servers, delegated agents and
// other remote content. It is the provenance signal behind the session taint
// (a session that read such text may have been steered by it). Local files,
// shell, Docker, Home Assistant and system telemetry are not untrusted for this
// predicate; neither are AuraGo's own stores (IsInternalToolOutput). The
// decision is by trusted tool identity, never by payload. Shell and code tools
// are not listed here: whether their output is untrusted depends on the call
// (a command that fetches from the network), which the agent package decides.
func UntrustedContentTool(tool, operation string) bool {
	if ExternalToolSource(tool, operation) != "" {
		return true
	}
	tool = strings.ToLower(strings.TrimSpace(tool))
	if strings.HasPrefix(tool, "gog_") {
		return true
	}
	switch tool {
	case "discord", "discord_read", "fetch_discord",
		"telegram_read", "matrix_read", "rocket_chat_read", "rocketchat",
		"telnyx_sms", "meshcore",
		"github", "notion", "nextcloud", "paperless", "paperless_ngx",
		"mcp_call", "composio_call", "manus", "evomap", "huggingface",
		"google_workspace", "gworkspace", "google_calendar", "call_webhook",
		"co_agent", "co_agents", "space_agent",
		"local_wikipedia", "virustotal_scan", "mqtt_get_messages":
		return true
	}
	return false
}
