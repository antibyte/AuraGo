package agent

import (
	"fmt"
	"strings"
	"sync"

	openai "github.com/sashabaranov/go-openai"
)

var discoverToolAliases = map[string]string{
	"mcp":                    "mcp_call",
	"mcp tool":               "mcp_call",
	"mcp tools":              "mcp_call",
	"mcp server":             "mcp_call",
	"mcp servers":            "mcp_call",
	"model context protocol": "mcp_call",
	"wikipedia":              "wikipedia_search",
	"wiki":                   "wikipedia_search",
	"duckduckgo":             "ddg_search",
	"duck duck go":           "ddg_search",
	"ddg":                    "ddg_search",
	"brave":                  "brave_search",
	"pdf extractor":          "pdf_extractor",
}

func resolveDiscoverToolName(name string) string {
	normalized := strings.ToLower(strings.TrimSpace(name))
	if normalized == "" {
		return ""
	}
	if alias, ok := discoverToolAliases[normalized]; ok {
		return alias
	}
	return strings.TrimSpace(name)
}

// ToolCategoryEntry describes a single tool within a category.
type ToolCategoryEntry struct {
	Name      string
	ShortDesc string
}

// toolCategoryDef maps each category to its member tools with short descriptions.
var toolCategoryDef = map[string][]ToolCategoryEntry{
	"system": {
		{"system_metrics", "CPU, memory, disk, network usage and host info"},
		{"process_analyzer", "Find top CPU/memory consumers, search and inspect OS processes"},
		{"process_management", "List, kill, or inspect AuraGo background processes"},
		{"manage_updates", "Check for and install AuraGo updates"},
		{"execute_sudo", "Run a shell command with sudo privileges"},
		{"execute_sandbox", "Run code in an isolated sandbox environment"},
		{"execute_shell", "Execute shell commands on the host system"},
		{"execute_python", "Execute Python code in a sandboxed virtual environment"},
		{"upnp_scan", "Discover UPnP/SSDP devices on the local network"},
		{"cron_scheduler", "Create, list, pause, resume, and delete scheduled cron jobs"},
		{"manage_plan", "Manage multi-step execution plans"},
		{"manage_missions", "Manage long-running autonomous missions"},
		{"manage_appointments", "Manage calendar appointments and reminders"},
		{"manage_todos", "Manage todo lists and tasks"},
		{"follow_up", "Schedule autonomous background tasks"},
		{"question_user", "Ask the user a question with predefined options"},
		{"wait_for_event", "Wait for a process, HTTP endpoint, or file event"},
	},
	"files": {
		{"filesystem", "Read, write, move, copy, delete files and directories"},
		{"file_editor", "Edit files with find-and-replace, insert, delete operations"},
		{"json_editor", "Query and modify JSON files using JSONPath"},
		{"yaml_editor", "Query and modify YAML files"},
		{"xml_editor", "Query and modify XML files using XPath"},
		{"text_diff", "Compare files or strings with unified diff"},
		{"file_search", "Grep text patterns across files or find files by name"},
		{"file_reader_advanced", "Read line ranges, head/tail, and contextual search in large files"},
		{"smart_file_read", "Intelligently inspect large files: analyze, sample, structure, summarize"},
		{"archive", "Create and extract tar.gz and zip archives"},
		{"pdf_extractor", "Extract and optionally summarise text from PDF documents"},
		{"pdf_operations", "Read, merge, split, extract text/images from PDFs"},
		{"image_processing", "Resize, crop, rotate, convert, compress, watermark images"},
		{"detect_file_type", "Identify file type by content (magic bytes)"},
		{"document_creator", "Create PDF, DOCX, XLSX, PPTX documents from structured data"},
		{"koofr", "Access Koofr cloud storage: list, read, download, upload, move, and copy files"},
	},
	"network": {
		{"api_request", "Make HTTP requests (GET, POST, PUT, DELETE, etc.)"},
		{"dns_lookup", "DNS resolution for any record type (A, AAAA, MX, TXT, etc.)"},
		{"port_scanner", "Scan TCP ports on a host"},
		{"network_ping", "Ping hosts to check availability and latency"},
		{"ddg_search", "Search the web with DuckDuckGo and summarise results"},
		{"wikipedia_search", "Look up encyclopedic topics and summaries on Wikipedia"},
		{"brave_search", "Search the web with Brave Search"},
		{"web_scraper", "Scrape web pages and extract content"},
		{"site_crawler", "Crawl websites following links and extracting data"},
		{"site_monitor", "Monitor websites for availability and changes"},
		{"web_capture", "Take screenshots and capture web pages"},
		{"browser_automation", "Drive a full browser session with clicks, typing, screenshots, and downloads"},
		{"web_performance_audit", "Lighthouse-style performance audit of a URL"},
		{"form_automation", "Automate web form submissions via headless browser"},
		{"whois_lookup", "WHOIS domain registration and ownership lookup"},
		{"mdns_scan", "Discover mDNS/Bonjour services on the local network"},
		{"mac_lookup", "Look up device manufacturer by MAC address"},
		{"firewall", "Manage UFW/iptables firewall rules"},
		{"cloudflare_tunnel", "Manage Cloudflare Tunnel connections"},
	},
	"media": {
		{"generate_image", "Generate images using AI (DALL-E, Stable Diffusion, etc.)"},
		{"generate_music", "Generate music tracks using AI"},
		{"generate_video", "Generate short videos using AI"},
		{"tts", "Text-to-Speech: convert text to audio"},
		{"send_image", "Send an image to the user"},
		{"send_audio", "Send an audio file to the user"},
		{"send_video", "Send a video file to the user"},
		{"send_youtube_video", "Send a YouTube video as an embed or link"},
		{"video_download", "Search, inspect, download, or transcribe videos when permitted"},
		{"send_document", "Send a document to the user"},
		{"analyze_image", "Analyze images using Vision LLM (OCR, describe, identify)"},
		{"transcribe_audio", "Transcribe audio files to text (Speech-to-Text)"},
		{"media_conversion", "Convert audio, video, and image files between formats"},
		{"chromecast", "Cast media to Chromecast devices"},
		{"media_registry", "Manage and search local media files"},
		{"jellyfin", "Control Jellyfin media server (libraries, playback, users)"},
	},
	"smart_home": {
		{"home_assistant", "Control Home Assistant devices (lights, switches, sensors, etc.)"},
		{"fritzbox_system", "Fritz!Box system info, logs, and reboot"},
		{"fritzbox_network", "Fritz!Box network devices, port forwarding, WLAN"},
		{"fritzbox_telephony", "Fritz!Box call list, phonebook, DECT phones"},
		{"fritzbox_smarthome", "Fritz!Box smart home devices (thermostats, switches, sensors)"},
		{"fritzbox_storage", "Fritz!Box NAS/USB storage management"},
		{"fritzbox_tv", "Fritz!Box TV streaming and EPG"},
		{"mqtt_publish", "Publish messages to MQTT topics"},
		{"mqtt_subscribe", "Subscribe to MQTT topics"},
		{"mqtt_unsubscribe", "Unsubscribe from MQTT topics"},
		{"mqtt_get_messages", "Get received MQTT messages from subscriptions"},
		{"wake_on_lan", "Send Wake-on-LAN magic packets to devices"},
		{"adguard", "Manage AdGuard Home DNS: filtering, rewrites, DHCP, clients"},
		{"uptime_kuma", "Read monitor health and outages from Uptime Kuma"},
		{"grafana", "Read Grafana dashboards, data sources, alerts, health, and org info"},
	},
	"infrastructure": {
		{"docker", "Manage Docker containers (list, start, stop, logs, exec, compose)"},
		{"proxmox", "Manage Proxmox VMs and containers (list, start, stop, clone, etc.)"},
		{"tailscale", "Manage Tailscale VPN network (devices, routes, ACL)"},
		{"ansible", "Run Ansible playbooks and manage infrastructure"},
		{"github", "GitHub repos, issues, PRs, actions, and code search"},
		{"remote_execution", "Execute commands on remote devices via SSH"},
		{"transfer_remote_file", "Transfer files to/from remote devices via SFTP"},
		{"s3_storage", "Manage AWS S3 buckets and objects"},
		{"sql_query", "Execute SQL queries on external databases"},
		{"manage_sql_connections", "Manage external SQL database connections"},
		{"ollama", "Manage local Ollama LLM models (list, pull, run, delete)"},
		{"truenas", "Manage TrueNAS storage (pools, datasets, snapshots, shares)"},
		{"netlify", "Deploy and manage sites on Netlify"},
		{"homepage", "Manage homepage web projects (create, edit, deploy, preview)"},
		{"meshcentral", "Remote desktop via MeshCentral (screenshot, command, file transfer)"},
		{"invasion_control", "Manage distributed egg/nest compute nodes"},
		{"mcp_call", "Call tools on connected MCP (Model Context Protocol) servers"},
	},
	"data_apis": {
		{"yepapi_seo", "SEO data via YepAPI: domain overviews, keywords, competitors, and backlinks"},
		{"yepapi_serp", "Search engine results via YepAPI: Google, Google Maps, News, Images, and autocomplete"},
		{"yepapi_scrape", "Scrape and extract web page content through YepAPI"},
		{"yepapi_youtube", "YouTube data via YepAPI: search, videos, transcripts, comments, channels, and playlists"},
		{"yepapi_tiktok", "TikTok data via YepAPI: videos, users, posts, comments, music, and challenges"},
		{"yepapi_instagram", "Instagram data via YepAPI: users, posts, reels, comments, hashtags, and places"},
		{"yepapi_amazon", "Amazon data via YepAPI: products, reviews, offers, categories, and search"},
	},
	"communication": {
		{"send_email", "Send emails via configured SMTP accounts"},
		{"fetch_email", "Fetch and read emails from IMAP accounts"},
		{"list_email_accounts", "List configured email accounts"},
		{"send_discord", "Send messages to Discord channels"},
		{"fetch_discord", "Read messages from Discord channels"},
		{"list_discord_channels", "List available Discord channels"},
		{"telnyx_sms", "Send and receive SMS via Telnyx"},
		{"telnyx_call", "Make and manage phone calls via Telnyx"},
		{"telnyx_manage", "Manage Telnyx phone numbers and messaging profiles"},
		{"call_webhook", "Trigger outgoing webhook requests"},
		{"manage_outgoing_webhooks", "Configure and manage outgoing webhooks"},
		{"manage_webhooks", "Manage incoming webhook endpoints"},
		{"co_agent", "Delegate tasks to specialized co-agent LLMs"},
		{"address_book", "Manage contacts and address book entries"},
	},
}

