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
		b.WriteString(fmt.Sprintf("- `%s`: %s\n", entry.Name, strings.TrimSpace(entry.Description)))
	}
	return strings.TrimSpace(b.String())
}
