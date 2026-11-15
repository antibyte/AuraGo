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
