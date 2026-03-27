package agent

import (
	"io"
	"log/slog"
	"testing"

	"aurago/internal/memory"
)

func TestAgentTelemetrySnapshotIncludesParseAndRecoveryCounts(t *testing.T) {
	resetAgentTelemetryForTest()

	RecordToolParseSource(ToolCallParseSourceNative)
	RecordToolParseSource(ToolCallParseSourceNative)
	RecordToolParseSource(ToolCallParseSourceContentJSON)
	RecordToolRecoveryEvent("provider_422_recovered")
	RecordToolRecoveryEvent("duplicate_tool_call_blocked")
	RecordToolPolicyEvent("conservative_profile_applied")

	snapshot := GetAgentTelemetrySnapshot()

	if got := snapshot.ParseSources[string(ToolCallParseSourceNative)]; got != 2 {
		t.Fatalf("native parse count = %d, want 2", got)
	}
	if got := snapshot.ParseSources[string(ToolCallParseSourceContentJSON)]; got != 1 {
		t.Fatalf("content json parse count = %d, want 1", got)
	}
	if got := snapshot.RecoveryEvents["provider_422_recovered"]; got != 1 {
		t.Fatalf("422 recovery count = %d, want 1", got)
	}
	if got := snapshot.RecoveryEvents["duplicate_tool_call_blocked"]; got != 1 {
		t.Fatalf("duplicate breaker count = %d, want 1", got)
	}
	if got := snapshot.PolicyEvents["conservative_profile_applied"]; got != 1 {
		t.Fatalf("policy event count = %d, want 1", got)
	}
	if len(snapshot.Scopes) != 0 {
		t.Fatalf("expected no scoped telemetry for unscoped records, got %d scopes", len(snapshot.Scopes))
	}
}

func TestAgentTelemetrySnapshotIncludesScopedCounters(t *testing.T) {
	resetAgentTelemetryForTest()

	scope := AgentTelemetryScope{ProviderType: "openrouter", Model: "deepseek-chat"}
	RecordToolParseSourceForScope(scope, ToolCallParseSourceNative)
	RecordToolRecoveryEventForScope(scope, "provider_422_recovered")
	RecordToolPolicyEventForScope(scope, "conservative_profile_applied")
	RecordScopedToolResultForTool(scope, "homepage", true)
	RecordScopedToolResultForTool(scope, "homepage", false)

	snapshot := GetAgentTelemetrySnapshot()
	if len(snapshot.Scopes) != 1 {
		t.Fatalf("expected 1 scope, got %d", len(snapshot.Scopes))
	}
	if snapshot.Scopes[0].ProviderType != "openrouter" || snapshot.Scopes[0].Model != "deepseek-chat" {
		t.Fatalf("unexpected scope: %+v", snapshot.Scopes[0])
	}
	if got := snapshot.Scopes[0].ParseSources["native"]; got != 1 {
		t.Fatalf("scoped native parse count = %d, want 1", got)
	}
	if got := snapshot.Scopes[0].RecoveryEvents["provider_422_recovered"]; got != 1 {
		t.Fatalf("scoped recovery count = %d, want 1", got)
	}
	if got := snapshot.Scopes[0].PolicyEvents["conservative_profile_applied"]; got != 1 {
		t.Fatalf("scoped policy event count = %d, want 1", got)
	}
	if snapshot.Scopes[0].ToolCalls != 2 || snapshot.Scopes[0].ToolFailures != 1 {
		t.Fatalf("unexpected scoped tool stats: %+v", snapshot.Scopes[0])
	}
	if snapshot.Scopes[0].SuccessRate != 0.5 || snapshot.Scopes[0].FailureRate != 0.5 {
		t.Fatalf("unexpected scoped rates: success=%v failure=%v", snapshot.Scopes[0].SuccessRate, snapshot.Scopes[0].FailureRate)
	}
	family := snapshot.Scopes[0].ToolFamilies["deployment"]
	if family.ToolCalls != 2 || family.ToolFailures != 1 {
		t.Fatalf("unexpected scoped family stats: %+v", family)
	}
}

