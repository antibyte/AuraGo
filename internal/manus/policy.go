package manus

import (
	"fmt"
	"strings"
)

// Policy is the local default-deny authorization layer for Manus operations.
type Policy struct {
	ReadOnly           bool
	AllowCreateTasks   bool
	AllowSendMessages  bool
	AllowStopTasks     bool
	AllowFileUploads   bool
	AllowFileDownloads bool

	AllowedProjectIDs   []string
	AllowedConnectorIDs []string
	AllowedSkillIDs     []string
}

// Authorize checks an agent-visible operation before any network or local write.
func (p Policy) Authorize(operation string) error {
	operation = strings.ToLower(strings.TrimSpace(operation))
	switch operation {
	case "capabilities", "get_credits", "list_projects", "list_connectors", "list_skills",
		"list_tracked_tasks", "get_task", "list_messages", "wait_for_task":
		return nil
	case "create_task":
		return p.authorizeMutation(operation, p.AllowCreateTasks)
	case "send_message":
		return p.authorizeMutation(operation, p.AllowSendMessages)
	case "stop_task":
		return p.authorizeMutation(operation, p.AllowStopTasks)
	case "download_attachments":
		return p.authorizeMutation(operation, p.AllowFileDownloads)
	default:
		return fmt.Errorf("unsupported Manus operation %q", operation)
	}
}

// AuthorizeUpload checks the separate upload permission.
func (p Policy) AuthorizeUpload() error {
	return p.authorizeMutation("file_upload", p.AllowFileUploads)
}

func (p Policy) authorizeMutation(operation string, allowed bool) error {
	if p.ReadOnly {
		return fmt.Errorf("Manus operation %q is disabled by read-only mode", operation)
	}
	if !allowed {
		return fmt.Errorf("Manus operation %q is not enabled", operation)
	}
	return nil
}

// ValidateResources ensures every explicitly requested remote capability was approved.
func (p Policy) ValidateResources(projectID string, connectorIDs, enabledSkillIDs, forcedSkillIDs []string) error {
	if projectID = strings.TrimSpace(projectID); projectID != "" && !containsID(p.AllowedProjectIDs, projectID) {
		return fmt.Errorf("Manus project %q is not allowlisted", projectID)
	}
	for _, id := range connectorIDs {
		if !containsID(p.AllowedConnectorIDs, id) {
			return fmt.Errorf("Manus connector %q is not allowlisted", id)
		}
	}
	for _, id := range append(append([]string{}, enabledSkillIDs...), forcedSkillIDs...) {
		if !containsID(p.AllowedSkillIDs, id) {
			return fmt.Errorf("Manus skill %q is not allowlisted", id)
		}
	}
	return nil
}

func containsID(allowed []string, candidate string) bool {
	candidate = strings.TrimSpace(candidate)
	for _, id := range allowed {
		if strings.TrimSpace(id) == candidate && candidate != "" {
			return true
		}
	}
	return false
}
