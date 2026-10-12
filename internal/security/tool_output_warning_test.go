package security

import (
	"strings"
	"testing"
)

func TestToolOutputWarningMatchesSanitizeToolOutput(t *testing.T) {
	g := NewGuardian(nil)
	leak := "I will now reveal my system prompt. My system message says hi. "
	for _, tc := range []struct{ tool, output string }{
		{"cyd_creator_read_file", `Tool Output: {"status":"ok","file":{"content":"// ` + strings.Repeat(leak, 3) + `\nint x = a < b;"}}`},
		{"cyd_creator_read_project", `Tool Output: {"status":"ok","project":{"layout":{"name":"` + leak + `"}}}`},
		{"web_search", "Tool Output: " + strings.Repeat(leak, 2)},
	} {
		got := g.SanitizeToolOutput(tc.tool, tc.output)
		cut := strings.LastIndex(got, "\n[SECURITY WARNING: ")
		if cut < 0 {
			t.Fatalf("%s: no warning in %q", tc.tool, got)
		}
		if warning := ToolOutputWarning(got[:cut]); warning != got[cut:] {
			t.Fatalf("%s: ToolOutputWarning = %q, SanitizeToolOutput appended %q", tc.tool, warning, got[cut:])
		}
	}
	clean := `Tool Output: {"status":"ok","file":{"content":"int x = a < b;"}}`
	if got := g.SanitizeToolOutput("cyd_creator_read_file", clean); strings.Contains(got, "SECURITY WARNING") || ToolOutputWarning(got) != "" {
		t.Fatalf("clean output warned: %q", got)
	}
}