// toolCategoryOrder defines the display order of categories.
var toolCategoryOrder = []string{
	"system", "files", "network", "media", "smart_home", "infrastructure", "data_apis", "communication",
}

// toolCategoryLabels provides human-readable labels for categories.
var toolCategoryLabels = map[string]string{
	"system":         "System & Automation",
	"files":          "Files & Documents",
	"network":        "Network & Web",
	"media":          "Media & Content",
	"smart_home":     "Smart Home & IoT",
	"infrastructure": "Infrastructure & DevOps",
	"data_apis":      "Data APIs & Intelligence",
	"communication":  "Communication & Messaging",
}

// toolToCategoryIndex is a reverse lookup: tool name → category name.
// Built once on first access.
var toolToCategoryOnce sync.Once
var toolToCategoryMap map[string]string

func buildToolToCategoryMap() {
	toolToCategoryOnce.Do(func() {
		m := make(map[string]string, 128)
		for cat, entries := range toolCategoryDef {
			for _, e := range entries {
				m[e.Name] = cat
			}
		}
		toolToCategoryMap = m
	})
}

func ToolCategoryForName(name string) string {
	buildToolToCategoryMap()
	return toolToCategoryMap[name]
}

// GetToolCategory returns the category a tool belongs to, or "" if not categorized.
func GetToolCategory(toolName string) string {
	buildToolToCategoryMap()
	return toolToCategoryMap[toolName]
}

