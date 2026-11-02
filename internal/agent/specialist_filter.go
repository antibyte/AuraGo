package agent

import (
	"path/filepath"
	"strings"
)

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
	return checkSpecialistToolCallRestriction(role, ToolCall{Action: action, Operation: operation})
}

// checkSpecialistToolCallRestriction checks the actual execution capability,
// including script arguments, before the existing role-specific restrictions.
func checkSpecialistToolCallRestriction(role string, tc ToolCall) string {
	action, operation := tc.Action, toolCallSnapshotOperation(tc)
	if role == "writer" {
		return `Tool Output: {"status":"policy_denied","message":"Writer specialists have no runtime tools. Write from the supplied material."}`
	}
	switch action {
	case "execute_sudo":
		action = "execute_shell"
	case "execute_remote_shell":
		if denied := checkSpecialistToolRestriction(role, "remote_control", operation); denied != "" {
			return denied
		}
		action = "execute_shell"
	case "run_tool":
		action = "execute_python"
	case "execute_sandbox", "run_agent_skill_script":
		language := strings.ToLower(strings.TrimSpace(decodeSandboxExecutionArgs(tc).Language))
		if action == "run_agent_skill_script" {
			script := firstNonEmptyToolString(tc.FilePath, tc.Path, toolArgString(tc.Params, "script"), toolArgString(tc.Params, "path"))
			switch strings.ToLower(filepath.Ext(script)) {
			case ".py":
				language = "python"
			case ".sh", ".js":
				language = "shell"
			default:
				return `Tool Output: {"status":"policy_denied","message":"Specialist script execution requires a supported explicit script path."}`
			}
		} else if language == "" {
			language = "python" // Same default as dispatchShellWithContext.
		}
		if language == "python" {
			action = "execute_python"
		} else {
			// Other interpreters require the general shell/code capability;
			// an unknown language must never grant a Python-only role more access.
			action = "execute_shell"
		}
	}
	if role != "" && action == "treg_call" && operation != "read" {
		return `Tool Output: {"status":"policy_denied","message":"Specialists may only use explicitly approved read-class treg endpoints; ask the main agent for external mutations."}`
	}
	// Dedicated native names inherit the existing family restriction.
	switch {
	case action == "generate_image":
		action = "image_generation"
	case strings.HasPrefix(action, "homepage_"):
		action = "homepage"
	case strings.HasPrefix(action, "remote_control_"):
		action = "remote_control"
	}
	switch role {
	case "researcher":
		return checkResearcherRestriction(action)
	case "coder":
		return checkCoderRestriction(action)
	case "designer":
		return checkDesignerRestriction(action)
	case "security":
		return checkSecurityRestriction(action, operation)
	}
	return ""
}

// checkResearcherRestriction blocks tools not relevant for research.
// Allowed: execute_skill, api_request, query_memory, knowledge_graph (read), filesystem (read), execute_python
func checkResearcherRestriction(action string) string {
	switch action {
	case "execute_shell":
		return `Tool Output: {"status": "policy_denied", "message": "Researcher specialist cannot execute shell commands. Use execute_skill or execute_python for data processing."}`
	case "image_generation":
		return `Tool Output: {"status": "policy_denied", "message": "Researcher specialist cannot generate images."}`
	case "remote_control":
		return `Tool Output: {"status": "policy_denied", "message": "Researcher specialist cannot use remote control."}`
	case "homepage":
		return `Tool Output: {"status": "policy_denied", "message": "Researcher specialist cannot manage websites."}`
	}
	return ""
}

// checkCoderRestriction blocks tools not relevant for coding.
// Allowed: execute_shell, execute_python, filesystem, execute_skill, query_memory, knowledge_graph (read), api_request
func checkCoderRestriction(action string) string {
	switch action {
	case "image_generation":
		return `Tool Output: {"status": "policy_denied", "message": "Coder specialist cannot generate images."}`
	case "remote_control":
		return `Tool Output: {"status": "policy_denied", "message": "Coder specialist cannot use remote control."}`
	}
	return ""
}

// checkDesignerRestriction blocks operational tools not relevant for design.
// Designers can create assets and design guidance, but direct homepage project
// management/deployment stays with the main agent or coder specialist.
// Allowed: image_generation, filesystem, execute_skill, query_memory, knowledge_graph (read), api_request
func checkDesignerRestriction(action string) string {
	switch action {
	case "execute_shell":
		return `Tool Output: {"status": "policy_denied", "message": "Designer specialist cannot execute shell commands."}`
	case "execute_python":
		return `Tool Output: {"status": "policy_denied", "message": "Designer specialist cannot execute Python code."}`
	case "remote_control":
		return `Tool Output: {"status": "policy_denied", "message": "Designer specialist cannot use remote control."}`
	case "homepage":
		return `Tool Output: {"status": "policy_denied", "message": "Designer specialist cannot manage or deploy homepage projects directly. Provide design assets or guidance for the main agent or coder specialist to implement."}`
	}
	return ""
}

// checkSecurityRestriction blocks tools not relevant for security analysis.
// Allowed: execute_shell, execute_python, filesystem (read), execute_skill, query_memory, knowledge_graph (read), api_request
func checkSecurityRestriction(action, operation string) string {
	operation = strings.ToLower(strings.TrimSpace(operation))
	switch action {
	case "image_generation":
		return `Tool Output: {"status": "policy_denied", "message": "Security specialist cannot generate images."}`
	case "remote_control":
		return `Tool Output: {"status": "policy_denied", "message": "Security specialist cannot use remote control."}`
	case "filesystem", "filesystem_op":
		switch operation {
		case "read", "read_file", "list", "ls", "list_dir", "stat", "info", "exists", "search", "tree":
			return ""
		}
		return securityFilesystemDenied
	case "file_editor":
		return securityFilesystemDenied
	case "json_editor", "yaml_editor", "toml_editor", "xml_editor":
		switch operation {
		case "read", "get", "query", "validate", "keys":
			return ""
		}
		return securityFilesystemDenied
	}
	return ""
}

const securityFilesystemDenied = `Tool Output: {"status":"policy_denied","message":"Security specialist has read-only filesystem access for analysis."}`
