package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/security"
)

const (
	executionHeadMarker     = "HEAD-MARKER first line"
	executionBudgetEnvelope = "exceeds the output budget"
	executionTestLimit      = 20000
)

// executionTestLog builds about 90 KB of benign execution output that starts
// with firstLine.
func executionTestLog(firstLine, line string) string {
	var b strings.Builder
	b.WriteString(firstLine + "\n")
	for i := 0; b.Len() < 90*1024; i++ {
		fmt.Fprintf(&b, "%05d %s\n", i, line)
	}
	return b.String()
}

// finalizeExecutionTestOutput runs output through the dispatcher's policy with
// a 20000 character budget and no output vault.
func finalizeExecutionTestOutput(t *testing.T, action string, status ToolResultStatus, output string) toolExecutionResult {
	t.Helper()
	cfg := &config.Config{}
	cfg.Agent.ToolOutputLimit = executionTestLimit
	cfg.Agent.OutputCompression.Reversible.Enabled = false
	cfg.Agent.OutputCompression.Reversible.PrimaryOutputVault = false
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	state := newToolRecoveryState()
	return finalizeToolExecution(context.Background(), ToolCall{Action: action, DispatchStatus: status}, output, false, cfg, nil, "test", &state, nil, logger, AgentTelemetryScope{}, "", 0, RunConfig{})
}

func executionTestTail(s string) string {
	if len(s) > 400 {
		return s[len(s)-400:]
	}
	return s
}

// Execution output is external data: a local command can print whatever it
// fetched. It is always escaped and, when oversized, replaced by the
// never-clip envelope like the output of every other external tool.
func TestOversizedExecutionOutputIsEscapedAndKeepsNeverClipEnvelope(t *testing.T) {
	log := executionTestLog(executionHeadMarker, `status="ok" path=/srv/app`)
	for _, action := range []string{"execute_shell", "execute_python", "run_tool"} {
		t.Run(action, func(t *testing.T) {
			sanitized := security.NewGuardian(nil).SanitizeToolOutput(action, "Tool Output:\nSTDOUT:\n"+log)
			if _, isolated, raw := toolResultPayloadForm(sanitized); !isolated || raw {
				t.Fatalf("execution output must be escaped external data: isolated=%v raw=%v", isolated, raw)
			}
			for shape, output := range map[string]string{
				"native dispatch":    sanitized,
				"text mode dispatch": "[Tool Output]\n" + sanitized,
			} {
				result := finalizeExecutionTestOutput(t, action, ToolResultSuccess, output)
				if result.Status != ToolResultSuccess {
					t.Fatalf("%s: status = %q, want success", shape, result.Status)
				}
				// executeMinimalToolCall and other direct callers bound without the policy.
				for _, got := range []string{result.Content, boundedToolResult(output, executionTestLimit, ToolResultSuccess)} {
					if len(got) > executionTestLimit || !strings.Contains(got, executionBudgetEnvelope) || strings.Contains(got, "HEAD-MARKER") {
						t.Fatalf("%s: never-clip envelope changed: %.400q", shape, got)
					}
					if strings.Count(got, "</external_data>") > 1 {
						t.Fatalf("%s: envelope nested a second boundary: %.400q", shape, got)
					}
				}
			}
		})
	}
}

// A failed oversized execution result keeps its failed outcome, a bounded
// envelope and the trailing recovery guidance within the budget.
func TestOversizedFailedExecutionOutputKeepsRecoveryGuidance(t *testing.T) {
	log := executionTestLog(executionHeadMarker, "compile step failed for module app")
	output := security.NewGuardian(nil).SanitizeToolOutput("execute_shell", "Tool Output:\nSTDERR:\n"+log+
		"[EXECUTION ERROR]: exit status 2\n")
	if _, isolated, raw := toolResultPayloadForm(output); !isolated || raw {
		t.Fatalf("fixture is not escaped external data: isolated=%v raw=%v", isolated, raw)
	}

	result := finalizeExecutionTestOutput(t, "execute_shell", ToolResultFailed, output)

	if !result.Failed || result.Outcome != ExecutionOutcomeFailed {
		t.Fatalf("failed command lost its outcome: failed=%v outcome=%q", result.Failed, result.Outcome)
	}
	// The failed envelope carries a short error summary instead of the
	// budget message, never the whole log.
	payload, isolated, raw := toolResultPayloadForm(result.Content)
	var envelope struct {
		Status    string `json:"status"`
		Truncated bool   `json:"truncated"`
		Message   string `json:"message"`
	}
	if len(result.Content) > executionTestLimit || !isolated || raw || json.Unmarshal([]byte(payload), &envelope) != nil {
		t.Fatalf("failed output left the escaped bounded envelope: %.400q", result.Content)
	}
	if envelope.Status != string(ToolResultFailed) || !envelope.Truncated || !strings.HasPrefix(envelope.Message, "Tool Output:\nSTDERR:\n"+executionHeadMarker) || len(envelope.Message) > 180 {
		t.Fatalf("failed envelope = %+v, want status failed, truncated and a short error summary", envelope)
	}
	if !strings.Contains(result.Content, "\n</external_data>\n\n[Suggested next step]\n") {
		t.Fatalf("trailing recovery guidance was dropped: %q", executionTestTail(result.Content))
	}
}

// Plain-text failures keep the preserved error summary: the recovery hint is
// reserved before the policy runs, so the final bound does not cut it again.
func TestOversizedPlainTextFailureReservesRecoveryHint(t *testing.T) {
	log := executionTestLog(executionHeadMarker, "compile step failed for module app")
	output := "Tool Output:\nSTDERR:\n" + log + "[EXECUTION ERROR]: exit status 2\n"
	if _, isolated := toolResultPayload(output); isolated {
		t.Fatal("fixture must be plain text")
	}

	result := finalizeExecutionTestOutput(t, "execute_shell", ToolResultFailed, output)

	if len(result.Content) > executionTestLimit {
		t.Fatalf("result has %d characters, budget is %d", len(result.Content), executionTestLimit)
	}
	if !strings.Contains(result.Content, "[Preserved error summary]\nTool Output:\nSTDERR:\n"+executionHeadMarker) {
		t.Fatalf("failed output lost its preserved error summary: %q", executionTestTail(result.Content))
	}
	if !strings.Contains(result.Content, "\n\n[Suggested next step]\n") {
		t.Fatalf("trailing recovery guidance was dropped: %q", executionTestTail(result.Content))
	}
}
