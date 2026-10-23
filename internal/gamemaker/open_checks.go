package gamemaker

import (
	"encoding/json"
	"strings"
)

const (
	// maxOpenChecks bounds the failure card payload. A repair round cannot act on
	// more than a handful of open checks anyway.
	maxOpenChecks = 6
	// maxOpenCheckSteps bounds the executed steps stored per check.
	maxOpenCheckSteps = 8
	// maxOpenCheckText bounds every free-text field in runes.
	maxOpenCheckText = 240
)

// openChecksFromResult keeps the concrete still-open checks of one validation
// result together with the bounded model-facing next step. "passed" and
// "skipped" checks carry no instruction for the operator and stay out.
func openChecksFromResult(result *BuildResult) []OpenCheck {
	if result == nil {
		return nil
	}
	next := ValidationNextAction(*result)
	checks := make([]OpenCheck, 0, maxOpenChecks)
	for _, check := range result.Checks {
		if len(checks) >= maxOpenChecks {
			break
		}
		if check.Status != "failed" && check.Status != "unavailable" {
			continue
		}
		entry := OpenCheck{
			ID:         truncateOpenCheckText(check.ID),
			Status:     check.Status,
			Expected:   truncateOpenCheckText(check.Expected),
			Observed:   truncateOpenCheckText(check.Observed),
			Repairable: check.Repairable,
			NextAction: next,
		}
		if len(check.Steps) > 0 {
			entry.Steps = append(entry.Steps, check.Steps[:min(len(check.Steps), maxOpenCheckSteps)]...)
		}
		checks = append(checks, entry)
	}
	return checks
}

func truncateOpenCheckText(value string) string {
	trimmed := strings.TrimSpace(value)
	runes := []rune(trimmed)
	if len(runes) <= maxOpenCheckText {
		return trimmed
	}
	return strings.TrimSpace(string(runes[:maxOpenCheckText])) + "…"
}

// encodeOpenChecks serializes the bounded list for the gm_jobs column. An empty
// list is stored as the empty string so old rows and new rows look the same.
func encodeOpenChecks(checks []OpenCheck) string {
	if len(checks) == 0 {
		return ""
	}
	if len(checks) > maxOpenChecks {
		checks = checks[:maxOpenChecks]
	}
	data, err := json.Marshal(checks)
	if err != nil {
		return ""
	}
	return string(data)
}

// decodeOpenChecks reads a stored column value. Unreadable or oversized values
// degrade to "no open checks" instead of failing the project read.
func decodeOpenChecks(raw string) []OpenCheck {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var checks []OpenCheck
	if err := json.Unmarshal([]byte(raw), &checks); err != nil {
		return nil
	}
	if len(checks) == 0 {
		return nil
	}
	if len(checks) > maxOpenChecks {
		checks = checks[:maxOpenChecks]
	}
	return checks
}
