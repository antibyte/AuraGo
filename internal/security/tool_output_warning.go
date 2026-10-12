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
	msgs := make([]string, 0, len(validation.Threats))
	for _, thr := range validation.Threats {
		msgs = append(msgs, thr.Message)
	}
	return "\n[SECURITY WARNING: " + strings.Join(msgs, "; ") + "]"
}