// SearchToolsInCategories searches all categorized tools by keyword (case-insensitive).
// Matches against tool name and short description.
func SearchToolsInCategories(query string) []struct {
	Category string
	Entry    ToolCategoryEntry
} {
	resolved := resolveDiscoverToolName(query)
	q := strings.ToLower(strings.TrimSpace(resolved))
	var results []struct {
		Category string
		Entry    ToolCategoryEntry
	}
	for _, cat := range toolCategoryOrder {
		for _, entry := range toolCategoryDef[cat] {
			if strings.Contains(strings.ToLower(entry.Name), q) || strings.Contains(strings.ToLower(entry.ShortDesc), q) {
				results = append(results, struct {
					Category string
					Entry    ToolCategoryEntry
				}{Category: cat, Entry: entry})
			}
		}
	}
	return results
}

// FormatToolCategories formats a category listing showing which tools are active vs hidden.
// activeTools is the set of tool names currently in the LLM tool schema.
func FormatToolCategories(category string, activeTools map[string]bool, enabledTools map[string]bool) string {
	var sb strings.Builder

	cats := toolCategoryOrder
	if category != "" {
		if _, ok := toolCategoryDef[category]; !ok {
			return fmt.Sprintf("Unknown category '%s'. Available categories: %s", category, strings.Join(toolCategoryOrder, ", "))
		}
		cats = []string{category}
	}

	for i, cat := range cats {
		if i > 0 {
			sb.WriteString("\n")
		}
		label := toolCategoryLabels[cat]
		sb.WriteString(fmt.Sprintf("── %s (%s) ──\n", label, cat))
		entries := toolCategoryDef[cat]
		for _, entry := range entries {
			enabled := enabledTools[entry.Name]
			active := activeTools[entry.Name]
			if !enabled {
				continue // skip tools that are disabled in config
			}
			status := "○"
			if active {
				status = "●"
			}
			sb.WriteString(fmt.Sprintf("  %s %s — %s\n", status, entry.Name, entry.ShortDesc))
		}
	}

	sb.WriteString("\n● = active in context   ○ = available (use get_tool_info to see parameters)")
	return sb.String()
}

