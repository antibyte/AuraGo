package security

import "strings"

type toolOutputTrust int

const (
	toolOutputTrusted toolOutputTrust = iota
	toolOutputSemiTrusted
	toolOutputExternal
)

func classifyToolOutput(action string) toolOutputTrust {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "activate_tools", "context_manager", "discover_tools", "get_tool_info":
		return toolOutputTrusted
	case "execute_shell", "execute_python", "run_tool":
		return toolOutputSemiTrusted
	case
		"adguard",
		"adguard_home",
		"agentmail",
		"ansible",
		"api_request",
		"brave_search",
		"browser_automation",
		"call_webhook",
		"check_email",
		"cloudflare_tunnel",
		"co_agent",
		"co_agents",
		"contacts_search",
		"ddg_search",
		"discord",
		"discord_read",
		"docker",
		"email",
		"execute_remote_shell",
		"execute_skill",
		"fetch_discord",
		"fetch_email",
		"fetch_url",
		"file_reader",
		"file_reader_advanced",
		"file_search",
		"filesystem",
		"filesystem_op",
		"fritzbox",
		"github",
		"google_calendar",
		"google_workspace",
		"gworkspace",
		"home_assistant",
		"influxdb_query",
		"jellyfin",
		"joplin_note",
		"matrix_read",
		"mcp_call",
		"meshcentral",
		"mqtt_get_messages",
		"netlify",
		"nextcloud",
		"notion",
		"obsidian",
		"paperless",
		"paperless_ngx",
		"proxmox",
		"proxmox_ve",
		"read_tool_output",
		"remote_execution",
		"retrieve_original_output",
		"rocket_chat_read",
		"rss_read",
		"s3",
		"s3_storage",
		"site_crawler",
		"smart_file",
		"smart_file_read",
		"sql_query",
		"tailscale",
		"telegram_read",
		"truenas",
		"virustotal_scan",
		"web_capture",
		"web_scraper",
		"webdav",
		"webdav_storage",
		"wikipedia_search",
		"yepapi_amazon",
		"yepapi_discover",
		"yepapi_fetch",
		"yepapi_instagram",
		"yepapi_scrape",
		"yepapi_search",
		"yepapi_seo",
		"yepapi_serp",
		"yepapi_tiktok",
		"yepapi_youtube":
		return toolOutputExternal
	default:
		return toolOutputExternal
	}
}
