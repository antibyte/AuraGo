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
// validator flags it, or "" when it does not. That is the whole suffix of
// source output (CYD Creator sizes its answers with it) and of external output
// the scanner left alone. External output the scanner rated Medium, or whose
// text has a directive shape, gets the same line with those categories added
// after the validator's; only a scan of the original text can name them, so
// this cheap check is then a prefix of the suffix, not all of it. The line
// grows with every flagged phrase, so tools that size their answers to the
// agent's output budget add its length (TestToolOutputWarningMatchesSanitizeToolOutput
// keeps both in step).
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
	return formatToolOutputWarningWith(threats, nil)
}

// formatToolOutputWarningWith lists the validator's threat categories followed
// by extra categories (the scanner's and the directive shape), each sanitized
// and listed once.
func formatToolOutputWarningWith(threats []promptsec.Threat, extra []string) string {
	cats := make([]string, 0, len(threats)+len(extra))
	seen := make(map[string]bool, len(threats)+len(extra))
	add := func(raw string) {
		cat := sanitizeThreatCategory(raw)
		if cat == "" || seen[cat] {
			return
		}
		seen[cat] = true
		cats = append(cats, cat)
	}
	for _, thr := range threats {
		add(string(thr.Type))
	}
	for _, raw := range extra {
		add(raw)
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
