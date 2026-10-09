package prompts

import (
	"slices"
	"strings"
)

// NonDisplacingTools are optional tools that must never take a slot from
// another tool or guide. The adaptive tool selection offers them only beyond
// its ranking, and every top-k search over the tool manual index drops their
// manuals before cutting to k while the manual cannot be used (the tool is
// disabled or its guide is skipped), so the results are exactly those of an
// index without these manuals.
var NonDisplacingTools = []string{"local_wikipedia"}

// IsNonDisplacingManual reports whether a manual ID (as ToolManualID returns
// it, or a manual file name without ".md") belongs to a NonDisplacingTools tool.
func IsNonDisplacingManual(manual string) bool {
	return slices.ContainsFunc(NonDisplacingTools, func(tool string) bool { return ToolManualID(tool) == manual })
}

// nonDisplacingToolEnabled reports whether the prompt flags enable the
// NonDisplacingTools tool behind a manual; without flags (callers that do not
// pass them) only the allowlist decides.
func nonDisplacingToolEnabled(manual string, flags *ContextFlags) bool {
	if flags == nil {
		return true
	}
	switch manual {
	case ToolManualID("local_wikipedia"):
		return flags.LocalWikipediaEnabled
	}
	return true
}

// ToolManualID records family documentation once for both discovery and guides.
// Tools with dedicated manuals keep their exact ID.
func ToolManualID(name string) string {
	switch name {
	case "mcp_call":
		return "mcp"
	case "mqtt_publish", "mqtt_subscribe", "mqtt_unsubscribe", "mqtt_get_messages":
		return "mqtt"
	case "send_email", "fetch_email", "list_email_accounts":
		return "email"
	case "send_discord", "fetch_discord", "list_discord_channels":
		return "discord"
	case "create_skill_from_template", "list_skill_templates":
		return "skill_templates"
	case "get_skill_documentation", "set_skill_documentation", "list_skills", "save_tool":
		return "skills_engine"
	case "call_webhook", "manage_outgoing_webhooks":
		return "manage_webhooks"
	case "execute_sandbox":
		return "sandbox"
	case "manage_memory":
		return "core_memory"
	case "mdns_scan":
		return "mdns"
	case "paperless_ngx":
		return "paperless"
	case "query_inventory", "register_device":
		return "remote_control_devices"
	case "transfer_remote_file":
		return "remote_control_files"
	case "retrieve_original_output":
		return "read_tool_output"
	case "telnyx_call", "telnyx_manage", "telnyx_sms":
		return "telnyx"
	case "treg_catalog", "treg_call", "treg_status":
		return "treg"
	case "here_now_site", "here_now_sites":
		return "homepage"
	}
	if strings.HasPrefix(name, "yepapi_") {
		return "yepapi"
	}
	return name
}

func ToolManualAbsenceReason(name string) string {
	switch name {
	case "game_maker_asset", "game_maker_file", "game_maker_project", "game_maker_validate":
		return "Phase-specific schemas and required Game Maker execution guidance are supplied by the isolated Studio run."
	case "send_telegram":
		return "The native schema documents this message operation, including the optional file_path document; channel and recipient policy is supplied by the active run."
	}
	return ""
}
