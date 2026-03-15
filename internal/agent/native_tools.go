package agent

// native_tools.go — Builds OpenAI-compatible tool schema definitions from the
// AuraGo built-in tool registry plus dynamically loaded skills and custom tools.
// Used when config.Agent.UseNativeFunctions = true.

import (
	"encoding/json"
	"log/slog"
	"regexp"
	"strings"

	openai "github.com/sashabaranov/go-openai"

	"aurago/internal/tools"
)

// prop creates a JSON Schema property entry.
func prop(typ, description string) map[string]interface{} {
	return map[string]interface{}{"type": typ, "description": description}
}

// schema builds a standard object schema with required fields.
func schema(properties map[string]interface{}, required ...string) map[string]interface{} {
	s := map[string]interface{}{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		s["required"] = required
	}
	return s
}

// tool creates an openai.Tool from a name, description, and parameters schema.
func tool(name, description string, params map[string]interface{}) openai.Tool {
	return openai.Tool{
		Type: openai.ToolTypeFunction,
		Function: &openai.FunctionDefinition{
			Name:        name,
			Description: description,
			Parameters:  params,
		},
	}
}

// ToolFeatureFlags controls which optional tool schemas are included.
type ToolFeatureFlags struct {
	HomeAssistantEnabled    bool
	DockerEnabled           bool
	CoAgentEnabled          bool
	SudoEnabled             bool
	WebhooksEnabled         bool
	ProxmoxEnabled          bool
	OllamaEnabled           bool
	TailscaleEnabled        bool
	AnsibleEnabled          bool
	InvasionControlEnabled  bool
	GitHubEnabled           bool
	MQTTEnabled             bool
	AdGuardEnabled          bool
	MCPEnabled              bool
	SandboxEnabled          bool
	MeshCentralEnabled      bool
	HomepageEnabled         bool
	NetlifyEnabled          bool
	FirewallEnabled         bool
	EmailEnabled            bool
	CloudflareTunnelEnabled bool
	GoogleWorkspaceEnabled  bool
	ImageGenerationEnabled  bool
	RemoteControlEnabled    bool
	// Danger Zone toggles
	AllowShell               bool
	AllowPython              bool
	AllowFilesystemWrite     bool
	AllowNetworkRequests     bool
	AllowRemoteShell         bool
	AllowSelfUpdate          bool
	HomepageAllowLocalServer bool // Allow Python HTTP server fallback when Docker unavailable
	// Built-in tool toggles
	MemoryEnabled            bool
	KnowledgeGraphEnabled    bool
	SecretsVaultEnabled      bool
	SchedulerEnabled         bool
	NotesEnabled             bool
	MissionsEnabled          bool
	StopProcessEnabled       bool
	InventoryEnabled         bool
	MemoryMaintenanceEnabled bool
	WOLEnabled               bool
	MediaRegistryEnabled     bool
	HomepageRegistryEnabled  bool
	JournalEnabled           bool
	MemoryAnalysisEnabled    bool
}

