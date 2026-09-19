package agent

// roleToolRestriction is shared by dispatch and discovery.
func roleToolRestriction(tc ToolCall, dc *DispatchContext) string {
	sessionID := dc.SessionID
	// Co-Agent blacklist: co-agents cannot access secrets,
	// mutate memory-like stores, or orchestrate additional autonomous work.
	isCoAgent := dc.IsCoAgent || isCoAgentSession(sessionID)
	if isCoAgent {
		switch tc.Action {
		case "manage_memory", "core_memory":
			if tc.Operation != "read" && tc.Operation != "query" && tc.Operation != "" {
				return `Tool Output: {"status": "policy_denied", "message": "Co-Agents cannot modify memory. Only read/query operations are allowed."}`
			}
		case "remember":
			return `Tool Output: {"status": "policy_denied", "message": "Co-Agents cannot store new facts or memories."}`
		case "manage_knowledge", "knowledge_graph":
			if tc.Operation != "query" && tc.Operation != "search" && tc.Operation != "get" && tc.Operation != "get_node" && tc.Operation != "get_neighbors" && tc.Operation != "subgraph" && tc.Operation != "" {
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
		case "cron_scheduler":
			return `Tool Output: {"status": "policy_denied", "message": "Co-Agents cannot manage cron jobs."}`
		case "manage_daemon":
			if tc.Operation != "list" && tc.Operation != "status" {
				return `Tool Output: {"status": "policy_denied", "message": "Co-Agents cannot control daemons. Only list and status are allowed."}`
			}
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
