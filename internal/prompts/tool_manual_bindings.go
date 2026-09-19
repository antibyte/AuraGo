package prompts

import "strings"

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
		return "The native schema documents this single message operation; channel and recipient policy is supplied by the active run."
	}
	return ""
}