// builtinToolSchemas returns schemas for all built-in AuraGo tools.
// Optional feature tools (home_assistant, docker, co_agent) are only
// included when their corresponding feature is enabled in the config.
func builtinToolSchemas(ff ToolFeatureFlags) []openai.Tool {
	executePythonDesc := "Save and execute a Python script. Use for data processing, automation, calculations, and scripting tasks."
	if ff.SandboxEnabled {
		executePythonDesc = "Save and execute a Python script on the HOST system (unsandboxed). Use ONLY for persistent tools (save_tool), registered skills, or when execute_sandbox is unavailable. Prefer execute_sandbox for all other code execution."
	}

	tools := []openai.Tool{
		tool("execute_skill",
			"Run a pre-built registered skill (e.g. web_search, ddg_search, pdf_extractor, wikipedia_search, virustotal_scan). Use for external data retrieval.",
			schema(map[string]interface{}{
				"skill": prop("string", "Name of the skill to execute (e.g. 'ddg_search', 'web_scraper', 'pdf_extractor', 'virustotal_scan')"),
				"skill_args": map[string]interface{}{
					"type":        "object",
					"description": "Arguments to pass to the skill as key-value pairs",
				},
			}, "skill"),
		),
		tool("filesystem",
			"Read, write, move, copy, delete files and directories, or list directory contents.",
			schema(map[string]interface{}{
				"operation": map[string]interface{}{
					"type":        "string",
					"description": "Operation to perform",
					"enum":        []string{"read_file", "write_file", "delete", "move", "list_dir", "create_dir", "stat"},
				},
				"file_path":   prop("string", "Path to the file or directory"),
				"content":     prop("string", "Content to write (for write_file operations)"),
				"destination": prop("string", "Destination path (for move operations)"),
				"preview":     prop("boolean", "If true, only return first 100 lines (for read_file)"),
			}, "operation", "file_path"),
		),
		tool("system_metrics",
			"Retrieve current system resource usage: CPU, memory, disk, running processes.",
			schema(map[string]interface{}{
				"target": map[string]interface{}{
					"type":        "string",
					"description": "Metrics to retrieve",
					"enum":        []string{"all", "cpu", "memory", "disk", "processes"},
				},
			}),
		),
		tool("process_management",
			"List, kill, or inspect running background processes managed by AuraGo.",
			schema(map[string]interface{}{
				"operation": map[string]interface{}{
					"type":        "string",
					"description": "Operation to perform",
					"enum":        []string{"list", "kill", "status"},
				},
				"pid":   prop("integer", "Process ID (for kill/status operations)"),
				"label": prop("string", "Process label (alternative to pid)"),
			}, "operation"),
		),
		tool("follow_up",
			"Schedule an autonomous background task for yourself to execute immediately after this response. "+
				"ONLY use this when you have all required information and will perform the work yourself. "+
				"⚠️ NEVER use follow_up to ask the user for input or relay a question — that creates an infinite loop. "+
				"If you are missing information needed to complete a task, respond DIRECTLY to the user with your question instead of using this tool.",
			schema(map[string]interface{}{
				"task_prompt": prop("string", "Concrete, self-contained task the agent will perform autonomously. Must NOT be a question directed at the user."),
			}, "task_prompt"),
		),
		tool("analyze_image",
			"Analyze an image file using the Vision LLM. Describe content, read text (OCR), identify objects.",
			schema(map[string]interface{}{
				"file_path": prop("string", "Path to the image file (JPEG, PNG, GIF, WebP)"),
				"prompt":    prop("string", "Custom analysis prompt (default: general description)"),
			}, "file_path"),
		),
		tool("transcribe_audio",
			"Transcribe an audio file to text using the configured Speech-to-Text service.",
			schema(map[string]interface{}{
				"file_path": prop("string", "Path to the audio file (MP3, WAV, OGG, FLAC, M4A)"),
			}, "file_path"),
		),
		tool("send_image",
			"Send an image to the user. Shown inline with a click-to-zoom lightbox in the Web UI, as a native photo in Telegram, and as a file attachment in Discord. Provide a local workspace path or an image URL.",
			schema(map[string]interface{}{
				"path":    prop("string", "Local file path within the workspace (e.g. 'images/chart.png') or a full HTTPS URL to an image"),
				"caption": prop("string", "Optional caption or description shown with the image"),
			}, "path"),
		),
	}

	// ── Conditionally-included built-in tools ────────────────────────────────

	if ff.AllowShell {
		tools = append(tools, tool("execute_shell",
			"Run a shell command on the local system. Use for file operations, system info, running programs, etc.",
			schema(map[string]interface{}{
				"command":    prop("string", "The shell command to execute"),
				"background": prop("boolean", "Run as background process (default false)"),
			}, "command"),
		))
	}

	if ff.AllowPython {
		tools = append(tools, tool("execute_python",
			executePythonDesc,
			schema(map[string]interface{}{
				"code":        prop("string", "The complete Python code to execute"),
				"description": prop("string", "Brief description of what this script does"),
			}, "code"),
		))

		tools = append(tools, tool("save_tool",
			"Save a new Python tool/script to the tools directory and register it in the manifest.",
			schema(map[string]interface{}{
				"name":        prop("string", "Filename for the tool (e.g. 'my_tool.py')"),
				"description": prop("string", "What this tool does"),
				"code":        prop("string", "Complete Python code for the tool"),
			}, "name", "description", "code"),
		))
	}

	if ff.AllowRemoteShell {
		tools = append(tools, tool("remote_execution",
			"Execute a command on a remote SSH server registered in the inventory.",
			schema(map[string]interface{}{
				"server_id": prop("string", "Server ID or hostname from the inventory"),
				"command":   prop("string", "Shell command to run on the remote server"),
				"direction": map[string]interface{}{
					"type":        "string",
					"description": "For file transfer: 'upload' or 'download'",
					"enum":        []string{"upload", "download"},
				},
				"local_path":  prop("string", "Local file path (for file transfer)"),
				"remote_path": prop("string", "Remote file path (for file transfer)"),
			}, "server_id", "command"),
		))
	}

	if ff.AllowNetworkRequests {
		tools = append(tools, tool("api_request",
			"Make an HTTP request to an external API endpoint.",
			schema(map[string]interface{}{
				"url":    prop("string", "The full URL to request"),
				"method": map[string]interface{}{"type": "string", "description": "HTTP method", "enum": []string{"GET", "POST", "PUT", "PATCH", "DELETE"}},
				"headers": map[string]interface{}{
					"type":        "object",
					"description": "HTTP headers as key-value string pairs",
				},
				"body": prop("string", "Request body (for POST/PUT/PATCH)"),
			}, "url"),
		))
	}

	if ff.MemoryEnabled {
		tools = append(tools,
			tool("manage_memory",
				"Store or delete facts in long-term memory, or save structured key-value data to core memory.",
				schema(map[string]interface{}{
					"operation": map[string]interface{}{
						"type":        "string",
						"description": "Operation to perform",
						"enum":        []string{"store", "delete", "save_core", "delete_core"},
					},
					"fact":  prop("string", "A factual statement to store (for 'store' operation)"),
					"key":   prop("string", "Key name for core memory (for 'save_core'/'delete_core')"),
					"value": prop("string", "Value to save for the given key (for 'save_core')"),
				}, "operation"),
			),
			tool("query_memory",
				"Search across ALL memory sources: long-term vector DB, knowledge graph, journal, notes, and core memory. Returns combined results from multiple sources. Use 'sources' to limit search to specific stores.",
				schema(map[string]interface{}{
					"query": prop("string", "Natural language search query"),
					"sources": map[string]interface{}{
						"type":        "array",
						"description": "Memory sources to search. Default: all available. Options: vector_db, knowledge_graph, journal, notes, core_memory, error_patterns",
						"items":       map[string]interface{}{"type": "string", "enum": []string{"vector_db", "knowledge_graph", "journal", "notes", "core_memory", "error_patterns"}},
					},
					"limit": map[string]interface{}{
						"type":        "integer",
						"description": "Max results per source (default 5)",
					},
				}, "query"),
			),
		)

		// memory_reflect — only when memory_analysis is enabled
		if ff.MemoryAnalysisEnabled {
			tools = append(tools, tool("memory_reflect",
				"Generate a reflection on recent memory activity: analyze patterns, detect contradictions, identify knowledge gaps, and suggest memory optimizations. Use periodically or when the user asks about memory health.",
				schema(map[string]interface{}{
					"scope": map[string]interface{}{
						"type":        "string",
						"description": "Scope of the reflection: 'recent' (last 7 days), 'monthly' (last 30 days), or 'full' (all memories)",
						"enum":        []string{"recent", "monthly", "full"},
					},
				}),
			))
		}
	}

	// Cheat Sheets — always available (DB is always initialized)
	tools = append(tools, tool("cheatsheet",
		"Manage cheat sheets (reusable workflow instructions). List, view, create, update, or delete cheat sheets that describe step-by-step procedures.",
		schema(map[string]interface{}{
			"operation": map[string]interface{}{
				"type":        "string",
				"description": "Operation to perform",
				"enum":        []string{"list", "get", "create", "update", "delete"},
			},
			"id":      prop("string", "Cheat sheet ID (for get/update/delete). Can also be the name for 'get'."),
			"name":    prop("string", "Name of the cheat sheet (for create/update)"),
			"content": prop("string", "Markdown content of the cheat sheet (for create/update)"),
			"active":  map[string]interface{}{"type": "boolean", "description": "Whether the cheat sheet is active (for update)"},
		}, "operation"),
	))

	if ff.KnowledgeGraphEnabled {
		tools = append(tools, tool("knowledge_graph",
			"Store or query relationships between entities in the knowledge graph.",
			schema(map[string]interface{}{
				"operation": map[string]interface{}{
					"type":        "string",
					"description": "Operation to perform",
					"enum":        []string{"add_relation", "query", "delete_relation"},
				},
				"source":     prop("string", "Source entity name"),
				"target":     prop("string", "Target entity name"),
				"relation":   prop("string", "Relationship type (e.g. 'owns', 'is_part_of')"),
				"query":      prop("string", "Natural language query for 'query' operation"),
				"properties": map[string]interface{}{"type": "object", "description": "Optional properties for the relation"},
			}, "operation"),
		))
	}

	if ff.SecretsVaultEnabled {
		tools = append(tools, tool("secrets_vault",
			"Store, retrieve, list, or delete secrets from the encrypted vault.",
			schema(map[string]interface{}{
				"operation": map[string]interface{}{
					"type":        "string",
					"description": "Vault operation",
					"enum":        []string{"get", "set", "delete", "list"},
				},
				"key":   prop("string", "Secret key name"),
				"value": prop("string", "Secret value (for 'set' operation)"),
			}, "operation"),
		))
	}

	if ff.SchedulerEnabled {
		tools = append(tools, tool("cron_scheduler",
			"Schedule, list, enable, disable, or remove recurring background tasks.",
			schema(map[string]interface{}{
				"operation": map[string]interface{}{
					"type":        "string",
					"description": "Scheduler operation",
					"enum":        []string{"add", "list", "remove", "enable", "disable"},
				},
				"cron_expr":   prop("string", "Cron expression (e.g. '0 9 * * *' for daily at 9am)"),
				"task_prompt": prop("string", "The prompt/task to execute on schedule"),
				"id":          prop("string", "Job ID (for remove/enable/disable)"),
				"label":       prop("string", "Human-readable label for the job"),
			}, "operation"),
		))
	}

	if ff.NotesEnabled {
		tools = append(tools, tool("manage_notes",
			"Create, list, update, toggle, or delete persistent notes and to-do items.",
			schema(map[string]interface{}{
				"operation": map[string]interface{}{
					"type":        "string",
					"description": "Notes operation",
					"enum":        []string{"add", "list", "update", "toggle", "delete"},
				},
				"title":    prop("string", "Title of the note (required for add)"),
				"content":  prop("string", "Detailed content or body text"),
				"category": prop("string", "Category tag (e.g. 'todo', 'ideas', 'shopping'). Default: 'general'"),
				"priority": prop("integer", "Priority: 1=low, 2=medium (default), 3=high"),
				"due_date": prop("string", "Due date in YYYY-MM-DD format"),
				"note_id":  prop("integer", "Note ID (required for update/toggle/delete)"),
				"done":     prop("integer", "Filter for list: -1=all, 0=open only, 1=done only"),
			}, "operation"),
		))
	}

	if ff.JournalEnabled {
		tools = append(tools, tool("manage_journal",
			"Add, list, search, or delete journal entries. Journal entries capture important events, milestones, preferences, and reflections. Use get_summary to retrieve a daily summary.",
			schema(map[string]interface{}{
				"operation": map[string]interface{}{
					"type":        "string",
					"description": "Journal operation",
					"enum":        []string{"add", "list", "search", "delete", "get_summary"},
				},
				"title":      prop("string", "Title of the journal entry (required for add)"),
				"content":    prop("string", "Detailed content of the journal entry"),
				"entry_type": prop("string", "Type of entry: reflection, milestone, preference, task_completed, integration, learning, error_recovery, system_event"),
				"tags":       prop("string", "Comma-separated tags for categorization"),
				"importance": prop("integer", "Importance level 1-5 (default 3). 5=critical milestone, 1=minor note"),
				"query":      prop("string", "Search keyword (required for search)"),
				"from_date":  prop("string", "Start date filter YYYY-MM-DD (for list/get_summary)"),
				"to_date":    prop("string", "End date filter YYYY-MM-DD (for list/get_summary)"),
				"entry_id":   prop("integer", "Entry ID (required for delete)"),
				"limit":      prop("integer", "Maximum entries to return (default 20)"),
			}, "operation"),
		))
	}

	if ff.MissionsEnabled {
		tools = append(tools, tool("manage_missions",
			"Create, list, update, delete, or run background automation tasks (missions) in the Mission Control system. Use this to schedule recurring work for the agent or define on-demand jobs.",
			schema(map[string]interface{}{
				"operation": map[string]interface{}{
					"type":        "string",
					"description": "Mission operation",
					"enum":        []string{"add", "list", "update", "delete", "run"},
				},
				"title":     prop("string", "Name of the mission (required for add)"),
				"command":   prop("string", "The task prompt that the agent will execute"),
				"cron_expr": prop("string", "Optional cron expression for scheduling (e.g. '0 9 * * *' for daily at 9am)"),
				"priority":  prop("integer", "Priority: 1=low, 2=medium (default), 3=high"),
				"locked":    prop("boolean", "If true, the mission is locked and cannot be deleted until unlocked"),
				"id":        prop("string", "Mission ID (required for update/delete/run)"),
			}, "operation"),
		))
	}

	if ff.InventoryEnabled {
		tools = append(tools, tool("query_inventory",
			"Search registered servers, virtual machines, and network devices by tag or hostname in the device inventory.",
			schema(map[string]interface{}{
				"tag":         prop("string", "Filter by a specific tag (e.g. 'prod', 'db', 'web')"),
				"device_type": prop("string", "Filter by type (e.g. 'server', 'docker', 'vm', 'network_device')"),
				"hostname":    prop("string", "Search for a specific name or substring"),
			}),
		))
		tools = append(tools, tool("register_device",
			"Add a new device to the inventory. Automatically stores credentials in the vault.",
			schema(map[string]interface{}{
				"hostname":         prop("string", "Device name or hostname"),
				"device_type":      prop("string", "Type (e.g. 'server', 'docker', 'vm', 'network_device')"),
				"ip_address":       prop("string", "IP address or FQDN"),
				"port":             prop("integer", "Port number (default 22 for SSH)"),
				"username":         prop("string", "Login username"),
				"password":         prop("string", "Login password (optional)"),
				"private_key_path": prop("string", "Path to private key (optional)"),
				"description":      prop("string", "Brief description"),
				"tags":             prop("string", "Comma-separated tags (e.g. 'prod,db')"),
				"mac_address":      prop("string", "MAC address for Wake-on-LAN (optional)"),
			}, "hostname", "device_type"),
		))
	}

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
			"Spawn and manage parallel co-agents that work on sub-tasks independently. Co-agents run in background goroutines with their own LLM context and return results when done.",
			schema(map[string]interface{}{
				"operation": map[string]interface{}{
					"type":        "string",
					"description": "Operation to perform",
					"enum":        []string{"spawn", "list", "get_result", "stop", "stop_all"},
				},
				"task":          prop("string", "Task description for the co-agent to work on (required for 'spawn')"),
				"co_agent_id":   prop("string", "Co-agent ID (required for 'get_result' and 'stop')"),
				"context_hints": map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": "Optional keywords or topics for RAG context injection (for 'spawn')"},
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
					"description": "Operation to perform",
					"enum":        []string{"init", "start", "stop", "status", "rebuild", "destroy", "exec", "init_project", "build", "install_deps", "lighthouse", "screenshot", "lint", "list_files", "read_file", "write_file", "optimize_images", "dev", "deploy", "test_connection", "webserver_start", "webserver_stop", "webserver_status", "publish_local"},
				},
				"command":     prop("string", "Shell command to execute (for 'exec')"),
				"framework":   prop("string", "Web framework: next, vite, astro, svelte, vue, html (for 'init_project')"),
				"name":        prop("string", "Project name (for 'init_project')"),
				"project_dir": prop("string", "Project subdirectory within /workspace (default: '.')"),
				"build_dir":   prop("string", "Build output directory (auto-detected if empty)"),
				"path":        prop("string", "File path relative to /workspace — MUST include the project subdirectory prefix (e.g. 'my-project/index.html', NOT just 'index.html'). Applies to 'read_file', 'write_file', 'list_files'."),
				"content":     prop("string", "File content to write (for 'write_file')"),
				"url":         prop("string", "URL for lighthouse audit or screenshot"),
				"viewport":    prop("string", "Viewport size for screenshot (e.g. '1280x720')"),
				"packages":    map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": "NPM packages to install (for 'install_deps')"},
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
					"enum":        []string{"list_sites", "get_site", "create_site", "update_site", "delete_site", "list_deploys", "get_deploy", "deploy_zip", "deploy_draft", "rollback", "cancel_deploy", "list_env", "get_env", "set_env", "delete_env", "list_files", "list_forms", "get_submissions", "list_hooks", "create_hook", "delete_hook", "provision_ssl", "check_connection"},
				},
				"site_id":       prop("string", "Netlify site ID (uses default_site_id if omitted)"),
				"site_name":     prop("string", "Site subdomain name for create (name.netlify.app)"),
				"custom_domain": prop("string", "Custom domain for the site"),
				"deploy_id":     prop("string", "Deploy ID (for get_deploy, rollback, cancel_deploy)"),
				"content":       prop("string", "Base64-encoded ZIP archive (for deploy_zip/deploy_draft). NOTE: For workspace projects use 'homepage' action with 'deploy_netlify' operation instead — it builds and ZIPs automatically."),
				"title":         prop("string", "Deploy title/message"),
				"draft":         map[string]interface{}{"type": "boolean", "description": "Deploy as draft (not published)"},
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

	if ff.ProxmoxEnabled {
		tools = append(tools, tool("proxmox",
			"Manage Proxmox VE virtual machines and containers: list nodes/VMs/CTs, start/stop/reboot, snapshots, storage info, cluster resources.",
			schema(map[string]interface{}{
				"operation": map[string]interface{}{
					"type":        "string",
					"description": "Operation to perform",
					"enum":        []string{"list_nodes", "list_vms", "list_containers", "status", "start", "stop", "shutdown", "reboot", "suspend", "resume", "node_status", "cluster_resources", "storage", "create_snapshot", "list_snapshots", "task_log"},
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
	if ff.MeshCentralEnabled {
		tools = append(tools, tool("meshcentral",
			"Manage MeshCentral devices. List device groups, list devices, wake devices via WOL, send power actions, run commands, and execute interactive shell commands with output.",
			schema(map[string]interface{}{
				"operation": map[string]interface{}{
					"type":        "string",
					"description": "Operation to perform",
					"enum":        []string{"list_groups", "list_devices", "wake", "power_action", "run_command", "shell"},
				},
				"mesh_id":      prop("string", "Mesh/Group ID (optional for list_devices to filter results)"),
				"node_id":      prop("string", "Node/Device ID (required for wake, power_action, run_command, shell)"),
				"power_action": map[string]interface{}{"type": "integer", "description": "Power action ID. 1=Sleep, 2=Hibernate, 3=PowerOff, 4=Reset (for power_action)"},
				"command":      prop("string", "Command to run on remote device (for run_command or shell)"),
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
		tools = append(tools, tool("execute_sandbox",
			"Execute code in an isolated Docker sandbox. Supports multiple languages (Python, JavaScript, Go, Java, C++, R). Use this as the DEFAULT tool for writing and running code — it is safer than execute_python because code runs in an isolated container with no host access.",
			schema(map[string]interface{}{
				"code":         prop("string", "The complete source code to execute"),
				"sandbox_lang": prop("string", "Programming language: python (default), javascript, go, java, cpp, r"),
				"libraries":    map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": "Optional packages to install before running (e.g. ['requests', 'pandas'])"},
				"description":  prop("string", "Brief description of what this code does"),
			}, "code"),
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
				"List devices, check status, execute commands, transfer files, and get system information from remote machines.",
			schema(map[string]interface{}{
				"operation": map[string]interface{}{
					"type":        "string",
					"description": "Operation to perform",
					"enum":        []string{"list_devices", "device_status", "execute_command", "read_file", "write_file", "list_files", "sysinfo", "revoke_device"},
				},
				"device_id":   prop("string", "Device ID (for device_status, execute_command, read_file, write_file, list_files, sysinfo, revoke_device)"),
				"device_name": prop("string", "Device name — alternative to device_id for lookup"),
				"command":     prop("string", "Shell command to execute (for execute_command)"),
				"path":        prop("string", "File or directory path on the remote device (for read_file, write_file, list_files)"),
				"content":     prop("string", "File content to write (for write_file)"),
				"recursive":   map[string]interface{}{"type": "boolean", "description": "List directory recursively (for list_files, default: false)"},
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

	return tools
}

// NativeToolCallToToolCall converts an OpenAI native ToolCall response to AuraGo's ToolCall struct.
// Arguments JSON is unmarshalled directly into the struct fields.
func NativeToolCallToToolCall(native openai.ToolCall, logger *slog.Logger) ToolCall {
	// Convert skill__name shortcut to execute_skill so the skill dispatcher handles it correctly.
	name := native.Function.Name
	skillFromShortcut := ""
	if strings.HasPrefix(name, "skill__") {
		skillFromShortcut = strings.TrimPrefix(name, "skill__")
		name = "execute_skill"
	}

	tc := ToolCall{
		IsTool:       true,
		Action:       name,
		Skill:        skillFromShortcut,
		NativeCallID: native.ID,
	}

	if native.Function.Arguments == "" {
		return tc
	}

	// Unmarshal the arguments JSON into the ToolCall struct
	if err := json.Unmarshal([]byte(native.Function.Arguments), &tc); err != nil {
		if logger != nil {
			logger.Warn("[NativeTools] Failed to unmarshal native tool arguments, using raw",
				"name", native.Function.Name, "error", err)
		}
		// Fallback 1: try to put the raw args into Params (works for valid-but-unexpected JSON)
		var rawMap map[string]interface{}
		if json.Unmarshal([]byte(native.Function.Arguments), &rawMap) == nil {
			tc.Params = rawMap
		}
		// Fallback 2: for truncated/malformed JSON, extract known string fields via regex.
		// LLMs occasionally return truncated JSON (e.g. connection reset, token limit).
		// The beginning of the JSON is usually intact, so we can rescue key fields.
		extractField := func(key string) string {
			re := regexp.MustCompile(`"` + regexp.QuoteMeta(key) + `"\s*:\s*"((?:[^"\\]|\\.)*)`)
			if m := re.FindStringSubmatch(native.Function.Arguments); len(m) > 1 {
				return strings.ReplaceAll(strings.ReplaceAll(m[1], `\"`, `"`), `\\`, `\`)
			}
			return ""
		}
		if tc.Prompt == "" {
			tc.Prompt = extractField("prompt")
		}
		if tc.Content == "" {
			tc.Content = extractField("content")
		}
		if tc.Query == "" {
			tc.Query = extractField("query")
		}
		if tc.Operation == "" {
			tc.Operation = extractField("operation")
		}
		if tc.Command == "" {
			tc.Command = extractField("command")
		}
		if tc.Code == "" {
			tc.Code = extractField("code")
		}
		if name == "execute_skill" && tc.Skill == "" {
			tc.Skill = extractField("skill")
		}
		return tc
	}

	// Ensure action is set correctly (unmarshal may overwrite it if the LLM included it)
	if tc.Action == "" {
		tc.Action = native.Function.Name
	}

	// Handle execute_skill: LLM may use "skill_name" key
	if tc.Action == "execute_skill" && tc.Skill == "" {
		for _, key := range []string{"skill_name", "name", "skill_name"} {
			if tc.Params != nil {
				if v, ok := tc.Params[key].(string); ok && v != "" {
					tc.Skill = v
					break
				}
			}
		}
	}

	return tc
}

// BuildNativeToolSchemas returns the full tool list: built-ins + registered skills + custom tools.
func BuildNativeToolSchemas(skillsDir string, manifest *tools.Manifest, ff ToolFeatureFlags, logger *slog.Logger) []openai.Tool {
	allTools := builtinToolSchemas(ff)

	// Add skills as sub-variants of execute_skill (informational context; already handled by execute_skill schema)
	if skills, err := tools.ListSkills(skillsDir); err == nil {
		for _, skill := range skills {
			allTools = append(allTools, tool(
				"skill__"+skill.Name,
				"(Skill) "+skill.Description+". Use execute_skill with skill='"+skill.Name+"'.",
				schema(map[string]interface{}{
					"skill_args": map[string]interface{}{
						"type":        "object",
						"description": "Arguments for this skill",
					},
				}),
			))
		}
	}

	// Add custom tools from manifest
	if manifest != nil {
		if entries, err := manifest.Load(); err == nil {
			for name, description := range entries {
				allTools = append(allTools, tool(
					"tool__"+name,
					"(Custom tool) "+description,
					schema(map[string]interface{}{
						"params": map[string]interface{}{
							"type":        "object",
							"description": "Parameters to pass to the tool",
						},
					}),
				))
			}
		}
	}

	// Inject _todo property into every tool schema so the agent can piggyback
	// a session-scoped task list on each tool call (optional, never required).
	todoProperty := map[string]interface{}{
		"type":        []string{"string", "null"},
		"description": "Optional: a compact task list for multi-step work. Format each task as '- [x] done task' or '- [ ] pending task', one per line. Update this on every tool call to track your progress through the current session's objectives. Leave empty or null if not needed.",
	}
	for i := range allTools {
		if allTools[i].Function == nil || allTools[i].Function.Parameters == nil {
			continue
		}
		params, ok := allTools[i].Function.Parameters.(map[string]interface{})
		if !ok {
			continue
		}
		props, ok := params["properties"].(map[string]interface{})
		if !ok {
			continue
		}
		props["_todo"] = todoProperty
	}

	if logger != nil {
		logger.Info("[NativeTools] Built tool schemas", "count", len(allTools))
	}

	return allTools
}
