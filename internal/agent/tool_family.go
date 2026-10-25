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
	case strings.Contains(name, "shell") || strings.Contains(name, "sudo") || name == "process_analyzer" || name == "process_management" || name == "system_metrics":
		return "shell"
	case strings.Contains(name, "python") || strings.Contains(name, "sandbox") || strings.Contains(name, "skill") || name == "generate_image" || name == "generate_video" || name == "document_creator":
		return "coding"
	case strings.Contains(name, "memory") || name == "remember" || name == "knowledge_graph" || name == "cheatsheet" || strings.Contains(name, "journal") || strings.Contains(name, "notes"):
		return "memory"
	case strings.Contains(name, "web_") || name == "site_crawler" || name == "api_request" || name == "virustotal_scan" || name == "form_automation" || name == "local_wikipedia":
		return "web"
	case strings.Contains(name, "homepage") || name == "netlify" || strings.HasPrefix(name, "here_now_") || strings.Contains(name, "update") || name == "cloudflare_tunnel":
		return "deployment"
	case strings.Contains(name, "network") || strings.Contains(name, "dns_") || strings.Contains(name, "port_") || strings.Contains(name, "mdns_") || strings.Contains(name, "whois") || strings.Contains(name, "upnp") || strings.Contains(name, "wake_on_lan") || strings.Contains(name, "fritzbox"):
		return "network"
	case name == "docker" || name == "proxmox" || name == "frigate" || name == "tailscale" || name == "ansible" || name == "github" || name == "mcp_call" || strings.HasPrefix(name, "sql_") || name == "manage_sql_connections" || strings.Contains(name, "meshcentral") || strings.Contains(name, "remote_") || name == "invasion_control" || name == "home_assistant" || name == "ollama" || name == "adguard" || strings.HasPrefix(name, "mqtt_") || name == "s3_storage" || name == "jellyfin" || name == "truenas" || strings.HasPrefix(name, "jellyfin_") || strings.HasPrefix(name, "truenas_") || name == "koofr" || name == "onedrive" || strings.HasPrefix(name, "koofr_") || strings.HasPrefix(name, "onedrive_"):
		return "infra"
	case strings.Contains(name, "email") || strings.Contains(name, "webhook") || strings.Contains(name, "telnyx") || name == "address_book":
		return "communication"
	case strings.Contains(name, "cron") || strings.Contains(name, "follow_up") || strings.Contains(name, "mission") || name == "co_agent" || name == "co_agents":
		return "automation"
	case strings.Contains(name, "image") || strings.Contains(name, "audio") || strings.Contains(name, "video") || name == "tts" || strings.Contains(name, "transcribe") || strings.Contains(name, "media_") || name == "send_document" || name == "retro_games":
		return "media"
	default:
		return "misc"
	}
}

// inferToolFamilyFromQuery names the tool family a user message is about, or
// "" for none. The message is matched by whole words, in English and German
// (toolFamilyTermSets); the family with the most distinct terms wins and a tie
// goes to the earlier family of the table, which keeps the long-standing
// precedence (deployment, files, shell, coding, memory, web, network, infra,
// communication, automation, media). A request for CPU or RAM usage counts as
// a shell term.
func inferToolFamilyFromQuery(query string) string {
	return toolFamilyOfText(newToolIntentText(query))
}

// toolFamilyOfText is inferToolFamilyFromQuery for a message that is already
// split into words.
func toolFamilyOfText(text toolIntentText) string {
	if len(text.words) == 0 {
		return ""
	}
	best, bestScore := "", 0
	for i, set := range toolFamilyTermSets {
		score := toolFamilyTerms[i].count(text)
		if set.family == "shell" && isResourceUsageIntent(text) {
			score++
		}
		if score > bestScore {
			best, bestScore = set.family, score
		}
	}
	return best
}
