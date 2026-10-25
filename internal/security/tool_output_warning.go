package security

import (
	"strings"
	"sync"

	"github.com/danielthedm/promptsec"
)

// toolOutputValidator is the output phase of the Guardian's promptsec chain:
// buildPromptSecGuards always adds the default output validator, and it is the
// only output guard (taint, heuristics and the judge run on input).
var toolOutputValidator = sync.OnceValue(func() *promptsec.Protector {
	return promptsec.New(promptsec.WithOutputValidator(&promptsec.OutputOptions{}))
})

// ToolOutputWarning returns the "\n[SECURITY WARNING: ...]" line that
// SanitizeToolOutput appends to an isolated tool output when the output
// validator flags it, or "" when it does not. The line grows with every
// flagged phrase, so tools that size their answers to the agent's output
// budget add its length (TestToolOutputWarningMatchesSanitizeToolOutput keeps
// both in step).
func ToolOutputWarning(isolated string) string {
	validation := toolOutputValidator().ValidateOutput(isolated, nil)
	if validation.Safe {
		return ""
	}
	return formatToolOutputWarning(validation.Threats)
}

// formatToolOutputWarning quotes only fixed detector categories. Detector
// prose stays in the log so a model-facing warning cannot carry payload text.
func formatToolOutputWarning(threats []promptsec.Threat) string {
	cats := make([]string, 0, len(threats))
	seen := make(map[string]bool, len(threats))
	for _, thr := range threats {
		cat := sanitizeThreatCategory(string(thr.Type))
		if cat == "" || seen[cat] {
			continue
		}
		seen[cat] = true
		cats = append(cats, cat)
	}
	if len(cats) == 0 {
		cats = []string{"unspecified"}
	}
	return "\n[SECURITY WARNING: " + strings.Join(cats, "; ") + "]"
}

func sanitizeThreatCategory(raw string) string {
	var b strings.Builder
	for _, r := range raw {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_':
			b.WriteRune(r)
		default:
			continue
		}
		if b.Len() >= 40 {
			break
		}
	}
	return b.String()
}
