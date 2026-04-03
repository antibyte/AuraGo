package agent

import openai "github.com/sashabaranov/go-openai"

func appendIntegrationToolSchemas(tools []openai.Tool, ff ToolFeatureFlags) []openai.Tool {
	// ── Integration tools (conditionally included) ───────────────────────────

	if ff.HomeAssistantEnabled {
		tools = append(tools, tool("home_assistant",
			"Control Home Assistant smart home devices. Get entity states, call services (turn on/off lights, switches, scenes, etc.), and list available services.",
			schema(map[string]interface{}{
				"operation": map[string]interface{}{
					"type":        "string",
					"description": "Operation to perform",
					"enum":        []string{"get_states", "get_state", "call_service", "list_services"},
				},
				"entity_id": prop("string", "Entity ID (e.g. 'light.living_room', 'switch.heater')"),
				"domain":    prop("string", "HA domain for filtering or service calls (e.g. 'light', 'switch', 'climate', 'scene')"),
				"service":   prop("string", "Service to call (e.g. 'turn_on', 'turn_off', 'toggle')"),
				"service_data": map[string]interface{}{
					"type":        "object",
					"description": "Additional parameters for the service call (e.g. brightness, temperature, color)",
				},
			}, "operation"),
		))
	}

	if ff.MeshCentralEnabled {
		tools = append(tools, tool("meshcentral",
			"Manage and interact with devices and groups managed by a MeshCentral server. Supports listing devices, wake-on-lan, power actions, running commands, and interactive shell access.",
			schema(map[string]interface{}{
				"operation": map[string]interface{}{
					"type":        "string",
					"description": "Operation to perform",
					"enum":        []string{"list_groups", "list_devices", "wake", "power_action", "run_command", "shell"},
				},
				"mesh_id":      prop("string", "Mesh/Group ID (for list_devices)"),
				"node_id":      prop("string", "Node/Device ID (for wake, power_action, run_command, shell)"),
				"power_action": prop("integer", "Action (1=Sleep, 2=Hibernate, 3=PowerOff, 4=Reset)"),
				"command":      prop("string", "Command string (for run_command or shell)"),
			}, "operation"),
		))
	}

	if ff.WOLEnabled {
		tools = append(tools, tool("wake_on_lan",
			"Send a Wake-on-LAN magic packet to wake up a device. Use the device's registered inventory ID or provide a MAC address directly. Only works on devices that support WOL and are on the local network.",
			schema(map[string]interface{}{
				"server_id":   prop("string", "Device ID from inventory (the registered MAC address will be used automatically)"),
				"mac_address": prop("string", "MAC address to wake up (e.g. 'AA:BB:CC:DD:EE:FF'). Required if server_id is not provided or the device has no MAC registered."),
				"ip_address":  prop("string", "Optional broadcast IP address (e.g. '192.168.1.255'). Defaults to 255.255.255.255."),
			}),
		))
	}

	if ff.ChromecastEnabled {
		tools = append(tools, tool("chromecast",
			"Control Chromecast and Google Cast devices on the local network. "+
				"Discover devices, play media URLs, speak text via TTS, stop playback, adjust volume, and query status. "+
				"Specify a device by name (resolved via inventory) or directly by IP address and port.",
			schema(map[string]interface{}{
				"operation": map[string]interface{}{
					"type":        "string",
					"description": "Chromecast operation to perform",
					"enum":        []string{"discover", "play", "speak", "stop", "volume", "status"},
				},
				"device_name":  prop("string", "Friendly device name (resolved via device registry, e.g. 'Living Room'). Use when device_addr is unknown."),
				"device_addr":  prop("string", "IP address of the Chromecast device (e.g. '192.168.1.42')."),
				"device_port":  map[string]interface{}{"type": "integer", "description": "Port of the Chromecast device (default: 8009)."},
				"url":          prop("string", "Media URL to cast (for 'play' operation)."),
				"content_type": prop("string", "MIME type of the media (for 'play', e.g. 'video/mp4', 'audio/mpeg'). Default: 'video/mp4'."),
				"text":         prop("string", "Text to speak aloud via TTS (for 'speak' operation)."),
				"language":     prop("string", "Language code for TTS speech (for 'speak', e.g. 'de', 'en'). Defaults to system language."),
				"volume":       map[string]interface{}{"type": "number", "description": "Volume level 0.0–1.0 (for 'volume' operation)."},
			}, "operation"),
		))
	}

	// TTS — synthesize speech audio. Always shown (runtime checks if TTS is actually configured).
	tools = append(tools, tool("tts",
		"Convert text to speech (TTS). The generated audio will AUTOMATICALLY be sent to the user and played in the chat UI! "+
			"Supports ElevenLabs, MiniMax, and Piper TTS providers. "+
			"When VOICE MODE is active, YOU MUST USE THIS TOOL to reply to the user instead of typing a long text response. "+
			"Put your conversational output in the 'text' argument.",
		schema(map[string]interface{}{
			"text":     prop("string", "Text to synthesize into speech. Can be a sentence, paragraph, or any text content."),
			"language": prop("string", "Language code for the speech (e.g. 'en', 'de', 'es', 'fr'). Defaults to the configured TTS language."),
		}, "text"),
	))

	if ff.DockerEnabled {
		tools = append(tools, tool("docker",
			"Manage Docker containers, images, networks, and volumes. List, inspect, start, stop, create, remove containers; pull/remove images; view logs; get system info.",
			schema(map[string]interface{}{
				"operation": map[string]interface{}{
					"type":        "string",
					"description": "Operation to perform",
					"enum":        []string{"list_containers", "inspect", "start", "stop", "restart", "pause", "unpause", "remove", "logs", "create", "run", "list_images", "pull", "remove_image", "list_networks", "list_volumes", "info"},
				},
				"container_id": prop("string", "Container ID or name (for container operations)"),
				"image":        prop("string", "Docker image name with optional tag (e.g. 'nginx:latest')"),
				"name":         prop("string", "Container name (for create/run)"),
				"command":      prop("string", "Command to run in the container"),
				"env":          map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": "Environment variables (e.g. ['KEY=value'])"},
				"ports":        map[string]interface{}{"type": "object", "description": "Port mappings: {'container_port': 'host_port'} (e.g. {'80': '8080'})"},
				"volumes":      map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": "Volume binds (e.g. ['/host/path:/container/path'])"},
				"restart":      prop("string", "Restart policy: no, always, unless-stopped, on-failure"),
				"force":        prop("boolean", "Force removal (for remove/remove_image)"),
				"tail":         prop("integer", "Number of log lines to return (default: 100)"),
				"all":          prop("boolean", "Include stopped containers (for list_containers)"),
			}, "operation"),
		))
	}

	if ff.CoAgentEnabled {
		tools = append(tools, tool("co_agent",
			"Spawn and manage parallel co-agents that work on sub-tasks independently. Co-agents run in background goroutines with their own LLM context and return results when done. Use 'spawn_specialist' to dispatch tasks to specialized experts (researcher, coder, designer, security, writer). When slots are full, co-agents may be queued automatically and started by priority.",
			schema(map[string]interface{}{
				"operation": map[string]interface{}{
					"type":        "string",
					"description": "Operation to perform",
					"enum":        []string{"spawn", "spawn_specialist", "list", "get_result", "stop", "stop_all"},
				},
				"task":          prop("string", "Task description for the co-agent to work on (required for 'spawn' and 'spawn_specialist')"),
				"specialist":    prop("string", "Specialist role (required for 'spawn_specialist'). One of: researcher, coder, designer, security, writer"),
				"co_agent_id":   prop("string", "Co-agent ID (required for 'get_result' and 'stop')"),
				"context_hints": map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": "Optional keywords or topics for RAG context injection (for 'spawn' and 'spawn_specialist'). Keep them short and specific."},
				"priority":      prop("integer", "Optional queue priority: 1=low, 2=normal, 3=high. Higher priority queued co-agents start first."),
			}, "operation"),
		))
	}

	if ff.HomepageEnabled {
		homepageDesc := "Design, develop, build, test and deploy websites using a Docker-based dev environment with Node.js, Playwright, Lighthouse and more. Supports Next.js, Vite, Astro, Svelte, Vue and static HTML."
		if !ff.HomepageAllowLocalServer {
			homepageDesc += " REQUIRES DOCKER: Local Python server fallback is disabled for security. Ensure Docker is running or enable homepage.allow_local_server in config."
		}
		tools = append(tools, tool("homepage",
			homepageDesc,
			schema(map[string]interface{}{
				"operation": map[string]interface{}{
					"type":        "string",
					"description": "Operation to perform. To deploy a workspace project to Netlify, use 'deploy_netlify' — it builds and packages automatically, no manual ZIP needed. Do NOT use the 'netlify' tool's deploy_zip/deploy_draft for workspace projects.",
					"enum":        []string{"init", "start", "stop", "status", "rebuild", "destroy", "exec", "init_project", "build", "install_deps", "lighthouse", "screenshot", "lint", "list_files", "read_file", "write_file", "edit_file", "json_edit", "yaml_edit", "xml_edit", "optimize_images", "dev", "deploy", "deploy_netlify", "test_connection", "webserver_start", "webserver_stop", "webserver_status", "publish_local", "tunnel", "git_init", "git_commit", "git_status", "git_diff", "git_log", "git_rollback"},
				},
				"command":       prop("string", "Shell command to execute (for 'exec')"),
				"framework":     prop("string", "Web framework: next, vite, astro, svelte, vue, html (for 'init_project')"),
				"name":          prop("string", "Project name (for 'init_project')"),
				"project_dir":   prop("string", "Project subdirectory within /workspace (default: '.')"),
				"build_dir":     prop("string", "Build output directory (auto-detected if empty)"),
				"template":      prop("string", "Project template for init_project: portfolio, blog, landing, dashboard (optional — applies starter content after scaffolding)"),
				"auto_fix":      map[string]interface{}{"type": "boolean", "description": "If true, attempt to auto-fix common build errors (missing deps, lint issues) and retry once (for 'build')"},
				"git_message":   prop("string", "Commit message (for 'git_commit')"),
				"count":         prop("integer", "Number of entries (for 'git_log': default 10) or commits to revert (for 'git_rollback': default 1)"),
				"path":          prop("string", "File path relative to /workspace — MUST include the project subdirectory prefix (e.g. 'my-project/index.html', NOT just 'index.html'). Applies to 'read_file', 'write_file', 'list_files', 'edit_file', 'json_edit', 'yaml_edit', 'xml_edit'."),
				"content":       prop("string", "File content to write (for 'write_file') or text to insert (for 'edit_file' insert_after/insert_before/append/prepend)"),
				"url":           prop("string", "URL for lighthouse audit or screenshot"),
				"viewport":      prop("string", "Viewport size for screenshot (e.g. '1280x720')"),
				"packages":      map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": "NPM packages to install (for 'install_deps')"},
				"sub_operation": prop("string", "Edit sub-operation for edit_file: str_replace, str_replace_all, insert_after, insert_before, append, prepend, delete_lines. For json_edit/yaml_edit: get, set, delete, keys, validate, format (json only). For xml_edit: get, set_text, set_attribute, add_element, delete, validate, format"),
				"old":           prop("string", "Text to find (for edit_file str_replace/str_replace_all)"),
				"new":           prop("string", "Replacement text (for edit_file str_replace/str_replace_all)"),
				"marker":        prop("string", "Anchor text (for edit_file insert_after/insert_before)"),
				"start_line":    prop("integer", "First line to delete (for edit_file delete_lines)"),
				"end_line":      prop("integer", "Last line to delete (for edit_file delete_lines)"),
				"json_path":     prop("string", "Dot-separated path for json_edit/yaml_edit (e.g. 'server.port', 'theme.colors.primary')"),
				"xpath":         prop("string", "XPath expression for xml_edit (e.g. '//server', './config/database')"),
				"set_value":     map[string]interface{}{"description": "Value to set for json_edit/yaml_edit/xml_edit operations (any JSON type)"},
				// deploy_netlify specific fields
				"site_id": prop("string", "Netlify site ID to deploy to (for 'deploy_netlify'). Leave empty to use the default site from config."),
				"draft":   map[string]interface{}{"type": "boolean", "description": "Deploy as preview/draft, not as production (for 'deploy_netlify')"},
				"title":   prop("string", "Deploy message shown in Netlify dashboard (for 'deploy_netlify')"),
			}, "operation"),
		))
	}

	if ff.WebhooksEnabled {
		tools = append(tools,
			tool("call_webhook",
				"Trigger an outgoing Webhook. The required 'parameters' depend on the webhook definition.",
				schema(map[string]interface{}{
					"webhook_name": prop("string", "Name of the webhook to execute"),
					"parameters": map[string]interface{}{
						"type":                 "object",
						"description":          "Parameters payload for the webhook.",
						"additionalProperties": true,
					},
				}, "webhook_name", "parameters"),
			),
			tool("manage_outgoing_webhooks",
				"Manage configured outgoing webhooks (list, create, update, delete). 'list' requires no other args.",
				schema(map[string]interface{}{
					"operation": map[string]interface{}{
						"type":        "string",
						"description": "Operation to perform",
						"enum":        []string{"list", "create", "update", "delete"},
					},
					"id":            prop("string", "Webhook ID (required for update/delete)"),
					"name":          prop("string", "Friendly name of the webhook (required for create)"),
					"description":   prop("string", "Description of what it does and parameters needed (required for create)"),
					"method":        map[string]interface{}{"type": "string", "enum": []string{"GET", "POST", "PUT", "DELETE"}},
					"url":           prop("string", "URL endpoint. Can contain {{variables}}"),
					"payload_type":  map[string]interface{}{"type": "string", "enum": []string{"json", "form", "custom"}},
					"body_template": prop("string", "Custom request body template. Applies only if payload_type is custom."),
					"headers":       map[string]interface{}{"type": "object", "additionalProperties": map[string]interface{}{"type": "string"}},
					"parameters": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"name":        map[string]interface{}{"type": "string"},
								"type":        map[string]interface{}{"type": "string", "enum": []string{"string", "number", "boolean"}},
								"description": map[string]interface{}{"type": "string"},
								"required":    map[string]interface{}{"type": "boolean"},
							},
						},
					},
				}, "operation"),
			),
		)
	}

	if ff.NetlifyEnabled {
		tools = append(tools, tool("netlify",
			"Manage Netlify sites, deploys, environment variables, forms, hooks, and SSL certificates via the Netlify API.",
			schema(map[string]interface{}{
				"operation": map[string]interface{}{
					"type":        "string",
					"description": "Operation to perform",
					"enum":        []string{"list_sites", "get_site", "create_site", "update_site", "delete_site", "list_deploys", "get_deploy", "rollback", "cancel_deploy", "list_env", "get_env", "set_env", "delete_env", "list_files", "list_forms", "get_submissions", "list_hooks", "create_hook", "delete_hook", "provision_ssl", "check_connection"},
				},
				"site_id":       prop("string", "Netlify site ID (uses default_site_id if omitted)"),
				"site_name":     prop("string", "Site subdomain name for create (name.netlify.app)"),
				"custom_domain": prop("string", "Custom domain for the site"),
				"deploy_id":     prop("string", "Deploy ID (for get_deploy, rollback, cancel_deploy)"),
				"env_key":       prop("string", "Environment variable key"),
				"env_value":     prop("string", "Environment variable value"),
				"env_context":   prop("string", "Env var context: all, production, deploy-preview, branch-deploy, dev"),
				"form_id":       prop("string", "Form ID (for get_submissions)"),
				"hook_id":       prop("string", "Hook ID (for delete_hook)"),
				"hook_type":     prop("string", "Hook type: url, email, slack"),
				"hook_event":    prop("string", "Hook event: deploy_created, deploy_building, deploy_failed, etc."),
				"url":           prop("string", "Webhook URL (for create_hook with type=url)"),
				"value":         prop("string", "Email address (for create_hook with type=email)"),
			}, "operation"),
		))
	}

	if ff.AllowSelfUpdate {
		tools = append(tools, tool("manage_updates",
			"Check for AuraGo updates on GitHub or install them. Use 'check' to see if a new version is available without installing. Use 'install' only after user approval.",
			schema(map[string]interface{}{
				"operation": map[string]interface{}{
					"type":        "string",
					"description": "Operation: 'check' (dry run) or 'install' (applies updates)",
					"enum":        []string{"check", "install"},
				},
			}, "operation"),
		))
	}

	if ff.SudoEnabled {
		tools = append(tools, tool("execute_sudo",
			"Run a shell command with sudo (root) privileges. Only available when explicitly enabled in config. Use ONLY when elevated privileges are strictly required — prefer execute_shell for normal tasks.",
			schema(map[string]interface{}{
				"command": prop("string", "The shell command to run as root via sudo"),
			}, "command"),
		))
	}

	if ff.WebhooksEnabled {
		tools = append(tools, tool("manage_webhooks",
			"Manage incoming webhook endpoints. Create, list, update, delete webhooks and view their logs.",
			schema(map[string]interface{}{
				"action": map[string]interface{}{
					"type":        "string",
					"description": "Operation to perform",
					"enum":        []string{"list", "get", "create", "update", "delete", "logs"},
				},
				"id":       prop("string", "Webhook ID (for get/update/delete/logs)"),
				"name":     prop("string", "Webhook name (for create/update)"),
				"slug":     prop("string", "URL slug (for create, e.g. 'github-push')"),
				"enabled":  prop("boolean", "Enable/disable webhook (for create/update)"),
				"token_id": prop("string", "Token ID to associate (for create/update)"),
			}, "action"),
		))
	}

	if ff.JellyfinEnabled {
		tools = append(tools, tool("jellyfin",
			"Manage Jellyfin media server: check server health, browse libraries, search media, view item details, list recent additions, monitor active sessions, control playback, refresh libraries, delete items, and view activity logs.",
			schema(map[string]interface{}{
				"operation": map[string]interface{}{
					"type":        "string",
					"description": "Operation to perform",
					"enum":        []string{"health", "library_list", "search", "item_details", "recent_items", "sessions", "playback_control", "library_refresh", "delete_item", "activity_log"},
				},
				"query":      prop("string", "Search query (for search)"),
				"media_type": prop("string", "Filter by media type: movie, series, episode, music, album, artist (for search, recent_items)"),
				"item_id":    prop("string", "Media item ID (for item_details, delete_item)"),
				"library_id": prop("string", "Library ID (for library_refresh)"),
				"session_id": prop("string", "Session ID (for playback_control)"),
				"command":    prop("string", "Playback command: play, pause, stop, next, previous (for playback_control)"),
				"limit":      map[string]interface{}{"type": "integer", "description": "Max results to return (default: 20)"},
			}, "operation"),
		))
	}

	if ff.TrueNASEnabled {
		tools = append(tools, tool("truenas",
			"Manage TrueNAS storage system: check health, list/scrub storage pools, manage ZFS datasets and snapshots, "+
				"manage SMB shares, and check filesystem space. Use 'action' to specify the operation.",
			schema(map[string]interface{}{
				"action": map[string]interface{}{
					"type":        "string",
					"description": "TrueNAS operation to perform",
					"enum": []string{
						"truenas_health",
						"truenas_pool_list", "truenas_pool_scrub",
						"truenas_dataset_list", "truenas_dataset_create", "truenas_dataset_delete",
						"truenas_snapshot_list", "truenas_snapshot_create", "truenas_snapshot_delete", "truenas_snapshot_rollback",
						"truenas_smb_list", "truenas_smb_create", "truenas_smb_delete",
						"truenas_fs_space",
					},
				},
				"name":      prop("string", "Dataset, snapshot, or SMB share name. Required for create/delete/rollback operations."),
				"path":      prop("string", "SMB share local filesystem path (for truenas_smb_create, e.g. '/mnt/pool/share')."),
				"query":     prop("string", "Pool name or dataset path for filtering (e.g. 'tank' for pool, 'tank/data' for dataset)."),
				"port":      map[string]interface{}{"type": "integer", "description": "Numeric pool ID for truenas_pool_scrub, or SMB share ID for truenas_smb_delete."},
				"limit":     map[string]interface{}{"type": "integer", "description": "Quota in GB for truenas_dataset_create, or snapshot retention days for truenas_snapshot_create."},
				"content":   prop("string", "Compression type for truenas_dataset_create: lz4 (default), zstd, gzip, off."),
				"recursive": map[string]interface{}{"type": "boolean", "description": "Enable recursive operation (for truenas_dataset_delete or truenas_snapshot_create)."},
				"force":     map[string]interface{}{"type": "boolean", "description": "Force rollback (for truenas_snapshot_rollback)."},
			}, "action"),
		))
	}

	if ff.ProxmoxEnabled {
		tools = append(tools, tool("proxmox",
			"Manage Proxmox VE virtual machines and containers: list nodes/VMs/CTs, start/stop/reboot, snapshots, storage info, cluster resources.",
			schema(map[string]interface{}{
				"operation": map[string]interface{}{
					"type":        "string",
					"description": "Operation to perform",
					"enum":        []string{"overview", "list_nodes", "list_vms", "list_containers", "status", "start", "stop", "shutdown", "reboot", "suspend", "resume", "node_status", "cluster_resources", "storage", "create_snapshot", "list_snapshots", "task_log"},
				},
				"node":          prop("string", "Node name (optional, uses default from config)"),
				"vmid":          prop("string", "VM or container ID (e.g. '100')"),
				"vm_type":       prop("string", "Type: 'qemu' (VM) or 'lxc' (container). Default: qemu"),
				"name":          prop("string", "Snapshot name (for create_snapshot)"),
				"description":   prop("string", "Snapshot description"),
				"upid":          prop("string", "Task UPID (for task_log)"),
				"resource_type": prop("string", "Filter type for cluster_resources: vm, node, storage"),
			}, "operation"),
		))
	}

	if ff.OllamaEnabled {
		tools = append(tools, tool("ollama",
			"Manage local Ollama LLM instance: list models, pull/delete models, show model details, load/unload models from GPU memory.",
			schema(map[string]interface{}{
				"operation": map[string]interface{}{
					"type":        "string",
					"description": "Operation to perform",
					"enum":        []string{"list", "running", "show", "pull", "delete", "copy", "load", "unload"},
				},
				"model":       prop("string", "Model name (e.g. 'llama3:latest')"),
				"source":      prop("string", "Source model name (for copy)"),
				"destination": prop("string", "Destination model name (for copy)"),
			}, "operation"),
		))
	}

	if ff.TailscaleEnabled {
		tools = append(tools, tool("tailscale",
			"Manage and inspect the Tailscale VPN network: list devices, get device details, manage subnet routes, query DNS config, and get local node status.",
			schema(map[string]interface{}{
				"operation": map[string]interface{}{
					"type":        "string",
					"description": "Operation to perform",
					"enum":        []string{"devices", "device", "routes", "enable_routes", "disable_routes", "dns", "acl", "local_status"},
				},
				"query": prop("string", "Device hostname, MagicDNS name, IP address, or node ID (for device/routes/enable_routes/disable_routes)"),
				"value": prop("string", "Comma-separated CIDR routes to enable or disable (e.g. '10.0.0.0/8,192.168.1.0/24')"),
			}, "operation"),
		))
	}
	if ff.CloudflareTunnelEnabled {
		tools = append(tools, tool("cloudflare_tunnel",
			"Manage a Cloudflare Tunnel (cloudflared) to expose local services to the internet securely. Supports Docker and native binary modes, token/named/quick tunnel authentication.",
			schema(map[string]interface{}{
				"operation": map[string]interface{}{
					"type":        "string",
					"description": "Operation to perform",
					"enum":        []string{"start", "stop", "restart", "status", "quick_tunnel", "logs", "list_routes", "install"},
				},
				"port": map[string]interface{}{"type": "integer", "description": "Port to expose (for quick_tunnel; defaults to web UI port)"},
			}, "operation"),
		))
	}
	if ff.EmailEnabled {
		tools = append(tools,
			tool("fetch_email",
				"Fetch emails from an IMAP mailbox. Returns a list of messages with sender, subject, date, and body.",
				schema(map[string]interface{}{
					"folder":  prop("string", "Mailbox folder to read (default: INBOX)"),
					"limit":   map[string]interface{}{"type": "integer", "description": "Max number of messages to return (default: 10)"},
					"account": prop("string", "Email account ID (use list_email_accounts to see available accounts; omit for default)"),
				}),
			),
			tool("send_email",
				"Send an email via SMTP.",
				schema(map[string]interface{}{
					"to":      prop("string", "Recipient email address"),
					"subject": prop("string", "Email subject"),
					"body":    prop("string", "Email body (plain text)"),
					"account": prop("string", "Email account ID to send from (omit for default)"),
				}, "to"),
			),
			tool("list_email_accounts",
				"List all configured email accounts with their IMAP/SMTP settings and status.",
				schema(map[string]interface{}{}),
			),
		)
	}

	if ff.DiscordEnabled {
		tools = append(tools,
			tool("send_discord",
				"Send a message to a Discord channel.",
				schema(map[string]interface{}{
					"message":    prop("string", "Message text to send"),
					"channel_id": prop("string", "Discord channel ID (uses default_channel_id from config if omitted)"),
				}, "message"),
			),
			tool("fetch_discord",
				"Fetch recent messages from a Discord channel.",
				schema(map[string]interface{}{
					"channel_id": prop("string", "Discord channel ID (uses default from config if omitted)"),
					"limit":      map[string]interface{}{"type": "integer", "description": "Number of messages to fetch (default: 10)"},
				}),
			),
			tool("list_discord_channels",
				"List all text channels in the configured Discord server (guild).",
				schema(map[string]interface{}{}),
			),
		)
	}

	if ff.FirewallEnabled {
		tools = append(tools, tool("firewall",
			"Manage and inspect local Linux firewall rules (iptables/ufw). Note: modification commands are blocked in 'readonly' mode.",
			schema(map[string]interface{}{
				"operation": map[string]interface{}{
					"type":        "string",
					"description": "Operation to perform",
					"enum":        []string{"get_rules", "modify_rule"},
				},
				"command": prop("string", "The modifying command, e.g. 'iptables -A INPUT -p tcp --dport 80 -j ACCEPT' or 'ufw allow 80/tcp' (required for modify_rule)"),
			}, "operation"),
		))
	}
	if ff.AnsibleEnabled {
		tools = append(tools, tool("ansible",
			"Run Ansible automation: execute playbooks, ad-hoc modules, pings, and gather host facts via the Ansible sidecar.",
			schema(map[string]interface{}{
				"operation": map[string]interface{}{
					"type":        "string",
					"description": "Operation to perform",
					"enum":        []string{"status", "list_playbooks", "inventory", "ping", "adhoc", "playbook", "check", "facts"},
				},
				"name":       prop("string", "Playbook filename relative to sidecar's PLAYBOOKS_DIR (e.g. 'site.yml') — required for playbook/check"),
				"hostname":   prop("string", "Target host pattern for ping/adhoc/facts (e.g. 'all', 'webservers', '192.168.1.10')"),
				"host_limit": prop("string", "--limit: restrict playbook execution to a host subset"),
				"module":     prop("string", "Ansible module name for adhoc (e.g. 'ping', 'shell', 'copy', 'service', 'apt')"),
				"command":    prop("string", "Module arguments for adhoc (e.g. \"cmd='uptime'\" or \"name=nginx state=started\")"),
				"tags":       prop("string", "Comma-separated playbook tags to run (--tags)"),
				"skip_tags":  prop("string", "Comma-separated playbook tags to skip (--skip-tags)"),
				"inventory":  prop("string", "Inventory path override (uses sidecar default if omitted)"),
				"body":       prop("string", "Extra variables as JSON string (e.g. '{\"env\":\"prod\",\"replicas\":3}')"),
				"preview":    map[string]interface{}{"type": "boolean", "description": "When true, adds --check flag (dry-run, no changes applied)"},
			}, "operation"),
		))
	}
	if ff.InvasionControlEnabled {
		tools = append(tools, tool("invasion_control",
			"Manage deployment nests (target servers/VMs/containers) and eggs (sub-agent configurations). "+
				"List, inspect, assign, deploy (hatch), stop, monitor eggs, send tasks and secrets to running eggs.",
			schema(map[string]interface{}{
				"operation": map[string]interface{}{
					"type":        "string",
					"description": "Operation to perform",
					"enum":        []string{"list_nests", "list_eggs", "nest_status", "assign_egg", "hatch_egg", "stop_egg", "egg_status", "send_task", "send_secret"},
				},
				"nest_id":   prop("string", "Nest ID (for nest_status, assign_egg, hatch_egg, stop_egg, egg_status, send_task, send_secret)"),
				"nest_name": prop("string", "Nest name — alternative to nest_id for lookup"),
				"egg_id":    prop("string", "Egg ID (for assign_egg)"),
				"task":      prop("string", "Task description in natural language (for send_task)"),
				"key":       prop("string", "Secret key name (for send_secret)"),
				"value":     prop("string", "Secret value (for send_secret)"),
			}, "operation"),
		))
	}
	if ff.GitHubEnabled {
		tools = append(tools, tool("github",
			"Manage GitHub repositories, issues, pull requests, branches, files, commits, and workflow runs. Also track local projects.",
			schema(map[string]interface{}{
				"operation": map[string]interface{}{
					"type":        "string",
					"description": "Operation to perform",
					"enum":        []string{"list_repos", "create_repo", "delete_repo", "get_repo", "list_issues", "create_issue", "close_issue", "list_pull_requests", "list_branches", "get_file", "create_or_update_file", "list_commits", "list_workflow_runs", "search_repos", "list_projects", "track_project", "untrack_project"},
				},
				"name":        prop("string", "Repository or project name"),
				"owner":       prop("string", "GitHub owner/org (defaults to configured owner)"),
				"description": prop("string", "Description for repo or issue body"),
				"title":       prop("string", "Issue title"),
				"body":        prop("string", "Issue body or commit message"),
				"path":        prop("string", "File path within the repository"),
				"content":     prop("string", "File content (base64) or purpose description for track_project"),
				"query":       prop("string", "Search query or branch name"),
				"value":       prop("string", "SHA for file updates or state filter (open/closed/all)"),
				"id":          prop("string", "Issue number (as string)"),
				"limit":       map[string]interface{}{"type": "integer", "description": "Max results to return"},
				"label":       prop("string", "Comma-separated labels for issues"),
			}, "operation"),
		))
	}
	if ff.MQTTEnabled {
		tools = append(tools, tool("mqtt_publish",
			"Publish a message to an MQTT topic.",
			schema(map[string]interface{}{
				"topic":   prop("string", "MQTT topic to publish to (e.g. home/living_room/light)"),
				"payload": prop("string", "Message payload (string or JSON)"),
				"qos":     map[string]interface{}{"type": "integer", "description": "Quality of Service (0, 1, or 2). Default: 0"},
				"retain":  map[string]interface{}{"type": "boolean", "description": "Whether the broker should retain this message"},
			}, "topic", "payload"),
		))
		tools = append(tools, tool("mqtt_subscribe",
			"Subscribe to an MQTT topic to receive messages.",
			schema(map[string]interface{}{
				"topic": prop("string", "MQTT topic or wildcard pattern to subscribe to (e.g. home/#)"),
				"qos":   map[string]interface{}{"type": "integer", "description": "Quality of Service (0, 1, or 2). Default: 0"},
			}, "topic"),
		))
		tools = append(tools, tool("mqtt_unsubscribe",
			"Unsubscribe from an MQTT topic.",
			schema(map[string]interface{}{
				"topic": prop("string", "MQTT topic to unsubscribe from"),
			}, "topic"),
		))
		tools = append(tools, tool("mqtt_get_messages",
			"Retrieve recently received MQTT messages from the message buffer.",
			schema(map[string]interface{}{
				"topic": prop("string", "Filter by topic (empty or '#' for all topics)"),
				"limit": map[string]interface{}{"type": "integer", "description": "Maximum number of messages to return (default: 20)"},
			}),
		))
	}
	if ff.MCPEnabled {
		tools = append(tools, tool("mcp_call",
			"Interact with external MCP (Model Context Protocol) servers. Use operation=list_servers to see connected servers, operation=list_tools to discover available tools on a server, or operation=call_tool to execute a tool.",
			schema(map[string]interface{}{
				"operation": prop("string", "One of: list_servers, list_tools, call_tool"),
				"server":    prop("string", "Name of the MCP server (required for list_tools, call_tool)"),
				"tool_name": prop("string", "Name of the tool to call (required for call_tool)"),
				"mcp_args":  map[string]interface{}{"type": "object", "description": "Arguments to pass to the MCP tool (for call_tool)"},
			}, "operation"),
		))
	}
	if ff.SandboxEnabled {
		sandboxProps := map[string]interface{}{
			"code":         prop("string", "The complete source code to execute"),
			"sandbox_lang": prop("string", "Programming language: python (default), javascript, go, java, cpp, r"),
			"libraries":    map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": "Optional packages to install before running (e.g. ['requests', 'pandas'])"},
			"description":  prop("string", "Brief description of what this code does"),
		}
		if ff.PythonSecretInjectionEnabled {
			sandboxProps["vault_keys"] = map[string]interface{}{
				"type":        "array",
				"description": "List of vault secret key names to inject as AURAGO_SECRET_<KEY> environment variables. Only user/agent-created secrets are accessible.",
				"items":       map[string]interface{}{"type": "string"},
			}
			sandboxProps["credential_ids"] = map[string]interface{}{
				"type":        "array",
				"description": "List of credential UUIDs to inject as AURAGO_CRED_<NAME>_USERNAME / _PASSWORD / _TOKEN environment variables. Only credentials with 'allow_python' enabled are accessible.",
				"items":       map[string]interface{}{"type": "string"},
			}
		}
		tools = append(tools, tool("execute_sandbox",
			"Execute code in an isolated Docker sandbox. Supports multiple languages (Python, JavaScript, Go, Java, C++, R). Use this as the DEFAULT tool for writing and running code — it is safer than execute_python because code runs in an isolated container with no host access.",
			schema(sandboxProps, "code"),
		))
	}
	if ff.AdGuardEnabled {
		tools = append(tools, tool("adguard",
			"Manage AdGuard Home DNS server. Supports: status, stats, stats_top, query_log, query_log_clear, filtering_status, filtering_toggle, filtering_add_url, filtering_remove_url, filtering_refresh, filtering_set_rules, rewrite_list, rewrite_add, rewrite_delete, blocked_services_list, blocked_services_set, safebrowsing_status, safebrowsing_toggle, parental_status, parental_toggle, dhcp_status, dhcp_set_config, dhcp_add_lease, dhcp_remove_lease, clients, client_add, client_update, client_delete, dns_info, dns_config, test_upstream.",
			schema(map[string]interface{}{
				"operation": prop("string", "The operation to perform (e.g. status, stats, query_log, rewrite_add, filtering_toggle, etc.)"),
				"query":     prop("string", "Search query for query_log"),
				"limit":     map[string]interface{}{"type": "integer", "description": "Max results to return (default: 25 for query_log)"},
				"offset":    map[string]interface{}{"type": "integer", "description": "Pagination offset for query_log"},
				"domain":    prop("string", "Domain for rewrite operations"),
				"answer":    prop("string", "Answer IP/CNAME for rewrite operations"),
				"name":      prop("string", "Name for filter lists or client delete"),
				"url":       prop("string", "URL for filter list add/remove"),
				"rules":     prop("string", "Custom filtering rules (newline-separated)"),
				"enabled":   map[string]interface{}{"type": "boolean", "description": "Enable/disable toggle for filtering, safebrowsing, parental"},
				"services":  map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": "Service IDs for blocked_services_set or upstream DNS servers for test_upstream"},
				"mac":       prop("string", "MAC address for DHCP lease operations"),
				"ip":        prop("string", "IP address for DHCP lease operations"),
				"hostname":  prop("string", "Hostname for DHCP lease operations"),
				"content":   prop("string", "Raw JSON config for DHCP, client, or DNS settings operations"),
			}, "operation"),
		))
	}
	if ff.GoogleWorkspaceEnabled {
		tools = append(tools, tool("google_workspace",
			"Interact with Google Workspace services (Gmail, Calendar, Drive, Docs, Sheets). "+
				"Perform operations like listing/reading/sending emails, managing calendar events, "+
				"searching Drive files, and reading/writing documents and spreadsheets.",
			schema(map[string]interface{}{
				"operation": map[string]interface{}{
					"type":        "string",
					"description": "Operation to perform",
					"enum": []string{
						"gmail_list", "gmail_read", "gmail_send", "gmail_modify_labels",
						"calendar_list", "calendar_create", "calendar_update",
						"drive_search", "drive_get_content",
						"docs_get", "docs_create", "docs_update",
						"sheets_get", "sheets_update", "sheets_create",
					},
				},
				"message_id":    prop("string", "Gmail message ID (for gmail_read, gmail_modify_labels)"),
				"to":            prop("string", "Recipient email address (for gmail_send)"),
				"subject":       prop("string", "Email subject (for gmail_send)"),
				"body":          prop("string", "Email body or document content"),
				"add_labels":    map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": "Label IDs to add (for gmail_modify_labels)"},
				"remove_labels": map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": "Label IDs to remove (for gmail_modify_labels)"},
				"query":         prop("string", "Search query (Gmail search syntax for gmail_list, or Drive search for drive_search)"),
				"max_results":   map[string]interface{}{"type": "integer", "description": "Maximum number of results to return (default: 10)"},
				"event_id":      prop("string", "Calendar event ID (for calendar_update)"),
				"title":         prop("string", "Event summary or document title"),
				"start_time":    prop("string", "Event start time in RFC3339 format (for calendar_create/update)"),
				"end_time":      prop("string", "Event end time in RFC3339 format (for calendar_create/update)"),
				"description":   prop("string", "Event description or additional details"),
				"file_id":       prop("string", "Drive file ID (for drive_get_content)"),
				"document_id":   prop("string", "Google Docs or Sheets document ID"),
				"range":         prop("string", "Sheet cell range in A1 notation (for sheets_get, sheets_update)"),
				"values":        map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}}, "description": "2D array of cell values (for sheets_update)"},
			}, "operation"),
		))
	}
	if ff.OneDriveEnabled {
		tools = append(tools, tool("onedrive",
			"Interact with the user's Microsoft OneDrive cloud storage. "+
				"List, read, search, upload, delete, move, copy files and folders, get storage quota, and create share links.",
			schema(map[string]interface{}{
				"operation": map[string]interface{}{
					"type":        "string",
					"description": "Operation to perform",
					"enum":        []string{"list", "info", "read", "download", "search", "quota", "upload", "write", "mkdir", "delete", "move", "copy", "share"},
				},
				"path":        prop("string", "Path in OneDrive (e.g. '/Documents/report.txt' or '/' for root). Required for most operations."),
				"destination": prop("string", "Destination path for move/copy operations"),
				"content":     prop("string", "File content to upload (for upload/write), or search query (for search)"),
				"max_results": map[string]interface{}{"type": "integer", "description": "Maximum number of results (default: 50 for list, 25 for search)"},
			}, "operation"),
		))
	}

	if ff.KoofrEnabled {
		tools = append(tools, tool("koofr",
			"Manage files in Koofr cloud storage: list directory contents, read files, write/upload files, "+
				"create directories, delete files/directories, rename/move, and copy files.",
			schema(map[string]interface{}{
				"operation": map[string]interface{}{
					"type":        "string",
					"description": "File operation to perform",
					"enum":        []string{"list", "read", "write", "mkdir", "delete", "rename", "copy"},
				},
				"path":        prop("string", "File or directory path in Koofr (e.g. '/My Files/documents/'). Required for all operations."),
				"destination": prop("string", "Destination path for rename/copy operations."),
				"content":     prop("string", "File content to write (for 'write' operation)."),
			}, "operation", "path"),
		))
	}

	if ff.ImageGenerationEnabled {
		tools = append(tools, tool("generate_image",
			"Generate images from text prompts using AI. Supports text-to-image and image-to-image generation. "+
				"Returns a markdown image link that can be included in the response to show the generated image to the user.",
			schema(map[string]interface{}{
				"prompt": map[string]interface{}{
					"type":        "string",
					"description": "Text description of the image to generate. Be detailed and specific for best results.",
				},
				"size":           prop("string", "Image size (e.g. '1024x1024', '1344x768', '768x1344'). Default: 1024x1024"),
				"quality":        prop("string", "Image quality ('standard' or 'hd'). Default: standard"),
				"style":          prop("string", "Image style ('natural' or 'vivid'). Default: natural"),
				"model":          prop("string", "Override the default model for this generation (optional)"),
				"source_image":   prop("string", "Path to an existing image for image-to-image generation (optional)"),
				"enhance_prompt": map[string]interface{}{"type": "boolean", "description": "If true, the prompt will be enhanced by the LLM before generation (optional)"},
			}, "prompt"),
		))
	}

	if ff.RemoteControlEnabled {
		tools = append(tools, tool("remote_control",
			"Manage and interact with remote devices connected to this AuraGo instance. "+
				"List devices, check status, execute commands, transfer files, edit files precisely, edit JSON/YAML/XML files, search files, read file sections, and get system information from remote machines.",
			schema(map[string]interface{}{
				"operation": map[string]interface{}{
					"type":        "string",
					"description": "Operation to perform",
					"enum":        []string{"list_devices", "device_status", "execute_command", "read_file", "write_file", "list_files", "sysinfo", "revoke_device", "edit_file", "json_edit", "yaml_edit", "xml_edit", "file_search", "file_read_advanced"},
				},
				"device_id":   prop("string", "Device ID (for all device-specific operations)"),
				"device_name": prop("string", "Device name — alternative to device_id for lookup"),
				"command":     prop("string", "Shell command to execute (for execute_command)"),
				"path":        prop("string", "File or directory path on the remote device"),
				"content":     prop("string", "File content to write (for write_file) or text to insert (for edit_file insert_after/insert_before/append/prepend)"),
				"recursive":   map[string]interface{}{"type": "boolean", "description": "List directory recursively (for list_files, default: false)"},
				"action":      prop("string", "Sub-operation. edit_file: str_replace, str_replace_all, insert_after, insert_before, append, prepend, delete_lines. json_edit/yaml_edit: get, set, delete, keys, validate, format. xml_edit: get, set_text, set_attribute, add_element, delete, validate, format. file_search: grep, grep_recursive, find. file_read_advanced: read_lines, head, tail, count_lines, search_context"),
				"old":         prop("string", "Text to find (for edit_file str_replace/str_replace_all)"),
				"new":         prop("string", "Replacement text (for edit_file str_replace/str_replace_all)"),
				"marker":      prop("string", "Anchor text (for edit_file insert_after/insert_before)"),
				"start_line":  prop("integer", "First line (for edit_file delete_lines, file_read_advanced read_lines)"),
				"end_line":    prop("integer", "Last line (for edit_file delete_lines, file_read_advanced read_lines)"),
				"json_path":   prop("string", "Dot-separated path for json_edit/yaml_edit (e.g. 'server.port')"),
				"xpath":       prop("string", "XPath expression for xml_edit (e.g. '//server', './config/database')"),
				"set_value":   map[string]interface{}{"description": "Value to set for json_edit/yaml_edit/xml_edit operations"},
				"pattern":     prop("string", "Search pattern (regex) for file_search and file_read_advanced search_context"),
				"glob":        prop("string", "File glob pattern for file_search grep_recursive/find"),
				"output_mode": prop("string", "Output mode for file_search: content (default) or count"),
				"line_count":  prop("integer", "Number of lines for file_read_advanced head/tail or context lines for search_context"),
			}, "operation"),
		))
	}

	if ff.MediaRegistryEnabled {
		tools = append(tools, tool("media_registry",
			"Search, browse, tag, and manage the media registry. Tracks all generated images, TTS audio, and other media files with metadata, tags, and descriptions. "+
				"Operations: register (manual add), search (full-text across description/prompt/tags), get (by id), list (optionally filter by media_type), update (description/tags), tag (add/remove/set tags), delete (soft-delete), stats (aggregate counts).",
			schema(map[string]interface{}{
				"operation": map[string]interface{}{
					"type":        "string",
					"description": "Operation to perform",
					"enum":        []string{"register", "search", "get", "list", "update", "tag", "delete", "stats"},
				},
				"media_type":  prop("string", "Media type filter: image, tts, audio, music"),
				"filename":    prop("string", "Filename of the media file (required for register)"),
				"file_path":   prop("string", "File path of the media file (required for register)"),
				"query":       prop("string", "Search query (searches description, prompt, tags, filename)"),
				"id":          map[string]interface{}{"type": "integer", "description": "Media item ID (for get/update/tag/delete)"},
				"description": prop("string", "Short description of the media item"),
				"tags":        map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": "Tags for the media item"},
				"tag_mode":    prop("string", "Tag operation mode: add (default), remove, set. Only for 'tag' operation."),
				"limit":       map[string]interface{}{"type": "integer", "description": "Max results (default: 20)"},
				"offset":      map[string]interface{}{"type": "integer", "description": "Pagination offset"},
			}, "operation"),
		))
	}
	if ff.HomepageRegistryEnabled {
		tools = append(tools, tool("homepage_registry",
			"Track and manage homepage/web projects. Records URL, framework, deploy history, edit reasons, and known problems. "+
				"Operations: register, search, get (by id or name), list, update, delete, log_edit (record edit with reason), log_deploy (record deploy with URL), log_problem (add known issue), resolve_problem (mark issue resolved).",
			schema(map[string]interface{}{
				"operation": map[string]interface{}{
					"type":        "string",
					"description": "Operation to perform",
					"enum":        []string{"register", "search", "get", "list", "update", "delete", "log_edit", "log_deploy", "log_problem", "resolve_problem"},
				},
				"name":        prop("string", "Project name (unique identifier)"),
				"query":       prop("string", "Search query (searches name, description, framework, URL, notes)"),
				"id":          map[string]interface{}{"type": "integer", "description": "Project ID"},
				"description": prop("string", "Project description"),
				"framework":   prop("string", "Web framework (next, vite, astro, svelte, vue, html, etc.)"),
				"project_dir": prop("string", "Project directory within workspace"),
				"url":         prop("string", "Live URL of the project or deploy URL for log_deploy"),
				"status":      prop("string", "Project status: active, archived, maintenance"),
				"reason":      prop("string", "Edit reason (for log_edit)"),
				"problem":     prop("string", "Problem description (for log_problem/resolve_problem)"),
				"tags":        map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": "Project tags"},
				"notes":       prop("string", "Additional notes"),
				"limit":       map[string]interface{}{"type": "integer", "description": "Max results (default: 20)"},
				"offset":      map[string]interface{}{"type": "integer", "description": "Pagination offset"},
			}, "operation"),
		))
	}

	if ff.DocumentCreatorEnabled {
		tools = append(tools, tool("document_creator",
			"Create PDF documents, convert files to PDF, merge PDFs, and take screenshots. "+
				"Backend is configured in settings: 'maroto' (built-in, create_pdf only) or 'gotenberg' (Docker sidecar, all operations). "+
				"Operations: create_pdf (structured document from sections), url_to_pdf (capture webpage), html_to_pdf (render HTML), "+
				"markdown_to_pdf (render Markdown), convert_document (Office files to PDF via LibreOffice), merge_pdfs (combine multiple PDFs), "+
				"screenshot_url (capture webpage as image), screenshot_html (render HTML to image), health (check Gotenberg status).",
			schema(map[string]interface{}{
				"operation": map[string]interface{}{
					"type":        "string",
					"description": "Operation to perform",
					"enum":        []string{"create_pdf", "url_to_pdf", "html_to_pdf", "markdown_to_pdf", "convert_document", "merge_pdfs", "screenshot_url", "screenshot_html", "health"},
				},
				"title":        prop("string", "Document title (for create_pdf)"),
				"content":      prop("string", "HTML content (for html_to_pdf, screenshot_html), Markdown content (for markdown_to_pdf), or text content (for create_pdf without sections)"),
				"url":          prop("string", "URL to capture (for url_to_pdf, screenshot_url)"),
				"filename":     prop("string", "Output filename without extension (auto-generated if omitted)"),
				"paper_size":   map[string]interface{}{"type": "string", "description": "Paper size", "enum": []string{"A4", "A3", "A5", "Letter", "Legal", "Tabloid"}},
				"landscape":    map[string]interface{}{"type": "boolean", "description": "Landscape orientation (default: false)"},
				"sections":     prop("string", "JSON array of sections for create_pdf. Each section: {\"type\":\"text|table|list\",\"header\":\"...\",\"body\":\"...\",\"rows\":[[...]]}"),
				"source_files": prop("string", "JSON array of file paths for merge_pdfs or convert_document"),
			}, "operation"),
		))
	}

	if ff.WebCaptureEnabled {
		tools = append(tools, tool("web_capture",
			"Take a screenshot (PNG) or render a PDF of any URL using a headless Chromium browser. "+
				"Does not require Gotenberg or any external service — uses the embedded go-rod browser. "+
				"Operations: 'screenshot' saves a PNG image, 'pdf' saves a PDF. "+
				"Optionally wait for a CSS selector before capture, and capture full scrollable page for screenshots.",
			schema(map[string]interface{}{
				"operation": map[string]interface{}{
					"type":        "string",
					"description": "Capture type",
					"enum":        []string{"screenshot", "pdf"},
				},
				"url":        prop("string", "Page URL to capture (http or https)"),
				"selector":   prop("string", "Optional CSS selector to wait for before capture"),
				"full_page":  map[string]interface{}{"type": "boolean", "description": "Capture full scrollable page height (screenshot only, default: false)"},
				"output_dir": prop("string", "Directory to save the file (default: agent_workspace/workdir)"),
			}, "operation", "url"),
		))

		tools = append(tools, tool("web_performance_audit",
			"Measure page load performance of any URL using a headless Chromium browser. "+
				"Returns Core Web Vitals and related metrics: TTFB, First Contentful Paint, DOM Content Loaded, "+
				"full Load time, DOM element count, resource count, total transfer size, JS heap usage, "+
				"and the 5 largest resources by size. Useful for diagnosing slow pages or comparing performance.",
			schema(map[string]interface{}{
				"url":      prop("string", "Page URL to audit (http or https)"),
				"viewport": prop("string", "Browser viewport size as 'WIDTHxHEIGHT' (default: '1280x720')"),
			}, "url"),
		))
	}

	if ff.NetworkPingEnabled {
		tools = append(tools, tool("network_ping",
			"Ping a host using ICMP echo requests and return latency statistics (min/avg/max RTT, packet loss). "+
				"Requires raw socket access — works without elevation on Windows; on Linux the process needs CAP_NET_RAW or root.",
			schema(map[string]interface{}{
				"host":    prop("string", "Hostname or IP address to ping"),
				"count":   map[string]interface{}{"type": "integer", "description": "Number of packets to send (1–20, default: 4)"},
				"timeout": map[string]interface{}{"type": "integer", "description": "Total timeout in seconds (1–60, default: 10)"},
			}, "host"),
		))
	}

	tools = append(tools, tool("detect_file_type",
		"Identify the true file type of one or more files using magic-byte detection (ignores file extension). "+
			"Returns MIME type, canonical extension, and type group (image, video, audio, application…). "+
			"Pass a single file path or a directory path. Set recursive to scan sub-directories.",
		schema(map[string]interface{}{
			"path":      prop("string", "Absolute or relative path to a file or directory"),
			"recursive": map[string]interface{}{"type": "boolean", "description": "Recurse into sub-directories (only when path is a directory, default: false)"},
		}, "path"),
	))

	return tools
}
