package agent

import "strings"

// extractSpecialistRole returns the specialist role from a session ID like "specialist-researcher-5".
// Returns "" for generic co-agents or non-specialist sessions.
func extractSpecialistRole(sessionID string) string {
	if !strings.HasPrefix(sessionID, "specialist-") {
		return ""
	}
	// Format: "specialist-<role>-<N>"
	rest := strings.TrimPrefix(sessionID, "specialist-")
	// Find the last dash to separate role from counter
	if idx := strings.LastIndex(rest, "-"); idx > 0 {
		return rest[:idx]
	}
	return rest
}

// checkSpecialistToolRestriction checks whether a specialist is allowed to use the given tool/operation.
// Returns an error message if blocked, or "" if allowed.
func checkSpecialistToolRestriction(role, action, operation string) string {
	switch role {
	case "researcher":
		return checkResearcherRestriction(action)
	case "coder":
		return checkCoderRestriction(action)
	case "designer":
		return checkDesignerRestriction(action)
	case "security":
		return checkSecurityRestriction(action, operation)
	case "writer":
		return checkWriterRestriction(action)
	}
	return ""
}

// checkResearcherRestriction blocks tools not relevant for research.
// Allowed: execute_skill, api_request, query_memory, knowledge_graph (read), filesystem (read), execute_python
func checkResearcherRestriction(action string) string {
	switch action {
	case "execute_shell":
		return `Tool Output: {"status": "error", "message": "Researcher specialist cannot execute shell commands. Use execute_skill or execute_python for data processing."}`
	case "image_generation":
		return `Tool Output: {"status": "error", "message": "Researcher specialist cannot generate images."}`
	case "remote_control":
		return `Tool Output: {"status": "error", "message": "Researcher specialist cannot use remote control."}`
	case "homepage":
		return `Tool Output: {"status": "error", "message": "Researcher specialist cannot manage websites."}`
	}
	return ""
}

// checkCoderRestriction blocks tools not relevant for coding.
// Allowed: execute_shell, execute_python, filesystem, execute_skill, query_memory, knowledge_graph (read), api_request
func checkCoderRestriction(action string) string {
	switch action {
	case "image_generation":
		return `Tool Output: {"status": "error", "message": "Coder specialist cannot generate images."}`
	case "remote_control":
		return `Tool Output: {"status": "error", "message": "Coder specialist cannot use remote control."}`
	}
	return ""
}

// checkDesignerRestriction blocks tools not relevant for design.
// Allowed: image_generation, filesystem, execute_skill, query_memory, knowledge_graph (read), api_request
func checkDesignerRestriction(action string) string {
	switch action {
	case "execute_shell":
		return `Tool Output: {"status": "error", "message": "Designer specialist cannot execute shell commands."}`
	case "execute_python":
		return `Tool Output: {"status": "error", "message": "Designer specialist cannot execute Python code."}`
	case "remote_control":
		return `Tool Output: {"status": "error", "message": "Designer specialist cannot use remote control."}`
	case "homepage":
		return `Tool Output: {"status": "error", "message": "Designer specialist cannot manage websites directly."}`
	}
	return ""
}

// checkSecurityRestriction blocks tools not relevant for security analysis.
// Allowed: execute_shell, execute_python, filesystem (read), execute_skill, query_memory, knowledge_graph (read), api_request
func checkSecurityRestriction(action, operation string) string {
	switch action {
	case "image_generation":
		return `Tool Output: {"status": "error", "message": "Security specialist cannot generate images."}`
	case "remote_control":
		return `Tool Output: {"status": "error", "message": "Security specialist cannot use remote control."}`
	case "filesystem":
		if operation == "write" || operation == "delete" || operation == "move" || operation == "copy" {
			return `Tool Output: {"status": "error", "message": "Security specialist has read-only filesystem access for analysis."}`
		}
	}
	return ""
}

// checkWriterRestriction blocks tools not relevant for writing.
// Allowed: query_memory, knowledge_graph (read), filesystem (read/write), execute_skill, api_request
func checkWriterRestriction(action string) string {
	switch action {
	case "execute_shell":
		return `Tool Output: {"status": "error", "message": "Writer specialist cannot execute shell commands."}`
	case "execute_python":
		return `Tool Output: {"status": "error", "message": "Writer specialist cannot execute Python code."}`
	case "image_generation":
		return `Tool Output: {"status": "error", "message": "Writer specialist cannot generate images."}`
	case "remote_control":
		return `Tool Output: {"status": "error", "message": "Writer specialist cannot use remote control."}`
	case "homepage":
		return `Tool Output: {"status": "error", "message": "Writer specialist cannot manage websites."}`
	}
	return ""
}
