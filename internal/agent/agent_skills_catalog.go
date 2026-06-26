package agent

import (
	"fmt"
	"strings"

	"aurago/internal/tools"
)

func buildAgentSkillsPromptCatalog() string {
	mgr := tools.DefaultAgentSkillManager()
	if mgr == nil {
		return ""
	}
	entries, err := mgr.ListAgentSkills(true, "")
	if err != nil || len(entries) == 0 {
		return ""
	}
	var b strings.Builder
	for _, entry := range entries {
		if entry.SecurityStatus == tools.SecurityDangerous || entry.SecurityStatus == tools.SecurityError || entry.SecurityStatus == tools.SecurityPending {
			continue
		}
		if entry.SecurityStatus == tools.SecurityWarning && !entry.WarningApproved {
			continue
		}
		name := safePromptMetadataText(entry.Name, 80)
		if name == "" {
			continue
		}
		description := safePromptMetadataText(entry.Description, 180)
		b.WriteString(fmt.Sprintf("- `%s`: %s\n", name, description))
	}
	return strings.TrimSpace(b.String())
}
