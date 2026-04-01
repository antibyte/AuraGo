package agent

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestApplyToolOutputPolicyTruncatesLargeSuccessOutput(t *testing.T) {
	resetAgentTelemetryForTest()

	scope := AgentTelemetryScope{ProviderType: "openrouter", Model: "gpt-4o-mini"}
	result := applyToolOutputPolicy(strings.Repeat("A", 300), 160, scope)

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

	result := applyToolOutputPolicy(raw, 220, scope)

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

func TestApplyToolOutputPolicyProducesValidUTF8WithinLimit(t *testing.T) {
	resetAgentTelemetryForTest()

	scope := AgentTelemetryScope{ProviderType: "openrouter", Model: "gpt-4o-mini"}
	limit := 160
	result := applyToolOutputPolicy(strings.Repeat("界", 120), limit, scope)

	if !result.Truncated {
		t.Fatal("expected result to be truncated")
	}
	if len(result.Content) > limit {
		t.Fatalf("content length = %d, want <= %d", len(result.Content), limit)
	}
	if !utf8.ValidString(result.Content) {
		t.Fatalf("expected valid UTF-8 output, got %q", result.Content)
	}
	if !strings.Contains(result.Content, "[Tool output truncated:") {
		t.Fatalf("expected truncation notice, got %q", result.Content)
	}
	if !strings.Contains(result.Content, "界") {
		t.Fatalf("expected preserved result prefix, got %q", result.Content)
	}
}

func TestApplyToolOutputPolicyPreservesErrorSummaryWithinLimit(t *testing.T) {
	resetAgentTelemetryForTest()

	scope := AgentTelemetryScope{ProviderType: "openrouter", Model: "deepseek-chat"}
	limit := 220
	raw := `{"status":"error","message":"界界界 permission denied while deploying homepage","details":"` + strings.Repeat("界", 150) + `"}`

	result := applyToolOutputPolicy(raw, limit, scope)

	if !result.Truncated {
		t.Fatal("expected result to be truncated")
	}
	if len(result.Content) > limit {
		t.Fatalf("content length = %d, want <= %d", len(result.Content), limit)
	}
	if !utf8.ValidString(result.Content) {
		t.Fatalf("expected valid UTF-8 output, got %q", result.Content)
	}
	if !strings.Contains(result.Content, "[Preserved error summary]") {
		t.Fatalf("expected preserved error summary block, got %q", result.Content)
	}
	if !strings.Contains(result.Content, "permission denied while deploying homepage") {
		t.Fatalf("expected preserved error summary, got %q", result.Content)
	}
}
