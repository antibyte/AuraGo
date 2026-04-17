package agent

import openai "github.com/sashabaranov/go-openai"

func appendMemoryToolSchemas(tools []openai.Tool, ff ToolFeatureFlags) []openai.Tool {
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
				"Search across ALL memory sources at once: recent activity timeline, vector DB (long-term facts), knowledge graph (entities/relationships), journal (events/milestones), notes (tasks/todos), planner (structured tasks/appointments), core memory (permanent facts), and error patterns (learned failures). By default searches everything — use 'sources' only to narrow results.",
				schema(map[string]interface{}{
					"query": prop("string", "Natural language search query"),
					"sources": map[string]interface{}{
						"type":        "array",
						"description": "Memory sources to search. Default: all available. Options: activity, vector_db, knowledge_graph, journal, notes, planner, core_memory, error_patterns",
						"items":       map[string]interface{}{"type": "string", "enum": []string{"activity", "vector_db", "knowledge_graph", "journal", "notes", "planner", "core_memory", "error_patterns"}},
					},
					"limit": map[string]interface{}{
						"type":        "integer",
						"description": "Max results per source (default 5)",
					},
				}, "query"),
			),
			tool("context_memory",
				"Run a context-aware memory query across recent activity, journal, notes, planner, core memory, knowledge graph, and long-term memory. Prefer this when you need a time-scoped overview, connected context, or a multi-source picture of the last days.",
				schema(map[string]interface{}{
					"query":           prop("string", "Natural language search query"),
					"context_depth":   map[string]interface{}{"type": "string", "description": "How broad the contextual expansion should be", "enum": []string{"shallow", "normal", "deep"}},
					"time_range":      map[string]interface{}{"type": "string", "description": "Optional temporal window", "enum": []string{"all", "today", "last_week", "last_month"}},
					"include_related": prop("boolean", "Whether related entities/contexts should be expanded where possible"),
					"sources": map[string]interface{}{
						"type":        "array",
						"description": "Sources to include. Default: activity, journal, notes, planner, core, kg, ltm",
						"items":       map[string]interface{}{"type": "string", "enum": []string{"activity", "journal", "notes", "planner", "core", "kg", "ltm"}},
					},
				}, "query"),
			),
			tool("context_manager",
				"Manage the current conversation context window. Operations: 'status' (check token budget and messages count), 'compact' (summarize old messages into a single statement to free up tokens), 'drop' (remove a specific message by its index).",
				schema(map[string]interface{}{
					"operation": map[string]interface{}{
						"type":        "string",
						"description": "Context operation",
						"enum":        []string{"status", "compact", "drop"},
					},
					"index": prop("integer", "The 0-based index of the message to drop (used only for 'drop')"),
				}, "operation"),
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
				"Generate a reflection on memory activity: analyze patterns, detect contradictions, identify knowledge gaps, and suggest memory optimizations. Weekly reflections now include the recent activity timeline.",
				schema(map[string]interface{}{
					"scope": map[string]interface{}{
						"type":        "string",
						"description": "Scope of the reflection: session, day, week/recent, month/monthly, project, or all_time/full",
						"enum":        []string{"session", "day", "week", "recent", "month", "monthly", "project", "all_time", "full"},
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
				"enum":        []string{"list", "get", "create", "update", "delete", "attach", "detach"},
			},
			"id":            prop("string", "Cheat sheet ID (for get/update/delete/attach/detach). Can also be the name for 'get'."),
			"name":          prop("string", "Name of the cheat sheet (for create/update)"),
			"content":       prop("string", "Markdown content of the cheat sheet (for create/update/attach)"),
			"active":        map[string]interface{}{"type": "boolean", "description": "Whether the cheat sheet is active (for update)"},
			"filename":      prop("string", "Filename of the attachment to add (for attach). Only .txt and .md allowed."),
			"source":        prop("string", "Source of the attachment: 'upload' or 'knowledge' (for attach). Defaults to 'upload'."),
			"attachment_id": prop("string", "Attachment ID to remove (for detach)."),
		}, "operation"),
	))

	if ff.KnowledgeGraphEnabled {
		tools = append(tools, tool("knowledge_graph",
			"Manage a structured graph of entities and relationships. Use for tracking people, devices, services, projects, and their connections. Nightly auto-extraction also populates the graph from conversations.",
			schema(map[string]interface{}{
				"operation": map[string]interface{}{
					"type":        "string",
					"description": "Operation: 'add_node' (create entity), 'add_edge' (create relationship), 'delete_node' (remove entity+edges), 'delete_edge' (remove relationship), 'update_node' (modify node properties, merges with existing), 'update_edge' (modify edge relation/properties), 'get_node' (retrieve single node), 'get_neighbors' (get connected nodes and edges), 'subgraph' (get neighborhood subgraph around a node), 'search' (full-text search across nodes and edges), 'explore' (traverse graph randomly), 'suggest_relations' (suggest new relations)",
					"enum":        []string{"add_node", "add_edge", "delete_node", "delete_edge", "update_node", "update_edge", "get_node", "get_neighbors", "subgraph", "search", "explore", "suggest_relations"},
				},
				"id":           prop("string", "Node ID (for add_node, delete_node, update_node, get_node, get_neighbors, subgraph)"),
				"label":        prop("string", "Human-readable label for the node (for add_node, update_node)"),
				"source":       prop("string", "Source node ID (for add_edge, delete_edge, update_edge)"),
				"target":       prop("string", "Target node ID (for add_edge, delete_edge, update_edge)"),
				"relation":     prop("string", "Relationship type (e.g. 'owns', 'uses', 'manages', 'connected_to')"),
				"content":      prop("string", "Search query text (for search operation)"),
				"properties":   map[string]interface{}{"type": "object", "description": "Optional metadata properties for the node or edge"},
				"new_relation": prop("string", "New relation type for update_edge (optional, defaults to current relation)"),
				"depth":        prop("integer", "Depth for subgraph traversal (1-3, default 2)"),
				"limit":        prop("integer", "Max results for get_neighbors (default 20)"),
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

	tools = append(tools, tool("manage_plan",
		"Create, inspect, and update the active structured work plan for the current session. Use this for complex multi-step work that benefits from tracked tasks and visible progress.",
		schema(map[string]interface{}{
			"operation": map[string]interface{}{
				"type":        "string",
				"description": "Plan operation",
				"enum":        []string{"create", "list", "get", "update_task", "advance", "set_status", "set_blocker", "clear_blocker", "append_note", "attach_artifact", "split_task", "reorder_tasks", "archive_completed", "delete"},
			},
			"id":               prop("string", "Plan ID (required for get, update_task, set_status, append_note, delete)"),
			"title":            prop("string", "Plan title (required for create)"),
			"description":      prop("string", "Plan description"),
			"content":          prop("string", "User request or note content. For append_note this is the note text."),
			"reason":           prop("string", "Blocker or status reason"),
			"priority":         prop("integer", "Priority: 1=low, 2=medium (default), 3=high"),
			"include_archived": prop("boolean", "Include archived plans in list results."),
			"status": map[string]interface{}{
				"type":        "string",
				"description": "Plan or task status",
				"enum":        []string{"draft", "active", "paused", "blocked", "completed", "cancelled", "pending", "in_progress", "failed", "skipped"},
			},
			"task_id":       prop("string", "Task ID for update_task, set_blocker, clear_blocker, or split_task"),
			"result":        prop("string", "Task result summary for update_task"),
			"error":         prop("string", "Task error summary for update_task"),
			"label":         prop("string", "Artifact label for attach_artifact"),
			"artifact_type": prop("string", "Artifact type for attach_artifact, e.g. file, url, id, report"),
			"limit":         prop("integer", "Maximum plans to return for list"),
			"items": map[string]interface{}{
				"type":        "array",
				"description": "Plan tasks for create or split_task. For reorder_tasks, pass items with task_id in the desired final order.",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"task_id":             prop("string", "Task ID used by reorder_tasks to define the desired order"),
						"title":               prop("string", "Task title"),
						"description":         prop("string", "Optional task description"),
						"kind":                prop("string", "Task kind: task, tool, reasoning, verification, note"),
						"acceptance_criteria": prop("string", "Optional acceptance criteria for this task"),
						"owner":               prop("string", "Optional owner: agent, user, or external"),
						"parent_task_id":      prop("string", "Optional parent task ID for nested subtasks"),
						"tool_name":           prop("string", "Optional tool name if this task is tied to a specific AuraGo tool"),
						"tool_args": map[string]interface{}{
							"type":        "object",
							"description": "Optional suggested tool arguments for the task",
						},
						"depends_on": map[string]interface{}{
							"type":        "array",
							"description": "Optional dependencies as task indices (1-based) or task IDs",
							"items":       map[string]interface{}{},
						},
					},
					"required": []string{"title"},
				},
			},
		}, "operation"),
	))

	if ff.JournalEnabled {
		tools = append(tools, tool("manage_journal",
			"Add, list, search, or delete journal entries. The system already auto-creates entries for lightweight activity traces, tool errors, task completions, and daily summaries during nightly maintenance. Use this to manually add reflections, milestones, or other important events.",
			schema(map[string]interface{}{
				"operation": map[string]interface{}{
					"type":        "string",
					"description": "Journal operation",
					"enum":        []string{"add", "list", "search", "delete", "get_summary"},
				},
				"title":      prop("string", "Title of the journal entry (required for add)"),
				"content":    prop("string", "Detailed content of the journal entry"),
				"entry_type": prop("string", "Type of entry: activity, reflection, milestone, preference, task_completed, integration, learning, error_recovery, system_event"),
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
			"Create, list, update, delete, or run background automation tasks (missions) in the Mission Control system. Use this to schedule recurring work for the agent or define on-demand jobs. The 'history' operation retrieves past mission execution records with optional filters.",
			schema(map[string]interface{}{
				"operation": map[string]interface{}{
					"type":        "string",
					"description": "Mission operation",
					"enum":        []string{"add", "list", "update", "delete", "run", "history"},
				},
				"title":        prop("string", "Name of the mission (required for add)"),
				"command":      prop("string", "The task prompt that the agent will execute"),
				"cron_expr":    prop("string", "Optional cron expression for scheduling (e.g. '0 9 * * *' for daily at 9am)"),
				"priority":     prop("integer", "Priority: 1=low, 2=medium (default), 3=high"),
				"locked":       prop("boolean", "If true, the mission is locked and cannot be deleted until unlocked"),
				"id":           prop("string", "Mission ID (required for update/delete/run, optional filter for history)"),
				"limit":        prop("integer", "Number of history entries to return (default 10, for history operation)"),
				"result":       prop("string", "Filter history by result: 'success' or 'error' (for history operation)"),
				"trigger_type": prop("string", "Filter history by trigger type, e.g. 'manual', 'cron', 'webhook', 'email' (for history operation)"),
				"from":         prop("string", "Filter history from date (ISO 8601, e.g. '2025-01-01T00:00:00Z', for history operation)"),
				"to":           prop("string", "Filter history to date (ISO 8601, e.g. '2025-12-31T23:59:59Z', for history operation)"),
			}, "operation"),
		))
	}

	if ff.DaemonSkillsEnabled {
		tools = append(tools, tool("manage_daemon",
			"Manage long-running daemon skills. List running daemons, check status, start/stop individual daemons, re-enable auto-disabled daemons, or refresh the daemon list from disk.",
			schema(map[string]interface{}{
				"operation": map[string]interface{}{
					"type":        "string",
					"description": "Daemon management operation",
					"enum":        []string{"list", "status", "start", "stop", "reenable", "refresh"},
				},
				"skill_id": prop("string", "Skill ID of the daemon (required for status/start/stop/reenable)"),
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

	return tools
}
