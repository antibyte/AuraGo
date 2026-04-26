package agent

import openai "github.com/sashabaranov/go-openai"

func appendEdgeToolSchemas(tools []openai.Tool, ff ToolFeatureFlags) []openai.Tool {
	// ── Telnyx SMS & Voice ──────────────────────────────────────────
	if ff.TelnyxSMSEnabled {
		tools = append(tools, tool("telnyx_sms",
			"Send and manage SMS/MMS messages via Telnyx. Can send text messages and multimedia messages to phone numbers.",
			schema(map[string]interface{}{
				"operation": map[string]interface{}{
					"type":        "string",
					"description": "Operation to perform",
					"enum":        []string{"send", "send_mms", "status"},
				},
				"to":      prop("string", "Recipient phone number in E.164 format (e.g. +491511234567). Required for send/send_mms."),
				"message": prop("string", "Text message content. Required for send/send_mms. Max 1600 chars."),
				"media_urls": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "URLs of media files to attach (for send_mms only). Max 10 items.",
				},
				"message_id": prop("string", "Message ID to check status (for status operation)."),
			}, "operation"),
		))
	}
	if ff.TelnyxCallEnabled {
		tools = append(tools, tool("telnyx_call",
			"Initiate and control voice calls via Telnyx. Can make calls, speak text (TTS), gather DTMF input, transfer, and record.",
			schema(map[string]interface{}{
				"operation": map[string]interface{}{
					"type":        "string",
					"description": "Call operation to perform",
					"enum":        []string{"initiate", "speak", "play_audio", "gather_dtmf", "transfer", "record_start", "record_stop", "hangup", "list_active"},
				},
				"to":              prop("string", "Phone number to call in E.164 format. Required for initiate/transfer."),
				"call_control_id": prop("string", "Call control ID of active call. Required for speak/play_audio/gather_dtmf/transfer/record_*/hangup."),
				"text":            prop("string", "Text to speak via TTS during the call. Required for speak/gather_dtmf."),
				"audio_url":       prop("string", "URL of audio file to play. Required for play_audio."),
				"max_digits":      map[string]interface{}{"type": "integer", "description": "Maximum DTMF digits to collect (for gather_dtmf). Default: 1."},
				"timeout_secs":    map[string]interface{}{"type": "integer", "description": "Timeout in seconds for DTMF gathering. Default: 10."},
			}, "operation"),
		))
	}
	if ff.TelnyxSMSEnabled || ff.TelnyxCallEnabled {
		tools = append(tools, tool("telnyx_manage",
			"Manage Telnyx phone resources: list phone numbers, check balance, view call/message history.",
			schema(map[string]interface{}{
				"operation": map[string]interface{}{
					"type":        "string",
					"description": "Management operation",
					"enum":        []string{"list_numbers", "check_balance", "message_history", "call_history"},
				},
				"limit": map[string]interface{}{"type": "integer", "description": "Max results to return. Default: 20."},
				"page":  map[string]interface{}{"type": "integer", "description": "Page number for pagination. Default: 1."},
			}, "operation"),
		))
	}

	if ff.UPnPScanEnabled {
		tools = append(tools, tool("upnp_scan",
			"Discover UPnP/SSDP devices on the local network (routers, Smart TVs, NAS, media renderers, printers, IoT devices). "+
				"Returns device name, manufacturer, model, type, and exposed services. "+
				"Use search_target 'ssdp:all' (default) to find everything, or filter by device type "+
				"(e.g. 'upnp:rootdevice', 'urn:schemas-upnp-org:device:MediaRenderer:1'). "+
				"Set auto_register=true to bulk-import all discovered devices into the device registry in a single call.",
			schema(map[string]interface{}{
				"search_target":      prop("string", "UPnP search target (default: 'ssdp:all'). Other values: 'upnp:rootdevice', 'urn:schemas-upnp-org:device:MediaRenderer:1', etc."),
				"timeout_secs":       map[string]interface{}{"type": "integer", "description": "Discovery timeout in seconds (1–30, default: 5)"},
				"auto_register":      map[string]interface{}{"type": "boolean", "description": "If true, automatically register all discovered devices into the device inventory in one call. Saves many token-costly individual manage_inventory calls."},
				"register_type":      prop("string", "Device type to assign when auto_register is true (e.g. 'router', 'media-server', 'iot'). Defaults to the UPnP device_type field."),
				"register_tags":      map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": "Tags to assign to auto-registered devices."},
				"overwrite_existing": map[string]interface{}{"type": "boolean", "description": "If true, update an existing device record when the name matches. Default: false (skip duplicates)."},
			}),
		))
	}

	if ff.SQLConnectionsEnabled {
		tools = append(tools, tool("sql_query",
			"Execute a SQL query against a registered database connection. Supports SELECT, INSERT, UPDATE, DELETE, and DDL statements. "+
				"Permissions are enforced per connection (read/write/change/delete). "+
				"When global SQL read-only mode is enabled (sql_connections.readonly), all mutating queries are blocked regardless of connection permissions. "+
				"Use operation 'query' to run SQL, 'describe' to get table structure, 'list_tables' to list all tables.",
			schema(map[string]interface{}{
				"operation": map[string]interface{}{
					"type":        "string",
					"description": "Operation to perform",
					"enum":        []string{"query", "describe", "list_tables"},
				},
				"connection_name": prop("string", "Name of the database connection to use"),
				"sql_query":       prop("string", "SQL statement to execute (for 'query' operation)"),
				"table_name":      prop("string", "Table name (for 'describe' operation)"),
			}, "operation", "connection_name"),
		))

		tools = append(tools, tool("manage_sql_connections",
			"Manage external database connections. By default, the agent can only list, get, and test connections. "+
				"Creating, updating, and deleting connections requires explicit administrator enablement via sql_connections.allow_management. "+
				"Supports PostgreSQL, MySQL/MariaDB, and SQLite. Credentials are stored securely in the vault. "+
				"Use 'docker_create' to spin up a new database container via Docker.",
			schema(map[string]interface{}{
				"operation": map[string]interface{}{
					"type":        "string",
					"description": "Operation to perform",
					"enum":        []string{"list", "get", "create", "update", "delete", "test", "docker_create"},
				},
				"connection_name": prop("string", "Connection name (unique identifier)"),
				"driver": map[string]interface{}{
					"type":        "string",
					"description": "Database driver",
					"enum":        []string{"postgres", "mysql", "sqlite"},
				},
				"host":              prop("string", "Database host (IP or hostname)"),
				"port":              map[string]interface{}{"type": "integer", "description": "Database port (default: 5432 for postgres, 3306 for mysql)"},
				"database_name":     prop("string", "Database name or SQLite file path"),
				"description":       prop("string", "Short description of the database purpose"),
				"username":          prop("string", "Database username (stored in vault)"),
				"password":          prop("string", "Database password (stored in vault)"),
				"ssl_mode":          prop("string", "SSL mode: disable, require, verify-ca, verify-full (default: disable)"),
				"credential_action": map[string]interface{}{"type": "string", "description": "Credential handling for update: keep, replace, or delete", "enum": []string{"keep", "replace", "delete"}},
				"allow_read":        map[string]interface{}{"type": "boolean", "description": "Allow SELECT queries (default: true)"},
				"allow_write":       map[string]interface{}{"type": "boolean", "description": "Allow INSERT queries (default: false)"},
				"allow_change":      map[string]interface{}{"type": "boolean", "description": "Allow UPDATE queries (default: false)"},
				"allow_delete":      map[string]interface{}{"type": "boolean", "description": "Allow DELETE queries (default: false)"},
				"docker_template": map[string]interface{}{
					"type":        "string",
					"description": "Docker template for docker_create: postgres, mysql, mariadb",
					"enum":        []string{"postgres", "mysql", "mariadb"},
				},
			}, "operation"),
		))
	}

	// ── YepAPI SEO ──────────────────────────────────────────────────
	if ff.YepAPISEOEnabled {
		tools = append(tools, tool("yepapi_seo",
			"SEO analysis via YepAPI: keyword research, domain overview, competitor analysis, backlink summary, on-page audits, and Google Trends data. "+
				"All operations are read-only and pay-per-call.",
			schema(map[string]interface{}{
				"operation": map[string]interface{}{
					"type":        "string",
					"description": "SEO operation to perform",
					"enum":        []string{"keywords", "keyword_ideas", "domain_overview", "domain_keywords", "competitors", "backlinks", "onpage", "trends"},
				},
				"keywords": prop("string", "JSON array of keywords (for 'keywords' operation)"),
				"seed":     prop("string", "Seed keyword for suggestions (for 'keyword_ideas' operation)"),
				"domain":   prop("string", "Domain name, e.g. 'example.com' (for domain_* operations)"),
				"target":   prop("string", "Target domain or URL (for 'backlinks' operation)"),
				"url":      prop("string", "Page URL to audit (for 'onpage' operation)"),
			}, "operation"),
		))
	}

	// ── YepAPI SERP ─────────────────────────────────────────────────
	if ff.YepAPISERPEnabled {
		tools = append(tools, tool("yepapi_serp",
			"Search engine results via YepAPI: Google, Bing, Yahoo, Baidu, YouTube SERP, Google Images, News, Maps, and more. "+
				"Returns real-time SERP data with titles, URLs, descriptions, and positions.",
			schema(map[string]interface{}{
				"operation": map[string]interface{}{
					"type":        "string",
					"description": "SERP engine to query",
					"enum":        []string{"google", "google_images", "google_news", "google_maps", "google_datasets", "google_autocomplete", "google_ads", "google_ai_mode", "google_finance", "yahoo", "bing", "baidu", "youtube"},
				},
				"query":    prop("string", "Search query (required)"),
				"depth":    map[string]interface{}{"type": "integer", "description": "Number of results to return (default: 10)"},
				"location": prop("string", "Country code for localised results, e.g. 'us', 'de', 'uk' (default: 'us')"),
				"language": prop("string", "Language code, e.g. 'en', 'de' (default: 'en')"),
				"limit":    map[string]interface{}{"type": "integer", "description": "Max results for Google Maps (default: 10)"},
				"open_now": map[string]interface{}{"type": "boolean", "description": "Filter Google Maps for currently open places"},
			}, "operation", "query"),
		))
	}

	// ── YepAPI Scraping ─────────────────────────────────────────────
	if ff.YepAPIScrapingEnabled {
		tools = append(tools, tool("yepapi_scrape",
			"Web scraping via YepAPI: standard scrape, JavaScript-rendered pages, stealth anti-bot bypass, full-page screenshots, and AI-powered data extraction. "+
				"Returns page content as markdown, HTML, or structured data.",
			schema(map[string]interface{}{
				"operation": map[string]interface{}{
					"type":        "string",
					"description": "Scraping operation to perform",
					"enum":        []string{"scrape", "js", "stealth", "screenshot", "ai_extract"},
				},
				"url":    prop("string", "URL to scrape (required)"),
				"format": map[string]interface{}{"type": "string", "description": "Output format for 'scrape' operation: 'markdown' or 'html' (default: markdown)"},
				"prompt": prop("string", "Natural language extraction prompt (for 'ai_extract' operation)"),
			}, "operation", "url"),
		))
	}
	return tools
}
