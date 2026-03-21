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
	OneDriveEnabled         bool
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
	ContactsEnabled         bool
	JournalEnabled           bool
	MemoryAnalysisEnabled    bool
	DocumentCreatorEnabled   bool
	WebCaptureEnabled        bool
	NetworkPingEnabled       bool
	WebScraperEnabled        bool
	S3Enabled                bool
	NetworkScanEnabled       bool
	FormAutomationEnabled    bool
	UPnPScanEnabled          bool
	// FritzBox sub-feature flags
	FritzBoxSystemEnabled    bool
	FritzBoxNetworkEnabled   bool
	FritzBoxTelephonyEnabled bool
	FritzBoxSmartHomeEnabled bool
	FritzBoxStorageEnabled   bool
	FritzBoxTVEnabled        bool
	// Telnyx integration flags
	TelnyxSMSEnabled  bool
	TelnyxCallEnabled bool
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
			"Retrieve current system resource usage: CPU, memory, disk, running processes, host info, temperatures, per-interface network stats, active connections, or per-disk I/O counters.",
			schema(map[string]interface{}{
				"target": map[string]interface{}{
					"type":        "string",
					"description": "Metrics category to retrieve",
					"enum":        []string{"all", "cpu", "memory", "disk", "processes", "host", "sensors", "network_detail", "connections", "disk_io"},
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
		tool("send_audio",
			"Send an audio file to the user. Shown with an inline audio player in the Web UI (play/pause, progress bar, speed control). Provide a local workspace path or a direct HTTPS URL to an audio file.",
			schema(map[string]interface{}{
				"path":  prop("string", "Local file path within the workspace (e.g. 'output.mp3') or a full HTTPS URL to an audio file (MP3, WAV, OGG, FLAC, M4A)"),
				"title": prop("string", "Optional title shown above the audio player"),
			}, "path"),
		),
		tool("send_document",
			"Send a document to the user. Shown with Open and Download buttons in the Web UI. PDF files can be viewed inline in the browser. Provide a local workspace path or a direct HTTPS URL.",
			schema(map[string]interface{}{
				"path":  prop("string", "Local file path within the workspace or a full HTTPS URL to a document (PDF, DOCX, XLSX, PPTX, TXT, MD, CSV)"),
				"title": prop("string", "Optional title shown with the document card"),
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
				"Manage permanently stored core memory facts. Use 'add' to store a new fact, 'update' to correct an existing fact by ID, 'delete' to remove a fact by ID, 'remove' to remove a fact by text match, 'list' to read all stored facts.",
				schema(map[string]interface{}{
					"operation": map[string]interface{}{
						"type":        "string",
						"description": "Operation: 'add' (store new fact), 'update' (edit by id), 'delete' (remove by id), 'remove' (remove by text match), 'list' (read all)",
						"enum":        []string{"add", "update", "delete", "remove", "list"},
					},
					"fact": prop("string", "The factual statement to add or remove. Required for 'add' and 'remove'."),
					"id":   prop("string", "Numeric ID of the fact to update or delete. Required for 'update' and 'delete'. IDs are shown in brackets when listing facts."),
				}, "operation"),
			),
			tool("query_memory",
				"Search across ALL memory sources at once: vector DB (long-term facts), knowledge graph (entities/relationships), journal (events/milestones), notes (tasks/todos), core memory (permanent facts), and error patterns (learned failures). By default searches everything — use 'sources' only to narrow results.",
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

			// remember — simplified single-entry-point for storing any kind of information
			tool("remember",
				"Store information without worrying about which memory system to use. Automatically routes to the right place: core memory (facts/preferences), journal (events/milestones), notes (tasks/todos), or knowledge graph (relationships). Use 'category' to override auto-classification.",
				schema(map[string]interface{}{
					"content":    prop("string", "The information to remember (required)"),
					"category":   prop("string", "Optional routing hint: 'fact' (core memory), 'event' (journal), 'task' (note/todo), 'relationship' (knowledge graph). If omitted, auto-classified from content."),
					"title":      prop("string", "Optional title (used for journal entries and notes)"),
					"source":     prop("string", "Source entity (only for relationship: source -[relation]-> target)"),
					"target":     prop("string", "Target entity (only for relationship)"),
					"relation":   prop("string", "Relationship type (only for relationship, e.g. 'owns', 'uses')"),
					"entry_type": prop("string", "Journal entry type when category=event (reflection, milestone, learning, etc.)"),
					"tags":       prop("string", "Comma-separated tags (for journal entries)"),
					"importance": prop("integer", "Importance 1-4 (for journal entries, default 2)"),
				}, "content"),
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
			"Manage a structured graph of entities and relationships. Use for tracking people, devices, services, projects, and their connections. Nightly auto-extraction also populates the graph from conversations.",
			schema(map[string]interface{}{
				"operation": map[string]interface{}{
					"type":        "string",
					"description": "Operation: 'add_node' (create/update entity), 'add_edge' (create relationship), 'delete_node' (remove entity+edges), 'delete_edge' (remove relationship), 'search' (full-text search across nodes and edges)",
					"enum":        []string{"add_node", "add_edge", "delete_node", "delete_edge", "search"},
				},
				"id":         prop("string", "Node ID for add_node/delete_node (e.g. 'app_db', 'server_prod')"),
				"label":      prop("string", "Human-readable label for the node (for add_node)"),
				"source":     prop("string", "Source node ID (for add_edge/delete_edge)"),
				"target":     prop("string", "Target node ID (for add_edge/delete_edge)"),
				"relation":   prop("string", "Relationship type (e.g. 'owns', 'uses', 'manages', 'connected_to')"),
				"content":    prop("string", "Search query text (for search operation)"),
				"properties": map[string]interface{}{"type": "object", "description": "Optional metadata properties for the node or edge"},
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
			"Add, list, search, or delete journal entries. The system already auto-creates entries for tool errors, task completions, and daily summaries during nightly maintenance. Use this to manually add reflections, milestones, or other important events.",
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

	if ff.ContactsEnabled {
		tools = append(tools, tool("address_book",
			"Manage the address book / contacts. Search, list, add, update, and delete contacts with name, email, phone, mobile, address, and relationship.",
			schema(map[string]interface{}{
				"operation": map[string]interface{}{
					"type":        "string",
					"description": "Operation to perform",
					"enum":        []string{"list", "search", "add", "update", "delete"},
				},
				"id":           prop("string", "Contact ID (required for update/delete)"),
				"name":         prop("string", "Full name of the contact"),
				"email":        prop("string", "Email address"),
				"phone":        prop("string", "Phone number"),
				"mobile":       prop("string", "Mobile phone number"),
				"address":      prop("string", "Postal address"),
				"relationship": prop("string", "Relationship (e.g. friend, colleague, family, client)"),
				"query":        prop("string", "Search query for search operation"),
			}, "operation"),
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

	// Archive (always enabled — uses stdlib only)
	tools = append(tools, tool("archive",
		"Create, extract, or list ZIP and TAR.GZ archives. "+
			"Operations: 'create' (build archive from files/directory), 'extract' (unpack to target directory), 'list' (show contents without extracting). "+
			"Supports ZIP and TAR.GZ/TGZ formats. Path traversal protection is enforced on extraction.",
		schema(map[string]interface{}{
			"operation": map[string]interface{}{
				"type":        "string",
				"description": "Archive operation to perform",
				"enum":        []string{"create", "extract", "list"},
			},
			"path":         prop("string", "Path to the archive file (target for create, source for extract/list)"),
			"destination":  prop("string", "Target directory: extraction destination (extract) or source directory (create)"),
			"source_files": prop("string", "JSON array of specific file paths to include (create only; alternative to destination)"),
			"format": map[string]interface{}{
				"type":        "string",
				"description": "Archive format (create only; extract/list auto-detect from extension)",
				"enum":        []string{"zip", "tar.gz"},
			},
		}, "operation", "path"),
	))

	// DNS Lookup (always enabled — uses stdlib only)
	tools = append(tools, tool("dns_lookup",
		"Perform DNS record lookups for a hostname. "+
			"Returns A, AAAA, MX, NS, TXT, CNAME, or PTR records. "+
			"Use record_type 'all' (default) to query all common record types at once.",
		schema(map[string]interface{}{
			"host": prop("string", "Hostname or domain to look up (e.g. 'example.com')"),
			"record_type": map[string]interface{}{
				"type":        "string",
				"description": "DNS record type to query (default: all)",
				"enum":        []string{"all", "A", "AAAA", "MX", "NS", "TXT", "CNAME", "PTR"},
			},
		}, "host"),
	))

	if ff.NetworkPingEnabled {
		// Port Scanner (gated with network_ping — same permission scope)
		tools = append(tools, tool("port_scanner",
			"Scan TCP ports on a target host using connect probes. "+
				"Returns open ports with service names and banners. "+
				"Port range can be: a single port ('80'), comma-separated ('80,443,8080'), a range ('1-1024'), or 'common' for top well-known ports. "+
				"Maximum 1024 ports per scan.",
			schema(map[string]interface{}{
				"host":       prop("string", "Hostname or IP address to scan"),
				"port_range": prop("string", "Ports to scan: single port, comma-separated, range (e.g. '1-1024'), or 'common' (default: common)"),
				"timeout_ms": map[string]interface{}{"type": "integer", "description": "Per-port timeout in milliseconds (100–5000, default: 1000)"},
			}, "host"),
		))
	}

	if ff.WebScraperEnabled {
		// Site Crawler (gated with web_scraper — same permission scope)
		tools = append(tools, tool("site_crawler",
			"Crawl a website starting from a URL, following links to discover and extract content from multiple pages. "+
				"Respects robots.txt and domain restrictions. Returns page titles and text previews. "+
				"Use for mapping site structure, finding content across pages, or extracting data from multi-page sites.",
			schema(map[string]interface{}{
				"url":             prop("string", "Starting URL to crawl (http or https)"),
				"max_depth":       map[string]interface{}{"type": "integer", "description": "Maximum link depth to follow (1–5, default: 2)"},
				"max_pages":       map[string]interface{}{"type": "integer", "description": "Maximum pages to crawl (1–100, default: 20)"},
				"allowed_domains": prop("string", "Comma-separated domain whitelist (default: auto-detect from start URL)"),
				"selector":        prop("string", "Optional CSS selector to extract specific content from each page"),
			}, "url"),
		))
	}

	if ff.S3Enabled {
		tools = append(tools, tool("s3_storage",
			"Manage objects in S3-compatible storage (AWS S3, MinIO, Wasabi, Backblaze B2). "+
				"Operations: list_buckets, list_objects (with optional prefix filter), upload (local file → S3), "+
				"download (S3 → local file), delete, copy (within or across buckets), move.",
			schema(map[string]interface{}{
				"operation": map[string]interface{}{
					"type":        "string",
					"description": "S3 operation to perform",
					"enum":        []string{"list_buckets", "list_objects", "upload", "download", "delete", "copy", "move"},
				},
				"bucket":             prop("string", "S3 bucket name (uses default if not specified)"),
				"key":                prop("string", "S3 object key (path within the bucket)"),
				"local_path":         prop("string", "Local file path for upload source or download destination"),
				"prefix":             prop("string", "Key prefix filter for list_objects (e.g. 'backups/2025/')"),
				"destination_bucket": prop("string", "Target bucket for copy/move (defaults to source bucket)"),
				"destination_key":    prop("string", "Target key for copy/move"),
			}, "operation"),
		))
	}

	// PDF Operations (always available, filesystem write gated)
	tools = append(tools, tool("pdf_operations",
		"Manipulate PDF files: merge multiple PDFs, split into pages, add text watermarks, "+
			"compress/optimize file size, encrypt/decrypt with password, read metadata and page count. "+
			"Uses local processing (no external service needed).",
		schema(map[string]interface{}{
			"operation": map[string]interface{}{
				"type":        "string",
				"description": "PDF operation to perform",
				"enum":        []string{"merge", "split", "watermark", "compress", "encrypt", "decrypt", "metadata", "page_count"},
			},
			"file_path":      prop("string", "Input PDF file path (required for all except merge)"),
			"output_file":    prop("string", "Output file/directory path (auto-generated if omitted)"),
			"source_files":   prop("string", "JSON array of PDF file paths for merge (e.g. '[\"a.pdf\",\"b.pdf\"]')"),
			"pages":          prop("string", "Page numbers for split (comma-separated, e.g. '3,5,8')"),
			"watermark_text": prop("string", "Text to use as watermark (diagonal, semi-transparent)"),
			"password":       prop("string", "Password for encrypt/decrypt operations"),
		}, "operation"),
	))

	// Image Processing (always available, filesystem write gated)
	tools = append(tools, tool("image_processing",
		"Process images: resize (with aspect ratio), convert between formats (PNG, JPEG, GIF, BMP, TIFF), "+
			"compress/optimize quality, crop to rectangle, rotate (90°/180°/270°), get image info.",
		schema(map[string]interface{}{
			"operation": map[string]interface{}{
				"type":        "string",
				"description": "Image operation to perform",
				"enum":        []string{"resize", "convert", "compress", "crop", "rotate", "info"},
			},
			"file_path":     prop("string", "Input image file path"),
			"output_file":   prop("string", "Output file path (auto-generated if omitted)"),
			"output_format": prop("string", "Target format: png, jpeg, gif, bmp, tiff (for convert)"),
			"width":         map[string]interface{}{"type": "integer", "description": "Target width in pixels (for resize)"},
			"height":        map[string]interface{}{"type": "integer", "description": "Target height in pixels (for resize)"},
			"quality_pct":   map[string]interface{}{"type": "integer", "description": "Quality percentage 1-100 (for compress/resize, default: 85)"},
			"crop_x":        map[string]interface{}{"type": "integer", "description": "Crop start X coordinate"},
			"crop_y":        map[string]interface{}{"type": "integer", "description": "Crop start Y coordinate"},
			"crop_width":    map[string]interface{}{"type": "integer", "description": "Crop width in pixels"},
			"crop_height":   map[string]interface{}{"type": "integer", "description": "Crop height in pixels"},
			"angle":         map[string]interface{}{"type": "integer", "description": "Rotation angle: 90, 180, or 270 degrees"},
		}, "operation", "file_path"),
	))

	// WHOIS Lookup (always available, network read-only)
	tools = append(tools, tool("whois_lookup",
		"Look up WHOIS registration information for a domain name. "+
			"Returns registrar, creation/expiry dates, name servers, domain status, and DNSSEC info. "+
			"Supports 30+ TLDs with automatic WHOIS server selection.",
		schema(map[string]interface{}{
			"domain":      prop("string", "Domain name to look up (e.g. 'example.com')"),
			"include_raw": map[string]interface{}{"type": "boolean", "description": "Include raw WHOIS response text (default: false)"},
		}, "domain"),
	))

	// Site Monitor (gated by WebScraperEnabled)
	if ff.WebScraperEnabled {
		tools = append(tools, tool("site_monitor",
			"Monitor websites for content changes. Add URLs to watch, check for changes manually or via cron, "+
				"and view change history. Uses content hashing to detect modifications. "+
				"Operations: add_monitor, remove_monitor, list_monitors, check_now, check_all, get_history.",
			schema(map[string]interface{}{
				"operation": map[string]interface{}{
					"type":        "string",
					"description": "Monitoring operation to perform",
					"enum":        []string{"add_monitor", "remove_monitor", "list_monitors", "check_now", "check_all", "get_history"},
				},
				"url":        prop("string", "URL to monitor (for add_monitor or check_now)"),
				"monitor_id": prop("string", "Monitor ID (for remove_monitor, check_now, get_history)"),
				"selector":   prop("string", "Optional CSS selector to focus monitoring on specific content"),
				"interval":   prop("string", "Suggested check interval description (e.g. 'every 6 hours')"),
				"limit":      map[string]interface{}{"type": "integer", "description": "Max history entries to return (default: 20, max: 100)"},
			}, "operation"),
		))
	}

	// mdns_scan / Network Scanner (gated by NetworkScanEnabled)
	if ff.NetworkScanEnabled {
		tools = append(tools, tool("mdns_scan",
			"Scan the local network for devices and services advertised via mDNS (Multicast DNS / Bonjour / ZeroConf). "+
				"Discovers Raspberry Pis, NAS devices, Apple devices, Chromecasts, printers, and any service "+
				"that announces itself via mDNS. Specify a service type (e.g. '_http._tcp', '_ssh._tcp', '_smb._tcp') "+
				"or use the default '_services._dns-sd._udp' to find all announced service types. "+
				"Set auto_register=true to bulk-import all discovered devices into the device registry in a single call.",
			schema(map[string]interface{}{
				"service_type":       prop("string", "mDNS service type to scan for (e.g. '_http._tcp', '_ssh._tcp', '_smb._tcp'). Default: '_services._dns-sd._udp' (discover all service types)"),
				"timeout":            map[string]interface{}{"type": "integer", "description": "Scan timeout in seconds (1–30, default: 5)"},
				"auto_register":      map[string]interface{}{"type": "boolean", "description": "If true, automatically register all discovered devices into the device inventory in one call. Saves many token-costly individual manage_inventory calls."},
				"register_type":      prop("string", "Device type to assign when auto_register is true (e.g. 'iot', 'printer', 'server'). Defaults to 'mdns-device'."),
				"register_tags":      map[string]interface{}{"type": "array", "items": map[string]string{"type": "string"}, "description": "Tags to assign to auto-registered devices (e.g. ['mdns', 'home-lab'])."},
				"overwrite_existing": map[string]interface{}{"type": "boolean", "description": "If true, update an existing device record when the name matches. Default: false (skip duplicates)."},
			}),
		))
	}

	// form_automation (gated by FormAutomationEnabled + WebCaptureEnabled as they share the headless browser)
	if ff.FormAutomationEnabled && ff.WebCaptureEnabled {
		tools = append(tools, tool("form_automation",
			"Interact with web forms using a headless Chromium browser. "+
				"Operations: 'get_fields' lists all form inputs on a page; "+
				"'fill_submit' fills form fields (by CSS selector) and submits; "+
				"'click' clicks any element by CSS selector. "+
				"Optionally saves a screenshot of the result page.",
			schema(map[string]interface{}{
				"operation": map[string]interface{}{
					"type":        "string",
					"description": "Form operation to perform",
					"enum":        []string{"get_fields", "fill_submit", "click"},
				},
				"url":            prop("string", "Page URL to load (http or https)"),
				"fields":         prop("string", "JSON object mapping CSS selector → value for fill_submit (e.g. '{\"#user\":\"alice\",\"#pass\":\"secret\"}')"),
				"selector":       prop("string", "CSS selector for click operation, or submit button for fill_submit (default: first submit button)"),
				"screenshot_dir": prop("string", "Directory to save post-action screenshot (optional; default: no screenshot)"),
			}, "operation", "url"),
		))
	}

	// upnp_scan (gated by UPnPScanEnabled)
	if ff.FritzBoxSystemEnabled {
		tools = append(tools, tool("fritzbox_system",
			"Fritz!Box system operations: get device info (model, firmware, uptime, serial), read system log, reboot (requires readonly=false).",
			schema(map[string]interface{}{
				"operation": map[string]interface{}{
					"type":        "string",
					"description": "Operation to perform",
					"enum":        []string{"get_info", "get_log", "reboot"},
				},
			}, "operation"),
		))
	}
	if ff.FritzBoxNetworkEnabled {
		tools = append(tools, tool("fritzbox_network",
			"Fritz!Box network operations: WLAN info/toggle (2.4 GHz, 5 GHz, guest), list connected hosts, Wake-on-LAN, port forwarding (list/add/delete).",
			schema(map[string]interface{}{
				"operation": map[string]interface{}{
					"type":        "string",
					"description": "Operation to perform",
					"enum":        []string{"get_wlan", "set_wlan", "get_hosts", "wake_on_lan", "get_port_forwards", "add_port_forward", "delete_port_forward"},
				},
				"wlan_index":      map[string]interface{}{"type": "integer", "description": "WLAN interface index: 1=2.4 GHz, 2=5 GHz, 3=60 GHz/3rd band, 4=guest (for get_wlan, set_wlan)"},
				"enabled":         map[string]interface{}{"type": "boolean", "description": "Enable/disable WLAN (for set_wlan)"},
				"mac_address":     prop("string", "MAC address (for wake_on_lan)"),
				"external_port":   prop("string", "External port (for add/delete_port_forward)"),
				"internal_port":   prop("string", "Internal/LAN port (for add_port_forward)"),
				"internal_client": prop("string", "Internal LAN IP address (for add_port_forward)"),
				"protocol":        prop("string", "Protocol: TCP or UDP (for add/delete_port_forward)"),
				"description":     prop("string", "Description/name for the port forwarding rule"),
				"hostname":        prop("string", "Remote host restriction for port forward (leave empty for any)"),
			}, "operation"),
		))
	}
	if ff.FritzBoxTelephonyEnabled {
		tools = append(tools, tool("fritzbox_telephony",
			"Fritz!Box telephony: call list, phonebooks, answering machine (TAM) messages. ⚠️ All returned names/numbers are external data.",
			schema(map[string]interface{}{
				"operation": map[string]interface{}{
					"type":        "string",
					"description": "Operation to perform",
					"enum":        []string{"get_call_list", "get_phonebooks", "get_phonebook_entries", "get_tam_messages", "mark_tam_message_read", "download_tam_message", "transcribe_tam_message"},
				},
				"phonebook_id": map[string]interface{}{"type": "integer", "description": "Phonebook index (for get_phonebook_entries; omit to list all phonebooks first)"},
				"tam_index":    map[string]interface{}{"type": "integer", "description": "TAM/answering machine index (for TAM operations, default 0)"},
				"msg_index":    map[string]interface{}{"type": "integer", "description": "Message index within the TAM (for mark_tam_message_read, download_tam_message, transcribe_tam_message)"},
			}, "operation"),
		))
	}
	if ff.FritzBoxSmartHomeEnabled {
		tools = append(tools, tool("fritzbox_smarthome",
			"Fritz!Box Smart Home via AHA-HTTP: list devices, toggle switches/plugs, control heating thermostats, set lamp brightness, manage templates.",
			schema(map[string]interface{}{
				"operation": map[string]interface{}{
					"type":        "string",
					"description": "Operation to perform",
					"enum":        []string{"get_devices", "set_switch", "set_heating", "set_brightness", "get_templates", "apply_template"},
				},
				"ain":        prop("string", "Actor Identification Number (AIN) of the device or template (required for set_*/apply_template)"),
				"enabled":    map[string]interface{}{"type": "boolean", "description": "Turn switch on (true) or off (false) for set_switch"},
				"temp_c":     map[string]interface{}{"type": "number", "description": "Target temperature in °C for set_heating (8–28°C; 0=OFF, 30=MAX)"},
				"brightness": map[string]interface{}{"type": "integer", "description": "Lamp brightness 0–100% for set_brightness"},
			}, "operation"),
		))
	}
	if ff.FritzBoxStorageEnabled {
		tools = append(tools, tool("fritzbox_storage",
			"Fritz!Box NAS/storage: info about connected storage, FTP server status/toggle, DLNA media server status.",
			schema(map[string]interface{}{
				"operation": map[string]interface{}{
					"type":        "string",
					"description": "Operation to perform",
					"enum":        []string{"get_storage_info", "get_ftp_status", "set_ftp", "get_media_server_status"},
				},
				"enabled": map[string]interface{}{"type": "boolean", "description": "Enable/disable FTP server (for set_ftp)"},
			}, "operation"),
		))
	}
	if ff.FritzBoxTVEnabled {
		tools = append(tools, tool("fritzbox_tv",
			"Fritz!Box DVB-C TV (cable models only): list channels with stream URLs.",
			schema(map[string]interface{}{
				"operation": map[string]interface{}{
					"type":        "string",
					"description": "Operation to perform",
					"enum":        []string{"get_channels"},
				},
			}, "operation"),
		))
	}

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
				"register_tags":      map[string]interface{}{"type": "array", "items": map[string]string{"type": "string"}, "description": "Tags to assign to auto-registered devices."},
				"overwrite_existing": map[string]interface{}{"type": "boolean", "description": "If true, update an existing device record when the name matches. Default: false (skip duplicates)."},
			}),
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
		"description": "Session task list. '- [x] done' / '- [ ] pending', one per line. Update each call. Null if unused.",
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
