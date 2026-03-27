package agent

import (
	"strings"
	"testing"
)

func TestApplyToolOutputPolicyTruncatesLargeSuccessOutput(t *testing.T) {
	resetAgentTelemetryForTest()

	scope := AgentTelemetryScope{ProviderType: "openrouter", Model: "gpt-4o-mini"}
	result := applyToolOutputPolicy(strings.Repeat("A", 300), 80, scope)

	if !result.Truncated {
		t.Fatal("expected result to be truncated")
	}
	if result.WasError {
		t.Fatal("did not expect success output to be marked as error")
	}
	if !strings.Contains(result.Content, "[Tool output truncated:") {
		t.Fatalf("expected truncation notice, got %q", result.Content)
	}

	snapshot, ok := GetScopedAgentTelemetrySnapshot(scope)
	if !ok {
		t.Fatal("expected scoped telemetry snapshot")
	}
	if got := snapshot.RecoveryEvents["tool_output_truncated"]; got != 1 {
		t.Fatalf("tool_output_truncated count = %d, want 1", got)
	}
	if got := snapshot.RecoveryEvents["error_output_truncated_preserved"]; got != 0 {
		t.Fatalf("error_output_truncated_preserved count = %d, want 0", got)
	}
}

func TestApplyToolOutputPolicyPreservesErrorSummaryWhenTruncated(t *testing.T) {
	resetAgentTelemetryForTest()

	scope := AgentTelemetryScope{ProviderType: "openrouter", Model: "deepseek-chat"}
	longTail := strings.Repeat("X", 400)
	raw := `{"status":"error","message":"permission denied while deploying homepage","details":"` + longTail + `"}`

	result := applyToolOutputPolicy(raw, 90, scope)

	if !result.Truncated {
		t.Fatal("expected error result to be truncated")
	}
	if !result.WasError {
		t.Fatal("expected error result to be marked as error")
	}
	if !strings.Contains(result.Content, "[Preserved error summary]") {
		t.Fatalf("expected preserved error summary block, got %q", result.Content)
	}
	if !strings.Contains(result.Content, "permission denied while deploying homepage") {
		t.Fatalf("expected preserved message in truncated content, got %q", result.Content)
	}
	if result.ErrorSummary != "permission denied while deploying homepage" {
		t.Fatalf("error summary = %q, want preserved JSON message", result.ErrorSummary)
	}

	snapshot, ok := GetScopedAgentTelemetrySnapshot(scope)
	if !ok {
		t.Fatal("expected scoped telemetry snapshot")
	}
	if got := snapshot.RecoveryEvents["tool_output_truncated"]; got != 1 {
		t.Fatalf("tool_output_truncated count = %d, want 1", got)
	}
	if got := snapshot.RecoveryEvents["error_output_truncated_preserved"]; got != 1 {
		t.Fatalf("error_output_truncated_preserved count = %d, want 1", got)
	}
}
