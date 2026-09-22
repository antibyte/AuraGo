package planner

import (
	"database/sql"
	"errors"
	"strings"
	"time"
)

// ResolveLegacyMaintenanceIssues closes only fingerprints emitted by the old
// explicit maintenance error paths, after the matching phase really completed.
func ResolveLegacyMaintenanceIssues(db *sql.DB, phase, promptPath string, now time.Time) error {
	var legacy []OperationalIssue
	switch phase {
	case "agent_loop":
		for _, title := range []string{"Maintenance agent loop failed", "Maintenance agent returned no response"} {
			legacy = append(legacy, OperationalIssue{Source: "maintenance", Context: "maintenance", Title: title, Reference: "daily_maintenance"})
		}
	case "prompt_load":
		if strings.TrimSpace(promptPath) != "" {
			legacy = append(legacy, OperationalIssue{Source: "maintenance", Title: "Maintenance prompt could not be read", Reference: promptPath})
		}
	}
	var result error
	for _, issue := range legacy {
		_, err := ResolveOperationalIssue(db, operationalIssueFingerprint(issue), "The matching maintenance phase completed successfully.", now)
		result = errors.Join(result, err)
	}
	return result
}