func TestInitializeAgentTelemetryPersistenceLoadsPersistedCounters(t *testing.T) {
	resetAgentTelemetryForTest()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	defer stm.Close()

	if err := stm.UpsertAgentTelemetry("parse_source", "native"); err != nil {
		t.Fatalf("UpsertAgentTelemetry parse_source: %v", err)
	}
	if err := stm.UpsertAgentTelemetry("recovery_event", "provider_422_recovered"); err != nil {
		t.Fatalf("UpsertAgentTelemetry recovery_event: %v", err)
	}
	if err := stm.UpsertAgentTelemetry("policy_event", "conservative_profile_applied"); err != nil {
		t.Fatalf("UpsertAgentTelemetry policy_event: %v", err)
	}

	InitializeAgentTelemetryPersistence(stm)
	snapshot := GetAgentTelemetrySnapshot()

	if got := snapshot.ParseSources["native"]; got != 1 {
		t.Fatalf("loaded native parse count = %d, want 1", got)
	}
	if got := snapshot.RecoveryEvents["provider_422_recovered"]; got != 1 {
		t.Fatalf("loaded recovery count = %d, want 1", got)
	}
	if got := snapshot.PolicyEvents["conservative_profile_applied"]; got != 1 {
		t.Fatalf("loaded policy event count = %d, want 1", got)
	}
	if len(snapshot.Scopes) != 0 {
		t.Fatalf("expected no scoped entries from unscoped persistence, got %d", len(snapshot.Scopes))
	}
}

func TestInitializeAgentTelemetryPersistenceLoadsScopedCounters(t *testing.T) {
	resetAgentTelemetryForTest()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	defer stm.Close()

	if err := stm.UpsertScopedAgentTelemetry("openrouter", "deepseek-chat", "parse_source", "native"); err != nil {
		t.Fatalf("UpsertScopedAgentTelemetry parse_source: %v", err)
	}
	if err := stm.UpsertScopedAgentTelemetry("openrouter", "deepseek-chat", "recovery_event", "provider_422_recovered"); err != nil {
		t.Fatalf("UpsertScopedAgentTelemetry recovery_event: %v", err)
	}
	if err := stm.UpsertScopedAgentTelemetry("openrouter", "deepseek-chat", "policy_event", "conservative_profile_applied"); err != nil {
		t.Fatalf("UpsertScopedAgentTelemetry policy_event: %v", err)
	}
	if err := stm.UpsertScopedAgentTelemetry("openrouter", "deepseek-chat", "tool_family_result", "deployment|success"); err != nil {
		t.Fatalf("UpsertScopedAgentTelemetry tool_family_result success: %v", err)
	}
	if err := stm.UpsertScopedAgentTelemetry("openrouter", "deepseek-chat", "tool_family_result", "deployment|failure"); err != nil {
		t.Fatalf("UpsertScopedAgentTelemetry tool_family_result failure: %v", err)
	}
	if err := stm.UpsertScopedAgentTelemetry("openrouter", "deepseek-chat", "tool_result", "success"); err != nil {
		t.Fatalf("UpsertScopedAgentTelemetry tool_result success: %v", err)
	}
	if err := stm.UpsertScopedAgentTelemetry("openrouter", "deepseek-chat", "tool_result", "failure"); err != nil {
		t.Fatalf("UpsertScopedAgentTelemetry tool_result failure: %v", err)
	}

	InitializeAgentTelemetryPersistence(stm)
	snapshot := GetAgentTelemetrySnapshot()

	if len(snapshot.Scopes) != 1 {
		t.Fatalf("expected 1 scoped entry, got %d", len(snapshot.Scopes))
	}
	if got := snapshot.Scopes[0].ParseSources["native"]; got != 1 {
		t.Fatalf("loaded scoped native parse count = %d, want 1", got)
	}
	if got := snapshot.Scopes[0].RecoveryEvents["provider_422_recovered"]; got != 1 {
		t.Fatalf("loaded scoped recovery count = %d, want 1", got)
	}
	if got := snapshot.Scopes[0].PolicyEvents["conservative_profile_applied"]; got != 1 {
		t.Fatalf("loaded scoped policy event count = %d, want 1", got)
	}
	if family := snapshot.Scopes[0].ToolFamilies["deployment"]; family.ToolCalls != 2 || family.ToolFailures != 1 {
		t.Fatalf("unexpected loaded scoped family stats: %+v", family)
	}
	if snapshot.Scopes[0].ToolCalls != 2 || snapshot.Scopes[0].ToolFailures != 1 {
		t.Fatalf("unexpected loaded scoped tool stats: %+v", snapshot.Scopes[0])
	}
}
