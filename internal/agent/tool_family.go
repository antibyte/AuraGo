package agent

import "strings"

func classifyToolFamily(toolName string) string {
	name := strings.ToLower(strings.TrimSpace(toolName))
	if name == "" {
		return ""
	}
	switch {
	case strings.HasPrefix(name, "file") || name == "filesystem" || name == "smart_file_read" || strings.Contains(name, "_editor") || name == "pdf_operations" || name == "detect_file_type" || name == "archive":
		return "files"
	case strings.Contains(name, "shell") || strings.Contains(name, "sudo") || name == "process_analyzer" || name == "process_management":
		return "shell"
	case strings.Contains(name, "python") || strings.Contains(name, "sandbox") || strings.Contains(name, "skill") || name == "generate_image" || name == "document_creator":
		return "coding"
	case strings.Contains(name, "memory") || name == "remember" || name == "knowledge_graph" || name == "cheatsheet" || strings.Contains(name, "journal") || strings.Contains(name, "notes"):
		return "memory"
	case strings.Contains(name, "web_") || name == "site_crawler" || name == "api_request" || name == "virustotal_scan" || name == "form_automation":
		return "web"
	case strings.Contains(name, "homepage") || name == "netlify" || strings.Contains(name, "update") || name == "cloudflare_tunnel":
		return "deployment"
	case strings.Contains(name, "network") || strings.Contains(name, "dns_") || strings.Contains(name, "port_") || strings.Contains(name, "mdns_") || strings.Contains(name, "whois") || strings.Contains(name, "upnp") || strings.Contains(name, "wake_on_lan") || strings.Contains(name, "fritzbox"):
		return "network"
	case name == "docker" || name == "proxmox" || name == "tailscale" || name == "ansible" || name == "github" || name == "mcp_call" || strings.HasPrefix(name, "sql_") || name == "manage_sql_connections" || strings.Contains(name, "meshcentral") || strings.Contains(name, "remote_") || name == "invasion_control" || name == "home_assistant" || name == "ollama" || name == "adguard" || strings.HasPrefix(name, "mqtt_") || name == "s3_storage":
		return "infra"
	case strings.Contains(name, "email") || strings.Contains(name, "webhook") || strings.Contains(name, "telnyx") || name == "address_book":
		return "communication"
	case strings.Contains(name, "cron") || strings.Contains(name, "follow_up") || strings.Contains(name, "mission") || name == "co_agent" || name == "co_agents":
		return "automation"
	case strings.Contains(name, "image") || strings.Contains(name, "audio") || name == "tts" || strings.Contains(name, "transcribe") || strings.Contains(name, "media_"):
		return "media"
	default:
		return "misc"
	}
}

func inferToolFamilyFromQuery(query string) string {
	q := normalizeAdaptiveIntentText(query)
	if q == "" {
		return ""
	}

	switch {
	case strings.Contains(q, "homepage") || strings.Contains(q, "landing page") || strings.Contains(q, "website") || strings.Contains(q, "netlify") || strings.Contains(q, "deploy"):
		return "deployment"
	case strings.Contains(q, "file") || strings.Contains(q, "folder") || strings.Contains(q, "directory") || strings.Contains(q, "yaml") || strings.Contains(q, "json") || strings.Contains(q, "xml") || strings.Contains(q, "pdf"):
		return "files"
	case strings.Contains(q, "shell") || strings.Contains(q, "command") || strings.Contains(q, "terminal") || strings.Contains(q, "script") || strings.Contains(q, "bash") || strings.Contains(q, "powershell"):
		return "shell"
	case strings.Contains(q, "python") || strings.Contains(q, "code") || strings.Contains(q, "coding") || strings.Contains(q, "program"):
		return "coding"
	case strings.Contains(q, "memory") || strings.Contains(q, "remember") || strings.Contains(q, "note") || strings.Contains(q, "journal") || strings.Contains(q, "knowledge"):
		return "memory"
	case strings.Contains(q, "scrape") || strings.Contains(q, "website audit") || strings.Contains(q, "web capture") || strings.Contains(q, "api") || strings.Contains(q, "form"):
		return "web"
	case strings.Contains(q, "network") || strings.Contains(q, "dns") || strings.Contains(q, "ping") || strings.Contains(q, "port") || strings.Contains(q, "scan") || strings.Contains(q, "fritz") || strings.Contains(q, "wake on lan"):
		return "network"
	case strings.Contains(q, "docker") || strings.Contains(q, "proxmox") || strings.Contains(q, "tailscale") || strings.Contains(q, "ansible") || strings.Contains(q, "github") || strings.Contains(q, "remote") || strings.Contains(q, "mqtt") || strings.Contains(q, "sql"):
		return "infra"
	case strings.Contains(q, "email") || strings.Contains(q, "webhook") || strings.Contains(q, "sms") || strings.Contains(q, "call") || strings.Contains(q, "contact"):
		return "communication"
	case strings.Contains(q, "schedule") || strings.Contains(q, "cron") || strings.Contains(q, "mission") || strings.Contains(q, "automation") || strings.Contains(q, "follow up"):
		return "automation"
	case strings.Contains(q, "image") || strings.Contains(q, "audio") || strings.Contains(q, "speech") || strings.Contains(q, "media"):
		return "media"
	default:
		return ""
	}
}
