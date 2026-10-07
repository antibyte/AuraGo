package agent

import (
	"strings"
	"testing"

	"aurago/internal/security"
)

// Error summaries feed error patterns, journal entries and learned rules; they
// must come from the decoded tool text, never from the isolation wrapper.
// Execution output is always escaped external data; the readable source form
// (project source tools) must decode the same way.
func TestExtractErrorMessageUnwrapsIsolatedExecutionOutput(t *testing.T) {
	g := security.NewGuardian(nil)
	shellBody := "STDOUT:\nbuilding app\n[EXECUTION ERROR]: exit status 2\n" +
		"[Shell: /bin/sh (POSIX sh). Bash-specific syntax (e.g. process substitution <(...), [[ ]], arrays) is NOT available.]\n"
	cases := []struct {
		name, isolateAs, body, want string
		raw                         bool
	}{
		{
			name:      "escaped execution output",
			isolateAs: "execute_shell",
			body:      shellBody,
			want:      "STDOUT:\nbuilding app\n[EXECUTION ERROR]: exit status 2",
		},
		{
			name:      "readable source form",
			isolateAs: "game_maker_file",
			body:      shellBody,
			want:      "STDOUT:\nbuilding app\n[EXECUTION ERROR]: exit status 2",
			raw:       true,
		},
		{
			name:      "escaped ampersands",
			isolateAs: "execute_shell",
			body:      "STDOUT:\nmake a && b failed at /srv/app?x=1&y=2\n[EXECUTION ERROR]: exit status 2\n",
			want:      "STDOUT:\nmake a && b failed at /srv/app?x=1&y=2\n[EXECUTION ERROR]: exit status 2",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			isolated := g.SanitizeToolOutput(tc.isolateAs, tc.body)
			if _, ok, raw := toolResultPayloadForm(isolated); !ok || raw != tc.raw {
				t.Fatalf("fixture form: isolated=%v raw=%v, want raw=%v: %q", ok, raw, tc.raw, isolated)
			}
			for shape, output := range map[string]string{
				"native":    isolated,
				"text mode": formatToolOutputForModel(ToolCall{Action: "execute_shell"}, isolated),
				"with hint": augmentToolFailureContent(ToolCall{Action: "execute_shell"}, isolated, ""),
			} {
				got := extractErrorMessage(output)
				if strings.Contains(got, "external_data") || !strings.Contains(got, "exit status 2") {
					t.Fatalf("%s: summary not derived from decoded text: %q", shape, got)
				}
				if !strings.HasPrefix(got, tc.want) || strings.Contains(got, "&amp;") {
					t.Fatalf("%s: summary = %q, want unescaped text starting %q", shape, got, tc.want)
				}
				policy := applyToolOutputPolicy(output, 50000, AgentTelemetryScope{}, ToolResultFailed)
				if policy.ErrorSummary != got {
					t.Fatalf("%s: ErrorSummary = %q, want %q", shape, policy.ErrorSummary, got)
				}
			}
		})
	}
}

func TestExtractErrorMessageKeepsNonIsolatedOutput(t *testing.T) {
	for _, tc := range []struct{ output, want string }{
		{`Tool Output: {"status":"error","message":"connect failed"}`, "connect failed"},
		{"Tool Output: [EXECUTION ERROR] 'command' is required for execute_shell.", "Tool Output: [EXECUTION ERROR] 'command' is required for execute_shell."},
		{"<external_data>\nunterminated wrapper", "<external_data>\nunterminated wrapper"},
	} {
		if got := extractErrorMessage(tc.output); got != tc.want {
			t.Errorf("extractErrorMessage(%q) = %q, want %q", tc.output, got, tc.want)
		}
	}
}
