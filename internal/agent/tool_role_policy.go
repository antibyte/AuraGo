package agent

import "strings"

// roleToolRestriction is shared by dispatch and discovery.
func roleToolRestriction(tc ToolCall, dc *DispatchContext) string {
	// Discovery also calls this before transport preparation.
	tc.Operation = toolCallSnapshotOperation(tc)
	// Model-written maintenance is limited to inspecting the stores maintained
	// by deterministic nightly phases. Operator action grants cannot widen it.
	if dc.IsMaintenance {
		allowed := false
		switch tc.Action {
		case "discover_tools", "read_tool_manual", "query_memory", "context_memory", "system_metrics", "get_system_metrics":
			allowed = true
		case "manage_memory", "core_memory", "manage_knowledge", "knowledge_graph", "manage_notes", "manage_journal", "manage_todos", "manage_appointments", "manage_cheatsheets", "manage_plan":
			allowed = readActionOperation(tc.Operation) || tc.Operation == "view_profile" || tc.Operation == "get_summary" || tc.Operation == "subgraph"
		}
		if !allowed {
			return `Tool Output: {"status":"policy_denied","code":"maintenance_scope_denied","message":"Maintenance may inspect its stores but cannot execute unrelated or mutating tools."}`
		}
	}
	sessionID := dc.SessionID
	// Co-Agent blacklist: co-agents cannot access secrets,
	// mutate memory-like stores, or orchestrate additional autonomous work.
	isCoAgent := dc.IsCoAgent || isCoAgentSession(sessionID)
	if isCoAgent {
		switch tc.Action {
		case "correct_memory", "remember_learning":
			return `Tool Output: {"status":"policy_denied","message":"Only the main agent can correct memory."}`
		case "manage_memory", "core_memory":
			if !oneOf(tc.Operation, "read", "query", "list", "get", "view_profile") {
				return `Tool Output: {"status": "policy_denied", "message": "Co-Agents cannot modify memory. Only read/query operations are allowed."}`
			}
		case "remember":
			return `Tool Output: {"status": "policy_denied", "message": "Co-Agents cannot store new facts or memories."}`
		case "manage_knowledge", "knowledge_graph":
			if !oneOf(tc.Operation, "query", "search", "get", "get_node", "get_neighbors", "subgraph") {
				return `Tool Output: {"status": "policy_denied", "message": "Co-Agents cannot modify the knowledge graph. Only read operations are allowed."}`
			}
		case "get_secret", "secrets_vault":
			return `Tool Output: {"status": "policy_denied", "message": "Co-Agents cannot access the secrets vault."}`
		case "manage_notes", "notes", "todo":
			if tc.Operation != "list" {
				return `Tool Output: {"status": "policy_denied", "message": "Co-Agents cannot modify notes. Only 'list' is allowed."}`
			}
		case "manage_journal", "journal":
			if tc.Operation != "list" && tc.Operation != "search" && tc.Operation != "get_summary" {
				return `Tool Output: {"status": "policy_denied", "message": "Co-Agents cannot modify journal entries. Only list, search, and get_summary are allowed."}`
			}
		case "manage_plan":
			if tc.Operation != "list" && tc.Operation != "get" {
				return `Tool Output: {"status": "policy_denied", "message": "Co-Agents cannot modify plans. Only list and get are allowed."}`
			}
		case "manage_missions":
			if !isCoAgentMissionReadOperation(tc.Operation) {
				return `Tool Output: {"status": "policy_denied", "message": "Co-Agents cannot modify or run missions. Only list, get, and status are allowed."}`
			}
		case "manage_appointments":
			if tc.Operation != "list" && tc.Operation != "get" {
				return `Tool Output: {"status": "policy_denied", "message": "Co-Agents cannot modify appointments. Only list and get are allowed."}`
			}
		case "manage_todos":
			if tc.Operation != "list" && tc.Operation != "get" {
				return `Tool Output: {"status": "policy_denied", "message": "Co-Agents cannot modify todos. Only list and get are allowed."}`
			}
		case "co_agent", "co_agents":
			return `Tool Output: {"status": "policy_denied", "message": "Co-Agents cannot spawn sub-agents."}`
		case "virtual_workspace", "virtual_browser":
			return `Tool Output: {"status": "policy_denied", "message": "Agent Workspaces are controlled only by the AuraGo main agent."}`
		case "follow_up":
			return `Tool Output: {"status": "policy_denied", "message": "Co-Agents cannot schedule follow-ups."}`
		case "wait_for_event":
			return `Tool Output: {"status": "policy_denied", "message": "Co-Agents cannot schedule wait events."}`
		case "question_user", "request_vault_secret":
			return `Tool Output: {"status": "policy_denied", "message": "Co-Agents cannot ask the user questions."}`
		case "cron_scheduler", "manage_schedule", "schedule_cron":
			return `Tool Output: {"status": "policy_denied", "message": "Co-Agents cannot manage cron jobs."}`
		case "manage_webhooks":
			return `Tool Output: {"status": "policy_denied", "message": "Co-Agents cannot manage webhooks."}`
		case "save_tool", "create_skill_from_template", "set_skill_documentation":
			return `Tool Output: {"status": "policy_denied", "message": "Co-Agents cannot create or change tools and skills. Only the main agent persists code that later runs under its authority."}`
		case "manage_daemon":
			if tc.Operation != "list" && tc.Operation != "status" {
				return `Tool Output: {"status": "policy_denied", "message": "Co-Agents cannot control daemons. Only list and status are allowed."}`
			}
		}
		// A remote peer's task is not the owner's request, so an A2A run may read
		// the workspace but not change it. Local co-agents keep their file tools.
		if isA2ARun(dc) && a2aFilesystemWrite(tc) {
			return `Tool Output: {"status": "policy_denied", "message": "A2A requests cannot change files. Only read operations are allowed."}`
		}
	}

	// Specialist-specific tool restrictions (additional to the generic co-agent blacklist)
	specialistRole := dc.CoAgentSpecialist
	if specialistRole == "" {
		specialistRole = extractSpecialistRole(sessionID)
	}
	if specialistRole != "" {
		if blocked := checkSpecialistToolRestriction(specialistRole, tc.Action, toolCallSnapshotOperation(tc)); blocked != "" {
			return blocked
		}
	}

	return ""
}

// isA2ARun reports a run that serves an Agent-to-Agent request. It is a co-agent
// run (IsCoAgent, session prefix "a2a-") whose message source is "a2a".
func isA2ARun(dc *DispatchContext) bool {
	return dc != nil && (dc.MessageSource == "a2a" || strings.HasPrefix(dc.SessionID, "a2a-"))
}

// a2aFilesystemWrite reports a call to the filesystem family that is not a read.
// Archives, PDFs, images and media conversion are held back by the A2A run's own
// configuration copy (agent.allow_filesystem_write is false there); this list is
// the policy-level answer for the tools whose schema changes with that flag.
func a2aFilesystemWrite(tc ToolCall) bool {
	switch tc.Action {
	case "filesystem", "filesystem_op":
		return !oneOf(tc.Operation, "read_file", "read", "list_dir", "list", "ls", "stat")
	case "file_editor":
		return true // Every file_editor operation edits the file.
	case "json_editor", "yaml_editor", "toml_editor", "xml_editor":
		return !oneOf(tc.Operation, "get", "keys", "validate", "format")
	}
	return false
}