// FormatToolInfo formats a tool's full schema and guide for the agent.
func FormatToolInfo(toolName string, schemas []openai.Tool, guide string) string {
	var sb strings.Builder

	// Find matching schema
	var schema *openai.FunctionDefinition
	for _, s := range schemas {
		if s.Function != nil && s.Function.Name == toolName {
			schema = s.Function
			break
		}
	}

	if schema == nil {
		return fmt.Sprintf("Tool '%s' not found. It may be disabled in config. Use 'list_categories' to see available tools.", toolName)
	}

	sb.WriteString(fmt.Sprintf("# %s\n", schema.Name))
	sb.WriteString(fmt.Sprintf("Description: %s\n\n", schema.Description))

	// Format parameters from schema
	if params, ok := schema.Parameters.(map[string]interface{}); ok {
		if props, ok := params["properties"].(map[string]interface{}); ok {
			sb.WriteString("Parameters:\n")
			// Show required params first
			var required []string
			if req, ok := params["required"].([]string); ok {
				required = req
			} else if reqI, ok := params["required"].([]interface{}); ok {
				for _, r := range reqI {
					if s, ok := r.(string); ok {
						required = append(required, s)
					}
				}
			}
			reqSet := make(map[string]bool, len(required))
			for _, r := range required {
				reqSet[r] = true
			}

			for name, val := range props {
				pMap, ok := val.(map[string]interface{})
				if !ok {
					continue
				}
				typ, _ := pMap["type"].(string)
				desc, _ := pMap["description"].(string)
				reqMark := ""
				if reqSet[name] {
					reqMark = " (required)"
				}
				sb.WriteString(fmt.Sprintf("  - %s: %s — %s%s\n", name, typ, desc, reqMark))

				// Show enum values if present
				if enumVals, ok := pMap["enum"].([]string); ok {
					sb.WriteString(fmt.Sprintf("    values: %s\n", strings.Join(enumVals, ", ")))
				} else if enumI, ok := pMap["enum"].([]interface{}); ok {
					vals := make([]string, 0, len(enumI))
					for _, v := range enumI {
						vals = append(vals, fmt.Sprintf("%v", v))
					}
					sb.WriteString(fmt.Sprintf("    values: %s\n", strings.Join(vals, ", ")))
				}
			}
		}
	}

	if guide != "" {
		sb.WriteString("\n--- Tool Guide ---\n")
		sb.WriteString(guide)
	}

	return sb.String()
}
